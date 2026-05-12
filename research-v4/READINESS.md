# READINESS.md — kiroxy production-readiness audit

> Binary, evidence-based checklist for answering "is kiroxy ready to
> ship to friends today?"
>
> Every claim is anchored to file:line or a BACKLOG item. No hand-waving.
>
> Scope: v1.0.0 tag (shipped 2026-05-12) + 31 post-tag commits leading
> to v1.0.1. Audit date: 2026-05-13 Asia/Makassar.

---

## TL;DR

**kiroxy v1.0.0 + post-tag fixes is** _**close to**_ **ship-ready for a
technical friend, but NOT ready for a non-technical friend.**

- Core path works: health, metrics, dashboards, OpenAI-compat surface,
  pool + vault, proactive refresh, observability, tests.
- Blocking for v1.0.1: one live upstream 403 regression (tracked in
  BACKLOG P0) and one `expires_at` miscalibration (BACKLOG P0).
- Soft blockers for "friend install in 10 min": no OPERATIONS.md, no
  TLS guide beyond three README one-liners, SMOKE_TEST.md advertises
  a historical FAIL, `SMOKE_TEST.md` never re-run post-fix.
- Security posture is consistent with "single-user self-hosted,
  loopback default" — explicit TODOs listed at §5.

Work through the checklist at §7 ("Ship-to-friends"). Anything checked
[ ] is a release blocker.

---

## 1. Functional readiness

### 1.1 `/v1/messages` end-to-end

Status: **BLOCKED on upstream-403 regression.**

- Route registered: `internal/server/server.go:112`.
- Handler: `internal/messages/handler.go:45` (`HandleMessages`).
- Model resolver: `internal/models/models.go:120`.
- Request builder: `internal/reqconv/build_payload.go:31`.
- Upstream client: `internal/kiroclient/client.go:178` with 3-retry
  backoff, idle-reader, 403-refresh-retry.
- Response converter: streaming (`internal/respconv/streaming.go`) +
  non-streaming (`internal/respconv/nonstreaming.go`).

