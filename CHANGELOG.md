# Changelog

All notable changes to kiroxy will be documented in this file. Format loosely follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); this project follows semver from v0.1.0 onwards.

## [Unreleased]

### Added (Phase 5 — Worker panic-protection + log polish, 2026-05-14)

Closes the panic-recovery gap left over from Phase 1. The HTTP recover
middleware (Phase 1) only catches panics in handler goroutines; long-lived
worker goroutines spawned at startup were unprotected and would die
silently on any panic, leaving the system in a degraded state with no
visible signal.

- **`internal/safego/`** (NEW package): `safego.Go(name, fn)` and
  `safego.Run(name, fn)` wrap goroutine spawn with deferred recover();
  on panic, emits ERROR slog event with worker name, recovered value,
  and full `runtime/debug.Stack()`. Optional `SetOnPanic(hook)` for
  metric instrumentation. Hook itself is panic-guarded so a buggy
  callback cannot turn recovery into a process kill.
- **Adopted in 4 worker goroutines** (idle_reader.go intentionally
  skipped — too hot path, low panic risk in net I/O):
  - `internal/respconv/streaming.go` — SSE keepalive emitter
    (`respconv-sse-keepalive`)
  - `internal/pool/stickiness.go` — Session pin pruner
    (`pool-stickiness-pruner`)
  - `internal/pool/usage.go` — Usage poller loop
    (`pool-usage-poller`)
  - `internal/server/openai.go` — OpenAI translator goroutine
    (`openai-translator`) — most critical: parent blocks on
    `<-done`, a silent panic would deadlock forever
  - `cmd/kiroxy/main.go` — Shutdown signal goroutine
    (`main-shutdown`) — guards graceful-shutdown sequencing
- **`internal/safego/safego_test.go`**: 5 unit tests covering recovery,
  cleanup-after-panic invariant for `Run`, hook invocation, hook nil
  disable, and hook-panic-doesn't-propagate.

### Changed (Phase 5 — Log polish)

- **`internal/messages/errors.go`**: `kiro api error` log level is now
  WARN for 4xx upstream classifications (UnknownOperationException,
  ValidationException, etc) and ERROR only for 5xx and non-HTTP
  transport failures. Eliminates spurious ERROR-level noise from
  upstream-side issues that are not actionable. Also exposes
  `UpstreamError.Reason` (Phase 3.1 field) as a top-level log
  attribute when present.
- **`internal/server/logging.go`**: `loggingResponseWriter` migrated
  from `int` + `sync.Mutex` to `atomic.Int32` + `atomic.Int64` for
  status and bytes_out counters. Closes the (pre-existing, low-risk)
  data race noted in audit M2 between `Write()` increments and the
  log-line read at handler return. Eliminates the muStatus mutex.
- **`internal/server/server.go` + `internal/server/models.go`**:
  Replaced `_ = json.NewEncoder(w).Encode(resp)` with explicit
  `slog.Debug("... encode failed", err)` — silent encode errors
  during response write (typically: client disconnect mid-stream)
  now surface in debug logs rather than vanishing.

### Added (Phase 4 — Test coverage + OTel runtime, 2026-05-14)

Closes three medium-priority audit items from BACKLOG.md (config tests,
OTel wiring, nextDateReset test gap).

- **`internal/kiroclient/usage_test.go`**: 12 sub-tests for the
  `nextDateReset` parser. Locks in the wire-format contract Phase 2.10
  fixed (integer seconds AND scientific notation E-notation) plus the
  ms-vs-s heuristic (>10B value treated as `UnixMilli`). Also pins the
  rejection path (NaN, Infinity, string).
- **`internal/config/config_test.go`**: From 1 test to ~50 sub-tests.
  Coverage now includes: defaults (Bind, Port, LogLevelRaw, KiroRegion,
  ShutdownTimeout, DBPath, APIKey); per-env overrides (BIND, PORT,
  API_KEY, DB_PATH, LOG_LEVEL, KIRO_REGION, KIRO_DB_PATH,
  SHUTDOWN_TIMEOUT) including their error paths (non-digit, zero,
  >65535); flag overrides (`-port`, `-bind`); flag parse errors;
  `LogLevel()` mapping (debug/info/warn/error/empty/unknown); `envOr`;
  `atoiWithDefault` strict-parser invariants (rejects negative,
  whitespace, '+' prefix, hex — pinned so future refactors don't
  silently widen); error message context (env var name appears in
  error so operators can locate misconfig).
