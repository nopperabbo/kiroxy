"""
Challenge detection + manual-solve recovery.

Phase G.FIX Layer 4.

When Google suspects automation, it injects one of a handful of challenge
flows. We can't (and should not try to) solve them automatically; instead
we detect them, tell the operator which one, and wait on stdin while they
solve it in the browser window.

The `detect()` function returns a `ChallengeKind` or None. It takes a
Playwright page; for unit testing without a browser, `detect_from_html()`
takes a raw HTML string + URL.

Detection order matters: BLOCKED is checked first because it's the only
hard-fail signal (no recovery possible). Everything else presumes the
operator can intervene.

Recovery modes:

  * ``ChallengeMode.AUTO`` (default) — auto-type; on detection, prompt &
    wait; resume after operator solves.
  * ``ChallengeMode.MANUAL`` — skip auto-type entirely; print prompt, wait
    for operator to complete login by hand; resume.
  * ``ChallengeMode.SKIP`` — G.1 behavior: type and pray.
"""

from __future__ import annotations

import enum
import sys
import time
from typing import Optional


class ChallengeKind(str, enum.Enum):
    VERIFY_ITS_YOU = "verify_its_you"
    RECAPTCHA = "recaptcha"
    DEVICE_APPROVAL = "device_approval"
    UNUSUAL_ACTIVITY = "unusual_activity"
    TWO_FA_CODE = "two_fa_code"
    BLOCKED = "blocked"
    CONNECTION_CHECK = "connection_check"


class ChallengeMode(str, enum.Enum):
    AUTO = "auto"
    MANUAL = "manual"
    SKIP = "skip"


# Case-insensitive text patterns. Each pattern is (kind, [phrases]).
# Order matters: earlier entries take precedence. BLOCKED first.
# Phrases are checked with substring match on lowercased body text.
_TEXT_PATTERNS: list[tuple[ChallengeKind, tuple[str, ...]]] = [
    (ChallengeKind.BLOCKED, (
        "sign-in blocked",
        "couldn't sign you in",
        "account has been disabled",
        "akun anda dinonaktifkan",
        "this account has been temporarily disabled",
        "akun anda telah dinonaktifkan sementara",
    )),
    (ChallengeKind.RECAPTCHA, (
        "recaptcha",
    )),
    (ChallengeKind.TWO_FA_CODE, (
        "2-step verification",
        "verifikasi 2 langkah",
        "enter the code",
        "masukkan kode",
    )),
    (ChallengeKind.DEVICE_APPROVAL, (
        "check your phone",
        "cek ponsel anda",
        "open the google app",
        "buka aplikasi google",
        "tap yes on your phone",
    )),
    (ChallengeKind.VERIFY_ITS_YOU, (
        "verify it's you",
        "confirm your identity",
        "konfirmasi identitas",
        "verifikasi identitas",
    )),
    (ChallengeKind.UNUSUAL_ACTIVITY, (
        "unusual activity",
        "couldn't verify",
        "aktivitas tidak biasa",
        "suspicious sign-in",
    )),
]

_RECAPTCHA_IFRAME_MARKERS = (
    'google.com/recaptcha',
    'title="recaptcha"',
    'title=\'recaptcha\'',
)

CONNECTION_CHECK_URL_MARKER = "checkconnection="
CONNECTION_CHECK_STALL_S = 15.0


def detect_from_html(
    html: str,
    url: str = "",
    seconds_on_current_url: float = 0.0,
) -> Optional[ChallengeKind]:
    """Pure-function detector for offline / unit-test use.

    `seconds_on_current_url` is the dwell time we've been stuck on `url`;
    CONNECTION_CHECK is only flagged after CONNECTION_CHECK_STALL_S so we
    don't misclassify the normal in-transit challenge page.
    """
    low_html = (html or "").lower()
    low_url = (url or "").lower()

    # BLOCKED first (hard fail)
    for kind, phrases in _TEXT_PATTERNS:
        for ph in phrases:
            if ph in low_html:
                return kind

    # reCAPTCHA iframe heuristic (text-only misses it sometimes)
    for marker in _RECAPTCHA_IFRAME_MARKERS:
        if marker in low_html:
            return ChallengeKind.RECAPTCHA

    # CONNECTION_CHECK: URL-based; only after a stall.
    if CONNECTION_CHECK_URL_MARKER in low_url and seconds_on_current_url >= CONNECTION_CHECK_STALL_S:
        return ChallengeKind.CONNECTION_CHECK

    return None


