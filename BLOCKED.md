# BLOCKED.md — kiroxy status tracker

**Current status: NOT BLOCKED.**

All blockers identified during Phase C.1 and Phase C.2 have been resolved.
End-to-end proxy operation was validated in **Phase C.2b** (tagged `v0.2.2`,
2026-05-12) using Desktop-flow refresh tokens obtained via a Camoufox-based
OAuth flow that hits `prod.us-east-1.auth.desktop.kiro.dev/login` directly
with the Kiro Desktop client ID.

---

## Historical — Phase C.1 (Builder ID OAuth) — RESOLVED as superseded

**Outcome:** Builder ID Free-tier tokens lack CodeWhisperer scopes
(`hasRequestedScopes: false` in decoded JWT); upstream rejected every
`/v1/messages`. The `kiroxy add-account` Builder ID path is retained in
code but should be treated as "best-effort — may not work on Free tier".
Production path is the Desktop-flow JSON import (Phase C.2b).

## Historical — Phase C.2 (kikirro triplet import) — RESOLVED as wrong-source

**Outcome:** `refresh_tokens.txt` from the kikirro extractor contains
`email:refresh_token` pairs (NOT triplets — the colon inside the cookie
value is internal). The refresh tokens themselves are scoped to
`app.kiro.dev` (Kiro Web Portal), not `auth.desktop.kiro.dev` (Kiro
Desktop). Different OAuth client_id, different audience → Kiro Desktop
auth correctly rejects them with 401 "Bad credentials". No fix possible
at kiroxy layer; tokens must come from the Desktop-flow extractor
instead (see Phase C.2b).

Full diagnostic matrix (Steps A–C, DIAG 1–4, region sweep) retained in
OVERNIGHT_LOG.md Phase C.2 entry.

## Phase C.2b (Desktop-flow JSON import) — RESOLVED, SUCCESS

**Outcome:** With tokens obtained via the Desktop-flow extractor
(`tools/onboard/` sidecar from Phase G, or the user's external camoufox
script), `/v1/messages` works end-to-end:

- Non-streaming: HTTP 200, 3.5s, valid Anthropic response body
- Streaming: HTTP 200, 2.6s, 1021-byte SSE body, 7 correct events
  (`message_start` → `content_block_start` → `content_block_delta` ×2 →
  `content_block_stop` → `message_delta` → `message_stop`)

Full details in OVERNIGHT_LOG.md Phase C.2b entry and the `v0.2.2` tag
annotation.

---

## Known operational gap (NOT a blocker — tracked in BACKLOG as P1)

Pool-mode refresher is not yet wired. Imported accounts stop working
after `expires_in` seconds (~1h) until the P1 backlog item ships.
Workaround: re-run `kiroxy import-accounts-json` with a fresh token
(via the onboarder) when expiry approaches. This is operational friction,
not a blocker.
