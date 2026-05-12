# Changelog

All notable changes to kiroxy will be documented in this file. Format loosely follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); this project follows semver from v0.1.0 onwards.

## [Unreleased]

### Added (Phase M — Prometheus metrics endpoint)

- `GET /metrics` exposition endpoint in the Prometheus text format. Turn
  kiroxy into a first-class Prometheus target with zero config on
  localhost; on non-loopback the endpoint requires `KIROXY_API_KEY` or
  explicit opt-in via `KIROXY_METRICS_PUBLIC=1`.
- `internal/metrics/` package with a single process-wide `Registry` and
  a nil-safe `*Sink` call-site handle. Adds
  `github.com/prometheus/client_golang v1.23.2` as the only new
  dependency. Standard Go + process collectors are registered out of
  the box (goroutines, heap, GC, FDs).
- Request lifecycle counters in `messages.Service`:
  `kiroxy_requests_total` (labels: model, status class, stream),
  `kiroxy_request_errors_total` (kind=upstream|auth|proxy|invalid_request),
  `kiroxy_request_duration_seconds` (histogram by model + stream),
  `kiroxy_upstream_ttfb_seconds` (pre-body first-byte timing, emitted
  once even across the invalid-state retry path),
  `kiroxy_tokens_input` / `kiroxy_tokens_output` (histograms).
- Pool health gauges via `pool.RegisterPoolGauges`:
  `kiroxy_accounts_available`, `kiroxy_accounts_cooldown`,
  `kiroxy_accounts_failed`. Snapshots happen on Prometheus scrape (no
  background poller). Plus `kiroxy_account_cooldowns_total{reason}`
  counter on transition into cooldown.
- `kiroxy_refresh_attempts_total{kind,result}` counter in the pool
  refresh path, covering proactive vs reactive triggers and
  success/fail_401/fail_transient/fail_other outcomes.
- `kiroxy_vault_generation` gauge: sum of bundle generations, exposed
  via `tokenvault.RegisterVaultGauges`. Monotonic-ish proxy for refresh
  activity.
- `kiroxy_uptime_seconds` gauge.
- `docs/METRICS.md` — scrape setup, full catalog with cardinality
  bounds, useful PromQL queries, security notes.
- `docs/METRICS.grafana.json` — starter dashboard (12 panels: request
  rate, error rate, typed errors, latency percentiles, TTFB, token
  histograms, pool stacked, refresh attempts, cooldown reasons,
  generation gauge, uptime).

### Added (Phase J — OpenAI-compatible API surface)

- `POST /v1/chat/completions` endpoint: streaming + non-streaming
  translation shim over the existing Anthropic `/v1/messages` pipeline.
  OpenAI SDK clients (Cursor, Continue, Cline, aider, raw `openai-python`)
  work against kiroxy with a one-line base-URL change. Auth reuses the
  existing Bearer / X-Api-Key. Thinking blocks are dropped (no OpenAI
  equivalent). Tool calls translate both directions: OpenAI
  `tools[].function` -> Anthropic `tools`, Anthropic `tool_use` blocks ->
  OpenAI `tool_calls`. Tool-role OpenAI messages fold into
  `tool_result` blocks on a synthesized user message (consecutive tool
  results coalesce into one message). Data-URI images (base64) pass
  through; https:// URLs are rejected (upstream limitation).
- `GET /v1/models` endpoint: lists all Kiro models plus the OpenAI
  alias table, stable sorted, no duplicates.
- Model alias resolver: `gpt-4o`, `gpt-4o-mini` -> `claude-sonnet-4-6`;
  `gpt-4-turbo`, `gpt-4`, `o1` -> `claude-opus-4-7`; `gpt-3.5-turbo` ->
  `claude-haiku-4.5`. `openai/<x>` prefix stripping. Unknown names pass
  through (`claude-*` IDs already work natively). Response `model`
  field echoes the caller's original alias verbatim.
- OpenAI error envelope shape (`{error:{message,type,param,code}}`)
  mapped from Anthropic error responses; status-to-type mapping
  (`400->invalid_request_error`, `401/403->authentication_error`,
  `5xx->api_error`).
