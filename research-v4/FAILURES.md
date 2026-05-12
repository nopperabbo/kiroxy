# FAILURES.md — Kiro-proxy ecosystem failure catalog

> Catalog of failures documented in the Kiro-proxy ecosystem (peer
> repos, issue trackers, production incidents) cross-referenced with
> kiroxy's mitigation status. This is the "what can break, and is
> kiroxy covered?" reference.
>
> Each entry shows:
> - Symptom
> - Root cause (if diagnosed)
> - Fix / workaround
> - **kiroxy status** — mitigated in code, tracked in BACKLOG, or unaddressed
> - Source citation
>
> Scope: 51 entries across 8 categories. Target is full peer coverage,
> not exhaustive bug listing.
>
> **Readiness summary** at §10.

---

## Categories

| Category | Count | § |
|---|---:|---|
| Authentication failures | 10 | §1 |
| Upstream rejection failures | 8 | §2 |
| Streaming / EventStream parse failures | 6 | §3 |
| Pool / account selection failures | 7 | §4 |
| Vault corruption / race failures | 5 | §5 |
| Deployment / runtime failures | 6 | §6 |
| Integration / downstream client failures | 6 | §7 |
| Onboarder failures | 3 | §8 |

---

## 1. Authentication failures

### FAIL-001 — Refresh token returns 401 despite apparently-valid input

**Symptom**: `POST prod.<region>.auth.desktop.kiro.dev/refreshToken`
returns 401/403 with `{"message":"Refresh token is invalid"}` even
though the token was recently minted (< 30 min ago, not near
expiration).

**Root cause** (inferred from peer community): `refresh_token`
server-side revocation. Common triggers:
- Kiro account password change
- AWS Admin action (Workspace org)
- Concurrent refresh from multiple IPs (suspected, not formally
  confirmed)
- "Forgot password" flow on kiro.dev

**Fix**: Re-onboard. Not recoverable from the proxy.

**Peer evidence**: Symptom mentioned in multiple peer discussions;
concrete fix in `jwadow/kiro-gateway` README ("If refresh fails
with 401, the account must be re-added").

**kiroxy status**: **MITIGATED.** `internal/auth/refresh_social.go:81`
classifies 401/403 as `ErrRefreshUnauthorized` (non-retryable).
`pool.RecordFailure(id, FailureQuota, ...)` kicks account into
1-hour cooldown on this error so the operator notices.

---

### FAIL-002 — Token propagation delay after refresh (~1-3s 403 window)

**Symptom**: Refresh endpoint returns fresh access_token, but
`runtime.kiro.dev` rejects it with 403 for a few seconds
immediately after.

**Root cause**: AWS IAM / gateway cache propagation. The auth-mint
service and the runtime service are regionally replicated; the
runtime can see a stale revoke list briefly.

**Peer evidence**: `bhaskoro/kiro-gateway PR #155` (endpoint
migration PR) documents this explicitly.

**kiroxy status**: **MITIGATED.** `internal/kiroclient/client.go:296-312`
retries on 403 up to 3 times with backoff; the 1-3s propagation
window is absorbed.

---

### FAIL-003 — Access_token coalesce drops refresh_token

**Symptom**: After several successful refreshes, the stored
`refresh_token` silently becomes empty string. Subsequent refresh
calls fail with malformed-request errors.

**Root cause**: Social endpoint sometimes returns `refreshToken:
""` (meaning "reuse existing") and sometimes returns a new one.
Code that unconditionally takes the returned value clobbers the
old one with empty string.

**Peer evidence**: `jwadow/kiro-gateway` had this bug pre-March 2026;
fixed by coalesce logic. `Quorinex/Kiro-Go` also had it; fixed in
PR history.

**kiroxy status**: **MITIGATED.** `internal/auth/refresh.go:266,287`
uses `auth.coalesce(result.RefreshToken, creds.RefreshToken)` to
retain old token when new is empty. See PROTOCOL.md §11.8.

---

### FAIL-004 — Singleflight missing on refresh (concurrent bursts)

**Symptom**: Under bursty concurrent load (e.g. claude-code parallel
tool calls), N-1 requests of a batch fail with
`"reserve: another reservation in-flight"`, receive 500/503 from
proxy.

**Root cause**: N goroutines independently call refresh; first wins
vault reservation, others get `ErrLockHeld` error.

**Peer evidence**: kiroxy's own BACKLOG item "Phase 2.5.2: wire
singleflight.Group.Do around refreshOne" (surfaced by
`refresh_concurrent_test.go`).

