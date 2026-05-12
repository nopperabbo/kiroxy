"""Unit tests for tools/onboard/batch.py — the batch orchestrator.

Covers: credential parsing, failure classification, state file round-trip,
resume semantics, abort thresholds, and the full run_batch loop with
injected fakes (no browser subprocess spawned).

    python3 -m unittest test_batch.py
"""

from __future__ import annotations

import json
import os
import tempfile
import unittest
from pathlib import Path

import batch
from batch import (
    AccountState,
    AbortReason,
    BatchState,
    Class,
    Credential,
    Kind,
    Status,
    classify_failure,
    count_unique_accounts,
    kind_class,
    load_state,
    parse_credentials_file,
    run_batch,
    save_state,
    should_abort,
)


# ──────────────────────────────────────────────────────────────────────────────
# Credential file parsing
# ──────────────────────────────────────────────────────────────────────────────


class TestCredentialParsing(unittest.TestCase):
    def _write(self, body: str) -> Path:
        tmp = tempfile.NamedTemporaryFile(
            mode="w", suffix=".txt", delete=False, encoding="utf-8"
        )
        tmp.write(body)
        tmp.close()
        self.addCleanup(os.unlink, tmp.name)
        return Path(tmp.name)

    def test_happy_path(self):
        p = self._write(
            "alice@example.com:pw-alice\n"
            "bob@example.com:pw-bob\n"
        )
        creds, reasons = parse_credentials_file(p)
        self.assertEqual(len(creds), 2)
        self.assertEqual(reasons, [])
        self.assertEqual(creds[0], Credential("alice@example.com", "pw-alice"))

    def test_comments_and_blanks_skipped(self):
        p = self._write(
            "# top comment\n"
            "\n"
            "alice@example.com:pw-alice\n"
            "    \n"
            "# another comment\n"
        )
        creds, reasons = parse_credentials_file(p)
        self.assertEqual(len(creds), 1)
        self.assertEqual(reasons, [])

    def test_passwords_with_colons(self):
        # Password field preserves colons after the first separator.
        p = self._write("david@example.com:pw:with:colons\n")
        creds, _ = parse_credentials_file(p)
        self.assertEqual(creds[0].password, "pw:with:colons")

    def test_passwords_preserve_whitespace(self):
        # Spaces in a password are legitimate. But CRLF is stripped.
        p = self._write("alice@example.com:  pw with spaces  \r\n")
        creds, _ = parse_credentials_file(p)
        self.assertEqual(creds[0].password, "  pw with spaces  ")

    def test_invalid_email_rejected(self):
        p = self._write("notanemail:pw\n")
        creds, reasons = parse_credentials_file(p)
        self.assertEqual(creds, [])
        self.assertTrue(any("not a valid email" in r for r in reasons))

    def test_empty_password_rejected(self):
        p = self._write("alice@example.com:\n")
        creds, reasons = parse_credentials_file(p)
        self.assertEqual(creds, [])
        self.assertTrue(any("empty password" in r for r in reasons))

    def test_missing_colon_rejected(self):
        p = self._write("alice@example.com-no-colon\n")
        creds, reasons = parse_credentials_file(p)
        self.assertEqual(creds, [])
        self.assertTrue(any("missing ':'" in r for r in reasons))

    def test_duplicate_email_keeps_first(self):
        p = self._write(
            "alice@example.com:first\n"
            "alice@example.com:second\n"
        )
        creds, reasons = parse_credentials_file(p)
        self.assertEqual(len(creds), 1)
        self.assertEqual(creds[0].password, "first")
        self.assertTrue(any("duplicate" in r for r in reasons))

    def test_email_lowercased(self):
        p = self._write("Alice@Example.Com:pw\n")
        creds, _ = parse_credentials_file(p)
        self.assertEqual(creds[0].email, "alice@example.com")

    def test_missing_file_raises(self):
        with self.assertRaises(FileNotFoundError):
            parse_credentials_file(Path("/nonexistent/does-not-exist.txt"))

    def test_redacted_hides_password(self):
        c = Credential("alice@example.com", "supersecret")
        r = c.redacted()
        self.assertIn("alice@example.com", r)
        self.assertNotIn("supersecret", r)


# ──────────────────────────────────────────────────────────────────────────────
# Failure classification
# ──────────────────────────────────────────────────────────────────────────────