def detect(page, seconds_on_current_url: float = 0.0) -> Optional[ChallengeKind]:
    """Playwright-backed detector.

    Reads ``document.body.innerText`` and ``document.documentElement.outerHTML``
    plus the page URL. Returns None on any error (detection must never raise
    into the main flow).
    """
    try:
        html = page.evaluate(
            "() => (document.body ? document.body.innerText : '')"
        ) or ""
        # Additionally grab iframes' outerHTML for reCAPTCHA marker — innerText
        # excludes markup so we wouldn't see src attributes otherwise.
        outer = page.evaluate(
            "() => document.documentElement ? document.documentElement.outerHTML : ''"
        ) or ""
        combined = html + "\n" + outer
        current_url = page.url or ""
    except Exception:
        return None
    return detect_from_html(combined, current_url, seconds_on_current_url)


def poll_for_challenge(
    page,
    timeout_s: int = 60,
    poll_interval_s: float = 1.0,
) -> Optional[ChallengeKind]:
    """Poll the page for a challenge for up to timeout_s.

    Tracks time-on-URL for CONNECTION_CHECK detection. Returns the first
    detected kind, or None on timeout.
    """
    deadline = time.monotonic() + timeout_s
    last_url = ""
    url_enter_ts = time.monotonic()
    while time.monotonic() < deadline:
        current_url = ""
        try:
            current_url = page.url or ""
        except Exception:
            pass
        if current_url != last_url:
            last_url = current_url
            url_enter_ts = time.monotonic()
        stall_s = time.monotonic() - url_enter_ts
        kind = detect(page, seconds_on_current_url=stall_s)
        if kind is not None:
            return kind
        time.sleep(poll_interval_s)
    return None


# ── Recovery prompts ─────────────────────────────────────────────────────


_KIND_MESSAGES = {
    ChallengeKind.VERIFY_ITS_YOU: "Google wants you to confirm your identity.",
    ChallengeKind.RECAPTCHA: "Google is showing a reCAPTCHA. Solve it in the window.",
    ChallengeKind.DEVICE_APPROVAL: "Google sent an approval prompt to your phone. Tap Yes, then come back.",
    ChallengeKind.UNUSUAL_ACTIVITY: "Google thinks the sign-in is unusual; verify in the window.",
    ChallengeKind.TWO_FA_CODE: "Google wants a 2FA code. Enter it in the window.",
    ChallengeKind.BLOCKED: "Google has hard-blocked this sign-in.",
    ChallengeKind.CONNECTION_CHECK: "Google is probing your session fingerprint. You may need to wait or solve a visible prompt.",
}


def is_hard_fail(kind: ChallengeKind) -> bool:
    """True if no operator intervention can recover — abort immediately."""
    return kind == ChallengeKind.BLOCKED


def prompt_and_wait_for_solve(
    kind: ChallengeKind,
    timeout_s: int = 600,
    *,
    stdin=None,
    stderr=None,
) -> bool:
    """Print a human-readable prompt to stderr and block on stdin for <ENTER>.

    Returns True if the operator pressed ENTER (resume), False on timeout or
    EOF (treated as abort). Input is read from stdin by default; overridable
    for tests.

    Timeout uses `select` so we don't deadlock forever on an unattended run.
    """
    import select  # stdlib, available on all supported platforms

    stdin = stdin if stdin is not None else sys.stdin
    stderr = stderr if stderr is not None else sys.stderr

    msg = _KIND_MESSAGES.get(kind, f"Google challenge detected: {kind.value}.")
    print(f"\n⚠ {msg}", file=stderr, flush=True)
    print(f"  Solve it in the browser window, then press ENTER to continue.", file=stderr, flush=True)
    print(f"  (Timeout: {timeout_s}s; Ctrl-C to abort the whole run.)\n", file=stderr, flush=True)

    try:
        rlist, _, _ = select.select([stdin], [], [], timeout_s)
    except Exception:
        # select can't monitor non-fileno stdin substitutes; fall back to a
        # blocking readline (tests inject StringIO which does support that).
        line = stdin.readline()
        return line != ""

    if not rlist:
        print(f"timeout: no input within {timeout_s}s; aborting", file=stderr, flush=True)
        return False
    line = stdin.readline()
    return line != ""  # empty = EOF


__all__ = [
    "ChallengeKind",
    "ChallengeMode",
    "CONNECTION_CHECK_URL_MARKER",
    "CONNECTION_CHECK_STALL_S",
    "detect",
    "detect_from_html",
    "poll_for_challenge",
    "is_hard_fail",
    "prompt_and_wait_for_solve",
]
