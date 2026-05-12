"""Unit tests for kiro_oauth.py — PKCE generation + URL + callback parsing.

Uses stdlib unittest only (no pytest dependency per brief). Run with:

    python3 -m unittest test_oauth.py
"""

from __future__ import annotations

import base64
import hashlib
import unittest
from urllib.parse import urlparse, parse_qs

import kiro_oauth


class TestPKCE(unittest.TestCase):
    def test_verifier_is_128_chars(self):
        verifier, challenge, state = kiro_oauth.generate_pkce()
        # Brief contract: len(verifier) == 128 (RFC 7636 maximum)
        self.assertEqual(len(verifier), 128, f"verifier length {len(verifier)} ≠ 128")

    def test_verifier_uses_pkce_alphabet(self):
        verifier, _, _ = kiro_oauth.generate_pkce()
        # RFC 7636 §4.1: ALPHA / DIGIT / "-" / "." / "_" / "~"
        # base64url produces A-Z a-z 0-9 - _ (no '.', no '~'), a strict subset.
        import string
        allowed = set(string.ascii_letters + string.digits + "-_")
        self.assertTrue(
            set(verifier).issubset(allowed),
            f"verifier has chars outside base64url: {set(verifier) - allowed!r}",
        )

    def test_challenge_is_sha256_b64url_of_verifier(self):
        verifier, challenge, _ = kiro_oauth.generate_pkce()
        expected_digest = hashlib.sha256(verifier.encode("ascii")).digest()
        # Reconstruct unpadded base64url from the challenge and compare digests.
        pad = "=" * (-len(challenge) % 4)
        decoded = base64.urlsafe_b64decode(challenge + pad)
        self.assertEqual(decoded, expected_digest)

    def test_state_is_nonempty_and_urlsafe(self):
        _, _, state = kiro_oauth.generate_pkce()
        self.assertGreater(len(state), 0)
        # 32 random bytes → 43 unpadded base64url chars
        self.assertEqual(len(state), 43)

    def test_each_generation_yields_fresh_values(self):
        v1, c1, s1 = kiro_oauth.generate_pkce()
        v2, c2, s2 = kiro_oauth.generate_pkce()
        self.assertNotEqual(v1, v2)
        self.assertNotEqual(c1, c2)
        self.assertNotEqual(s1, s2)


class TestLoginURL(unittest.TestCase):
    def test_google_url_shape(self):
        url = kiro_oauth.build_login_url("google", "CHALLENGE", "STATE")
        parsed = urlparse(url)
        self.assertEqual(parsed.scheme, "https")
        self.assertEqual(parsed.netloc, kiro_oauth.AUTH_HOST)
        self.assertEqual(parsed.path, "/login")
        qs = parse_qs(parsed.query)
        self.assertEqual(qs["idp"], ["Google"])
        self.assertEqual(qs["redirect_uri"], [kiro_oauth.REDIRECT_URI])
        self.assertEqual(qs["code_challenge"], ["CHALLENGE"])
        self.assertEqual(qs["code_challenge_method"], ["S256"])
        self.assertEqual(qs["state"], ["STATE"])

    def test_github_provider_accepted(self):
        url = kiro_oauth.build_login_url("github", "C", "S")
        self.assertIn("idp=Github", url)

    def test_provider_casing_normalized(self):
        self.assertIn("idp=Google", kiro_oauth.build_login_url("GOOGLE", "C", "S"))
        self.assertIn("idp=Github", kiro_oauth.build_login_url("GitHub", "C", "S"))

    def test_unknown_provider_rejected(self):
        with self.assertRaises(ValueError):
            kiro_oauth.build_login_url("facebook", "C", "S")


