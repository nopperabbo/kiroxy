"""
Camoufox (Firefox-based stealth browser) wrapper with humanization helpers.

Kept intentionally thin: one class, sync Playwright API, no asyncio. The
onboarder is single-account per invocation, so sync is simpler than async
and matches the brief's batch-mode-is-deferred-to-G.3 stance.

Camoufox rationale (vs Patchright/Chromium): kikirro already used
Patchright/Chromium against the Kiro WEB portal. The brief pins Camoufox
here for the DESKTOP auth flow — Camoufox's Firefox fingerprint is distinct
from kikirro's Chromium fingerprint, so the two tools won't accidentally
collide if the same account is hit by both.

Profile dict shape (matches kikirro profiles.json entries):
  id, platform, user_agent, sec_ch_ua, sec_ch_ua_platform, sec_ch_ua_mobile,
  viewport {width,height}, locale, timezone_id, accept_language
"""

from __future__ import annotations

import contextlib
import random
import time
from pathlib import Path
from typing import Any, Callable, Dict, Optional

try:
    # Camoufox ships a sync Playwright wrapper that launches a fingerprint-
    # hardened Firefox. Import is deferred so syntax / --help paths work
    # even before `pip install -r requirements.txt` has run.
    from camoufox.sync_api import Camoufox  # type: ignore
except ImportError:  # pragma: no cover — runtime-time import guard
    Camoufox = None  # type: ignore


class BrowserDriverUnavailableError(RuntimeError):
    """Raised when Camoufox isn't importable at run time."""


