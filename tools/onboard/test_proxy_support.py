"""Unit tests for proxy_support.py.

    python3 -m unittest test_proxy_support.py
"""

from __future__ import annotations

import os
import unittest
from unittest.mock import patch

import proxy_support as ps


class TestParseProxyURL(unittest.TestCase):
    def test_http_with_auth(self):
        c = ps.parse_proxy_url("http://u:p@host.example:8080")
        self.assertEqual(c.server, "http://host.example:8080")
        self.assertEqual(c.username, "u")
        self.assertEqual(c.password, "p")

    def test_no_auth(self):
        c = ps.parse_proxy_url("http://host.example:8080")
        self.assertEqual(c.server, "http://host.example:8080")
        self.assertIsNone(c.username)
        self.assertIsNone(c.password)

    def test_https(self):
        c = ps.parse_proxy_url("https://u:p@host:443")
        self.assertEqual(c.server, "https://host:443")

    def test_socks5(self):
        c = ps.parse_proxy_url("socks5://u:p@host:1080")
        self.assertEqual(c.server, "socks5://host:1080")

    def test_percent_decoded_password(self):
        c = ps.parse_proxy_url("http://u:p%40ss%21@h:1")
        self.assertEqual(c.password, "p@ss!")

    def test_reject_empty(self):
        with self.assertRaises(ps.ProxyConfigError):
            ps.parse_proxy_url("")

    def test_reject_unsupported_scheme(self):
        with self.assertRaises(ps.ProxyConfigError):
            ps.parse_proxy_url("ftp://h:1")

    def test_reject_no_host(self):
        with self.assertRaises(ps.ProxyConfigError):
            ps.parse_proxy_url("http://:1234")

    def test_reject_no_port(self):
        with self.assertRaises(ps.ProxyConfigError):
            ps.parse_proxy_url("http://hostonly")

    def test_reject_garbage(self):
        with self.assertRaises(ps.ProxyConfigError):
            ps.parse_proxy_url("not a url")


class TestAsCamoufoxDict(unittest.TestCase):
    def test_server_only_when_no_auth(self):
        c = ps.parse_proxy_url("http://h:1")
        self.assertEqual(c.as_camoufox_dict(), {"server": "http://h:1"})

    def test_full_dict_with_auth(self):
        c = ps.parse_proxy_url("http://u:p@h:1")
        d = c.as_camoufox_dict()
        self.assertEqual(d["server"], "http://h:1")
        self.assertEqual(d["username"], "u")
        self.assertEqual(d["password"], "p")


class TestAsHttpxURL(unittest.TestCase):
    def test_roundtrip_with_auth(self):
        c = ps.parse_proxy_url("http://user:pass@h.example:1234")
        self.assertEqual(c.as_httpx_url(), "http://user:pass@h.example:1234")

    def test_no_auth(self):
        c = ps.parse_proxy_url("http://h.example:1234")
        self.assertEqual(c.as_httpx_url(), "http://h.example:1234")


class TestResolveProxy(unittest.TestCase):
    def test_cli_flag_wins(self):
        with patch.dict(os.environ, {ps.ENV_PROXY_VAR: "http://env:env@h:1"}):
            r = ps.resolve_proxy("http://cli:cli@h:2")
            assert r is not None
            self.assertEqual(r.username, "cli")

    def test_env_var_used_when_no_flag(self):
        with patch.dict(os.environ, {ps.ENV_PROXY_VAR: "http://env:env@h:1"}):
            r = ps.resolve_proxy(None)
            assert r is not None
            self.assertEqual(r.username, "env")

    def test_none_when_neither_set(self):
        with patch.dict(os.environ, {}, clear=False):
            os.environ.pop(ps.ENV_PROXY_VAR, None)
            self.assertIsNone(ps.resolve_proxy(None))
            self.assertIsNone(ps.resolve_proxy(""))

    def test_invalid_flag_raises(self):
        with self.assertRaises(ps.ProxyConfigError):
            ps.resolve_proxy("not a url")


if __name__ == "__main__":
    unittest.main()
