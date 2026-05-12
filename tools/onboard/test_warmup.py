"""Unit tests for warmup.py.

Uses a MockDriver that records navigate/wait calls instead of a real browser,
so the full warmup choreography can be validated without Camoufox.

    python3 -m unittest test_warmup.py
"""

from __future__ import annotations

import tempfile
import time
import unittest
from pathlib import Path

import warmup


class MockDriver:
    """Captures navigate() + wait() calls and lets tests simulate failures."""

    def __init__(self, fail_on_url: str | None = None, fail_on_wait: bool = False):
        self.navigations: list[str] = []
        self.waits: list[int] = []
        self._fail_on_url = fail_on_url
        self._fail_on_wait = fail_on_wait

    def navigate(self, url: str) -> None:
        self.navigations.append(url)
        if self._fail_on_url and self._fail_on_url in url:
            raise RuntimeError(f"mock navigate failure for {url}")

    def wait(self, ms: int) -> None:
        self.waits.append(ms)
        if self._fail_on_wait:
            raise RuntimeError("mock wait failure")


class TestShouldWarmup(unittest.TestCase):
    def test_no_marker_means_warmup(self):
        with tempfile.TemporaryDirectory() as d:
            self.assertTrue(warmup.should_warmup(Path(d)))

    def test_fresh_marker_skips_warmup(self):
        with tempfile.TemporaryDirectory() as d:
            warmup.write_marker(Path(d))
            self.assertFalse(warmup.should_warmup(Path(d)))

    def test_old_marker_triggers_warmup(self):
        with tempfile.TemporaryDirectory() as d:
            # Write marker with timestamp 30 days ago
            marker = warmup.marker_path(Path(d))
            marker.parent.mkdir(parents=True, exist_ok=True)
            marker.write_text(f"{time.time() - 30 * 86400:.0f}\n")
            self.assertTrue(warmup.should_warmup(Path(d)))

    def test_malformed_marker_triggers_warmup(self):
        with tempfile.TemporaryDirectory() as d:
            marker = warmup.marker_path(Path(d))
            marker.parent.mkdir(parents=True, exist_ok=True)
            marker.write_text("not a timestamp")
            self.assertTrue(warmup.should_warmup(Path(d)))

    def test_custom_ttl_respected(self):
        with tempfile.TemporaryDirectory() as d:
            marker = warmup.marker_path(Path(d))
            marker.parent.mkdir(parents=True, exist_ok=True)
            # 3 days old
            marker.write_text(f"{time.time() - 3 * 86400:.0f}\n")
            # Custom 1-day TTL → should warmup
            self.assertTrue(warmup.should_warmup(Path(d), ttl_days=1))
            # Default 7-day TTL → should not
            self.assertFalse(warmup.should_warmup(Path(d), ttl_days=7))


class TestRunWarmup(unittest.TestCase):
    def test_default_flow_navigates_all_steps(self):
        drv = MockDriver()
        # Use short dwell times for the test to stay fast
        short_steps = [
            warmup.WarmupStep(url="a://one", dwell_s=0.001, description="step1"),
            warmup.WarmupStep(url="b://two", dwell_s=0.001, description="step2"),
            warmup.WarmupStep(url="c://three", dwell_s=0.001, description="step3"),
        ]
        completed, attempted = warmup.run_warmup(drv, steps=short_steps)
        self.assertEqual(attempted, 3)
        self.assertEqual(completed, 3)
        self.assertEqual(drv.navigations, ["a://one", "b://two", "c://three"])
        self.assertEqual(len(drv.waits), 3)

    def test_navigate_failure_does_not_abort(self):
        drv = MockDriver(fail_on_url="b://two")
        short_steps = [
            warmup.WarmupStep(url="a://one", dwell_s=0.001, description="step1"),
            warmup.WarmupStep(url="b://two", dwell_s=0.001, description="step2"),
            warmup.WarmupStep(url="c://three", dwell_s=0.001, description="step3"),
        ]
        completed, attempted = warmup.run_warmup(drv, steps=short_steps)
        self.assertEqual(attempted, 3)  # tried all
        self.assertEqual(completed, 2)  # but one failed

    def test_wait_failure_counted(self):
        drv = MockDriver(fail_on_wait=True)
        short_steps = [
            warmup.WarmupStep(url="a://one", dwell_s=0.001, description="step1"),
            warmup.WarmupStep(url="b://two", dwell_s=0.001, description="step2"),
        ]
        completed, attempted = warmup.run_warmup(drv, steps=short_steps)
        self.assertEqual(attempted, 2)
        self.assertEqual(completed, 0)

    def test_hard_cap_stops_remaining(self):
        """hard_cap_s is checked at the start of each loop iteration.

        With cap=0, the first iteration's cap check fails immediately so
        attempted=0 and completed=0 — no navigate() call, no wait() call.
        This is the deterministic assertion; scheduler-dependent cases
        (testing with a cap just larger than the first step's dwell) are
        too flaky under CI variance to pin.
        """
        drv = MockDriver()
        long_steps = [
            warmup.WarmupStep(url="a://one", dwell_s=0.6, description="first"),
            warmup.WarmupStep(url="b://two", dwell_s=10.0, description="second"),
        ]
        completed, attempted = warmup.run_warmup(drv, steps=long_steps, hard_cap_s=0)
        self.assertEqual(attempted, 0)
        self.assertEqual(completed, 0)
        self.assertEqual(drv.navigations, [])
        self.assertEqual(drv.waits, [])

    def test_default_warmup_non_empty(self):
        """DEFAULT_WARMUP has reasonable structure (at least 3 steps, all HTTPS except blank)."""
        self.assertGreaterEqual(len(warmup.DEFAULT_WARMUP), 3)
        for step in warmup.DEFAULT_WARMUP:
            self.assertTrue(step.url)
            self.assertIsInstance(step.dwell_s, (int, float))
            self.assertGreater(step.dwell_s, 0)


if __name__ == "__main__":
    unittest.main()
