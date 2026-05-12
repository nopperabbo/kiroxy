"""Unit tests for challenge.py — detection patterns, hard-fail classification,
prompt flow.

    python3 -m unittest test_challenge.py
"""

from __future__ import annotations

import io
import unittest

import challenge as ch


class TestDetectFromHTML(unittest.TestCase):
    def test_verify_its_you(self):
        html = "Please Verify it's you before continuing."
        self.assertEqual(ch.detect_from_html(html), ch.ChallengeKind.VERIFY_ITS_YOU)

    def test_verify_its_you_indonesian(self):
        html = "Verifikasi identitas sebelum melanjutkan"
        self.assertEqual(ch.detect_from_html(html), ch.ChallengeKind.VERIFY_ITS_YOU)

    def test_recaptcha_via_iframe(self):
        html = '<iframe src="https://www.google.com/recaptcha/api2/anchor"></iframe>'
        self.assertEqual(ch.detect_from_html(html), ch.ChallengeKind.RECAPTCHA)

    def test_recaptcha_via_text(self):
        html = "Please solve the reCAPTCHA"
        self.assertEqual(ch.detect_from_html(html), ch.ChallengeKind.RECAPTCHA)

    def test_two_fa_english(self):
        html = "2-Step Verification\nEnter the code from your phone"
        self.assertEqual(ch.detect_from_html(html), ch.ChallengeKind.TWO_FA_CODE)

    def test_two_fa_indonesian(self):
        html = "Masukkan kode untuk Verifikasi 2 Langkah"
        self.assertEqual(ch.detect_from_html(html), ch.ChallengeKind.TWO_FA_CODE)

    def test_device_approval(self):
        html = "Check your phone; open the Google app and tap Yes"
        self.assertEqual(ch.detect_from_html(html), ch.ChallengeKind.DEVICE_APPROVAL)

    def test_unusual_activity(self):
        html = "We detected unusual activity on this sign-in"
        self.assertEqual(ch.detect_from_html(html), ch.ChallengeKind.UNUSUAL_ACTIVITY)

    def test_blocked(self):
        html = "Sign-in blocked. Try again later."
        self.assertEqual(ch.detect_from_html(html), ch.ChallengeKind.BLOCKED)

    def test_blocked_indonesian(self):
        html = "Akun Anda dinonaktifkan sementara"
        self.assertEqual(ch.detect_from_html(html), ch.ChallengeKind.BLOCKED)

    def test_no_challenge_on_clean_page(self):
        html = "Welcome to Kiro. Sign in with your organization."
        self.assertIsNone(ch.detect_from_html(html, "https://kiro.dev"))

    def test_empty_html(self):
        self.assertIsNone(ch.detect_from_html(""))

    def test_none_html_safe(self):
        # Function should tolerate None safely (the production caller sometimes
        # returns None on page.evaluate failure).
        self.assertIsNone(ch.detect_from_html(None, None))  # type: ignore[arg-type]

    def test_blocked_precedes_softer_kinds(self):
        """When both BLOCKED and DEVICE_APPROVAL phrases are present,
        BLOCKED wins (hard-fail must not be masked).
        """
        html = "sign-in blocked — check your phone was requested earlier"
        self.assertEqual(ch.detect_from_html(html), ch.ChallengeKind.BLOCKED)


class TestConnectionCheck(unittest.TestCase):
    URL = "https://accounts.google.com/v3/signin/challenge/pwd?checkConnection=youtube"

    def test_not_detected_immediately(self):
        self.assertIsNone(
            ch.detect_from_html("loading", self.URL, seconds_on_current_url=5),
        )

    def test_detected_after_stall(self):
        self.assertEqual(
            ch.detect_from_html("loading", self.URL, seconds_on_current_url=20),
            ch.ChallengeKind.CONNECTION_CHECK,
        )

    def test_not_detected_when_url_doesnt_match(self):
        self.assertIsNone(
            ch.detect_from_html("loading", "https://kiro.dev", seconds_on_current_url=30),
        )

    def test_concrete_challenge_takes_precedence_over_connection_check(self):
        """If there's an explicit 2FA prompt showing, we classify as 2FA even
        if the URL also has checkConnection=. Explicit wins over URL heuristic.
        """
        html = "Enter the code from your Authenticator app"
        got = ch.detect_from_html(html, self.URL, seconds_on_current_url=30)
        self.assertEqual(got, ch.ChallengeKind.TWO_FA_CODE)


class TestIsHardFail(unittest.TestCase):
    def test_blocked_is_hard_fail(self):
        self.assertTrue(ch.is_hard_fail(ch.ChallengeKind.BLOCKED))

    def test_others_are_not(self):
        for k in ch.ChallengeKind:
            if k == ch.ChallengeKind.BLOCKED:
                continue
            self.assertFalse(ch.is_hard_fail(k), f"{k} should not be hard-fail")


class TestPromptAndWait(unittest.TestCase):
    def test_returns_true_when_enter_pressed(self):
        stdin = io.StringIO("\n")
        err = io.StringIO()
        ok = ch.prompt_and_wait_for_solve(
            ch.ChallengeKind.TWO_FA_CODE,
            timeout_s=1,
            stdin=stdin,
            stderr=err,
        )
        self.assertTrue(ok)
        out = err.getvalue()
        self.assertIn("2FA", out + "code")  # loose; message includes "2FA" or "code"

    def test_returns_false_on_eof(self):
        # Empty StringIO → readline returns "" → False
        stdin = io.StringIO("")
        err = io.StringIO()
        ok = ch.prompt_and_wait_for_solve(
            ch.ChallengeKind.VERIFY_ITS_YOU,
            timeout_s=1,
            stdin=stdin,
            stderr=err,
        )
        self.assertFalse(ok)


if __name__ == "__main__":
    unittest.main()