**kiroxy status**: **TRACKED IN BACKLOG (P1).**
`RefreshConfig.group singleflight.Group` exists at
`internal/pool/refresh.go:60` but is never invoked. Concurrent
callers all call `refreshOne` independently. Fix estimate: 10-20
LoC. `AuthManager` path (kiro-cli SQLite mode) DOES use
singleflight correctly (`refresh.go:100`) — this only affects
pool mode.

---

### FAIL-005 — IDC/OIDC refresh missing device-registration credentials

**Symptom**: Refresh fails with
`"idc credentials missing device registration (clientId/clientSecret)"`.

**Root cause**: kiro-cli SQLite DB was opened by a kiroxy binary that
can't find the OIDC client registration row. Usually happens when:
- Operator changed kiro-cli profile
- kiro-cli was re-initialized without full state
- `~/.aws/sso/cache/` wiped

**Peer evidence**: `internal/auth/refresh.go:155` error literal
shipped in kiroxy.

**kiroxy status**: **MITIGATED (detected) but no auto-recovery.**
Error surfaces to operator; runbook: re-authenticate kiro-cli with
`kiro-cli login`.

---

### FAIL-006 — OIDC client secret rotated server-side

**Symptom**: IDC refresh returns 401 despite valid refresh_token.
Only 1-2 kiro-cli SQLite rows affected at once.

**Root cause**: AWS rotates the OIDC device-flow client
secret periodically. kiro-cli handles this on its side; proxies
reading the SQLite see stale clientSecret.

**Peer evidence**: jwadow issue tracker mentions "re-auth with
kiro-cli" as the fix.

**kiroxy status**: **UNADDRESSED.** kiroxy cannot mint a new client
registration on behalf of the operator (would require interactive
device flow). Operator must `kiro-cli login` to refresh the local
registration.

**Recommendation**: Detect + log an operator-targeted warning:
"IDC client secret appears rotated; run `kiro-cli login` to update".

---

### FAIL-007 — Region mismatch on refresh

**Symptom**: `oidc.eu-central-1.amazonaws.com/token` returns 401
when token was minted for us-east-1 (or vice versa).

**Root cause**: `creds.SSORegion` diverges from the region the
refresh token was issued against.

**Peer evidence**: `internal/auth/refresh.go:157`:
`"idc credentials missing region (check kiro-cli configuration)"`.

**kiroxy status**: **MITIGATED.** Region is parsed from SQLite
state table (`db.go:117-127`) and persisted per-account. The
credentials object carries it to every refresh call.

---

### FAIL-008 — Workspace profileArn collision

**Symptom**: Multiple kiroxy operators who are members of the same
Google Workspace Kiro org import their tokens; vault collapses them
to one row; only the last-imported token works.

**Root cause**: Older kiroxy versions used `profileArn` as the
dedupe key. Workspace members share `profileArn` within the org.

**Peer evidence**: kiroxy Phase G.BATCH BUG 4 — "Workspace
profileArn collision in dedupe key", closed 2026-05-13.

**kiroxy status**: **MITIGATED in v1.0.1+.** 4-layer dedupe cascade:
email → JWT claim → profileArn → token prefix. Collision detection
at import time with `-allow-overwrite` opt-in for rotation.
Legacy vaults need `rm tokens.db && re-import` — operator-visible
migration note in CHANGELOG.

---

### FAIL-009 — Password change revokes all refresh tokens

**Symptom**: Operator changes their Kiro account password (e.g.
routine rotation). Every subsequent refresh fails with 401.

**Root cause**: Password change → full refresh_token revocation
across all issued tokens for that account.

**Peer evidence**: Kiro UX choice; universal across identity
providers.

**kiroxy status**: **OPERATIONAL (not a code bug).** Operator
runbook in research-v4/OPERATIONS.md §9.5. No auto-recovery
possible.

---

### FAIL-010 — Expired refresh_token from long-idle account

**Symptom**: Account unused for ~90 days; refresh returns 401.

**Root cause**: AWS ages refresh_tokens out after prolonged idle.
Exact TTL undocumented but community consensus is ~90 days.

**Peer evidence**: PROTOCOL.md §9.2; peer community observations.

**kiroxy status**: **OPERATIONAL.** Operator must re-onboard.
Recommendation: mention in docs/OPERATIONS.md §7.1 quarterly
health check.

---

## 2. Upstream rejection failures

### FAIL-011 — "profileArn is required for this request"

**Symptom**: `runtime.kiro.dev` returns
`UnauthorizedException: profileArn is required for this request`.

