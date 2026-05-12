"""Integration test against the mock_kiro fixture.

Verifies the non-Google portions of the onboarder work end-to-end:

    1. PKCE generation → login URL building
    2. Mock /login serves an HTML page that redirects to kiro:// with code + state
    3. httpx follows the chain and captures the callback URL
    4. parse_callback_url extracts code + state
    5. exchange_code POSTs to mock /oauth/token
    6. Response shape matches what onboard.py expects (accessToken, refreshToken,
       profileArn, expiresIn)

Skips Camoufox entirely. This validates the non-browser half of the pipeline.

    python3 -m unittest test_onboard_mock.py
"""

from __future__ import annotations

import json
import unittest
from unittest.mock import patch

import httpx
import kiro_oauth
from fixtures.mock_kiro import (
    MOCK_AUTH_CODE,
    MOCK_TOKEN_RESPONSE,
    MockKiroServer,
)


class TestMockKiroRoundTrip(unittest.TestCase):
    def test_login_page_serves(self):
        """GET /login?state=… returns HTML with the state embedded."""
        with MockKiroServer() as server:
            resp = httpx.get(server.login_url(state="ABC123"), timeout=5)
            self.assertEqual(resp.status_code, 200)
            self.assertIn("ABC123", resp.text)
            self.assertIn("Continue with Google", resp.text)

    def test_redirect_issues_kiro_scheme(self):
        """GET /redirect?state=… returns a 302 to kiro://…?code=…&state=…."""
        with MockKiroServer() as server:
            # Follow=False so we see the raw Location header.
            resp = httpx.get(
                f"{server.base_url}/redirect?state=XYZ999",
                follow_redirects=False,
                timeout=5,
            )
            self.assertEqual(resp.status_code, 302)
            loc = resp.headers["Location"]
            self.assertTrue(loc.startswith("kiro://"))
            code, state = kiro_oauth.parse_callback_url(loc)
            self.assertEqual(code, MOCK_AUTH_CODE)
            self.assertEqual(state, "XYZ999")

    def test_token_exchange_happy_path(self):
        """POST /oauth/token with the issued code returns the mock token blob."""
        with MockKiroServer() as server:
            body = {
                "code": MOCK_AUTH_CODE,
                "code_verifier": "dummy_verifier",
                "redirect_uri": kiro_oauth.REDIRECT_URI,
            }
            resp = httpx.post(server.token_url(), json=body, timeout=5)
            self.assertEqual(resp.status_code, 200)
            self.assertEqual(resp.json(), MOCK_TOKEN_RESPONSE)

    def test_token_exchange_rejects_wrong_code(self):
        with MockKiroServer() as server:
            body = {
                "code": "WRONG_CODE",
                "code_verifier": "dummy",
                "redirect_uri": kiro_oauth.REDIRECT_URI,
            }
            resp = httpx.post(server.token_url(), json=body, timeout=5)
            self.assertEqual(resp.status_code, 400)

    def test_exchange_code_against_mock(self):
        """kiro_oauth.exchange_code() works against mock when pointed at it.

        We monkey-patch AUTH_BASE_URL for the duration of this test so the
        module's existing exchange_code function targets the mock server.
        """
        with MockKiroServer() as server:
            with patch.object(kiro_oauth, "AUTH_BASE_URL", server.base_url):
                data = kiro_oauth.exchange_code(MOCK_AUTH_CODE, "dummy_verifier")
        self.assertEqual(data["accessToken"], MOCK_TOKEN_RESPONSE["accessToken"])
        self.assertEqual(data["refreshToken"], MOCK_TOKEN_RESPONSE["refreshToken"])
        self.assertEqual(data["profileArn"], MOCK_TOKEN_RESPONSE["profileArn"])
        self.assertEqual(data["expiresIn"], MOCK_TOKEN_RESPONSE["expiresIn"])

    def test_exchange_code_surfaces_upstream_errors(self):
        with MockKiroServer() as server:
            with patch.object(kiro_oauth, "AUTH_BASE_URL", server.base_url):
                with self.assertRaises(kiro_oauth.TokenExchangeError):
                    kiro_oauth.exchange_code("WRONG_CODE", "dummy")


