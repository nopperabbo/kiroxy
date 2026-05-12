"""
PKCE + URL building + /oauth/token exchange for Kiro Desktop OAuth.

Kiro Desktop's authorization endpoint is AWS-hosted at:
  https://prod.us-east-1.auth.desktop.kiro.dev

Flow:
  login   → GET /login?idp=…&redirect_uri=…&code_challenge=…&code_challenge_method=S256&state=…
            (browser drives through Google/GitHub and is redirected back to
             kiro://kiro.kiroAgent/authenticate-success?code=…&state=…)
  token   → POST /oauth/token with JSON {code, code_verifier, redirect_uri}
            returns {accessToken, refreshToken, profileArn, expiresIn}

Pure stdlib + httpx. No camoufox / patchright dependency in this module so
unit tests can exercise PKCE without the browser toolchain installed.
"""

from __future__ import annotations

import base64
import hashlib
import json
import os
import re
from typing import Optional, Tuple
from urllib.parse import urlencode, urlparse, parse_qs

import httpx

# ──────────────────────────────────────────────────────────────────────────────
# Endpoint constants. Only us-east-1 is DNS-registered (verified during Phase
# C.2 region sweep). Do not parametrize without re-verifying upstream DNS.
# ──────────────────────────────────────────────────────────────────────────────

AUTH_HOST = "prod.us-east-1.auth.desktop.kiro.dev"
AUTH_BASE_URL = f"https://{AUTH_HOST}"
LOGIN_PATH = "/login"
TOKEN_PATH = "/oauth/token"
REDIRECT_URI = "kiro://kiro.kiroAgent/authenticate-success"

# Valid idp values per Kiro IDE source. Extend only after verifying upstream
# actually accepts the value — bad values produce generic 400s with no signal.
_VALID_IDPS = {"Google", "Github"}

# RFC 7636 PKCE: verifier is 43-128 characters from unreserved URL alphabet.
# We target exactly 128 (the max) for maximum entropy. 96 random bytes base64url-
# encoded (no padding) is exactly 128 chars, satisfying both the brief's
# `len(verifier) == 128` assertion and RFC 7636.
_VERIFIER_BYTES = 96
_STATE_BYTES = 32

_HTTP_TIMEOUT_S = 30.0


# ──────────────────────────────────────────────────────────────────────────────
# PKCE
# ──────────────────────────────────────────────────────────────────────────────


def _b64url(data: bytes) -> str:
    """RFC 4648 §5 base64url encoding, unpadded (PKCE spec requires unpadded)."""
    return base64.urlsafe_b64encode(data).rstrip(b"=").decode("ascii")


def generate_pkce() -> Tuple[str, str, str]:
    """Generate a fresh (verifier, challenge, state) triple.

    verifier  — 128-char RFC 7636 code_verifier (96 random bytes b64url)
    challenge — base64url(sha256(verifier)); used as code_challenge with method S256
    state     — 43-char opaque token for CSRF binding between /login and callback
    """
    verifier = _b64url(os.urandom(_VERIFIER_BYTES))
    challenge = _b64url(hashlib.sha256(verifier.encode("ascii")).digest())
    state = _b64url(os.urandom(_STATE_BYTES))
    return verifier, challenge, state


# ──────────────────────────────────────────────────────────────────────────────
# URL building
# ──────────────────────────────────────────────────────────────────────────────


def build_login_url(provider: str, challenge: str, state: str) -> str:
    """Compose the Kiro Desktop login URL for the given social IDP.

    provider is user-facing and case-insensitive ('google' / 'github'); we
    normalize to the exact casing Kiro expects.
    """
    norm = {"google": "Google", "github": "Github"}.get(provider.lower())
    if norm is None:
        raise ValueError(
            f"unsupported provider {provider!r}; expected one of {sorted(_VALID_IDPS)}"
        )
    params = {
        "idp": norm,
        "redirect_uri": REDIRECT_URI,
        "code_challenge": challenge,
        "code_challenge_method": "S256",
        "state": state,
    }
    return f"{AUTH_BASE_URL}{LOGIN_PATH}?{urlencode(params)}"


# ──────────────────────────────────────────────────────────────────────────────
# Callback parsing
# ──────────────────────────────────────────────────────────────────────────────

# Matches kiro://kiro.kiroAgent/authenticate-success?code=…&state=… .
# urlparse handles non-http schemes, but the netloc rules differ; we use a
# regex as a defensive double-check on the scheme + host portion.
_KIRO_CB_PREFIX = re.compile(
    r"^kiro://kiro\.kiroAgent/authenticate-success(?:\?|$)", re.IGNORECASE
)


