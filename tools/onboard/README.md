# kiroxy Onboarder

Semi-automated Kiro Desktop OAuth token acquisition with manual-solve
recovery for Google challenges.

## Reality Check

This tool attempts to automate Google SSO login for Kiro. Google invests
heavily in bot detection. **There is no "full-auto" in the real world.**
Realistic expectations after Phase G.FIX (layered stealth):

| Scenario | Success rate |
|---|---|
| Fresh Gmail, residential proxy, warmed profile, no 2FA | **65–80%** |
| Fresh Gmail, warmed profile, no proxy | **25–45%** |
| Account previously used in automation | **30–50%** with proxy |
| Account with 2FA active | **0% full-auto**; 100% with challenge-mode=auto (prompts for code) |
| Account Google has flagged | **5–15%**; usually fall back to `kiro_login.py` |

These numbers are estimates from public stealth research, not SLAs.
Google's detection models update; what works today may stop next week.

### When automation fails

The tool is designed to *fail gracefully*, not to always succeed:

- **Challenge detected** (reCAPTCHA, 2FA, "verify it's you"): script prompts
  you to solve it in the browser window, then resumes. This is the common
  case; just press ENTER in the terminal after solving.
- **Hard block** (Google explicitly says "sign-in blocked"): script exits
  with a screenshot. Account cooled down; try a different IP / profile, or
  use `kiro_login.py` (fully manual sign-in) to recover.
- **Timeout without challenge**: Google is stalling the risk-score probe.
  Screenshot saved; consider switching providers (Google → GitHub if the
  account supports it), or use `kiro_login.py`.

## Setup

```bash
cd tools/onboard
python3 -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
python -m camoufox fetch
```

The `camoufox fetch` step downloads the Camoufox browser binary (~150MB
as a one-time per-machine cost).

### Optional: residential proxy

**Strongly recommended.** Google assigns low risk scores to residential IPs
and high scores to datacenter IPs (home Wi-Fi that has been flagged is
effectively datacenter). A residential proxy lifts success rate by
15–25 percentage points in our testing.

Providers we'd pick first: Bright Data, Smartproxy, IPRoyal, Thordata. Not
sponsored. Expect ~$50–100/month for personal-scale use. Don't bundle a
proxy with this tool. Don't use free proxies (all flagged).

```bash
export KIROXY_ONBOARD_PROXY='http://user:pass@residential.example:8080'
# socks5 also supported; for socks install the extra:
#   pip install 'httpx[socks]'
```

## Usage

```bash
# Password on CLI (visible in `ps`):
python onboard.py --email you@gmail.com --password 'yourpass' \
  --provider google --output ../../kiro_tokens.json

# Password from stdin (preferred, not visible in `ps`):
python onboard.py --email you@gmail.com --password - \
  --provider google --output ../../kiro_tokens.json
# then type the password and press Enter
```

### Recommended first-run command

```bash
export KIROXY_ONBOARD_PROXY='http://user:pass@residential.example:8080'
python onboard.py \
  --email you@gmail.com \
  --password - \
  --provider google \
  --output ~/kiro_tokens.json
```

The first run for a new account does a 60–90s warmup (YouTube → Google
search → GitHub) to build session cookies; subsequent runs skip warmup
for 7 days.

Output JSON shape (array; upserts by `email`, falling back to
`profileArn` for legacy entries without an `email` field):

```json
[
  {
    "provider": "Google",
    "authMethod": "social",
    "email": "you@gmail.com",
    "accessToken": "aoa...",
    "refreshToken": "aor...",
    "profileArn": "arn:aws:codewhisperer:...:profile/XXXXXXXX",
    "expiresIn": 3600,
    "addedAt": "2026-05-12T07:45:00Z"
  }
]
```

> **v1.0.1 dedupe change.** Earlier output files keyed accounts by the
> last segment of `profileArn`, which silently collapsed Google Workspace
> users within the same Kiro org (they share a `profileArn`) into a
> single entry. `onboard.py` and `kiroxy import-accounts-json` now key
> on `email` first. Legacy JSON files without `email` still import via a
> `profileArn → token prefix` fallback cascade. If you'\''re on a pre-v1.0.1
> vault and need Workspace dedupe, `rm tokens.db` and re-import; your
> refresh tokens remain valid.

## Batch mode — `batch.py`

Drives multiple single-account onboards with state, rate limit, failure
classification, and resume. Use this when you have a credentials list
(email:password per line) and want an overnight or unattended run.

**Input format** (same shape as kikirro'\''s `email.txt`):

```
# comments OK
alice@gmail.com:password1
bob@gmail.com:password with spaces and:colons-preserved
```

**Pre-flight checklist** before batching more than 2–3 accounts:

- [ ] `KIROXY_ONBOARD_PROXY` exported (residential). The tool prints a
      huge warning if it'\''s unset — batching from a home IP will trip
      Google'\''s IP flagging after 5–10 fresh-account onboards.
- [ ] `python3 -m unittest discover -s . -p "test_*.py"` passes
      (153 tests, ~3s). Skip this, skip everything.