**Root cause**: Request sent with `X-Amz-Target:
AmazonCodeWhispererStreamingService.GenerateAssistantResponse` but
no `profileArn` in body. CodeWhisperer target rejects without it.

**Peer evidence**: Quorinex PR history; jwadow limitations (jwadow
only supports CodeWhisperer target).

**kiroxy status**: **MITIGATED.**
`internal/kiroclient/target.go:19` auto-picks `AmazonQDeveloperStreamingService.SendMessage`
target when `payload.ProfileARN == ""`. See PROTOCOL.md §4.2.

---

### FAIL-012 — Endpoint migration: 404 on `q.<region>.amazonaws.com`

**Symptom**: After 2026-05-15 (AWS deprecation deadline), requests
to legacy `q.<region>.amazonaws.com` return 404 / DNS SERVFAIL.

**Root cause**: AWS sunset of legacy endpoint; requests must route
to `runtime.<region>.kiro.dev`.

**Peer evidence**: `jwadow/kiro-gateway#146`. Migration PRs in
every peer repo.

**kiroxy status**: **MITIGATED.**
`internal/kiroclient/client.go:167-175` defaults to
`runtime.<region>.kiro.dev`. `KIROXY_USE_LEGACY_ENDPOINT=1`
escape hatch available until deprecation.

---

### FAIL-013 — Live 403 regression: fresh creds + new endpoint still rejected

**Symptom**: Fresh refreshed access_token + migrated
`runtime.us-east-1.kiro.dev` + correct profileArn → 403 empty body.
Manual curl with simpler payload shape from same token succeeds.

**Root cause**: **UNKNOWN** as of 2026-05-13. Suspect: request body
field that differs between kiroxy and minimal working curl.
Candidates: elaborate history/tool-context/agentTaskType fields.

**Peer evidence**: kiroxy live smoke 2026-05-12 (BACKLOG P0).

**kiroxy status**: **TRACKED IN BACKLOG (P0, v1.0.1 blocker).**
Investigation plan: KIROXY_TAP body capture + field-by-field diff
against working minimal payload. LoC estimate: 30-100.

---

### FAIL-014 — 200 with application/json error envelope

**Symptom**: Upstream returns HTTP 200 but Content-Type is
`application/json`, body is
`{"__type":"com.amazon.coral.service#ThrottlingException","message":"Rate exceeded"}`.
Naive EventStream parser crashes with "prelude CRC mismatch".

**Root cause**: AWS peculiarity — exception envelopes sent with 200
status for backward-compat with older clients that couldn't handle
non-200.

**Peer evidence**: jwadow shipped detection April 2026; Quorinex
same.

**kiroxy status**: **MITIGATED.** `isEventStreamContentType()` at
`internal/kiroclient/aws_error.go:104` detects JSON Content-Type and
routes to retry-on-throttle path (`client.go:263-287`).

---

### FAIL-015 — ValidationException on tool input schema

**Symptom**: `ValidationException: Invalid input: tool '{name}'
expected property 'x'`.

**Root cause**: Anthropic tool inputSchema uses JSON Schema keywords
that Kiro's tool validator doesn't recognize (e.g. `format`,
`pattern`, `enum` with non-string values).

**Peer evidence**: Quorinex schema sanitization (similar pattern
across the ecosystem).

**kiroxy status**: **MITIGATED.**
`internal/reqconv/schema_sanitize.go` strips unsupported JSON
Schema keywords before sending to Kiro.

---

### FAIL-016 — Request body exceeds size limit

**Symptom**: 413 Payload Too Large or ValidationException with
"content-length exceeded" message on requests >~3 MB.

**Root cause**: Kiro enforces server-side request size limit (not
publicly documented; observed around 3-4 MB).

**Peer evidence**: `jwadow/kiro-gateway#153` (write-tool truncation
bug) is tangentially related.

**kiroxy status**: **PARTIALLY MITIGATED.** kiroxy's inbound
`http.MaxBytesReader` at 4 MiB (`messages/request.go:75`) prevents
clients from sending too-large bodies. Does NOT handle the case
where kiroxy's own history + tools + message accretion produces a
too-large outbound body. Recommendation: surface as operational
warning metric.

---

### FAIL-017 — Invalid conversationId format

**Symptom**: `ValidationException: conversationId must match UUID
pattern`.

**Root cause**: Client-supplied `X-Claude-Code-Session-Id` used as
`conversationId` but isn't valid UUID.

**Peer evidence**: claude-code issue tracker; kiroxy's own handler
accepts any string.

**kiroxy status**: **PARTIALLY MITIGATED.** kiroxy forwards the
header verbatim. Retry-on-invalidStateReason path
(`internal/messages/errors.go:28`) clears `ConversationID` on
`INVALID_CONVERSATION_STATE` and retries once, which covers this.

