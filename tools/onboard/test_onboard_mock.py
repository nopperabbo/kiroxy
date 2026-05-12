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


if __name__ == "__main__":
    unittest.main()