import onboard  # noqa: E402 — intentional late import; module imports heavy browser deps


class TestDedupeKey(unittest.TestCase):
    """BUG 4: dedupe by email, not profileArn.

    Workspace accounts within the same Kiro org share a profileArn, so the
    legacy profileArn-only key silently collapsed N users into 1 vault
    entry. New cascade: email → JWT claim → profileArn → token prefix.
    """

    def test_email_wins_over_profile_arn(self):
        e = {
            "email": "alice@dineu.tech",
            "profileArn": "arn:aws:codewhisperer:us-east-1:x/SHARED",
            "accessToken": "aoa-opaque-alice",
        }
        self.assertEqual(onboard._dedupe_key(e), "email:alice@dineu.tech")

    def test_email_is_case_insensitive(self):
        e = {"email": "Alice@Dineu.Tech", "profileArn": "arn:x/P"}
        self.assertEqual(onboard._dedupe_key(e), "email:alice@dineu.tech")

    def test_email_is_whitespace_trimmed(self):
        e = {"email": "  bob@dineu.tech  "}
        self.assertEqual(onboard._dedupe_key(e), "email:bob@dineu.tech")

    def test_workspace_same_profile_arn_two_emails_are_distinct(self):
        """Regression for BUG 4: two users, same profileArn, must get distinct keys."""
        shared_arn = "arn:aws:codewhisperer:us-east-1:111222/WORKSPACE_SHARED"
        e1 = {"email": "user1@dineu.tech", "profileArn": shared_arn}
        e2 = {"email": "user2@dineu.tech", "profileArn": shared_arn}
        self.assertNotEqual(onboard._dedupe_key(e1), onboard._dedupe_key(e2))

    def test_falls_back_to_profile_arn_without_email(self):
        # Legacy JSON file that predates BUG 4 fix.
        e = {"profileArn": "arn:aws:codewhisperer:us-east-1:x/LEGACY"}
        self.assertEqual(onboard._dedupe_key(e), "arn:LEGACY")

    def test_falls_back_to_token_prefix_without_email_or_arn(self):
        e = {"accessToken": "aoa-token-abcdefghij-more"}
        self.assertEqual(onboard._dedupe_key(e), "at:aoa-token-ab")

    def test_empty_entry_returns_empty_key(self):
        self.assertEqual(onboard._dedupe_key({}), "")


class TestUpsert(unittest.TestCase):
    def test_upsert_by_email_rotates_in_place(self):
        entries = [{
            "email": "alice@dineu.tech",
            "accessToken": "aoa-old",
            "refreshToken": "aor-old",
            "profileArn": "arn:aws:codewhisperer:us-east-1:x/SHARED",
        }]
        new = {
            "email": "alice@dineu.tech",
            "accessToken": "aoa-new",
            "refreshToken": "aor-new",
            "profileArn": "arn:aws:codewhisperer:us-east-1:x/SHARED",
        }
        action = onboard._upsert(entries, new)
        self.assertEqual(action, "updated")
        self.assertEqual(len(entries), 1)
        self.assertEqual(entries[0]["accessToken"], "aoa-new")

    def test_upsert_two_workspace_users_land_as_two_entries(self):
        """BUG 4 regression: two Workspace users must NOT collapse into one entry."""
        shared_arn = "arn:aws:codewhisperer:us-east-1:x/WORKSPACE_SHARED"
        entries: list = []
        action1 = onboard._upsert(entries, {
            "email": "user1@dineu.tech",
            "accessToken": "aoa-one",
            "refreshToken": "aor-one",
            "profileArn": shared_arn,
        })
        action2 = onboard._upsert(entries, {
            "email": "user2@dineu.tech",
            "accessToken": "aoa-two",
            "refreshToken": "aor-two",
            "profileArn": shared_arn,
        })
        self.assertEqual(action1, "added")
        self.assertEqual(action2, "added")
        self.assertEqual(len(entries), 2)
        emails = sorted(e["email"] for e in entries)
        self.assertEqual(emails, ["user1@dineu.tech", "user2@dineu.tech"])


if __name__ == "__main__":
    unittest.main()