def parse_callback_url(url: str) -> Tuple[str, str]:
    """Extract (code, state) from the kiro://… redirect.

    Raises ValueError on anything that isn't a well-formed Kiro callback.
    """
    if not url or not _KIRO_CB_PREFIX.match(url):
        raise ValueError(f"not a Kiro desktop callback URL: {url!r}")
    parsed = urlparse(url)
    qs = parse_qs(parsed.query, keep_blank_values=False)
    code = (qs.get("code") or [""])[0]
    state = (qs.get("state") or [""])[0]
    if not code:
        raise ValueError("callback URL missing 'code' parameter")
    if not state:
        raise ValueError("callback URL missing 'state' parameter")
    return code, state


# ──────────────────────────────────────────────────────────────────────────────
# Token exchange
# ──────────────────────────────────────────────────────────────────────────────


class TokenExchangeError(RuntimeError):
    """Raised when /oauth/token returns a non-2xx response."""


def exchange_code(code: str, verifier: str) -> dict:
    """POST /oauth/token and return the decoded response body as a dict.

    Expected keys on success: accessToken, refreshToken, profileArn, expiresIn.
    Raises TokenExchangeError on non-2xx; the error message includes the HTTP
    status and body text so callers can distinguish credential/PKCE/network
    problems.
    """
    if not code:
        raise ValueError("code must be non-empty")
    if not verifier:
        raise ValueError("verifier must be non-empty")

    url = f"{AUTH_BASE_URL}{TOKEN_PATH}"
    body = {
        "code": code,
        "code_verifier": verifier,
        "redirect_uri": REDIRECT_URI,
    }
    with httpx.Client(timeout=_HTTP_TIMEOUT_S) as client:
        resp = client.post(url, json=body)
    if resp.status_code >= 300:
        # Truncate body for safety; tokens could in theory end up in the error
        # body on some upstream shapes. 400 chars is enough to see the error
        # type without leaking a whole response payload.
        snippet = (resp.text or "")[:400]
        raise TokenExchangeError(
            f"token exchange failed: HTTP {resp.status_code}: {snippet}"
        )

    try:
        data = resp.json()
    except Exception as e:  # noqa: BLE001 — httpx bubbles up json.JSONDecodeError
        raise TokenExchangeError(
            f"token response not JSON (HTTP {resp.status_code}): {e}"
        ) from e
    if not isinstance(data, dict):
        raise TokenExchangeError(f"token response not a JSON object: {type(data).__name__}")
    return data


# ─────────────────────────────────────────────────────────────────────────────
# JWT claim extraction (defensive fallback for email-based dedupe)
#
# Kiro's current accessTokens are opaque (`aoa...` prefix) and NOT JWTs, so
# `jwt_sub_or_email` returns None for them. The helper exists so that future
# token shape changes (or third-party callers who import kiro_oauth) can
# derive a stable per-account identifier when the CLI --email flag is not
# the authoritative source.
#
# Contract: returns the 'email' claim if present, else 'sub', else None.
# NEVER raises — any decode / JSON / claim issue yields None so the caller
# can cleanly fall back to other id sources (profileArn, token prefix).
# ─────────────────────────────────────────────────────────────────────────────


def jwt_sub_or_email(token: str) -> Optional[str]:
    """Extract 'email' or 'sub' claim from a JWT access token, or None.

    Defensive: returns None for empty strings, non-JWT shapes, malformed
    base64, non-JSON payloads, objects without either claim, etc. Never
    raises.

    Claim preference: 'email' wins over 'sub' because 'sub' may be an
    opaque provider id (UUID, subject identifier) with no human mapping.
    """
    if not token or not isinstance(token, str):
        return None
    # JWT shape: header.payload.signature — exactly 3 segments.
    parts = token.split(".")
    if len(parts) != 3:
        return None
    payload_b64 = parts[1]
    # base64url — pad to a multiple of 4.
    pad = "=" * (-len(payload_b64) % 4)
    try:
        payload_bytes = base64.urlsafe_b64decode(payload_b64 + pad)
    except Exception:  # noqa: BLE001
        return None
    try:
        claims = json.loads(payload_bytes)
    except Exception:  # noqa: BLE001
        return None
    if not isinstance(claims, dict):
        return None
    email = claims.get("email")
    if isinstance(email, str) and email.strip():
        return email.strip()
    sub = claims.get("sub")
    if isinstance(sub, str) and sub.strip():
        return sub.strip()
    return None


__all__ = [
    "AUTH_HOST",
    "AUTH_BASE_URL",
    "REDIRECT_URI",
    "TokenExchangeError",
    "build_login_url",
    "exchange_code",
    "generate_pkce",
    "jwt_sub_or_email",
    "parse_callback_url",
]