class TestClassification(unittest.TestCase):
    def test_exit_zero_is_success(self):
        self.assertEqual(classify_failure(0, ""), Kind.SUCCESS)

    def test_exit_124_is_transient(self):
        # /usr/bin/timeout's signal.
        self.assertEqual(classify_failure(124, ""), Kind.TIMEOUT_WAIT)

    def test_classifies_hard_blocked(self):
        stderr = "error: Google hard-blocked sign-in (BLOCKED)."
        self.assertEqual(classify_failure(1, stderr), Kind.BLOCKED)

    def test_classifies_2fa(self):
        stderr = "challenge detected: VERIFY_ITS_YOU"
        self.assertEqual(classify_failure(1, stderr), Kind.TWO_FACTOR)

    def test_classifies_wrong_password(self):
        stderr = 'error: password field not fillable after submit'
        self.assertEqual(classify_failure(1, stderr), Kind.WRONG_PASS)

    def test_classifies_consent_declined(self):
        stderr = "consent_declined: user hit Cancel"
        self.assertEqual(classify_failure(1, stderr), Kind.CONSENT)

    def test_classifies_network(self):
        stderr = "httpx.ConnectionError: connection refused"
        self.assertEqual(classify_failure(1, stderr), Kind.NETWORK)

    def test_classifies_browser_crash(self):
        stderr = "BrowserDriverUnavailableError: browser closed unexpectedly"
        self.assertEqual(classify_failure(1, stderr), Kind.BROWSER)

    def test_classifies_redirect_timeout(self):
        stderr = "waiting for the kiro:// redirect timed out after 120s"
        self.assertEqual(classify_failure(1, stderr), Kind.TIMEOUT_WAIT)

    def test_hard_wins_when_multiple_markers(self):
        # When the stderr mentions both "network" (transient) and
        # "hard-blocked" (hard), we classify as hard — we don't want
        # to retry a locked account.
        stderr = (
            "NetworkError while loading, "
            "then error: Google hard-blocked sign-in."
        )
        self.assertEqual(classify_failure(1, stderr), Kind.BLOCKED)

    def test_unknown_hard_fallback(self):
        # Non-zero exit, no matching marker.
        stderr = "totally unfamiliar error text"
        self.assertEqual(classify_failure(1, stderr), Kind.UNKNOWN_HARD)

    def test_kind_class_mapping(self):
        self.assertEqual(kind_class(Kind.SUCCESS), Class.SUCCESS)
        self.assertEqual(kind_class(Kind.NETWORK), Class.TRANSIENT)
        self.assertEqual(kind_class(Kind.BROWSER), Class.TRANSIENT)
        self.assertEqual(kind_class(Kind.TIMEOUT_WAIT), Class.TRANSIENT)
        self.assertEqual(kind_class(Kind.BLOCKED), Class.HARD)
        self.assertEqual(kind_class(Kind.TWO_FACTOR), Class.HARD)
        self.assertEqual(kind_class(Kind.UNKNOWN_HARD), Class.HARD)


# ──────────────────────────────────────────────────────────────────────────────
# Abort thresholds
# ──────────────────────────────────────────────────────────────────────────────


class TestAbortThresholds(unittest.TestCase):
    def test_no_recent_no_abort(self):
        self.assertIsNone(should_abort([]))

    def test_single_success_no_abort(self):
        self.assertIsNone(should_abort([Kind.SUCCESS]))

    def test_two_consecutive_hard_no_abort(self):
        # Threshold is 3, so 2 should NOT abort.
        self.assertIsNone(should_abort([Kind.BLOCKED, Kind.BLOCKED]))

    def test_three_consecutive_hard_aborts(self):
        self.assertEqual(
            should_abort([Kind.BLOCKED, Kind.TWO_FACTOR, Kind.BLOCKED]),
            AbortReason.CONSECUTIVE_HARD_FAILS,
        )

    def test_interleaved_hard_does_not_count_as_consecutive(self):
        # Hard-Success-Hard-Hard is only 2 consecutive at the tail.
        self.assertIsNone(should_abort([
            Kind.BLOCKED, Kind.SUCCESS, Kind.BLOCKED, Kind.BLOCKED,
        ]))

    def test_transient_breaks_consecutive_hard(self):
        # Transient is neither success nor hard — but it should break
        # the all-hard tail window.
        self.assertIsNone(should_abort([
            Kind.BLOCKED, Kind.NETWORK, Kind.BLOCKED, Kind.BLOCKED,
        ]))

    def test_browser_crash_rate_aborts(self):
        # 5 attempts, 3 browser crashes = 60% > 20% threshold.
        kinds = [Kind.BROWSER] * 3 + [Kind.SUCCESS, Kind.SUCCESS]
        self.assertEqual(should_abort(kinds), AbortReason.BROWSER_CRASH_RATE)

    def test_browser_crash_rate_below_sample_size_no_abort(self):
        # Only 3 attempts — below the min-sample threshold, no abort
        # even though 100% are crashes.
        self.assertIsNone(should_abort([Kind.BROWSER, Kind.BROWSER, Kind.BROWSER]))

    def test_browser_crash_rate_under_threshold(self):
        # 10 attempts, 1 crash = 10% < 20%.
        kinds = [Kind.BROWSER] + [Kind.SUCCESS] * 9
        self.assertIsNone(should_abort(kinds))


