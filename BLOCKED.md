# BLOCKED.md — v0.2.1-patch smoke test

**Phase:** C (autonomous smoke test)  
**Halted at:** after 2 of 3 allowed fix attempts  
**Reason:** upstream Kiro rejects our Builder ID access_token with no further code-side diagnostic avenues available without user input

See `SMOKE_TEST.md` for full evidence + per-test outputs.

## One-line summary

Kiro upstream returns `ValidationException: The provided credential is invalid` for every `POST /v1/messages` call that uses the Builder ID OAuth account we registered in Phase B. The error shape changes as we vary our outgoing `X-Amz-Target`, which proves the downstream routing is correct; but the credential itself is never accepted.

## Why I stopped rather than attempt a 3rd fix

The brief states: "3 consecutive failures on a phase → write BLOCKED.md, halt. No retry loops. Prefer Oracle over shotgun debugging. Oracle consultation mandatory for: auth-header edge cases."

The next plausible fixes are:

- Change the refresh endpoint (OIDC → Kiro Desktop social) and re-refresh the stored token.
- Add a `ListAvailableProfiles` call post-OAuth to obtain a profileArn and attach it.
- Switch account auth flow to a triplet-paste (from an extractor that captured an already-paid Kiro session).

Each requires either upstream API knowledge I don't have cached or a user-side action (provide working credentials from a different source). Iterating here without input is shotgun debugging.

## What's clean

- `make gate` → GATE GREEN (18 packages + new kiroclient tests)
- `./kiroxy serve` starts on `:8788`, healthy in <1s
- `/healthz`, `/readyz`, `/dashboard`, inbound API-key auth, pool selection, vault read, request translation, response translation, SSE writer, graceful shutdown — all verified working
- Port 8788 freed, no kiroxy processes left running
- `~/.config/opencode/*` untouched

## What's broken

- `POST /v1/messages` → 502 for the only configured account, regardless of streaming/non-streaming/target-header
- `/readyz` currently reports `ready` because the vault is reachable and the pool has 1 account; it does not validate upstream-reachability (documented decision from M7)

## Artifacts

- `SMOKE_TEST.md` — full evidence, error bodies, vault forensics, 3 hypotheses
- Commit `<pending>` — includes chooseAmzTarget + UA swap + test updates, all green, valuable even though not the root-cause fix
- Server logs: `/tmp/kiroxy-smoke.log.final`

## What I need from you

One of:

1. **Kiro-cli present?** Supply the path to its SQLite and I'll test via `KIROXY_KIRO_DB_PATH`. That proves the rest of the chain works.

2. **Triplet file available?** Run `kiroxy import-accounts --file=...` with extractor output; it should produce Kiro-social-auth tokens that, once we route the refresh through `auth.desktop.kiro.dev` instead of `oidc.amazonaws.com`, should unlock CodeWhisperer with profileArn included in the refresh response.

3. **Permission to consult specialists (Oracle / librarian)** to search Quorinex issues + Kiro-CLI source for the Builder-ID-to-CodeWhisperer exchange pattern. This is the Oracle-mandatory case from the brief.

4. **Guidance to proceed to Phase D and F** despite Phase C failure — Docker + opencode docs can still be built on top of current v0.2.1-patch, because they're independent of the upstream-auth issue.

No changes will be made until you choose.
