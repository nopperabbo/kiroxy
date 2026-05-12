"""
Camoufox (Firefox-based stealth browser) wrapper with humanization helpers
and persistent-profile support.

Kept intentionally thin: one class, sync Playwright API, no asyncio. Single
account per invocation.

Two operating modes:

  * fresh (G.1 default) — user_data_dir is None, we do browser.new_context().
    Every run starts cold; Google dislikes this.

  * persistent (Phase G.FIX default) — user_data_dir is a real path. Camoufox
    yields a BrowserContext directly instead of a Browser; we adopt it.
    Cookies, localStorage, IndexedDB, history persist across runs. This is
    what lets warmup-built state accrue.

Camoufox's sync API semantics differ between the two modes (Browser vs
BrowserContext returned from __enter__), so the wrapper hides that with
a `persistent` boolean. Callers don't care which mode; they get a page.

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

    Usage (fresh):

        with BrowserDriver(profile=profile, headless=False) as drv:
            drv.navigate("https://example.com")
            drv.type_humanized('input[name="q"]', "hello")
            drv.click('button[type="submit"]')

    Usage (persistent):

        with BrowserDriver(profile=profile, user_data_dir="./profile") as drv:
            drv.navigate("https://youtube.com")
            # session state persists on context exit

    Proxy support:

        with BrowserDriver(profile=profile, proxy={
            "server": "http://proxy.example:8080",
            "username": "user", "password": "pass",
        }) as drv:
            ...

        geoip defaults to True when a proxy is set, matching Camoufox best
        practice; pass explicit geoip to override.
    """

    def __init__(
        self,
        profile: Dict[str, Any],
        headless: bool = False,
        default_step_timeout_ms: int = 30_000,
        default_nav_timeout_ms: int = 60_000,
        user_data_dir: Optional[str] = None,
        proxy: Optional[Dict[str, str]] = None,
        geoip: Optional[Any] = None,
        disable_coop: bool = True,
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
        self.user_data_dir = user_data_dir
        self.proxy = proxy
        # geoip: None → auto (True when proxy set, False otherwise); caller-provided takes precedence
        if geoip is None:
            geoip = True if proxy else False
        self.geoip = geoip
        self.disable_coop = disable_coop
        self.persistent = user_data_dir is not None

        self._cm = None
        self._browser = None
        self._context = None
        self._page = None
        self._url_log: list[tuple[str, str]] = []

    # ──────────────────────────────────────────────────────────────────────
    # Lifecycle
    # ──────────────────────────────────────────────────────────────────────

    def __enter__(self) -> "BrowserDriver":
        os_hint_map = {
            "macOS": "macos",
            "Windows": "windows",
            "Linux": "linux",
            "Chrome OS": "linux",
        }
        os_hints = [os_hint_map.get(self.profile.get("platform", "macOS"), "macos")]

        viewport = self.profile.get("viewport") or {"width": 1920, "height": 1080}

        kwargs: Dict[str, Any] = dict(
            headless=self.headless,
            humanize=True,
            os=os_hints,
            locale=self.profile.get("locale", "en-US"),
            geoip=self.geoip,
            window=[viewport.get("width", 1920), viewport.get("height", 1080)],
            i_know_what_im_doing=True,
            disable_coop=self.disable_coop,
        )
        if self.proxy:
            kwargs["proxy"] = self.proxy
        if self.persistent:
            # Persistent mode: ensure the dir exists and yield a context directly.
            Path(self.user_data_dir).mkdir(parents=True, exist_ok=True)
            kwargs["persistent_context"] = True
            kwargs["user_data_dir"] = str(self.user_data_dir)

        self._cm = Camoufox(**kwargs)
        entered = self._cm.__enter__()

        # Camoufox returns:
        #   - persistent_context=True → a BrowserContext directly
        #   - persistent_context=False → a Browser
        # We detect which by duck-typing: BrowserContext has `new_page` but no `new_context`.
        if self.persistent:
            self._context = entered
            self._browser = None
            # Persistent contexts have a default blank page; reuse if present, else new one.
            pages = getattr(self._context, "pages", None) or []
            self._page = pages[0] if pages else self._context.new_page()
        else:
            self._browser = entered
            self._context = self._browser.new_context()
            self._page = self._context.new_page()

        self._page.set_default_timeout(self.default_step_timeout_ms)
        self._page.set_default_navigation_timeout(self.default_nav_timeout_ms)

        self._install_url_listeners()

        try:
            accept_language = self.profile.get("accept_language", "en-US,en;q=0.9")
            self._page.set_extra_http_headers({"Accept-Language": accept_language})
        except Exception:
            pass

        return self

    def _install_url_listeners(self) -> None:
        """Attach URL listeners immediately after page creation.

        Must run BEFORE any navigation so kiro:// redirects that fire during
        the Google OAuth flow don't get missed. Captured URLs sit in
        self._url_log until wait_for_navigation_matching drains them.
        """
        def _log_url(url: str, source: str) -> None:
            if url:
                self._url_log.append((source, url))

        def _on_frame_nav(frame) -> None:
            try:
                if frame.parent_frame is None:
                    _log_url(frame.url, "framenav")
            except Exception:
                pass

        def _on_request(request) -> None:
            try:
                _log_url(request.url, "request")
            except Exception:
                pass

        def _on_response(response) -> None:
            try:
                if 300 <= response.status < 400:
                    loc = response.headers.get("location", "") or ""
                    _log_url(loc, "redirect")
            except Exception:
                pass

        self._context.on("request", _on_request)
        self._context.on("response", _on_response)
        self._page.on("framenavigated", _on_frame_nav)

    def __exit__(self, exc_type, exc, tb) -> None:
        with contextlib.suppress(Exception):
            if self._cm is not None:
                self._cm.__exit__(exc_type, exc, tb)
        self._page = None
        self._context = None
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

    def wait(self, ms: int) -> None:
        self.page.wait_for_timeout(ms)

    # ──────────────────────────────────────────────────────────────────────
    # Humanized input — thin wrappers; full suite lives in human.py
    # ──────────────────────────────────────────────────────────────────────

    def type_humanized(
        self,
        selector: str,
        text: str,
        delay_range_ms: tuple = (50, 180),
        pre_click: bool = True,
    ) -> None:
        """DEPRECATED: forwards to human_type for backwards compatibility.

        New code should use ``human_type`` which implements burst-pause
        timing instead of uniform delay. This shim exists because
        onboard.py G.1 still calls the old name.
        """
        self.human_type(selector, text, pre_click=pre_click)

    def human_type(
        self,
        selector: str,
        text: str,
        pre_click: bool = True,
    ) -> None:
        """Type text with burst-pause humanization + occasional typo+backspace.

        Timing strategy (see human.py::burst_pause_delays for distribution):
          - Bursts of 3-5 chars at 40-90ms inter-char.
          - 250-600ms pause between bursts.
          - 1-2% typo rate per char, capped at 2 typos per text, with
            120-300ms pause + backspace + correct keystroke.
        """
        from human import burst_pause_delays, inject_typos  # lazy import

        if pre_click:
            self.page.click(selector, timeout=self.default_step_timeout_ms)

        locator = self.page.locator(selector).first
        keys = inject_typos(text)
        delays = burst_pause_delays(len(keys))
        for (key, delay_ms) in zip(keys, delays):
            if key == "\b":
                locator.press("Backspace")
            else:
                locator.press_sequentially(key, delay=delay_ms)

    def human_pause(self, lo_ms: int = 500, hi_ms: int = 2000) -> None:
        """Random 'reading' pause before submit clicks."""
        self.page.wait_for_timeout(random.randint(lo_ms, hi_ms))

    def drift_cursor(self, selector: str) -> None:
        """Move the mouse along a curved path toward the selector's center.

        No-op if the element doesn't exist or has no bounding box. Best-effort
        cosmetic layer — click() still does its own targeting, we just make
        the mouse look human before that.
        """
        try:
            from human import drift_points  # lazy import
        except ImportError:
            return
        try:
            box = self.page.locator(selector).first.bounding_box()
            if not box:
                return
            target_x = box["x"] + box["width"] / 2
            target_y = box["y"] + box["height"] / 2
            # Start from current position; Playwright exposes that on mouse
            # via a hidden attribute, so we fake it: start from a random
            # offset and steps=n takes care of interpolation inside Playwright.
            start_x = target_x + random.uniform(-200, 200)
            start_y = target_y + random.uniform(-150, 150)
            for px, py in drift_points(start_x, start_y, target_x, target_y):
                self.page.mouse.move(px, py, steps=1)
            self.page.wait_for_timeout(random.randint(100, 300))
        except Exception:
            pass  # never let mouse drift failures break the flow

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
        """Drain persistent URL log until predicate matches.

        Listeners are installed at __enter__ time (see _install_url_listeners),
        so URLs captured during the Google OAuth flow are already in
        self._url_log by the time this is called. We check both the historical
        log and the live page.url on each poll iteration.
        """
        scanned_idx = 0
        deadline = time.monotonic() + timeout_s
        while time.monotonic() < deadline:
            while scanned_idx < len(self._url_log):
                _src, url = self._url_log[scanned_idx]
                scanned_idx += 1
                if predicate(url):
                    return url
            current = self.page.url or ""
            if predicate(current):
                return current
            time.sleep(poll_interval_s)

        recent = "\n".join(f"  [{s}] {u[:140]}" for s, u in self._url_log[-15:])
        raise TimeoutError(
            f"timed out after {timeout_s}s waiting for navigation.\n"
            f"Last 15 URLs captured by listeners:\n{recent or '  (none)'}\n"
            f"Current page.url: {self.page.url or '(empty)'}"
        )

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
