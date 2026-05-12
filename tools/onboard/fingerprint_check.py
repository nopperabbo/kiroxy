#!/usr/bin/env python3
"""
Fingerprint diagnostic tool.

Phase G.FIX Layer 6.

Launches Camoufox with the same profile / proxy / geoip config `onboard.py`
would use, visits bot.sannysoft.com and CreepJS, and prints the top-level
signals. No automated pass/fail — the operator eyeballs the output and
decides whether the config needs adjusting.

Usage
─────

    python fingerprint_check.py                              # direct connect
    python fingerprint_check.py --proxy http://u:p@h:p       # with proxy
    KIROXY_ONBOARD_PROXY=http://u:p@h:p python fingerprint_check.py
    python fingerprint_check.py --profile-dir ./profiles_data/abc123
    python fingerprint_check.py --headless                   # no window

Output is appended to ``./fingerprint_report.txt`` and printed to stdout.
Screenshots of each probe page are saved under ``./screenshots/fp_*.png``.
"""

from __future__ import annotations

import argparse
import sys
import time
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Dict, Optional

BASE_DIR = Path(__file__).resolve().parent
SCREENSHOT_DIR = BASE_DIR / "screenshots"
REPORT_FILE = BASE_DIR / "fingerprint_report.txt"

SANNYSOFT_URL = "https://bot.sannysoft.com/"
CREEPJS_URL = "https://abrahamjuliot.github.io/creepjs/"
IPIFY_URL = "https://api.ipify.org"


def _log(msg: str) -> None:
    ts = time.strftime("%H:%M:%S")
    print(f"[{ts}] {msg}", flush=True)


def _ts_iso() -> str:
    return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def _parse_args(argv: Optional[list] = None) -> argparse.Namespace:
    p = argparse.ArgumentParser(
        prog="fingerprint_check",
        description="Diagnostic: launch Camoufox with onboard config and inspect fingerprint signals.",
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )
    p.add_argument(
        "--proxy", default=None,
        help="residential proxy URL; overrides KIROXY_ONBOARD_PROXY",
    )
    p.add_argument(
        "--profile-dir", default=None,
        help="Camoufox user_data_dir to use (default: fresh ephemeral dir)",
    )
    p.add_argument(
        "--headless", action="store_true",
        help="run headless (screenshots still saved)",
    )
    p.add_argument(
        "--timeout-s", type=int, default=60,
        help="per-probe timeout (default: 60)",
    )
    return p.parse_args(argv)


def _sample_profile() -> Dict[str, Any]:
    """Minimal profile for the diagnostic. Doesn't need to match any account."""
    return {
        "id": "diagnostic",
        "platform": "macOS",
        "viewport": {"width": 1680, "height": 1050},
        "locale": "en-US",
        "timezone_id": "America/Los_Angeles",
        "accept_language": "en-US,en;q=0.9",
    }


def _probe_sannysoft(drv, timeout_s: int, shot_prefix: Path) -> Dict[str, Any]:
    """Visit bot.sannysoft.com and scrape the pass/fail table.

    Sannysoft's test page shows a table where each row is a test and cells
    are coloured red/green. We grab all rows and count reds.
    """
    _log("probe: bot.sannysoft.com")
    try:
        drv.navigate(SANNYSOFT_URL, wait_until="networkidle")
    except Exception as e:
        _log(f"sannysoft: navigation failed: {e}")
        return {"ok": False, "error": str(e)}
    drv.wait(5000)  # let async checks finish

    shot = drv.screenshot(str(shot_prefix) + "_sannysoft.png")

    # Count "failed" markers. The page uses class "failed" / "passed" on cells.
    try:
        counts = drv.page.evaluate("""
            () => {
                const passed = document.querySelectorAll('.passed').length;
                const failed = document.querySelectorAll('.failed').length;
                const warn = document.querySelectorAll('.warn').length;
                return {passed, failed, warn};
            }
        """)
    except Exception as e:
        counts = {"error": str(e)}
    _log(f"sannysoft: {counts}  screenshot={shot}")
    return {"ok": True, "counts": counts, "screenshot": shot}


