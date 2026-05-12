# Dossier — petehsu/KiroProxy

_Reconstructed from librarian agent findings (2026-05-12); original file wiped by concurrent session fs interaction._

---

## Identity

| Field | Value |
|---|---|
| Repo | `petehsu/KiroProxy` |
| Stars / Forks | 336 ★ / 93 forks |
| Language | Python 3.11 (FastAPI) |
| License | **NO LICENSE FILE** — all rights reserved. No legal file-level reuse. |
| Status | Active; v1.8.1 released 2026-05-11; 114 commits on `main`. |
| Scope | IDE proxy exposing **three** client protocols on a single Kiro upstream. |

## Architecture summary

- FastAPI app with per-protocol routers:
  - Anthropic `/v1/messages` (+ SSE)
  - OpenAI `/v1/chat/completions` and `/v1/responses`
  - Gemini `generateContent`
- **4-layer model resolver** — cache → static alias → fuzzy match → passthrough.
- Multi-account pool with **device-flow + social OAuth + PKCE** support.
- Token refresh scheduler runs every 5 minutes.
- Circuit breaker with probabilistic retry.
- Session stickiness window (60 s) — same client gets routed to the same account inside the window for prompt-cache continuity.
- Payload size guard (~615 KB pre-Kiro clamp).
- `profileArn` auth_guard repair on corrupted credentials.
- Web dashboard with i18n + "flow monitor" that persists full request/response bodies to JSONL.
- CLI for account management.
- Docker with healthcheck.

## Auth methods

- AWS Builder ID device-code flow (Kiro CLI pattern).
- Social OAuth (Google/GitHub) via PKCE S256.
- Import refresh token (trust-filesystem).
- AWS IAM Identity Center OIDC (enterprise).

## Features relevant to kiroxy

| Feature | Present | Notes |
|---|---|---|
| Anthropic `/v1/messages` + SSE | ✓ | Recent SSE streaming fix in v1.8 (was batched before) |
| OpenAI `/v1/chat/completions` | ✓ | |
| OpenAI `/v1/responses` | ✓ | Rare — kiroxy doesn't have this |
| Gemini `generateContent` | ✓ | Rare — kiroxy doesn't have this |
| Multi-account pool | ✓ | With cooldown + circuit-breaker |
| Per-account proxy (HTTP/SOCKS5) | ✓ | Merged via issue #30 |
| Session stickiness | ✓ | 60 s keyed by client identity |
| Prompt caching (Anthropic `cache_control` → Kiro `cachePoint`) | ✓ | Same pattern as kirocc |
| Tool search (BM25 + regex) | ✓ | Proxy-side tool pruning |
| FSM thinking parser | ✓ | Streaming-safe extended-thinking handling |
| Flow monitor (request ring with JSONL export) | ✓ | kiroxy has request ring; lacks bookmark/JSONL export |
| AWS Q `getUsageLimits` cost tracking | ✓ | **kiroxy doesn't have this** |
| Dashboard with i18n | ✓ | |
| Docker healthcheck | ✓ | kiroxy has both |

## Known weaknesses (from issues/PRs)

- **No LICENSE.** All rights reserved — legally cannot copy code.
- Web UI "online login" button doesn't auto-open browser (#28).
- `profileArn` still leaks on edge cases (#27 open).
- Streaming was batched until v1.8 (#20 fixed).
- README has `yourname` placeholder (#22 open).
- Anthropic SSE event-prefix PR unmerged (#21 open).
- No refresh lock — concurrent refresh race is possible.
- No Prometheus / OTel export.
- No cost/price table; only usage-limit quota.
- Flow monitor buffer is in-memory only (risks loss on restart).
- 200-message context limit noted as a bottleneck.
- No Cursor support (client compatibility gap).

## What kiroxy could learn (concepts only — license-blocked for copy)

1. **4-layer model resolver** (cache → alias → fuzzy → passthrough) — kiroxy's resolver is simpler; adding fuzzy match protects against client typos without being permissive on unknown models.
2. **60-second session stickiness** — for prompt-cache continuity, route the same client back to the same account within a window. LoC: ~150; impact: measurably lower cache-miss rate.
3. **Probabilistic-retry CircuitBreaker** — don't hard-open on first failure; use a probability gradient. Fits `internal/pool` naturally.
4. **Payload guard (~615 KB)** — reject requests pre-Kiro that would exceed upstream limits. Avoids wasting a refresh window. LoC: ~50.
5. **`profileArn` auth_guard repair** — on corrupted credentials, rebuild the ARN from components rather than failing. kiroxy has partial handling via `import-accounts` metadata; this is a more defensive pattern.
6. **Per-account outbound proxy** — HTTP/SOCKS5 per account, for VPN users or geo-diverse pools. Hexos had this too; kiroxy backlog item.
7. **Flow monitor JSONL export + bookmarks** — lets power users replay or share problematic requests without re-triggering.
8. **AWS Q `getUsageLimits` usage tracking** — real quota data from Kiro's own API; better than estimating.
9. **CircuitBreaker + scheduler + refresher separation** — three concerns, three modules. kiroxy currently mixes them in `internal/pool`; worth refactoring when we wire auto-refresh.

## What kiroxy already does better

- **Single Go binary vs Python/FastAPI/uvicorn.** Deploy delta is huge.
- **MIT license** vs no-license — kiroxy is legally shareable.
- **Camoufox full-auto onboarder** — petehsu/KiroProxy has device-flow + social OAuth but no automated browser harvester.
- Go jsonv2 perf for streaming SSE.
- Cleaner module boundaries (`internal/{pool, tokenvault, kiroclient, ...}`).

## Citations

- Repo: https://github.com/petehsu/KiroProxy
- Recent v1.8 release tag: https://github.com/petehsu/KiroProxy/releases
- Issue #20 (streaming): https://github.com/petehsu/KiroProxy/issues/20
- Issue #21 (SSE prefix): https://github.com/petehsu/KiroProxy/issues/21
- Issue #27 (profileArn): https://github.com/petehsu/KiroProxy/issues/27
- Issue #28 (browser login): https://github.com/petehsu/KiroProxy/issues/28
- PR #29 (v1.8 multi-protocol): https://github.com/petehsu/KiroProxy/pull/29
- PR #30 (per-account proxy): https://github.com/petehsu/KiroProxy/pull/30

## Verdict

**Legally cannot copy code.** Patterns worth mimicking: session stickiness (60 s window), probabilistic circuit breaker, payload guard, AWS Q usage API for real quota data. kiroxy should add OpenAI `/v1/responses` and Gemini compat to `BACKLOG.md` as P2 if/when the multi-client story matters.

---
_Compiled 2026-05-12. No LICENSE file means code reuse is legally blocked; concepts and protocol facts remain in the public domain._