- **`cmd/kiroxy/main.go`**: OpenTelemetry initialization gated behind
  `KIROXY_OTEL_ENABLED=1`. When enabled, `tracing.Init(ctx)` runs once
  at startup, the returned shutdown hook is threaded into
  `awaitShutdown` so pending spans are flushed AFTER vault close on
  SIGINT/SIGTERM. `tracing.Init` failure is non-fatal — server logs
  WARN and continues without traces. Endpoint reads from
  `OTEL_EXPORTER_OTLP_ENDPOINT` per OTel SDK convention.

### Added (Phase 3 — Throttle stampede prevention, 2026-05-14)

Distinguishes server-side capacity shortage from per-account quota
exhaustion so the pool no longer cooldowns the whole fleet when Kiro
itself is overloaded. Investigation showed ~73% of `ThrottlingException`
events carry `reason=INSUFFICIENT_MODEL_CAPACITY` (server side, no
account at fault); the existing classifier treated all of them as
`FailureQuota` → 1h cooldown per account, eventually quarantining every
account in the pool during a single Kiro overload window. Phase 3 traces
the AWS reason code, classifies it separately, and short-circuits the
RecordFailure path so capacity events never accrue toward cooldown.

- **`internal/kiroclient/aws_error.go`**: `UpstreamError` gains a
  `Reason` field. New `parseAWSExceptionFields(body) (exType, reason)`
  extracts both `__type` and `reason` from the JSON body in a single
  pass; `resolveAWSExceptionFields(body, header)` is the companion that
  prefers header-derived `:exception-type` when present. The legacy
  `parseAWSExceptionType` and `resolveAWSException` remain as shims for
  callers that don't need the reason yet. `Error()` now appends
  `(reason=...)` when populated.
- **`internal/kiroclient/client.go`**: All four error paths (200 with
  AWS exception body; 401/403 with refresh attempt; 429/5xx with
  Retry-After; default 4xx including ThrottlingException) call
  `resolveAWSExceptionFields` and propagate the reason onto
  `UpstreamError`. All five WARN/INFO sites now log
  `account_id` + `reason` for forensic attribution.
- **`internal/messages/execute.go`**: New `isCapacityShortage(ue)`
  helper. Match is explicit on `Reason == "INSUFFICIENT_MODEL_CAPACITY"`
  with a heuristic body-substring fallback (`"experiencing high
  traffic"`) for older response shapes that lack the reason field.
  `isQuotaFailure(ue)` early-returns false on capacity shortage, and
  `RecordFailure` is gated behind `&& !isCapacityShortage(ue)` — even
  the transient FailureRecord path is skipped because Consecutive would
  otherwise trip the cooldown threshold over a long capacity window.
  `rotationFailureReason(ue)` now appends the reason suffix
  (`upstream_exception:ThrottlingException/INSUFFICIENT_MODEL_CAPACITY`)
  for observable cooldown metrics.
- **`internal/messages/execute.go`** (continued): New
  `capacityShortageBaseDelay = 2*time.Second`, used by
  `rotationBackoffFor(attempt, base)`. Capacity events get
  2s/4s/8s rotation backoff instead of 500ms/1s/2s, giving Kiro time to
  recover before the next account is tried. Total worst-case rotation
  latency for capacity events: ~14s (vs. ~2s for genuine per-account
  failures). The legacy `poolRotationBackoff(attempt)` is preserved as
  a thin wrapper.
- **`internal/logging/logging.go`**: New `WithAccountID(ctx, id)` and
  `AccountIDFromContext(ctx)` helpers. Threads the account ID through
  context so deeper layers (`kiroclient`) can attribute log lines
  without invasive signature changes.

Verified live (4-min soak): 2 capacity events landed against 2 distinct
accounts; 0 accounts entered cooldown; pool returned to 78 healthy
weight=1 immediately. Baseline 502 rate (2.5%) preserved.

### Added (Phase 2 — Quality / correctness fixes, 2026-05-14)

Ten targeted patches addressing the highest-impact items from the
backend audit (see `BACKLOG.md` and the audit summary in the Phase 1
notes). Each change ships with a build+verify cycle.

- **`internal/messages/execute.go`** (Phase 2.1): Replaces fragile
  `containsCI` substring matching with typed error classification via
  `errors.As(http2.GoAwayError{})` + `errors.As(http2.StreamError{})` +
  `errors.Is(err, context.DeadlineExceeded)` + `net.Error.Timeout()`.
  Detects REFUSED_STREAM, ENHANCE_YOUR_CALM, CANCEL, INTERNAL,
  PROTOCOL_ERROR per RFC 7540 §8.1.4. Returns stable Exception tags
  (`Http2:GOAWAY`, `Http2:REFUSED_STREAM`) for metric labels.