# ──────────────────────────────────────────────────────────────────────────────
# State file round-trip
# ──────────────────────────────────────────────────────────────────────────────


class TestStateRoundTrip(unittest.TestCase):
    def test_load_missing_creates_empty(self):
        with tempfile.TemporaryDirectory() as td:
            s = load_state(Path(td) / "state.json")
            self.assertEqual(s.accounts, {})
            self.assertTrue(s.run_started_at)

    def test_save_and_reload_preserves(self):
        with tempfile.TemporaryDirectory() as td:
            p = Path(td) / "state.json"
            s = BatchState(
                accounts={
                    "alice@x.com": AccountState(
                        status="done", attempts=1,
                        last_kind="success",
                        completed_at="2026-05-13T00:00:00Z",
                    ),
                    "bob@x.com": AccountState(
                        status="failed", attempts=3,
                        last_kind="hard:blocked",
                        last_error="Google hard-blocked",
                    ),
                },
                run_started_at="2026-05-13T00:00:00Z",
            )
            save_state(p, s)
            loaded = load_state(p)
            self.assertEqual(len(loaded.accounts), 2)
            self.assertEqual(loaded.accounts["alice@x.com"].status, "done")
            self.assertEqual(loaded.accounts["bob@x.com"].attempts, 3)
            self.assertEqual(loaded.run_started_at, "2026-05-13T00:00:00Z")

    def test_save_is_atomic(self):
        # After save, only the final file exists (no .tmp).
        with tempfile.TemporaryDirectory() as td:
            p = Path(td) / "state.json"
            s = BatchState(run_started_at="2026-05-13T00:00:00Z")
            save_state(p, s)
            self.assertTrue(p.exists())
            self.assertFalse(p.with_suffix(p.suffix + ".tmp").exists())

    def test_corrupt_state_file_exits(self):
        with tempfile.TemporaryDirectory() as td:
            p = Path(td) / "state.json"
            p.write_text("not valid json", encoding="utf-8")
            with self.assertRaises(SystemExit):
                load_state(p)


# ──────────────────────────────────────────────────────────────────────────────
# Collision detection helper
# ──────────────────────────────────────────────────────────────────────────────


class TestCountUniqueAccounts(unittest.TestCase):
    def test_missing_file_returns_zero(self):
        with tempfile.TemporaryDirectory() as td:
            self.assertEqual(count_unique_accounts(Path(td) / "no.json"), 0)

    def test_single_entry(self):
        with tempfile.TemporaryDirectory() as td:
            p = Path(td) / "out.json"
            p.write_text(
                json.dumps([{
                    "email": "alice@x.com",
                    "profileArn": "arn:x/P", "accessToken": "aoa-a",
                }]),
                encoding="utf-8",
            )
            self.assertEqual(count_unique_accounts(p), 1)

    def test_workspace_two_users_distinct(self):
        # Two users, same profileArn — must still count as 2 because the
        # cascade uses email first.
        with tempfile.TemporaryDirectory() as td:
            p = Path(td) / "out.json"
            p.write_text(
                json.dumps([
                    {"email": "a@x.com", "profileArn": "arn:x/SHARED"},
                    {"email": "b@x.com", "profileArn": "arn:x/SHARED"},
                ]),
                encoding="utf-8",
            )
            self.assertEqual(count_unique_accounts(p), 2)

    def test_empty_and_malformed_entries_ignored(self):
        with tempfile.TemporaryDirectory() as td:
            p = Path(td) / "out.json"
            p.write_text(json.dumps([{}, "not-a-dict", None]), encoding="utf-8")
            self.assertEqual(count_unique_accounts(p), 0)


# ──────────────────────────────────────────────────────────────────────────────
# run_batch integration (no subprocess, fakes injected)
# ──────────────────────────────────────────────────────────────────────────────


