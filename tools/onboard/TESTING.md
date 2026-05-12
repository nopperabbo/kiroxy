# Onboarder Testing Protocol

Automated tests cover everything that can be tested without a real Google
account. This document is the manual checklist for operators validating a
code change against a live Google account.

## Automated tests (run first, always pass before live testing)

```bash
cd tools/onboard
python3 -m unittest discover -s . -p "test_*.py"
```

Expected: **85 tests pass in ~3.5s**.

Breakdown:

- `test_oauth.py` — 14 tests: PKCE, URL building, callback parsing.
- `test_human.py` — 17 tests: typing distribution statistical properties.
- `test_challenge.py` — 21 tests: detection patterns in all kinds, prompt flow.
- `test_warmup.py` — 10 tests: marker TTL, hard cap, failure isolation.
- `test_proxy_support.py` — 17 tests: URL parsing + env/flag precedence.
- `test_onboard_mock.py` — 6 tests: end-to-end against stdlib mock HTTP fixture.

If any of these fail, **do not proceed to live testing** — the deterministic
layers are broken and live results will be noise.

## Manual test 1 — syntax-only smoke (no Google)

Goal: confirm `--help` and module imports work.

```bash
python3 onboard.py --help
python3 fingerprint_check.py --help
```

Both should exit 0 with all flags listed. No tracebacks.

## Manual test 2 — fingerprint baseline

Goal: confirm Camoufox launches and the fingerprint looks "human-ish".

```bash
python3 fingerprint_check.py                                   # direct
python3 fingerprint_check.py --proxy "$KIROXY_ONBOARD_PROXY"   # with proxy
```

Expected signals from `fingerprint_report.txt`:

- **sannysoft**: `{passed: <high>, failed: <low>, warn: <some>}`. Anything
  catastrophic shows up as failed count > 20 (normal Camoufox shows 0–5).
- **egress_ip**: should match your proxy's exit IP (or your home IP if
  direct). If proxy is set but egress_ip is your home IP, the proxy isn't
  working.
- **creepjs**: screenshot only. Trust score typically "medium" to "high"
  for Camoufox. "Low" or "very low" indicates fingerprint leak.

Screenshots in `screenshots/fp_*.png` are the real evidence — inspect them.

## Manual test 3 — live onboard, challenge-mode=manual

Goal: confirm the pipeline works end-to-end with a real account when you
sign in by hand. Isolates all non-Google bugs.

Pre: a test Google account you don't mind cooking on. Recommended to
actually 2FA it so we know the challenge prompt path works.

```bash
python onboard.py \
  --email test@gmail.com \
  --password - \
  --provider google \
  --challenge-mode manual \
  --output /tmp/test_kiro_tokens.json
```

Expected:

1. Log prints profile/warmup/login URL.
2. Camoufox window opens at the Kiro login page.
3. Warmup runs (YouTube → Google search → GitHub) visible in the window.
4. Kiro `/login` loads; operator clicks "Continue with Google" and signs
   in manually.
5. Browser redirects to `kiro://…` and Camoufox shows a protocol-handler
   prompt (Firefox intercepts non-http schemes).
6. Script captures the URL, exchanges for tokens, writes
   `/tmp/test_kiro_tokens.json`.
7. Log ends with `✓ added account test@gmail.com (profile=…, expiresIn=…s)`.

Verify:

```bash
jq . /tmp/test_kiro_tokens.json
# Should have provider, authMethod, accessToken, refreshToken, profileArn, expiresIn, addedAt
```

```bash
./kiroxy import-accounts-json -file /tmp/test_kiro_tokens.json -provider kiro -dry-run
# Should show "dry-run: 1 valid, 0 skipped"
```

## Manual test 4 — live onboard, challenge-mode=auto, fresh account

Goal: confirm the fully-automated path works on a cold account that
probably won't challenge.

Pre:

- Fresh Gmail account, no 2FA, never used in automation.
- Residential proxy configured (`KIROXY_ONBOARD_PROXY`).
- Account's profile dir does NOT exist (delete
  `profiles_data/<id>/` if it does — forces warmup).