- **`internal/pool/pool.go`** (Phase 2.2): Cooldown-transition
  double-fire fix. Previous logic compared the just-set
  `h.CooldownUntil` against `prev`, which always fired during 429
  storms. New `wasInactive = prev.IsZero() || !prev.After(now)` only
  emits the metric on transition from no-cooldown into active.
- **`internal/pool/pool.go`** (Phase 2.3): `stick.Release(id)` now
  gated behind `cooldownJustApplied` so transient sub-threshold
  failures no longer unbind the session pin.
- **`internal/pool/pool.go` + `internal/pool/refresh.go`** (Phase 2.4):
  New exported `Pool.RecordUnauthorized(id, reason)` sets cooldown to
  `MaxCooldown` (max ladder), emits
  `metrics.CooldownReasonUnauthorized`, and releases stickiness.
  Triggered from `GetToken` when proactive refresh returns
  `auth.ErrRefreshUnauthorized` (refresh_token dead → account quietly
  removed from rotation until operator action).
- **`internal/reqconv/schema_sanitize.go`** (Phase 2.5): `anyOf`/
  `oneOf` with multiple non-null branches now go through
  `mergeObjectBranches()` which unions properties and intersects
  required across all object branches. Heterogeneous-branch fallback
  drops `required` from the first branch so calls matching other
  variants aren't hard-rejected for fields that don't apply. Cuts the
  recurring "lossy schema conversion" warnings (was firing 66× / 16
  min on tools with union types).
- **`internal/messages/request.go`** (Phase 2.6): `count_tokens` now
  builds an Anthropic-shaped text representation
  (system + each message Role+Content + each tool
  Name+Description+Schema) rather than counting the Kiro wire payload
  (synthetic ack + ConversationState + ProfileARN +
  tool_specification wrapper, ~3 KiB overhead/request). Eliminates
  the Claude Code / Cline over-trim issue.
- **`internal/kiroclient/backoff.go` + `client.go`** (Phase 2.7):
  Honors `Retry-After` on 429/5xx. Parses both integer-seconds and
  HTTP-date forms; clamps to `maxRetryAfter = 30s`; returns
  `max(server, natural)` so the proxy is never more aggressive than
  the server requested but never waits longer than 30s if the server
  forgot to bound it.
- **`internal/pool/pool.go`** (Phase 2.8): `Consecutive` decay on
  `Pick`. After cooldown expires, the old failure count carried over
  and inflated the next cooldown multiplier
  (`(Consecutive - threshold + 1) × ShortCooldown`). Now reset on the
  next pick after cooldown lapses — recovery is the signal.
- **`internal/pool/usage.go` + `cmd/kiroxy/main.go`** (Phase 2.9):
  Usage poller now refreshes-and-retries on 401/403. Eliminates the
  log spam (~2031 / 2h) of `bearer token invalid` for accounts whose
  access token expired between import and the first chat hot-path
  use. Shares the same `RefreshConfig` as the chat path so a single
  refresh round-trip covers both subsystems via singleflight (Phase 1).
- **`internal/kiroclient/usage.go`** (Phase 2.10): `nextDateReset`
  parser is now float-tolerant. Upstream sometimes emits scientific
  notation (`1.780272E9`), which Go's `int64` unmarshaler rejects
  outright; using `*float64` and casting before `time.Unix` covers
  both forms with a 53-bit mantissa (plenty for any plausible epoch).

### Added (Phase 1 — Stability + foundational hygiene, 2026-05-14)

Six fixes shipped together as the first stability batch after the
backend audit. Verified live across two restarts; 502 rate dropped from
a 24% baseline (pre-audit) to 7% (post-HTTP/2-keepalive in
pre-Phase-1) to 2.5% (post-Phase-1) over a 2h soak with 40
`/v1/messages` requests.

- **`internal/respconv/streaming.go`**: SSE keepalive ping every 15s.
  Resolves FAIL-043 — opus-thinking turns >30s no longer get severed
  by intermediate proxies (CloudFlare/Caddy/nginx default ~30s idle
  timeout). New `keepaliveLoop()` goroutine emits
  `event: ping\ndata: {"type":"ping"}\n\n`. All writes are mutex-
  serialized to prevent interleaving with the event-handler goroutine.
  Stopped on `Finish()` / `WriteError()` via `sync.Once`.
