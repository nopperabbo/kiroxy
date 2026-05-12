# Phase G.BATCH — Dedupe fix + batch orchestrator

Date: 2026-05-13
Author: Sisyphus (overnight autonomous)
Budget: 4h total
Scope owner: `tools/onboard/*` + `cmd/kiroxy/import_json.go` only.

---

## Problem

Operator wants to onboard **76 Google Workspace accounts across 7 orgs** via
the Phase G.FIX onboarder. Live-test surfaced **BUG 4**: all accounts in the
same Workspace org share the same `profileArn` (same CodeWhisperer profile),
so the current dedupe — which keys on the last segment of `profileArn` —
silently overwrites earlier imports. A 76-account batch would land as 7
accounts in the vault.

Operator also needs a **batch driver** that onboards N accounts with state,
rate limit, failure classification, and collision detection so overnight /
unattended runs don't silently lose work.

## Scope

Two orthogonal pieces land together because they interact at the schema
boundary (the batch driver expects email in the output JSON so it can
confirm the per-account id is unique).

### Part 1 — BUG 4 fix: dedupe by email

1. **Capture email in the output JSON.** `onboard.py` already accepts
   `--email` on the CLI, so the value is available in-process. Simply add
   `"email": args.email` to the output dict. No JWT decode required on the
   Python side for the MVP — the CLI email is authoritative.

2. **Python dedupe key prefers email.**

   ```python
   def _dedupe_key(entry):
       email = (entry.get("email") or "").strip().lower()
       if email:
           return f"email:{email}"
       arn = (entry.get("profileArn") or "").strip()
       ...
   ```

3. **Defensive JWT decode in `kiro_oauth.py`** for the case where a legacy
   or external caller invokes `exchange_code()` without a matching email on
   the CLI. `exchange_code()` stays pure and returns the dict unchanged;
   we add a module-level helper `jwt_sub_or_email(token) -> str | None`
   that `onboard.py` + `batch.py` can call as a fallback. Returns `None`
   on malformed / non-JWT tokens (Kiro `aoa...` tokens are not JWT, so
   this will return `None` in practice — but the helper is tested and
   available for future changes).

4. **Go side `deriveAccountID` cascade** (priority order):
   - P1: `entry.Email` — trim-lowercase.
   - P2: JWT `sub` or `email` claim decoded from `entry.AccessToken`
     (base64url decode of middle segment, parse JSON).
   - P3: last segment of `entry.ProfileArn`.
   - P4: first 12 chars of `entry.AccessToken`.
   - Log which layer fired in the `id_source` metadata.

5. **Collision detection at import.** If id already exists and token prefix
   differs, log a loud warning and skip — unless `-allow-overwrite` is
   passed, in which case rotate in place and note it.

6. **Backward compat.** Legacy JSONs without `"email"` still import (P3
   fallback). Existing vault entries keyed by profileArn are NOT rekeyed;
   docs note that Workspace operators should `rm vault.db` + re-import to
   benefit from per-email dedupe. This is safe because a re-import of
   Desktop-flow tokens always works as long as the refresh token is still
   valid (which it is for the current ~1h window).

### Part 2 — batch orchestrator (`tools/onboard/batch.py`)

**Core module:** a Python driver that reads `email:password` lines, drives
`onboard.py` as a subprocess per account, and persists per-email state.

- **State file** (`batch_state.json`): `{email: {status, last_attempt,
  error_kind, attempts}}`. JSON, atomic write.
- **Resume logic:** if state file exists, skip `done` entries, retry
  `failed` ones (subject to `--max-retries`), and continue `pending`.
- **Rate limit:** `--cooldown-s` (default 60) between accounts. Applied as
  a sleep between subprocess invocations.
- **Failure classification:** parse exit code + stderr of the subprocess.
  - Transient: timeout (124), Camoufox crash (matched by error regex),
    network errors. Retry up to `--max-retries` (default 2).
  - Hard: wrong password ("Google hard-blocked"), consent declined,
    2FA required text, `BLOCKED` screenshot note. Mark failed immediately.
