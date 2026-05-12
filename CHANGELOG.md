# Changelog

All notable changes to kiroxy will be documented in this file. Format loosely follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); this project follows semver from v0.1.0 onwards.

## [Unreleased]

### Added
- **Docker deployment path (Phase D).** Multi-stage `Dockerfile` (distroless-nonroot runtime, ~30 MiB) + hardened `docker-compose.yml` (read-only root FS, all caps dropped, `no-new-privileges`, named volume at `/data/tokens.db`). Pins `GOEXPERIMENT=jsonv2` in the build stage and injects version via `-ldflags -X main.version=$VERSION`.
- **`kiroxy healthcheck` subcommand.** In-binary `/healthz` probe used by the container `HEALTHCHECK` directive; distroless has no shell/curl, so the image re-executes itself.
- **Makefile targets** `docker-build`, `docker-run`, `docker-compose-up`, `docker-compose-down`, `docker-clean`. Each target exits cleanly with a readable error when `docker` is not on PATH.
- **`.dockerignore`** excluding `.env*`, `*.db*`, `refresh_tokens.txt`, `kiro_tokens.json`, and VCS/build artefacts from the build context.
- **README "Run with Docker"** section covering quickstart, manual `docker run`, security posture, and the `KIROXY_BIND=0.0.0.0` / volume gotchas.

## [0.1.0-mvp] — 2026-05-11

First cut of kiroxy. MIT, personal-use, self-hosted Kiro-to-Anthropic proxy.

### Added
- `./kiroxy serve` HTTP server (default 127.0.0.1:8787).
- `POST /v1/messages` Anthropic Messages API (streaming + non-streaming, tool calls, vision, thinking, tool_search).
- `POST /v1/messages/count_tokens` via tiktoken.
- `GET /healthz` liveness.
- `GET /readyz` readiness with per-dep subchecks (vault + pool).
- `GET /dashboard` HTML dashboard with live account stats; loopback bypasses auth, remote sources still need `KIROXY_API_KEY`.
- `GET /dashboard/api/state` JSON snapshot for the dashboard's fetch loop.
- Inbound auth via `X-Api-Key` or `Authorization: Bearer` with SHA-256 constant-time compare.
- Multi-account pool with LRU selection, per-account cooldown, and 3-strikes circuit breaker.
- SQLite-backed token vault with generation-locked OAuth refresh (50-goroutine race-safe).
- CLI subcommands `add-account`, `list-accounts`, `remove-account`, `status`, `version`, `help`.
- Structured JSON logs with per-request ULID; `X-Request-Id` in + out.
- Graceful 30s SIGTERM shutdown that flushes SSE streams and closes the vault.
- Makefile with `make build / gate / test / test-race` (pins `GOEXPERIMENT=jsonv2`).

### Attribution
- Derived from `d-kuro/kirocc` @ `5633c47f` (Apache-2.0): most of `internal/*`.
- Derived from `Quorinex/Kiro-Go` @ `940dc782` (MIT): `internal/pool/pool.go` (selection policy swapped to LRU).
- Derived from `kadangkesel/hexos` @ `d4c0d1ce` (MIT): `internal/tokenvault/vault.go` (TS → Go port, generation-lock preserved).
- Full per-file attribution + `NOTICE`.

### Known follow-ups (see `BACKLOG.md`)
- AWS Builder ID device-code flow inside `kiroxy add-account`.
- OpenAI-compatible `/v1/chat/completions` surface.
- Prometheus / OTel exporters (wiring already in `internal/tracing/`).
