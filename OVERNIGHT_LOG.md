# OVERNIGHT_LOG.md — kiroxy post-MVP execution log

Append-only. One entry per phase.

## Phase D — Docker Deployment Path  (2026-05-12 15:15 UTC)
- Hours: ~1.25
- Commit: (this one)
- Tag: (none — user will tag after full Phase D+F+G review)
- Gate: **green** (`make gate` — 18 packages, all cached or pass)
- Docker-level verification: **static only** (docker not installed on host; docker-* Make targets fail gracefully with clear errors, which is the designed behaviour for devs without docker)
- Live smoke of the new subcommand:
  ```
  ./kiroxy help                                           → 'healthcheck' listed
  ./kiroxy healthcheck --url=http://127.0.0.1:1/healthz   → exit 1, 'connection refused' (expected)
  ./kiroxy serve &                                        → running on :18799
  ./kiroxy healthcheck --url=http://127.0.0.1:18799/healthz
                                                           → exit 0 (verifies status=="ok" from /healthz)
  ```

### Added files
- `Dockerfile` — two-stage build:
  - `builder`: `golang:1.26-alpine` with `GOEXPERIMENT=jsonv2`, BuildKit cache
    mounts on `/go/pkg/mod` + `/root/.cache/go-build`, ldflags inject
    `main.version` from `--build-arg VERSION`. `-trimpath -s -w` for
    reproducible, stripped output.
  - `runtime`: `gcr.io/distroless/static-debian12:nonroot`. No shell, no
    package manager, UID 65532. Ships exactly one file plus `/data`.
    OCI labels, `EXPOSE 8787`, in-binary `HEALTHCHECK` via
    `kiroxy healthcheck`, `VOLUME ["/data"]`.
- `docker-compose.yml` — hardened single-service compose:
  - 127.0.0.1:8787 host port mapping by default; `KIROXY_HOST_PORT` env
    override for operators who front with Caddy/nginx on the Docker network.
  - `read_only: true`, `cap_drop: [ALL]`, `no-new-privileges:true`,
    `tmpfs: /tmp:size=16m,mode=1777`, named volume `kiroxy-data:/data`,
    json-file log rotation (10 MB × 5), `stop_grace_period: 35s`.
  - Healthcheck re-states the Dockerfile's for visibility.
- `.dockerignore` — excludes `.env*`, `*.db*`, `refresh_tokens.txt`,
  `kiro_tokens.json`, `research/`, `_vendors/`, docs that aren't needed
  at build time, VCS + IDE + OS cruft. Keeps build context ~5 MiB.
- `cmd/kiroxy/healthcheck.go` — new subcommand. In-process HTTP GET to
  `/healthz`, decodes `{"status":"ok"}`, exits 0/1. Needed because
  distroless has no `curl` and no shell for `docker exec`-style probes.

### Modified files
- `cmd/kiroxy/main.go` — dispatch + help entry for `healthcheck`.
- `Makefile` — 5 new targets (`docker-build`, `docker-run`,
  `docker-compose-up`, `docker-compose-down`, `docker-clean`) and two
  new variables (`IMAGE`, `LATEST`). Each target prechecks for
  `command -v docker` and prints a readable error when missing, so
  `make docker-build` on a non-Docker host exits 1 with
  `"docker not found in PATH"` rather than a cryptic shell failure.
- `README.md` — new "Run with Docker" section covering quickstart,
  `docker run` one-liner, security posture table, and 3 gotchas
  (`KIROXY_BIND=0.0.0.0` inside the container, `down -v` wipes the
  vault, no `:latest` tag).
- `.env.example` — documented `KIROXY_HOST_PORT` and `KIROXY_VERSION`
  overrides that compose reads.
- `CHANGELOG.md` — [Unreleased] entry enumerating Phase D deliverables.

### Design decisions
- **Distroless-nonroot, not Alpine, not scratch.** Scratch would work
  for the static binary, but distroless-static ships `/etc/passwd`,
  CA certs, tzdata, and a pre-created `nonroot` user — all things we
  need or will need. `:nonroot` tag is hash-pinnable and gets CVE fixes
  without tag churn.
