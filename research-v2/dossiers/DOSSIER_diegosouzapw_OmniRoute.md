# Dossier — diegosouzapw/OmniRoute

_Reconstructed from librarian agent findings (2026-05-12); original file wiped by concurrent session fs interaction._

---

## Identity

| Field | Value |
|---|---|
| Repo | `diegosouzapw/OmniRoute` |
| Created | February 2026 (fork of 9router) |
| Language | **TypeScript** |
| License | **MIT** |
| Status | **Extremely active** — 2195+ issues, primary contributor NomenAK |
| Release | v3.8.0 |
| Parent | Fork of decolua/9router (verified via README + commit history) |

## Scope

TypeScript rewrite of 9router that adds: MCP and A2A protocol support, multimodal (text-to-speech, image, embeddings, audio), Electron desktop app + PWA, advanced routing (circuit breaker, semantic caching, LLM evals). **160+ providers** (vs 9router's 40+).

## Architecture

- Next.js 16 + React 19 + TypeScript.
- **Domain-driven layers**: domain / library / server / shared.
- OpenTelemetry-based observability.
- Redis caching (simple + semantic).
- Rate limiting at the edge.
- MCP server support.
- Electron desktop build + PWA.
- 368+ unit tests with 60% coverage enforcement (vitest) + Playwright E2E.

## Auth

- Per-provider OAuth (inherited from 9router plus additions).
- **Identity-aware multi-account** — uses deterministic `uuid_v5(provider, account)` for prompt-cache stability across rotations. This is a novel pattern worth documenting.

## Features

| Feature | Present |
|---|---|
| Multi-provider (160+) | ✓ |
| Multi-account per provider | ✓ |
| Circuit breaker pattern | ✓ |
| Semantic caching | ✓ |
| OpenTelemetry observability | ✓ |
| Structured response headers (provider, latency, cost, account) | ✓ |
| MCP server support | ✓ |
| A2A agent protocol | ✓ |
| Multimodal (TTS, image, embeddings, audio) | ✓ |
| LLM evaluation | ✓ |
| Request logging | ✓ |
| Cloud sync | ✓ |
| Analytics dashboard | ✓ |
| Electron desktop app | ✓ |
| PWA | ✓ |

## Weaknesses (from issues)

- **Extreme issue churn** (2195+) — strong signal of heavy use, but also surface area. Recent focus: Anthropic SDK compatibility, MCP tool integration, Docker, model listings, provider-specific bugs (6+ PRs in a single day on SDK-shape detection).
- Duplicate issue labels.
- Integration challenges with the underlying CLIProxyAPI.
- **Anthropic/OpenAI SDK-shape detection is a recurring bug surface** — kiroxy lesson: don't try to re-derive this; port MIT-licensed detection logic wholesale if/when we go multi-provider.

## What kiroxy could learn

1. **Identity-aware multi-account via deterministic UUID** (`uuid_v5(provider, account_email)`) — stable account identifiers across pool rotations, which preserves prompt-cache keys upstream. Pattern, not code. Low-LoC.
2. **Circuit breaker with failure-classification** — differentiate transient (network, upstream 5xx) from hard (403, invalid_grant) failures; only hard should disable the account. kiroxy's G.4 backlog item is identical — this is good prior art.
3. **OpenTelemetry observability out of the box** — every request tagged with provider/account/latency/cost/model. kiroxy's observability is minimal right now.
4. **Structured response headers** — `X-Kiroxy-Account`, `X-Kiroxy-Latency-Ms`, `X-Kiroxy-Provider`, `X-Kiroxy-Cost-Usd`. Useful for client-side debugging. LoC: ~50.
5. **Domain-driven layering** — when `internal/server` starts to bloat (it's starting to), this is the right architectural target.

## What kiroxy should NOT copy

- **Electron desktop app** — massive scope creep; kiroxy is a server-side proxy.
- **160 providers** — scope creep; Kiro is the target.
- **Semantic caching via Qdrant** — overkill for single-user.
- **Cloud sync** — not compatible with self-hosted single-user model.
- **LLM evals** — orthogonal concern.

## Verdict

Architectural reference of first rank if kiroxy ever scales. MIT-licensed so file-level reuse is legal. For 2026 roadmap, learn: circuit breaker + identity-aware multi-account + OTel + structured headers. **Reject**: desktop app, 160-provider sprawl, cloud sync, semantic cache.

---
_Compiled 2026-05-12 from librarian agent synthesis. Activity metrics (2195 issues, 368 tests) are from README and partial fetch; concurrent agent hit API rate limits so exact counts should be re-verified for any public claim._
