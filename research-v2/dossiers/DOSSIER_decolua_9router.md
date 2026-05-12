# Dossier — decolua/9router

_Reconstructed from librarian agent findings (2026-05-12); original file wiped by concurrent session fs interaction._

---

## Identity

| Field | Value |
|---|---|
| Repo | `decolua/9router` |
| Created | Early January 2026 |
| Language | JavaScript / Next.js 16 + React 19 |
| License | **MIT** |
| Status | Very active; published to npm; most recent commit May 2026 |
| Reported metrics | 22k ★ / 1048+ issues (extremely high activity; note: agent hit GitHub rate limits mid-verification, so star count should be re-verified before citation) |
| Origin | Port of **CLIProxyAPI** to JS |

## Scope

Multi-provider AI proxy routing 40+ providers / 100+ models across OpenAI, Claude, Gemini, Cursor, and other platforms. Exposes OpenAI-compatible API endpoint.

## Architecture

- Next.js 16 app with API routes.
- Multi-account registration via OAuth for each provider.
- **Tiered fallback chains** — `subscription → cheap → free` tiers user-defined per model.
- Round-robin inside each tier.
- Real-time quota tracking per account.
- Token-compression mode for context budgets.

## Auth

- Per-provider OAuth (for each of the 40+ providers).
- No evidence of AWS Builder ID / Kiro-specific auth — 9router targets Claude/Gemini/Cursor, not Kiro.

## Features

| Feature | Present |
|---|---|
| OpenAI-compatible `/v1/chat/completions` | ✓ |
| Multi-provider (40+) | ✓ |
| Multi-account per provider | ✓ |
| User-defined fallback chains (tiered) | ✓ |
| Real-time quota tracking | ✓ |
| Token compression | ✓ |
| Dashboard | ✓ (Next.js UI) |
| OAuth automation | ✓ |
| Docker | ⚠ Known issues reported |

## Weaknesses (from issues)

- Known bugs around OAuth systems for specific providers.
- Docker configuration issues.
- Project is "private" despite being published to npm (licensing/access model unclear).

## What kiroxy could learn

1. **Tiered fallback chains** (`subscription → cheap → free`) — user-configurable routing ladder. For kiroxy even single-provider, this translates to `pro-accounts → free-accounts → personal-api-key-fallback`. Concept generalizes cleanly.
2. **Round-robin inside a tier** — simple, deterministic, ships. kiroxy's LRU pool is close; round-robin is cheaper.
3. **Real-time quota tracking per account** — not usage limits polled periodically, but tracked in-flight. Matches Quorinex and petehsu/KiroProxy patterns.
4. **Multi-account rotation as the highest-ROI single change** — 5× quota by stacking OAuth accounts for the same provider. Already kiroxy's design, but worth citing as industry-validated.

## What kiroxy does differently (by design)

- Kiro-specific — we don't need 40+ provider support.
- Go single binary vs Next.js full-stack.
- Single-user focus — no multi-tenant DB.

## Verdict

MIT-licensed, so file-level reuse is legal if we ever expand to multi-provider. For now, concepts only. **Don't copy**: Electron desktop, 40+ provider surface (scope creep), complex quota dashboards.

---
_Compiled 2026-05-12. Note: concurrent librarian agent hit GitHub API rate limits; star counts and issue counts are from partial fetch and should be re-verified for any public claim._