- **`KIROXY_BIND=0.0.0.0` baked into the image**, not the compose file.
  The container's network namespace IS the boundary; binding to
  loopback inside the container would make the port unreachable from
  the host. Operators control real-world exposure via `docker run -p`
  or compose's `ports:` mapping (which defaults to `127.0.0.1:8787`).
- **Healthcheck re-uses the kiroxy binary**, not a sidecar. Shipping
  curl/wget into distroless would double the attack surface of the
  runtime stage; a ~5 KiB `http.Get` wrapper in the same binary is
  strictly better.
- **BuildKit cache mounts**, not `--mount=bind`. Subsequent builds on
  the same host reuse module and compile caches without leaking into
  the image layers. First cold build: ~90 s on estimated Apple-Silicon
  hardware; warm rebuild: <10 s (both estimates — docker not on host
  for actual measurement).
- **No CGO**, because kiroxy uses `modernc.org/sqlite` (pure Go).
  `CGO_ENABLED=0` hard-sets this and keeps the runtime distroless
  image valid (distroless-static has no libc).
- **Two image tags per build** (`kiroxy:$VERSION` + `kiroxy:local`) —
  immutable-version for deploys, stable-alias for local compose.
  Explicitly NOT tagging `:latest`; :latest is an anti-pattern for
  reproducibility.

### Security posture audit (self-review against dockerfile-generator skill)

| Check | Status |
|---|---|
| Pinned base tags (no `:latest`) | ✅ `golang:1.26-alpine`, `distroless/static-debian12:nonroot` |
| Non-root runtime user | ✅ `USER nonroot:nonroot` (UID 65532) |
| Multi-stage build | ✅ builder + runtime |
| No secrets in ENV or build args | ✅ `VERSION` is the only build-arg; API key comes from compose env |
| `.dockerignore` excludes `.env`, `*.db`, secrets | ✅ explicit allowlist of `.env.example`, denylist of the rest |
| Exec-form CMD/ENTRYPOINT | ✅ `["kiroxy"]`, `["serve"]` |
| Cleaned package caches in same layer | ✅ `apk add --no-cache` |
| HEALTHCHECK present | ✅ in-binary subcommand |
| `EXPOSE` documented | ✅ 8787 |
| OCI labels | ✅ title, description, licenses, source, vendor |
| Cap-drop ALL + no-new-privileges | ✅ in compose |
| Read-only root FS | ✅ in compose |
| Reproducibility (`-trimpath`, `-s -w`) | ✅ in builder RUN |

### Ruled-out alternatives
- **Alpine runtime** — viable but adds an unneeded libc + busybox; distroless is stricter and the tradeoff isn't worth it for a Go binary.
- **scratch** — no CA certs, no tzdata, no non-root user by default; would need 3 `COPY --from=builder` lines to bolt those in. Distroless-static already bundles them.
- **`HEALTHCHECK CMD ["wget", ...]`** — wget doesn't exist in distroless. We'd have had to ship it, re-introducing a shell dependency. In-binary probe is cleaner.
- **Cgo + mattn/go-sqlite3** — would need a runtime with libc (Alpine at minimum); Phase A already settled on modernc for zero-cgo reasons.

### Not done (explicit)
- **No `docker build` actually run.** Docker is not installed on this host; docker-build and docker-compose-up targets verify-abort with a clear error, which is their designed behaviour. User on any machine with Docker Desktop can run `make docker-compose-up` from the repo root and observe behaviour matching this log.
- **No live end-to-end smoke through `/v1/messages` via the container.** End-to-end credential flow was resolved in Phase C.2b (v0.2.2 smoke succeeded); actual container-level smoke deferred to a host with Docker installed.
- **No multi-platform (ARM + AMD) build.** The Dockerfile is arch-agnostic; actual `buildx --platform linux/amd64,linux/arm64` is a CI concern and deferred.

