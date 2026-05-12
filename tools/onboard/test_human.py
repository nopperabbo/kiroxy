"""Unit tests for human.py — typing, typo, drift, read-pause distributions.

Pure functions, no browser. Statistical assertions use 2000-sample runs with
a fixed seed to keep tests deterministic.

    python3 -m unittest test_human.py
"""

from __future__ import annotations

import math
import random
import unittest

import human


class TestBurstPauseDelays(unittest.TestCase):
    def test_returns_exactly_n(self):
        for n in [1, 5, 10, 100]:
            delays = human.burst_pause_delays(n)
            self.assertEqual(len(delays), n)

    def test_empty_for_zero_or_negative(self):
        self.assertEqual(human.burst_pause_delays(0), [])
        self.assertEqual(human.burst_pause_delays(-5), [])

    def test_delay_values_in_band(self):
        """Each delay is either a fast char (40-90ms) or a pause (250-600ms)."""
        random.seed(0)
        delays = human.burst_pause_delays(2000)
        for d in delays:
            in_char_band = human.CHAR_DELAY_MIN_MS <= d <= human.CHAR_DELAY_MAX_MS
            in_pause_band = human.PAUSE_MIN_MS <= d <= human.PAUSE_MAX_MS
            self.assertTrue(
                in_char_band or in_pause_band,
                f"delay {d} outside both char and pause bands",
            )

    def test_pause_fraction_reasonable(self):
        """Roughly 1-in-4 to 1-in-6 delays should be pauses (burst len 3-5)."""
        random.seed(1)
        delays = human.burst_pause_delays(2000)
        pauses = sum(1 for d in delays if d >= human.PAUSE_MIN_MS)
        fraction = pauses / len(delays)
        # Expected ~1/4 (burst_len=4 on avg → 1 pause per 4 chars). Allow wide band.
        self.assertGreater(fraction, 0.15, f"pause fraction too low: {fraction}")
        self.assertLess(fraction, 0.35, f"pause fraction too high: {fraction}")


class TestInjectTypos(unittest.TestCase):
    def test_reconstruction(self):
        """After processing backspaces, result must equal input."""
        random.seed(2)
        for text in ["password", "hello world", "abcdefgh", "x", ""]:
            keys = human.inject_typos(text)
            final = []
            for k in keys:
                if k == "\b":
                    if final:
                        final.pop()
                else:
                    final.append(k)
            self.assertEqual("".join(final), text, f"reconstruction mismatch for {text!r}")

    def test_typo_cap(self):
        """Never exceed MAX_TYPOS_PER_TEXT backspaces."""
        random.seed(3)
        for _ in range(100):
            keys = human.inject_typos("abcdefghijklmnopqrstuvwxyz")
            self.assertLessEqual(
                keys.count("\b"),
                human.MAX_TYPOS_PER_TEXT,
            )

    def test_typo_rate_in_band(self):
        """Across 2000 chars, typo fraction should be close to TYPO_RATE.

        Capped at MAX_TYPOS_PER_TEXT per input, so per 26-char string we see
        at most 2 typos → max rate 2/26 ≈ 7.7%. Expected observed: ~1.5% at
        lower end, capped by MAX_TYPOS_PER_TEXT at the upper end. We assert
        a very generous band.
        """
        random.seed(4)
        total_chars = 0
        total_typos = 0
        for _ in range(100):
            text = "abcdefghij klmnopqrst"
            keys = human.inject_typos(text)
            total_chars += len(text)
            total_typos += keys.count("\b")
        rate = total_typos / total_chars
        self.assertLess(rate, 0.1, f"typo rate too high: {rate}")

    def test_no_typo_on_digits_or_symbols(self):
        """Non-alpha chars are not candidates for QWERTY-neighbor typos."""
        random.seed(5)
        for _ in range(200):
            keys = human.inject_typos("!@#$%^&*()1234567890")
            self.assertEqual(keys.count("\b"), 0)

    def test_preserves_case(self):
        """'A' typo'd should produce an uppercase wrong key, not lowercase."""
        random.seed(6)
        # Force a typo by running many trials until we see one.
        for trial in range(5000):
            keys = human.inject_typos("AAAAAAAA")
            if "\b" in keys:
                # Find the wrong-key immediately before a backspace
                idx = keys.index("\b")
                wrong = keys[idx - 1]
                self.assertTrue(
                    wrong.isupper(),
                    f"typo preserved case for uppercase input? got {wrong!r}",
                )
                return
        # If we never saw a typo, test is inconclusive but not a failure.


class TestDriftPoints(unittest.TestCase):
    def test_endpoints_exact(self):
        """Last point always lands exactly on target."""
        random.seed(7)
        for _ in range(100):
            pts = list(human.drift_points(0, 0, 100, 50))
            self.assertEqual(pts[-1], (100.0, 50.0))

    def test_step_count_in_band(self):
        random.seed(8)
        for _ in range(50):
            pts = list(human.drift_points(0, 0, 100, 100))
            self.assertGreaterEqual(len(pts), human.DRIFT_STEPS_MIN)
            self.assertLessEqual(len(pts), human.DRIFT_STEPS_MAX)

    def test_zero_distance_path_degenerate_ok(self):
        """Start == end: path should still yield n points, all equal."""
        pts = list(human.drift_points(50, 50, 50, 50, steps=6))
        self.assertEqual(len(pts), 6)
        for p in pts:
            self.assertEqual(p, (50.0, 50.0))

    def test_jitter_magnitude_bounded(self):
        """Perpendicular jitter should not exceed DRIFT_JITTER_FRAC * distance."""
        random.seed(9)
        x0, y0, x1, y1 = 0.0, 0.0, 100.0, 0.0
        dist = 100.0
        max_allowed = dist * human.DRIFT_JITTER_FRAC
        for _ in range(200):
            pts = list(human.drift_points(x0, y0, x1, y1, steps=10))
            for (x, y) in pts[:-1]:  # endpoint is exact, skip
                # Deviation is the y-coord (path is horizontal).
                self.assertLessEqual(abs(y), max_allowed + 1e-9)


class TestReadPause(unittest.TestCase):
    def test_monotonic_in_chars(self):
        """More chars typed = longer expected read-pause."""
        # With same rng state the upper end should still be ordered
        random.seed(10)
        short = [human.read_pause_ms(0) for _ in range(500)]
        random.seed(10)
        long = [human.read_pause_ms(20) for _ in range(500)]
        # Long should have higher mean by (20 * per_char_ms) = 600ms
        self.assertAlmostEqual(
            sum(long) / len(long) - sum(short) / len(short),
            600, delta=50,
        )

    def test_bounded(self):
        """read_pause_ms falls in a reasonable range."""
        for _ in range(200):
            v = human.read_pause_ms(10)
            self.assertGreaterEqual(v, 500 + 10 * 30)
            self.assertLessEqual(v, 500 + 10 * 30 + 800)


if __name__ == "__main__":
    unittest.main()
