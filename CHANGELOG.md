# Changelog

All notable changes to kiroxy will be documented in this file. Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); this project uses semver from v0.1.0 onwards.

## [Unreleased]

### Added
- OIDC rotation detection — pool emits error when 3+ accounts hit 401 within 5 minutes
- API key min-length warning at startup (< 16 chars)
- Latency recorder interface for pool EWMA health tracking
- Stickiness in dashboard state API

### Improved
- Test coverage: `internal/messages` 37→77%, `cmd/kiroxy` 24→50%, `internal/anthropic` 48→98%

---

## [v1.4.0] — 2026-05-15

Public release. Frontend polish complete, all 7 dashboard views production-grade.

### Added
- **Mansion dashboard** — 7 polished views: Live Stream, Metrics, Pool, Logs, Settings, Tools, Models
- Pool health minimap — 8×10 cell density visualization
- Motion budget policy (4 ambient animation slots max)
- Landing page at `web/landing/` (Astro, mobile-responsive)
- Worker panic-protection via `internal/safego/` package
- Prometheus metrics endpoint (`GET /metrics`) with 12 counters/histograms/gauges
- OpenAI-compatible API (`POST /v1/chat/completions`, `GET /v1/models`)
- Pool-mode proactive token refresh (5-min-before-expiry, singleflight-coalesced)
- GetUsageLimits polling — per-account credit visibility, weighted selection
- Throttle stampede prevention — capacity shortage vs quota exhaustion classification

### Improved
- WCAG AA contrast in both light and dark schemes
- Mobile responsiveness across all dashboard views
- HTTP/2 error classification (typed errors vs substring matching)
- Cooldown transition logic (no double-fire during 429 storms)
- `count_tokens` accuracy (Anthropic-shaped representation, not Kiro wire payload)
- Retry-After header support on 429/5xx (integer-seconds + HTTP-date)
- SSE keepalive ping every 15s (prevents proxy idle timeout)
- Vault permissions tightened to 0600 on WAL + SHM files

### Fixed
- Upstream 403 loop with fresh credentials (model resolver + expires_at miscalc)
- Idle reader data race on stream close
- Pool singleflight for concurrent refresh (was declared but never invoked)
- Schema sanitization for `anyOf`/`oneOf` with multiple non-null branches

### Changed
- Module path rebranded to `github.com/nopperabbo/kiroxy`

---

## [0.3.0] — 2026-05-12

### Added
- Docker deployment — multi-stage Dockerfile, distroless runtime (~30 MiB)
- `kiroxy healthcheck` subcommand (in-binary probe for distroless containers)
- `kiroxy opencode-config` subcommand (emit opencode.json provider snippet)
- `tools/onboard/` Python sidecar for automated Kiro Desktop OAuth acquisition
- Makefile targets: `docker-build`, `docker-run`, `docker-compose-up/down`

---

## [0.2.2] — 2026-05-12

First end-to-end working proxy.

### Added
- `kiroxy import-accounts-json` subcommand for Desktop-flow tokens
- `kiroxy debug-refresh` admin/diagnostic tool
- Profile ARN threading from vault metadata into auth credentials

### Validated
- `/v1/messages` non-streaming: HTTP 200, valid Anthropic response
- `/v1/messages` streaming: HTTP 200, correct SSE events

---

## [0.2.1-patch] — 2026-05-12

### Fixed
- Vault `Open()` auto-creates parent directory at 0700
- `kiroxy --version`, `-v`, `--help`, `-h` work at top level
- Subcommand `--help` prints subcommand-specific usage

---

## [0.2.0] — 2026-05-12

### Added
- AWS Builder ID device-code OAuth in `kiroxy add-account`
- Token refresh + rotation

---

## [0.1.1] — 2026-05-12

### Added
- `kiroxy import-accounts` subcommand (line-delimited triplet format)
- Vault `metadata` column for opaque per-account data

---

## [0.1.0-mvp] — 2026-05-11

First cut of kiroxy.

### Added
- `./kiroxy serve` HTTP server (default 127.0.0.1:8787)
- `POST /v1/messages` — Anthropic Messages API (streaming + non-streaming)
- `POST /v1/messages/count_tokens` via tiktoken
- `GET /healthz` liveness, `GET /readyz` readiness
- `GET /dashboard` HTML dashboard with live account stats
- Inbound auth via `X-Api-Key` or `Authorization: Bearer` (constant-time compare)
- Multi-account pool with LRU selection, cooldowns, circuit breaker
- SQLite-backed token vault with generation-locked OAuth refresh
- CLI: `add-account`, `list-accounts`, `remove-account`, `status`, `version`
- Structured JSON logs with per-request ULID
- Graceful 30s SIGTERM shutdown

### Attribution
- Derived from `d-kuro/kirocc` (Apache-2.0), `Quorinex/Kiro-Go` (MIT), `kadangkesel/hexos` (MIT)
- Full per-file attribution in [`NOTICE`](NOTICE)