### Environment cleanliness
- No server kept running after tests (`pkill -f 'kiroxy serve'`).
- Test SQLite files at `/tmp/dkgate.db*` removed.
- `~/.kiroxy/tokens.db` untouched.
- No git push.

### BACKLOG diff
- No new items; Phase D closes the long-standing "D1 — Docker?" decision from `BUILD_PLAN.md` as **included**, not deferred.

---




## Phase C.2 — Triplet Path Acceptance Test  (2026-05-12 14:20 UTC)
- Hours: ~45 min (within 60 min cap)
- Commit: (this one)
- Tag: NONE — verdict BLOCKED pending fresh credential
- Gate: **green** (make gate OK; upstream refresh fails at credential layer)
- Verdict: **BLOCKED — refresh_token in `refresh_tokens.txt` rejected by upstream**
- Model tested (per addendum): none reached — never got to Step D
  - `kiro/sonnet-4.5` planned; not exercised because Step C refresh failed
  - Canonical naming format verification: deferred

### Added files
- `cmd/kiroxy/debug_refresh.go` — `kiroxy debug-refresh` subcommand.
  Flags: `--provider`, `--id`, `--region`, `--persist`, `--verbose`,
  `--wire`, `--user-agent`, `--snake-case`. Calls
  `prod.{region}.auth.desktop.kiro.dev/refreshToken` directly with stored
  refresh_token; persists new access_token on 2xx. Useful admin/diag tool.

### Modified files
- `cmd/kiroxy/main.go` — dispatch `debug-refresh` subcommand.

### Diagnostic matrix
| # | Variant | Result |
|---|---|---|
| Step C | default UA, camelCase, us-east-1 | 401 Bad credentials |
| DIAG 1 | wire dump (verify wire shape) | 401 (no wire issue) |
| DIAG 2 | `aws-sdk-js/...KiroIDE-` UA | 401 (UA format not the gate) |
| DIAG 3 | `refresh_token` snake_case | 400 ValidationException — camelCase required (rules out field-name mismatch) |
| DIAG 2-REDO | `KiroIDE-0.10.32-<64hex>` + `Sec-Fetch-Mode: cors` | 401 (full IDE mimicry doesn't help) |
| DIAG 4 us-west-2 | regional sweep | DNS no-such-host (endpoint only at us-east-1) |
| DIAG 4 eu-west-1 | regional sweep | DNS no-such-host |

### Conclusion
All request-shape hypotheses ruled out. Only plausible remaining cause
is that the refresh_token itself is no longer valid (expired/revoked/already-
consumed). Kiro's `kiroauthservice` gives crisp `UnauthorizedException: Bad
credentials` for dead tokens; it gives a distinct `ValidationException` for
wire-level issues (confirmed by DIAG 3). The two error shapes are
distinguishable; we're seeing the former.

### Not done (deferred)
- Step D smoke `/v1/messages` with `kiro/sonnet-4.5` (blocked on valid creds)
- Step E finalize with BUILD_LOG Phase C.2 entry, BACKLOG updates
  (this entry IS the finalize for the "blocked" branch)

### BACKLOG promotions (appended via separate edit)
- **P1 (PROMOTED from P2):** wire pool-mode token refresher for
  `source="import-accounts"` accounts. Pool path currently has no
  `WithTokenRefresher`; triplet-imported accounts break after
  access_token expires (~1h) because refresh never fires.
- **P2:** pool tier-awareness — warn/error when Pro model requested
  but picked account is Free tier.
- **P2:** `opencode-config` subcommand should emit all 13 canonical
  models from Kiro tier display.

### Safety verification
- Port 8788 never bound in this phase (no server launched)
- No kiroxy serve process touched
- Production vault at `~/.kiroxy/tokens.db` untouched (we used KIROXY_DB_PATH=/tmp/kiroxy-triplet-smoke.db)
- `~/.config/opencode/*` untouched
- No git push
- `/tmp/kiroxy-triplet-smoke.db*` cleaned before commit

---




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
