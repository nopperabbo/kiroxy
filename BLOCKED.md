# BLOCKED.md — Phase C.2 status (updated 2026-05-12 14:20 UTC)

**Phase C.2 verdict: BLOCKED on credential availability.**
**Phase C.1 (Builder ID OAuth) remains BLOCKED separately.**
kiroxy internals unchanged: still working + fully green on `make gate`.

---

## Phase C.2 Diagnosis Timeline

1. **Step A (import)** — triplet imported cleanly from `refresh_tokens.txt`. Vault correctly persists `refreshToken` + `metadata.signature`. Account `darlenebowen@dineu.tech` listed in pool.
2. **Step B (debug-refresh subcommand)** — added `kiroxy debug-refresh` to isolate the pool-layer refresh gap. Calls `prod.us-east-1.auth.desktop.kiro.dev/refreshToken` directly with the stored token.
3. **Step C (initial refresh)** — HTTP 401 `{"message":"Bad credentials"}`.
4. **DIAG 1** (wire dump, Go default UA) — 401 with `X-Amzn-Errortype: UnauthorizedException:com.amazon.kiroauthservice`.
5. **DIAG 2** (Kiro aws-sdk-js style UA) — identical 401. UA format ruled out.
6. **DIAG 3** (snake_case body `refresh_token`) — HTTP 400 `ValidationException` demanding camelCase `refreshToken`. Confirmed our wire format is canonical.
7. **DIAG 2-REDO** (correct `KiroIDE-0.10.32-<64-hex-machineid>` UA + `Sec-Fetch-Mode: cors`) — identical 401. Full Kiro IDE mimicry didn't help.
8. **DIAG 4** (region sweep `us-west-2`, `eu-west-1`) — **DNS no-such-host**. Kiro auth endpoint only exists at `us-east-1`; alternative regions aren't DNS-registered.

## Ruled Out

- Signature requirement (would produce signature-named error; we got credential-level errors)
- Wire format / field casing
- Client User-Agent validation
- `Sec-Fetch-Mode` missing
- Regional endpoint mismatch
- Machine ID fingerprint binding

## Remaining Cause (high confidence)

**The refresh_token in `refresh_tokens.txt` is no longer valid.** Either:
- **Expired** — Kiro social refresh tokens have finite lifetime (commonly days-weeks). Token was issued by kikirro at some prior time; if not freshly extracted, natural expiry explains everything.
- **Revoked** — any of (account sign-out elsewhere, AWS abuse-detection invalidation, or already-consumed-by-another-refresh) produces identical 401 shape.
- **Reused** — Kiro social refresh tokens rotate. If any other tool (including a past `debug-refresh` on this token) already consumed it, subsequent uses return 401.

The only decisive next step is a **fresh triplet**: re-run kikirro, replace `refresh_tokens.txt`, retry.

## Keep / Discard

- ✅ **KEEP** — `cmd/kiroxy/debug_refresh.go` — useful admin tool for triplet validation. Wire-dump + UA override + region override + dry-run (`--persist=false`) all useful for diagnostics going forward.
- ✅ **KEEP** — the Phase C.1 fixes in `internal/kiroclient/` (target switch + KiroIDE UA + 5 test updates/additions). Functional and green.
- ✅ **KEEP** — current vault schema with `metadata` column (Phase A Phase 2 migration).

## Waiting for user

Options:

1. **Fresh triplet.** Re-run kikirro against a currently-active Kiro IDE session, drop the new triplet at `refresh_tokens.txt`, reply with "run".
   Expected outcome: 200 with `accessToken` + `profileArn` + optional rotated `refreshToken`. On that, I proceed to Step D (smoke `/v1/messages` with `model: kiro/sonnet-4.5`) + Step E (finalize).

2. **Different account.** If multiple triplets available, swap the file, reply with "run". Same as (1).

3. **Proceed to Phase D + F anyway.** Docker restore + opencode documentation are independent of upstream auth. End-to-end smoke remains blocked until credentials work; but the deliverables are buildable + committable.

4. **Accept as done.** Stop Phase C.2, commit everything, update BUILD_LOG, treat smoke as "pending fresh credentials" indefinitely. Proceed with whatever you want next.

Environment clean: no server running, port 8788 free, `/tmp/kiroxy-triplet-smoke.db*` removed, `~/.config/opencode/*` untouched. No git pushes.
