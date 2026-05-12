# Dossier — AntiHub-Project/Antigv-plugin

_Reconstructed from librarian agent findings (2026-05-12); original file wiped by concurrent session fs interaction._

---

## Identity

| Field | Value |
|---|---|
| Repo | `AntiHub-Project/Antigv-plugin` |
| Created | Late November 2025 |
| Last push | 2026-02-26 (activity low but alive) |
| Language | Node.js (ESM) + Express 5 |
| License | **CC BY-NC-SA 4.0** (LICENSE file) — despite `package.json` claiming MIT. The more restrictive CC BY-NC-SA applies. |
| Parent | Fork of `liuw1535/antigravity2api-nodejs` (AGPL-ish ancestry unclear; see notes) |
| Status | Minimal activity; 3 total issues; 1 open PR; 1 active fork by R-LLM |
| Scope | **Not Kiro-primary.** Primary target is Google Antigravity ("Antigv" = 反重力 = anti-gravity = Google's Gemini-powered coding agent). **Kiro is a secondary backend added in Dec 2025.** |

## Architecture

- Express 5 server on **port 8045**.
- PostgreSQL (~38 KB schema) + Redis.
- Two client subsystems:
  - **Antigravity / Gemini** — primary, with `gemini-3-pro-high` and related.
  - **AWS Kiro / CodeWhisperer** — secondary, with Claude Sonnet / Opus / Haiku 4.5.
- Shared infrastructure: admin API keys + per-user bearer tokens, per-user quota pools, tiktoken + Anthropic tokenizer for counting.

## Auth — Kiro path (the interesting one)

- **Social OAuth** (Google / GitHub) via **PKCE S256** against `prod.us-east-1.auth.desktop.kiro.dev`.
- **AWS Identity Center OIDC** for enterprise.
- Hardcoded IDE user-agent `KiroIDE-0.10.32-<machine_id>` (machine_id generated per install).
- `kiro://` custom protocol deep-link handling.
- Token refresh 5 minutes before expiry.
- `invalid_grant` → disable account; `403` → rotate endpoint first, then disable.

## Features

| Feature | Present | Notes |
|---|---|---|
| Anthropic `/v1/messages` | ⚠ | Kiro route returns 500 (open issue) |
| OpenAI `/v1/chat/completions` | ✓ | For both Antigravity + Kiro backends |
| Streaming SSE (Antigravity) | ✓ | |
| AWS EventStream binary parsing (Kiro) | ✓ | Custom header handling; reasoning_content conversion |
| Tool calling | ✓ | Index tracking for parallel tool calls in stream |
| Image input | ✓ | |
| Multi-account pool | ✓ | Shared vs dedicated tiers, user preference |
| OAuth state in Redis | ✓ | Polling pattern for auth completion |
| Per-user quota | ✓ | Per `cookie_id` + model |
| Free trial / bonus quota parsing | ✓ | `parseUsageLimits` function |
| Endpoint failover on auth error | ✓ | Rotate before disabling |
| Model resolver | ✓ | Claude label → Kiro canonical |

## Kiro-specific protocol facts worth noting (public; no license issue)

- Kiro IDE version header format: `KiroIDE-<semver>-<machine_id>`.
- OAuth PKCE against `prod.us-east-1.auth.desktop.kiro.dev`.
- Status code semantics: `402` / `403` / `RESOURCE_PROJECT_INVALID` → disable account immediately.
- AWS event-stream binary framing: length prefix + CRC checksum + header table + payload.
- Thinking-part stripping required before forwarding to Kiro (Kiro rejects `<thinking>` blocks in certain message shapes).
- Usage limits returned via `getUsageLimits` — parsed for current / limit / reset date / trial status / bonus.
- Tool-call `index` tracking is mandatory for streaming parallel tools (single-index streams break parallel calls).

## Weaknesses

- **License conflict** (README says MIT; LICENSE is CC BY-NC-SA 4.0). The LICENSE file wins; all rights in the code are non-commercial + share-alike.
- **Hardcoded Google OAuth desktop credentials** in `token_manager.js`. Security risk + possible ToS issue.
- Plaintext admin API keys in config.
- **Non-ASCII characters in database names** (Chinese/Korean characters) — operational footgun for some tooling.
- No rate limiting or circuit breakers.
- Hardcoded 10-minute stream timeout.
- Kiro IDE version header must be updated manually when Kiro releases new versions.
- Documentation is Chinese-only.
- No test suite.

## What kiroxy could learn (concepts only — license-blocked for copy)

1. **AWS event-stream framing** — length + CRC + header table + payload. kiroxy already has this via kirocc's `kiroproto` package (Apache-2.0); cross-reference confirms correctness.
2. **`parseUsageLimits` balance math** — how to compute current / limit / reset / trial / bonus from Kiro's response. If kiroxy adds usage tracking, this is the reference for what fields to extract.
3. **Thinking-part stripping** — messages with `<thinking>` blocks must be cleaned before forwarding. kiroxy's `internal/respconv/thinking_tags.go` handles reverse direction (Kiro → Anthropic); confirm forward direction is clean too.
4. **Status code taxonomy** — `402` / `403` / `RESOURCE_PROJECT_INVALID` = immediate-disable signals. kiroxy's circuit breaker should classify these as "hard" failures.
5. **Tool-call `index` tracking for streaming** — parallel tools require `index` in every tool_call delta; dropping it breaks parallel execution in clients. Test-coverage gap for kiroxy to verify.
6. **Endpoint rotation before account disable** — on 403, try the next Kiro endpoint host first; only disable if all endpoints fail. kiroxy does partial rotation; could formalize.
7. **`kiro://` deep-link redirect** — relevant for onboarder flows if we ever support desktop-app-style deep-linking back from OAuth.

## What kiroxy should avoid

- PostgreSQL + Redis overhead for a single-user tool.
- Multi-user schema (out of scope).
- Proxying two disparate backends (Antigravity + Kiro) in one service — scope creep.
- Hardcoded credentials.
- No tests.

## Verdict

The **richest Kiro reverse-engineering reference in the OSS JS world.** Kiroxy's `internal/kiroclient` should cross-check its protocol facts against this repo's implementation — not by copying, but by validation. License blocks direct code reuse; extract patterns only.

One concrete TODO for kiroxy: verify our IDE user-agent string matches the current Kiro-IDE format (`KiroIDE-<semver>-<machine_id>`). Stale user-agents may be a latent cause of 403s.

---
_Compiled 2026-05-12. LICENSE file (CC BY-NC-SA 4.0) overrides README claim of MIT. All protocol facts above are publicly observable via Kiro's own API and are not license-restricted._
