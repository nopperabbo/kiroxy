# Changelog

All notable changes to kiroxy will be documented in this file. Format loosely follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); this project follows semver from v0.1.0 onwards.

## [Unreleased]

### In progress
- Building v0.1.0-mvp per `../BUILD_PLAN.md`.

## [0.1.0-mvp] — TBD

MVP milestones M1..M10 — see commit history and BUILD_LOG.md for per-milestone detail.

### Added (milestones M1–M7, complete)
- `./kiroxy serve` HTTP server binding to 127.0.0.1:8787 by default.
- `POST /v1/messages` Anthropic Messages API compatibility (streaming + non-streaming).
- `POST /v1/messages/count_tokens` tiktoken-based count endpoint.
- `GET /healthz` liveness probe (bypasses auth).
- `GET /readyz` readiness probe with per-dependency subchecks (vault + pool).
- Inbound auth via `X-Api-Key` or `Authorization: Bearer` with constant-time compare.
- Multi-account pool with LRU selection, per-account cooldown + circuit breaker.
- SQLite-backed token vault with generation-locked OAuth refresh (50-goroutine safety test).
- Structured JSON logs with per-request ULID, echoed via `X-Request-Id` header.
- Graceful 30s SIGTERM shutdown that flushes SSE streams and closes the vault.
- Makefile with `make build / gate / test / test-race` (pins `GOEXPERIMENT=jsonv2`).

### In progress (milestones M8–M10)
- M8: this README + quickstart.
- M9: `kiroxy add-account / list-accounts / remove-account / status` subcommands.
- M10: minimal HTML dashboard with localhost-bypass auth.

