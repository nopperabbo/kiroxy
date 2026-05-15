# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 1.4.x   | :white_check_mark: |
| 1.3.x   | :white_check_mark: |
| < 1.3   | :x:                |

## Reporting a Vulnerability

kiroxy handles authentication credentials (Kiro refresh tokens, Builder ID auth, inbound API keys) on behalf of the operator. Vulnerabilities that could leak these credentials, allow unauthorized access to the proxy, or enable account compromise are taken seriously.

**Please do NOT report security vulnerabilities through public GitHub issues.**

### How to report

Email **security@nopperabbo.dev** (or open a private GitHub Security Advisory at https://github.com/nopperabbo/kiroxy/security/advisories/new) with:

- A description of the vulnerability
- Steps to reproduce (proof-of-concept where applicable)
- Affected version(s)
- Potential impact assessment
- Any suggested mitigation

### What to expect

- Acknowledgment within **72 hours**
- Initial assessment within **7 days**
- Coordinated disclosure timeline negotiated case-by-case (default: 90 days from acknowledgment)
- Public credit in the CHANGELOG and release notes (unless you prefer anonymous)

### Scope

In scope:

- Authentication bypass (inbound API key validation, Kiro auth flow)
- Credential exposure (vault file leakage, log redaction failures, token in error responses)
- Authorization issues (cross-account leakage in multi-account pool)
- Server-side request forgery (SSRF) via upstream proxy chain
- Path traversal in dashboard / docs serving
- Cryptographic weaknesses in token storage (vault uses mode 0600, but report any structural flaws)

Out of scope:

- Issues requiring physical access to the operator's machine
- Social engineering against operators
- Vulnerabilities in third-party Kiro IDE / AWS Builder ID services (report those to AWS)
- DoS attacks against your own self-hosted instance (you control the deployment surface)

## Hardening Recommendations for Operators

Even without specific CVEs, kiroxy operators should:

1. **Bind to loopback only** by default (`-port 8787` listens on `127.0.0.1` unless `-bind 0.0.0.0` is set)
2. **Set a strong inbound API key** via `-api-key` flag or `KIROXY_API_KEY` env var
3. **Keep `~/.kiroxy/tokens.db` mode 0600** (kiroxy enforces this on startup)
4. **Audit the wire** with `tcpdump -i lo0 port 8787` if running on shared hardware
5. **Run behind a reverse proxy with TLS** if exposing publicly (Caddy / Traefik recommended)
6. **Rotate refresh tokens** monthly via Kiro IDE re-authentication
7. **Pin the binary version** rather than tracking `latest` for production deployments

## Disclosure History

No public CVEs as of v1.4.0.
