"""
Human interaction patterns — burst-pause typing, typo injection, mouse drift.

Phase G.FIX Layer 3.

Pure functions, no Camoufox / Playwright dependency, so they're unit-testable
in isolation with `python3 -m unittest test_human.py`.

Design notes
────────────

**Typing model.** Real humans don't type at uniform inter-keystroke intervals.
A common pattern is: 3-5 keys fast, then a cognitive micro-pause (250-600ms)
while the next chunk is formed, then another burst. This matches keylog
research on password fields. Uniform 50-180ms — what G.1 shipped — is a
decent baseline but easily clustered by anti-bot classifiers that look at
the second moment of the distribution.

**Typo model.** Humans typo. 1-2% of characters in fast typing are
mis-struck then corrected. We inject at most `MAX_TYPOS_PER_TEXT` typos
per field (capped to avoid passwords looking unhinged). A typo corrects
via Backspace, emitted as the sentinel character ``"\b"`` in the returned
string; the caller is expected to map that to a Backspace key press.

**Drift model.** Playwright's `mouse.move(x, y, steps=n)` already does
linear interpolation. We add perpendicular jitter by offsetting each
interpolation point by a small random amount orthogonal to the line from
start to end. This produces a slightly curved, noisy path that looks
hand-driven under animation.

Tests assert statistical properties (mean, p95, typo rate bands) over
2000 samples. See test_human.py.
"""

from __future__ import annotations

import math
import random
from typing import Iterable, Iterator, List, Tuple

# ── Typing distribution knobs ─────────────────────────────────────────────

BURST_MIN = 3
BURST_MAX = 5
CHAR_DELAY_MIN_MS = 40
CHAR_DELAY_MAX_MS = 90
PAUSE_MIN_MS = 250
PAUSE_MAX_MS = 600

# Typo knobs
TYPO_RATE = 0.015           # 1.5% per-char probability
MAX_TYPOS_PER_TEXT = 2
TYPO_PAUSE_MIN_MS = 120     # "oh, typo" reaction time
TYPO_PAUSE_MAX_MS = 300

# Adjacency map for realistic typos (QWERTY neighbors, lowercase).
# We pick a random neighbor as the wrong key. Non-alpha chars never typo.
_QWERTY_NEIGHBORS = {
    "q": "wa",  "w": "qes",  "e": "wdr",  "r": "edft", "t": "rfgy",
    "y": "tghu","u": "yhji", "i": "ukjo", "o": "ikpl", "p": "ol",
    "a": "qws",  "s": "awedx","d": "serfc","f": "drtgv","g": "ftyhb",
    "h": "gyujn","j": "huikm","k": "jiol", "l": "kop",
    "z": "asx",  "x": "zsdc", "c": "xdfv", "v": "cfgb", "b": "vghn",
    "n": "bhjm", "m": "njk",
}


def _typo_key_for(ch: str) -> str:
    """Return a plausible wrong-key for ch, or empty if no good substitution."""
    low = ch.lower()
    neighbors = _QWERTY_NEIGHBORS.get(low)
    if not neighbors:
        return ""
    wrong = random.choice(neighbors)
    return wrong.upper() if ch.isupper() else wrong


def inject_typos(text: str, max_typos: int = MAX_TYPOS_PER_TEXT) -> List[str]:
    """Return a list of "keys" to press, including backspace corrections.

    Each element is either a single character to type, or the sentinel
    "\\b" meaning Backspace.

    For text "abc" with a typo at index 1, a possible output is:
      ["a", "c", "\\b", "b", "c"]  — types a, wrong 'c', backspace, correct 'b', c
    """
    keys: List[str] = []
    typos_used = 0
    for ch in text:
        if typos_used < max_typos and random.random() < TYPO_RATE:
            wrong = _typo_key_for(ch)
            if wrong:
                keys.append(wrong)
                keys.append("\b")
                typos_used += 1
        keys.append(ch)
    return keys