- **`internal/kiroclient/idle_reader.go`**: Drain the producer
  goroutine after `Close()` on timeout. The previous code returned
  to the caller while the goroutine was still potentially writing
  into the caller's buffer — `bufio.Reader` reuses its backing
  array, so this was a reachable data race on stream bytes. Adds
  `<-ch` after `r.rc.Close()`.
- **`internal/kiroclient/client.go` + `internal/messages/execute.go`**:
  401/403 → refresh + rotate. Previously only 403 triggered a refresh,
  and 401 fell through to the default branch with no recovery. The
  full `(401|403)` branch now reads the response body, resolves the
  AWS exception type, and either retries with a fresh token or — if
  refresh has been attempted and surrendered — surfaces an
  `UpstreamError` that `isRotatableUpstreamError` honors as rotatable
  to a different account.
- **`internal/tokenvault/vault.go`**: New `tightenVaultPerms(path)`
  applies mode 0600 to the main DB **plus** `path-wal` and
  `path-shm`. The sidecar files contained plaintext refresh + access
  tokens (uncheckpointed WAL pages, mmap shared region) and were
  inheriting umask 0644. Called from `Open()` after schema init so
  every vault path tightens itself, not just `cmd/kiroxy/main.go`'s
  callsite.
- **`internal/pool/refresh.go`**: Wires `cfg.group.Do(provider+"/"+id,
  ...)` around `refreshOne`. The `singleflight.Group` field had been
  declared but never invoked — concurrent callers hitting an expired
  token simultaneously raced on `vault.Reserve`; losers got
  `ErrLockHeld` → 401 to the user even though a sibling refresh was
  in flight. Closes the BACKLOG P1 thundering-herd hazard.
- **`internal/server/recover.go`** (new) + **`internal/server/server.go`**:
  Outermost panic recovery middleware. Single in-handler panic no
  longer crashes the process; it logs the stack via `slog.Error` with
  `request_id` + `path` + `method` and returns the Anthropic
  `api_error` envelope (or no-op if headers were already flushed —
  for SSE streams, the framing is preserved).


### Added (Phase COMPLETION-C — GetUsageLimits polling, 2026-05-13)

Closes the largest enowX parity gap per
`research-v4/sources/rate-limiting-research.md`: kiroxy now has
per-account credit visibility instead of zero. Five-package landing.

- **`internal/kiroclient/usage.go`**: `GetUsageLimits(ctx, httpClient,
  token, profileArn, region)` hits the AWS Q Developer management plane
  `GET https://q.<region>.amazonaws.com/getUsageLimits` and returns a
  flat `UsageLimits` struct with derived `MonthlyCreditsRemaining` and
  `PercentRemaining()` helpers. `UsageError` classifies 401 / 403+ban /
  423 / 429 / 5xx so the pool can quarantine banned accounts vs retry
  transient ones. Calling `getUsageLimits` does NOT consume credits.
  Override the endpoint via `KIROXY_USAGE_LIMITS_URL` for tests. 14
  unit tests cover the canonical Pro-tier 1000-credit body, Builder-ID
  profileArn omission, derived field correctness, and every error
  classification.
- **`internal/pool/usage.go`**: `UsagePoller` background goroutine
  walks the account list every 60s (configurable), calls the upstream
  via an injected `UsagePollFn`, and stashes results on
  `AccountHealth.UsageLimits`. `ForcePoll(accountID)` lets the chat
  hot path enqueue an out-of-band poll (e.g. on 429); the request is
  coalesced via a buffered channel — full = drop, next tick covers it.
  Banned accounts get a sentinel stamp (cap=used=1, remaining=0) so
  weighted selection deweights them, but no hard cooldown is imposed
  here. Transient failures preserve the stale cache. 10 unit tests
  including disabled-no-op, banned-sentinel, transient stale
  preservation, ForcePoll trigger, idempotent Stop, vault-miss silent
  skip. Race-clean.
- **`internal/pool/health.go`**: `AccountHealth.Weight()` now
  multiplies by `usageFactor(UsageLimits)`:
  - nil / unknown cap → 1.0 (introspection failure never unpicks an
    account)
  - 0..10% remaining → floor (drained; effectively skipped)
  - >10% remaining → linear `PercentRemaining` (50% remaining → half
    weight → spreads load across fleet)

  `usageDrainedThreshold = 0.10` aligns with the enowX 60-minute-window
  finding. `HealthSnapshot` gains `UsageKnown / Cap / Used / Remaining
  / PercentUsed / LastPolled / DaysUntilReset` fields. 8 new tests
  cover nil safety, drained collapse, half-drained linearity, fresh-vs-
  burnt 2:1 selection bias, drained-vs-alive effective skip.
