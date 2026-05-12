# OVERNIGHT_LOG.md — kiroxy post-MVP execution log

Append-only. One entry per phase.

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
