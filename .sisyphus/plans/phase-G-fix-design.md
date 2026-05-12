# Phase G.FIX — Design

**Goal:** lift onboarder reliability from "stuck at Google password challenge 100% of the time" to a **realistic 40-70% band** on Google SSO, with graceful manual-assist recovery when automation fails.

**Ceiling established by sibling tools:** kikirro with Patchright + 100-profile rotation has documented 30%+ block rate. That is the state of the art for unaided automation; our ceiling is similar. We are not building "autonomous"; we are building "auto-with-recovery".

**Scope boundary:** this phase touches only `tools/onboard/`. Go code is untouched. kiroxy core (`internal/`, `cmd/`, `main.go`, auth, pool) is untouched.

---

## 1. Root-cause model (why G.1 fails today)

Observed: Camoufox session types email+password correctly, Google navigates to `/v3/signin/challenge/pwd?checkConnection=youtube`, and that is terminal — the `kiro://` redirect never fires within the 120s budget.

`checkConnection=youtube` is the literal query Google uses to probe whether the caller has a live YouTube session cookie. If it doesn't, the risk score rises. On top of that, Google's password-challenge page fingerprints the browser on click (JA3, canvas, audio, WebGL, navigator traits) and if any axis looks automated the flow stalls silently. We can't see which axis fires because Google returns the same DOM either way.

**Hypothesis ranking by expected lift** (based on public stealth research + arms-race chatter):

| # | Axis | Expected lift | Why |
|---|---|---|---|
| 1 | Cold profile (no history, no YouTube cookies) | +30-45pp | `checkConnection=youtube` is literally this check |
| 2 | Datacenter IP | +15-25pp | IP reputation is a strong prior for risk score |
| 3 | Robotic timing | +5-15pp | Uniform 50-180ms typing is a tell |
| 4 | Canvas/WebGL/audio fingerprint inconsistency | +0-10pp | Camoufox covers baseline; this is marginal |

Layers 1-4 in the brief map 1:1 to this ranking. Layer 5 (session reuse) is a long-term multiplier. Layer 6 (fingerprint check) is diagnostic only.

