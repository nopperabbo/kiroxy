# kiroxy Competitive Analysis — May 2026

> **Status:** v0.3.0 shipped. v0.4.0 ≈ pool-mode refresh-wiring (Phase 2.5, in-flight). v1.0.0 target driven by this analysis.
> **Assumed reader:** has already read `BUILD_LOG.md`, `OVERNIGHT_LOG.md`, and the prior `research/` dossier set (May 2026). This doc compares **shipping kiroxy** against the wider ecosystem, not against its own build plan.
> **Method:** sources are cited per factual claim; LoC estimates are grounded in kiroxy's own package sizes. All "we should steal X" suggestions gate on a license compatibility check before adoption.
> **Last updated:** 2026-05-12.

---

## Executive Summary

_To be filled in after deep dives — anchored on observed gaps and actual kiroxy positioning._

- Where kiroxy wins: _TBD_
- Where kiroxy loses: _TBD_
- Top 3 features to add for v1.0.0: _TBD_
- Anti-goal to publicly deprioritize: _TBD_
- Strategic tweak to README messaging: _TBD_

---

## Projects Studied

| Project | Tier | Stars | Language | License | Last commit | Active? | Scope |
|---|---|---:|---|---|---|---|---|
| _Rows filled in during deep dives; re-counted fresh from GitHub this session._ | | | | | | | |

---

## Feature Matrix

Features vs projects. `✓` = has it, `~` = partial, `✗` = absent, `?` = unverified, `n/a` = out of scope for that project.

| Feature | jwadow/kiro-gateway | AIClient2API | Quorinex/Kiro-Go | hexos | KiroaaS | hj01857655/kam | caidaoli/kiro2api | kirocc | petehsu/KiroProxy | 9router | OmniRoute | Antigv-plugin | LiteLLM | Portkey | Helicone | OpenRouter (public) | **kiroxy v0.3.0** | **Gap in kiroxy?** |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| Auth: AWS Builder ID device code | | | | | | | | | | | | | n/a | n/a | n/a | n/a | | |
| Auth: Social (Google/MS OAuth) | | | | | | | | | | | | | n/a | n/a | n/a | n/a | | |
| Auth: Import pre-acquired refresh token | | | | | | | | | | | | | n/a | n/a | n/a | n/a | | |
| Auto token refresh (background) | | | | | | | | | | | | | | | | | | |
| Multi-account pool | | | | | | | | | | | | | | | | | | |
| Model resolver / alias mapping | | | | | | | | | | | | | | | | | | |
| Streaming SSE (Anthropic-compat) | | | | | | | | | | | | | | | | | | |
| Streaming SSE (OpenAI-compat) | | | | | | | | | | | | | | | | | | |
| OpenAI-compat API surface | | | | | | | | | | | | | | | | | | |
| Anthropic-compat API surface | | | | | | | | | | | | | | | | | | |
| Token counting (tiktoken/Anthropic tokenizer) | | | | | | | | | | | | | | | | | | |
| Cost / usage tracking (persisted) | | | | | | | | | | | | | | | | | | |
| Rate-limit awareness per account | | | | | | | | | | | | | | | | | | |
| Fallback orchestration (primary→secondary) | | | | | | | | | | | | | | | | | | |
| Request caching (prompt/response) | | | | | | | | | | | | | | | | | | |
| Request logging / audit trail | | | | | | | | | | | | | | | | | | |
| Dashboard (web UI) | | | | | | | | | | | | | | | | | | |
| CLI tooling depth | | | | | | | | | | | | | | | | | | |
| Docker / container support | | | | | | | | | | | | | | | | | | |
| Onboarding automation (token harvest) | | | | | | | | | | | | | | | | | | |
| Multi-provider (Claude+OpenAI+Gemini) | | | | | | | | | | | | | | | | | | |
| Custom model routing rules | | | | | | | | | | | | | | | | | | |
| Request transformation middleware | | | | | | | | | | | | | | | | | | |
| Circuit breaker / health tracking | | | | | | | | | | | | | | | | | | |
| Metrics export (Prometheus/OTel) | | | | | | | | | | | | | | | | | | |

---

## Project Deep Dives

### Tier 1 — Direct Kiro proxies

#### jwadow/kiro-gateway

_Deep dive pending. Placeholder for overview / architecture / unique strengths / known weaknesses / learn-from / already-do-better._

#### AIClient2API (justlovemaki/AIClient2API)

_Deep dive pending._

#### Quorinex/Kiro-Go

_Deep dive pending._

#### kadangkesel/hexos

_Deep dive pending — re-check for missed features beyond v1 dossier._

#### petehsu/KiroProxy

_Deep dive pending._

#### hj01857655/kiro-account-manager

_Deep dive pending — Rust Tauri desktop UI pattern._

#### decolua/9router

_Deep dive pending._

#### diegosouzapw/OmniRoute

_Deep dive pending._

#### AntiHub-Project/Antigv-plugin

_Deep dive pending._

### Tier 2 — AI gateways / multi-provider routers

#### LiteLLM (BerriAI/litellm)

_Deep dive pending — routing/fallback/cost/caching patterns._

#### Portkey-AI/gateway

_Deep dive pending — enterprise gateway features._

#### helicone/helicone

_Deep dive pending — observability for LLM traffic._

#### simonw/llm

_Deep dive pending — CLI-oriented architecture notes._

#### OpenRouter (public docs)

_Deep dive pending — closed-source but well-documented patterns._

#### TypingMind proxy patterns (public)

_Deep dive pending._

### Tier 3 — Auth / extraction tooling (brief)

_Camoufox/Playwright-based harvesters, AWS Builder ID token extractors, Cognito social auth proxies. Covered only where they informed a design decision in kiroxy or a competitor._

---

## Gaps in kiroxy

Gaps are prioritized against kiroxy's current state and the project's identity as a single-user self-hosted proxy. Each gap lists: what, who has it, why it matters, rough sketch, LoC estimate.

### P0 — Blocks the v1.0.0 "production-ready" claim

_Filled after feature matrix is complete._

### P1 — v1.1.0 target

_Filled after feature matrix is complete._

### P2 — v1.2.0+

_Filled after feature matrix is complete._

### P3 — Backlog

_Filled after feature matrix is complete._

---

## Unique kiroxy Strengths

_Validates positioning. Built from feature matrix deltas where only kiroxy has `✓`._

---

## White-space Opportunities

_Features that no project in the survey has — and that would matter. Each includes rough implementation cost and a why-now rationale._

---

## Recommendations

_Three concrete actions after all deep dives are complete:_

1. **Next feature to build (and why):** _TBD_
2. **Feature to publicly deprioritize (anti-goal):** _TBD_
3. **Strategic positioning tweak:** _TBD_

---

## Sources & Citations

_Every factual claim below is backed by a commit hash, issue URL, or README snippet pulled during this analysis. Entries are appended during deep dives._