---

### FAIL-018 — Stale conversation state

**Symptom**: `invalidStateEvent{reason: STALE_CONVERSATION}`
mid-stream, partial response; downstream client sees broken output.

**Root cause**: Kiro's conversation-state cache expired between the
last turn and this one.

**Peer evidence**: Observed in peer logs; mentioned in Quorinex
commit history.

**kiroxy status**: **MITIGATED.**
`internal/messages/errors.go:28` classifies STALE_CONVERSATION as
retryable. `internal/messages/execute.go:86-102` clears
ConversationID and retries once.

---

## 3. Streaming / EventStream parse failures

### FAIL-019 — Prelude CRC mismatch mid-stream

**Symptom**: `prelude CRC mismatch: got X, want Y` error after N
frames.

**Root cause**: Network corruption, upstream misbehavior, or
(historically) 200+application/json delivered bytes to the
eventstream parser (see FAIL-014).

**Peer evidence**: Universal bug pre-jwadow detection fix (April
2026).

**kiroxy status**: **MITIGATED.** kiroxy's Content-Type guard
prevents JSON-into-parser; true CRC mismatches return error and
log raw bytes (`internal/kiroproto/frame.go:42-53`) for debug.

---

### FAIL-020 — Truncated prelude / partial frame

**Symptom**: `truncated prelude: read X/12 bytes` error; stream
ends before complete frame.

**Root cause**: Upstream connection drop, proxy/LB idle timeout,
intermediate network device killing long-lived connections.

**Peer evidence**: Community reports of 502s with Cloudflare-fronted
deployments.

**kiroxy status**: **MITIGATED (detected).**
`internal/kiroproto/frame.go:35` returns typed error. Idle-reader
(`idle_reader.go`, 180s) catches the symmetric case: headers
arrived, bytes stopped flowing. No auto-retry; downstream client
retries.

---

### FAIL-021 — Unknown event type

**Symptom**: Kiro emits a new `:event-type` name that proxy doesn't
recognize; proxy either errors or silently drops.

**Root cause**: Kiro ships new features; event catalog grows.
`contextUsageEvent` was added unannounced in early 2026.

**Peer evidence**: Quorinex PR #37 adds `contextUsageEvent` parsing
after community-reporting. jwadow issue #147 discusses unknown
event handling.

**kiroxy status**: **MITIGATED.**
`internal/kiroproto/eventstream.go:252-259` logs a truncated
payload warning for unknown types but continues parsing. Does NOT
error out. Operator sees warnings in logs.

---

### FAIL-022 — Tool-use delta accumulator off-by-one

**Symptom**: Tool call sent with truncated JSON input; downstream
client errors parsing tool input.

**Root cause**: `toolUseEvent` arrives as delta stream; if proxy
flushes on stop frame but a subsequent delta arrives late, the
final delta is missed.

**Peer evidence**: AntiHub and petehsu both had variants of this.

**kiroxy status**: **MITIGATED.**
`internal/kiroproto/tooluse.go` accumulator flushes on EOF if no
explicit stop frame seen (`eventstream.go:96-99`). Handles
interleaved parallel tool calls via `toolUseId` keying.

---

### FAIL-023 — Empty end_turn thinking-only response

**Symptom**: Request with thinking enabled returns a stream with
only reasoningContentEvent + metadataEvent, no
assistantResponseEvent. Proxy emits `message_stop` with no visible
content; downstream client displays empty response.

**Root cause**: Kiro sometimes completes thinking and stops without
emitting a final assistant text, especially on edge-case prompts.

**Peer evidence**: Observed in kiroxy live testing (code path at
`internal/messages/response.go:100-113`).

**kiroxy status**: **MITIGATED.**
`internal/respconv/accumulator.go:104-109` detects
"thinking only, no text, no tool_use" and kiroxy retries once with
cleared ConversationID. Second failure returns 502 with clear
error message.

---

### FAIL-024 — Buffering proxy strips SSE flushes

**Symptom**: Client (opencode, claude-code) times out with
"chunk timeout" after 30-60s; kiroxy logs show normal streaming.

**Root cause**: Intermediate reverse proxy (nginx default,
Cloudflare with wrong config) buffers SSE responses; client sees
no chunks until upstream completes.

**Peer evidence**: Community reports across reverse-proxy configs.

**kiroxy status**: **NOT A KIROXY BUG**, but operator-impacting.
research-v4/OPERATIONS.md §2 documents required settings:
`flush_interval -1` (Caddy), `proxy_buffering off` (nginx),
`disableChunkedEncoding: false` (cloudflared).

