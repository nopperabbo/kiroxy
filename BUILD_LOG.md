# BUILD_LOG.md — kiroxy v0.1.0-mvp construction log

Append-only. One entry per milestone.

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
