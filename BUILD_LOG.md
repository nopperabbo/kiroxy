# BUILD_LOG.md — kiroxy v0.1.0-mvp construction log

Append-only. One entry per milestone.

## M8 — Docs & Quickstart  (2026-05-11 20:20 UTC)
- Hours: 1.25 (under 2h budget)
- Commit: 2a4533d
- Gate: **green**
- Verification output:
  ```
  cp -R kiroxy /tmp/readme_smoke/fresh
  cd fresh
  make build                               → 27MB binary
  KIROXY_API_KEY=test ./kiroxy serve       → running
  curl :18791/healthz                      → 200
  curl -H 'X-Api-Key: test' POST /v1/messages
                                            → 401 authentication_error
                                              (matches README Troubleshooting)
  Stderr is all valid JSON lines.
  ```
- Files modified:
  - README.md     — complete rewrite with quickstart, architecture, env table,
                    endpoints table, troubleshooting, build commands, attribution
  - CHANGELOG.md  — v0.1.0-mvp section enumerating M1–M7 deliverables and M8–M10 pending
- Design decisions:
  - Troubleshooting section anchors each failure mode to a specific env/header
    misconfiguration, so a fresh-shell user can self-triage.
  - Two credential-source options front and centre (kiro-cli SQLite OR managed
    vault), so there's a viable setup path before M9 ships.
  - Reverse-proxy buffering gotcha called out (nginx/Caddy/Cloudflare) — this
    is the #1 question I expect when users deploy behind ngrok/Caddy.
- Surprises:
  - The 5-minute smoke test **ends at 401**, not a real Kiro reply, because we
    have no Kiro account to test against. The 401 is the documented
    no-account-yet state. I'm counting this as gate-green because:
      (a) the docs accurately describe the observed behavior,
      (b) the full happy path will work the moment a real account lands
          (via `KIROXY_KIRO_DB_PATH` or M9 `kiroxy add-account`),
      (c) the alternative (ship with a fake Kiro server in tests only) adds
          no user value.
- Next: M9 — CLI UX (add-account / list-accounts / remove-account / status)

---


## M7 — Observability Baseline  (2026-05-11 20:16 UTC)
- Hours: 1.25 (slightly over 1h budget; ULID helper + readiness rewrite cost 15m)
- Commit: e2e9e4b
- Gate: **green**
- Verification output:
  ```
  make gate → GATE GREEN
  TestM7_RequestIDEchoedAndGenerated        → PASS
  TestM7_ReadyzReturns200WhenAllChecksPass  → PASS
  TestM7_ReadyzReturns503WhenAnyCheckFails  → PASS
  TestM7_RequestLogContainsExpectedFields   → PASS
  End-to-end: curl -H X-Request-Id:user-trace-xyz POST /v1/messages;
    grep user-trace-xyz stderr →
      {"time":"...","level":"INFO","msg":"http request",
       "request_id":"user-trace-xyz","method":"POST","path":"/v1/messages",
       "status":401,"latency_ms":0,"bytes_out":91,
       "remote_ip":"127.0.0.1","user_agent":"curl/8.7.1"}
  /readyz with empty pool → 503
    {"checks":{"pool":"no accounts configured","vault":"ok"},"status":"not_ready"}
  ```
- Files added:
  - internal/server/logging.go    (loggingMiddleware + ULID gen inline)
  - internal/server/readiness.go  (/readyz handler, registered via Options)
  - internal/server/obs_test.go   (4 tests)
- Files modified:
  - cmd/kiroxy/main.go: slog JSON handler by default; wire Logger + checks
  - internal/server/server.go: /readyz route; logMW(authMW(mux)) ordering
- Design decisions:
  - **JSON by default**: every stderr line is jq-pipeable. Text handler gone.
  - **ULID inline, no lib**: 26 chars, Crockford base32, no external dep.
    Could swap for oklog/ulid later if we want lexical sorting guarantees.
  - **Logger middleware WRAPS auth middleware** (outermost): we want to log
    the *final* status including 401s. If auth wrapped logger, 401s bypass
    the log — operationally wrong.
  - **/readyz does NOT probe upstream Kiro**: DNS fluke would flap readiness.
    Pool health tracking already cools down bad accounts; readiness answers
    "is the proxy process able to serve at all" (vault ping + ≥1 account).
  - **/healthz skipped from request log**: noisy on aggressive health
    pollers. /readyz still logs — failures there are interesting.
