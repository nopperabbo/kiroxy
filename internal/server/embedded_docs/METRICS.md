# METRICS

kiroxy exposes Prometheus-scrapeable metrics at `GET /metrics`. This doc
covers the scrape setup, the metric catalog (with cardinality notes), and
a starter Grafana dashboard.

---

## Scrape Setup

### Prometheus

Minimal `prometheus.yml` scrape_config:

```yaml
scrape_configs:
  - job_name: kiroxy
    metrics_path: /metrics
    static_configs:
      - targets: ["localhost:8787"]
```

For non-loopback scrapes, kiroxy requires the inbound API key by default:

```yaml
scrape_configs:
  - job_name: kiroxy
    metrics_path: /metrics
    static_configs:
      - targets: ["kiroxy.internal:8787"]
    authorization:
      type: Bearer
      credentials: "<KIROXY_API_KEY>"
```

Alternative for a trusted private network: set
`KIROXY_METRICS_PUBLIC=1` on the kiroxy process, and the `/metrics`
endpoint will serve anonymously. This is intended for a dedicated
metrics subnet or a sidecar Prometheus instance; avoid it on any host
reachable from outside your trust boundary.

### docker-compose

```yaml
services:
  kiroxy:
    image: kiroxy:latest
    ports: ["8787:8787"]
    environment:
      KIROXY_API_KEY: "<key>"
      # KIROXY_METRICS_PUBLIC: "1"   # only if prometheus is on a trusted net

  prometheus:
    image: prom/prometheus:latest
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml:ro
    ports: ["9090:9090"]
```

### Sanity check

```bash
curl -H "X-Api-Key: $KIROXY_API_KEY" http://localhost:8787/metrics | head -40
```

Expect to see `# HELP kiroxy_...` lines followed by samples.

---

## Metric Catalog

All metrics are prefixed `kiroxy_`. Standard `go_*` and `process_*` metrics
are also emitted (heap, GC, goroutines, open FDs).

### Counters

| Metric | Labels | Description |
|---|---|---|
| `kiroxy_requests_total` | `model`, `status`, `stream` | Count of `/v1/messages` requests. `status` is coarse (`2xx`/`3xx`/`4xx`/`5xx`), `stream` is `true`/`false`. |
| `kiroxy_request_errors_total` | `kind` | Typed error count. `kind` ∈ {`upstream`, `auth`, `proxy`, `invalid_request`}. |
| `kiroxy_refresh_attempts_total` | `kind`, `result` | Token refresh outcomes. `kind` ∈ {`proactive`, `reactive`}; `result` ∈ {`success`, `fail_401`, `fail_transient`, `fail_other`}. |
| `kiroxy_account_cooldowns_total` | `reason` | Pool cooldown transitions. `reason` ∈ {`quota`, `consecutive_errors`, `unauthorized`, `manual`}. |

### Gauges

| Metric | Labels | Description |
|---|---|---|
| `kiroxy_accounts_available` | none | Pool accounts enabled and not on cooldown. Snapshotted at scrape time. |
| `kiroxy_accounts_cooldown` | none | Pool accounts in an active cooldown window. |
| `kiroxy_accounts_failed` | none | Pool accounts administratively disabled. |
| `kiroxy_vault_generation` | none | Sum of generation counters across all bundles; monotonic-ish proxy for refresh activity. |
| `kiroxy_uptime_seconds` | none | Process uptime in seconds. |

### Histograms

| Metric | Labels | Buckets |
|---|---|---|
| `kiroxy_request_duration_seconds` | `model`, `stream` | 0.1, 0.25, 0.5, 1, 2, 5, 10, 20, 30, 60 seconds. |
| `kiroxy_upstream_ttfb_seconds` | `model` | 0.1, 0.25, 0.5, 0.75, 1, 1.5, 2, 3, 5 seconds. |
| `kiroxy_tokens_input` | `model` | 100, 200, 400, ..., 204800 (exponential, factor 2, 12 buckets). |
| `kiroxy_tokens_output` | `model` | Same as tokens_input. |

### Cardinality

