"""
Mock Kiro login server for integration testing the non-Google portions of
onboard.py. Phase G.FIX.

Serves a tiny HTML page that mimics the structure of the Kiro auth page:
shows a "Continue with Google" button, but clicking it immediately
redirects to ``kiro://kiro.kiroAgent/authenticate-success?code=…&state=…``
using the state value we embedded in the /login query string.

The server also exposes /oauth/token to return fake token JSON so the
full onboard flow can be exercised end-to-end without touching Google or
the real Kiro backend. When running the full pipeline, the operator
points onboard.py at a mock /login URL (via test-only env overrides) and
asserts the resulting JSON has the expected shape.

This is stdlib-only: http.server + threading. No pytest required.
"""

from __future__ import annotations

import contextlib
import json
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from typing import Optional, Tuple
from urllib.parse import parse_qs, urlparse

# Fixed token blob returned for every /oauth/token request. Consumers use
# these exact values to assert that the upsert path wrote the right thing.
MOCK_TOKEN_RESPONSE = {
    "accessToken": "aoa_MOCK_ACCESS_TOKEN_FOR_TESTING",
    "refreshToken": "aor_MOCK_REFRESH_TOKEN_FOR_TESTING",
    "profileArn": "arn:aws:codewhisperer:us-east-1:000000000000:profile/MOCKTEST123",
    "expiresIn": 3600,
}

# Fake auth code handed out on the /login page. Paired with whatever state
# the client passed in.
MOCK_AUTH_CODE = "MOCK_AUTH_CODE_123"


class _MockKiroHandler(BaseHTTPRequestHandler):
    # Quiet the default access log (uses stderr).
    def log_message(self, format: str, *args) -> None:  # noqa: A002 — signature
        return

    def do_GET(self) -> None:  # noqa: N802 — BaseHTTPRequestHandler API
        parsed = urlparse(self.path)
        if parsed.path == "/login":
            qs = parse_qs(parsed.query)
            state = (qs.get("state") or [""])[0]
            body = self._login_html(state).encode("utf-8")
            self.send_response(200)
            self.send_header("Content-Type", "text/html; charset=utf-8")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)
            return
        if parsed.path == "/redirect":
            qs = parse_qs(parsed.query)
            state = (qs.get("state") or [""])[0]
            # Emit a 302 to the kiro:// scheme. Firefox will treat this as a
            # protocol-handler prompt, which fires framenavigated before
            # ultimately cancelling; the listener catches the URL just fine.
            target = (
                f"kiro://kiro.kiroAgent/authenticate-success"
                f"?code={MOCK_AUTH_CODE}&state={state}"
            )
            self.send_response(302)
            self.send_header("Location", target)
            self.end_headers()
            return
        self.send_error(404, "not found")

    def do_POST(self) -> None:  # noqa: N802
        parsed = urlparse(self.path)
        if parsed.path == "/oauth/token":
            length = int(self.headers.get("Content-Length") or 0)
            raw = self.rfile.read(length) if length else b""
            try:
                req = json.loads(raw.decode("utf-8"))
            except Exception:
                self.send_error(400, "bad JSON")
                return
            # Minimal validation: code must match what /login hands out.
            if req.get("code") != MOCK_AUTH_CODE:
                self.send_error(400, "bad code")
                return
            body = json.dumps(MOCK_TOKEN_RESPONSE).encode("utf-8")
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)
            return
        self.send_error(404, "not found")

    @staticmethod
    def _login_html(state: str) -> str:
        # Minimal page: "Continue with Google" link that hits /redirect?state=…
        return (
            "<!doctype html><html><head><title>Mock Kiro Login</title></head>"
            "<body>"
            f"<h1>Mock Kiro Login</h1>"
            f"<a id='continue' href='/redirect?state={state}'>Continue with Google</a>"
            "</body></html>"
        )


class MockKiroServer:
    """Context-managed mock server. Binds to an ephemeral port on 127.0.0.1.

    Usage::

        with MockKiroServer() as server:
            login_url = server.login_url(state="xyz")
            ...  # drive a browser or httpx client against login_url
    """

    def __init__(self, host: str = "127.0.0.1") -> None:
        self._host = host
        self._httpd: Optional[ThreadingHTTPServer] = None
        self._thread: Optional[threading.Thread] = None

    def __enter__(self) -> "MockKiroServer":
        self._httpd = ThreadingHTTPServer((self._host, 0), _MockKiroHandler)
        self._thread = threading.Thread(
            target=self._httpd.serve_forever, daemon=True
        )
        self._thread.start()
        return self

    def __exit__(self, exc_type, exc, tb) -> None:
        with contextlib.suppress(Exception):
            if self._httpd is not None:
                self._httpd.shutdown()
                self._httpd.server_close()
        self._httpd = None
        self._thread = None

    @property
    def address(self) -> Tuple[str, int]:
        if self._httpd is None:
            raise RuntimeError("MockKiroServer used outside a `with` block")
        return self._httpd.server_address

    @property
    def base_url(self) -> str:
        host, port = self.address
        return f"http://{host}:{port}"

    def login_url(self, state: str = "MOCK_STATE") -> str:
        return f"{self.base_url}/login?state={state}"

    def token_url(self) -> str:
        return f"{self.base_url}/oauth/token"