def burst_pause_delays(n: int) -> List[int]:
    """Return n inter-keystroke delays in ms, following burst-pause pattern.

    Algorithm:
      - Draw burst length in [BURST_MIN..BURST_MAX].
      - Each char inside burst gets uniform [CHAR_DELAY_MIN_MS..CHAR_DELAY_MAX_MS].
      - After a burst, insert one pause in [PAUSE_MIN_MS..PAUSE_MAX_MS] before
        the next burst's first character.

    Returns exactly n delays. The first delay in a burst is already the pause
    (for non-first bursts). The first burst's first delay is a normal char
    delay, not a pause — the "read the field" pause is handled separately
    by `human_pause()` in the driver.
    """
    if n <= 0:
        return []
    delays: List[int] = []
    remaining = n
    first_burst = True
    while remaining > 0:
        burst_len = min(random.randint(BURST_MIN, BURST_MAX), remaining)
        for i in range(burst_len):
            if i == 0 and not first_burst:
                delays.append(random.randint(PAUSE_MIN_MS, PAUSE_MAX_MS))
            else:
                delays.append(random.randint(CHAR_DELAY_MIN_MS, CHAR_DELAY_MAX_MS))
        remaining -= burst_len
        first_burst = False
    return delays


def typo_pause_ms() -> int:
    """How long to 'react' between wrong key and backspace."""
    return random.randint(TYPO_PAUSE_MIN_MS, TYPO_PAUSE_MAX_MS)


# ── Mouse drift ───────────────────────────────────────────────────────────

DRIFT_STEPS_MIN = 6
DRIFT_STEPS_MAX = 10
DRIFT_JITTER_FRAC = 0.08  # max perpendicular offset as fraction of straight distance


def drift_points(
    x0: float,
    y0: float,
    x1: float,
    y1: float,
    steps: int = 0,
) -> Iterator[Tuple[float, float]]:
    """Yield (x, y) points along a jittered path from (x0,y0) to (x1,y1).

    The path is the straight line plus per-step perpendicular jitter sampled
    from a small bell-like distribution (sum of two uniforms → triangular).
    Endpoints are exact (no jitter on the final point).

    steps=0 picks a random count in [DRIFT_STEPS_MIN..DRIFT_STEPS_MAX].
    """
    if steps <= 0:
        steps = random.randint(DRIFT_STEPS_MIN, DRIFT_STEPS_MAX)

    dx, dy = x1 - x0, y1 - y0
    dist = math.hypot(dx, dy)
    # Perpendicular unit vector. If start == end, zero jitter.
    if dist > 0:
        px, py = -dy / dist, dx / dist
    else:
        px = py = 0.0
    max_jitter = dist * DRIFT_JITTER_FRAC

    for i in range(1, steps + 1):
        t = i / steps
        # Triangular distribution in [-max_jitter, +max_jitter]
        jitter = (random.random() + random.random() - 1.0) * max_jitter
        if i == steps:
            jitter = 0.0  # land exactly on target
        x = x0 + dx * t + px * jitter
        y = y0 + dy * t + py * jitter
        yield (x, y)


# ── Read pause ────────────────────────────────────────────────────────────


def read_pause_ms(field_chars: int, base_ms: int = 500, per_char_ms: int = 30,
                  random_range_ms: Tuple[int, int] = (0, 800)) -> int:
    """How long to 'read the form' before clicking Next.

    Scales with how much the user just typed (longer passwords = slightly
    longer think-time before confirming).
    """
    lo, hi = random_range_ms
    return base_ms + field_chars * per_char_ms + random.randint(lo, hi)


__all__ = [
    "BURST_MIN", "BURST_MAX",
    "CHAR_DELAY_MIN_MS", "CHAR_DELAY_MAX_MS",
    "PAUSE_MIN_MS", "PAUSE_MAX_MS",
    "TYPO_RATE", "MAX_TYPOS_PER_TEXT",
    "TYPO_PAUSE_MIN_MS", "TYPO_PAUSE_MAX_MS",
    "DRIFT_STEPS_MIN", "DRIFT_STEPS_MAX", "DRIFT_JITTER_FRAC",
    "burst_pause_delays", "inject_typos", "typo_pause_ms",
    "drift_points", "read_pause_ms",
]
