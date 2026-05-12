"""
Residential proxy support — env var, CLI override, egress validation.

Phase G.FIX Layer 2.

Format accepted for proxy URLs:

  http://user:pass@host:port
  http://host:port              (no auth)
  https://user:pass@host:port
  socks5://user:pass@host:port

Parsing yields a dict shape Camoufox expects::

  {"server": "http://host:port", "username": "user", "password": "pass"}

No proxy auth in the `server` field — Camoufox splits it internally.

Egress validation: if a proxy is configured, we do a quick httpx.get
through it to api.ipify.org to confirm it works and to discover the
egress IP (passed to Camoufox as geoip=<ip> so timezone/locale match
the proxy's region). Validation failures are reported clearly; the
caller decides whether to fall back to no-proxy.

Do NOT ship a default proxy. Do NOT fetch free proxy lists. Operator
provides their own residential proxy or opts out.
"""

from __future__ import annotations

import os
from dataclasses import dataclass
from typing import Optional, Tuple
from urllib.parse import urlparse, urlunparse, unquote

import httpx

ENV_PROXY_VAR = "KIROXY_ONBOARD_PROXY"
EGRESS_PROBE_URL = "https://api.ipify.org"
EGRESS_PROBE_TIMEOUT_S = 15.0


class ProxyConfigError(ValueError):
    """Raised when a proxy URL is malformed or unsupported."""


@dataclass(frozen=True)
class ProxyConfig:
    server: str  # scheme://host:port (no auth)
    username: Optional[str] = None
    password: Optional[str] = None

    def as_camoufox_dict(self) -> dict:
        """Return the shape Camoufox's `proxy=` kwarg expects."""
        d: dict = {"server": self.server}
        if self.username is not None:
            d["username"] = self.username
        if self.password is not None:
            d["password"] = self.password
        return d

    def as_httpx_url(self) -> str:
        """Return the URL shape httpx wants (userinfo embedded)."""
        parsed = urlparse(self.server)
        netloc = parsed.netloc
        if self.username is not None:
            userinfo = self.username
            if self.password is not None:
                userinfo = f"{userinfo}:{self.password}"
            netloc = f"{userinfo}@{netloc}"
        return urlunparse((parsed.scheme, netloc, parsed.path or "", "", "", ""))


def parse_proxy_url(url: str) -> ProxyConfig:
    """Parse a proxy URL into scheme://host:port + credentials.

    Supported schemes: http, https, socks5 (Camoufox supports all three).
    Credentials may be percent-encoded (e.g. passwords with special chars);
    we unquote on extraction so the plain values reach Camoufox.
    """
    if not url or not isinstance(url, str):
        raise ProxyConfigError("proxy URL must be a non-empty string")
    url = url.strip()
    parsed = urlparse(url)
    if parsed.scheme not in ("http", "https", "socks5"):
        raise ProxyConfigError(
            f"proxy scheme {parsed.scheme!r} not supported (use http / https / socks5)"
        )
    if not parsed.hostname:
        raise ProxyConfigError(f"proxy URL missing host: {url!r}")
    port = parsed.port  # may be None
    if port is None:
        raise ProxyConfigError(f"proxy URL missing port: {url!r}")

    server = f"{parsed.scheme}://{parsed.hostname}:{port}"
    username = unquote(parsed.username) if parsed.username else None
    password = unquote(parsed.password) if parsed.password else None
    return ProxyConfig(server=server, username=username, password=password)


def resolve_proxy(
    cli_flag: Optional[str],
    env_var_name: str = ENV_PROXY_VAR,
) -> Optional[ProxyConfig]:
    """CLI flag beats env var. Return None if neither is set.

    cli_flag='' (explicit empty) also means unset; treat as None.
    """
    raw = (cli_flag or "").strip() or os.environ.get(env_var_name, "").strip()
    if not raw:
        return None
    return parse_proxy_url(raw)


def validate_egress(
    proxy: ProxyConfig,
    probe_url: str = EGRESS_PROBE_URL,
    timeout_s: float = EGRESS_PROBE_TIMEOUT_S,
) -> Tuple[bool, str]:
    """Attempt an HTTPS GET through the proxy and return (ok, detail).

    On success, `detail` is the egress IP reported by probe_url (e.g.
    "203.0.113.42"). On failure, `detail` is a short error description.

    httpx accepts the proxy URL directly for http/https proxies. For socks5,
    httpx needs the `httpx[socks]` extra which may or may not be installed;
    we report a clean warning on that missing dependency.
    """
    try:
        proxy_url = proxy.as_httpx_url()
        # httpx 0.28+: use `proxy=` kwarg on Client.
        with httpx.Client(proxy=proxy_url, timeout=timeout_s) as client:
            resp = client.get(probe_url)
            resp.raise_for_status()
        egress = (resp.text or "").strip()
        if not egress:
            return False, "probe returned empty body"
        return True, egress
    except httpx.ProxyError as e:
        return False, f"proxy connection failed: {e.__class__.__name__}"
    except httpx.HTTPError as e:
        return False, f"HTTP error through proxy: {e.__class__.__name__}: {e}"
    except ImportError as e:
        # httpx raises ImportError for socks when httpx[socks] is missing.
        return False, (
            "socks proxy support not installed; "
            "run `pip install 'httpx[socks]'`"
        )
    except Exception as e:  # noqa: BLE001
        return False, f"{e.__class__.__name__}: {e}"


__all__ = [
    "ENV_PROXY_VAR",
    "ProxyConfig",
    "ProxyConfigError",
    "parse_proxy_url",
    "resolve_proxy",
    "validate_egress",
]