- Streaming translation: Anthropic SSE events -> OpenAI
  `chat.completion.chunk` frames with terminal `data: [DONE]`. Usage
  stats included in the final chunk unconditionally. `X-Accel-Buffering:
  no` set for nginx/proxy transparency.
- `docs/OPENAI.md` integration guide (Cursor, Continue, Cline, aider,
  `openai-python` snippets + alias table + feature support matrix).
- `internal/openai` package (types, translators, helpers, alias
  resolver) with 43 unit tests + 10 handler integration tests.
- Not implemented in v1.0 (documented as v1.1 follow-ups):
  `tool_choice: "required"` / specific-function-name choice,
  `response_format: json_object`, `stream_options.include_usage`
  opt-in, `/v1/responses`, `/v1/completions`, `/v1/embeddings`.

### Added (Phase 2.5 — pool-mode token refresh)

- Proactive token refresh for social-auth accounts in the pool path.
  Before each request, `TokenGetter.GetToken` checks `metadata.expires_at`
  and rotates the refresh_token against `prod.us-east-1.auth.desktop.kiro.dev/refreshToken`
  when within 5 minutes of expiry. Persists new tokens + `expires_at` via
  `vault.CommitWithMetaPatch` (preserves profile_arn / auth_method).
  Concurrent requests are coalesced via `golang.org/x/sync/singleflight`;
  cross-process via vault generation-lock.
- Reactive refresh via `kiroclient.WithTokenRefresher` installed on the
  pool path for 403 retry edge cases.
- `internal/auth.RefreshSocial` standalone helper with typed errors
  (`ErrRefreshUnauthorized`, `ErrRefreshTransient`, `ErrRefreshMalformed`).
- `vault.CommitWithMetaPatch` for metadata-preserving commits during
  refresh.

### Known limitations (Phase 2.5)

- Automated test coverage ships narrower than the plan spec: only
  `needsRefresh` boundary + `parseAccountMetadata` tolerance unit tests
  landed. Singleflight / 401-cooldown / 5xx-retry / vault-write soft-fail
  paths are implemented but untested; tracked as BACKLOG P1
  "Expand Phase 2.5 refresh test coverage".
- Non-social accounts (Builder ID / kiro-cli SQLite) are intentionally
  excluded from the refresh path. Builder ID Phase C.1 blocker unchanged.


## [0.3.0] — 2026-05-12

Phase D + Phase F + Phase G.0/G.1 landed on top of v0.2.2. Proxy is
end-to-end validated (v0.2.2), ships as a Docker container (D), speaks
opencode's config format (F), and has a Python sidecar that automates
Kiro Desktop OAuth onboarding (G).

### Added
- **Docker deployment path (Phase D).** Multi-stage `Dockerfile`
  (`gcr.io/distroless/static-debian12:nonroot` runtime, ~30 MiB) +
  hardened `docker-compose.yml` (read-only root FS, all caps dropped,
  `no-new-privileges`, named volume at `/data/tokens.db`). Pins
  `GOEXPERIMENT=jsonv2` in the build stage and injects version via
  `-ldflags -X main.version=$VERSION`. `-trimpath -s -w` for
  reproducible, stripped output.
- **`kiroxy healthcheck` subcommand.** In-binary `/healthz` probe used
  by the container `HEALTHCHECK` directive; distroless has no
  shell/curl, so the image re-executes itself.
- **Makefile targets** `docker-build`, `docker-run`, `docker-compose-up`,
  `docker-compose-down`, `docker-clean`. Each target exits cleanly with
  a readable error when `docker` is not on PATH.
- **`.dockerignore`** excluding `.env*`, `*.db*`, `refresh_tokens.txt`,
  `kiro_tokens.json`, and VCS/build artefacts from the build context.
- **README "Run with Docker"** section covering quickstart, manual
  `docker run`, security posture, and the `KIROXY_BIND=0.0.0.0` /
  volume gotchas.