class BrowserDriver:
    """Context-managed Camoufox session driving a single page.

    Usage:

        with BrowserDriver(profile=profile, headless=False) as drv:
            drv.navigate("https://example.com")
            drv.type_humanized('input[name="q"]', "hello")
            drv.click('button[type="submit"]')
    """

    def __init__(
        self,
        profile: Dict[str, Any],
        headless: bool = False,
        default_step_timeout_ms: int = 30_000,
        default_nav_timeout_ms: int = 60_000,
    ) -> None:
        if Camoufox is None:
            raise BrowserDriverUnavailableError(
                "camoufox is not installed; run `pip install -r requirements.txt` "
                "and then `python -m camoufox fetch`"
            )
        self.profile = profile
        self.headless = headless
        self.default_step_timeout_ms = default_step_timeout_ms
        self.default_nav_timeout_ms = default_nav_timeout_ms
        self._cm = None
        self._browser = None  # Camoufox context manager yields a Browser
        self._page = None

    # ──────────────────────────────────────────────────────────────────────
    # Lifecycle
    # ──────────────────────────────────────────────────────────────────────

    def __enter__(self) -> "BrowserDriver":
        # Camoufox's sync_api.Camoufox(…) returns a context manager that
        # yields a Page directly (not a Browser). We adopt the yielded page.
        #
        # Humanization flag turns on mouse jitter, typing delays, scroll
        # micro-movements inside Camoufox itself; we still add our own
        # per-keystroke jitter in type_humanized() for belt-and-braces.
        os_hint_map = {
            "macOS": "macos",
            "Windows": "windows",
            "Linux": "linux",
            "Chrome OS": "linux",  # Camoufox doesn't model CrOS — closest match
        }
        os_hints = [os_hint_map.get(self.profile.get("platform", "macOS"), "macos")]

        viewport = self.profile.get("viewport") or {"width": 1920, "height": 1080}

        self._cm = Camoufox(
            headless=self.headless,
            humanize=True,
            os=os_hints,
            locale=self.profile.get("locale", "en-US"),
            # Camoufox supports per-profile timezone / geo / screen; we pass
            # what we have and let it derive the rest from the fingerprint.
            geoip=False,  # do NOT pin to caller's IP; keep fingerprint decoupled
            window=[viewport.get("width", 1920), viewport.get("height", 1080)],
        )
        self._page = self._cm.__enter__()
        self._page.set_default_timeout(self.default_step_timeout_ms)
        self._page.set_default_navigation_timeout(self.default_nav_timeout_ms)

        # Override Accept-Language explicitly; Camoufox's locale argument
        # controls navigator.language but not always the HTTP header shape.
        try:
            accept_language = self.profile.get("accept_language", "en-US,en;q=0.9")
            self._page.set_extra_http_headers({"Accept-Language": accept_language})
        except Exception:
            pass  # best-effort — non-fatal

        return self

    def __exit__(self, exc_type, exc, tb) -> None:
        with contextlib.suppress(Exception):
            if self._cm is not None:
                self._cm.__exit__(exc_type, exc, tb)
        self._page = None
        self._browser = None
        self._cm = None

    # ──────────────────────────────────────────────────────────────────────
    # Navigation
    # ──────────────────────────────────────────────────────────────────────

    @property
    def page(self):
        if self._page is None:
            raise RuntimeError("BrowserDriver used outside a `with` block")
        return self._page

    def navigate(self, url: str, wait_until: str = "domcontentloaded") -> None:
        self.page.goto(url, wait_until=wait_until)

    def wait_for_selector(self, selector: str, timeout_ms: int = 30_000):
        return self.page.wait_for_selector(selector, timeout=timeout_ms)

    # ──────────────────────────────────────────────────────────────────────
    # Humanized input
    # ──────────────────────────────────────────────────────────────────────

    def type_humanized(
        self,
        selector: str,
        text: str,
        delay_range_ms: tuple = (50, 180),
        pre_click: bool = True,
    ) -> None:
        """Type text char-by-char with randomized inter-keystroke delay.

        pre_click=True clicks the field first so focus is certain; turn it
        off when the field is already focused (e.g. right after Tab).
        """
        if pre_click:
            self.page.click(selector, timeout=self.default_step_timeout_ms)
        lo, hi = delay_range_ms
        locator = self.page.locator(selector).first
        for ch in text:
            locator.press_sequentially(ch, delay=random.randint(lo, hi))

    def click(self, selector: str, timeout_ms: int = 10_000) -> None:
        self.page.click(selector, timeout=timeout_ms)

    # ──────────────────────────────────────────────────────────────────────
    # Redirect / URL watching
    # ──────────────────────────────────────────────────────────────────────

    def wait_for_navigation_matching(
        self,
        predicate: Callable[[str], bool],
        timeout_s: int = 120,
        poll_interval_s: float = 0.25,
    ) -> str:
        """Poll page.url until predicate(url) is True, up to timeout_s.

        Used to catch the kiro:// redirect — Playwright's built-in
        expect_navigation won't fire for non-http schemes because the
        browser cancels the nav. We sample both the live URL and
        the last-seen framenavigated target.
        """
        deadline = time.monotonic() + timeout_s
        seen_urls: list[str] = []

        def _on_frame_nav(frame):
            try:
                u = frame.url
            except Exception:
                return
            if u:
                seen_urls.append(u)

        # framenavigated fires even when the navigation is ultimately
        # cancelled (e.g. non-http scheme) — exactly what we need.
        self.page.on("framenavigated", _on_frame_nav)
        try:
            while time.monotonic() < deadline:
                current = self.page.url or ""
                if predicate(current):
                    return current
                for u in reversed(seen_urls):
                    if predicate(u):
                        return u
                time.sleep(poll_interval_s)
            raise TimeoutError(f"timed out after {timeout_s}s waiting for navigation")
        finally:
            # Playwright's event emitters don't expose 'off' for callbacks
            # declared inline; we leak this until context teardown. That's
            # fine — driver is single-use, GC'd on __exit__.
            pass

    # ──────────────────────────────────────────────────────────────────────
    # Debug
    # ──────────────────────────────────────────────────────────────────────

    def screenshot(self, path: str) -> Optional[str]:
        """Save a full-page screenshot. Returns the path on success, else None.

        Silently swallows errors: a failing screenshot must never mask the
        original exception that motivated the screenshot.
        """
        try:
            p = Path(path)
            p.parent.mkdir(parents=True, exist_ok=True)
            self.page.screenshot(path=str(p), full_page=True)
            return str(p)
        except Exception:
            return None


__all__ = [
    "BrowserDriver",
    "BrowserDriverUnavailableError",
]