---

## 4. Pool / account selection failures

### FAIL-025 — All accounts in cooldown simultaneously

**Symptom**: `/v1/messages` returns 503; logs show
`pool: no usable account available`.

**Root cause**: Enough sequential failures to cool every account.
Could be quota exhaustion across all accounts, or a transient
upstream outage.

**Peer evidence**: Inherent to any pool-based proxy.

**kiroxy status**: **MITIGATED (detection) + runbook.**
`pool.ErrNoAccount` returned to caller. Operator-facing: dashboard
shows all accounts in cooldown state. Runbook:
research-v4/OPERATIONS.md §9.1.

---

### FAIL-026 — Account selected while token is in-flight for refresh

**Symptom**: Race condition — one goroutine starts refreshing
account A; another picks A before refresh completes; second one
gets old token and sends to upstream; upstream 403s.

**Root cause**: LRU pick + async refresh without a "reserved"
state.

**Peer evidence**: Described in kiroxy Phase 2.5 design notes.

**kiroxy status**: **MITIGATED.**
`internal/pool/refresh.go` marks refresh-in-progress at the vault
layer (`refresh_in_progress`, `refresh_started_at`,
`refresh_lock_expires_at` columns). Pool.Pick respects this state.

---

### FAIL-027 — Pool empty (no accounts added)

**Symptom**: Binary starts, `/readyz` returns 503,
`{"pool":"no accounts configured"}`.

**Root cause**: Operator started kiroxy before running
`kiroxy add-account` or `import-accounts-json`.

**Peer evidence**: First-install stumble in every proxy.