- [ ] `python batch.py --file email.txt --dry-run` shows the expected
      account list and skip reasons. Fix typos here, not mid-batch.
- [ ] Output target cleared (or you'\''re intentionally resuming). Stale
      state + new output file is a weird combination.
- [ ] Test with ONE account via `onboard.py` first to confirm the
      profile + proxy combo is live.

**Usage:**

```bash
python batch.py --file email.txt \
                --output ~/kiro_tokens.json \
                --cooldown-s 60 \
                --state batch_state.json \
                --provider google \
                --max-retries 2

# Validate only (no browser launch):
python batch.py --file email.txt --dry-run
```

**Flags:**

| flag | default | description |
|---|---|---|
| `--file` | (required) | path to `email:password` file |
| `--output` | `./kiro_tokens.json` | output JSON (upserts) |
| `--state` | `./batch_state.json` | per-email status (for resume) |
| `--provider` | `google` | `google` \| `github` |
| `--cooldown-s` | `60` | seconds between accounts (single-threaded) |
| `--max-retries` | `2` | transient retry ceiling per account |
| `--onboard-script` | `./onboard.py` | override onboarder path |
| `--log-root` | `./batch_logs` | per-account subprocess logs |
| `--challenge-mode` | `auto` | passed through to `onboard.py` |
| `--headless` | off | passed through to `onboard.py` |
| `--timeout-login-s` | `120` | passed through to `onboard.py` |
| `--dry-run` | off | parse + plan; don'\''t launch browsers |

**Failure classification:**

| Class | Kinds | Action |
|---|---|---|
| transient | network, browser, timeout | retry up to `--max-retries` |
| hard | blocked, 2FA, wrong_pass, consent, unknown | fail immediately |

All classification keys off subprocess exit code + stderr. The full
onboarder stderr is teed to `batch_logs/<safe_email>/<ts>.log` so you
can post-mortem without re-running.

**Safety thresholds (automatic abort):**

- 3 consecutive hard fails in the last 5 attempts → abort (IP likely
  flagged; take a break, rotate proxies).
- Camoufox crash rate > 20% of ≥ 5 samples → abort (local/tooling
  problem; re-check `python -m camoufox fetch`).

**Resume:**

Re-run the same command. `DONE` entries are skipped. `FAILED` with a
hard kind are sticky (won'\''t be retried — fix the root cause and set
their status back to `pending` in the state file manually if you want
another shot). Transient failures within the retry ceiling are
pending and retried.

**Output:**

```
starting batch: 76 accounts, state=batch_state.json, output=kiro_tokens.json
[1/76] DONE <redacted>@example.com (42.1s, attempt=1)
[2/76] TRANSIENT <redacted>@example.com (browser, attempt=1/3) — will retry
[3/76] FAILED <redacted>@example.com (blocked, attempt=1)
...
────────────────────────────────────────────────────────────
Batch complete: 69/76 succeeded (failed=7, skipped=0)

Failed:
  <redacted>@example.com — hard:blocked (attempts=1)
  ...

Resume: re-run the same command. Transient failures will
be retried up to --max-retries; hard failures are sticky.
```

**Known limitations (v1):**

- Single-threaded only. Parallel onboards trip Google'\''s per-IP rate
  limit fast. `--parallel N` may land later once we'\''ve measured safe
  concurrency with residential proxies.
- Credentials are read from a plain text file. G.2 (credential
  encryption with age or macOS Keychain) is still open in the backlog.

## Integrating with kiroxy

```bash
python onboard.py --email … --password - --output ~/kiro_tokens.json
cd ../..
./kiroxy import-accounts-json -file ~/kiro_tokens.json -provider kiro
./kiroxy list-accounts
```

## Flags

| flag | default | description |
|---|---|---|
| `--email` | (required) | Google / GitHub account email |
| `--password` | (required) | Password; use `-` to read from stdin |
| `--provider` | `google` | `google` \| `github` |
| `--output` | `./kiro_tokens.json` | Output file; upserts if it exists |
| `--profile-id` | (hash) | Override profile selection from `profiles.json` |
| `--profile-dir` | `profiles_data/<account-id>/` | Override Camoufox `user_data_dir` |
| `--proxy` | (env) | Residential proxy URL; overrides `KIROXY_ONBOARD_PROXY` |
| `--skip-warmup` | off | Skip the YouTube/Google/GitHub warmup (debugging only) |
| `--challenge-mode` | `auto` | `auto` \| `manual` \| `skip` (see below) |
| `--headless` | off | Run Camoufox headless (harder to debug and to solve challenges) |
| `--timeout-login-s` | `120` | Seconds to wait for the `kiro://` redirect |

### `--challenge-mode`

