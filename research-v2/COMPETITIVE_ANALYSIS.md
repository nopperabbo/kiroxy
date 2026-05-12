# kiroxy Competitive Analysis — May 2026

> **Status:** v0.3.0 shipped 2026-05-12. Phase 2.5 (pool-mode refresh wiring) in-flight in a concurrent session; this doc assumes v0.4.0 will close that gap as a mechanical P0.
> **Assumed reader:** has read `BUILD_LOG.md`, `OVERNIGHT_LOG.md`, and the `research/` v1 dossiers. This doc compares **shipping kiroxy** against the wider ecosystem and proposes v1.0.0 / v1.1.0 / backlog priorities.
> **Method:** live GitHub + docs surveys on 2026-05-12 Asia/Makassar; five new Tier 1 dossiers and one Tier 2 dossier in `research-v2/`. Every factual claim is cited inline or in a dossier.
> **Reading order:** Executive Summary → Feature Matrix → Gap Analysis → Recommendations. Deep dives are in `research-v2/dossiers/` and `research-v2/tier2/`.

---

## Executive Summary

- **Where kiroxy wins:** (1) single-binary Go deploy beats every Python/Node peer on ops friction; (2) MIT license + clean attribution beats jwadow/KiroaaS/aliom-v (AGPL), caidaoli (no license), and petehsu/KiroProxy (no license); (3) full-auto Camoufox onboarder (G.1) is unique among Go proxies and rare in OSS generally; (4) jsonv2 performance is a latent edge on SSE hot paths.
- **Where kiroxy loses:** (1) no **auto token refresh** for imported social-flow accounts — the defining gap that v0.4.0 must close; (2) no OpenAI-compat surface — we ship only `/v1/messages`, while every other serious peer ships both; (3) no accurate `input_tokens` (Quorinex shipped `contextUsageEvent` parsing 2026-05-11, making estimator the losing position); (4) no cost tracking, no metrics export, no session grouping.
- **Top 3 features for v1.0.0:** **(1)** OpenAI-compatible `/v1/chat/completions` + `/v1/models`; **(2)** `contextUsageEvent`-based accurate `input_tokens`; **(3)** `*.kiro.dev` endpoint migration audit before 2026-05-15 (AWS deprecation deadline, jwadow issue #146).
- **Anti-goal to publicly deprioritize:** **multi-provider expansion**. 9router, OmniRoute, LiteLLM, Portkey all saturate this niche; kiroxy's edge is being the *best Kiro proxy*, not the Nth gateway. README should add an explicit "non-goals" section.
- **Positioning tweak:** the README should lead with "self-hosted Kiro proxy for one user" — not "Anthropic-compatible endpoint". The target audience is the solo Kiro-subscription holder who wants to use `claude-code` / Cursor / custom clients against their own account, *not* a platform team building a multi-tenant gateway. Rewriting the first three sentences shrinks the expected-feature surface and reduces support-question scope.

---

## Projects Studied

| Project | Tier | Stars | Language | License | Last commit | Active? | Scope for kiroxy |
|---|---|---:|---|---|---|---|---|
| jwadow/kiro-gateway | 1 (v1) | 1311 | Python (FastAPI) | AGPL-3.0 | 2026-05-05 (features paused since) | maintenance | Ecosystem reference, pain-point source |
| Quorinex/Kiro-Go | 1 (v1) | 479 | Go | MIT | 2026-05-11 | **active** | Our architectural donor; new feature deltas |
| justlovemaki/AIClient2API | 1 (v1) | 7723 | JS (ESM) | GPL-3.0 | 2026-05-09 | **very active** | Multi-provider gateway; v3.0.0 shipped |
| kadangkesel/hexos | 1 (v1) | 1 | TS/Bun | MIT | 2026-05-10 | **archived** | Salvage-only; vault + pool patterns |
| hnewcity/KiroaaS | 1 (v1) | 96 | Rust+TS+Py | AGPL-3.0 | 2026-05-12 | active | Tauri desktop wrap of jwadow |
| d-kuro/kirocc | 1 (v1) | 15 | Go | Apache-2.0 | 2026-05-11 (no feat work since 04-26) | paused | Our converter donor |
| caidaoli/kiro2api | 1 (v1) | 596 | Go | **NONE** | 2025-10-19 | stale | Unreusable (no license) |
| aliom-v/KiroGate | 1 (v1) | 403 | Python | AGPL-3.0 | 2026-02-15 | stale | jwadow fork, ignore |
| petehsu/KiroProxy | 1 (v2) | 336 | Python (FastAPI) | **NONE** | 2026-05-11 (v1.8.1) | **active** | Pattern source (cannot copy code) |
| hj01857655/kiro-account-manager | 1 (v2) | ? | Rust+Tauri | (see dossier) | 2026-05-10 (v1.8.6) | **active** | Desktop UX patterns for kiroxy dashboard v2 |
| AntiHub-Project/Antigv-plugin | 1 (v2) | 3 issues | Node (Express) | CC-BY-NC-SA 4.0 | 2026-02-26 | low | Kiro protocol reference |
| decolua/9router | 1 (v2) | 22k?/1048+ | JS (Next.js) | MIT | 2026-05 | very active | Multi-provider routing patterns |
| diegosouzapw/OmniRoute | 1 (v2) | ?/2195+ | TS (Next.js) | MIT | 2026-05 | very active | Circuit breaker, identity-aware multi-account |
| LiteLLM (BerriAI/litellm) | 2 | high | Python | MIT (core) | 2026-05-12 | **extremely active** | Virtual keys + spend tracking patterns |
| Portkey-AI/gateway | 2 | high | TS | MIT | 2026-03-25 | main-frozen; 2.0 branch active | Config-as-data routing, MCP gateway |
| Helicone/helicone | 2 | high | TS | Apache-2.0 | 2026-05-02 | active | Observability + session grouping + cost API |
| simonw/llm | 2 | high | Python | Apache-2.0 | 2026-05-12 | active | Plugin architecture for future multi-provider |
| OpenRouter (docs only) | 2 | n/a | closed-source | proprietary | live | production | Request-body routing API shape |
| TypingMind proxy | 2 | n/a | closed-source | proprietary | live | production | Out of scope (chat UI) |

Full dossiers:
- `research/` — v1 set (jwadow, Quorinex, AIClient2API, hexos, KiroaaS, kirocc, caidaoli, aliom-v)
- `research-v2/dossiers/` — v2 set (petehsu, hj01857655, AntiHub, 9router, OmniRoute, v1 delta recheck)
- `research-v2/tier2/TIER2_DOSSIER.md` — gateway Tier 2 (LiteLLM, Portkey, Helicone, simonw/llm, OpenRouter, TypingMind)

---

## Feature Matrix

`✓` has · `~` partial · `✗` absent · `?` unverified · `n/a` out of scope for that project's niche. **kiroxy** column reflects v0.3.0 at `0564c26`; **Gap?** marks whether the absence matters for v1.0.0.

| Feature | jwadow | Quorinex | AIClient2API | hexos | KiroaaS | petehsu | kiro-acct-mgr | Antigv-plugin | 9router | OmniRoute | LiteLLM | Portkey | Helicone | OpenRouter | **kiroxy v0.3.0** | **Gap?** |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| Auth: AWS Builder ID device-code | ✓ | ✓ | ✓ | n/a | ✓ | ✓ | ✓ | n/a | n/a | n/a | n/a | n/a | n/a | n/a | **✓ (v0.2.0)** | - |
| Auth: Social OAuth (Google/GitHub PKCE) | ✓ | ✓ | ✓ | n/a | ✓ | ✓ | ✓ | ✓ | n/a | n/a | n/a | n/a | n/a | n/a | **✗** (via Camoufox onboarder) | P2 |
| Auth: Import pre-acquired refresh token | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | n/a | n/a | n/a | n/a | n/a | n/a | **✓ (v0.2.2)** | - |
| Auto token refresh (background) | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | n/a | n/a | ✓ (enterprise) | ✓ (enterprise) | n/a | n/a | **~ (vault-mode only; pool-mode P0)** | **P0** |
| Token refresh with grace period | ? | ? | ? | ✓ | ? | ? | ? | ~ | n/a | n/a | **✓** | ? | n/a | n/a | ✗ | P2 |
| Multi-account pool w/ circuit breaker | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ~ | ✓ | ✓ (id-aware) | ✓ | ✓ | ✓ | ✓ | **✓** | - |
| Model resolver / alias mapping | ✓ | ✓ | ✓ | ✓ | = jwadow | ✓ (4-layer) | ✗ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | **✓** | - |
| Anthropic `/v1/messages` | ✓ | ✓ | ✓ | ~ | = jwadow | ✓ | n/a | ⚠ (500s) | n/a | n/a | ✓ | ✓ | ✓ | ✓ | **✓ (flagship)** | - |
| OpenAI `/v1/chat/completions` | ✓ | ✓ | ✓ | ✓ | = jwadow | ✓ | n/a | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | **✗** | **P0** |
| OpenAI `/v1/models` | ✓ | ✓ | ✓ | ? | = jwadow | ✓ | n/a | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | **✗** | **P0** |
| OpenAI `/v1/responses` | ? | ? | ? | ? | ? | ✓ | n/a | ✗ | ? | ? | ✓ | ~ | ✗ | ~ | ✗ | P3 |
| Gemini `generateContent` | ✗ | ✗ | ✓ | ✗ | ✗ | ✓ | ✗ | ~ (via Antigv) | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✗ | P3 |
| Streaming SSE (Anthropic-compat) | ✓ | ✓ | ✓ | ✓ | = jwadow | ✓ | n/a | ⚠ | n/a | n/a | ✓ | ✓ | ✓ | ✓ | **✓** | - |
| Accurate `input_tokens` via `contextUsageEvent` | ✗ | **✓ (PR #37, 2026-05-11)** | ? | ? | = jwadow | ~ | n/a | ✗ | n/a | n/a | n/a | n/a | n/a | n/a | **✗** | **P0** |
| Prompt caching (`cache_control` → `cachePoint`) | ✗ | ✓ | ✗ | ✗ | = jwadow | ✓ | n/a | ~ | n/a | n/a | n/a | n/a | n/a | n/a | **✗** | **P1** |
| Tool search (BM25 + regex) | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ | n/a | ✗ | n/a | n/a | n/a | n/a | n/a | n/a | **✓ (via kirocc graft)** | - |
| Session stickiness (prompt-cache continuity) | ✗ | ? | ? | ? | ? | ✓ (60 s) | n/a | ? | ? | ✓ | ? | ? | ✓ (sessions) | ? | ✗ | **P1** |
| Cost / usage tracking (persisted) | ✓ | ✓ | ✓ | ✓ (by-model, by-acct) | = jwadow | ✓ (AWS Q) | ~ | ✓ | ✓ | ✓ | ✓✓ | ✓ | ✓✓ | ✓ | **✗** | **P1** |
| Pricing source (public pricing API) | ✗ | ✗ | ✗ | ✗ | = jwadow | ✗ | ✗ | ✗ | ✓ | ✓ | ✓ | ✓ | ✓✓ | ✓ | ✗ | P2 |
| Rate-limit awareness per account | ~ | ✓ | ✓ | ✓ | = jwadow | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | **~ (cooldown only)** | P2 |
| Fallback orchestration (primary→secondary) | ✓ | ✓ | ✓ | ✓ | = jwadow | ✓ | ✗ | ✓ | ✓ (tiered) | ✓ | ✓ | ✓ | ✓ | ✓ | **~ (next-account only)** | P2 |
| Outbound proxy per account (HTTP/SOCKS5) | ✗ | **✓ (2026-05-11)** | ✗ | ✓ | ✗ | ✓ | ✗ | ✗ | ✗ | ✗ | n/a | n/a | n/a | n/a | **✗** | P2 |
| Request caching (prompt/response) | ✗ | ✗ | ✗ | ✗ | = jwadow | ✗ | ✗ | ✗ | ✗ | ✓ (semantic) | ✓ | ✓ | ✗ | ✗ | **✗** | P3 |
| Request logging / audit trail | ✓ | ✓ | ✓ | ✓ | = jwadow | ✓ (JSONL flow monitor) | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓✓ | ? | **~ (request ring, in-mem)** | P2 |
| Dashboard (web UI) | ✗ | ✓ (Vue) | ✓ | ✓ | ✓ (Tauri React) | ✓ (i18n) | ✓✓ (Tauri primary) | ✗ | ✓ (Next.js) | ✓ (Next.js+Electron) | ✓ (Next.js) | ✓ | ✓ | n/a | **✓ (plain HTML + request ring)** | P1 (v2 upgrade) |
| CLI tooling depth | ~ | ✓ | ✓ | ✓ | ~ | ✓ | ~ | ✓ | ✓ | ✓ | ✓ | ✓ | ~ | ~ | **✓** | - |
| Docker / container support | ✓ | ✓ | ✓ | ✗ | = jwadow | ✓ | ✗ | ? | ~ | ~ | ✓ | ✓ | ✓ | n/a | **✓ (distroless)** | - |
| Onboarding automation (token harvest) | ✗ | ✗ | ✗ | ✗ | ~ | ✗ | ~ | ✗ | ~ (OAuth auto) | ~ (OAuth auto) | n/a | n/a | n/a | n/a | **✓ (Camoufox G.1)** | - |
| Multi-provider (Claude+OpenAI+Gemini) | ✗ | ✗ | ✓ | ✓ | ✗ | ✓ | ✗ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | **✗** | anti-goal |
| Custom routing rules (declarative) | ✗ | ~ | ~ | ~ | ✗ | ~ | ✗ | ✗ | ✓ | ✓ | ✓ | ✓✓ | ~ | ✓✓ | **✗** | P2 |
| Request transformation middleware | ✗ | ~ | ✓ | ✓ | ✗ | ✓ | ✗ | ~ | ✓ | ✓ | ✓ | ✓ | ✓ | ? | **~ (reqconv/respconv)** | - |
| Circuit breaker / health tracking | ~ | ✓ | ✓ | ✓ | = jwadow | ✓ (probabilistic) | ✓ | ~ | ✓ | ✓ | ✓ | ✓ | ~ | ✓ | **✓ (3-strikes)** | - |
| Prometheus metrics export | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ | ✓ | ✓ | ✓ | n/a | **✗** | P2 |
| OpenTelemetry tracing | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ | ~ | ~ | ✓ | n/a | **✓ (via kirocc graft)** | - |
| MCP gateway (tools exposed in chat completions) | ✗ | ✗ | ✗ | ✗ | ~ (tools) | ✗ | ✗ | ✗ | ✓ | ✓ | ✓ | ✓ | ✗ | ✗ | **✗** | P2 (novel) |
| Virtual keys / per-client spend | ✗ | ✗ | ✓ (plugin) | ~ | ✗ | ✗ | ✗ | ✓ (admin+user) | ✗ | ✓ | ✓✓ | ✓ (ent) | ✓ | n/a | **✗ (single key)** | P1 |
| Session grouping (multi-turn aggregation) | ✗ | ✗ | ✗ | ✗ | ✗ | ~ (stickiness) | ✗ | ✗ | ✗ | ✓ | ~ | ~ | ✓✓ | ~ | **✗** | P1 |

**Observations:**
1. kiroxy has the **cleanest Anthropic-compat baseline** (tool_search via kirocc, jsonv2 streaming, distroless Docker, OTel scaffolding) but is **missing the OpenAI surface every peer ships**.
2. **Auto refresh for import-accounts is the single defining gap.** Every other project has it; we have it for vault-mode (`KIROXY_KIRO_DB_PATH`) only. v0.4.0 makes or breaks the claim of parity.
3. **Multi-provider** is where we choose not to compete. Explicitly labeled anti-goal.

---

## Project Deep Dives

Detailed deep dives live in per-project dossier files. This document summarizes; see dossiers for citations and commit-level evidence.

### Tier 1 — Kiro proxies (direct competitors)

- **jwadow/kiro-gateway** — `research/dossiers/DOSSIER_jwadow_kiro-gateway.md` + `research-v2/dossiers/DOSSIER_v1_delta_recheck.md`. Summary: dominant Python proxy, AGPL-3.0 blocks most reuse, quiet 2-week activity, issue #146 tracks *.kiro.dev migration deadline.
- **Quorinex/Kiro-Go** — `research/dossiers/DOSSIER_Quorinex_Kiro-Go.md` + delta recheck. Summary: our architectural donor, shipped outbound proxy + `thinking` config + `contextUsageEvent` parsing in the last 2 weeks. The "three new feature deltas" that directly raise kiroxy's P0 bar.
- **AIClient2API** — dossier + delta recheck. Summary: GPL-3.0, v3.0.0 shipped with AI self-discovery architecture; 13 releases in 2 weeks; Kiro compat PR #585 (tool-name aliasing + throttling).
- **hexos** — dossier + delta recheck. Summary: archived 2026-05-10. Final sprint added Devin/Windsurf/Fireworks; salvage plan from v1 stands.
- **KiroaaS** — delta recheck. Summary: vendored jwadow's full python-backend on `85ec9ef`. Python ecosystem moves as one bloc; Go-side isolation is strategically correct.
- **kirocc** — delta recheck. Summary: nearly stale (1 dep-bump in 2 weeks); our grafts are stable.
- **petehsu/KiroProxy** — `research-v2/dossiers/DOSSIER_petehsu_KiroProxy.md`. Summary: no-license Python FastAPI proxy with 3-protocol surface (Anthropic + OpenAI + Gemini) and the richest pattern set. Cannot copy code; learn session stickiness, probabilistic circuit breaker, AWS Q usage API, flow monitor JSONL export.
- **hj01857655/kiro-account-manager** — `research-v2/dossiers/DOSSIER_hj01857655_kiro-account-manager.md`. Summary: Rust+Tauri **desktop** app. UX patterns for kiroxy dashboard v2. Not a server; different threat model.
- **AntiHub-Project/Antigv-plugin** — `research-v2/dossiers/DOSSIER_AntiHub-Project_Antigv-plugin.md`. Summary: Antigravity-primary, Kiro-secondary Node proxy. CC-BY-NC-SA license blocks code reuse, but it's the richest Kiro protocol reference in OSS JS: PKCE URL, IDE user-agent format, event-stream framing, usage-limits parser.
- **9router / OmniRoute** — `research-v2/dossiers/DOSSIER_decolua_9router.md` + `DOSSIER_diegosouzapw_OmniRoute.md`. Summary: MIT TypeScript multi-provider proxies. Patterns for tiered fallback, identity-aware multi-account, structured response headers. kiroxy should **not** try to become them; extract concepts only.

### Tier 2 — Gateways

All covered in `research-v2/tier2/TIER2_DOSSIER.md`:
- **LiteLLM** — virtual keys + spend tracking + key rotation with grace period
- **Portkey** — config-as-data routing + MCP gateway + edge runtime
- **Helicone** — session grouping + public LLM cost API + cautionary tale (raw body logging cost $22k)
- **simonw/llm** — plugin architecture reference
- **OpenRouter** — request-body routing shape (`provider.order`, `allow_fallbacks`, `models: string[]`)
- **TypingMind** — out of scope

### Tier 3 — Auth harvesters (brief)

Beyond what kiroxy already uses (Camoufox for `tools/onboard/`), the space is thin and mostly private. **kikirro** (referenced in our own BUILD_LOG) is the one relevant reference implementation, and kiroxy already mines it (profile rotation adapted to onboarder). No new donor material since v1 survey. The only **security concern** surfaced by Tier 1 (AntiHub-Project/Antigv-plugin) is the hardcoded Google desktop OAuth credentials — kiroxy's Camoufox flow uses user-supplied credentials, which is the correct posture; leave it.

---

## Gaps in kiroxy

Prioritized against v1.0.0 "production-ready" positioning. Each gap lists what, who has it, why, rough sketch, LoC estimate. LoC figures are grounded in kiroxy's package sizes (e.g., `internal/messages/` is ~900 LoC; `internal/reqconv/` 4600 LoC; `internal/server/` ~1800 LoC).

### P0 — Blocks v1.0.0 claim

#### 1. Auto refresh for `import-accounts` + `import-accounts-json` pool accounts

- **Who has it:** every Kiro proxy in the survey.
- **Why:** imported accounts expire in ~1h (Desktop-flow `expires_in`) without refresh wiring. Without this, kiroxy v0.3.0 is a demo, not a proxy.
- **Sketch:** extend `pool.TokenGetter` + `main.go` to call `auth.refreshSocialToken` for Desktop-flow accounts (`authMethod="social"` in metadata) when rotation is needed; persist rotated refresh+access tokens back to the vault. Tracked in a concurrent session's `.sisyphus/plans/phase-2.5-refresh-plan.md`.
- **LoC estimate:** 300-500 (pool-side refresh dispatch + vault upsert + tests).

#### 2. OpenAI-compatible `/v1/chat/completions` + `/v1/models`

- **Who has it:** jwadow, Quorinex, AIClient2API, hexos, KiroaaS, petehsu, Antigv-plugin, every Tier 2 gateway.
- **Why:** this is the lingua franca. Cursor, many CLI tools, and internal tooling all assume OpenAI compat. Claiming "v1.0.0 production-ready" without it is a miss.
- **Sketch:** new `internal/server` routes + new `internal/openai` package for request/response translation. Reuse `internal/kiroclient` unchanged; translators map OpenAI shapes to Anthropic shapes (kirocc's internal model is Anthropic) and back. Leverage `internal/models/models.go` resolver. Stream via SSE with OpenAI delta format.
- **LoC estimate:** 1500-2200 (openai request parser + response translator + streaming converter + tests).

#### 3. Accurate `input_tokens` via `contextUsageEvent` parsing

- **Who has it:** Quorinex (shipped PR #37 on 2026-05-11).
- **Why:** tiktoken estimates are now the losing position; clients trust upstream-reported counts. Cost/usage features downstream depend on this.
- **Sketch:** extend `internal/kiroproto/eventstream.go` (or add a new event-type handler) to recognize `contextUsageEvent` frames; thread count into `internal/respconv/usage.go` and emit the accurate value in Anthropic usage object.
- **LoC estimate:** 150-250.

#### 4. `*.kiro.dev` endpoint migration audit — URGENT

- **Deadline:** 2026-05-15 (AWS deprecation; jwadow issue #146).
- **Sketch:** grep `internal/kiroclient` and `internal/auth` for `q.<region>.amazonaws.com`; replace with `*.kiro.dev` endpoint set if found. If already migrated, verify + document.
- **LoC estimate:** 0-50 (likely already correct; audit is the work).

### P1 — v1.1.0 target

#### 5. Prompt caching (Anthropic `cache_control` → Kiro `cachePoint`)

- **Who has it:** Quorinex, petehsu. kirocc has the converter in `cache_points.go` (Apache-2.0, already donor-matched).
- **Why:** prompts with large fixed prefixes (systems, codebases) cost 10× less with caching. Default for serious `claude-code` users.
- **Sketch:** graft kirocc's `internal/reqconv/cache_points.go` (Apache-2.0 compliant with existing NOTICE); ensure session stickiness is implemented first (P1 item 6) because cachePoint without stickiness is wasted.
- **LoC estimate:** 150 (mostly graft; integration + tests).

#### 6. Session stickiness (60 s window, client-identity keyed)

- **Who has it:** petehsu (60 s); OmniRoute (identity-aware multi-account).
- **Why:** reaps the upstream prompt-cache savings. Without stickiness, consecutive turns hit different accounts and cache keys invalidate.
- **Sketch:** client identity = hash(`X-Api-Key` + `X-Forwarded-For` + `User-Agent`); a 60 s LRU in `internal/pool` maps identity → last-used account; override selection if account still healthy.
- **LoC estimate:** 150-200.

#### 7. Virtual keys + per-client spend tracking (persisted)

- **Who has it:** LiteLLM (schema reference), AIClient2API, petehsu (usage only, no spend).
- **Why:** unlocks dashboard view of "which client used how much"; foundation for cost guardrails; naturally integrates with the emerging Dashboard v2.
- **Sketch:** extend tokenvault schema with `virtual_keys(token_hash TEXT PK, label TEXT, created_at, last_used_at, spend_usd REAL, budget_usd REAL, rate_limit_rpm INT, allowed_models TEXT)`. Use LiteLLM's naming (`VerificationToken`) conceptually, not code. Inbound auth middleware records per-key metadata on each request.
- **LoC estimate:** 600-900.

#### 8. Dashboard v2 (request ring + accounts + keys + usage)

- **Who has it:** many. hj01857655/kiro-account-manager is the UX reference.
- **Why:** current plain-HTML dashboard works but looks like a 2015 monitoring panel. A concurrent session has scaffolded Svelte-based UI at `internal/server/next/client/` (per most recent commit). Work-in-flight; defer detail.
- **LoC estimate:** see concurrent session (Svelte + Vite scaffold already in tree).

### P2 — v1.2.0+

- Outbound proxy per account (HTTP/SOCKS5). Matches hexos + Quorinex + petehsu. Enables VPN-needing users.
- Declarative per-request config (Portkey-style). `{"retry": {...}, "fallback": {...}, "preferred_accounts": [...]}`.
- Prometheus metrics exporter. `/metrics` endpoint.
- Rate-limit awareness per account (not just cooldown). Parse Kiro's own rate-limit headers; expose as dashboard signal.
- Probabilistic circuit breaker (petehsu-style). Replace current 3-strikes with probability-gradient.
- `max_price` guardrails (OpenRouter-style). Pre-estimate cost; reject if over user-set ceiling.
- Persistent request log / audit trail with JSONL export (petehsu flow monitor pattern).
- Credential encryption for Camoufox onboarder (G.2, already on BACKLOG).
- Social OAuth direct in `kiroxy add-account` (no Camoufox). Match petehsu's device-flow + social path in Go.

### P3 — Backlog

- Gemini `generateContent` compat.
- OpenAI `/v1/responses` compat.
- Multi-provider expansion (Gemini via your own key, OpenAI via your own key) — **anti-goal signal**: only do if user demand is strong and explicit.
- Cursor IDE compatibility audit.
- Cloudflare Tunnel / ngrok integration docs.
- Homebrew tap.
- Plugin-style provider registry (simonw/llm-inspired).
- MCP bridge at `/mcp/` (LiteLLM-inspired).

---

## Unique kiroxy Strengths

Validated from feature matrix deltas where kiroxy alone has `✓`:

1. **Camoufox full-auto onboarder (G.1).** No other proxy in the survey has an automated browser-driven token harvester. petehsu has device-flow + social OAuth but requires human confirmation. hj01857655/kiro-account-manager has a token manager UI but not automation. Our Python sidecar + humanized typing + profile rotation is genuinely novel.
2. **Single binary Go deploy + distroless Docker.** Only kirocc matches this; kirocc is single-user. Every Python proxy (jwadow, aliom-v, petehsu, KiroaaS) requires Python + FastAPI + uvicorn + deps. Every Node proxy (9router, OmniRoute, Antigv-plugin, hexos) requires Node + package manager. Our 30 MB distroless image beats all.
3. **MIT + clean attribution.** Kiroxy's LICENSE + NOTICE are the cleanest in the Kiro ecosystem. jwadow/KiroaaS/aliom-v are AGPL (commercial-hostile); caidaoli/petehsu have no license (unreusable); AIClient2API is GPL (strong copyleft). kiroxy is the only serious option for commercial derivatives.
4. **jsonv2 streaming performance.** Only Go-1.26-aware project in the survey. Quorinex uses standard `encoding/json`; kirocc pins jsonv2 but we've continued that. On SSE hot paths, measurable even if not yet benchmarked publicly.
5. **Tool search (BM25 + regex) proxy-side.** Via kirocc graft. Only kirocc and petehsu have it; petehsu cannot be copied.
6. **Generation-locked OAuth refresh.** Hexos pattern, ported to Go, 50-goroutine race-safe. More defensive than any other Go proxy's refresh locks.
7. **Clean module boundaries.** `internal/{pool, tokenvault, kiroclient, reqconv, respconv, ...}` with single-responsibility packages. Quorinex has a 89 KB `handler.go` god-file (unchanged in 2-week delta).

---

## White-space Opportunities

Features that **no project in the survey** has, or has done well. Ordered by novelty × feasibility.

### 1. "One-command onboarding + serve" quickstart

**Gap:** every proxy requires 3-5 manual steps (install, configure vault, add account, serve). The closest single-command experience is `litellm --model gpt-4o` (but that needs API keys pre-set).

**Proposal:** `kiroxy quickstart` that: (a) launches Camoufox G.1 onboarder if no account exists, (b) writes tokens to vault, (c) picks a port, (d) prints the opencode / claude-code env-var line, (e) starts serving.

**LoC estimate:** 150-200 (orchestrator subcommand that composes existing pieces).

**Why it wins:** kiroxy already has the pieces. Nobody else can ship this without also building the onboarder.

### 2. Dashboard that tells the user "you have 47% of your Pro quota left"

**Gap:** nobody surfaces Kiro's own `getUsageLimits` data prominently. petehsu has it internally but doesn't make it the headline dashboard stat.

**Proposal:** every account card on dashboard shows live quota % + resets-in-N-hours; the aggregate across accounts is a big number at top. This is the metric the user actually cares about.

**LoC estimate:** 300-500 (AWS Q `getUsageLimits` call in `internal/auth`, dashboard state field, UI card).

**Why it wins:** this is the question every Kiro-subscription user asks daily. Answering it visually = user delight.

### 3. "Silent-fallback" monitor + alert

**Gap:** the model resolver currently silently rewrites unknown `kiro/*` labels to `claude-sonnet-4-6` (per `BACKLOG.md`). Nobody surfaces these silent fallbacks. In competitors, this leads to cost surprises.

**Proposal:** per-request telemetry flags when a requested model → resolved model transformation occurred; dashboard shows a "silent-fallback rate" metric; optional log-level warning.

**LoC estimate:** 50-100.

**Why it wins:** tiny change, big transparency win. Only possible because our resolver is already disciplined.

### 4. Portable `kiroxy backup` / `kiroxy restore`

**Gap:** nobody has clean backup tooling. hj01857655/kiro-account-manager has desktop-level "export" but it's Rust+Tauri-specific. Users restoring on a new machine go through onboarding again.

**Proposal:** `kiroxy backup --out=tokens.age` (age-encrypted passphrase-derived vault dump) and `kiroxy restore --in=tokens.age`. Integrates with the P2 credential-encryption G.2 item.

**LoC estimate:** 200.

**Why it wins:** single-user tool should have single-user ops ergonomics. Composes with G.2.

### 5. `kiroxy doctor` troubleshooting subcommand

**Gap:** every proxy has issues threads full of "Why isn't it working?" — jwadow's #146, petehsu's #27, #28, kirocc's #60. A first-line-self-diagnosis is universally absent.

**Proposal:** `kiroxy doctor` runs: (a) vault mode check, (b) network reachability to Kiro endpoints (both old and `*.kiro.dev`), (c) per-account token validity probe, (d) resolver self-check, (e) common env-var misconfig detection, (f) prints actionable next steps.

**LoC estimate:** 250-400.

**Why it wins:** cuts support-question noise. Also serves as a correctness harness for our own changes.

---

## Recommendations

### Recommendation 1 — Next feature to build: **OpenAI-compatible `/v1/chat/completions` + `/v1/models`**

- P0 in the matrix. Unblocks Cursor, claude-code (`OPENAI_API_KEY` paths), and the tooling mass-market.
- **LoC: 1500-2200.** New `internal/openai` package; wire into `internal/server`.
- Scope: chat completions + models listing + streaming. Keep `/v1/completions` (legacy) out.
- **Why now:** every peer has it; we ship without it = "not production-ready" signal.

### Recommendation 2 — Feature to publicly deprioritize: **multi-provider expansion**

- Add to README a **Non-Goals** section:
  > "kiroxy will not become a multi-provider gateway. For Claude-plus-GPT-plus-Gemini routing, use LiteLLM, Portkey, OpenRouter, or 9router/OmniRoute. kiroxy does one thing — Kiro as an Anthropic endpoint — and does it well."
- This saves us from 2195-issue surface area (OmniRoute), from 13-release-per-two-weeks pressure (AIClient2API), and from scope creep into Portkey/LiteLLM territory.
- **Why:** positioning clarity is a feature. The anti-goal is as valuable as a roadmap item.

### Recommendation 3 — Strategic positioning tweak: **rewrite README lede + add "Who is this for?"**

Current lede:
> "A single-user, self-hosted proxy that exposes your Kiro IDE subscription (Amazon Q Developer / AWS CodeWhisperer) as an **Anthropic Messages API** endpoint."

Proposed:
> "**kiroxy is for one person: you.** If you have a Kiro subscription and want `claude-code`, Cursor, or any Anthropic-compatible client to use it, kiroxy is the MIT-licensed Go binary that stands up that endpoint in 5 minutes — with auto-refreshing tokens, multi-account pool, and a self-contained Docker image. It is **not** a multi-tenant gateway; for that see LiteLLM/Portkey/OpenRouter."

This frames the product as a single-person productivity tool, collapses the "who is this for?" question, and ends with an explicit boundary that deflects the class of issues about teams / tenants / multi-provider that will otherwise flood the tracker.

Also add at the top of README:
```
## Who is this for?
- You have a Kiro subscription.
- You want to use claude-code / Cursor / other Anthropic-compatible clients against it.
- You are one person, or a small (<5) trusted group.

## Who is this NOT for?
- Multi-tenant AI platforms (use LiteLLM).
- Multi-provider routing across Claude/GPT/Gemini (use OpenRouter or 9router/OmniRoute).
- Enterprise observability + RBAC (use Helicone or Portkey enterprise).
```

---

## Appendix — Decision Rationale for Each "Gap?" Classification

- **P0** = feature that every serious peer ships and whose absence materially limits kiroxy's v1.0.0 story. 4 items.
- **P1** = high-value for the solo-user story, not yet table-stakes, but Quorinex / petehsu shipping them in the last 30 days means the bar is rising. 4 items.
- **P2** = useful but not defining; ship during post-v1.0 polish. ~9 items.
- **P3** = truly optional or anti-goal. Remaining items.

## Appendix — References for Any Factual Claim Above

- Tier 1 (v1) — `research/dossiers/DOSSIER_*.md` + `research/COMPARISON.md` + `research/RECOMMENDATION.md`.
- Tier 1 (v2) — `research-v2/dossiers/DOSSIER_*.md`.
- Tier 2 — `research-v2/tier2/TIER2_DOSSIER.md`.
- Delta recheck (2-week activity on v1 projects) — `research-v2/dossiers/DOSSIER_v1_delta_recheck.md`.
- kiroxy source of truth — `CHANGELOG.md`, `BACKLOG.md`, `internal/server/server.go` route table, `BUILD_LOG.md`.

---

_Compiled 2026-05-12 Asia/Makassar. All sources cross-cited in per-project dossiers._