**kiroxy status**: **MITIGATED (detected, actionable message).**
`/readyz` response is operator-readable. `kiroxy` on stdout from
`serve` prints "no Kiro account configured; run `kiroxy
add-account`" when pool is empty.

---

### FAIL-028 — Account marked failed after single transient 5xx

**Symptom**: Single Kiro-side 502 marks account as failed;
subsequent requests skip it for an hour.

**Root cause**: Too-aggressive error threshold.

**Peer evidence**: Discussed in Quorinex issue tracker.

**kiroxy status**: **MITIGATED.** `pool.DefaultPolicy()` requires
`ConsecutiveErrorThreshold = 3` before short cooldown
(`pool.go:61`). Single transient fault doesn't cool the account.
Quota failures (429) DO short-circuit to 1h cooldown immediately,
because continuing to hit a quota-full account just wastes calls.

---

### FAIL-029 — LRU selection concentrates load on one account

**Symptom**: With 3 accounts in pool, one handles 90% of traffic.

**Root cause**: LRU picks always-oldest; if one account is
persistently idle longest (because another is the last-used),
selection patterns skew.

**Peer evidence**: Observed in load-tests; pre-Quorinex had
weighted round-robin for this.

**kiroxy status**: **ACCEPTED TRADE-OFF.** For personal use (1-5
req/s), LRU's imbalance is negligible. Quorinex-style weighted
round-robin is a BACKLOG P3 item if usage justifies.

---

### FAIL-030 — Account metadata lost on bundle update

**Symptom**: After refresh, account loses its `profile_arn` or
`auth_method` metadata; subsequent refreshes fail with
"auth_method: idc" branching incorrectly.

**Root cause**: Refresh writes new bundle that overwrites instead
of merges metadata.

**Peer evidence**: kiroxy Phase 2.5.1 test
`tokenvault/commit_meta_patch_test.go`.

**kiroxy status**: **MITIGATED.** `CommitWithMetaPatch` merges
metadata patches onto existing bundle; only the fields present in
patch are overwritten. See test suite for the merge semantics.

---

### FAIL-031 — Account disabled doesn't survive restart

**Symptom**: Operator disables account; after kiroxy restart,
account is re-enabled.

**Root cause**: Disabled flag stored in-memory on pool, not
persisted to vault.

**Peer evidence**: Discussed in kiroxy Phase H notes.

**kiroxy status**: **MITIGATED.** Disabled state stored on
`Account` struct loaded from vault at boot. Persisted across
restarts.

---

## 5. Vault corruption / race failures

### FAIL-032 — SQLite lock contention on concurrent writes

**Symptom**: `database is locked` or
`ErrLockHeld` errors under burst load.

**Root cause**: SQLite's single-writer limitation + kiroxy's
`MaxOpenConns=1` (`vault.go:82`) means writers queue. Under
sub-50ms burst, callers time out.

**Peer evidence**: Universal in any SQLite-backed gateway.

**kiroxy status**: **PARTIALLY MITIGATED.** WAL mode +
`busy_timeout=5000` (`vault.go:77`) handle most cases.
Singleflight missing on refresh is a separate issue (FAIL-004).
For normal workloads (<100 req/s), adequate.

---

### FAIL-033 — Stale generation rejection

**Symptom**: Two refresh attempts race; one succeeds, the other
tries to commit older state; kiroxy rejects the commit.

**Root cause**: Optimistic concurrency on `generation` column;
commit with stale generation returns error.

**Peer evidence**: kiroxy Phase 2.5.1 test shipped specifically
for this.

**kiroxy status**: **MITIGATED.** Generation increment enforced;
stale commits return `ErrStaleGeneration`. Caller expected to
retry.

---

### FAIL-034 — Malformed metadata JSON

**Symptom**: Corrupt metadata field (e.g. from manual
SQLite editing or truncation) causes account to be skipped at
boot.

**Root cause**: `parseAccountMetadata` can't parse malformed JSON.

**Peer evidence**: kiroxy Phase 2.5.1 test shipped for this.

**kiroxy status**: **MITIGATED.** `parseAccountMetadata` returns
zero-value on parse error; account is included with empty metadata.
Operator sees "no profile_arn" warning. Not a hard-fail.

---

### FAIL-035 — Vault file mode drift to 0644

**Symptom**: `tokens.db` becomes world-readable after external
tool (backup, rsync, cp) touches it.

**Root cause**: Restoring from backup that didn't preserve mode
bits; `cp` without `-p`.

**Peer evidence**: Universal file-mode hygiene issue.

**kiroxy status**: **PARTIALLY MITIGATED.** kiroxy sets 0600 on
DB create (`accounts.go:138-148`) but doesn't re-chmod on each
open. Recommendation: add a boot-time mode check that warns if
not 0600.

---

### FAIL-036 — WAL journal not cleaned up after crash

**Symptom**: After kiroxy crash, `tokens.db-wal` + `tokens.db-shm`
files remain. Next start may see pending transactions.

**Root cause**: SQLite WAL recovery happens on next open; normal
behavior but can confuse operators who `ls -la`.

**Peer evidence**: SQLite documentation; not a kiroxy-specific bug.

**kiroxy status**: **NOT A BUG.** SQLite's normal recovery.
Documented expectation in research-v4/OPERATIONS.md §6.

---

## 6. Deployment / runtime failures

### FAIL-037 — Docker container runs as root

**Symptom**: Container processes run as UID 0; any breakout gains
root on host.

**Root cause**: Missing `USER` directive in Dockerfile.

**Peer evidence**: `litellm` GHSA mentioned in
research-v4/SECURITY.md synthesis.

**kiroxy status**: **MITIGATED.** `Dockerfile:101` uses
`gcr.io/distroless/static-debian12:nonroot` (UID 65532).
`/data` volume pre-chowned. Also
`docker-compose.yml:68-69` drops all capabilities.

---

### FAIL-038 — Port 0.0.0.0 exposed by default

**Symptom**: Binary binds 0.0.0.0 without inbound auth; anyone on
network can drain accounts.

**Root cause**: Default bind should be 127.0.0.1 for single-user.

**Peer evidence**: `jwadow/kiro-gateway/config.py:85-86` defaults
to 0.0.0.0 (research-v4/SECURITY.md §1.2).

**kiroxy status**: **MITIGATED.** `KIROXY_BIND` defaults to
`127.0.0.1` (`config.go:67`). Docker overrides to 0.0.0.0 but
relies on container netns + compose's loopback port mapping.

---

### FAIL-039 — Graceful shutdown truncates in-flight streams

**Symptom**: SIGTERM during active SSE stream; downstream client
sees connection reset.

**Root cause**: Server shutdown doesn't wait for active SSE.

**Peer evidence**: Universal Go net/http server shutdown issue.

**kiroxy status**: **PARTIALLY MITIGATED.**
`KIROXY_SHUTDOWN_TIMEOUT=30s` gives active requests time. Compose
`stop_grace_period: 35s` extends this. Very long streams (>30s)
may still truncate; downstream client retries.

---

### FAIL-040 — Log file fills disk

**Symptom**: kiroxy logs fill stderr / journal; disk fills; binary
crashes on write.

**Root cause**: No built-in log rotation. journald limits by
default but compose json-file driver needs explicit limits.

**Peer evidence**: docker-compose.yml:92-96 shows kiroxy's
included limits (10 MiB × 5 files max).

**kiroxy status**: **MITIGATED.** docker-compose includes log
limits. systemd handles via journald. Docs/OPERATIONS.md §5.5
mentions logrotate for custom deployments.

---

### FAIL-041 — Rate-limit counter not reset on pod restart

**Symptom**: After kiroxy restart, pool health counters reset
(consecutive errors, cooldown state).

**Root cause**: Health state is in-memory (`pool.go:43`).

**Peer evidence**: kiroxy design choice; easy to restore on
restart by re-adding accounts from vault.

**kiroxy status**: **ACCEPTED.** In-memory health state is
intentional; cooldowns should expire on restart. Persisted state
would create stuck-in-cooldown scenarios that require operator
manual reset.

---

### FAIL-042 — Goroutine leak from aborted idle-reader

**Symptom**: Long-running kiroxy process accumulates goroutines;
eventually OOM.

**Root cause**: Idle-reader spawns a ticker goroutine per request;
if not properly cleaned up on context cancel, leaks.

**Peer evidence**: kiroxy idle_reader.go has explicit Close()
semantics per kirocc port comments.

**kiroxy status**: **MITIGATED.**
`internal/kiroclient/client.go:288` wraps body; idle_reader's
Close() is called via downstream response body close. Goroutine
exits cleanly.

---

## 7. Integration / downstream client failures

### FAIL-043 — claude-code chunkTimeout aborts slow thinking

**Symptom**: claude-code aborts stream with "chunk timeout" when
Kiro takes >30s to emit first assistantResponseEvent (common with
thinking enabled + large context).

**Root cause**: claude-code expects a chunk within N seconds; a
purely-thinking prefix without any reasoningContent + assistant
text can exceed this window.

**Peer evidence**: research-v4/ECOSYSTEM.md §1.4 + §9.2.

**kiroxy status**: **UNADDRESSED.** Recommendation (per
ECOSYSTEM.md): emit SSE `ping` events or `: keepalive\n\n`
comments every 15s during long thinking. Not yet implemented.

---

### FAIL-044 — opencode strict Zod schema parse failure

**Symptom**: opencode displays "parse error" in UI; kiroxy logs
show successful response.

**Root cause**: `@ai-sdk/anthropic` uses strict zod validation.
Missing `input_tokens` in `message_start` or missing
`output_tokens` in `message_delta` throws. Extra top-level fields
may also throw.

**Peer evidence**: research-v4/ECOSYSTEM.md §6 + §9.3.

**kiroxy status**: **MITIGATED.**
`internal/respconv/streaming.go` emits well-formed Anthropic SSE
events with proper `usage` fields. Verified against opencode
integration docs.

---

### FAIL-045 — Cursor / Zed URL normalization breaks

**Symptom**: Cursor / Zed point at kiroxy base URL; get 404 on
every request.

**Root cause**: Each client appends a different path:
- Cursor: `{base}/chat/completions`
- Zed: `{base}/v1/messages`
- AI SDK: `{base}/messages`
- LiteLLM: auto-suffixes `/v1/messages`

Operator sets base URL per one client's convention, other
clients fail.

**Peer evidence**: research-v4/ECOSYSTEM.md §9.1.

**kiroxy status**: **MITIGATED.** kiroxy accepts
`{base}`, `{base}/v1`, `{base}/v1/messages`, and the OpenAI
variants simultaneously. Router matches longest-prefix.

---

### FAIL-046 — Zed base URL env var not supported

**Symptom**: Setting `ANTHROPIC_BASE_URL` in env; Zed still hits
api.anthropic.com.

**Root cause**: Zed reads only `settings.json` `api_url`, not env
vars.

**Peer evidence**: research-v4/ECOSYSTEM.md §7.

**kiroxy status**: **NOT A KIROXY BUG.** Operator documentation
should mention Zed's settings.json pattern explicitly.

---

### FAIL-047 — Tool name case-fold mid-stream

**Symptom**: opencode's `experimental_repairToolCall` lowercases
tool names; if proxy was CamelCasing tool names, mismatches occur.

**Root cause**: opencode's defensive repair clashes with proxy
namespace mangling.

**Peer evidence**: research-v4/ECOSYSTEM.md §9.9.

**kiroxy status**: **MITIGATED.** kiroxy preserves tool names
exactly. Only does tool-name shortening for Kiro's 64-char limit
via deterministic SHA suffix, which is reversible via the
ToolNameMap.

---

### FAIL-048 — anthropic-beta header stripped by intermediate proxy

**Symptom**: Client sends `anthropic-beta: context-1m-2025-08-07`;
kiroxy forwards; but upstream doesn't return 1M context.

**Root cause**: Reverse proxy in front of kiroxy strips
`anthropic-*` headers as "unknown".

**Peer evidence**: research-v4/ECOSYSTEM.md §9.5.

**kiroxy status**: **MITIGATED in kiroxy.** kiroxy forwards
`anthropic-beta` verbatim. Operator-side: reverse-proxy config
must not strip.

---

## 8. Onboarder failures

### FAIL-049 — Camoufox profile corruption

**Symptom**: `tools/onboard/onboard.py` hangs at browser launch;
logs show profile-directory errors.

**Root cause**: Previous Camoufox crash left profile in
inconsistent state.

**Peer evidence**: kiroxy Phase G.FIX notes.

**kiroxy status**: **MITIGATED via runbook.** Delete profile dir:
`rm -rf tools/onboard/profiles/` and re-run.

---

### FAIL-050 — Google account flagged for automation

**Symptom**: Onboarder completes Kiro auth flow, but Google shows
CAPTCHA / "unusual activity" blocks.

**Root cause**: Google's anti-automation detection flags the
account or the source IP.

**Peer evidence**: research-v4/FUTURE.md §Camoufox; Phase G.FIX
documentation.

**kiroxy status**: **MITIGATED (layered defenses, not guaranteed).**
6-layer stealth stack: warm profile, residential proxy support,
human-like interaction, challenge detection, session reuse,
fingerprint diagnostic. Honest reliability band 65-80% (documented
in `tools/onboard/README.md`).

---

### FAIL-051 — Kiro login flow changed

**Symptom**: Onboarder completes Google auth; selector for
"Authorize Kiro" button not found; times out.

**Root cause**: Kiro updates its auth UI; Camoufox selectors
break.

**Peer evidence**: Inherent to UI automation against
externally-controlled service.

**kiroxy status**: **OPERATIONAL.** Operator runbook: test
manually via Kiro IDE first. If flow has changed, update selectors
in `kiro_login.py` / `kiro_oauth.py`. Recommendation for v1.1: add
pre-onboard "selector probe" that sanity-checks UI elements.

---

## 9. Kiroxy failure-readiness score

Tallied mitigation status across 51 entries:

| Status | Count | % |
|---|---:|---:|
| Mitigated in code | 33 | 65% |
| Partially mitigated | 5 | 10% |
| Operational (runbook-only) | 5 | 10% |
| Tracked in BACKLOG | 3 | 6% |
| Unaddressed | 3 | 6% |
| Not a kiroxy bug | 2 | 4% |

**Verdict**: kiroxy covers 75% of known failure modes in code
(mitigated + partially mitigated). The remaining 25% split between:
- Tracked items (acceptable; actively worked)
- Operational (acceptable; documented runbooks)
- Unaddressed (3 items; see below)

### Highest-risk unaddressed categories

1. **FAIL-013 (live 403 regression)** — BLOCKING for v1.0.1. No
   code mitigation; investigation ongoing.
2. **FAIL-006 (OIDC client secret rotation)** — No auto-detect;
   operator must recognize and re-run `kiro-cli login`. Log a
   warning when a cluster of OIDC 401s happens.
3. **FAIL-043 (claude-code chunk timeout on slow thinking)** —
   Recommendation to emit SSE ping events during long thinking
   blocks. Easy win; ~10 LoC.

---

## 10. Kiroxy-specific BACKLOG items surfaced by this catalog

Roll these into BACKLOG.md for v1.0.1:

- **NEW P0**: Ship SSE keepalive pings during long thinking blocks
  to avoid claude-code chunk timeouts (FAIL-043). ~10 LoC.
- **NEW P1**: Detect OIDC client-secret rotation signal; emit
  operator-targeted warning (FAIL-006).
- **NEW P1**: Boot-time vault mode sanity check; warn if not 0600
  (FAIL-035).
- **NEW P2**: Selector probe for onboarder; sanity-check Kiro UI
  elements before full run (FAIL-051).
- **NEW P3**: Weighted round-robin as alternative to LRU
  (FAIL-029; probably unnecessary for personal use).
- **NEW P3**: Request size metrics / warnings (FAIL-016).

Existing BACKLOG items confirmed by this audit:
- P0 upstream-403 regression (FAIL-013)
- P0 `added_at + expires_in` miscalibration (FAIL-003 related)
- P1 Phase 2.5.2 singleflight wiring (FAIL-004)

---

*Compiled 2026-05-13 from peer evidence + kiroxy BACKLOG +
research-v4 cross-references. Update on each new failure report.*
