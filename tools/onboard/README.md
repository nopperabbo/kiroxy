# kiroxy Onboarder

Full-auto Kiro Desktop OAuth token acquisition.

## WARNING

This tool automates Google credential entry in a browser. This may violate
Google's Terms of Service and can result in temporary or permanent account
lockouts. Use only with accounts you own and accept this risk on.

## What it does

Kiro Desktop's OAuth flow is a PKCE code-grant against
`prod.us-east-1.auth.desktop.kiro.dev` with redirect to
`kiro://kiro.kiroAgent/authenticate-success`. Normally you'd run Kiro IDE and
click "Sign in"; this tool automates that step end-to-end:

1. Generate PKCE verifier/challenge/state.
2. Launch a stealth browser (Camoufox, Firefox-based) at the Kiro login URL
   with `idp=Google` (or `idp=Github`).
3. Fill the email and password fields; handle the consent screen.
4. Intercept the `kiro://…?code=…&state=…` redirect.
5. POST the code + verifier to `/oauth/token` and collect
   `accessToken / refreshToken / profileArn / expiresIn`.
6. Append the result to an output JSON whose shape is exactly what
   `kiroxy import-accounts-json` expects.

## Setup

```bash
cd tools/onboard
python3 -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
python -m camoufox fetch
```

The `camoufox fetch` step downloads the Camoufox browser binary (~150MB). It
runs once per machine.

## Usage (single account, G.1)

```bash
# password on CLI (convenient; visible in `ps`)
python onboard.py --email you@gmail.com --password 'yourpass' \
  --provider google --output ../../kiro_tokens.json

# password via stdin (hardens against `ps` leakage)
python onboard.py --email you@gmail.com --password - \
  --provider google --output ../../kiro_tokens.json
# then type the password and press Enter
```

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
python onboard.py --email … --password … --output ~/kiro_tokens.json
kiroxy import-accounts-json -file ~/kiro_tokens.json -provider kiro
kiroxy list-accounts
```

## Flags

| flag | default | description |
|---|---|---|
| `--email` | (required) | Google / GitHub account email |
| `--password` | (required) | Password; use `-` to read from stdin |
| `--provider` | `google` | `google` \| `github` |
| `--output` | `./kiro_tokens.json` | Output file; appends if it exists |
| `--profile-id` | (hash) | Override profile selection from profiles.json |
| `--headless` | `false` | Run Camoufox headless (harder to debug) |
| `--timeout-login-s` | `120` | Total time to wait for redirect |

## Troubleshooting

- **`Browser closed unexpectedly` on first run** — run
  `python -m camoufox fetch` to download the browser binary.
- **`Google sign-in blocked`** — Google triggered anti-abuse detection.
  Wait ≥30 min; or log in manually once in a real browser to clear the
  challenge; then retry.
- **`Timeout waiting for kiro:// redirect`** — consent screen likely
  needed manual click. Re-run with `--headless=false` (default) and watch.
- **Screenshots on failure** — saved to `./screenshots/` for debugging.

## What's deferred (Phase G.2+)

- G.2: Credential encryption (age or macOS Keychain) for stored creds.
- G.3: Batch mode with concurrency cap (multi-account in one invocation).
- G.4: Retry logic + failure classification (transient vs hard).
- G.5: Polish, progress UI, cleaner docs.

Tracked in `../BACKLOG.md`.
