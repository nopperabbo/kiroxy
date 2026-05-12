# Tier 2 Dossier — AI Gateways & Multi-Provider Routers

Scope: AI gateways and multi-provider LLM routers that are *not* Kiro-specific. We study these for **patterns** (routing, fallback, cost, caching, observability, multi-tenant) that inform kiroxy's roadmap beyond Kiro.

All material collected 2026-05-12.

---

## 1. LiteLLM (BerriAI/litellm)

### Identity
- Repo: `BerriAI/litellm`
- License: **MIT** for everything outside `enterprise/`; `enterprise/` has its own commercial license (verified from `LICENSE`, SHA `3bfef5b`, 2023-dated MIT text).
- Language: Python (FastAPI for proxy; CLI `litellm --model gpt-4o`)
- Stars/status: YC W23 backed; very active (most recent commit on `main` 2026-05-12T09:52Z, merge `7bb5eb5b…` "Add pricing for openai/gpt-realtime-2").
- Used by: Stripe, Netflix, Google ADK, Greptile, OpenHands, OpenAI Agents SDK (per README OSS Adopters section).
- Scope: sprawling. Two surfaces: **SDK** (`uv add litellm`) and **AI Gateway Proxy** (`litellm --model gpt-4o`). 100+ providers × many endpoints (`/chat/completions`, `/responses`, `/embeddings`, `/images`, `/audio`, `/batches`, `/rerank`, `/a2a`, `/messages`). The `/messages` row confirms Anthropic Messages-API compatibility layer.

### Architecture summary
- Python `uvicorn`-served proxy; Prisma-managed DB (Postgres by default); Redis optional for cache/sync; UI at `ui/litellm-dashboard` (Next.js).
- Virtual keys stored in `LiteLLM_VerificationTokenTable`; spend in `LiteLLM_UserTable` + `LiteLLM_TeamTable`.
- Router layer handles retry / fallback / loadbalance across **deployments** (same model on multiple provider keys).
- Providers are plugins; each has its own param translator.

### Features relevant to kiroxy

| Feature | Present? | Notes |
|---|---|---|
| OpenAI `/v1/chat/completions` | ✓ | Universal |
| Anthropic `/v1/messages` | ✓ | Cross-provider — any model reachable via `/messages` (see README feature matrix, column 2) |
| Multi-provider auth pooling | ✓ | Per-deployment `api_key`; virtual keys allocate them transparently |
| Token-refresh background worker | ✓ | **Key Rotations** (enterprise) — `LITELLM_KEY_ROTATION_ENABLED` + `rotation_interval` + optional `grace_period` keeps old+new keys valid during cutover |
| Cost / usage tracking (persisted) | ✓ | Per key / user / team / org; `/spend/report` and `/spend/tags` endpoints; `completion_cost()` is the pricing pivot |
| Multi-tenant (Teams ↔ Users ↔ Keys) | ✓ (OSS) | Teams + Virtual Keys fully in MIT tier. Organizations (hierarchy layer) is enterprise-only. |
| Guardrails / content filtering | ✓ | Built-in set + "bring your own" via partners |
| Caching | ✓ | Simple + semantic (hosted/enterprise for semantic) |
| Admin UI | ✓ | Next.js dashboard with virtual key / spend / model management |
| Prometheus / OTel | ✓ (partial) | Logging/metrics section; exports to PostHog, GCS, Azure Blob. Prometheus is supported via compose |
| MCP Gateway | ✓ | Novel — MCP servers as first-class `"type": "mcp"` tools inside `/chat/completions`; also an MCP endpoint at `/mcp/` for Cursor IDE |
| A2A agents | ✓ | LangGraph, Vertex Agent Engine, Bedrock AgentCore, Pydantic AI |