Label cardinality is bounded by design:

- `model`: one of the 7 canonical Claude aliases returned to the client,
  plus the literal string `"unknown"` for pre-resolution error paths. The
  upstream Kiro SKU is NOT used as a label (it would duplicate).
- `status`: exactly 5 values (`2xx`, `3xx`, `4xx`, `5xx`, `other`).
- `stream`: exactly 2 values (`true`, `false`).
- `kind` / `result` / `reason`: small closed enums defined in
  `internal/metrics/sink.go`.

Account IDs, tokens, user identifiers, and session IDs are **never** used
as labels. This is a hard rule — adding one creates an unbounded
cardinality hazard that will OOM any Prometheus pulling from kiroxy.

---

## Useful Queries

### Error rate

```promql
sum(rate(kiroxy_requests_total{status=~"4..|5.."}[5m]))
/ sum(rate(kiroxy_requests_total[5m]))
```

### p99 request latency, streaming only

```promql
histogram_quantile(0.99,
  sum(rate(kiroxy_request_duration_seconds_bucket{stream="true"}[5m])) by (le, model)
)
```

### Refresh success rate

```promql
sum(rate(kiroxy_refresh_attempts_total{result="success"}[5m]))
/ sum(rate(kiroxy_refresh_attempts_total[5m]))
```

### Pool health

```promql
kiroxy_accounts_available
/ (kiroxy_accounts_available + kiroxy_accounts_cooldown + kiroxy_accounts_failed)
```

---

## Grafana Dashboard

A starter dashboard JSON is provided at
[`METRICS.grafana.json`](./METRICS.grafana.json). Import via
Grafana → Dashboards → Import → Upload JSON File. It assumes a datasource
named `Prometheus`; edit the `datasource.uid` fields if yours differs.

Panels included:

1. **Request rate** (`sum(rate(kiroxy_requests_total[5m])) by (status)`)
2. **Error rate** — 4xx and 5xx as a % of total.
3. **p50 / p95 / p99 request latency** per model.
4. **Upstream TTFB p95.**
5. **Input / output tokens** — histogram heatmap.
6. **Pool health** — stacked gauge: available / cooldown / failed.
7. **Refresh attempts** — rate by `(kind, result)`.
8. **Cooldown reasons** — rate of `kiroxy_account_cooldowns_total` by reason.
9. **Uptime** — single-stat of `kiroxy_uptime_seconds`.

---

## Performance

- A single `/metrics` scrape touches O(N) account slots for the pool
  gauges (via `Pool.Snapshot`). With a few dozen accounts the scrape
  completes in sub-millisecond. The vault generation sum issues one
  SQLite aggregate query per scrape (also sub-millisecond).
- No background goroutine is started for metrics collection; everything
  is pull-driven at scrape time.
- Binary size impact: ~6 MiB (client_golang + deps). We deemed this
  acceptable for first-class Prometheus compatibility.

---

## Security Notes

- The `/metrics` endpoint reveals operational characteristics (request
  counts, latency distributions, pool size). Treat it as sensitive
  telemetry and keep it behind the same access controls as `/dashboard`.
- No secrets are ever emitted as label values or metric values. The
  nil-safety of the Sink ensures a misconfigured registry cannot crash
  the request path.
- `KIROXY_METRICS_PUBLIC=1` disables auth entirely. Use only on a
  trusted private network.

---

## Implementation

- Package: `internal/metrics/` (Registry, Sink, label constants).
- Instrumentation hooks:
  - `messages.Service` — request lifecycle (handler.go, execute.go, response.go).
  - `pool.Pool` — cooldown transitions (`RecordFailure`), snapshot gauges.
  - `pool.RefreshConfig` — per-attempt result (`refreshOne`).
  - `tokenvault.Vault` — generation sum gauge.
- HTTP handler: `server.registerMetrics` (`internal/server/metrics.go`),
  auth decision layered into `authMiddleware`
  (`internal/server/auth.go`).

See `internal/metrics/sink.go` for the call-site-facing API and
`internal/metrics/registry.go` for the collector definitions and bucket
choices.
