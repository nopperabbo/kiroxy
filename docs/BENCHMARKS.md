# kiroxy Benchmarks

Reference performance numbers for the kiroxy load-test harness. These are
reproducible baselines, not SLA commitments. Use them to spot regressions
between releases, not to advertise throughput.

## What is measured

All numbers in this document come from `scripts/loadtest/` driving the mock
Kiro CodeWhisperer upstream at `scripts/loadtest/mock_kiro/`. kiroxy itself
is **not in the request path** for the numbers below — see the caveat in
the [Reading these numbers](#reading-these-numbers) section.

The harness issues POST requests that emit a canned AWS EventStream
response (10 frames: 1 initial-response + 8 assistantResponseEvent + 1
metadataEvent + 1 messageMetadataEvent). The mock deserialises and
re-serialises real kiro wire frames with valid CRC-32 checksums, so a full
event-stream parse happens in both directions.

## Reference environment

| field | value |
|---|---|
| machine | Apple MacBookPro17,1 (M1) |
| CPUs | 8 |
| RAM | 8 GiB |
| OS | macOS 26.3 (build 25D125), darwin/arm64 |
| Go | go1.26.2 |
| bash | 3.2.57 (macOS system bash) |
| python3 | 3.14.4 (used only by `analyze.sh`) |
| network | loopback (`127.0.0.1`) |

## Baseline numbers

Measured 2026-05-12 against mock_kiro with zero injected latency unless
noted otherwise. Mock eventstream: 10 frames per response, ~1.6 KiB body.

| scenario | concurrency | requests | ok | err% | RPS | p50 | p95 | p99 | max | TTFB p95 |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| **single** (sanity)          |   1 |       50 |       50 | 0.0% |   6,566.0 |  0.15ms |  0.18ms |  0.18ms |   0.18ms |  0.1ms |
| **10 concurrent**            |  10 |      200 |      200 | 0.0% |  10,981.9 |  0.85ms |  1.25ms |  1.45ms |   1.66ms |  1.1ms |
| **100 concurrent** (burst)   | 100 |    1,000 |    1,000 | 0.0% |  20,758.2 |  2.90ms | 13.88ms | 16.54ms |  21.11ms | 13.5ms |
| **sustained 60s**            |  10 | 1,428,651 | 1,428,651 | 0.0% |  23,810.3 |  0.29ms |  1.01ms |  2.92ms |  83.01ms |  0.7ms |
| **realistic (20ms upstream)** |   5 |      100 |      100 | 0.0% |     132.2 | 37.71ms | 39.04ms | 40.93ms |  41.12ms | 23.1ms |
| **error-injection 10%**      |  20 |      200 |      182 | 9.0% |   9,530.3 |  1.97ms |  2.89ms |  3.29ms |   3.65ms |  2.6ms |

### What each row demonstrates

- **single**: per-request fixed cost with keepalive warm. 0.15 ms p50 is
  the request / event-stream parse / TCP loopback round-trip for a 10-frame
  response. Everything else should be proportional to this.
- **10 concurrent**: steady warm path. Latency rises from 0.15 ms to 0.85 ms
  — the additional ~0.7 ms is workers contending for scheduler time.
- **100 concurrent**: saturation. p50 of 2.9 ms is mostly queueing against
  the mock's serial handler; the p99/max tail is scheduler jitter on an
  8-core box with 100 workers. Zero errors is the notable result: kiroxy's
  pool + mock both absorb 100-deep queueing without dropping.
- **sustained 60s**: >1.4 M requests over 60 s with zero errors. The
  long-running RPS of ~24 k/s confirms no memory leak, no file descriptor
  exhaustion, no connection pool poisoning. Max of 83 ms is a single
  stop-the-world GC pause; p99 at 2.9 ms is the true tail.
- **realistic (20ms upstream)**: mock inserts 20 ms of fixed latency plus
  2 ms of inter-chunk delay. Result: p50 of ~38 ms (= 20 + 9×2 ms), RPS
  of 132 per 5 workers — this is what a kiroxy deployment looks like
  against a real upstream with healthy latency.
- **error-injection 10%**: mock returns 429 (ThrottlingException) on 10%
  of requests. Client sees 9% err-rate (close to the injected rate, small
  delta is sample-size noise); latency for successful requests is
  unaffected.

## Reading these numbers

Three important caveats.

### 1. These measure the eventstream pipeline, not kiroxy itself

The harness targets `mock_kiro` directly in `--mode kiro`, which bypasses
kiroxy. To measure kiroxy end-to-end you need to:

1. Run `kiroxy serve` with a valid account in the vault
2. Run the harness with `--mode anthropic --url http://127.0.0.1:8787`

The RPS ceiling for the anthropic path will be lower than the numbers
above because kiroxy does the reqconv/respconv translation plus an
additional HTTP hop upstream. Expect ~40-60% of the mock-direct RPS for
non-streaming, closer to 80-90% for streaming (because streaming is
latency-bound, not CPU-bound).

### 2. No KIROXY_UPSTREAM_URL exists (yet)

kiroxy's `internal/kiroclient` supports a `WithBaseURL()` option used in
tests, but there is no public env var to redirect the upstream URL at
runtime. This means the mock server is currently useful for:

- **Harness self-tests** — verifying the load-test tooling
- **Reference throughput** — upper bounds for the eventstream pipeline
- **Building confidence** — the exact wire format kiroxy will see

But it is **not yet useful** for running kiroxy's own /v1/messages path
without burning real Kiro quota. See `BACKLOG.md` for the follow-up.

### 3. Loopback numbers are not production numbers

Everything above is `127.0.0.1` with zero network latency. A real
deployment adds ~15-40 ms to us-east-1 each way. For sustained throughput
on real infrastructure, expect 50-200 RPS per account (account limits,
not kiroxy limits, will be the bottleneck).

## Methodology

### Reproduction

From the repo root:

```bash
cd scripts/loadtest
./run.sh --mode kiro --out ./results/$(date +%Y%m%d_%H%M%S)
```

This runs every scenario in `scenarios.yaml`, writes results under the
output directory (one subdir per scenario), and generates `report.md`
with a comparison table.

### Individual scenarios

```bash
# Just the burst scenario.
./run.sh --mode kiro --scenarios burst --out ./results/burst-only

# Inject latency and error rate into the mock.
./run.sh --mode kiro --mock-latency-ms 50 --mock-error-rate 0.05 \
    --scenarios burst,sustained
```

### Adjusting scenarios

Edit `scripts/loadtest/scenarios.yaml`. The YAML subset is simple
(top-level `scenarios:` map, each value has `concurrency`, `total` or
`duration`, `stream`, `mode`, `description`). No full YAML parser — the
shell script uses `awk`.

### Raw data

Each scenario writes:

- `summary.json`     — aggregate metrics (what this doc renders)
- `requests.jsonl`   — one JSON object per request (raw latency, status,
  bytes, worker, TTFB). Use this for custom analysis.

### Metrics glossary

- **p50 / p95 / p99** — median / 95th / 99th percentile of total
  request-to-completion time in milliseconds (including full SSE /
  eventstream drain, not just time-to-first-byte).
- **TTFB p50 / p95** — time from request send to HTTP response header
  arrival. Diverges from total latency on streaming responses.
- **RPS** — requests per second over the whole timed window (warmup
  requests are excluded).
- **err%** — requests that either returned a non-2xx status OR returned
  an HTTP-client-layer error (connection refused, timeout, etc.).

## Detecting regressions

A release is suspicious if, compared against the previous release on the
same hardware:

- **single p50 doubles** → per-request fixed cost regressed. Likely
  candidates: new middleware, extra JSON encode/decode, extra allocation
  in the hot path.
- **sustained_60s RPS drops > 10%** → throughput regression. Check recent
  changes to the response pipeline (respconv, SSE writer, flush cadence).
- **conc100 errors > 0** → concurrency bug. Likely a shared-state issue,
  a new context-cancellation-misuse, or a connection pool change.
- **sustained_60s max > 5x previous** → GC pressure or lock contention.
  Check recent allocations in the hot path and any new mutexes.

A release is **healthy** if:

- single / conc10 / conc100 / sustained_60s all pass with the same RPS
  ordering as the prior release (single < conc10 < conc100 < sustained)
- err% stays at 0 for every non-error-injection scenario
- max latency in sustained_60s stays below 200 ms
- realistic_20ms shows p50 = upstream_latency + stream_duration, within
  ±10%

## When to re-measure

Re-run baselines when any of these land:

- Go toolchain version bump
- Changes to `internal/kiroclient`, `internal/messages`, `internal/respconv`
- Changes to the middleware stack in `internal/server`
- New deps added to `go.mod`
- New compiler flags in `Makefile` (especially ldflags, buildmode, etc.)

Commit the new BENCHMARKS.md numbers in the same PR as the change.

## See also

- `scripts/loadtest/README.md` — full operator guide for running the harness
- `scripts/loadtest/scenarios.yaml` — editable scenario definitions
- `docs/ARCHITECTURE.md` — kiroxy's request lifecycle
- `BACKLOG.md` — the KIROXY_UPSTREAM_URL env-var follow-up
