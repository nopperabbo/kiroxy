"""
Human interaction patterns — burst-pause typing, typo injection, mouse drift.

Pure functions, no Camoufox dependency, so they're unit-testable in isolation
with `python3 -m unittest test_human.py`.

This module is imported lazily by browser_driver.py so syntax-only lint of the
onboarder still works when camoufox is absent.

NOTE: Layer 3 of Phase G.FIX. This initial stub gives browser_driver.py the
minimal surface it calls (burst_pause_delays, inject_typos, drift_points).
The full humanization algorithm — including tuned typo rates and curved mouse
paths — lands in the Layer 3 commit. Keeping those two commits atomic.
"""

from __future__ import annotations

import random
from typing import Iterable, Iterator, List, Tuple


def burst_pause_delays(n: int) -> List[int]:
    """Return n inter-keystroke delays in ms.

    Minimal stub for Layer 1 integration. Layer 3 replaces with a proper
    burst-pause distribution.
    """
    return [random.randint(50, 180) for _ in range(max(0, n))]


def inject_typos(text: str) -> str:
    """Return a possibly-modified typing script, with '\\b' meaning Backspace.

    Minimal stub for Layer 1 integration. Layer 3 replaces with actual
    typo injection. For now just returns text unchanged.
    """
    return text


def drift_points(x0: float, y0: float, x1: float, y1: float, steps: int = 8) -> Iterator[Tuple[float, float]]:
    """Yield (x, y) points along a curved path from (x0,y0) to (x1,y1).

    Minimal stub for Layer 1. Layer 3 replaces with Bezier-like jittered curve.
    """
    for i in range(1, steps + 1):
        t = i / steps
        yield (x0 + (x1 - x0) * t, y0 + (y1 - y0) * t)


__all__ = ["burst_pause_delays", "inject_typos", "drift_points"]