### Unique strengths
- **"One interface for 100+ LLMs"** is genuinely the broadest provider support in OSS.
- **Virtual keys with spend budgets** is a complete multi-tenant primitive, not just auth. Per-key rate limits, per-team budgets, organization-level caps, all tracked in one schema.
- **Key rotation with grace period** is a pattern kiroxy should study for vault refresh (see below).
- **Router encryption affinity** — PR `#27703` (2026-05-12) pins Responses-API requests to the originating Azure resource via `(api_base, api_key)` boundary, even when model alias changes. Non-trivial correctness work.
- **Cost attribution hierarchy** — Org → Team → User → Key is crystallized in the data model; budgets enforce at every level.

### Known pain points (from recent commits)
- Budget cache invalidation across pods was buggy enough to warrant three follow-up fixes in May 2026 (`#27631` merge of `#27481` + `#27572` + MCP server eviction paths). Multi-pod cache coherence is apparently still being hardened.
- Pydantic-vs-dict internal API inconsistency caused a routing regression fixed in `40db114a` (2026-05-12). Implies the router layer has grown organically and is fragile in spots.
- No kiroxy-adjacent complaint: Kiro is not yet a supported provider (not in README table).

### What kiroxy could learn (license-safe since LiteLLM core is MIT)
1. **Virtual-key schema** — replace our ad-hoc `KIROXY_API_KEY` with `VerificationToken`-style records (`token_hash`, `user_id`, `team_id`, `spend_usd`, `budget_usd`, `rate_limit_rpm`, `allowed_models`). Even for single-user, this unlocks per-client spending visibility.
2. **Spend rollup worker** — background task that aggregates per-request cost into per-key / per-model totals; Kiro itself doesn't report cost, but we can compute it from token counts against Anthropic's public pricing table.
3. **Key rotation with grace period** — for vault refresh, retaining the previous refresh_token for N minutes after issuing a new one would make hot-reloads safer (kiroxy already has some of this in `tokenvault.Vault`, but the grace-period pattern is worth codifying).
4. **Router encryption affinity concept** — for kiroxy this maps to "never re-route a streaming response mid-stream to a different account"; LiteLLM's `_encryption_boundary_key` pattern is the reference implementation.
5. **MCP gateway pattern** — LiteLLM bridges MCP tools into `/chat/completions`; kiroxy could expose a simple `/mcp` endpoint routing to the user's local MCP servers, letting Kiro-backed Claude call them transparently. Novel + low-LoC.

### What kiroxy already does better
- Single-binary Go deploy beats Python/FastAPI/Prisma install footprint for local self-host.
- Kiro-specific tooling (Anthropic → Kiro translation, Builder ID auth, Camoufox harvester) is out of LiteLLM's scope entirely.

### Citations
- README: https://github.com/BerriAI/litellm/blob/main/README.md
- LICENSE: https://github.com/BerriAI/litellm/blob/main/LICENSE (SHA 3bfef5bae9b48c334acf426d5b7f21bc1913aab9)
- Multi-tenant docs: https://docs.litellm.ai/docs/proxy/multi_tenant_architecture
- Virtual keys docs: https://docs.litellm.ai/docs/proxy/virtual_keys
- Key rotation (`LITELLM_KEY_ROTATION_ENABLED`): https://docs.litellm.ai/docs/proxy/virtual_keys#key-rotations
- Router affinity fix PR: https://github.com/BerriAI/litellm/commit/f3b8aad883e502826078be4af7678c463242306d + https://github.com/BerriAI/litellm/commit/40db114a23142ad37ef8298ca43f837314426eb8

---

## 2. Portkey AI Gateway (Portkey-AI/gateway)

