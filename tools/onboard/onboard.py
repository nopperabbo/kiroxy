#!/usr/bin/env python3
"""kiroxy onboarder — single-account Kiro Desktop OAuth (Phase G / G.FIX).

Automates the Kiro Desktop authorization flow:

    (1) (optional) warm up a persistent Camoufox profile so Google sees
        session state (YouTube, google.com, github.com) before login
    (2) generate PKCE triple
    (3) open Camoufox at Kiro's /login with idp=Google|Github
    (4) drive through Google/GitHub sign-in with human-like typing
    (5) if Google shows a challenge, pause and let the operator solve it
    (6) intercept kiro://…?code=…&state=… redirect
    (7) POST /oauth/token → collect {accessToken, refreshToken, profileArn, expiresIn}
    (8) upsert result into output JSON compatible with `kiroxy import-accounts-json`

Security posture:
  * Passwords never written to disk, never logged.
  * --password - reads stdin (single line) to avoid process-list exposure.
  * Error messages strip passwords if the input ever appears in a string.
  * Failure screenshots in ./screenshots/ are .gitignore'd.

Reliability posture (Phase G.FIX):
  * This tool is BEST-EFFORT against Google SSO. Expect 40-70% success on
    fresh accounts with a residential proxy + warmed profile; lower without.
  * When Google shows a challenge (reCAPTCHA, 2FA, "verify it's you"), the
    script prompts the operator to solve it manually, then resumes.
  * For accounts with active 2FA or heavy prior automation, prefer manually
    signing in via `kiro_login.py` and importing the resulting token.

Use only with accounts you own. Google automation may violate TOS.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import re
import sys
import time
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Dict, List, Optional

from kiro_oauth import (  # noqa: E402
    TokenExchangeError,
    build_login_url,
    exchange_code,
    generate_pkce,
    parse_callback_url,
)

BASE_DIR = Path(__file__).resolve().parent
DEFAULT_OUTPUT = BASE_DIR / "kiro_tokens.json"
PROFILES_PATH = BASE_DIR / "profiles.json"
PROFILES_DATA_DIR = BASE_DIR / "profiles_data"
SCREENSHOT_DIR = BASE_DIR / "screenshots"

# ──────────────────────────────────────────────────────────────────────────────
# Google sign-in selectors — kept wide to tolerate Google's A/B variants and
# locale changes. Order matters: earlier entries are more specific / reliable.
# ──────────────────────────────────────────────────────────────────────────────

_GOOGLE_EMAIL_SELECTORS = [
    'input[type="email"]',
    'input[name="identifier"]',
    'input#identifierId',
]
_GOOGLE_EMAIL_NEXT_SELECTORS = [
    "#identifierNext button",
    "#identifierNext",
    'button:has-text("Next")',
    'button:has-text("Berikutnya")',  # id
]
_GOOGLE_PASSWORD_SELECTORS = [
    'input[type="password"]',
    'input[name="Passwd"]',
    'input[name="password"]',
]
_GOOGLE_PASSWORD_NEXT_SELECTORS = [
    "#passwordNext button",
    "#passwordNext",
    'button:has-text("Next")',
    'button:has-text("Berikutnya")',
]
_CONSENT_SELECTORS = [
    'button:has-text("Continue")',
    'button:has-text("Allow")',
    'button:has-text("Lanjutkan")',
    'button:has-text("Izinkan")',
    '[role="button"]:has-text("Continue")',
]
_COOKIE_DISMISS_SELECTORS = [
    'button:has-text("Accept all")',
    'button:has-text("Accept All")',
    'button:has-text("I agree")',
    '[aria-label*="accept" i]',
]

# GitHub — minimal selectors; GitHub login is simpler (email+password on one page).
_GH_EMAIL_SELECTOR = 'input[name="login"]'
_GH_PASSWORD_SELECTOR = 'input[name="password"]'
_GH_SUBMIT_SELECTOR = 'input[type="submit"][value*="Sign in"], button[type="submit"]:has-text("Sign in")'
_GH_AUTHORIZE_SELECTOR = 'button:has-text("Authorize"), input[type="submit"][value*="Authorize"]'

# ──────────────────────────────────────────────────────────────────────────────
# Helpers
# ──────────────────────────────────────────────────────────────────────────────


def _log(msg: str) -> None:
    ts = time.strftime("%H:%M:%S")
    print(f"[{ts}] {msg}", flush=True)


def _safe_slug(s: str) -> str:
    return re.sub(r"[^A-Za-z0-9._-]", "_", s)


def _now_iso() -> str:
    return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def _account_id(email: str) -> str:
    """Stable content-addressed identifier for an account.

    Used for profile dir name (`profiles_data/<id>/`) so the email
    never appears on disk. 12 hex chars = 48 bits, collision-safe at
    human scale.
    """
    return hashlib.sha256(email.lower().encode("utf-8")).hexdigest()[:12]


def _derive_profile_dir(email: str, override: Optional[str]) -> Path:
    if override:
        return Path(override).expanduser().resolve()
    return (PROFILES_DATA_DIR / _account_id(email)).resolve()


def _load_profiles() -> List[Dict[str, Any]]:
    try:
        data = json.loads(PROFILES_PATH.read_text(encoding="utf-8"))
        profiles = data.get("profiles", []) if isinstance(data, dict) else []
        return [p for p in profiles if isinstance(p, dict)]
    except Exception as e:
        _log(f"warn: could not load profiles.json ({e}); using built-in default")
        return []


_DEFAULT_PROFILE: Dict[str, Any] = {
    "id": "default",
    "platform": "macOS",
    "user_agent": (
        "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) "
        "AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36"
    ),
    "viewport": {"width": 1920, "height": 1080},
    "locale": "en-US",
    "timezone_id": "America/Los_Angeles",
    "accept_language": "en-US,en;q=0.9",
}


def _pick_profile(email: str, override_id: Optional[str]) -> Dict[str, Any]:
    profiles = _load_profiles()
    if override_id:
        for p in profiles:
            if p.get("id") == override_id:
                return p
        _log(f"warn: profile id {override_id!r} not found; falling back to hash pick")
    if not profiles:
        return _DEFAULT_PROFILE
    digest = hashlib.sha256(email.lower().encode("utf-8")).digest()
    idx = int.from_bytes(digest[:8], "big") % len(profiles)
    return profiles[idx]


def _read_password(raw: str) -> str:
    if raw != "-":
        return raw
    if sys.stdin.isatty():
        try:
            import getpass
            return getpass.getpass("password: ")
        except Exception:
            pass
    line = sys.stdin.readline()
    if not line:
        raise SystemExit("error: --password - but no input on stdin")
    return line.rstrip("\r\n")


def _redact_password(msg: str, password: str) -> str:
    if not password:
        return msg
    return msg.replace(password, "[REDACTED]")


def _is_kiro_callback(url: str) -> bool:
    return url.startswith("kiro://") and "code=" in url


# ──────────────────────────────────────────────────────────────────────────────
# Provider flows
# ──────────────────────────────────────────────────────────────────────────────


def _dismiss_cookie_banner(drv) -> None:
    for sel in _COOKIE_DISMISS_SELECTORS:
        try:
            loc = drv.page.locator(sel).first
            if loc.is_visible(timeout=400):
                loc.click(timeout=1500)
                drv.page.wait_for_timeout(300)
                return
        except Exception:
            continue


def _try_selectors_click(drv, selectors: list, timeout_ms: int = 1500) -> bool:
    for sel in selectors:
        try:
            loc = drv.page.locator(sel).first
            if loc.is_visible(timeout=timeout_ms // len(selectors)):
                loc.click(timeout=timeout_ms)
                return True
        except Exception:
            continue
    return False


def _try_selectors_fill_humanized(drv, selectors: list, text: str) -> bool:
    for sel in selectors:
        try:
            loc = drv.page.locator(sel).first
            if loc.is_visible(timeout=1500):
                drv.human_type(sel, text)
                return True
        except Exception:
            continue
    return False


def _drive_google(drv, email: str, password: str) -> None:
    _log("→ waiting for Google identifier field")
    drv.wait_for_selector(_GOOGLE_EMAIL_SELECTORS[0], timeout_ms=30_000)

    _dismiss_cookie_banner(drv)

    _log("→ typing email (humanized)")
    if not _try_selectors_fill_humanized(drv, _GOOGLE_EMAIL_SELECTORS, email):
        raise RuntimeError("Google email field not fillable")

    # Read-pause before click (simulate human reading the form).
    drv.human_pause(lo_ms=500, hi_ms=1500)

    if not _try_selectors_click(drv, _GOOGLE_EMAIL_NEXT_SELECTORS, timeout_ms=4000):
        drv.page.keyboard.press("Enter")

    _log("→ waiting for password field")
    try:
        drv.wait_for_selector(_GOOGLE_PASSWORD_SELECTORS[0], timeout_ms=30_000)
    except Exception as e:
        raise RuntimeError(f"password field did not appear: {e}") from e

    _log("→ typing password (humanized)")
    if not _try_selectors_fill_humanized(drv, _GOOGLE_PASSWORD_SELECTORS, password):
        raise RuntimeError("password field not fillable")

    drv.human_pause(lo_ms=800, hi_ms=2000)

    if not _try_selectors_click(drv, _GOOGLE_PASSWORD_NEXT_SELECTORS, timeout_ms=4000):
        drv.page.keyboard.press("Enter")

    _log("→ password submitted; checking for consent screen")
    drv.page.wait_for_timeout(2500)
    _try_selectors_click(drv, _CONSENT_SELECTORS, timeout_ms=3000)


def _drive_github(drv, email: str, password: str) -> None:
    _log("→ waiting for GitHub login form")
    drv.wait_for_selector(_GH_EMAIL_SELECTOR, timeout_ms=30_000)
    _log("→ typing email (humanized)")
    drv.human_type(_GH_EMAIL_SELECTOR, email)
    drv.human_pause(lo_ms=300, hi_ms=900)
    _log("→ typing password (humanized)")
    drv.human_type(_GH_PASSWORD_SELECTOR, password)
    drv.human_pause(lo_ms=500, hi_ms=1500)
    drv.click(_GH_SUBMIT_SELECTOR, timeout_ms=10_000)
    _log("→ sign-in submitted; handling authorize screen if present")
    drv.page.wait_for_timeout(2500)
    try:
        drv.click(_GH_AUTHORIZE_SELECTOR, timeout_ms=3000)
    except Exception:
        pass


# ──────────────────────────────────────────────────────────────────────────────
# Output file upsert
# ──────────────────────────────────────────────────────────────────────────────


def _load_existing(path: Path) -> List[Dict[str, Any]]:
    if not path.exists():
        return []
    try:
        data = json.loads(path.read_text(encoding="utf-8"))
    except Exception as e:
        raise SystemExit(f"error: existing output file {path} is not valid JSON: {e}")
    if not isinstance(data, list):
        raise SystemExit(f"error: existing output file {path} is not a JSON array")
    return [e for e in data if isinstance(e, dict)]


def _dedupe_key(entry: Dict[str, Any]) -> str:
    """Return a stable per-account key.

    Priority (matches Go deriveAccountID cascade):
      1. entry.email (normalized lowercase)
      2. JWT 'email' or 'sub' claim on the access token (defensive fallback;
         today's Kiro tokens are opaque, so this layer is inert but kept
         for parity with the Go side and future token-shape changes)
      3. last segment of entry.profileArn
      4. first 12 chars of entry.accessToken

    Why this order: Google Workspace accounts share a profileArn across
    users in the same org, so profileArn alone cannot distinguish users.
    Email is authoritative because it comes from the operator's CLI
    invocation. Legacy JSON files without 'email' still dedupe sensibly
    via the fallback layers.
    """
    email = (entry.get("email") or "").strip().lower()
    if email:
        return f"email:{email}"
    from kiro_oauth import jwt_sub_or_email  # local import to avoid bootstrap cycle
    claim = jwt_sub_or_email((entry.get("accessToken") or "").strip())
    if claim:
        return f"jwt:{claim.lower()}"
    arn = (entry.get("profileArn") or "").strip()
    if arn:
        seg = arn.rsplit("/", 1)[-1]
        return f"arn:{seg}" if seg else f"arn:{arn}"
    at = (entry.get("accessToken") or "").strip()
    return f"at:{at[:12]}" if at else ""


def _upsert(entries: List[Dict[str, Any]], new: Dict[str, Any]) -> str:
    key = _dedupe_key(new)
    if not key:
        entries.append(new)
        return "added"
    for i, e in enumerate(entries):
        if _dedupe_key(e) == key:
            entries[i] = new
            return "updated"
    entries.append(new)
    return "added"


def _atomic_write(path: Path, entries: List[Dict[str, Any]]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    tmp = path.with_suffix(path.suffix + ".tmp")
    tmp.write_text(json.dumps(entries, indent=2) + "\n", encoding="utf-8")
    try:
        os.chmod(tmp, 0o600)
    except Exception:
        pass
    tmp.replace(path)


# ──────────────────────────────────────────────────────────────────────────────
# Main flow
# ──────────────────────────────────────────────────────────────────────────────


def _parse_args(argv: Optional[List[str]] = None) -> argparse.Namespace:
    p = argparse.ArgumentParser(
        prog="onboard",
        description="Kiro Desktop OAuth token acquisition (single account, best-effort auto).",
        epilog=(
            "Example:\n"
            "  python onboard.py --email you@gmail.com --password - \\\n"
            "    --provider google --output ../../kiro_tokens.json"
        ),
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )
    p.add_argument("--email", required=True, help="account email")
    p.add_argument(
        "--password", required=True,
        help="password (use '-' to read one line from stdin; avoids `ps` exposure)",
    )
    p.add_argument(
        "--provider", choices=["google", "github"], default="google",
        help="social IDP (default: google)",
    )
    p.add_argument(
        "--output", default=str(DEFAULT_OUTPUT),
        help="output JSON path; appends if file exists (default: ./kiro_tokens.json)",
    )
    p.add_argument(
        "--profile-id", default=None,
        help="override profile id from profiles.json (default: hash of email)",
    )
    p.add_argument(
        "--profile-dir", default=None,
        help="override Camoufox user_data_dir (default: profiles_data/<account-id>/)",
    )
    p.add_argument(
        "--skip-warmup", action="store_true",
        help="skip the YouTube/Google/GitHub warmup flow (debugging only; reduces success rate)",
    )
    p.add_argument(
        "--proxy", default=None,
        help=(
            "residential proxy URL (http/https/socks5 with optional user:pass); "
            "overrides KIROXY_ONBOARD_PROXY env var. Unset = direct connection "
            "(Google success rate may drop; see README)."
        ),
    )
    p.add_argument(
        "--headless", action="store_true",
        help="run Camoufox headless (default: windowed, for debug visibility)",
    )
    p.add_argument(
        "--challenge-mode", choices=["auto", "manual", "skip"], default="auto",
        help=(
            "how to handle Google challenges: "
            "'auto' = detect + prompt operator to solve (default); "
            "'manual' = skip auto-type entirely, wait for operator to sign in; "
            "'skip' = G.1 behavior (type and hope)."
        ),
    )
    p.add_argument(
        "--timeout-login-s", type=int, default=120,
        help="seconds to wait for the kiro:// redirect (default: 120)",
    )
    return p.parse_args(argv)


def main(argv: Optional[List[str]] = None) -> int:
    args = _parse_args(argv)
    password = _read_password(args.password)
    if not password:
        print("error: password must not be empty", file=sys.stderr)
        return 1

    output_path = Path(args.output).expanduser().resolve()
    profile_dir = _derive_profile_dir(args.email, args.profile_dir)

    # Resolve and validate residential proxy (CLI flag > env > none).
    from proxy_support import resolve_proxy, validate_egress, ProxyConfigError

    proxy_dict: Optional[Dict[str, str]] = None
    proxy_geoip: Optional[Any] = None
    try:
        proxy_cfg = resolve_proxy(args.proxy)
    except ProxyConfigError as e:
        print(f"error: invalid proxy: {e}", file=sys.stderr)
        return 1

    if proxy_cfg is None:
        _log(
            "warn: residential proxy unset "
            "(KIROXY_ONBOARD_PROXY or --proxy); "
            "Google success rate may drop to <40%"
        )
    else:
        _log(f"proxy: {proxy_cfg.server} (validating egress…)")
        ok, detail = validate_egress(proxy_cfg)
        if not ok:
            print(
                f"error: proxy egress validation failed: {detail}\n"
                f"  proxy: {proxy_cfg.server}",
                file=sys.stderr,
            )
            return 1
        _log(f"proxy: ok (egress IP {detail})")
        proxy_dict = proxy_cfg.as_camoufox_dict()
        proxy_geoip = detail  # let Camoufox derive tz/locale from this IP

    profile = _pick_profile(args.email, args.profile_id)
    _log(
        f"profile={profile.get('id')} platform={profile.get('platform')} "
        f"locale={profile.get('locale')} tz={profile.get('timezone_id')}"
    )
    _log(f"profile_dir={profile_dir}")

    verifier, challenge, state = generate_pkce()
    login_url = build_login_url(args.provider, challenge, state)
    _log(f"login URL built (provider={args.provider}, state={state[:8]}…)")

    from browser_driver import BrowserDriver, BrowserDriverUnavailableError
    from warmup import run_warmup, should_warmup, write_marker
    from challenge import (
        ChallengeMode, is_hard_fail, poll_for_challenge, prompt_and_wait_for_solve,
    )

    screenshot_slug = _safe_slug(args.email)
    screenshot_base = SCREENSHOT_DIR / f"onboard_{screenshot_slug}_{int(time.time())}"

    warmup_needed = (not args.skip_warmup) and should_warmup(profile_dir)
    if args.skip_warmup:
        _log("warmup: skipped (--skip-warmup)")
    elif not warmup_needed:
        _log("warmup: skipped (profile recently warmed)")
    else:
        _log("warmup: profile cold or stale; will warm before login")

    challenge_mode = ChallengeMode(args.challenge_mode)
    _log(f"challenge-mode: {challenge_mode.value}")

    try:
        with BrowserDriver(
            profile=profile,
            headless=args.headless,
            user_data_dir=str(profile_dir),
            proxy=proxy_dict,
            geoip=proxy_geoip,
        ) as drv:
            if warmup_needed:
                completed, attempted = run_warmup(drv, log=_log)
                if completed > 0:
                    write_marker(profile_dir)
                else:
                    _log("warmup: 0 steps completed; continuing without marker update")

            _log("→ opening Kiro /login")
            drv.navigate(login_url, wait_until="domcontentloaded")
            drv.page.wait_for_timeout(800)  # let SPA hydrate

            if challenge_mode == ChallengeMode.MANUAL:
                # Don't auto-type anything. Just prompt once and wait for the
                # operator to sign in manually. Same UX as kiro_login.py, but
                # in-tree.
                _log("manual mode: not typing credentials; waiting for operator sign-in")
                print(
                    "\n⚠ Manual sign-in mode. Please complete sign-in in the browser "
                    "window, then press ENTER to continue.\n",
                    file=sys.stderr, flush=True,
                )
                try:
                    sys.stdin.readline()
                except Exception:
                    pass
            else:
                # AUTO or SKIP: drive the flow.
                if args.provider == "google":
                    _drive_google(drv, args.email, password)
                else:
                    _drive_github(drv, args.email, password)

                if challenge_mode == ChallengeMode.AUTO:
                    # Poll up to 60s for a challenge. If detected:
                    #   BLOCKED → abort immediately.
                    #   Others → prompt, wait, resume.
                    _log("→ scanning for Google challenges (up to 60s)")
                    kind = poll_for_challenge(drv.page, timeout_s=60)
                    if kind is not None:
                        _log(f"challenge detected: {kind.value}")
                        if is_hard_fail(kind):
                            # Save a screenshot of the blocked state for context.
                            SCREENSHOT_DIR.mkdir(parents=True, exist_ok=True)
                            shot = drv.screenshot(
                                str(screenshot_base) + "_blocked.png"
                            )
                            print(
                                f"error: Google hard-blocked sign-in ({kind.value}). "
                                f"This account is cooked on this IP/profile. "
                                f"Try a different account or residential proxy, or use "
                                f"kiro_login.py for manual sign-in.",
                                file=sys.stderr,
                            )
                            if shot:
                                print(f"debug screenshot: {shot}", file=sys.stderr)
                            return 1
                        ok = prompt_and_wait_for_solve(kind)
                        if not ok:
                            print(
                                f"error: aborted waiting for operator to solve {kind.value}",
                                file=sys.stderr,
                            )
                            return 1
                        _log("operator signalled resume; waiting for redirect")
                    else:
                        _log("no challenge detected within 60s; redirect should arrive")

            _log(f"→ waiting up to {args.timeout_login_s}s for kiro:// redirect")
            callback_url = drv.wait_for_navigation_matching(
                _is_kiro_callback, timeout_s=args.timeout_login_s
            )
            _log(f"✓ captured callback URL (len={len(callback_url)})")

    except BrowserDriverUnavailableError as e:
        print(f"error: {e}", file=sys.stderr)
        return 1
    except Exception as e:
        shot = None
        try:
            if "drv" in locals() and drv is not None:  # type: ignore[name-defined]
                SCREENSHOT_DIR.mkdir(parents=True, exist_ok=True)
                shot = drv.screenshot(str(screenshot_base) + "_fail.png")  # type: ignore[name-defined]
        except Exception:
            pass
        redacted = _redact_password(str(e), password)
        print(f"error: browser flow failed: {redacted}", file=sys.stderr)
        if shot:
            print(f"debug screenshot: {shot}", file=sys.stderr)
        return 1

    try:
        code, returned_state = parse_callback_url(callback_url)
    except ValueError as e:
        print(f"error: {e}", file=sys.stderr)
        return 1

    if returned_state != state:
        _log(f"warn: state mismatch (sent={state[:8]}… got={returned_state[:8]}…)")

    _log("→ exchanging code for tokens")
    try:
        resp = exchange_code(code, verifier)
    except TokenExchangeError as e:
        print(f"error: {_redact_password(str(e), password)}", file=sys.stderr)
        return 1

    access_token = resp.get("accessToken") or ""
    refresh_token = resp.get("refreshToken") or ""
    profile_arn = resp.get("profileArn") or ""
    expires_in = int(resp.get("expiresIn") or 0)

    if not (access_token and refresh_token and profile_arn):
        print(
            "error: token response missing required fields "
            f"(accessToken={'ok' if access_token else 'MISSING'}, "
            f"refreshToken={'ok' if refresh_token else 'MISSING'}, "
            f"profileArn={'ok' if profile_arn else 'MISSING'})",
            file=sys.stderr,
        )
        return 1

    provider_cap = "Google" if args.provider == "google" else "Github"
    entry: Dict[str, Any] = {
        "provider": provider_cap,
        "authMethod": "social",
        "email": args.email.strip().lower(),
        "accessToken": access_token,
        "refreshToken": refresh_token,
        "profileArn": profile_arn,
        "expiresIn": expires_in,
        "addedAt": _now_iso(),
    }

    entries = _load_existing(output_path)
    action = _upsert(entries, entry)
    _atomic_write(output_path, entries)

    arn_tail = profile_arn.rsplit("/", 1)[-1] or profile_arn
    _log(
        f"✓ {action} account {args.email} "
        f"(profile={arn_tail}, expiresIn={expires_in}s)"
    )
    print(f"Output: {output_path}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