class FakeRunner:
    """Stand-in for run_onboard_once that returns canned (exit_code, stderr)."""

    def __init__(self, scripted: list):
        # scripted: list of (exit_code, stderr) — consumed in order
        self.scripted = list(scripted)
        self.calls: list = []

    def __call__(self, cred, **kw):
        self.calls.append(cred.email)
        if not self.scripted:
            raise AssertionError("FakeRunner exhausted; unexpected call")
        return self.scripted.pop(0)


class FakeSleep:
    """Records sleep invocations; never actually sleeps."""

    def __init__(self):
        self.calls: list = []

    def __call__(self, seconds):
        self.calls.append(seconds)


def _simulate_successful_write(output_path: Path, emails: list) -> None:
    """Mimic what onboard.py would do: append entries to the output file.

    Tests use this helper when they want count_unique_accounts() to reflect
    a real growth so the collision warning doesn't trigger.
    """
    data = []
    if output_path.exists():
        try:
            data = json.loads(output_path.read_text(encoding="utf-8"))
        except Exception:
            data = []
    for e in emails:
        data.append({"email": e, "profileArn": "arn:x/P", "accessToken": f"aoa-{e}"})
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(json.dumps(data), encoding="utf-8")


class TestRunBatch(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.dir = Path(self.tmp.name)
        self.output = self.dir / "tokens.json"
        self.state = self.dir / "state.json"

    def _cred(self, email: str) -> Credential:
        return Credential(email, "pw-" + email.split("@")[0])

    def test_happy_path_all_succeed(self):
        creds = [self._cred("a@x.com"), self._cred("b@x.com")]

        # Runner reports SUCCESS for both, and simulates the output write
        # side-effect so collision check sees real growth.
        def runner(cred, **kw):
            _simulate_successful_write(self.output, [cred.email])
            return 0, ""

        sleep = FakeSleep()
        result = run_batch(
            creds, output_path=self.output, state_path=self.state,
            cooldown_s=5.0, run_one=runner, sleep_fn=sleep,
        )
        self.assertEqual(result.succeeded, 2)
        self.assertEqual(result.failed, 0)
        self.assertEqual(result.skipped, 0)
        self.assertIsNone(result.aborted)
        # One cooldown between two accounts.
        self.assertEqual(sleep.calls, [5.0])

        # State persisted.
        state = load_state(self.state)
        self.assertEqual(state.accounts["a@x.com"].status, "done")
        self.assertEqual(state.accounts["b@x.com"].status, "done")

    def test_transient_retries_up_to_max(self):
        # First attempt network-fails, then succeeds. max_retries=2 means
        # up to 3 total attempts per account.
        #
        # Note: run_batch iterates `creds` once per call; a retry comes
        # from re-running the batch. This test checks a SINGLE-run
        # transient failure marks the account PENDING so the next run
        # picks it up.
        creds = [self._cred("a@x.com")]
        runner = FakeRunner([(1, "httpx.ConnectionError: refused")])
        sleep = FakeSleep()
        result = run_batch(
            creds, output_path=self.output, state_path=self.state,
            cooldown_s=0.1, max_retries=2,
            run_one=runner, sleep_fn=sleep,
        )
        # It's not "failed" (still has retries left) and not "done".
        state = load_state(self.state)
        rec = state.accounts["a@x.com"]
        self.assertEqual(rec.status, Status.PENDING.value)
        self.assertEqual(rec.attempts, 1)
        self.assertEqual(rec.last_kind, Kind.NETWORK.value)
        # Run again — should attempt again.
        runner2 = FakeRunner([(0, "")])
        def success_runner(cred, **kw):
            _simulate_successful_write(self.output, [cred.email])
            return runner2(cred)
        run_batch(
            creds, output_path=self.output, state_path=self.state,
            cooldown_s=0.1, max_retries=2,
            run_one=success_runner, sleep_fn=sleep,
        )
        state = load_state(self.state)
        self.assertEqual(state.accounts["a@x.com"].status, Status.DONE.value)

    def test_hard_fail_is_sticky_no_retry_after_cap(self):
        creds = [self._cred("a@x.com")]
        runner = FakeRunner([(1, "Google hard-blocked sign-in (BLOCKED)")])
        sleep = FakeSleep()
        result = run_batch(
            creds, output_path=self.output, state_path=self.state,
            cooldown_s=0.1, max_retries=2,
            run_one=runner, sleep_fn=sleep,
        )
        state = load_state(self.state)
        rec = state.accounts["a@x.com"]
        # Hard failures with attempts > max_retries marked as failed — BUT
        # on the first attempt, hard fails go straight to FAILED (no retry
        # for hard kind).
        self.assertEqual(rec.status, Status.FAILED.value)
        self.assertEqual(rec.last_kind, Kind.BLOCKED.value)

    def test_resume_skips_done_retries_failed(self):
        # Pre-populate state: one done, one failed (exhausted).
        pre_state = BatchState(
            accounts={
                "a@x.com": AccountState(
                    status=Status.DONE.value, attempts=1,
                    last_kind=Kind.SUCCESS.value,
                ),
                "b@x.com": AccountState(
                    status=Status.FAILED.value, attempts=3,
                    last_kind=Kind.BLOCKED.value,
                ),
            },
            run_started_at="2026-05-13T00:00:00Z",
        )
        save_state(self.state, pre_state)

        creds = [self._cred("a@x.com"), self._cred("b@x.com"), self._cred("c@x.com")]
        visited = []
        def runner(cred, **kw):
            visited.append(cred.email)
            _simulate_successful_write(self.output, [cred.email])
            return 0, ""

        sleep = FakeSleep()
        result = run_batch(
            creds, output_path=self.output, state_path=self.state,
            cooldown_s=0.1, max_retries=2,
            run_one=runner, sleep_fn=sleep,
        )
        # a@x.com (done) skipped, b@x.com (exhausted hard-fail) skipped,
        # c@x.com actually attempted.
        self.assertEqual(visited, ["c@x.com"])
        self.assertEqual(result.skipped, 2)
        self.assertEqual(result.succeeded, 1)

    def test_abort_on_three_consecutive_hard_fails(self):
        creds = [
            self._cred("a@x.com"),
            self._cred("b@x.com"),
            self._cred("c@x.com"),
            self._cred("d@x.com"),
        ]
        runner = FakeRunner([
            (1, "Google hard-blocked"),
            (1, "Google hard-blocked"),
            (1, "Google hard-blocked"),
        ])
        sleep = FakeSleep()
        result = run_batch(
            creds, output_path=self.output, state_path=self.state,
            cooldown_s=0.1, run_one=runner, sleep_fn=sleep,
        )
        # First 3 attempted, abort triggers before d is touched.
        self.assertEqual(runner.calls, ["a@x.com", "b@x.com", "c@x.com"])
        self.assertEqual(result.aborted, AbortReason.CONSECUTIVE_HARD_FAILS)
        self.assertEqual(result.failed, 3)
        self.assertEqual(result.succeeded, 0)

    def test_single_successful_run_marks_completed_at(self):
        creds = [self._cred("a@x.com")]
        def runner(cred, **kw):
            _simulate_successful_write(self.output, [cred.email])
            return 0, ""
        run_batch(
            creds, output_path=self.output, state_path=self.state,
            cooldown_s=0.1, run_one=runner, sleep_fn=FakeSleep(),
        )
        rec = load_state(self.state).accounts["a@x.com"]
        self.assertIsNotNone(rec.completed_at)
        self.assertEqual(rec.status, Status.DONE.value)

    def test_no_cooldown_sleep_after_last_account(self):
        creds = [self._cred("a@x.com")]
        def runner(cred, **kw):
            _simulate_successful_write(self.output, [cred.email])
            return 0, ""
        sleep = FakeSleep()
        run_batch(
            creds, output_path=self.output, state_path=self.state,
            cooldown_s=60.0, run_one=runner, sleep_fn=sleep,
        )
        # Single account — no cooldown between (the "last" account never
        # gets a sleep-after).
        self.assertEqual(sleep.calls, [])


class TestSummaryRendering(unittest.TestCase):
    def test_summary_lists_failed_accounts(self):
        with tempfile.TemporaryDirectory() as td:
            state_path = Path(td) / "state.json"
            save_state(state_path, BatchState(
                accounts={
                    "a@x.com": AccountState(
                        status=Status.FAILED.value, attempts=3,
                        last_kind=Kind.BLOCKED.value,
                    ),
                    "b@x.com": AccountState(
                        status=Status.DONE.value, attempts=1,
                        last_kind=Kind.SUCCESS.value,
                    ),
                },
                run_started_at="2026-05-13T00:00:00Z",
            ))
            creds = [
                Credential("a@x.com", "pw"),
                Credential("b@x.com", "pw"),
            ]
            from batch import BatchResult, _summary
            result = BatchResult(succeeded=1, failed=1)
            text = _summary(result, total=2, state_path=state_path, creds=creds)
            self.assertIn("a@x.com", text)
            self.assertIn("hard:blocked", text)
            self.assertIn("1/2 succeeded", text)


if __name__ == "__main__":
    unittest.main()