### Identity
- Repo: `Portkey-AI/gateway`
- License: **MIT** (verified `LICENSE` SHA `2c0759f8`, "Copyright (c) 2024 Portkey, Inc").
- Language: TypeScript; runs on Node, Cloudflare Workers, or Docker.
- Latest commit on `main`: `351692fd` on 2026-03-25 (a feature-freeze period — they're working on `2.0.0` branch per README top banner).
- Stated metrics: "<1ms latency", "tiny footprint (122kb)", "10B tokens/day across adopters".
- Supports 1600+ models across 45+ providers.

### Architecture summary
- Extremely lean: a TypeScript app with provider-specific "configs" (thin adapters).
- No database by default — stateless proxy.
- **Configs** are the central abstraction: declarative retry / load-balance / fallback / guardrail rules attached at request time via `client.with_options(config=...)`.
- Enterprise version adds DB-backed virtual keys, RBAC, PII redaction, SSO.

### Features relevant to kiroxy

| Feature | Present? | Notes |
|---|---|---|
| Fallbacks (provider A → B → C) | ✓ | Config-driven; declarative JSON |
| Automatic retries w/ backoff | ✓ | `{"retry": {"attempts": 5}}` with exponential backoff |
| Load balancing with weights | ✓ | Across providers or API keys |
| Request timeouts | ✓ | Per-request granular |
| Guardrails (input/output) | ✓ | 40+ pre-built + BYO + partner integrations |
| Multi-modal (vision/audio/image) | ✓ | Unified OpenAI signature |
| Realtime APIs (OpenAI Realtime, websockets) | ✓ | Integrated websocket server |
| Caching (simple + semantic) | ✓ | Semantic is hosted/enterprise |
| Virtual keys | ✓ (enterprise) | OSS has per-provider auth; virtual-key store is enterprise |
| MCP Gateway | ✓ | Auth + observability for MCP servers; identity forwarding; per-user ACL |

### Unique strengths
- **Config-as-data routing** — the declarative JSON config is far more expressive than LiteLLM's Python-y `Router` object. Composable. Can be switched per-request.
- **Runtime flexibility** — runs on Cloudflare Workers (edge), Node, Docker. A true edge gateway.
- **MCP Gateway** — arguably the cleanest MCP implementation in OSS. Single auth layer, full tool-call audit, identity forwarding. Claude Desktop/Cursor/VS Code compatible.
- **Gateway 2.0 (pre-release)** merges enterprise features into OSS (per README top note). Worth tracking.

### Known weaknesses (from commits)
- Header-forwarding bug (`x-portkey-forward-headxers` — typo preserved in commit message — causing infinite loops) fixed only in `351692fd` 2026-03-25.
- Main branch has been quiet since March 2026; all work is on the `2.0.0` branch, which is pre-release. Implies the current stable is frozen.

### What kiroxy could learn
1. **Declarative config object** — instead of ad-hoc env vars for routing, let users attach a config blob per-request (or per-API-key). Example kiroxy equivalent: `{"retry": {"attempts": 3}, "fallback": {"on_rate_limit": "next_account"}, "preferred_accounts": ["label1", "label2"]}`. Low-LoC win.
2. **Edge runtime compatibility** — kiroxy is Go, so Cloudflare Workers is out (WASM target would be a lift), but Fly.io / Railway / Render deploys could be smoother if the single binary stays under a similar footprint.
3. **MCP identity forwarding** — if kiroxy adds an MCP bridge, forwarding the requesting client's identity (who's calling which tool) is a cheap audit-log win.

### Citations
- README: https://github.com/Portkey-AI/gateway/blob/main/README.md
- LICENSE: https://github.com/Portkey-AI/gateway/blob/main/LICENSE (MIT, 2c0759f8)
- Recent routing fix: https://github.com/Portkey-AI/gateway/commit/de0e6c5a3f93e525eb25de0fe7495df343ff7783

---

## 3. Helicone (Helicone/helicone)

### Identity
- Repo: `Helicone/helicone`
- License: **Apache-2.0** (stated in README; LICENSE file retrieval rate-limited — confirmed from README "Helicone is licensed under the Apache v2.0 License").
- Language: TypeScript (multi-service).
- YC-backed; SOC2 + GDPR compliant.
- Most recent commit on `main`: `3f4bd44b` on 2026-05-02 ("fix: remove RAW request/response body debug logs" — dumping full bodies drove CloudWatch ingestion to ~1.5 TB/day, $22k in April).
- Also has the **AI Gateway** surface at `ai-gateway.helicone.ai` (100+ models, OpenAI format).