```bash
python onboard.py \
  --email fresh_test@gmail.com \
  --password - \
  --provider google \
  --output /tmp/test_kiro_tokens.json
```

Expected (ideal path):

1. Proxy validation logs egress IP.
2. Warmup runs for ~85 seconds.
3. Google login page loads.
4. Email + password auto-typed (watch the window — should look
   human-ish: bursts, pauses, maybe a typo+backspace).
5. After password submit, script logs `scanning for Google challenges`.
6. Google redirects back to Kiro → Kiro redirects to `kiro://`.
7. Tokens written, script exits 0.

Expected (challenge path):

1-4. Same as above.
5. Script logs `challenge detected: VERIFY_ITS_YOU` (or similar).
6. Prompt prints `⚠ Google wants you to confirm your identity. Solve it
   in the browser window, then press ENTER to continue.`
7. Operator solves it in the window.
8. Operator presses ENTER in the terminal.
9. Flow resumes; tokens written.

If the script times out without detecting a challenge, save the screenshot
from `screenshots/` and check what Google was showing — likely a
previously-unknown challenge kind. Add to `challenge.py::_TEXT_PATTERNS`.

## Manual test 5 — profile persistence across runs

Goal: confirm warmup runs once per account, not per invocation.

```bash
python onboard.py --email test@gmail.com --password - ...
# (complete first-run flow with ~85s warmup)

python onboard.py --email test@gmail.com --password - ...
# (second run: log should say "warmup: skipped (profile recently warmed)")
```

Also verify `profiles_data/<account-id>/.warmed-at` exists after the first
run and contains a recent unix timestamp.

## Manual test 6 — hard-block handling

Goal: confirm hard-block path exits cleanly with a useful screenshot.

Pre: an account Google has already blocked (or a previously-automated
account that now gets blocked reliably). If you don't have one, skip.

```bash
python onboard.py --email blocked@gmail.com --password - ...
```

Expected:

- Script types credentials.
- Script detects `BLOCKED` challenge kind.
- Script exits 1 with `Google hard-blocked sign-in`.
- Screenshot saved to `screenshots/onboard_blocked_gmail_*_blocked.png`.
- No stacktrace, no retry.

## Manual test 7 — residential proxy ok vs broken

Goal: confirm egress validation fails fast on a bad proxy.

### Ok case

```bash
export KIROXY_ONBOARD_PROXY='http://GOOD:CREDS@proxy:8080'
python onboard.py --email any@gmail.com --password - ...
# Expect log: "proxy: ok (egress IP X.Y.Z.W)"
```

### Broken case

```bash
export KIROXY_ONBOARD_PROXY='http://wrong:creds@proxy:8080'
python onboard.py --email any@gmail.com --password - ...
# Expect: exit 1 with "proxy egress validation failed"
# Expect: no Camoufox launched (failed at validation step)
```

## Regression checklist (run after every Phase G change)

- [ ] Automated tests pass: `python3 -m unittest discover -s . -p "test_*.py"`
- [ ] `python3 onboard.py --help` exit 0
- [ ] `python3 fingerprint_check.py --help` exit 0
- [ ] Manual test 3 (challenge-mode=manual) works against the operator's
      own Google account
- [ ] `jq .` on the output shows the expected 7 keys
- [ ] `kiroxy import-accounts-json -dry-run` accepts the output
- [ ] No passwords in any log line (grep your terminal scrollback)
- [ ] No credentials or tokens in any committed file (`git diff
      HEAD~10..HEAD -- tools/onboard/`)

## Known-failure modes to document when encountered

If the onboarder fails in a new way (challenge pattern we don't catch,
Camoufox error, rate limit response shape we haven't seen), capture:

1. Screenshot from `screenshots/`.
2. Log output (with `--password` redacted — double-check).
3. `profiles_data/<id>/.warmed-at` timestamp.
4. Egress IP from `fingerprint_report.txt`.
5. Account's approximate history (fresh / previously-automated / 2FA on).

Add to a new section of this doc with date + git SHA so future operators
know it's been seen.
