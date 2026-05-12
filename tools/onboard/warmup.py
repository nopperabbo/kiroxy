"""
Warm-up flows that build session state before attempting Kiro login.

Google's password-challenge page probes for session cookies on youtube.com
and other properties (literal `checkConnection=youtube` query param). A
persistent Camoufox profile with recent YouTube/Google/GitHub activity
flies past those checks; a fresh profile tends to stall.

This module defines a reusable warm-up flow as a list of (url, dwell_s)
tuples. Each step is best-effort: an individual navigation failure is
logged and skipped, never fatal. Total budget is hard-capped so a stuck
warmup can't block the main login flow forever.

A marker file `.warmed-at` with a unix timestamp is written to the profile
dir after successful warmup. Subsequent runs skip warmup if the marker is
recent (< WARMUP_TTL_DAYS old).

Phase G.FIX Layer 1.
"""

from __future__ import annotations

import time
from dataclasses import dataclass
from pathlib import Path
from typing import Callable, List, Optional, Sequence, Tuple

WARMUP_TTL_DAYS = 7
WARMUP_HARD_CAP_S = 180
MARKER_FILENAME = ".warmed-at"


@dataclass(frozen=True)
class WarmupStep:
    url: str
    dwell_s: float
    description: str


DEFAULT_WARMUP: List[WarmupStep] = [
    WarmupStep(
        url="https://www.youtube.com/",
        dwell_s=45.0,
        description="YouTube visitor cookie + SPA settle",
    ),
    WarmupStep(
        url="https://www.google.com/search?q=weather",
        dwell_s=15.0,
        description="Google SERP session state",
    ),
    WarmupStep(
        url="https://github.com/",
        dwell_s=10.0,
        description="GitHub referrer variety",
    ),
    WarmupStep(
        url="about:blank",
        dwell_s=15.0,
        description="idle dwell",
    ),
]


def marker_path(profile_dir: Path) -> Path:
    return Path(profile_dir) / MARKER_FILENAME


def should_warmup(profile_dir: Path, ttl_days: int = WARMUP_TTL_DAYS) -> bool:
    """Return True if warmup should run for this profile dir.

    Rules:
      - No marker file → yes.
      - Marker older than ttl_days → yes (stale session looks suspicious too).
      - Fresh marker → no.
      - Malformed marker (unparseable) → yes (treat as if absent).
    """
    p = marker_path(profile_dir)
    if not p.exists():
        return True
    try:
        ts = float(p.read_text().strip())
    except Exception:
        return True
    age_s = time.time() - ts
    return age_s > (ttl_days * 86400)


def write_marker(profile_dir: Path) -> None:
    """Record successful warmup timestamp. Best-effort."""
    try:
        Path(profile_dir).mkdir(parents=True, exist_ok=True)
        marker_path(profile_dir).write_text(f"{time.time():.0f}\n", encoding="utf-8")
    except Exception:
        pass


# Logger type: (msg) -> None. Keeps this module dependency-free for tests.
Logger = Callable[[str], None]


def run_warmup(
    driver,
    steps: Sequence[WarmupStep] = DEFAULT_WARMUP,
    hard_cap_s: float = WARMUP_HARD_CAP_S,
    log: Optional[Logger] = None,
) -> Tuple[int, int]:
    """Execute each step against the given BrowserDriver.

    Returns (completed, attempted). Completed steps reached their dwell; attempted
    includes ones that raised during navigation or dwell.

    The driver must have a `.navigate(url)` method and a `.wait(ms)` method (or
    equivalent). BrowserDriver provides both.

    hard_cap_s applies to the total wall clock across all steps. When exceeded,
    remaining steps are skipped.
    """
    _log = log or (lambda msg: None)
    started = time.monotonic()
    completed = 0
    attempted = 0

    for step in steps:
        if time.monotonic() - started > hard_cap_s:
            _log(
                f"warmup: hard cap {hard_cap_s}s hit; skipping remaining "
                f"{len(steps) - attempted} step(s)"
            )
            break

        attempted += 1
        _log(f"warmup[{attempted}/{len(steps)}]: {step.description}  ({step.url})")
        try:
            driver.navigate(step.url)
        except Exception as e:
            _log(f"warmup: navigate failed ({e.__class__.__name__}: {e}); skipping")
            continue

        remaining = hard_cap_s - (time.monotonic() - started)
        dwell = min(step.dwell_s, max(0.0, remaining))
        try:
            driver.wait(int(dwell * 1000))
        except Exception as e:
            _log(f"warmup: dwell failed ({e.__class__.__name__}: {e}); continuing")
            continue

        completed += 1

    _log(f"warmup: {completed}/{attempted} step(s) completed in "
         f"{time.monotonic() - started:.1f}s")
    return completed, attempted


__all__ = [
    "WARMUP_TTL_DAYS",
    "WARMUP_HARD_CAP_S",
    "DEFAULT_WARMUP",
    "WarmupStep",
    "marker_path",
    "should_warmup",
    "write_marker",
    "run_warmup",
]