### Architecture summary
- 5 services: Web (Next.js dashboard), Worker (Cloudflare Workers proxy), Jawn (Express + Tsoa server for logs), Supabase (auth + app DB), ClickHouse (analytics DB), Minio (log object storage).
- Logs are the primary product; proxy is a means to collect them.
- Observability-first design: every request traced, every response cost-calculated, sessions grouped for multi-turn visibility.

### Features relevant to kiroxy

| Feature | Present? | Notes |
|---|---|---|
| Observability (logs, traces, sessions) | ✓✓ | Headline feature — session grouping for agents/chatbots |
| Cost / latency tracking per request | ✓✓ | Per-model, per-user; `/v1/public/stats/models/{model}` endpoint (commented out as of `548832f8` — backend endpoint not yet shipped) |
| Custom dashboards (PostHog export) | ✓ | One-line integration |
| Prompt versioning + deployment | ✓ | Prompts live in Helicone, changes don't require code redeploy |
| Playground (test prompts/sessions/traces) | ✓ | Built-in UI for iteration |
| Fine-tuning partners | ✓ | OpenPipe, Autonomi |
| Fallbacks | ✓ | Part of their AI Gateway |
| Unified 100+ model API | ✓ | Via AI Gateway |
| LLM cost API (public pricing db) | ✓ | 300+ models/providers; https://www.helicone.ai/llm-cost |

### Unique strengths
- **Sessions** — first-class concept for multi-turn agents. Not just request-level; session-level aggregation (total cost, latency across all turns).
- **LLM cost API** is a public resource — the pricing table for any Claude/OpenAI/etc model is queryable without scraping docs. Worth using as kiroxy's pricing source for cost estimation.
- **Prompt management** decoupled from app code — version prompts separately, deploy via gateway without redeployment.
- **ClickHouse for analytics** — the observability-scale choice. Postgres would choke at 10 MReq/day.

### Known weaknesses (from commits)
- **Cost catastrophe in April 2026**: raw request/response bodies were being console.log'd, CloudWatch ingestion hit 1.5 TB/day, ~$22k in April (own commit `3f4bd44b`). Honest disclosure in commit message. Cautionary tale — don't log raw bodies.
- `anthropic/claude-opus-4.6` pricing aliasing bug (`660ee54e`): their cost DB missed date-suffixed model variants, reporting $0.01 instead of ~$0.24 per request. Implies model-ID drift is a perennial issue.
- Dockerfile CVE comments were misleading and had to be cleaned up (`548832f8`).
- `ModelUsageSection` component references an unimplemented Jawn endpoint — UI work outpaces backend.

### What kiroxy could learn
1. **Session grouping** — if a user runs `claude-code` against kiroxy for 3 hours across 100 turns, treating those as one session (by client-provided session ID header) and rolling up cost/latency/tokens per session is useful. LoC: ~200.
2. **Use Helicone's LLM cost API as pricing source** — free data source for cost estimation; avoids us maintaining a pricing table.
3. **Don't log raw bodies.** Audit current request ring + any log sink for accidentally-persisted full request bodies.
4. **Model-ID aliasing rigor** — Helicone still has alias bugs; kiroxy's `internal/models/models.go` should have a test enumerating every user-facing label → canonical resolver ID (probably already via `opencode-config` tests; verify).

### Citations
- README: https://github.com/Helicone/helicone/blob/main/README.md
- RAW log fix: https://github.com/Helicone/helicone/commit/3f4bd44b85f9837feb4a696cce4bba6c99fbdc7e
- Cost alias bug: https://github.com/Helicone/helicone/commit/660ee54e7015a9c28eaa8accb8e628ab808634d2
- LLM cost API: https://www.helicone.ai/llm-cost

---

## 4. simonw/llm — CLI-oriented LLM client