**What we can't fix from code:**
- 2FA on the target account (operator must disable or use `kiro_login.py` manual path)
- Google-flagged account (prior automation has raised its risk score to "always challenge")
- Brand-new accounts (no activity history at all; Google's "new risky account" prior)

These are documented honestly in the README, not engineered around.

---

## 2. Interface changes

### `tools/onboard/browser_driver.py`

**Before:** always calls `browser.new_context()` → fresh profile every run.

**After:** accepts `user_data_dir=<path>` and switches to `persistent_context=True` mode when a profile path is provided. Camoufox's sync API in persistent mode yields a **BrowserContext directly** (not a Browser); our wrapper transparently handles both. Also accepts `proxy={...}` kwarg (pass-through to Camoufox) and `disable_coop=True` by default (needed for challenge iframe clicks).

**New methods:**
- `warmup(flows: list)` — run a sequence of `(url, dwell_s)` tuples pre-Kiro-login to build session state.
- `human_type(selector, text)` — replaces `type_humanized`; burst+pause distribution, occasional typo+backspace.
- `human_pause(lo=500, hi=2000)` — random think-pause before submit clicks.
- `scan_challenge()` — returns `None` or a `ChallengeKind` enum indicating what's blocking us.

Backwards-compat: old `type_humanized` kept as deprecated alias forwarding to `human_type` so existing tests don't break.

### `tools/onboard/onboard.py`

**New flags:**
- `--proxy URL` — explicit override; otherwise reads `KIROXY_ONBOARD_PROXY` env.
- `--profile-dir PATH` — override auto-derived `profiles_data/<account-id>/` path.
- `--skip-warmup` — debugging; skip the pre-login warmup flow.
- `--challenge-mode {auto,manual,skip}` — default `auto`: detect+prompt; `manual`: pause for manual solve regardless; `skip`: ignore, G.1 behavior.

**Layered orchestration:**
```
   config → profile dir prep → proxy validation → warmup (unless skipped)
   → Kiro login URL → human-like email/password → challenge scan loop
   → redirect capture OR manual-solve prompt → token exchange → upsert JSON
```

Each layer logs a single line indicating its state so a live run is legible.

### New modules

- `tools/onboard/warmup.py` — defines `_DEFAULT_WARMUP` flow (youtube watch 60s → google weather search → github scroll → idle 30s). Each step is atomic; failures are logged but non-fatal (warmup is best-effort).
- `tools/onboard/challenge.py` — `ChallengeKind` enum + `detect(page) -> ChallengeKind|None` + `prompt_and_wait_for_solve(kind)` that blocks on stdin.
- `tools/onboard/human.py` — burst-pause typing distribution, typo injection, mouse drift helpers. Pure functions; no Camoufox dependency (stays testable).
- `tools/onboard/fingerprint_check.py` — standalone CLI: `python fingerprint_check.py [--proxy URL]` → opens browserscan.net + creepjs, prints top-level signals. Diagnostic only.
- `tools/onboard/fixtures/mock_kiro.py` — stdlib HTTP server that serves a minimal page with a button that redirects to `kiro://kiro.kiroAgent/authenticate-success?code=TEST_CODE_123&state=<state>`. Used by `test_onboard_nogoogle.py` to verify the non-Google portions of the flow work end-to-end.

---

## 3. Layer-by-layer design

### Layer 1 — Warm profile persistence (biggest impact)

**Storage model:** `tools/onboard/profiles_data/<sha256(email)[:12]>/` (content-addressed directory; prevents email leakage in path while being stable).

**Warmup flow (first run only):**
1. Navigate `https://www.youtube.com` — dwell 45s (lets YouTube SPA settle, sets visitor cookie).
2. Navigate `https://www.google.com/search?q=weather` — dwell 15s.
3. Navigate `https://github.com` — dwell 10s.
4. Idle 15s on a blank page.

Total ≈ 85s. Bailout after 180s hard cap — if warmup itself stalls, don't block main flow; proceed with whatever cookies got set.

**Marker file:** `profiles_data/<id>/.warmed-at` stores unix timestamp. Skip warmup if < 7 days old. Re-warm if older; long-dormant sessions look as suspicious as cold ones.

### Layer 2 — Residential proxy support

**Env:** `KIROXY_ONBOARD_PROXY` → `http://user:pass@host:port` or `socks5://...`. CLI flag `--proxy` overrides.

**Validation:** before launching Camoufox, a quick `httpx.get("https://api.ipify.org")` through the proxy to confirm it responds and to capture the egress IP. Egress IP is passed as `geoip=<IP>` to Camoufox so timezone/locale match.

**Fallback:** if no proxy set, print a single `warn:` line: `residential proxy unset; Google success rate may drop to <40%`. Proceed anyway — G.1 fallback behavior.

**Hardcoded anti-pattern:** do NOT fetch free proxy lists. Do NOT ship a default proxy. Operator provides their own paid residential proxy.

### Layer 3 — Human-like interaction

**Typing distribution** (`human.py::burst_pause_delays(n)`):
- Bursts of 3-5 chars at 40-90ms inter-char.
- Between bursts: 250-600ms pause.
- 1-2% per-char typo probability: type wrong-key → pause 120-300ms → backspace → type correct.
- Typo injection capped at 2 typos total per text (operators don't want three typos in a 10-char password).

**Mouse drift** (`human.py::drift_cursor(page, to_selector)`):
- Resolve target element bounds via `locator.bounding_box()`.
- Move mouse in 6-10 steps via `page.mouse.move(x, y, steps=n)` along a curved path (Bezier-ish: linear with perpendicular jitter).
- Dwell briefly (100-300ms) before emitting click.

**Read pause** (`human.py::read_pause(field_chars)`):
- Before clicking Next/Submit: 500ms base + 30ms/char user typed + 0-800ms random.

**Verification strategy:** `test_human.py` runs each distribution 2000 times and asserts statistical properties (mean, p5, p95, typo rate bounded). No browser dependency.

### Layer 4 — Challenge detection + manual-solve recovery

**Detection patterns (polled every 1s for 60s after password submit):**

| ChallengeKind | Detection signal |
|---|---|
| `VERIFY_ITS_YOU` | text: "Verify it's you" |
| `RECAPTCHA` | iframe `src*="google.com/recaptcha"` OR `title*="reCAPTCHA"` |
| `DEVICE_APPROVAL` | text: "Check your phone" OR "open the Google app" |
| `UNUSUAL_ACTIVITY` | text: "unusual activity" OR "couldn't verify" |
| `TWO_FA_CODE` | `input[type="tel"]` OR text: "2-Step Verification" OR "enter the code" |
| `BLOCKED` | text: "sign-in blocked" OR "couldn't sign you in" → HARD failure, don't prompt |
| `CONNECTION_CHECK` | URL contains `checkConnection=` AND dwell > 15s — the one we're actually hitting |

Text matches are case-insensitive, OR'd across localized equivalents (en/id patterns at minimum; kikirro's `GOOGLE_BLOCK_PHRASES` list reused).

**Recovery flows by mode:**

- `--challenge-mode auto` (default):
  - `BLOCKED` → fail immediately; save screenshot; suggest `kiro_login.py`.
  - Any other `ChallengeKind` → print the prompt:
    ```
    ⚠ Google challenge detected: TWO_FA_CODE
      Solve it in the browser window, then press ENTER to continue.
    ```
    Block on stdin (or timeout after 600s = 10 min).
  - Post-press: resume the `kiro://` redirect wait.

- `--challenge-mode manual`: skip auto-typing entirely after navigating to the login URL. Print "solve manually". This is identical to `kiro_login.py` behavior in-process. Use for accounts you know will challenge.

- `--challenge-mode skip`: G.1 behavior. For accounts/proxies you're confident don't challenge.

**State mismatch is NOT a challenge.** It's already a warn-only log in G.1; leave that alone.

### Layer 5 — Session reuse (long-term)

No code change beyond Layer 1 (profile dirs already persist). Documented in README: "don't delete `profiles_data/` unless you want to redo warmups."

Note: the refresh_token path (`kiroxy` internals) doesn't depend on this. It only helps when a *future* re-onboard is needed.

### Layer 6 — Fingerprint diagnostic tool

`fingerprint_check.py`:
- Launch Camoufox with the same config `onboard.py` would use (profile dir + proxy + geoip).
- Visit `https://bot.sannysoft.com` → screenshot to `screenshots/sannysoft_<ts>.png`.
- Visit `https://abrahamjuliot.github.io/creepjs` → wait for fingerprint compute (~15s) → grab the trust score text.
- Print: `sannysoft: <failures_count>  creepjs_trust: <score>  egress_ip: <ip>  webgl: <renderer>`.

No automated pass/fail. Operator reads output and decides if config needs adjusting.

---

## 4. Test strategy without live Google

**Unit-testable layers (3/6):**
- `test_human.py` — distributions, typo rate, read-pause bounds, drift point count (statistical assertions over 2000 samples each).
- `test_challenge.py` — feed static HTML strings to a `detect_from_html(html)` helper that mirrors `detect(page)`. Covers all 7 `ChallengeKind` values plus "no challenge present".
- `test_warmup.py` — mock a `MockBrowserDriver` with `navigate()` + `wait()` recorders; assert `_DEFAULT_WARMUP` calls them in the right order with dwell times in the right bands.
- `test_oauth.py` (existing, 14 tests) — unchanged, still passes.

**Integration test with mock:**
- `test_onboard_mock.py` launches `mock_kiro.py` on an ephemeral port, then runs `onboard.main()` with `--headless` (mock server uses plain HTML) pointed at the mock URL (via a new internal `KIROXY_ONBOARD_MOCK_LOGIN_URL` env for tests only). Verifies PKCE + callback extraction + JSON upsert work when Google is bypassed.
- This exercises 70% of `onboard.py` without needing Camoufox, using Python stdlib `http.server`.

**What we can't test:**
- Actual Google drive. Documented in `TESTING.md` as manual checklist the operator runs against a live Gmail account.
- Proxy validation against a real residential proxy. Operator tests.
- Camoufox persistent_context actually persisting. Operator tests (run twice, see no warmup the second time).

---

## 5. Atomic commit plan

1. `feat(onboard): warm profile persistence per account` — browser_driver + warmup module + profile dir derivation + marker file.
2. `feat(onboard): residential proxy support via env var` — proxy flag, env read, egress validation, geoip pass-through.
3. `feat(onboard): human-like interaction patterns` — human.py + drv.human_type/human_pause/drift_cursor + deprecation alias.
4. `feat(onboard): challenge detection with manual-solve recovery` — challenge.py + onboard.py integration + 3-mode CLI flag.
5. `feat(onboard): fingerprint diagnostic tool` — fingerprint_check.py.
6. `test(onboard): mock-kiro integration fixture and per-layer tests` — fixtures/mock_kiro.py + 3 new test files + layer-specific tests.
7. `docs(onboard): honest reliability documentation + testing protocol` — README rewrite + TESTING.md.
8. `chore(onboard): pin Python deps to exact versions` — requirements.txt with pins.
9. `docs(phase-g-fix): OVERNIGHT_LOG entry + BACKLOG updates` — close-out.

---

## 6. Reliability expectations (for operator)

- **Fresh Gmail, fresh Kiro session, residential proxy, warmup completed:** 65-80% first-attempt success.
- **Previously-automated account:** 30-50%. Re-warming a cold profile helps less once Google's risk score is sticky.
- **Account with 2FA:** 0% full-auto. `--challenge-mode auto` will prompt for code entry; effectively manual-assist.
- **No proxy, fresh account:** 25-45%. Rate-limited by Google's datacenter-IP prior.
- **No proxy, previously-automated account:** 5-15%. Expect to fall back to `kiro_login.py`.

These numbers are estimates, not guarantees. Google's models update weekly; what works today may stop working tomorrow. That is in the README verbatim.

---

## 7. What this does NOT solve

- Google deciding to require 2FA mid-flow on an account that never had it.
- Google showing a captcha Camoufox can't click (disable_coop helps; not a guarantee).
- Kiro changing its auth endpoint (unrelated; would need new code everywhere).
- refresh_token revocation (kiroxy core concern, not ours).

All these fall through to operator running `kiro_login.py` manually.