- **Safety rails:**
  - Abort if ≥3 consecutive hard-fails in the last 5 attempts ("IP cooked").
  - Abort if Camoufox crash rate > 20%.
  - Print a HUGE warning when `KIROXY_ONBOARD_PROXY` is unset.
- **Collision detection:** after every successful onboard, diff the output
  JSON against the pre-run snapshot and confirm the account count went up
  by 1 (or was already present for resume). If count stayed flat, log
  "possible collision — check email field in output".
- **Progress UI:** one line per account, plus a summary block at the end.
- **Safety:** always writes state file atomically; subprocess output is
  teed to per-email log files in `./batch_logs/<email>/<ts>.log`.

**Input format:** `email:password` per line, `#` comments allowed. Same as
what the operator already has.

**CLI:**

```
python batch.py --file email.txt \
                --output ../../kiro_tokens.json \
                [--cooldown-s 60] \
                [--state batch_state.json] \
                [--provider google|github] \
                [--max-retries 2] \
                [--dry-run]
```

`--dry-run`: parse, validate schema, print plan, exit.

### Deferred (explicit non-goals for tonight)

- Parallel concurrent onboarding. Single-threaded is safer for v1.
- Credential encryption (G.2). Batch reads plain `email:password` lines.
  Documented as a known limitation in README.
- Live end-to-end testing with real Google accounts. Operator will run
  TESTING.md checklist tomorrow.

## Test plan

### Unit

- `test_kiro_oauth.py` already exists. Add `TestJwtSubExtraction`:
  - Happy: craft minimal JWT with `sub` claim, decode round-trips.
  - `email` claim wins if both present (per helper spec).
  - Malformed token returns `None`.
  - Non-JWT string returns `None`.
- `test_onboard_mock.py` already exists. Add `TestDedupeKey`:
  - email present → `email:…` key.
  - email absent + profileArn present → `arn:…` key.
  - both absent → accessToken prefix.
  - upsert by email rotates tokens in place.
- New `test_batch.py`:
  - State round-trip: write → read → match.
  - Resume: pre-populate state with `done`/`failed`, confirm
    skipped/retried correctly.
  - Classification: stderr snippets map to transient vs hard.
  - Abort threshold: 3 consecutive hard fails triggers abort.
  - Parse `email:password` input format with comments + empty lines.

### Integration

- `import_json_test.go` table-driven cascade:
  - email-only entry.
  - JWT-bearing accessToken with no email.
  - profileArn with no email/JWT.
  - token-only (malformed everything else).
  - collision: same id, different tokens → warn/skip without `-allow-overwrite`.
  - collision + `-allow-overwrite`: rotate in place.

### Manual

- `python batch.py --file examples/fake.txt --dry-run` — plan prints, no
  browser launch, exit 0.
- `python batch.py --file examples/fake.txt` without proxy — huge warning
  printed, three entries attempted, all fail with classification hard/
  transient per-email, state file written.
- Re-run same command — resumes retrying failed entries (subject to
  max-retries ceiling).

## Rollout

- Part 1 → 4 commits (c1–c5).
- Part 2 → 3 commits (c6–c8) + 2 docs commits (c9–c10).
- No tag. Operator will cut v1.0.1 after manually smoke-testing tomorrow.
- No `git push`.

## Risk / HALT conditions

- Track 1 (concurrent session) modifying `import_json.go` — mitigated by
  keeping changes additive (new field, new helper, new test file) and
  re-reading the file before every Go edit.
- Python JWT decode ambiguity — mitigated by making the JWT helper
  defensive (returns None on any parse failure) and fallback-only.
- Budget overrun — Part 1 must land even if Part 2 is only partially done.
  Part 1 is a correctness fix; Part 2 is a productivity feature.