- **`internal/server/dashboard.go` + `cmd/kiroxy/dashboard.go`**:
  `DashboardAccount` exposes `usage_*` JSON fields with omitempty.
  `UsageKnown` is the render gate (false → show "—"). 3 new tests
  cover provider-level wire-up, full HTTP round-trip via
  `/dashboard/api/state`, and the negative case (never-polled accounts
  produce no `usage_*` keys at all).
- **`cmd/kiroxy/main.go`**: poller wired into the pool branch of
  `runServe` with 60s interval, 10s per-account timeout, 2s startup
  delay. `awaitShutdown` extended to stop the poller before
  `vault.Close` so in-flight `Vault.Get()` doesn't race the SQLite
  close. Disabled via `KIROXY_USAGE_POLL_DISABLED=1` for offline mock
  testing. Legacy `KIROXY_KIRO_DB_PATH` branch leaves polling off.

Net effect: dashboards show "Credits: 487 / 1000 (49%)" per account,
and the pool stops sending requests to accounts that are about to hit
their monthly cap or have been quarantined upstream. `make gate` green.



- **BUG 1: Upstream 403 loop with fresh credentials (compounded).** Two
  root causes: (a) `internal/models.Resolve` didn't recognize Anthropic's
  dashed forms (`claude-sonnet-4-5`, `claude-haiku-4-5`, `claude-opus-4-5`)
  so they fell through to pass-through and upstream returned `400 Invalid
  model ID (INVALID_MODEL_ID)`. (b) Because metadata.expires_at was
  miscalculated (see BUG 2), Phase 2.5 proactive refresh never fired, so
  the reactive 403-retry path returned the same stale token 3 times before
  giving up as a 502. Fix (a) is in `internal/models/models.go`; fix (b)
  is in the BUG 2 entry below. Verified end-to-end: `POST /v1/messages`
  with `model=claude-sonnet-4-5` now returns 200 with a real Anthropic
  response body.

- **BUG 2: `expires_at` miscalc in import-accounts-json.** Previously set
  to `time.Now().Unix() + ExpiresIn` at import time; should be derived
  from the entry's `addedAt` timestamp (token issue time) + `expiresIn`.
  When a token is imported hours after issue, the computed expiry was
  hours beyond the true expiry, suppressing Phase 2.5 proactive refresh.
  Added `deriveExpiresAt` helper parsing RFC3339 first, then the legacy
  local-time format `2006-01-02T15:04:05` emitted by `kiro_login.py`,
  with `time.Now()` fallback on empty/unparseable input. 5 table-driven
  subtests in `cmd/kiroxy/import_json_expires_test.go`.

- **BUG 3: `KIROXY_UPSTREAM_URL` env override.** New `Config.KiroUpstreamURL`
  field parsed from the eponymous env var. When set, `main.go` threads it
  into `kiroclient.WithBaseURL` for both the `KIROXY_KIRO_DB_PATH` path
  and the pool-mode path. Unblocks pointing kiroxy at Phase L `mock_kiro`
  for integration tests or at experimental regional endpoints without
  code changes. Tests in `internal/config/config_test.go` cover the env
  wiring; the `WithBaseURL` behavior itself is already tested by
  `internal/kiroclient :: TestHTTPClient_EndpointURL`.

### Added (Phase 2.5.1 — pool refresh test coverage)

- 7 new tests close the Phase 2.5 test gap:
  - `internal/pool/refresh_concurrent_test.go` — concurrent `GetToken`
    across 50 goroutines (singleflight-adjacent: observes that the vault
    Reserve lock serializes; RefreshFn called exactly once; documents the
    real-singleflight gap as BACKLOG P1).
  - 401 → cooldown: `RefreshFn` returning `auth.ErrRefreshUnauthorized`
    surfaces up, caller records `FailureQuota` with reason
    `refresh_rejected`; subsequent `GetToken` sees the cooldown engaged.
  - 5xx backoff: 2 transient errors then success = 3 RefreshFn calls,
    wall time within the expected backoff window.
  - `internal/tokenvault/commit_meta_patch_test.go` — CommitWithMetaPatch
    merges existing metadata (preserves `profile_arn`/`auth_method`),
    rejects stale-generation commits, tolerates malformed prior metadata.
  - `internal/server/refresh_e2e_test.go` — full-stack E2E: seed vault
    with expired social account, POST /v1/messages, assert the refresh
    fired once, the downstream kiroclient saw the NEW access_token in
    its Bearer header, and the vault generation bumped + metadata updated.

- No production code changed. 5 deferred Phase 2.5 tests now landed.


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