- **`kiroxy opencode-config` subcommand (Phase F).** Emits a JSON
  snippet the operator pastes into `~/.config/opencode/opencode.json`.
  Flags: `-base-url`, `-api-key`, `-provider-name`, `-models` filter,
  `-output` file. Emits only the 7 resolver-verified Claude model IDs
  (`claude-opus-4-7`, `claude-opus-4-6`, `claude-opus-4.5`,
  `claude-sonnet-4-6`, `claude-sonnet-4-6[1m]`, `claude-sonnet-4.5`,
  `claude-haiku-4.5`). Unknown `kiro/*` labels from the Pro tier UI
  are excluded because the resolver silently rewrites them to
  `claude-sonnet-4-6`.
- **`docs/OPENCODE.md`** — setup guide, JSON snippet example,
  display-label → API-ID mapping table, troubleshooting, multi-account
  pool note, silent-fallback caveat.
- **`tools/onboard/` Python sidecar (Phase G.0 + G.1).** Full-auto
  Kiro Desktop OAuth acquisition. Orchestrates PKCE → login URL →
  Camoufox browser drive → callback capture → token exchange →
  output JSON (matches `kiroxy import-accounts-json` schema).
  G.1 single-account flow with humanized typing, 100-profile
  rotation (adapted from kikirro), stdlib-only PKCE unit test.
  External tool by design — kiroxy Go binary does not ship Python
  or Camoufox.

### Changed
- Nothing. Phase D/F/G are purely additive on top of v0.2.2.

### Fixed
- Nothing. No regressions found.

### Known gaps (see `BACKLOG.md`)
- P1: pool-mode token refresher not wired for
  `source="import-accounts-json"` accounts; imports stop working after
  `expires_in` seconds (~1h) until this lands.
- P2-P3: Phase G.2–G.5 (credential encryption, batch mode, retry
  logic, polish UI) deferred.

## [0.2.2] — 2026-05-12

First end-to-end working proxy. See `v0.2.2` tag annotation and
`OVERNIGHT_LOG.md` Phase C.2b entry for full detail. Highlights:

### Added
- `kiroxy import-accounts-json` subcommand for Desktop-flow tokens.
- `kiroxy debug-refresh` admin/diagnostic tool.
- `pool.TokenGetter` now threads `profile_arn` from vault metadata
  into `auth.Credentials`, closing the gap where profileArn was stored
  but not surfaced.
- `internal/kiroclient.chooseAmzTarget` — switches to AmazonQ target
  when `ProfileARN` empty (Builder ID path), CodeWhisperer target
  otherwise (Desktop-flow path).

### Validated
- `/v1/messages` non-streaming: HTTP 200, valid Anthropic response.
- `/v1/messages` streaming: HTTP 200, 7 correct SSE events.

## [0.2.1-patch] — 2026-05-12

CLI ergonomics fixes. See OVERNIGHT_LOG Phase C-PREP entry.

### Fixed
- Vault `Open()` now auto-creates parent directory at 0700.
- `kiroxy --version`, `-v`, `--help`, `-h` work at top level.
- Subcommand `--help` prints subcommand-specific usage and exits 0.
- `VERSION` wired from `git describe --tags --always --dirty`.

## [0.2.0] — 2026-05-12

### Added
- **AWS Builder ID device-code OAuth** inside `kiroxy add-account`.
- Token refresh + rotation.

### Known limitation (see `BLOCKED.md`)
- Builder ID Free-tier accounts lack CodeWhisperer scopes; upstream
  rejects `/v1/messages`. Retained in code but superseded by the
  Desktop-flow JSON import path in v0.2.2.

## [0.1.1] — 2026-05-12

### Added
- `kiroxy import-accounts` subcommand (line-delimited triplet format).
- Vault `metadata` column for opaque per-account data.

### Note
- The triplet format (`email:refresh_token:signature`) was designed
  for the kikirro extractor output. Later analysis (Phase C.2) showed
  kikirro emits 2-field `email:refresh_token` pairs where the colon
  inside the cookie value fooled the parser into splitting on 3 fields.
  Triplet-format accounts imported via this command require tokens
  scoped to Kiro Desktop; kikirro Web Portal tokens do not work.
  Users should prefer `import-accounts-json` (v0.2.2+) with Desktop-flow
  tokens from the `tools/onboard/` sidecar.

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
- OpenAI-compatible `/v1/chat/completions` surface.
- Prometheus / OTel exporters (wiring already in `internal/tracing/`).