- **`auto`** (default): script types credentials; if Google shows a
  challenge (2FA, reCAPTCHA, verify-it's-you), prompts you to solve it in
  the browser and resumes. This is the right mode for most accounts.
- **`manual`**: script does NOT type credentials. Prints a prompt; you sign
  in manually in the Camoufox window. Equivalent to `kiro_login.py` but
  runs in-tree with the onboarder's profile persistence + proxy support.
- **`skip`**: G.1 behavior. Type credentials and hope. No challenge
  detection. Use only for accounts you've confirmed don't challenge.

## Maximizing success

1. **Use a residential proxy.** Set `KIROXY_ONBOARD_PROXY`. Cheap insurance.
2. **Keep profile dirs.** Don't delete `profiles_data/` unless you want to
   redo 90s of warmup per account. They contain session cookies that
   Google's risk score leans on.
3. **Don't run more than 3 accounts per hour.** Google tracks rate per IP.
   Batch mode is explicitly deferred (G.3) to force this.
4. **Prefer fresh Gmail accounts** when possible. Accounts with prior
   automation history trigger challenges more often.
5. **Solve challenges promptly** — the prompt has a 10-minute timeout; if
   you step away, the whole run fails.

## Diagnostic: `fingerprint_check.py`

Run before troubleshooting to see what Google sees:

```bash
python fingerprint_check.py                                # direct connect
python fingerprint_check.py --proxy "$KIROXY_ONBOARD_PROXY"  # with proxy
python fingerprint_check.py --profile-dir profiles_data/<id>  # with warmed profile
```

Visits bot.sannysoft.com and CreepJS, saves screenshots to
`screenshots/fp_*.png`, appends a summary to `fingerprint_report.txt`.
Operator eyeballs the output. Lower failed counts on Sannysoft ≈ better;
higher trust score on CreepJS ≈ better.

## Manual testing protocol

Automated tests cover the deterministic parts (PKCE, proxy parsing,
challenge detection, warmup orchestration, mock-Kiro round-trip). The
live-Google parts require operator hands on keyboard — see
[`TESTING.md`](./TESTING.md) for the checklist.

```bash
python3 -m unittest discover -s . -p "test_*.py"
# 153 tests in ~3s; all should pass.
```

## Troubleshooting

- **`Browser closed unexpectedly` on first run** — run
  `python -m camoufox fetch` to download the browser binary.
- **`Google sign-in blocked`** — Google hard-blocked this IP/account combo.
  Wait ≥24h, switch proxy, or use `kiro_login.py` manually.
- **`proxy egress validation failed`** — proxy creds wrong, or the proxy
  itself is down. Test independently:
  `curl -x "$KIROXY_ONBOARD_PROXY" https://api.ipify.org`.
- **Stuck on `/signin/challenge/pwd?checkConnection=youtube`** — classic
  cold-profile tell. Delete `profiles_data/<account-id>/`, re-run (triggers
  warmup), or switch to a residential proxy.
- **Challenge prompt timed out while you were AFK** — re-run; profile
  state is preserved.
- **`socks proxy support not installed`** — `pip install 'httpx[socks]'`.
- **Screenshots on failure** — saved to `./screenshots/` for post-mortem.

## What's deferred (Phase G.2+)

Phase G.FIX shipped layers 1–6 of the stealth plan; v1.0.1 closed BUG 4
(per-email dedupe) and shipped G.3 (batch orchestrator). Still open:

- **G.2: Credential encryption.** Store passwords encrypted at rest
  (age or macOS Keychain). Today `batch.py` reads plain `email:password`
  lines; encrypted-at-rest credentials are a natural next step.
- **G.4+: Parallel onboards.** Current batch is single-threaded
  by design — Google rate-limits hard per-IP. Needs concurrency
  measurement before a `--parallel N` flag is safe.
- **G.5: Polish & progress UI.**

Tracked in `../../BACKLOG.md`.

## Files

```
tools/onboard/
├── README.md                 — this file
├── TESTING.md                — manual-testing checklist for live Google
├── requirements.txt          — pinned Python deps
├── .gitignore                — runtime artifacts (profiles_data/, screenshots/, batch_logs/, batch_state.json, …)
├── profiles.json             — 100-profile fingerprint rotation table
├── onboard.py                — main single-account CLI entry
├── batch.py                  — multi-account orchestrator (G.3)
├── kiro_oauth.py             — PKCE + login URL + /oauth/token exchange + JWT helper
├── browser_driver.py         — Camoufox wrapper (persistent + proxy + humanize)
├── warmup.py                 — YouTube/Google/GitHub pre-login warmup
├── human.py                  — burst-pause typing, typos, mouse drift
├── challenge.py              — Google challenge detection + manual-solve recovery
├── proxy_support.py          — proxy URL parsing + egress validation
├── fingerprint_check.py      — diagnostic: run to see what Google sees
├── fixtures/mock_kiro.py     — stdlib HTTP server mocking /login + /oauth/token
├── test_oauth.py             — 25 tests: PKCE, URL, callback parsing, JWT claims
├── test_human.py             — 17 tests: typing distribution, typos, drift
├── test_challenge.py         — 21 tests: detection patterns, prompt flow
├── test_warmup.py            — 10 tests: marker, TTL, cap, failure paths
├── test_proxy_support.py     — 17 tests: URL parsing, env/flag precedence
├── test_onboard_mock.py      — 15 tests: mock HTTP + dedupe cascade + upsert
└── test_batch.py             — 48 tests: credential parsing, classification,
                                abort thresholds, state round-trip,
                                run_batch integration with fakes
```