class TestCallbackParsing(unittest.TestCase):
    def test_happy_path(self):
        url = (
            "kiro://kiro.kiroAgent/authenticate-success"
            "?code=abc123&state=xyz789"
        )
        code, state = kiro_oauth.parse_callback_url(url)
        self.assertEqual(code, "abc123")
        self.assertEqual(state, "xyz789")

    def test_missing_code_raises(self):
        url = "kiro://kiro.kiroAgent/authenticate-success?state=xyz"
        with self.assertRaises(ValueError):
            kiro_oauth.parse_callback_url(url)

    def test_missing_state_raises(self):
        url = "kiro://kiro.kiroAgent/authenticate-success?code=abc"
        with self.assertRaises(ValueError):
            kiro_oauth.parse_callback_url(url)

    def test_wrong_scheme_rejected(self):
        with self.assertRaises(ValueError):
            kiro_oauth.parse_callback_url("https://example.com/?code=a&state=b")

    def test_empty_rejected(self):
        with self.assertRaises(ValueError):
            kiro_oauth.parse_callback_url("")


def _mint_jwt(payload: dict) -> str:
    """Build a minimal, unsigned JWT for tests (header + payload + empty sig)."""
    header = base64.urlsafe_b64encode(
        b'{"alg":"none","typ":"JWT"}'
    ).rstrip(b"=").decode("ascii")
    import json as _json
    body = base64.urlsafe_b64encode(
        _json.dumps(payload, separators=(",", ":")).encode("utf-8")
    ).rstrip(b"=").decode("ascii")
    return f"{header}.{body}."


class TestJwtSubExtraction(unittest.TestCase):
    def test_extracts_email_when_present(self):
        jwt = _mint_jwt({"email": "alice@example.com", "sub": "uuid-123"})
        self.assertEqual(kiro_oauth.jwt_sub_or_email(jwt), "alice@example.com")

    def test_falls_back_to_sub_when_no_email(self):
        jwt = _mint_jwt({"sub": "uuid-123"})
        self.assertEqual(kiro_oauth.jwt_sub_or_email(jwt), "uuid-123")

    def test_strips_whitespace_on_email(self):
        jwt = _mint_jwt({"email": "  bob@example.com  "})
        self.assertEqual(kiro_oauth.jwt_sub_or_email(jwt), "bob@example.com")

    def test_empty_token_returns_none(self):
        self.assertIsNone(kiro_oauth.jwt_sub_or_email(""))
        self.assertIsNone(kiro_oauth.jwt_sub_or_email(None))  # type: ignore[arg-type]

    def test_non_jwt_returns_none(self):
        # Kiro's current tokens have `aoa...` prefix and no dots — classic
        # opaque token shape. Must return None cleanly.
        self.assertIsNone(kiro_oauth.jwt_sub_or_email("aoaDummyTokenNoDotsHere"))

    def test_wrong_segment_count_returns_none(self):
        self.assertIsNone(kiro_oauth.jwt_sub_or_email("only.two"))
        self.assertIsNone(kiro_oauth.jwt_sub_or_email("four.segments.here.wrong"))

    def test_malformed_base64_returns_none(self):
        # "!!!" is not valid base64url.
        self.assertIsNone(kiro_oauth.jwt_sub_or_email("header.!!!.sig"))

    def test_non_json_payload_returns_none(self):
        # Valid base64url of 'not-json-at-all'
        import base64 as _b
        bad = _b.urlsafe_b64encode(b"not-json-at-all").rstrip(b"=").decode("ascii")
        self.assertIsNone(kiro_oauth.jwt_sub_or_email(f"hdr.{bad}.sig"))

    def test_payload_without_email_or_sub_returns_none(self):
        jwt = _mint_jwt({"iss": "https://example.com", "aud": "kiro"})
        self.assertIsNone(kiro_oauth.jwt_sub_or_email(jwt))

    def test_payload_with_empty_email_and_sub_returns_none(self):
        jwt = _mint_jwt({"email": "   ", "sub": ""})
        self.assertIsNone(kiro_oauth.jwt_sub_or_email(jwt))

    def test_payload_not_object_returns_none(self):
        # Payload is a JSON array, not an object.
        import base64 as _b
        body = _b.urlsafe_b64encode(b"[1,2,3]").rstrip(b"=").decode("ascii")
        self.assertIsNone(kiro_oauth.jwt_sub_or_email(f"hdr.{body}.sig"))


if __name__ == "__main__":
    unittest.main()