def _probe_creepjs(drv, timeout_s: int, shot_prefix: Path) -> Dict[str, Any]:
    """Visit CreepJS and grab its top-level trust score text.

    CreepJS takes 10-20s to compute. We navigate, wait, then scrape the
    displayed "trust score" element if present. Any scrape failure is
    logged but non-fatal — the screenshot is what operators actually use.
    """
    _log("probe: CreepJS (slow; ~20s compute time)")
    try:
        drv.navigate(CREEPJS_URL, wait_until="networkidle")
    except Exception as e:
        _log(f"creepjs: navigation failed: {e}")
        return {"ok": False, "error": str(e)}
    drv.wait(20000)  # compute

    shot = drv.screenshot(str(shot_prefix) + "_creepjs.png")

    # CreepJS elements change across versions; best-effort scrape.
    try:
        excerpt = drv.page.evaluate("""
            () => {
                const txt = document.body ? document.body.innerText : '';
                // Pull up to 40 lines for the report.
                return txt.split('\\n').slice(0, 40).join('\\n');
            }
        """) or ""
    except Exception as e:
        excerpt = f"(scrape error: {e})"
    _log(f"creepjs: screenshot={shot}")
    return {"ok": True, "excerpt_first_lines": excerpt[:800], "screenshot": shot}


def _probe_ip(drv) -> Dict[str, Any]:
    """Grab the egress IP the browser sees (matches / contradicts proxy config).

    Uses ipify.org's plain-text endpoint. If the proxy is working, this IP
    should differ from the operator's direct IP.
    """
    _log("probe: api.ipify.org")
    try:
        drv.navigate(IPIFY_URL, wait_until="domcontentloaded")
        drv.wait(1500)
        ip = drv.page.evaluate(
            "() => document.body ? document.body.innerText.trim() : ''"
        )
    except Exception as e:
        return {"ok": False, "error": str(e)}
    return {"ok": True, "egress_ip": ip}


def main(argv: Optional[list] = None) -> int:
    args = _parse_args(argv)

    # Resolve proxy using the same machinery as onboard.py.
    try:
        from proxy_support import resolve_proxy, validate_egress
    except ImportError as e:
        print(f"error: cannot import proxy_support: {e}", file=sys.stderr)
        return 1

    try:
        proxy_cfg = resolve_proxy(args.proxy)
    except Exception as e:
        print(f"error: bad proxy config: {e}", file=sys.stderr)
        return 1

    proxy_dict = None
    proxy_geoip = None
    if proxy_cfg is not None:
        _log(f"proxy: {proxy_cfg.server} (validating egress…)")
        ok, detail = validate_egress(proxy_cfg)
        if not ok:
            print(
                f"error: proxy egress validation failed: {detail}",
                file=sys.stderr,
            )
            return 1
        _log(f"proxy: ok (egress IP {detail})")
        proxy_dict = proxy_cfg.as_camoufox_dict()
        proxy_geoip = detail
    else:
        _log("proxy: direct (no KIROXY_ONBOARD_PROXY / --proxy)")

    SCREENSHOT_DIR.mkdir(parents=True, exist_ok=True)
    shot_prefix = SCREENSHOT_DIR / f"fp_{int(time.time())}"

    try:
        from browser_driver import BrowserDriver
    except ImportError as e:
        print(f"error: cannot import browser_driver: {e}", file=sys.stderr)
        return 1

    profile = _sample_profile()
    user_data_dir = args.profile_dir  # None = ephemeral

    report: Dict[str, Any] = {
        "timestamp": _ts_iso(),
        "proxy": proxy_cfg.server if proxy_cfg else None,
        "profile_dir": user_data_dir,
        "headless": bool(args.headless),
    }

    try:
        with BrowserDriver(
            profile=profile,
            headless=args.headless,
            user_data_dir=user_data_dir,
            proxy=proxy_dict,
            geoip=proxy_geoip,
        ) as drv:
            report["ip"] = _probe_ip(drv)
            report["sannysoft"] = _probe_sannysoft(drv, args.timeout_s, shot_prefix)
            report["creepjs"] = _probe_creepjs(drv, args.timeout_s, shot_prefix)
    except Exception as e:
        print(f"error: diagnostic run failed: {e}", file=sys.stderr)
        return 1

    # Write the report.
    lines = [
        f"──── fingerprint_check report {report['timestamp']} ────",
        f"proxy:        {report['proxy'] or '(direct)'}",
        f"profile_dir:  {report['profile_dir'] or '(ephemeral)'}",
        f"headless:     {report['headless']}",
        f"egress ip:    {report.get('ip', {}).get('egress_ip', '?')}",
        f"sannysoft:    {report.get('sannysoft', {}).get('counts', '?')}",
        f"  screenshot: {report.get('sannysoft', {}).get('screenshot', '?')}",
        f"creepjs:      {report.get('creepjs', {}).get('screenshot', '?')}",
        "",
    ]
    txt = "\n".join(lines)
    print(txt)
    try:
        with REPORT_FILE.open("a", encoding="utf-8") as f:
            f.write(txt + "\n")
        _log(f"report appended to {REPORT_FILE}")
    except Exception as e:
        _log(f"warn: could not append report: {e}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
