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

Output JSON shape (array; upserts by `profileArn`):

```json
[
  {
    "provider": "Google",
    "authMethod": "social",
    "accessToken": "aoa...",
    "refreshToken": "aor...",
    "profileArn": "arn:aws:codewhisperer:...:profile/XXXXXXXX",
    "expiresIn": 3600,
    "addedAt": "2026-05-12T07:45:00Z"
  }
]
```

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
# 85 tests in ~3.5s; all should pass.
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

Phase G.FIX shipped layers 1–6 of the stealth plan. Not yet in tree:

- **G.2: Credential encryption.** Store passwords encrypted at rest (age
  or macOS Keychain) for batch mode.
- **G.3: Batch mode with concurrency cap.** Run multiple accounts from a
  credentials file, respecting Google's rate limits.
- **G.4: Retry logic + failure classification.** Distinguish transient
  failures (retry with backoff) from hard blocks (fail fast).
- **G.5: Polish & progress UI.**

Tracked in `../../BACKLOG.md`.

## Files

```
tools/onboard/
├── README.md                 — this file
├── TESTING.md                — manual-testing checklist for live Google
├── requirements.txt          — pinned Python deps
├── .gitignore                — runtime artifacts (profiles_data/, screenshots/, …)
├── profiles.json             — 100-profile fingerprint rotation table
├── onboard.py                — main single-account CLI entry
├── kiro_oauth.py             — PKCE + login URL + /oauth/token exchange
├── browser_driver.py         — Camoufox wrapper (persistent + proxy + humanize)
├── warmup.py                 — YouTube/Google/GitHub pre-login warmup
├── human.py                  — burst-pause typing, typos, mouse drift
├── challenge.py              — Google challenge detection + manual-solve recovery
├── proxy_support.py          — proxy URL parsing + egress validation
├── fingerprint_check.py      — diagnostic: run to see what Google sees
├── fixtures/mock_kiro.py     — stdlib HTTP server mocking /login + /oauth/token
├── test_oauth.py             — 14 tests: PKCE, URL, callback parsing
├── test_human.py             — 17 tests: typing distribution, typos, drift
├── test_challenge.py         — 21 tests: detection patterns, prompt flow
├── test_warmup.py            — 10 tests: marker, TTL, cap, failure paths
├── test_proxy_support.py     — 17 tests: URL parsing, env/flag precedence
└── test_onboard_mock.py      —  6 tests: end-to-end against mock HTTP fixture
```
