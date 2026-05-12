# OVERNIGHT_LOG.md — kiroxy post-MVP execution log

Append-only. One entry per phase.

## Phase C — Autonomous Smoke Test  (2026-05-12 13:08 UTC)
- Hours: ~40 min
- Commit: (this one)
- Tag: NONE — verdict FAIL, BLOCKED.md written
- Gate: **green** (make gate OK; `/v1/messages` upstream rejected)
- Verdict: **FAIL — upstream Kiro rejects our Builder ID access_token**

### What was tested (port 8788, account e3ba0c18, temp inbound API key)
- TEST 1 (non-stream): HTTP 502, upstream "profileArn is required" / "credential is invalid"
- TEST 2 (stream):    502 as JSON body, 0 SSE chunks
- TEST 3 (bad key):   PASS, 401 problem+json (kiroxy auth middleware correct)
- TEST 4 (5x):        5x 502, same root cause

### Fix attempts (2 of 3 before hard-stop)
1. **Route to AmazonQ target when profileArn absent** (new `internal/kiroclient/target.go`).
   Result: error namespace changed from `com.amazon.kiro.runtimeservice` to
   `com.amazon.aws.codewhisperer`. Proves routing works; different backend
   now parses our request. But credential still rejected.
2. **Swap kirocc's kiro-cli User-Agent to Kiro IDE aws-sdk-js UA** (matches
   Quorinex). Result: no change; same "credential invalid" error.

Attempt 3 NOT MADE per BUILD_PLAN hard-stop + Oracle-mandatory directive.

### Root-cause evidence
Decoded the `client_secret` JWT (stored in vault metadata) and found
`hasRequestedScopes: false` + `areAllScopesConsentedTo: false`. Stored
tokens have the Kiro social-auth `aoa...`/`aor...` prefix but were obtained
through AWS SSO OIDC. Three hypotheses in SMOKE_TEST.md; user decision
required to proceed.

### Files retained from investigation (kept, all green in `make gate`)
- `internal/kiroclient/target.go` + `target_test.go` — `chooseAmzTarget()`
  switch logic. Works as intended; demonstrated by 2 distinct upstream
  error shapes.
- `internal/kiroclient/client.go` — KiroIDE UA constants + wired to
  `chooseAmzTarget`. 2 pre-existing tests updated accordingly.

### Clean-up verified
- Port 8788 released after each run.
- No kiroxy serve process leaked.
- `~/.config/opencode/*` untouched (safety rule 1).
- No git push.

### What this blocks
- Phase D (Docker) can still proceed (independent of upstream auth).
- Phase F (opencode integration) can still proceed in terms of docs,
  snippets, and `kiroxy opencode-config` subcommand; real end-to-end
  through opencode waits on an account that authenticates upstream.

### Waiting for user
4 options enumerated in BLOCKED.md — kiro-cli path, triplet import,
specialist consult, or proceed-despite-failure.

---




## Phase C-PREP — Bug Fixes Before Smoke Test  (2026-05-12 12:26 UTC)
- Hours: ~30 min (on budget)
- Commit: 9b599f3
- Tag: v0.2.1-patch
- Gate: **green**
- Verification output:
  ```
  make gate → GATE GREEN (18 packages)
  New vault tests: TestOpen_AutoCreatesParentDir, TestOpen_RejectsReadOnlyParent
  
  # Repro from user report, all now fixed:
  Bug 1: KIROXY_DB_PATH=/tmp/missing/nested/tokens.db ./kiroxy list-accounts
         → 'no accounts' (dirs created 0700)
  Bug 2: ./kiroxy version                    → v0.2.1-patch
  Bug 3: ./kiroxy --version / -version / -v  → version printed, exit 0
         ./kiroxy --help / -h                → usage printed, exit 0
  Bug 4: ./kiroxy add-account --help         → subcommand usage, exit 0
         ./kiroxy serve --help               → subcommand usage, exit 0
         ./kiroxy import-accounts --help     → subcommand usage, exit 0
  ```
- Files modified:
  - internal/tokenvault/vault.go       — Open() now MkdirAll(parent, 0o700)
  - internal/tokenvault/vault_test.go  — +2 tests for auto-create + readonly
  - Makefile                           — VERSION via git describe + ldflags -X
  - cmd/kiroxy/main.go                 — version is var (not const) + top-level
                                          shortcut block for --version/-v/--help
                                          + flag.ErrHelp → exit 0
- Design decisions:
  - **Top-level shortcuts before subcommand dispatch.** The alternative was to
    swallow all unknown flags in serve's flag.FlagSet, but that would mask
    real typos. Explicit whitelist is safer.
  - **VERSION uses `git describe --tags --always --dirty`.** `--dirty` so the
    dev loop shows '-dirty' when tree has uncommitted changes (this commit
    produced 'v0.2.0-dirty' until committed; post-commit clean build prints
    'v0.2.1-patch').
  - **flag.ErrHelp → os.Exit(0) at main, not at each subcommand.** One place to
    catch, applies uniformly.
