# Phase 2.5 — Pool-Mode Token Refresh Wiring

**Target:** ship in v0.4.0. Imported-account tokens expire after ~1h; today kiroxy dies silently. We wire proactive refresh for `authMethod="social"` accounts (Desktop-flow via `import-accounts-json`).

**Scope:** only the pool path. `KIROXY_KIRO_DB_PATH` (kiro-cli SQLite mode) already has a working refresher; untouched. Builder ID (Phase B OAuth, no profileArn) does not get refresh wired.

---

## Code inventory (from Step 0 read)

| Component | File | Current behavior |
|---|---|---|
| `pool.TokenGetter.GetToken` | `internal/pool/pool.go:278-302` | Picks account, loads access_token, reads metadata for profile_arn + auth_method. No expiry check. No refresh. |
| `auth.refreshSocialToken` | `internal/auth/refresh.go:272` | Method on `AuthManager`. POSTs `{refreshToken}` to `prod.{region}.auth.desktop.kiro.dev/refreshToken`. `refresh_test.go:320` already instantiates with just `httpClient` (no DB) and calls the method directly. |
| `kiroclient.WithTokenRefresher` | `internal/kiroclient/client.go` | Reactive 403 path. Pool path never installs a refresher. |
| `vault.Save` | `internal/tokenvault/vault.go:148-200` | Upserts whole row; bumps generation; **replaces metadata wholesale**. |
| `tokenvault.Reserve/Commit/Release` | vault | Generation-locked cross-process safety; unused by pool today. |

---

## Design decisions

### D1 — Expiration tracking (no schema migration)

- Store `expires_at` (unix seconds, absolute) inside `metadata` JSON.
- Written at:
  - `import-accounts-json`: `time.Now().Unix() + entry.ExpiresIn`
  - Post-refresh: `time.Now().Unix() + tokenResp.ExpiresIn`
- Non-social accounts or missing `expires_at`: refresh path skipped.
- Backfill: on read, if `expires_at` missing but `added_at + expires_in` present, compute once, persist on next successful refresh.

### D2 — Refresh trigger policy

- **Proactive** in `TokenGetter.GetToken`: if `expires_at < now + 5*time.Minute`, refresh before returning.
- **Reactive fallback**: install `kiroclient.WithTokenRefresher` on pool path (same pattern as KiroDBPath mode).
- Skipped for: empty `auth_method`, `auth_method != "social"`, missing `refresh_token`.

### D3 — Concurrency

- `golang.org/x/sync/singleflight` keyed by `provider + "|" + connection_id`.
- In-process coalescing + vault's existing `Reserve`/`Commit` for cross-process.

### D4 — Failure modes

| Condition | Action |
|---|---|
| Refresh 401 | `pool.RecordFailure(FailureQuota)` → 1h cooldown; log ERROR; no retry |
| Refresh 5xx or net err | Retry ×3 exp backoff 500ms→1s→2s; then bubble up |
| Vault write error post-refresh | Log WARN; return fresh token; next request re-refreshes (idempotent) |

---

## Implementation

### Files to modify

1. `internal/auth/refresh.go` — expose `RefreshSocial(ctx, httpClient, endpoint, refreshToken)` pkg-level. `refreshSocialToken` method wraps.
2. `internal/pool/pool.go` — add `RefreshFn` + `singleflight.Group` to `TokenGetter`. New helper `maybeRefresh`.
3. `internal/tokenvault/vault.go` — new `UpdateTokens(ctx, provider, id, Tokens, mdPatch map[string]any) (*Bundle, error)` that shallow-merges metadata, bumps generation.
4. `cmd/kiroxy/main.go` — build RefreshFn closure, inject into TokenGetter, install WithTokenRefresher for pool path.
5. `cmd/kiroxy/import_json.go` — set `metadata.expires_at` on import.
6. `cmd/kiroxy/accounts.go` — `add-account` fallback: set `expires_at = now+1h` when auth-method="social".

### Files new

- `internal/pool/refresh.go` — `RefreshFn` type, needsRefresh(), parseExpiresAt().
- `internal/pool/refresh_test.go`.
- `internal/server/refresh_integration_test.go`.

---

## Test matrix

### Unit — `internal/pool/refresh_test.go`
- `TestNeedsRefresh_Boundaries` — zero/now-1h/now+1h/now+1m
- `TestRefresher_SkipsNonSocial`
- `TestRefresher_SkipsValidToken`
- `TestRefresher_RefreshesExpired`
- `TestRefresher_SingleflightCoalesces` (50 goroutines → 1 call)
- `TestRefresher_401MarksPoolFailure`
- `TestRefresher_5xxRetries`
- `TestRefresher_NetworkErrorBubblesUp`

### Vault — `internal/tokenvault/vault_test.go`
- `TestUpdateTokens_MergesMetadata`
- `TestUpdateTokens_BumpsGeneration`
- `TestUpdateTokens_EmptyPatchKeepsMeta`

### Integration — `internal/server/refresh_integration_test.go`
- `TestE2E_RefreshOnExpiredAccount` — 2 httptest servers (mock refresh + mock Kiro API), import expired, `/v1/messages`, assert refresh called once, new bearer, valid Anthropic response, vault gen bumped.

---

## Non-goals (strict)

- Do not touch Phase G onboarder code.
- Modify `docs/OPENCODE.md` only to add 1-line note.
- No Prometheus, no rate limiting.
- Builder ID path (`auth_method != "social"`) continues as-is.

## Budget

- 90 min impl + 30 min tests + 15 min verify + 15 min commits = 2.5h of 3h.