**Regression**: BACKLOG.md P0 entry ("Upstream 403 with fresh
credentials + new endpoint", dated 2026-05-12). Symptoms: freshly
refreshed access_token + migrated `runtime.us-east-1.kiro.dev` +
Phase 2.5 reactive refresh all present, yet `/v1/messages` returns
502 (upstream 403, empty body). Manual minimal curl with same token
+ profileArn returns 200 with valid EventStream. Hypothesis: kiroxy's
payload construction diverges from what the new endpoint accepts.
Suspect: elaborate history/tool-context fields. Investigation
ongoing via KIROXY_TAP body capture.

**Consequence**: the primary operator-facing endpoint does not
currently succeed against real Kiro accounts. Blocking for v1.0.1.

### 1.2 `/v1/chat/completions` + `/v1/models` (OpenAI-compat)

Status: **Shipped, not live-verified.**

- Routes: `internal/server/server.go:122-123`.
- Handler: `internal/server/openai.go`.
- Translation: `internal/openai/translate_request.go` (OpenAI →
  Anthropic) then reuses `messages.Service`.
- Tests: `internal/server/openai_test.go`, `internal/openai/*_test.go`
  — 4 test files.

Since OpenAI surface reuses `messages.Service`, it's affected by the
same upstream-403 regression as `/v1/messages`. Verified functional
at the translation layer; not verified against live Kiro.

### 1.3 Onboarder (`tools/onboard/`)

Status: **Works for individual Google accounts; Workspace dedupe
bug outstanding.**

- Sidecar Python, Camoufox anti-detect browser
- Mock-upstream integration tests pass (commit `b78009a`)
- Live Google-flow works (operator has used it to onboard their own
  account successfully — per OVERNIGHT_LOG.md entries)
- Known bug: Workspace-org accounts share a `profileArn` across
  members → vault dedupe collapses them. Tracked in BACKLOG as
  "Workspace profileArn collision", commit `7a60bad`.

Recommendation: document the Workspace limitation in OPERATIONS.md
("if you're in a Workspace org, use the member email as the `--label`
explicitly because the proxy can't distinguish you from your
teammates") rather than block v1.0.1.

### 1.4 Token refresh (proactive + reactive)

Status: **Works, with one scheduling miscalibration.**

Proactive:
- `internal/pool/refresh.go` runs on pool `Pick` when
  `expiresAt - skew < now`. Skew default 5 min
  (`pool/refresh.go:68`).
- Cited: `pool/pool.go:313-325`.

Reactive:
- 403 from upstream triggers refresh via `TokenRefresher` callback
  (`kiroclient/client.go:296-312`).
- Invalidates AuthManager cache, refreshes, retries once.

Tests: `refresh_concurrent_test.go`, `refresh_test.go`,
`refresh_e2e_test.go` (Phase 2.5.1 expired-token E2E added commit
`5ee3dbc`).

**Miscalibration**: BACKLOG P0 entry — `expires_at` observed 1h
beyond `added_at + expires_in` math. First import at 22:08 claimed
expiry at 00:09 (2h later) instead of 23:08 (1h). Root cause TBD.
Impact: proactive refresh fires too early or too late. Fix
requires investigation.

### 1.5 Dashboard

Status: **Live state displays.**

Three dashboards served:
- `/dashboard` (Phase H original) — `server/dashboard.go`
- `/dashboard-next` — experimental Svelte SPA (`server/next/`)
- `/dashboard-mansion` — operator-desk design (`server/mansion/`,
  committed `b632a3f`)

All loopback-only by default (auth.go:44-47 bypasses localhost).
Control actions (import/remove account) defined as
`DashboardControlProvider` interface but **not wired from main.go**
— effectively read-only today.

Recommendation: document as read-only in v1.0.1; wire write actions
in v1.1.

### 1.6 opencode integration

Status: **Untested live.**

- `kiroxy opencode-config` emits provider JSON
  (`cmd/kiroxy/opencode_config.go`)
- docs/OPENCODE.md covers hookup (6.7 KB)
- No automated smoke test against live opencode binary

Contingent on §1.1 fix; meanwhile the emitted config is correct and
kiroxy's OpenAI-compat surface is tested.

---

## 2. Operational readiness

### 2.1 Runbooks

| Runbook | Exists? | Where |
|---|---|---|
| First-time install | **Yes** | README.md quickstart |
| Troubleshooting by symptom | **Yes** | docs/TROUBLESHOOTING.md |
| Production deploy (homelab / cloud) | **No** | Gap — research-v4/OPERATIONS.md fills this |
| Backup & restore vault | **No** | Gap |
| Upgrade procedure | **No** | Implicit: compose `down && up -d --build` |
| Rollback procedure | **No** | Gap |
| Emergency: all accounts in cooldown | **No** | Gap |
| Emergency: upstream returning 502s | **Partial** | docs/TROUBLESHOOTING.md has error table |
| Migration to new host | **No** | Gap |

Recommendation: OPERATIONS.md closes the top-4 gaps before v1.0.1
tag.

### 2.2 Monitoring

| Signal | Instrumented? | Dashboard? |
|---|---|---|
| Request rate | Yes (`kiroxy_requests_total`) | Yes (METRICS.grafana.json) |
| Request latency p50/p95/p99 | Yes (`kiroxy_request_duration_seconds`) | Yes |
| Upstream TTFB | Yes (`kiroxy_upstream_ttfb_seconds`) | Yes |
| Error rate by kind | Yes (`kiroxy_request_errors_total{kind}`) | Yes |
| Accounts available / cooldown / failed | Yes (pool gauges) | Yes |
| Refresh attempt outcomes | Yes (`kiroxy_refresh_attempts_total`) | Yes |
| Cooldown events | Yes (`kiroxy_account_cooldowns_total`) | Yes |
| Token counts per request | Yes | Yes |
| Vault generation | Yes (`kiroxy_vault_generation`) | Yes |

Cardinality: `status` (5) × `model` (~7 canonical + `unknown`) ×
`stream` (2) = ~80 series. Safe.

### 2.3 Alert rules

**Not shipped.** METRICS.md does not include recording/alert rules
for Prometheus.

Suggested v1.0.1 alert rule set:
```yaml
# alerts.yml
- alert: KiroxyPoolDepleted
  expr: kiroxy_accounts_available == 0 and kiroxy_accounts_cooldown > 0
  for: 5m
  annotations:
    summary: All kiroxy accounts in cooldown — service degraded

- alert: KiroxyHighErrorRate
  expr: rate(kiroxy_request_errors_total[5m]) / rate(kiroxy_requests_total[5m]) > 0.1
  for: 10m
  annotations:
    summary: kiroxy error rate >10% for 10 minutes

- alert: KiroxyRefreshFailing
  expr: rate(kiroxy_refresh_attempts_total{result=~"fail_.*"}[5m]) > 0.01
  for: 10m
  annotations:
    summary: Token refresh failing — imminent auth outage
```

Recommendation: ship `docs/alerts.yml` with v1.0.1. 30 min of work.

### 2.4 Health endpoints

- `/healthz` — `server.go:105, 136`. Always 200 when HTTP server up.
- `/readyz` — `server.go:106`, `readiness.go:26`. Checks: `vault`
  (list round-trip), `pool` (non-zero account count).
  **Deliberately excludes upstream reachability** (main.go:317-319)
  — appropriate for personal-use; readiness shouldn't fail because
  Kiro is down.

Docker HEALTHCHECK wired to `kiroxy healthcheck` subcommand
(`Dockerfile:98`), which in-process probes `/healthz`. Compose
inherits (`docker-compose.yml:83`).

### 2.5 Graceful shutdown

- `KIROXY_SHUTDOWN_TIMEOUT=30s` default (config.go:93-99).
- Compose `stop_grace_period: 35s` — 5s buffer (docker-compose.yml:91).
- Process catches SIGTERM, flushes in-flight requests.

Not tested under load. Recommendation: smoke-test SIGTERM during a
long stream once.

---

## 3. Observability readiness

### 3.1 Structured logs

- JSON lines to stderr (`slog.NewJSONHandler`,
  `cmd/kiroxy/main.go:143-145`).
- Per-request log schema: `time, level, msg, request_id, method,
  path, status, latency_ms, bytes_out, remote_ip, user_agent`
  (`server/logging.go:54-63`).
- `/healthz` logs suppressed (logging.go:50-52).
- `request_id` = 26-char ULID (logging.go:37-43). X-Request-Id
  header passthrough.
- Log level via `KIROXY_LOG_LEVEL` env (config.go:47-58).

### 3.2 Distributed tracing

**Defined but dormant.**

`internal/tracing/` contains a full OTLP HTTP exporter
implementation with W3C propagators, but `tracing.Init` is NOT called
from `main.go`. Only `tracing.Tracer()` is referenced once
(`kiroclient/client.go:183`), which resolves to the global no-op
tracer without `Init`.

To activate: call `tracing.Init(...)` in `main.go` behind a
`KIROXY_OTEL_ENABLED=1` env. Env `OTEL_EXPORTER_OTLP_ENDPOINT`
already plumbed (tracing.go:26-30).

Recommendation: wire it in v1.0.1 gated on env. ~20 LoC.

### 3.3 Prometheus metrics

See §2.2. Exposed at `/metrics` with auth bypass for loopback +
`KIROXY_METRICS_PUBLIC=1` escape hatch
(`server/metrics.go:24`).

### 3.4 Audit trail

- Refresh attempts logged via `kiroxy_refresh_attempts_total` (tracked
  count) and via slog events ("token refreshed", "token refresh
  failed").
- No dedicated audit log file. Personal-use operator runs a single
  binary; stderr log is the audit trail.

Recommendation: document "grep 'refresh' proc.log for audit" in
OPERATIONS.md.

---

## 4. Security readiness

### 4.1 At-rest token storage

Status: **Plaintext SQLite.**

- Path: `~/.kiroxy/tokens.db` (config.go:82-90), or `/data/tokens.db`
  in Docker (Dockerfile:89).
- Directory perms: 0700 (main.go:172-181).
- File perms: 0600 (accounts.go:138-148).
- **No encryption** of access_token / refresh_token values.

Risk: any process running as the operator's UID can read the tokens.
Mitigation: filesystem-level encryption (FileVault, LUKS) expected
of self-hoster. Explicit in README.

Recommendation: v1.1 add optional encryption with user-supplied
passphrase via `--vault-password` flag + PBKDF2. Not a v1.0.1
blocker.

### 4.2 Inbound auth

- `KIROXY_API_KEY` env, SHA-256 constant-time compare
  (`server/auth.go:62-63`).
- Header accepted: `X-Api-Key: <key>` or
  `Authorization: Bearer <key>` (auth.go:71-81).
- Empty key = auth disabled; loopback bind is the only security
  boundary.
- Missing key → 401 `application/problem+json`
  (auth.go:83-93).

Loopback bypass at `auth.go:44-47` — `127.0.0.1` / `::1` requests
skip auth check entirely.

**Hardening recommendation for v1.0.1**: enforce a minimum key
length when set (e.g. reject keys <16 chars with a startup warning).

### 4.3 Outbound TLS

- `net/http.DefaultTransport.Clone()` — system CA bundle
  (client.go:126-131). No cert pinning.
- Connection reuse: `MaxIdleConns=100`, `IdleConnTimeout=90s`
  (client.go:127-129).
- Response header timeout: 30s (client.go:130).

Kiroxy does NOT pin TLS to `*.kiro.dev` or `*.amazonaws.com`. A
compromised root CA in the OS trust store could MITM. Matches
industry default for this scale of tool.

### 4.4 Input validation

- Request body limit: 4 MiB via `http.MaxBytesReader`
  (messages/request.go:75).
- Header sanitization: none explicit (X-Forwarded-For merely
  read for logging).
- JSON schema validation on tool_use inputs: partial
  (`reqconv/schema_sanitize.go` strips unsupported JSON Schema
  keywords).
- Prompt-injection defense: none — it's a pass-through proxy;
  injection is the downstream client's responsibility.

### 4.5 Log redaction

- `server/logging.go` DOES NOT log headers, bodies, or auth tokens
  — only method/path/status/latency/remote_ip/user_agent.
- `internal/logging.SafeHeaders` utility exists but is **not invoked**
  by the runtime middleware. It's a trap for future edits where
  someone enables header logging without routing through the
  redactor.

Recommendation: write a test that fails if kiroxy ever logs raw
`Authorization` values. Pin redaction discipline.

### 4.6 Dependency audit

- `make vuln` runs govulncheck (Makefile:98-106).
- Daily scheduled CI: `.github/workflows/vuln.yml` — fails build on
  reachable CVE, opens GitHub issue.
- Known clean as of latest CI run (verify with `make vuln` locally
  at release time).

### 4.7 Supply chain

- Binary built from source in CI (no prebuilt toolchain caveats).
- Goreleaser emits SHA-256 checksums file.
- **No cosign / GPG signatures** — checksums are integrity only, not
  authenticity.

Recommendation: v1.1 add cosign keyless signatures. Documentation
change at minimum: README should say "verify via checksums file
only, we do not currently sign releases".

### 4.8 Config surface hardening

- `KIROXY_BIND` defaults to loopback. Docker overrides to `0.0.0.0`
  but relies on container netns + compose's loopback port mapping
  for safety. Correct.
- `KIROXY_METRICS_PUBLIC=1` is an explicit opt-out of the loopback
  bypass for `/metrics`. Documented in config.go:24.
- `KIROXY_USE_LEGACY_ENDPOINT=1` is undocumented (only in
  kiroclient/client.go:171 comment). Recommend: add to README env
  table with explicit deprecation deadline (2026-08-15).

---

## 5. Documentation readiness

| Doc | Status | Notes |
|---|---|---|
| README.md | ✓ | 465 lines; quickstart covers 5-min install |
| CHANGELOG.md | ✓ | Keep-a-Changelog; `Unreleased` up to date |
| docs/ARCHITECTURE.md | ✓ | 18 KB; covers major flows |
| docs/TROUBLESHOOTING.md | ✓ | 10 KB; error table + diagnostic flow |
| docs/METRICS.md | ✓ | Scrape config + catalog |
| docs/METRICS.grafana.json | ✓ | Importable dashboard |
| docs/OPENAI.md | ✓ | OpenAI-compat surface |
| docs/OPENCODE.md | ✓ | opencode.ai hookup |
| docs/BENCHMARKS.md | ✓ | Load-test numbers + caveats |
| docs/DASHBOARD_NEXT.md | ✓ | Design rationale for dashboard-next |
| **docs/OPERATIONS.md** | ✗ **GAP** | research-v4/OPERATIONS.md fills this |
| **docs/SECURITY.md** | ✗ **GAP** | research-v4/SECURITY.md fills this |
| **docs/BACKUP.md** | ✗ **GAP** | Recommend v1.0.1 |
| **docs/UPGRADE.md** | ✗ **GAP** | Recommend v1.0.1 |
| SMOKE_TEST.md | ⚠ | v0.2.1-patch FAIL report. **Re-run for v1.0.1 or banner** |
| BLOCKED.md | ✓ | "NOT BLOCKED" — accurate |
| BACKLOG.md | ✓ | P0/P1 items triaged |
| BUILD_LOG.md | ✓ | Phase-by-phase engineering log (internal) |
| OVERNIGHT_LOG.md | ✓ | Autonomous run log (internal) |

### Critical doc issues

1. **SMOKE_TEST.md reports a historical FAIL that has since been
   fixed.** A first-time reader lands on the FAIL verdict and
   assumes kiroxy is broken. Fix: either (a) delete the file and
   re-run a fresh smoke test for v1.0.1, or (b) prepend a
   "SUPERSEDED — see commit X" banner.

2. **No OPERATIONS.md** means the README's three-line TLS guide is
   the only production deployment reference. research-v4/OPERATIONS.md
   is a research doc and should graduate to `docs/OPERATIONS.md` once
   trimmed.

3. **No SECURITY.md** means the security posture in §4 is only
   visible by reading the code. research-v4/SECURITY.md graduates to
   `docs/SECURITY.md`.

---

## 6. Test coverage readiness

Detailed per-package table in the ops-surface inventory (research-v4
context).

**High-coverage packages** (multiple test files, race-safe):
- `internal/pool` — 3 src, 4 test
- `internal/reqconv` — 13 src, 9 test
- `internal/server` — 8 src, 10 test
- `internal/tokenvault` — 2 src, 3 test
- `internal/tracing` — 5 src, 5 test (but runtime-dormant)

**Gaps**:
- `internal/config` — 1 src, **0 test**. Env parsing is safety-critical.
  Recommendation: add a regression table before v1.0.1.
- `cmd/kiroxy/*` — 8 src, **1 test** (only `import_test.go`).
  `add-account`, `debug-refresh`, `healthcheck`, `dashboard` subcommands
  rely on manual smoke-testing. Ship as-is for v1.0.1; target
  contributors for v1.1.
- `internal/messages/toolsearch.go` — no unit tests.
- `internal/respconv` — 12 src, 5 test. Weak coverage on edge-case
  events (thinking, cache hits).

CI runs `make gate` + `-race` on every PR (ci.yml:57-74).

---

## 7. Ship-to-friends checklist

Run through this before saying "my friend can install and use this
today". Every unchecked item is a release blocker.

### 7.1 Install path works

- [ ] `curl -L ... | sh` (or `brew install ...`, or `go install`) completes
- [ ] `kiroxy version` prints expected tag
- [ ] Docker image pulls from registry (if published)
- [ ] `docker compose up -d` completes and container is healthy
- [ ] `kiroxy --help` shows all subcommands

### 7.2 First-time setup

- [ ] `kiroxy add-account --label=me` works (either via Builder ID
      device-code OR via `import-accounts-json` from Desktop tokens)
- [ ] `kiroxy list-accounts` shows the imported account
- [ ] `kiroxy status` reports vault + pool healthy

### 7.3 First real request

- [ ] **BLOCKED**: `curl -X POST http://127.0.0.1:8787/v1/messages ...`
      returns a valid response (upstream-403 regression must be
      fixed)
- [ ] `curl -X POST http://127.0.0.1:8787/v1/chat/completions ...`
      returns a valid response
- [ ] Second request after 30 minutes still works (proactive refresh
      validated)
- [ ] Fourth request after deliberate access_token invalidation
      triggers reactive refresh and succeeds

### 7.4 Integration

- [ ] opencode picks up kiro/* models via emitted config
- [ ] claude-code works with `ANTHROPIC_BASE_URL=http://127.0.0.1:8787`
      + kiroxy's API key
- [ ] At least one other downstream client (aider, Cline, Continue)
      smoke-tested

### 7.5 Observability

- [ ] `/healthz` returns 200
- [ ] `/readyz` returns 200 when pool has accounts
- [ ] `/metrics` returns text/plain with 10+ metric families
- [ ] Dashboard renders at `/dashboard` with live account state
- [ ] Grafana dashboard imports METRICS.grafana.json cleanly

### 7.6 Hardening

- [ ] Firewall allows `*.kiro.dev:443` + `oidc.*.amazonaws.com:443`
      outbound only
- [ ] Vault file is mode 0600
- [ ] Vault directory is mode 0700
- [ ] No secrets leak in 30 seconds of trace logs:
      `kiroxy serve 2>&1 | head -300 | grep -iE 'aor|aoa|bearer ' | wc -l`
      should be 0
- [ ] `make vuln` passes (no reachable CVE)

### 7.7 Emergency preparedness

- [ ] Friend knows how to rotate `KIROXY_API_KEY`
- [ ] Friend knows how to add another account when pool depletes
- [ ] Friend knows how to check `docs/TROUBLESHOOTING.md`
- [ ] Friend knows what version they installed (so they can report
      bugs with context)

### 7.8 Documentation

- [ ] README.md updated to current tag
- [ ] CHANGELOG.md has a dated v1.0.1 entry
- [ ] SMOKE_TEST.md either updated or banner'd as superseded
- [ ] At least a stub docs/OPERATIONS.md exists
- [ ] BLOCKED.md says "NOT BLOCKED"

---

## 8. Gate-by-gate summary

| Gate | Status |
|---|---|
| Build passes | ✓ (CI `ci.yml`) |
| Tests pass | ✓ (`make gate` green on main) |
| Lint / fmt | ✓ (`make fmt` + `vet`) |
| Race detector | ✓ (CI runs `-race -timeout 120s`) |
| Vulnerability scan | ✓ (daily `vuln.yml`) |
| Smoke test against live Kiro | ✗ **BLOCKED** (upstream-403 regression) |
| Docs alignment | ⚠ (gaps listed §5) |
| Ship-to-friends | ✗ (§7 has unchecked items) |

---

## 9. Recommended path to v1.0.1 GA

Ordered by blast radius:

1. **Fix upstream-403 regression** — unblocks §1.1 + §7.3. Top
   priority. LoC estimate: 30-100 (per BACKLOG note).
2. **Fix `expires_at` miscalibration** — unblocks §1.4. LoC estimate:
   10-30.
3. **Ship `docs/alerts.yml`** — closes §2.3 gap. 30 min.
4. **Re-run SMOKE_TEST.md or prepend superseded banner** — closes
   §5 #1. 1-2 hours.
5. **Graduate research-v4/OPERATIONS.md → docs/OPERATIONS.md** —
   closes §5 #2. Trim + re-home. 1 hour.
6. **Graduate research-v4/SECURITY.md → docs/SECURITY.md** —
   closes §5 #3. 1 hour.
7. **Wire OTel** — activates §3.2. ~20 LoC + gate env. 1 hour.
8. **Add tests for `internal/config`** — closes §6 gap. 2 hours.

Total: ~1-2 engineer-days past the upstream-403 fix.

---

*Every line in this file is verifiable against kiroxy source as of
2026-05-13. Update this document each time a checked item changes
state.*