- Surprises:
  - First attempt made version a `const`; ldflags `-X` silently no-op on
    consts. Had to change to `var`. Go linker caveat I should have remembered.
  - Subcommand dispatch initially used `startsWithDash(args[0])` to route
    '-version' to 'serve', which then tried to parse it as a flag and failed.
    The fix (top-level shortcuts) side-steps that entirely.
- BACKLOG diff:
  - No new items. 4 bugs closed.

---




## Phase B — Builder ID Device-Code OAuth  (2026-05-12 11:35 UTC)
- Hours: ~2.5 (under 3h budget)
- Commit: c89057a
- Tag: v0.2.0
- Gate: **green**
- Verification output:
  ```
  make gate → GATE GREEN (18 packages)
  go test -race ./internal/builderid → 8/8 PASS in ~7s
    (SlowDown test really sleeps 5s+ to prove the interval bump)
  Smoke:
    kiroxy add-account --refresh-token=rt → still works (fallback)
    kiroxy add-account -h                 → new flags visible
  ```
- Files added:
  - internal/builderid/builderid.go       (420 LoC, new package)
  - internal/builderid/builderid_test.go  (290 LoC, 8 mock-OIDC tests)
- Files modified:
  - cmd/kiroxy/accounts.go  — split into addAccountWithRefreshToken (old)
                               + addAccountViaOAuth (new default). Opens
                               browser, polls, persists.
- Design decisions:
  - **Rewrote rather than ported Quorinex's code.** Same wire shapes + URLs +
    scopes, but cleaner: typed errors instead of 6-return-value tuple,
    no package-level session registry (caller scope), no background GC
    goroutine (Go context deadline is enough). MIT attribution preserved
    in file header.
  - **Metadata column stores client_id + client_secret** from the registered
    OIDC client. This is what Quorinex persists for the 'IdC' auth path.
    kirocc's refresh flow only needs refresh_token for desktop-auth, but if
    we ever add the OIDC refresh flow we already have what we need.
  - **Browser auto-open is opt-in-by-default**. --open=false for headless
    environments. Falls back silently to manual URL copy if open fails.
  - **Ticker prints '.' every 3 poll attempts**. Light progress feedback
    without spam.
  - **5-minute default timeout**. Generous for human pace; the underlying
    device authorization expires in 600s anyway.
- Surprises: none. State machine matches AWS OIDC spec as documented in
  Quorinex + cross-referenced with kirocc's auth/refresh.go handling.
- Not tested: live OAuth against prod AWS. That's the Phase C smoke test.
- BACKLOG diff:
  - Closed: 'AWS Builder ID device-code OAuth inside add-account' (was P1)

---


## Phase A — Triplet Bulk Import  (2026-05-12 11:21 UTC)
- Hours: ~50 min (under 1h budget)
- Commit: 9cbcdbb
- Tag: v0.1.1
- Gate: **green**
- Verification output:
  ```
  make gate → GATE GREEN (18 packages)
  go test -race ./... → all pass
  6 new tests for import:
    TestParseTriplets_HappyPath
    TestParseTriplets_InvalidLinesSkipped
    TestParseTriplets_EmptyInput
    TestImportOne_AddsThenUpdates
    TestRunImportAccounts_StdinIntegration
    TestRunImportAccounts_MissingSource
  End-to-end (4-line file, 1 invalid):
    imported 3/4 (added=3 updated=0 skipped=1)
    stdin → added
    re-import alice → warn + updated, gen=2, metadata refreshed
  ```
- Files added:
  - cmd/kiroxy/import.go      (210 LoC)
  - cmd/kiroxy/import_test.go (140 LoC, 6 tests)
- Files modified:
  - cmd/kiroxy/main.go        (dispatch + help)
  - internal/tokenvault/vault.go (metadata column + migration)
  - README.md                 (triplet doc)
- Signature investigation (BUILD_PLAN required decision):
  **Outcome: signature is NOT required by Kiro upstream.**
  Evidence cross-checked across 4 repos:
    - jwadow/kiro-gateway (1.3k⭐, reference impl):
      POST https://prod.{region}.auth.desktop.kiro.dev/refreshToken
      body = {"refreshToken": "..."}  — nothing else
    - AIClient2API (7.7k⭐): same shape, confirmed in src/scripts/kiro-token-refresh.js
    - Quorinex/Kiro-Go auth/builderid.go: stores ClientID + ClientSecret
      for OIDC, but NO signature. Its `Signature` field in proxy/translator.go
      is Anthropic's extended-thinking block signature (response payload), not
      a credential.
    - hexos has a generateSignature() but that's for Qoder upstream, not Kiro.
  **Decision:** signature goes to vault.metadata as opaque JSON, never sent
  upstream. This preserves the extractor's output without coupling us to
  its semantics. If a future Kiro auth flow ever requires it, the column
  exists and is reachable.
- Schema migration: metadata TEXT NOT NULL DEFAULT '', idempotent ADD COLUMN.
- Surprises:
  - First run showed 'skipped=2' with only 1 reason printed. Bug in the
    summary math (over-counted by adding 'total' back in). Fixed within the
    phase budget. Reported correctly now: 'skipped=1'.
- BACKLOG diff:
  - No new items.

---


---