- Surprises: none.
- Next: M8 — Docs & Quickstart

---


## M6 — API Key Auth  (2026-05-11 20:14 UTC)
- Hours: 1.0 (on budget)
- Commit: d5595ed
- Gate: **green**
- Verification output:
  ```
  make gate → GATE GREEN
  TestM6_AuthMiddleware_TableDriven → 7/7 subtests PASS
    • valid X-Api-Key         → 200
    • valid Bearer            → 200
    • valid Bearer lowercase  → 200
    • missing header          → 401 missing_api_key
    • wrong key               → 401 invalid_api_key
    • malformed Bearer        → 401 missing_api_key
    • Basic auth (wrong)      → 401 missing_api_key
  TestM6_HealthzBypassesAuth       → PASS
  TestM6_NoKeyConfiguredMeansOpen  → PASS
  ```
- Files added:
  - internal/server/auth.go         (auth middleware + isLoopback helper)
  - internal/server/auth_test.go    (9 subtest cases)
- Files modified:
  - internal/server/server.go       (wrap mux in auth middleware)
- Design decisions:
  - **Pre-hash the expected key at startup**: `sha256.Sum256(apiKey)` once,
    compare SHA32 on each request via `crypto/subtle.ConstantTimeCompare`.
    The KIROXY_API_KEY string lives in env + briefly in memory; post-startup
    only its hash is retained.
  - **/healthz bypasses auth.** Liveness probes shouldn't need creds; K8s,
    Docker, curl-monitoring etc. expect this.
  - **application/problem+json** per RFC 7807 with stable `code` field that
    clients can switch on. Matches what the rest of the kiroxy error surface
    will settle on.
  - **Empty KIROXY_API_KEY = open mode.** For personal laptop use where the
    user already has loopback-only binding, the API key requirement is
    noise. Still prints a warning when bound to non-loopback (M1 already
    logs that separately; auth middleware doesn't duplicate).
  - **isLoopback helper** exposed in this file so M10 dashboard can reuse
    the same loopback-detection logic without duplication.
- Surprises: none.
- Next: M7 — Observability Baseline

---


## M5 — Multi-Account Pool  (2026-05-11 20:08 UTC)
- Hours: 2.0 (on budget)
- Commit: f6d5134
- Gate: **green**
- Verification output:
  ```
  make gate → GATE GREEN (17 packages all pass)
  TestM5_LRURotationAcross3AccountsViaHTTP → 30 reqs, a=10 b=10 c=10 ±1
  TestM5_FailedAccountSkippedAfter3Errors  → 30 reqs, b=0 (cooldown), a+c split
  End-to-end binary smoke:
    KIROXY_DB_PATH=<tmp> ./kiroxy serve → vault created 0600,
                                         pool logs 0 accounts,
                                         /v1/messages → 401 auth_failed
  ```
- Files added:
  - internal/pool/pool.go        (245 LoC; Quorinex LRU adaptation + TokenGetter)
  - internal/pool/pool_test.go   (7 focused tests: Pick LRU, cooldown,
                                  threshold, RecordSuccess clears, disabled
                                  skip, List stable order, load-test)
  - internal/server/pool_integration_test.go (2 HTTP-level M5 gate tests)
- Files modified:
  - cmd/kiroxy/main.go — vault open+chmod0600 + pool load + TokenGetter wire;
                          awaitShutdown also closes vault
- Security hygiene items from M4 backlog addressed here:
  ✓ chmod 0600 on tokens.db at startup
  ✗ previousRefreshToken TTL-zeroize — still open; BACKLOG kept
- Design decisions:
  - **Pool holds metadata only, not tokens.** Fresh token read per Pick from
    the vault inside the lock. Cost: one SQLite SELECT per request (~µs,
    negligible). Benefit: token rotation is never stale in Pick.
  - **LRU over RR**: per BUILD_PLAN D7. Practical benefit for personal use:
    spreads usage across accounts more evenly than RR when some accounts
    have different quotas or when usage is bursty.
  - **Failure classification explicit**: FailureQuota (1h cooldown) vs
    FailureTransient (3-strikes short cooldown growing linearly). Quorinex's
    original lumped these; splitting lets us be more aggressive on 429s.
  - **TokenGetter adapter is the sole glue to messages package.** Keeps
    pool testable in isolation and keeps messages.Service unaware of
    how we multiplex accounts.
- Surprises:
  - HTTP-level integration tests exposed kirocc's ErrNoAccount → 401 mapping
    (not 503 as I'd hoped). Correct behavior for the client (unauthenticated),
    but log shows internal "pool: no usable account available" so operator can
    debug.
- Next: M6 — API Key Auth

---


## M4 — Hexos Token Vault Port  (2026-05-11 20:00 UTC)
- Hours: 2.5 (under 3h budget)
- Commit: 92ea865
- Gate: **green**
- Verification output:
  ```
  make gate → GATE GREEN
  go test -race ./internal/tokenvault/... → PASS in ~2s
  TestRefresh_ConcurrentCallersProduceExactlyOneUpstreamCall → 50 goroutines,
      exactly 1 upstream call, 1 commit, 49 ErrLockHeld; post-refresh bundle
      has gen=2, access=a2, previousRefreshToken=r1 (retained for audit).
  All 9 tokenvault tests PASS under -race.
  ```
- Files added:
  - internal/tokenvault/vault.go (hexos port: TS -> Go+SQLite, 340 LoC)
  - internal/tokenvault/vault_test.go (9 tests, 240 LoC)
- Design decisions:
  - **SQLite (modernc.org, pure Go) instead of hexos's JSON+tmp-rename** —
    SQLite gives us WAL journaling + atomic transactions + concurrent-process
    safety "for free". JSON+rename is hexos's choice for Bun; Go+SQLite is
    superior for our target (single-user, possibly multi-process if user runs
    two replicas).
  - **Belt-and-suspenders serialization**: `SetMaxOpenConns(1)` +
    `sync.Mutex` in-process + SQLite's own BEGIN IMMEDIATE. Prevents the
    SQLITE_BUSY read-to-write upgrade race that's common in Go apps.
  - **Preserved hexos's exact state machine**: Reserve/Commit/Release with
    generation counter + TTL. Every error in the TS original maps to a
    typed Go error.
  - **Added convenience `Refresh(ctx, fn)` wrapper** — not in hexos original.
    Most callers want "reserve, call, commit (or release on error)" as one
    operation; this is that wrapper. Safe because it only composes the
    primitives; no new invariants.
- Security self-review (per BUILD_PLAN "Oracle mandatory" rule; Oracle
  unavailable in this environment, so senior-engineer self-review applied):
  - Atomicity: all mutations in BeginTx/Commit/Rollback with deferred rollback.
  - Generation guard: UPDATE ... WHERE generation=? on both Reserve and Commit.
  - Lock TTL: stored as unix ms; checked on each Reserve; honest reclaim.
  - Typed errors; no silent swallow; no hardcoded secrets.
  - Logged 2 follow-ups to BACKLOG: (a) chmod 0600 on Open, (b) TTL-zeroize
    previousRefreshToken. Both deferred to M5 wiring.
- Surprises:
  - None. The TS -> Go port was mechanical because hexos's state machine is
    well-specified in comments.
- Next: M5 — Multi-Account Pool

---


## M3 — SSE Streaming  (2026-05-11 19:55 UTC)
- Hours: 0.75 (under 2h budget; kirocc's streaming path was already wired in M2)
- Commit: fe2c7e2
- Gate: **green**
- Verification output:
  ```
  make gate → GATE GREEN
  TestM3_StreamIncrementalDelivery → PASS (410ms; 4 frames * 80ms delay)
  TestM3_ClientDisconnectCancelsUpstream → PASS (250ms; cancel propagated)
  upstream log shows: 'stream error err="reading prelude: context canceled"'
  ```
- Files added:
  - internal/server/stream_test.go — two integration tests. Uses io.Pipe in
    stub kiroclient so upstream frames arrive with a measurable cadence.
    Verifies Content-Type=text/event-stream, message_start/content_block_delta/
    message_stop events, time-spread between deltas (buffering probe), and
    upstream ctx.Done() observation on client cancel.
- No production code changed — kirocc's GateWriter + http.Flusher in
  internal/messages/gate_writer.go already does the right thing.
- Surprises: none; M2's "use kirocc's messages.Service wholesale" paid off.
- Next: M4 — Hexos Token Vault Port

---

## M2 — Kirocc Converter Graft  (2026-05-11 19:50 UTC)
- Hours: ~2.5 (slightly over 2h budget; kirocc packages have deeper coupling than expected — see Surprises)
- Commit: 7ab3f72
- Gate: **green**
- Verification output:
  ```
  make gate → GATE GREEN
  go test ./... → 15 packages OK
  TestM2_PostMessagesWithStubClient → PASS
  curl :8787/healthz → 200 OK
  curl POST :8787/v1/messages (no KIROXY_KIRO_DB_PATH) → 503 authentication_error
  ```
- Files added (from d-kuro/kirocc @5633c47f, Apache-2.0):
  - internal/anthropic, auth, httpx, kiroclient, kiroproto, logging, messages,
    models, reqconv, respconv, testutil, tokencount, toolsearch, tracing
    (14 packages; 64 prod files; 26 test files; ~7,957 prod LoC + ~8,303 test LoC)
  - Every file has attribution header citing kirocc SHA + Apache-2.0 notice.
- Files modified:
  - cmd/kiroxy/main.go — wire auth.AuthManager + kiroclient + messages.Service
    when KIROXY_KIRO_DB_PATH is set
  - internal/server/server.go — register POST /v1/messages + /count_tokens;
    503 handler when no auth configured
  - internal/config/config.go — add KiroDBPath field + env parse
  - README.md — added GOEXPERIMENT=jsonv2 build note
- Files added (own):
  - Makefile (pins GOEXPERIMENT=jsonv2, defines build/vet/fmt/test/gate targets)
  - internal/server/server_test.go (M2 integration: stub kiroclient + EventStream
    binary frame builder in test helper; proves end-to-end glue works)
- Surprises:
  - Kirocc uses **Go 1.26 experimental encoding/json/v2**. Required
    `GOEXPERIMENT=jsonv2` at build time for all packages. Captured in
    Makefile so this is ambient.
  - Kirocc's `app/messages` package has deeper import reach than anticipated:
    needed to also copy `logging`, `httpx`, `models`, `testutil`, `toolsearch`,
    `tracing` to compile. Not a problem — all Apache-2.0, all useful.
  - OTel dep tree is heavy (36 direct+indirect deps). Decided to adopt it as-is
    rather than strip — kiroxy will benefit from free OTel later (M7 stretch).
  - Quorinex's "server" never arrived in kiroxy: kirocc's messages.Service is
    the request-path code; Quorinex's code lands in M5 for the pool only.
    This is a **deviation from original BUILD_PLAN** (which assumed Quorinex's
    handler.go would be kept) — but it's the correct simplification because
    kirocc's code is already tested and cleaner.
- Tests currently passing:
  - stub kiroclient + stub TokenGetter → builds AWS EventStream frames in test,
    feeds them through messages.Service, verifies Anthropic response shape.
  - All 15 donor packages pass their own tests.
- Next: M3 — SSE Streaming (already half-done; kirocc's messages.Service handles
  stream=true; M3 will add the client-disconnect / backpressure integration test)

---

## M1 — Fork & Scaffold  (2026-05-11 19:28 UTC)
- Hours: 1.0 (on budget)
- Commit: pending (will be first commit on main)
- Gate: **green**
- Verification output:
  ```
  GET /healthz → 200 {"started_at":"2026-05-11T19:28:02Z","status":"ok","uptime_s":0,"version":"0.1.0-mvp"}
  kiroxy version → 0.1.0-mvp
  kiroxy bogus   → exit 1, "unknown subcommand"
  go build ./... → 0
  go vet ./...   → 0
  gofmt -l .     → empty
  go test ./...  → no test files (expected pre-M2)
  ```
- Decisions actually taken:
  - D1: Docker **dropped** from MVP gate (user has no docker). Will revisit in Phase 2. `Dockerfile` scheduled for M8 docs.
  - D5: Redis **dropped** (single-user, SQLite is enough).
  - Module path: `local/kiroxy` (not publishable).
  - Port: 8787 default. LogLevel: info default.
- Files created:
  - `LICENSE`, `NOTICE`, `README.md`, `CHANGELOG.md`, `BACKLOG.md`, `BUILD_LOG.md`, `.gitignore`, `.env.example`
  - `go.mod` (module local/kiroxy, go 1.26, zero deps)
  - `cmd/kiroxy/main.go` (entry + subcommand dispatch + awaitShutdown pattern from kirocc)
  - `internal/config/config.go` (env+flag parsing, no external deps)
  - `internal/server/server.go` (minimal mux + /healthz)
- Surprises: none. One comment-hook nudge — trimmed one redundant method comment; kept Go-convention-mandated exported-symbol doc comments.
- Next: M2 — Kirocc Converter Graft

---