### Identity
- Repo: `simonw/llm`
- License: **Apache-2.0** (badge in README).
- Language: Python (CLI tool + Python library).
- Most recent commit on `main`: `6952ff1c` on 2026-05-12 ("Ensure add_tool_call() is emitted as a Part, if necessary" — closes #1433).
- Release cadence: 0.32a1 on 2026-04-29 (commit `9a5c24e2`).
- Author: Simon Willison (Datasette, Django co-creator).

### Scope
Not a gateway, not a proxy, not multi-tenant. A **CLI + Python library** for prompting models with a plugin architecture. Included for one reason: its **plugin model** is the cleanest OSS example of provider-registration that kiroxy could learn from if we ever extend beyond Kiro.

### Architecture summary
- Python package installed via pip/brew/uv.
- Plugin hooks via entrypoints: `register_commands(cli)`, `register_models(register, model_aliases)`, `register_embedding_models(register)`, `register_tools(register)`, `register_template_loaders(register)`, `register_fragment_loaders(register)`.
- SQLite for prompt/response logging, embeddings, fragments.

### What kiroxy could learn
1. **Plugin hook architecture** — if kiroxy adds multi-provider, following simonw/llm's entrypoint plugin pattern is the cleanest way. Go equivalent via compile-time registration into a global registry (standard Go pattern).
2. **SQLite for everything** — prompts, responses, tools, fragments, embeddings all in one SQLite DB. kiroxy already uses SQLite for the vault; expanding for audit logging is natural.
3. **Fragments** — reusable prompt fragments with aliases.
4. **Tool calling + schemas on the client side** — we already do this via Anthropic tools passthrough.

### Citations
- README: https://github.com/simonw/llm/blob/main/README.md
- 0.32a1 release: https://github.com/simonw/llm/commit/9a5c24e20cbcc8ba24dbc112e2af32e6be7a1b8f
- Plugin hooks docs: https://llm.datasette.io/en/stable/plugins/plugin-hooks.html

---

## 5. OpenRouter (closed-source, public docs)

### Identity
- Closed-source; very well-documented.
- Unified API for 100+ models from ~50 providers.
- Single endpoint `https://openrouter.ai/api/v1/chat/completions` (OpenAI-compatible).
- SDK packages: `@openrouter/sdk` (TS), `openrouter` (Python).

### Routing architecture (documented in detail)
The `provider` object in every request body controls routing:

| Field | Purpose |
|---|---|
| `order: string[]` | Ordered list of provider slugs to try. |
| `allow_fallbacks: bool` | Whether to try backup providers when primary is down (default `true`). |
| `require_parameters: bool` | Only pick providers supporting every param in request (e.g., `tools`, `response_format`, `logit_bias`). |
| `data_collection: "allow" \| "deny"` | Filter providers by data-retention policy. |
| `only: string[]` | Allow list of provider slugs. |
| `ignore: string[]` | Deny list of provider slugs. |
| `quantizations: ["int4" \| "int8" \| ...]` | Precision-based filtering. |
| `sort: "price" \| "throughput" \| "latency"` | Live-metrics-based ordering. |
| `max_price: {prompt, completion, image, audio, total}` | Hard price ceiling per request. |
| `zdr: bool` | Restrict to Zero-Data-Retention providers. |
| `api_keys: {provider: key}` | BYOK — use your own provider keys. |

### Default behavior
> "Prioritize providers that have not seen significant outages in the last 30 seconds. For the stable providers, look at the lowest-cost candidates and select one weighted by inverse square of the price. Use the remaining providers as fallbacks."

### Model fallbacks
Top-level `models: string[]` parameter — array of model IDs in priority order. If the primary model's providers all fail (rate-limited, moderation, downtime, context-length), next model is tried automatically. Billed at the model that actually ran (returned in `model` response field).

`sort.partition: "model" | "none"` controls grouping: `"model"` (default) tries all endpoints of the primary model first before falling back; `"none"` sorts across all models+providers globally.

### What kiroxy could learn
OpenRouter is closed-source, so we cannot copy code. We can copy **API surface patterns**:

1. **`provider.order` + `allow_fallbacks` model** — industry-standard UX for multi-provider routing.
2. **`sort: "price" | "throughput" | "latency"`** — even single-provider Kiro with multiple accounts benefits from `sort: "cooldown"` and `sort: "least_used_today"`. Fits `internal/pool` naturally.
3. **`models: string[]` fallback cascade** — if Kiro rate-limits, try a user-configured fallback (e.g., personal Anthropic key for emergencies).
4. **`max_price.{prompt,completion,total}`** — cost guardrails. LoC: ~100.
5. **"Inverse square of price" weighting** — for account selection, inverse-square-of-recent-failures is cheap but effective.

### Citations
- Provider routing: https://openrouter.ai/docs/guides/routing/provider-selection.mdx
- Model fallbacks: https://openrouter.ai/docs/guides/routing/model-fallbacks
- Principles: https://openrouter.ai/docs/guides/overview/principles.mdx
- AI SDK provider deepwiki: https://deepwiki.com/OpenRouterTeam/ai-sdk-provider/6.2-provider-routing-and-fallbacks

---

## 6. TypingMind proxy (brief)

TypingMind is a chat UI that exposes a proxy pattern to let users BYO provider keys and switch between OpenAI / Anthropic / Gemini transparently. Public docs are thin; the product is primarily a closed-source Electron app. **Pattern of note**: per-conversation "plugin" store that injects system prompts or tool definitions based on matched regexes.

No competitive threat to kiroxy (different surface: end-user chat UI, not API proxy).

---

## Tier 2 Cross-Cutting Patterns Worth Adopting

Distilled from all five projects above. Ordered by estimated ROI for kiroxy.

| Pattern | Source(s) | kiroxy LoC estimate | Why it matters for kiroxy |
|---|---|---:|---|
| **Virtual keys with spend + budget + rate limits** | LiteLLM | 600-900 | Even single-user, per-client visibility is a 10x dashboard upgrade. Also lets kiroxy say "$X/month, $Y remaining" in the UI. |
| **Session grouping for multi-turn agents** | Helicone | 200-300 | Claude Code sessions last hours; aggregating cost/latency/tokens per session is the natural unit of analysis. |
| **Use Helicone's LLM cost API as pricing source** | Helicone | 50 | Free data source for cost estimation; avoids maintaining our own pricing table. |
| **Declarative config object per-request** | Portkey | 300-400 | `{retry, fallback, preferred_accounts}` as inline config beats env vars for flexibility. |
| **`provider.order` + `allow_fallbacks` API shape** | OpenRouter | 400-600 | If kiroxy ever adds multi-provider, industry-standard UX. Prep with internal abstractions even if only Kiro wired. |
| **Key rotation with grace period** | LiteLLM | 100-150 | Hot-swap refresh tokens without 10-second reconnect glitches. |
| **Cost guardrails (`max_price`)** | OpenRouter | 100 | Reject requests whose pre-estimate exceeds user-configured ceiling. |
| **Plugin-style provider registry** | simonw/llm | 200 | Compile-time registration for future multi-provider. Prep-work only. |
| **Don't log raw bodies** | Helicone cautionary | 0 (audit) | Audit current request ring + any log sink for accidentally-persisted full request bodies. |
| **MCP bridge in `/chat/completions`** | LiteLLM + Portkey | 400-600 | Expose local MCP tools through kiroxy; let Kiro-backed Claude call them. Novel + differentiating. |

---

## Notes on Out-of-Scope Projects

Considered and skipped:
- **Vercel AI SDK** — Node SDK, not a gateway.
- **LangChain / LlamaIndex** — orchestration frameworks.
- **Replicate / Together / Fireworks** — upstream providers, not gateways.
- **PromptLayer / Langfuse / Weights & Biases LLM monitoring** — pure observability; Helicone covers the same ground.

---

_Compiled 2026-05-12 Asia/Makassar. All sources retrievable as of this date; some LICENSE files rate-limited and cross-checked via README-level license statements._
