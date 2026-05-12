# kiroxy Client Compatibility Matrix — ECOSYSTEM.md

> Wire-level facts about how each major AI coding client speaks to an Anthropic-or-OpenAI-compatible base URL.
> Every claim is cited with a GitHub permalink (commit SHA pinned) or an official docs URL.
> Last verified: 2026-05-13. Current-year research.

**Scope:** What a kiroxy implementer must know to serve each client correctly, without reverse-engineering each client themselves.

**Primary target:** `opencode` (section 6 — most detailed).

---

## Reading key

- **Anthropic-native** = client POSTs `/v1/messages`, parses Anthropic SSE event types (`message_start`, `content_block_*`, `message_delta`, `message_stop`, `ping`, `error`), expects JSON error shape `{"type":"error","error":{"type","message"}}`, auth via `x-api-key` or `Authorization: Bearer`, requires `anthropic-version: 2023-06-01`.
- **OpenAI-compat** = client POSTs `/v1/chat/completions`, parses OpenAI SSE (`data: {...}\n\n` + terminator `data: [DONE]\n\n`), expects JSON error shape `{"error":{"message","type","code"}}`, auth via `Authorization: Bearer`.
- **Both** = client has separate providers and picks per-config.

---

## 1. claude-code (Anthropic official CLI)

Source: [anthropics/claude-code](https://github.com/anthropics/claude-code) — mostly closed-source runtime; public docs at `docs.claude.com/en/docs/claude-code` are authoritative.

### 1.1 Endpoint shape
**Anthropic-native only.** When `ANTHROPIC_BASE_URL` is set, Claude Code POSTs to `{BASE_URL}/v1/messages` and `{BASE_URL}/v1/messages/count_tokens`, forwarding `anthropic-beta` and `anthropic-version` headers. ([LLM gateway docs](https://docs.claude.com/en/docs/claude-code/llm-gateway))

Three alternate formats supported (per-gateway env vars):
- Anthropic Messages (default via `ANTHROPIC_BASE_URL`)
- Bedrock InvokeModel (`ANTHROPIC_BEDROCK_BASE_URL`)
- Vertex rawPredict (`ANTHROPIC_VERTEX_BASE_URL`)

A kiroxy proxy MUST expose `/v1/messages` to work with claude-code out of the box.

### 1.2 Request body shape
- Anthropic native: top-level `model`, `messages[]`, `system` (string or `[{type:"text", text, cache_control?}]`), `tools[]`, `tool_choice`, `max_tokens`, `temperature`, `stream` (defaults to streaming in CLI).
- `cache_control: {"type":"ephemeral"}` blocks are set on system + tools + last user message.
- Extended thinking: `thinking: {type:"enabled", budget_tokens}` or `{type:"adaptive"}` for Opus 4.5+.
- Adds these Claude Code headers ([docs](https://docs.claude.com/en/docs/claude-code/llm-gateway#request-headers)):
  - `X-Claude-Code-Session-Id` — per session
  - `X-Claude-Code-Agent-Id` — per subagent (present only for in-process subagents)
  - `X-Claude-Code-Parent-Agent-Id` — for nested agents
- Prepends an attribution block to the system prompt (client version + prompt fingerprint). Can be disabled via `CLAUDE_CODE_ATTRIBUTION_HEADER=0` to improve proxy cache-hit rates.

### 1.3 Response parsing
Claude Code parses the full Anthropic SSE event sequence. Proxies MUST emit standard events (`message_start` → `content_block_start` → `content_block_delta` (text/input_json/thinking/signature) → `content_block_stop` → `message_delta` → `message_stop`). `ping` events are tolerated. `usage.cache_creation_input_tokens` / `usage.cache_read_input_tokens` are read for cost accounting. Tool use blocks parsed as `{type:"tool_use", id, name, input}`.

### 1.4 Timeouts
Default: `API_TIMEOUT_MS = 600000` (10 min); max `2147483647` ms; values above max overflow the timer and fail immediately. Configurable via env var. ([env-vars docs](https://docs.claude.com/en/docs/claude-code/env-vars))

Separate `CLAUDE_ASYNC_AGENT_STALL_TIMEOUT_MS = 600000` for background-subagent stall detection; resets on each streaming progress event.

### 1.5 Retry behavior
Claude Code has built-in retry for 429 (honors `Retry-After`), 5xx, and transient network errors. Exact backoff is closed-source, but issues indicate exponential with jitter. Retry loops are per-request; a proxy returning a properly formed 429 with `retry-after` will be respected.

### 1.6 Error shape
Parses Anthropic error JSON `{"type":"error","error":{"type":"<type>","message":"<msg>"}}`. Known error types surfaced to user ([see error reference](https://docs.claude.com/en/docs/claude-code/errors)): `overloaded_error` (529), `rate_limit_error` (429), `api_error` (500), `invalid_request_error` (400), `authentication_error` (401).

### 1.7 Session model
**Full history replay each turn.** No `conversation_id` header. `X-Claude-Code-Session-Id` is informational (for proxy attribution) and does NOT cause the upstream to persist state. Proxy must therefore not assume stickiness.

### 1.8 Proxy config surface (env vars)
From [env-vars](https://docs.claude.com/en/docs/claude-code/env-vars):
- `ANTHROPIC_BASE_URL` — override endpoint (most common)
- `ANTHROPIC_API_KEY` — sent as `x-api-key` header
- `ANTHROPIC_AUTH_TOKEN` — sent as `Authorization: Bearer <value>` (preferred for proxies)
- `ANTHROPIC_CUSTOM_HEADERS` — newline-separated `Name: Value` pairs
- `ANTHROPIC_BETAS` — comma-separated, appended to `anthropic-beta` header
- `CLAUDE_CODE_ATTRIBUTION_HEADER=0` — strip attribution block for better cache
- `CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY=1` — query `/v1/models` on startup (v2.1.129+)

When base URL is non-first-party, **MCP tool_search is disabled by default**. Set `ENABLE_TOOL_SEARCH=true` if proxy forwards `tool_reference` blocks.

### 1.9 Known proxy quirks (issue tracker)
- [#52307](https://github.com/anthropics/claude-code/issues/52307) — 2.1.118 regression: 401 with custom `ANTHROPIC_BASE_URL` for third-party providers (auth header flipped). Rollback to 2.1.112 fixes.
- [#49932](https://github.com/anthropics/claude-code/issues/49932) — Organization-level policies bypassed when `ANTHROPIC_BASE_URL` is set despite OAuth.
- [#50085](https://github.com/anthropics/claude-code/issues/50085) — Missing doc: `CLAUDE_CODE_ATTRIBUTION_HEADER=0` is required to keep prompt-cache hit rate high when routing through a proxy.
- [#56310](https://github.com/anthropics/claude-code/issues/56310) — VS Code extension silently drops image/file attachments when using custom model backend.
- [#43663](https://github.com/anthropics/claude-code/issues/43663) — Crash in `isSearchOrReadCommand` on Windows with custom `ANTHROPIC_BASE_URL`.
- [#48365](https://github.com/anthropics/claude-code/issues/48365) — `ANTHROPIC_DEFAULT_*_MODEL` from settings.json injected even in OAuth mode, but `BASE_URL` is not, causing tool failures.
- [#48011](https://github.com/anthropics/claude-code/issues/48011) — Feature req: make OAuth/admin base URL configurable alongside `ANTHROPIC_BASE_URL`.
- [#57964](https://github.com/anthropics/claude-code/issues/57964) — `CLAUDE_CODE_AUTO_COMPACT_WINDOW` capped at hardcoded model context window, preventing larger-context custom APIs from working.

**Implication for kiroxy:** Must return `anthropic-version`-compatible responses, forward `anthropic-beta` correctly, and support both `x-api-key` and `Authorization: Bearer` auth styles. Do NOT strip attribution blocks unless client sets `CLAUDE_CODE_ATTRIBUTION_HEADER=0` itself. Provide a `/v1/models` endpoint returning only IDs that start with `claude` or `anthropic` if gateway-model-discovery is a goal.

---

## 2. Cursor (closed-source, cursor.com)

Cursor is closed-source; facts are drawn from official docs, community tutorials, and reverse-engineered integration guides.

### 2.1 Endpoint shape
**OpenAI-compat ONLY.** Cursor's "Override OpenAI Base URL" sends POST to `{BASE_URL}/chat/completions`. Cursor appends `/chat/completions` to the exact URL you provide — include `/v1` suffix yourself. ([ofox.ai guide 2026](https://ofox.ai/blog/cursor-claude-code-cline-custom-api-setup-2026/), [superagent docs](https://docs.superagent.sh/cursor-ide))

There is no Anthropic-native path. Even Claude models must be served via OpenAI-format translation at the proxy.

### 2.2 Request body shape
Standard OpenAI Chat Completions shape:
- `messages[]` with `system` as first message (role "system"), tools via OpenAI function-calling `tools: [{type:"function", function:{name,description,parameters}}]`, `tool_choice`, `stream: true` for chat.
- The model name must EXACTLY match what the endpoint's `model` field expects. Cursor whitelists model names by string — custom models must be added via "+ Add Model" in Settings ([coderouter.io integrations](https://www.coderouter.io/docs/integrations)).

### 2.3 Response parsing
OpenAI SSE: `data: {...}\n\ndata: {...}\n\n...data: [DONE]\n\n`. `delta.content` chunks, `delta.tool_calls[index]` for streaming function calls, `usage` in final non-`[DONE]` chunk when `stream_options: {include_usage: true}` is set.

### 2.4 Timeouts
**No user-facing timeout setting.** Cursor has an internal default timeout that causes failures on slow models. Community-documented, not officially specified ([ofox.ai guide 2026](https://ofox.ai/blog/cursor-claude-code-cline-custom-api-setup-2026/)). *Unverified but widely reported: ~5 minutes.*

### 2.5 Retry behavior
Community-reported: Cursor retries on transient 5xx with exponential backoff. Auto-fallback between models on error is a product feature (Composer). Exact algorithm not published.

### 2.6 Error shape
OpenAI-style `{"error":{"message","type","code"}}`. Surfaced to user as in-UI error banner.

### 2.7 Session model
**Full history replay each turn.** No public session-ID header for upstream. Composer maintains its own local conversation state; each LLM call is a fresh stateless POST with the accumulated messages.

### 2.8 Proxy config surface (UI-only)
Location: Cursor Settings > Models > toggle "Override OpenAI Base URL" ON.
- `Base URL` text field
- `OpenAI API Key` text field (sent as `Authorization: Bearer <key>`)
- "Verify" button performs a test against built-in model first (OK if it fails so long as auth works) ([coderouter.io docs](https://www.coderouter.io/docs/integrations))
- Custom models added via "+ Add Model"

No env var support. No per-model base-URL override. Tab completion continues using Cursor's built-in models — only chat/Composer/inline edit routes through custom endpoints.

### 2.9 Known proxy quirks
- **"Verify" step fails on first attempt** — Cursor tests the default model, not the added one. The check confirms auth only; real requests work once a custom model is added.
- **No streaming retries visible to user** — a broken stream mid-response surfaces as a truncated reply with no retry.
- **Double-`/v1` on URL misconfiguration** — Cursor appends `/chat/completions` to exact base URL; if your proxy expects `/v1/chat/completions` at the path, base URL must end with `/v1`.
- **The "Anthropic API Key" field is separate** — only used for direct Anthropic connections, NOT for custom endpoints, even if those endpoints serve Claude.
- **Key stored plaintext-style in Cursor settings** — no env var / keychain path.
- **Agent mode quirks with custom base URLs** — historical community reports that Composer works only with a subset of models on custom endpoints.

**Implication for kiroxy:** Must expose OpenAI-compat `/v1/chat/completions`. Must accept `Authorization: Bearer` with any key (since Cursor sends the user-configured key verbatim). If serving Claude via OpenAI-format, kiroxy must translate tool_use → tool_calls at the boundary. Do NOT assume `stream_options: {include_usage: true}` — Cursor may or may not set it.

---

## 3. Cline (VSCode extension, formerly Claude Dev)

Source: `cline/cline` at commit [`03f47045f338dcb6ac45b1ac1d6279a78be2b118`](https://github.com/cline/cline/tree/03f47045f338dcb6ac45b1ac1d6279a78be2b118) (2026-05-11).

### 3.1 Endpoint shape (per provider)
Cline has **40+ provider implementations** under `src/core/api/providers/`. Per-provider wire protocol:

| Provider | File | Endpoint shape |
| --- | --- | --- |
| `anthropic` | [`anthropic.ts`](https://github.com/cline/cline/blob/03f47045f338dcb6ac45b1ac1d6279a78be2b118/src/core/api/providers/anthropic.ts) | Anthropic-native `/v1/messages` via `@anthropic-ai/sdk` |
| `openai` (OpenAI-compat / custom) | [`openai.ts`](https://github.com/cline/cline/blob/03f47045f338dcb6ac45b1ac1d6279a78be2b118/src/core/api/providers/openai.ts) | OpenAI `/v1/chat/completions` |
| `openai-native` | [`openai-native.ts`](https://github.com/cline/cline/blob/03f47045f338dcb6ac45b1ac1d6279a78be2b118/src/core/api/providers/openai-native.ts) | OpenAI responses API + WebSocket for realtime |
| `openrouter` | `openrouter.ts` | OpenAI-format with custom OR headers |
| `claude-code` | `claude-code.ts` | Spawns claude-code CLI subprocess |
| `litellm` | `litellm.ts` | LiteLLM proxy (OpenAI-format typically) |

Cline instantiates `@anthropic-ai/sdk` with `baseURL: options.anthropicBaseUrl` at [anthropic.ts#L51-L56](https://github.com/cline/cline/blob/03f47045f338dcb6ac45b1ac1d6279a78be2b118/src/core/api/providers/anthropic.ts#L42-L61).

### 3.2 Request body
**Anthropic provider** ([anthropic.ts#L114-L159](https://github.com/cline/cline/blob/03f47045f338dcb6ac45b1ac1d6279a78be2b118/src/core/api/providers/anthropic.ts#L114-L159)):
```ts
{
  model: modelId,
  thinking: { type: "enabled"|"adaptive", budget_tokens },
  max_tokens: info.maxTokens ?? 8192,
  temperature: 0,
  system: [{ text, type: "text", cache_control: { type: "ephemeral" } }],
  messages: sanitizeAnthropicMessages(msgs, true),
  stream: true,
  tools,           // Anthropic native format
  tool_choice: { type: "any" },  // or undefined when thinking enabled
}
```
When `supportsPromptCache` true, Cline puts cache breakpoint on system and tools only (avoids per-message breakpoints to save overhead).
When `speed: "fast"` on fast-mode models, sends `client.beta.messages.create` with `betas: ["fast-mode-2026-02-01"]`.
1M-context models toggle `anthropic-beta: context-1m-2025-08-07`.

**OpenAI provider** ([openai.ts#L96-L140](https://github.com/cline/cline/blob/03f47045f338dcb6ac45b1ac1d6279a78be2b118/src/core/api/providers/openai.ts#L96-L140)): standard OpenAI Chat Completions — `messages[]` with system prepended, `tools`, `tool_choice`, `stream: true`, `max_tokens`, `temperature`, `reasoning_effort` for o1/o3/o4/gpt-5 family.

### 3.3 Response parsing
Anthropic provider reads ([anthropic.ts#L178-L299](https://github.com/cline/cline/blob/03f47045f338dcb6ac45b1ac1d6279a78be2b118/src/core/api/providers/anthropic.ts#L178-L299)): `message_start` (usage.input_tokens, cache_creation_input_tokens, cache_read_input_tokens), `message_delta` (output_tokens), `message_stop`, `content_block_start` (thinking/redacted_thinking/tool_use/text), `content_block_delta` (thinking_delta/signature_delta/text_delta/input_json_delta), `content_block_stop`. Does NOT handle `ping` or `error` explicitly — falls through switch default (tolerated). Tool use is converted to OpenAI-compat tool_calls internally.

OpenAI provider reads usage via `prompt_tokens`, `completion_tokens`, `prompt_tokens_details.cached_tokens`. `cache_write_tokens = 0` for OpenAI (always).

### 3.4 Timeouts
No explicit request-level timeout in code; relies on `@anthropic-ai/sdk` defaults (approx 10 minutes per SDK default). Users cannot configure.

### 3.5 Retry behavior
Decorator `@withRetry()` at [retry.ts#L29-L88](https://github.com/cline/cline/blob/03f47045f338dcb6ac45b1ac1d6279a78be2b118/src/core/api/retry.ts#L29-L88):
- `maxRetries: 3`, `baseDelay: 1000ms`, `maxDelay: 10000ms`
- Retries **only on 429 or `RetriableError`** (not all errors by default)
- Reads `retry-after`, `x-ratelimit-reset`, `ratelimit-reset` from headers
- Handles delta-seconds AND Unix-timestamp formats
- Exponential backoff (delay doubles per attempt) when no header present

### 3.6 Error shape
Wraps `@anthropic-ai/sdk` errors. Surfaces `{"message","modelId","providerId"}` to user (e.g., [#7528](https://github.com/cline/cline/issues/7528)).

### 3.7 Session model
**Full history replay each turn.** Cline maintains message history in the extension (`ClineStorageMessage[]`) and sends the full array on every call. Context compression via a separate summarization pass when over window.

### 3.8 Proxy config surface
- `anthropicBaseUrl` — string in user settings ([state-keys.ts#L105](https://github.com/cline/cline/blob/03f47045f338dcb6ac45b1ac1d6279a78be2b118/src/shared/storage/state-keys.ts#L105))
- `openAiBaseUrl` — string (for OpenAI-compat provider)
- `openAiHeaders` — `Record<string, string>` for custom auth (sent as `defaultHeaders`)
- `anthropicApiKey`, `openAiApiKey`, etc. — stored in VSCode secret storage
- **No env var support** — configuration is UI-only in the "Cline" extension settings pane
- Remote-config can override via `anthropicSettings.baseUrl` ([utils.ts#L212](https://github.com/cline/cline/blob/03f47045f338dcb6ac45b1ac1d6279a78be2b118/src/core/storage/remote-config/utils.ts#L212))

### 3.9 Known proxy quirks
- [#7114](https://github.com/cline/cline/issues/7114), [#7128](https://github.com/cline/cline/issues/7128) — "Base URL field missing in OpenAI Provider" (regression, now closed).
- [#7528](https://github.com/cline/cline/issues/7528) — Generic "Connection error" surfaced without upstream error-type detail.
- [#4633](https://github.com/cline/cline/issues/4633) — Historical feature req for custom provider with configurable base URL, API key, model ID.
- [#6924](https://github.com/cline/cline/issues/6924) — OpenAI-compatible provider CLI auth does not work (only VSCode UI path is reliable).
- [#8586](https://github.com/cline/cline/issues/8586) / [#8348](https://github.com/cline/cline/issues/8348) — Telemetry still sent to `otel.cline.bot` despite custom OTEL endpoint (separate from LLM, but worth noting for privacy-sensitive proxy deployments).
- [#76](https://github.com/cline/cline/issues/76) — Historical feature req for "Custom OpenAPI/Claude Endpoint Support" (implemented).

**Implication for kiroxy:** Expose `/v1/messages` with full Anthropic SSE + tolerate tools on system-level cache breakpoint. Must handle Cline's `speed: "fast"` beta header and `anthropic-beta: context-1m-2025-08-07` passthrough. 429 retry-after is strictly honored — emit properly formatted headers.

---

## 4. Continue

Source: `continuedev/continue` at commit [`cb273098d968906d25ee737b454f0b5f13ea2482`](https://github.com/continuedev/continue/tree/cb273098d968906d25ee737b454f0b5f13ea2482) (2026-04-16).

### 4.1 Endpoint shape (per provider)
Each provider is a `BaseLLM` subclass in `core/llm/llms/`:

| Provider | File | Endpoint |
| --- | --- | --- |
| `anthropic` | [`Anthropic.ts#L43`](https://github.com/continuedev/continue/blob/cb273098d968906d25ee737b454f0b5f13ea2482/core/llm/llms/Anthropic.ts#L43) | `apiBase: "https://api.anthropic.com/v1/"` + relative `messages` → `/v1/messages` |
| `openai` | [`OpenAI.ts#L204`](https://github.com/continuedev/continue/blob/cb273098d968906d25ee737b454f0b5f13ea2482/core/llm/llms/OpenAI.ts#L204) | `apiBase: "https://api.openai.com/v1/"` + `chat/completions` |
| `openai-compatible` | via `OpenAI.ts` with custom `apiBase` | Same as openai |
| `bedrock`, `gemini`, `ollama`, `deepseek`, `mistral`, `groq`, etc. | separate files | Provider-specific |

Critical wire fact (Anthropic): URL built via `new URL("messages", this.apiBase)` at [`Anthropic.ts#L446`](https://github.com/continuedev/continue/blob/cb273098d968906d25ee737b454f0b5f13ea2482/core/llm/llms/Anthropic.ts#L446). This means `apiBase` MUST end with trailing slash (`/v1/`) or the path is dropped. A kiroxy pointed at `https://proxy.example.com/anthropic` would break — user must configure `apiBase: "https://proxy.example.com/anthropic/v1/"`.

OpenAI URL built via `this._getEndpoint("chat/completions")` at [`OpenAI.ts#L542`](https://github.com/continuedev/continue/blob/cb273098d968906d25ee737b454f0b5f13ea2482/core/llm/llms/OpenAI.ts#L542) — also uses `new URL(endpoint, this.apiBase)`.

### 4.2 Request body
Anthropic ([`Anthropic.ts#L62-L102`](https://github.com/continuedev/continue/blob/cb273098d968906d25ee737b454f0b5f13ea2482/core/llm/llms/Anthropic.ts#L62-L102)):
```ts
{
  top_k, top_p, temperature,
  max_tokens: options.maxTokens ?? 2048,
  model,
  stop_sequences,
  stream: options.stream ?? true,
  tools: options.tools?.map(convertToolToAnthropicTool),
  thinking: { type:"enabled", budget_tokens: 4096 } if options.reasoning,
  tool_choice: options.toolChoice ? { type:"tool", name } : undefined,
  messages: convertMessages(...),
  system: "text" | [{type:"text", text, cache_control:{type:"ephemeral"}}]
}
```
Cache control toggled via `this.cacheBehavior.cacheSystemMessage` / `cacheConversation` or `this.completionOptions.promptCaching`. Helper `addCacheControlToLastTwoUserMessages` is used from `@continuedev/openai-adapters`.

### 4.3 Response parsing
Anthropic at [`Anthropic.ts#L311-L400`](https://github.com/continuedev/continue/blob/cb273098d968906d25ee737b454f0b5f13ea2482/core/llm/llms/Anthropic.ts#L311-L400). Events: `message_start` (captures usage.input_tokens, cache_read_input_tokens, cache_creation_input_tokens), `message_delta` (output_tokens), `content_block_start` (tool_use id/name, redacted_thinking passthrough), `content_block_delta` with deltas: `text_delta`, `thinking_delta`, `signature_delta`, `input_json_delta`. Uses `streamSse` from `@continuedev/fetch`. No explicit `ping` handler (tolerated).

### 4.4 Timeouts
Default `TIMEOUT = 7200` seconds (2 hours) in [`getAgentOptions.ts#L12-L13`](https://github.com/continuedev/continue/blob/cb273098d968906d25ee737b454f0b5f13ea2482/packages/fetch/src/getAgentOptions.ts#L12-L13). Configurable per-request via `requestOptions.timeout` (seconds) in `config.yaml`.

### 4.5 Retry behavior
Retry utilities exist at [`retry.ts`](https://github.com/continuedev/continue/blob/cb273098d968906d25ee737b454f0b5f13ea2482/core/llm/utils/retry.ts):
- `@withRetry`, `withLLMRetry`, `retryAsync`
- Default: `maxAttempts: 3`, `baseDelay: 1000ms`, `maxDelay: 30000ms`, `jitterFactor: 0.3`
- `defaultShouldRetry`: retries on ENOTFOUND/ECONNRESET/ECONNREFUSED/ETIMEDOUT, AWS throttling, 429, 5xx
- **Honors `retry-after`, `x-ratelimit-reset`, `ratelimit-reset` headers** (delta-seconds or HTTP-date formats) — [`retry.ts#L135-L175`](https://github.com/continuedev/continue/blob/cb273098d968906d25ee737b454f0b5f13ea2482/core/llm/utils/retry.ts#L135-L175)

**BUT**: Retry is NOT applied to provider `_streamChat` by default. Providers must opt-in. Anthropic.ts / OpenAI.ts at this SHA do not use the decorator, so a streaming failure is not auto-retried by default.

### 4.6 Error shape
Uses `@continuedev/openai-adapters` helper `getAnthropicErrorMessage`. For OpenAI-style providers, reads `error.message`. Surfaces a generic "Connection error" for many upstream failures (tracking issue [#11818](https://github.com/continuedev/continue/issues/11818)).

### 4.7 Session model
Full history replay. Continue manages a chat history ring and sends the full accumulated array on each call. No `conversation_id` header.

### 4.8 Proxy config surface
- `apiBase` in `config.yaml` per-model (example: `apiBase: https://my-proxy.com/v1/`)
- `requestOptions.headers` for custom auth headers
- `requestOptions.timeout` (seconds)
- `requestOptions.verifySsl`, `caBundlePath`, `proxy`, `noProxy`, `clientCertificate` ([`index.d.ts#L1064-L1074`](https://github.com/continuedev/continue/blob/cb273098d968906d25ee737b454f0b5f13ea2482/core/index.d.ts#L1064-L1074))
- `cacheBehavior: { cacheSystemMessage, cacheConversation }`
- No standard env var support in core — providers may read `OPENAI_API_KEY` etc. via user config interpolation.

### 4.9 Known proxy quirks
- [#11872](https://github.com/continuedev/continue/issues/11872) — "Invalid URL" errors across multiple providers (`apiBase` normalization).
- [#10474](https://github.com/continuedev/continue/issues/10474) — `exponentialBackoff` bypasses apiBase check for Responses API routing.
- [#8292](https://github.com/continuedev/continue/issues/8292) — Ollama on Mac: `apiBase` not working as expected.
- [#10696](https://github.com/continuedev/continue/issues/10696) — 404 on Llama 3.1 8B local.
- [#11818](https://github.com/continuedev/continue/issues/11818) — Tracking: generic "Connection error" across multiple providers, often a base-URL misconfig.
- [#12191](https://github.com/continuedev/continue/issues/12191) — Model names wrongfully trimmed to last delimiter in CLI `config.yaml`.

**Implication for kiroxy:** User config MUST instruct trailing slash on `apiBase`. Proxy should be lenient on both `https://proxy/v1/messages` and `https://proxy/v1/` + `messages`. Emit properly structured anthropic errors since Continue uses `@continuedev/openai-adapters` `getAnthropicErrorMessage` to parse them.


---

## 5. aider

Source: `Aider-AI/aider` at commit [`3ec8ec5a7d695b08a6c24fe6c0c235c8f87df9af`](https://github.com/Aider-AI/aider/tree/3ec8ec5a7d695b08a6c24fe6c0c235c8f87df9af) (2026-04-25).

**Architecture note:** aider does NOT speak wire protocols itself. It delegates to [LiteLLM](#8-litellm) — so aider's wire behavior == LiteLLM's. aider passes requests via `litellm.completion(**kwargs)` at [`models.py#L1029`](https://github.com/Aider-AI/aider/blob/3ec8ec5a7d695b08a6c24fe6c0c235c8f87df9af/aider/models.py#L1029).

### 5.1 Endpoint shape
Routing is governed by LiteLLM model-string prefix:
- `anthropic/claude-3-5-sonnet` → Anthropic `/v1/messages`
- `openai/gpt-4o` → OpenAI `/v1/chat/completions`
- `openrouter/anthropic/claude-3.5` → OpenRouter
- `bedrock/anthropic.claude-3-5-sonnet` → Bedrock InvokeModel
- No prefix → LiteLLM inferred based on model name heuristics

### 5.2 Request body
Pre-processed by aider:
- Diff-format messages (search/replace blocks, unified diff depending on `--edit-format`)
- Repo map injected as system context
- **Cache warming**: aider has a dedicated `warm_cache_worker` thread at [`base_coder.py#L1360-L1389`](https://github.com/Aider-AI/aider/blob/3ec8ec5a7d695b08a6c24fe6c0c235c8f87df9af/aider/coders/base_coder.py#L1360-L1389) that sends small `max_tokens: 1` probe requests to keep the Anthropic prompt cache alive. Proxies MUST handle cache writes for these probes the same as real requests (or cost inflation occurs per [#46917](https://github.com/anthropics/claude-code/issues/46917)-style effects).
- `litellm.drop_params = True` set at [`llm.py`](https://github.com/Aider-AI/aider/blob/3ec8ec5a7d695b08a6c24fe6c0c235c8f87df9af/aider/llm.py) — LiteLLM will silently drop unsupported params. Proxy implementations should not rely on aider sending all canonical fields.

### 5.3 Response parsing
Delegates to LiteLLM's `CustomStreamWrapper`. aider reads `response.choices[0].message.content` for non-streaming, iterates streaming chunks for interactive mode. Reconstructs diffs from streamed content character-by-character.

### 5.4 Timeouts
Default `request_timeout = 600` seconds (10 min) at [`models.py#L28`](https://github.com/Aider-AI/aider/blob/3ec8ec5a7d695b08a6c24fe6c0c235c8f87df9af/aider/models.py#L28). Set via `kwargs["timeout"] = request_timeout` at [`models.py#L1014`](https://github.com/Aider-AI/aider/blob/3ec8ec5a7d695b08a6c24fe6c0c235c8f87df9af/aider/models.py#L1014). CLI flag: `--timeout` (sets `models.request_timeout`).

### 5.5 Retry behavior
`simple_send_with_retries` at [`models.py#L1032-L1080`](https://github.com/Aider-AI/aider/blob/3ec8ec5a7d695b08a6c24fe6c0c235c8f87df9af/aider/models.py#L1032-L1080):
- Starts `retry_delay = 0.125` seconds, doubles each attempt
- Stops when `retry_delay > RETRY_TIMEOUT = 60` ([models.py#L26](https://github.com/Aider-AI/aider/blob/3ec8ec5a7d695b08a6c24fe6c0c235c8f87df9af/aider/models.py#L26))
- Retries on `LiteLLMExceptions` with `retry: True` flag. See [`exceptions.py#L14-L58`](https://github.com/Aider-AI/aider/blob/3ec8ec5a7d695b08a6c24fe6c0c235c8f87df9af/aider/exceptions.py#L14-L58): retries `APIConnectionError`, `APIError`, `BadGatewayError`, `ContentPolicyViolationError`, `InternalServerError`, `InvalidRequestError`, `JSONSchemaValidationError`, `OpenAIError`, `RateLimitError`, `ServiceUnavailableError`, `UnprocessableEntityError`, `UnsupportedParamsError`, `Timeout`, `BudgetExceededError`, `AzureOpenAIError`, `APIResponseValidationError`, `RouterRateLimitError`
- Does NOT retry: `AuthenticationError`, `BadRequestError`, `ContextWindowExceededError`, `NotFoundError`, `PermissionDeniedError`

So ~6–7 attempts total over ~2 minutes.

### 5.6 Error shape
aider catches LiteLLM's translated exception hierarchy, surfaces `str(err)` to user and stops or retries per `exception_info`. Non-LiteLLM errors (e.g., `AttributeError`) silently return None and abort.

### 5.7 Session model
Full history replay. aider trims via `/clear` or auto-summarization. Has `check_tokens` at [`base_coder.py`](https://github.com/Aider-AI/aider/blob/3ec8ec5a7d695b08a6c24fe6c0c235c8f87df9af/aider/coders/base_coder.py) that warns if `input_tokens >= max_input_tokens` — but still sends request.

### 5.8 Proxy config surface
CLI flags → env vars (set during startup at [`main.py#L620-L625`](https://github.com/Aider-AI/aider/blob/3ec8ec5a7d695b08a6c24fe6c0c235c8f87df9af/aider/main.py#L620-L625)):
- `--openai-api-base` → `OPENAI_API_BASE`
- `--anthropic-api-key` → `ANTHROPIC_API_KEY`
- `--openai-api-key` → `OPENAI_API_KEY`
- `--set-env KEY=VALUE` — generic env var setter
- `--timeout SECONDS` — overrides `models.request_timeout`
- `.aider.conf.yml` can set these same keys

Env vars read by LiteLLM at runtime:
- `OPENAI_API_BASE`, `ANTHROPIC_API_BASE` / `ANTHROPIC_BASE_URL`, `OPENROUTER_API_BASE`, `AZURE_API_BASE`, `AWS_BEDROCK_BASE_URL`, etc.

**GitHub Copilot integration** at [`models.py#L1019-L1028`](https://github.com/Aider-AI/aider/blob/3ec8ec5a7d695b08a6c24fe6c0c235c8f87df9af/aider/models.py#L1019-L1028) adds `Editor-Version: aider/<version>` and `Copilot-Integration-Id: vscode-chat` headers when `GITHUB_COPILOT_TOKEN` is in env.

### 5.9 Known proxy quirks
- [#3218](https://github.com/Aider-AI/aider/issues/3218) — "Configuring LLMLite proxy" — ongoing thread on LiteLLM-proxy setup patterns (pre-built config templates needed).
- [#3323](https://github.com/Aider-AI/aider/issues/3323) — "LiteLLM and model aliasing" — users want per-model proxy URL override.
- [#3426](https://github.com/Aider-AI/aider/issues/3426) — "[feature request] changing api_base" — ability to swap `api_base` mid-session.
- [#3879](https://github.com/Aider-AI/aider/issues/3879) — "Overriding model with extra_params/extra_body not working" — custom fields dropped by `litellm.drop_params=True`.
- [#3916](https://github.com/Aider-AI/aider/issues/3916) — Cannot override `GEMINI_API_BASE`.
- [#2765](https://github.com/Aider-AI/aider/issues/2765) — OpenRouter config quirk — historical, now closed.
- [#3285](https://github.com/Aider-AI/aider/issues/3285) — 0 responses from aider with litellm-compat API (proxy emits non-standard response).
- [#1849](https://github.com/Aider-AI/aider/issues/1849) — "How to use Gemini with proxy url?" — documented working pattern.

**Implication for kiroxy:** Since aider uses LiteLLM, kiroxy must be LiteLLM-compatible at the wire. Specifically: accept `litellm`'s exact Anthropic request shape (which matches the published Anthropic API). For OpenAI-compat, accept `litellm`'s exact chat-completions body (including LiteLLM-added fields like `user`, `metadata`, `litellm_params`). Cache-warming probes (`max_tokens: 1`) need to return a valid `message_start` → `message_stop` sequence without erroring.

---

## 6. opencode (PRIMARY TARGET)

Source: `sst/opencode` at commit [`53a3f95088941aa0d3979af90b28b071c40fd866`](https://github.com/sst/opencode/tree/53a3f95088941aa0d3979af90b28b071c40fd866) (2026-05-12).

**Architecture:** opencode uses the Vercel [AI SDK](https://github.com/vercel/ai) (`ai` package + `@ai-sdk/*` provider adapters) to speak to every LLM. opencode itself calls `streamText(...)` from the AI SDK; each provider adapter translates to its provider's wire format.

### 6.1 Endpoint shape per provider

opencode maintains a provider registry at [`provider.ts`](https://github.com/sst/opencode/blob/53a3f95088941aa0d3979af90b28b071c40fd866/packages/opencode/src/provider/provider.ts). Bundled providers at [`provider.ts#L92-L110`](https://github.com/sst/opencode/blob/53a3f95088941aa0d3979af90b28b071c40fd866/packages/opencode/src/provider/provider.ts#L92-L110):

| npm package | Default endpoint |
| --- | --- |
| `@ai-sdk/anthropic` | `https://api.anthropic.com/v1` + `/messages` → `/v1/messages` |
| `@ai-sdk/openai` | `https://api.openai.com/v1` + `/chat/completions` → `/v1/chat/completions` |
| `@ai-sdk/openai-compatible` | User-configured `baseURL` + `/chat/completions` |
| `@ai-sdk/amazon-bedrock` | AWS Bedrock InvokeModel |
| `@ai-sdk/google`, `@ai-sdk/google-vertex`, `@ai-sdk/google-vertex/anthropic` | Google Gen AI / Vertex |
| `@openrouter/ai-sdk-provider` | OpenRouter |
| `@ai-sdk/xai`, `@ai-sdk/mistral`, `@ai-sdk/groq`, `@ai-sdk/deepinfra`, `@ai-sdk/cerebras`, `@ai-sdk/cohere` | Provider-native |

Critical: the AI SDK's Anthropic adapter hardcodes `${baseURL}/messages` at [`anthropic-language-model.ts#L810`](https://github.com/vercel/ai/blob/148f5fc3cf687ff205b2b001e1742880ccb8c5fc/packages/anthropic/src/anthropic-language-model.ts#L810). Default `baseURL = 'https://api.anthropic.com/v1'` at [`anthropic-provider.ts#L110`](https://github.com/vercel/ai/blob/148f5fc3cf687ff205b2b001e1742880ccb8c5fc/packages/anthropic/src/anthropic-provider.ts#L110). So the POST path resolved is `/v1/messages`. Any `baseURL` override MUST include the `/v1` suffix. A kiroxy published at `https://kiroxy.example.com` and set as `baseURL` would break — user must configure `baseURL: "https://kiroxy.example.com/v1"`.

OpenAI AI SDK: default `baseURL = 'https://api.openai.com/v1'` at [`openai-provider.ts#L166`](https://github.com/vercel/ai/blob/148f5fc3cf687ff205b2b001e1742880ccb8c5fc/packages/openai/src/openai-provider.ts#L166), hits `baseURL + '/chat/completions'` at [`openai-chat-language-model.ts#L351`](https://github.com/vercel/ai/blob/148f5fc3cf687ff205b2b001e1742880ccb8c5fc/packages/openai/src/chat/openai-chat-language-model.ts#L351).

### 6.2 Request body
Constructed by `streamText()` + provider adapter. opencode's call site at [`session/llm.ts#L337-L408`](https://github.com/sst/opencode/blob/53a3f95088941aa0d3979af90b28b071c40fd866/packages/opencode/src/session/llm.ts#L337-L408) includes:
- `messages: ModelMessage[]` (AI SDK normalized shape)
- `tools`, `toolChoice`, `activeTools`
- `temperature`, `topP`, `topK`, `maxOutputTokens`
- `providerOptions: ProviderTransform.providerOptions(...)` — provider-specific options (thinking, reasoningEffort)
- `headers` — includes `User-Agent: opencode/<version>`; for opencode-hosted providers, adds `x-opencode-project`, `x-opencode-session`, `x-opencode-request`, `x-opencode-client`; for third-party, adds `x-session-affinity: <sessionID>` and optional `x-parent-session-id`.
- `maxRetries: input.retries ?? 0` — **opencode disables AI SDK retry by default**. It uses its own retry wrapper.
- `abortSignal: input.abort`
- `experimental_repairToolCall` — auto-lowercases tool names + wraps invalid tool calls into an "invalid" tool

**Cache control placement** ([`transform.ts#L340-L385`](https://github.com/sst/opencode/blob/53a3f95088941aa0d3979af90b28b071c40fd866/packages/opencode/src/provider/transform.ts#L340-L385)):
- First 2 system messages and last 2 non-system messages get cache breakpoints
- For Anthropic / Bedrock: message-level (`providerOptions.anthropic.cacheControl = {type:"ephemeral"}`)
- For openai-compatible: content-level (`cache_control: {type:"ephemeral"}`)
- For Copilot: `copilot_cache_control: {type:"ephemeral"}`
- For OpenRouter: `providerOptions.openrouter.cacheControl`

### 6.3 Response parsing (AI SDK Anthropic adapter schema)
opencode delegates to `@ai-sdk/anthropic`. The adapter's zod schema at [`anthropic-api.ts#L968-L1440`](https://github.com/vercel/ai/blob/148f5fc3cf687ff205b2b001e1742880ccb8c5fc/packages/anthropic/src/anthropic-api.ts#L968-L1440) defines a discriminated union that REQUIRES:
- `message_start` with `message.id`, `message.model`, `message.usage.{input_tokens, cache_creation_input_tokens?, cache_read_input_tokens?}`, optional pre-populated `content` array for programmatic tool calling
- `content_block_start` with `index`, `content_block`
- `content_block_delta` with `index`, `delta` (type: `text_delta` | `thinking_delta` | `signature_delta` | `input_json_delta`)
- `content_block_stop` with `index`
- `message_delta` with `delta.stop_reason`, `delta.stop_sequence?`, `usage.output_tokens`
- `message_stop`
- `ping` — handled as no-op at [`anthropic-language-model.ts#L1545-L1547`](https://github.com/vercel/ai/blob/148f5fc3cf687ff205b2b001e1742880ccb8c5fc/packages/anthropic/src/anthropic-language-model.ts#L1545-L1547)
- `error` — stream is terminated with `AnthropicError`

**Strict**: the schema is a `z.discriminatedUnion('type', [...])`. Any unknown event type will throw. Missing required fields (e.g. `usage.input_tokens` on `message_start`) will throw.

For OpenAI: `@ai-sdk/openai` parses OpenAI-standard SSE, reads `delta.content`, `delta.tool_calls[index].{id,function.{name,arguments}}`, `finish_reason`, `usage` (when `stream_options.include_usage: true`). The AI SDK sets `includeUsage: true` automatically when `@ai-sdk/openai-compatible` is used ([`provider.ts#L1431-L1433`](https://github.com/sst/opencode/blob/53a3f95088941aa0d3979af90b28b071c40fd866/packages/opencode/src/provider/provider.ts#L1431-L1433)).

### 6.4 Timeouts
Per-provider config option at [`config/provider.ts#L87-L102`](https://github.com/sst/opencode/blob/53a3f95088941aa0d3979af90b28b071c40fd866/packages/opencode/src/config/provider.ts#L87-L102):
- `options.timeout` — default **300000 ms (5 min)**. `false` disables. Applied as `AbortSignal.timeout()`.
- `options.chunkTimeout` — per-SSE-chunk timeout (no default). If no chunk arrives within window, request is aborted via `wrapSSE` at [`provider.ts#L40-L82`](https://github.com/sst/opencode/blob/53a3f95088941aa0d3979af90b28b071c40fd866/packages/opencode/src/provider/provider.ts#L40-L82).

The Bun-specific `timeout: false` is set on `fetch()` at [`provider.ts#L1513`](https://github.com/sst/opencode/blob/53a3f95088941aa0d3979af90b28b071c40fd866/packages/opencode/src/provider/provider.ts#L1513) to disable Bun's default timeout, deferring to the AbortSignal.

### 6.5 Retry behavior
`maxRetries: input.retries ?? 0` at [`session/llm.ts#L390`](https://github.com/sst/opencode/blob/53a3f95088941aa0d3979af90b28b071c40fd866/packages/opencode/src/session/llm.ts#L390). AI SDK's default is 2, but opencode sets 0 to disable — it uses its own retry wrapper at [`session/retry.ts`](https://github.com/sst/opencode/blob/53a3f95088941aa0d3979af90b28b071c40fd866/packages/opencode/src/session/retry.ts).

### 6.6 Error shape
Errors translated via [`provider/error.ts`](https://github.com/sst/opencode/blob/53a3f95088941aa0d3979af90b28b071c40fd866/packages/opencode/src/provider/error.ts) into opencode's `ProviderError` types. AI SDK surfaces `APICallError`, `InvalidResponseDataError`. A proxy returning non-standard Anthropic error JSON will cause the AI SDK to throw `InvalidResponseDataError` because its zod schemas are strict.

### 6.7 Session model
Full history replay. opencode loads conversation from `.opencode/sessions` SQLite-backed store ([`session/session.sql.ts`](https://github.com/sst/opencode/blob/53a3f95088941aa0d3979af90b28b071c40fd866/packages/opencode/src/session/session.sql.ts)) and rebuilds the `messages` array on each turn. The `x-session-affinity` header is informational — the upstream does not persist state.

### 6.8 Proxy config surface

User config file: `opencode.json`, `opencode.jsonc` (project root). Schema at [`config/provider.ts`](https://github.com/sst/opencode/blob/53a3f95088941aa0d3979af90b28b071c40fd866/packages/opencode/src/config/provider.ts).

Minimal proxy config (documented in [`providers.mdx#L34-L48`](https://github.com/sst/opencode/blob/53a3f95088941aa0d3979af90b28b071c40fd866/packages/web/src/content/docs/providers.mdx#L34-L48)):

```json
{
  "$schema": "https://opencode.ai/config.json",
  "provider": {
    "anthropic": {
      "options": {
        "baseURL": "https://kiroxy.example.com/v1",
        "apiKey": "sk-xxxxx",
        "timeout": 300000,
        "chunkTimeout": 60000,
        "headers": { "X-Custom": "value" }
      }
    }
  }
}
```

Custom provider (different npm package, e.g. openai-compat):
```json
{
  "provider": {
    "my-proxy": {
      "npm": "@ai-sdk/openai-compatible",
      "name": "kiroxy (OpenAI-compat)",
      "options": {
        "baseURL": "https://kiroxy.example.com/v1",
        "apiKey": "sk-xxxxx"
      },
      "models": {
        "claude-sonnet-4-5": { "name": "Claude Sonnet 4.5 via kiroxy" }
      }
    }
  }
}
```

Env vars (evaluated via `${VAR}` in config values at [`provider.ts#L1446-L1451`](https://github.com/sst/opencode/blob/53a3f95088941aa0d3979af90b28b071c40fd866/packages/opencode/src/provider/provider.ts#L1446-L1451)):
- `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `ANTHROPIC_BASE_URL` (via AI SDK adapter fallbacks at [`anthropic-provider.ts#L104-L110`](https://github.com/vercel/ai/blob/148f5fc3cf687ff205b2b001e1742880ccb8c5fc/packages/anthropic/src/anthropic-provider.ts#L104-L110))
- Provider auth credentials stored in `~/.local/share/opencode/auth.json` (set via `/connect` CLI command)

Per-provider autoload behavior: when `baseURL` is set, provider is considered "already configured" and autoload checks are skipped ([`provider.ts#L693-L697`](https://github.com/sst/opencode/blob/53a3f95088941aa0d3979af90b28b071c40fd866/packages/opencode/src/provider/provider.ts#L693-L697), comment: "When baseURL is already configured (e.g. corporate config routing through a proxy/gateway), skip").

### 6.9 models.dev integration

Per-provider / per-model metadata (ID, context window, costs, capabilities) comes from [models.dev](https://models.dev). opencode fetches and caches this. User config can override via `provider.<id>.models.<id>` block ([`config/provider.ts#L5-L65`](https://github.com/sst/opencode/blob/53a3f95088941aa0d3979af90b28b071c40fd866/packages/opencode/src/config/provider.ts#L5-L65)).

The `model.api.url` field is the canonical endpoint (from models.dev). opencode's resolution order at [`provider.ts#L1437`](https://github.com/sst/opencode/blob/53a3f95088941aa0d3979af90b28b071c40fd866/packages/opencode/src/provider/provider.ts#L1437):
```
options.baseURL (user config) > model.api.url (models.dev)
```

### 6.10 Known proxy quirks

The opencode issue tracker's search is rate-limited for "baseURL" queries, but from code/docs analysis:
- **Trailing-slash sensitivity**: Because `@ai-sdk/anthropic` does `${baseURL}/messages` and strips trailing slashes with `withoutTrailingSlash()`, a baseURL ending in `/v1/` becomes `/v1` → `/v1/messages`. Proxies must handle both normalizations.
- **Strict Anthropic schema**: `@ai-sdk/anthropic` uses zod discriminated union (`anthropicChunkSchema`). Unknown event types throw. Proxies MUST NOT invent new event types or omit required fields.
- **`includeUsage` auto-injection** for `@ai-sdk/openai-compatible` providers ([`provider.ts#L1431-L1433`](https://github.com/sst/opencode/blob/53a3f95088941aa0d3979af90b28b071c40fd866/packages/opencode/src/provider/provider.ts#L1431-L1433)) — proxy should support `stream_options: {include_usage: true}`.
- **OpenAI-specific body mutation** at [`provider.ts#L1490-L1506`](https://github.com/sst/opencode/blob/53a3f95088941aa0d3979af90b28b071c40fd866/packages/opencode/src/provider/provider.ts#L1490-L1506): strips `id` field from `input[]` items for @ai-sdk/openai (Responses API) unless `store: true`. Proxies mimicking Responses API must tolerate missing IDs.
- **chunk-timeout behavior**: SSE wrapper aborts and emits "SSE read timed out" if no data arrives within `chunkTimeout`. Proxies serving long-running tool calls or slow upstreams should emit `ping` events or `:` SSE comments to keep the stream alive.
- **User-Agent**: `opencode/<version>` is set — proxies that rate-limit by UA should recognize this.

### 6.11 SSE event tolerance

From AI SDK anthropic adapter ([`anthropic-language-model.ts#L1540-L1575`](https://github.com/vercel/ai/blob/148f5fc3cf687ff205b2b001e1742880ccb8c5fc/packages/anthropic/src/anthropic-language-model.ts#L1540-L1575)):
- `ping`: no-op (tolerated)
- `error`: terminates stream with parsed error
- Missing `ping` events: tolerated
- Out-of-order `content_block_stop` before `content_block_start`: likely breaks (state machine assumes ordering)
- Missing `message_stop`: stream hangs until `chunkTimeout` or connection close
- `message_start.usage.input_tokens` is **required** (z.number()) — proxies MUST emit this
- `message_delta.usage.output_tokens` is **required**

**Implication for kiroxy's opencode compatibility (CRITICAL):**
1. MUST serve `/v1/messages` with byte-exact AI SDK-compliant Anthropic SSE.
2. MUST include `input_tokens` and `output_tokens` in `message_start` / `message_delta` respectively, or the zod parse throws.
3. Should emit `ping` events periodically (every ~15–20s) to avoid `chunkTimeout` aborts.
4. Accept both `x-api-key` and `Authorization: Bearer` (AI SDK sends `x-api-key` by default).
5. Honor `anthropic-version: 2023-06-01` header.
6. Support `providerOptions.anthropic.cacheControl = {type:"ephemeral"}` translation to Anthropic `cache_control` blocks.
7. For OpenAI-compat: emit `stream_options: {include_usage: true}` response format and `data: [DONE]\n\n` terminator.


---

## 7. Zed

Source: `zed-industries/zed` at commit [`917c6984834a2b6c08fd169520393b5187e33dda`](https://github.com/zed-industries/zed/tree/917c6984834a2b6c08fd169520393b5187e33dda) (2026-05-12).

### 7.1 Endpoint shape
Zed has dedicated Rust crates per provider:

| Crate | Protocol | Default URL |
| --- | --- | --- |
| [`anthropic`](https://github.com/zed-industries/zed/blob/917c6984834a2b6c08fd169520393b5187e33dda/crates/anthropic/src/anthropic.rs) | Anthropic-native | `https://api.anthropic.com` (at [L17](https://github.com/zed-industries/zed/blob/917c6984834a2b6c08fd169520393b5187e33dda/crates/anthropic/src/anthropic.rs#L17)) |
| [`open_ai`](https://github.com/zed-industries/zed/blob/917c6984834a2b6c08fd169520393b5187e33dda/crates/open_ai/src/open_ai.rs) | OpenAI-compat | `https://api.openai.com/v1` (at [L18](https://github.com/zed-industries/zed/blob/917c6984834a2b6c08fd169520393b5187e33dda/crates/open_ai/src/open_ai.rs#L18)) |
| `language_models_cloud` | Zed Pro (proprietary) | Zed cloud |

Anthropic POST URL: `format!("{api_url}/v1/messages")` at [anthropic.rs#L304](https://github.com/zed-industries/zed/blob/917c6984834a2b6c08fd169520393b5187e33dda/crates/anthropic/src/anthropic.rs#L304). User-configured `api_url` in `settings.json` is used raw — Zed appends `/v1/messages` itself.

OpenAI POST URL: `format!("{api_url}/chat/completions")` at [open_ai.rs#L674](https://github.com/zed-industries/zed/blob/917c6984834a2b6c08fd169520393b5187e33dda/crates/open_ai/src/open_ai.rs#L674). User-configured `api_url` in `settings.json` must end with `/v1`.

### 7.2 Request body
Anthropic request at [`anthropic.rs#L298-L330`](https://github.com/zed-industries/zed/blob/917c6984834a2b6c08fd169520393b5187e33dda/crates/anthropic/src/anthropic.rs#L298-L330):
- Method: POST
- Headers: `Anthropic-Version: 2023-06-01`, `X-Api-Key: <key>`, `Content-Type: application/json`, optional `Anthropic-Beta: <csv>`
- Body: serde-serialized `Request` struct with `messages`, `system`, `tools`, `thinking`, `cache_control` fields.
- `cache_control` enum: only `Ephemeral` variant at [`anthropic.rs#L502-L512`](https://github.com/zed-industries/zed/blob/917c6984834a2b6c08fd169520393b5187e33dda/crates/anthropic/src/anthropic.rs#L502-L512) — applied to only the last segment ([completion.rs#L148-L165](https://github.com/zed-industries/zed/blob/917c6984834a2b6c08fd169520393b5187e33dda/crates/anthropic/src/completion.rs#L148-L165)).

### 7.3 Response parsing
Anthropic `Event` enum at [`anthropic.rs#L747-L770`](https://github.com/zed-industries/zed/blob/917c6984834a2b6c08fd169520393b5187e33dda/crates/anthropic/src/anthropic.rs#L747-L770):
```rust
#[serde(tag = "type")]
pub enum Event {
    MessageStart { message: Response },
    ContentBlockStart { index: usize, content_block: ResponseContent },
    ContentBlockDelta { index: usize, delta: ContentDelta },
    ContentBlockStop { index: usize },
    MessageDelta { delta: MessageDelta, usage: Usage },
    MessageStop,
    Ping,
    Error { error: ApiError },
}
```
Uses serde untagged enum for content deltas: `TextDelta`, `ThinkingDelta`, `SignatureDelta`, `InputJsonDelta`. Non-matching event types fail serde parse → `AnthropicError::DeserializeResponse`.

OpenAI SSE at [`open_ai.rs#L742-L753`](https://github.com/zed-industries/zed/blob/917c6984834a2b6c08fd169520393b5187e33dda/crates/open_ai/src/open_ai.rs#L742-L753): strips `data: ` or `data:` prefix, stops on `[DONE]`, otherwise parses `ResponseStreamResult` (Ok/Err union).

### 7.4 Timeouts
Relies on `reqwest` default behavior via Zed's `http_client::HttpClient` trait. No explicit timeout in `anthropic.rs` or `open_ai.rs`. Zed's HTTP client configures read/connect timeouts elsewhere — not user-visible in AI settings.

Rate-limit info is extracted from response headers and stored in `RateLimitInfo` ([anthropic.rs#L328](https://github.com/zed-industries/zed/blob/917c6984834a2b6c08fd169520393b5187e33dda/crates/anthropic/src/anthropic.rs#L328)).

### 7.5 Retry behavior
Provider-level: Zed surfaces errors including `AnthropicError::ServerOverloaded { retry_after }` (HTTP 529) and `AnthropicError::RateLimit { retry_after }` (429). Retries are handled at the **model-request layer** (`language_model` crate), not at the HTTP layer — the assistant thread may retry depending on error type but there's no simple retry loop inside the anthropic crate.

### 7.6 Error shape
`ApiError` at [anthropic.rs#L824-L831](https://github.com/zed-industries/zed/blob/917c6984834a2b6c08fd169520393b5187e33dda/crates/anthropic/src/anthropic.rs#L824-L831) deserializes `{"type": "<error_type>", "message": "<msg>"}` from the `error` field of the Anthropic response. HTTP 529 → `ServerOverloaded`. HTTP 429 with retry-after header → `RateLimit`. Other non-2xx → `HttpResponseError { status_code, message }`.

### 7.7 Session model
Full history replay. Thread model in `assistant` / `assistant2` crates maintains conversation state client-side. No conversation ID header.

### 7.8 Proxy config surface
Per-provider settings at [`settings.json`](https://zed.dev/docs/configuring-zed#language-models):

```json
{
  "language_models": {
    "anthropic": {
      "api_url": "https://kiroxy.example.com",
      "available_models": [
        { "name": "claude-sonnet-4-5", "max_tokens": 200000, "max_output_tokens": 8192 }
      ]
    },
    "openai": {
      "api_url": "https://kiroxy.example.com/v1",
      "available_models": [ ... ]
    }
  }
}
```

API keys are stored via `credentials_provider` (OS keychain). Env var: `ANTHROPIC_API_KEY` (recognized at [`anthropic.rs#L42`](https://github.com/zed-industries/zed/blob/917c6984834a2b6c08fd169520393b5187e33dda/crates/language_models/src/provider/anthropic.rs#L42)).

**CRITICAL ZED LIMITATION** (from closed issue [#54764](https://github.com/zed-industries/zed/issues/54764)):
> ANTHROPIC_API_KEY env var is recognized, but **ANTHROPIC_BASE_URL is NOT**. To set a custom Anthropic endpoint, users must edit `settings.json` `language_models.anthropic.api_url` — no env var parity.

### 7.9 Known proxy quirks
- [#54764](https://github.com/zed-industries/zed/issues/54764) — "Support ANTHROPIC_BASE_URL environment variable and enable native Anthropic protocol for custom endpoints" (closed, not merged — pointed to discussions). Users forced to choose between Anthropic-native via `api_url` (settings-only) OR OpenAI-compat via `openai_compatible` provider (which loses thinking, prompt caching, adaptive thinking, beta headers).
- [#47559](https://github.com/zed-industries/zed/issues/47559) — `openai_compatible` doesn't support `<think>` tags (no reasoning_content field translation).
- [#52567](https://github.com/zed-industries/zed/issues/52567) — Inline assistant regression for OpenAI-compat providers after v0.228.0.
- [#53135](https://github.com/zed-industries/zed/issues/53135) — Inline assistant can't handle thinking from openai-compatible provider.
- [#55407](https://github.com/zed-industries/zed/issues/55407) — "New from Summary" fails with Omniroute API: BadRequestFormat error.
- [#41897](https://github.com/zed-industries/zed/issues/41897) — AI: Issues while using MiniMax M2 (area:ai/anthropic).
- [#51180](https://github.com/zed-industries/zed/issues/51180) — Agent infinite repetition loop with kimi-k2p5 on Fireworks/OpenRouter.
- [#29965](https://github.com/zed-industries/zed/issues/29965) — "Custom anthropic model settings not work" (closed).

**Implication for kiroxy:** Zed users configure `api_url` in `settings.json`, not an env var. kiroxy docs for Zed users must provide exact JSON snippet. For Zed's Anthropic crate, the URL must be a base (no path) because Zed appends `/v1/messages` itself. For Zed's OpenAI crate, URL must end with `/v1`. serde-strict parsing means kiroxy's SSE events must exactly match the `Event` enum discriminator (`type` field values). Do not invent new event types.

---

## 8. LiteLLM

Source: `BerriAI/litellm` at commit [`fc8a9a34067bb1571bb02bf6b9dc308f89ba168e`](https://github.com/BerriAI/litellm/tree/fc8a9a34067bb1571bb02bf6b9dc308f89ba168e) (2026-05-12).

LiteLLM is BOTH a Python library (used by aider) AND a proxy server (used by teams as an LLM gateway). For kiroxy, LiteLLM's architecture is the best reference because it solves the same problem: translate between Anthropic and OpenAI formats while proxying to multiple upstreams.

### PART A: LiteLLM as a CLIENT (what it sends to upstream providers)

#### 8.A.1 Endpoint shape
Anthropic client flow in [`main.py#L2913-L2930`](https://github.com/BerriAI/litellm/blob/fc8a9a34067bb1571bb02bf6b9dc308f89ba168e/litellm/main.py#L2913-L2930):
```python
api_base = (
    api_base
    or litellm.api_base
    or get_secret("ANTHROPIC_API_BASE")
    or get_secret("ANTHROPIC_BASE_URL")
    or "https://api.anthropic.com/v1/messages"
)

disable_url_suffix = get_secret_bool("LITELLM_ANTHROPIC_DISABLE_URL_SUFFIX")
if api_base is not None and not disable_url_suffix and not api_base.endswith("/v1/messages"):
    api_base += "/v1/messages"
```

Note: LiteLLM auto-appends `/v1/messages` to user-supplied `api_base` unless `LITELLM_ANTHROPIC_DISABLE_URL_SUFFIX=1`. This is the opposite of Zed (which appends `/v1/messages` internally) but similar to the AI SDK (which appends `/messages`). Means kiroxy should accept ALL shapes for Anthropic: `{base}`, `{base}/`, `{base}/v1`, `{base}/v1/`, `{base}/v1/messages`.

Default Anthropic URL resolver at [`common_utils.py#L579-L589`](https://github.com/BerriAI/litellm/blob/fc8a9a34067bb1571bb02bf6b9dc308f89ba168e/litellm/llms/anthropic/common_utils.py#L579-L589):
```python
return (
    api_base
    or get_secret_str("ANTHROPIC_API_BASE")
    or get_secret_str("ANTHROPIC_BASE_URL")
    or "https://api.anthropic.com"
)
```

#### 8.A.2 Request body
LiteLLM's `AnthropicConfig.validate_environment()` at [`transformation.py`](https://github.com/BerriAI/litellm/blob/fc8a9a34067bb1571bb02bf6b9dc308f89ba168e/litellm/llms/anthropic/chat/transformation.py) builds the Anthropic message body. Key behaviors:
- `cache_control: {"type":"ephemeral"}` is injected automatically when `cache_control` is used in messages ([transformation.py#L591](https://github.com/BerriAI/litellm/blob/fc8a9a34067bb1571bb02bf6b9dc308f89ba168e/litellm/llms/anthropic/chat/transformation.py#L591))
- Tool format translation: OpenAI function → Anthropic tools
- `x-anthropic-billing-header` system message content is stripped ([transformation.py#L1601-L1659](https://github.com/BerriAI/litellm/blob/fc8a9a34067bb1571bb02bf6b9dc308f89ba168e/litellm/llms/anthropic/chat/transformation.py#L1601-L1659))

#### 8.A.3 Auth
From [`common_utils.py#L607-L622`](https://github.com/BerriAI/litellm/blob/fc8a9a34067bb1571bb02bf6b9dc308f89ba168e/litellm/llms/anthropic/common_utils.py#L607-L622):
- Checks `ANTHROPIC_API_KEY` → sends as `x-api-key` header
- Falls back to `ANTHROPIC_AUTH_TOKEN` → sends as `Authorization: Bearer`
- Special-cases `is_anthropic_oauth_key(key)` → uses `authorization: Bearer` header

#### 8.A.4 Timeouts
Default at [`constants.py#L437-L450`](https://github.com/BerriAI/litellm/blob/fc8a9a34067bb1571bb02bf6b9dc308f89ba168e/litellm/constants.py#L437-L450):
```python
DEFAULT_REQUEST_TIMEOUT_SECONDS: float = 6000.0
COMPLETION_HTTP_FALLBACK_SECONDS: float = 600.0
HTTP_HANDLER_CONNECT_TIMEOUT_SECONDS: float = 5.0
request_timeout: float = float(os.getenv("REQUEST_TIMEOUT", "6000"))
```
So LiteLLM default: 5s connect, 600s per-completion, with the sentinel 6000s for longer-running surfaces (Router, speech, responses, vector stores).

#### 8.A.5 Retry behavior
`litellm.num_retries` (default 3) wraps all calls via tenacity ([main.py#L4522-L4540](https://github.com/BerriAI/litellm/blob/fc8a9a34067bb1571bb02bf6b9dc308f89ba168e/litellm/main.py#L4522-L4540)). Uses exponential backoff. Applied per request unless `kwargs["num_retries"] = 0`.

#### 8.A.6 Error translation
LiteLLM maps upstream errors to its own `litellm.exceptions` classes: `APIConnectionError`, `APIError`, `AuthenticationError`, `BadRequestError`, `RateLimitError`, `ServiceUnavailableError`, `Timeout`, `InternalServerError`, `ContextWindowExceededError`, `ContentPolicyViolationError`, etc. (consumed by aider's `EXCEPTIONS` list).

### PART B: LiteLLM as a SERVER (endpoints it exposes for clients)

#### 8.B.7 Endpoints
Core chat endpoints at [`proxy_server.py#L7867-L7883`](https://github.com/BerriAI/litellm/blob/fc8a9a34067bb1571bb02bf6b9dc308f89ba168e/litellm/proxy/proxy_server.py#L7867-L7883):
```python
@router.post("/v1/chat/completions", ...)
@router.post("/chat/completions", ...)
@router.post("/engines/{model:path}/chat/completions", ...)
@router.post("/openai/deployments/{model:path}/chat/completions", ...)
async def chat_completion(...): ...
```

Anthropic `/v1/messages` endpoint at [`anthropic_endpoints/endpoints.py#L22-L27`](https://github.com/BerriAI/litellm/blob/fc8a9a34067bb1571bb02bf6b9dc308f89ba168e/litellm/proxy/anthropic_endpoints/endpoints.py#L22-L27):
```python
@router.post(
    "/v1/messages",
    tags=["[beta] Anthropic `/v1/messages`"],
    dependencies=[Depends(user_api_key_auth)],
)
async def anthropic_response(...)
```

The handler notes: _"Use `{PROXY_BASE_URL}/anthropic/v1/messages` instead — this was a BETA endpoint that calls 100+ LLMs in the anthropic format."_ — so there are TWO ways to hit Anthropic-format endpoints on LiteLLM:
1. **Unified endpoint**: `POST /v1/messages` (beta, any model, cost tracking consistent)
2. **Pass-through endpoint**: `POST /anthropic/v1/messages` (routes directly to Anthropic upstream)

Claude Code docs ([llm-gateway.mdx](https://docs.claude.com/en/docs/claude-code/llm-gateway#unified-endpoint-recommended)) recommend the unified endpoint for load balancing, fallbacks, and end-user tracking.

#### 8.B.8 /v1/messages translation layer
- If upstream model is Anthropic: passthrough (no translation)
- If upstream model is OpenAI/Gemini/etc.: translate Anthropic request → OpenAI format, make upstream call, translate OpenAI response → Anthropic format. Translation logic in [`experimental_pass_through/adapters/`](https://github.com/BerriAI/litellm/tree/fc8a9a34067bb1571bb02bf6b9dc308f89ba168e/litellm/llms/anthropic/experimental_pass_through/adapters).
- Reverse: /v1/chat/completions can route to Anthropic model, translates to /v1/messages upstream.

#### 8.B.9 SSE emission
LiteLLM re-emits SSE events. For streaming passthrough, events are relayed with minimal modification ([streaming_iterator.py#L50](https://github.com/BerriAI/litellm/blob/fc8a9a34067bb1571bb02bf6b9dc308f89ba168e/litellm/llms/anthropic/experimental_pass_through/messages/streaming_iterator.py#L50)). For cross-format translation, LiteLLM synthesizes `message_start` / `content_block_*` / `message_stop` events from OpenAI deltas.

#### 8.B.10 Model routing (`config.yaml`)
Schema example:
```yaml
model_list:
  - model_name: claude-sonnet-4-5
    litellm_params:
      model: anthropic/claude-sonnet-4-5
      api_key: os.environ/ANTHROPIC_API_KEY
      api_base: https://api.anthropic.com   # optional override
  - model_name: claude-via-openrouter
    litellm_params:
      model: openrouter/anthropic/claude-sonnet-4.5
      api_key: os.environ/OPENROUTER_API_KEY
```

Supports per-model `api_base`, `api_key_env_var`, `aws_region_name`, `aws_profile`, etc.

#### 8.B.11 Known LiteLLM compat issues
- [#26749](https://github.com/BerriAI/litellm/issues/26749) — Anthropic passthrough: `server_tool_use` parsed as dict instead of `ServerToolUse`.
- [#25250](https://github.com/BerriAI/litellm/issues/25250) — Prometheus metrics emit model name without provider prefix.
- [#19828](https://github.com/BerriAI/litellm/issues/19828) — Feature: add Prometheus metrics for Anthropic passthrough.
- [#17476](https://github.com/BerriAI/litellm/issues/17476) — Pass-through streaming Langfuse callback fails with `complete_streaming_response: None`.
- [#27038](https://github.com/BerriAI/litellm/issues/27038) — `disable_end_user_cost_tracking` doesn't gate SpendLogs/DailyEndUserSpend writes.
- **Security incident**: [#24518](https://github.com/BerriAI/litellm/issues/24518) — LiteLLM PyPI versions 1.82.7 and 1.82.8 were compromised with credential-stealing malware. Anthropic's official docs warn about this. kiroxy should NOT depend on LiteLLM as a library without supply-chain review.

#### 8.B.12 Architecture lessons for kiroxy
- **Separate endpoint groups**: LiteLLM uses separate FastAPI routers per API family (`/v1/chat/completions` in `proxy_server.py`, `/v1/messages` in `anthropic_endpoints/endpoints.py`). kiroxy should mirror this split.
- **`get_complete_url()` pattern**: LiteLLM's per-provider transformation classes expose a `get_complete_url(api_base, ...)` method. kiroxy should centralize URL composition to avoid double-`/v1` bugs like Cursor users hit.
- **Auth header multiplexing**: Support `x-api-key`, `Authorization: Bearer`, and `apiKey: ...` parameter-based auth simultaneously.
- **Passthrough vs translation vs unified**: LiteLLM offers all three modes (pass-through, adapted, unified). kiroxy starts with passthrough + translation; unified can come later.
- **Beta header forwarding**: `anthropic-beta` and `anthropic-version` are forwarded unchanged. kiroxy MUST do this or Claude Code extended thinking breaks ([LLM gateway req](https://docs.claude.com/en/docs/claude-code/llm-gateway#api-format)).
- **Attribution header stripping**: LiteLLM doesn't strip — respect the client's `CLAUDE_CODE_ATTRIBUTION_HEADER=0` preference instead.


---

## Synthesis

### Anthropic-compat clients (expect `/v1/messages` endpoint)
- **claude-code** — PRIMARY (native, all features). Auth: `x-api-key` or `Authorization: Bearer`. Env: `ANTHROPIC_BASE_URL`.
- **Cline** (when "Anthropic" provider selected) — via `@anthropic-ai/sdk`. UI config: `anthropicBaseUrl`.
- **Continue** (when "anthropic" provider configured) — direct `fetch` to `{apiBase}messages`. Requires trailing slash on `apiBase`.
- **Zed** (Anthropic provider) — direct reqwest to `{api_url}/v1/messages`. Config: `settings.json` `language_models.anthropic.api_url`. NO env var support.
- **opencode** (via `@ai-sdk/anthropic`) — AI SDK. Config: `opencode.json` `provider.anthropic.options.baseURL`.
- **aider** (model prefix `anthropic/...`) — via LiteLLM. Env: `ANTHROPIC_API_BASE` / `ANTHROPIC_BASE_URL`.
- **LiteLLM** (as upstream client) — URL composition adds `/v1/messages` unless `LITELLM_ANTHROPIC_DISABLE_URL_SUFFIX=1`.

### OpenAI-compat clients (expect `/v1/chat/completions` endpoint)
- **Cursor** — OPENAI-ONLY. No Anthropic-native path. All Claude routes must translate to OpenAI-format at proxy.
- **Cline** ("OpenAI Compatible" provider) — via `openai` SDK. UI: `openAiBaseUrl`.
- **Continue** ("openai" provider) — direct `fetch` to `{apiBase}chat/completions`.
- **Zed** (OpenAI provider / `openai_compatible`) — direct reqwest to `{api_url}/chat/completions`. Loses Anthropic features (thinking, caching).
- **opencode** (via `@ai-sdk/openai` or `@ai-sdk/openai-compatible`) — AI SDK adapter.
- **aider** (model prefix `openai/...`) — via LiteLLM.
- **LiteLLM** (as upstream + as server) — primary interop format.

### Which clients require SSE vs accept JSON
**SSE (streaming) required by default:**
- claude-code — always streams
- Cline — `stream: true` hardcoded in Anthropic provider
- opencode — `streamText()` always streams
- Zed — always streams
- Cursor — always streams (composer + chat)

**Both supported (non-streaming for one-shot, streaming for interactive):**
- Continue — `options.stream ?? true` default, but honors `false`
- aider — `--stream` flag controls (default streams in interactive, non-streams for `simple_send_with_retries`)
- LiteLLM — both endpoints honor `stream: true|false`

**kiroxy MUST support streaming SSE as the default transport.** A stream-only implementation will work for all 7 clients; a JSON-only implementation will work for none in their default configuration.

### Timeout defaults (seconds unless noted)

| Client | Default | Configurable | Location |
| --- | --- | --- | --- |
| claude-code | 600s | `API_TIMEOUT_MS` env var | [docs](https://docs.claude.com/en/docs/claude-code/env-vars) |
| Cursor | ~5 min (community-reported) | No user-facing setting | [ofox.ai 2026](https://ofox.ai/blog/cursor-claude-code-cline-custom-api-setup-2026/) |
| Cline | ~10 min (SDK default) | No user-facing setting | `@anthropic-ai/sdk` default |
| Continue | 7200s (2 hr) | `requestOptions.timeout` | [`getAgentOptions.ts`](https://github.com/continuedev/continue/blob/cb273098d968906d25ee737b454f0b5f13ea2482/packages/fetch/src/getAgentOptions.ts#L12) |
| aider | 600s | `--timeout` CLI flag | [`models.py#L28`](https://github.com/Aider-AI/aider/blob/3ec8ec5a7d695b08a6c24fe6c0c235c8f87df9af/aider/models.py#L28) |
| opencode | 300s | `options.timeout` in opencode.json | [`config/provider.ts#L93`](https://github.com/sst/opencode/blob/53a3f95088941aa0d3979af90b28b071c40fd866/packages/opencode/src/config/provider.ts#L93) |
| Zed | reqwest default (~30s idle) | Not user-configurable | |
| LiteLLM | 600s completion / 6000s sentinel | `litellm.request_timeout` or env `REQUEST_TIMEOUT` | [`constants.py#L437`](https://github.com/BerriAI/litellm/blob/fc8a9a34067bb1571bb02bf6b9dc308f89ba168e/litellm/constants.py#L437) |

**kiroxy recommendation**: Set read timeout ≥ 600s. Emit keep-alive SSE pings (`: keepalive\n\n` or `event: ping\ndata: {}\n\n`) every 15–20s to prevent mid-stream client timeouts.

### Retry defaults

| Client | Max attempts | Which errors | Honors `Retry-After`? |
| --- | --- | --- | --- |
| claude-code | Unknown (closed-source) | 429, 5xx, transient | Yes |
| Cursor | Unknown | Unknown | Unknown |
| Cline | 3 | 429, `RetriableError` only | Yes (`retry-after`, `x-ratelimit-reset`, `ratelimit-reset`) |
| Continue | 3 (opt-in via decorator) | 429, 5xx, network, AWS throttling | Yes (`retry-after`, `x-ratelimit-reset`, `ratelimit-reset`, HTTP-date) |
| aider | ~6-7 (exp. doubling from 0.125s to 60s) | `LiteLLMExceptions` retry-flagged | Yes (via LiteLLM) |
| opencode | 0 (AI SDK retries disabled) | N/A | opencode's own retry logic |
| Zed | 0 in HTTP layer (thread-level may retry) | N/A | Yes (emits `retry_after` in error) |
| LiteLLM | 3 (tenacity-wrapped) | APIConnection, APIError, RateLimit, etc. | Yes |

**kiroxy recommendation**: Return proper `Retry-After: <seconds>` or `Retry-After: <HTTP-date>` headers with 429s. Clients vary in header name — set ALL THREE: `Retry-After`, `x-ratelimit-reset`, `ratelimit-reset`.

### Auth header conventions

| Client | Primary | Fallback | Notes |
| --- | --- | --- | --- |
| claude-code | `Authorization: Bearer` (from `ANTHROPIC_AUTH_TOKEN`) | `x-api-key` (from `ANTHROPIC_API_KEY`) | `apiKeyHelper` script generates both |
| Cursor | `Authorization: Bearer` | — | "OpenAI API Key" field |
| Cline | `Authorization: Bearer` (OpenAI) / `x-api-key` (Anthropic) | — | Via SDK defaults |
| Continue | `Authorization: Bearer` (OpenAI) / `x-api-key` (Anthropic) | `requestOptions.headers` override | |
| aider | Via LiteLLM — depends on upstream provider | — | |
| opencode | `x-api-key` (Anthropic default AI SDK) / `Authorization: Bearer` (OpenAI) | `options.headers` override | |
| Zed | `X-Api-Key` | — | No bearer fallback in code |
| LiteLLM | Pass-through from upstream env vars | — | |

**kiroxy recommendation**: Accept BOTH `x-api-key: <key>` AND `Authorization: Bearer <key>` simultaneously on both `/v1/messages` and `/v1/chat/completions`. Map them internally to the upstream's expected scheme.

---

## Known pain points across ALL clients when using custom proxies

### 1. URL shape ambiguity
Every client has a different convention for when to include `/v1` in the base URL:
- **Zed Anthropic** — user sets base (no `/v1`), Zed appends `/v1/messages`
- **Zed OpenAI** — user sets `/v1` URL, Zed appends `/chat/completions`
- **Cursor** — user sets `/v1`, Cursor appends `/chat/completions`
- **AI SDK Anthropic (opencode)** — user sets URL ending in `/v1`, SDK appends `/messages`
- **LiteLLM Anthropic** — auto-appends `/v1/messages` unless suffix disabled
- **Continue** — uses `new URL("messages", apiBase)` — trailing slash REQUIRED
- **Aider/LiteLLM OpenAI** — `OPENAI_API_BASE` expected to end with `/v1`

**Mitigation for kiroxy:** Normalize internally. Accept all of `{base}`, `{base}/v1`, `{base}/v1/`, `{base}/v1/messages`, `{base}/messages`, and route to correct handler. Log a warning when normalization changes the URL. This was LiteLLM's approach and it avoids 80% of the "base_url" proxy support issues.

### 2. SSE chunk-timeout under slow tool-call generation
opencode's `chunkTimeout` will abort a stream if no SSE chunk arrives within the window. This bites when the upstream takes 30+ seconds to start emitting after a tool_use thinking block. Proxies that buffer server responses before flushing will trigger this.

**Mitigation for kiroxy:** Stream SSE with `Transfer-Encoding: chunked`, set `X-Accel-Buffering: no` and `Cache-Control: no-cache` headers. Emit `ping` events or SSE comments (`: keepalive\n\n`) every 15s for long-running requests.

### 3. Strict schema validation breaks on extra fields
`@ai-sdk/anthropic` (opencode), Zed's serde enums, and Cline's `@anthropic-ai/sdk` all use strict schema validation. If a proxy adds custom event types (e.g., `data: {"type":"proxy_info","source":"kiroxy"}`) the stream fails.

**Mitigation for kiroxy:** Use SSE `event: proxy_info` (named event) or `:` comments for out-of-band info. Do NOT emit unknown `type: "..."` values in the data payload.

### 4. Cache-control placement differences
- **Cline**: cache_control at system+tools level only (top-level)
- **opencode**: message-level for Anthropic, content-level for openai-compatible
- **Continue**: content-level with `addCacheControlToLastTwoUserMessages` helper
- **Zed**: content-level, only last segment
- **LiteLLM**: passes through user-supplied cache_control
- **aider**: via LiteLLM + cache-warming probe thread

**Mitigation for kiroxy:** Accept both placements in request body and normalize to the upstream's preferred style. Pass-through mode should forward unchanged.

### 5. Beta header forwarding
Each client may send different `anthropic-beta` values:
- Cline sends `fast-mode-2026-02-01` and `context-1m-2025-08-07`
- Claude Code sends `tool-use-2024-04-04` and model-specific betas
- opencode sends betas via AI SDK's `anthropic-beta` option

Proxies that don't forward `anthropic-beta` will silently break extended features.

**Mitigation for kiroxy:** Forward `anthropic-beta` and `anthropic-version` headers verbatim, as required by the Claude Code LLM gateway spec.

### 6. Prompt attribution / cache-key drift
Claude Code prepends a per-session attribution block to system prompts. This changes the cache key, reducing prompt-cache hit rate. Users can disable with `CLAUDE_CODE_ATTRIBUTION_HEADER=0`, but many don't know.

**Mitigation for kiroxy:** Don't strip anything silently. Offer an optional "normalize-system-prompt" mode that removes the attribution block, gated by explicit config flag. Document the trade-off.

### 7. Error JSON shape mismatch
- Anthropic: `{"type":"error","error":{"type":"<subtype>","message":"<msg>"}}`
- OpenAI: `{"error":{"message":"<msg>","type":"<type>","code":"<code>","param":"<param>"}}`

Some clients (Zed, Continue) parse one format only. If a proxy serves `/v1/messages` but returns OpenAI-shape errors on failure, the client surfaces generic "Connection error" with no actionable info.

**Mitigation for kiroxy:** Always return errors in the endpoint's native shape (`/v1/messages` → Anthropic-shape; `/v1/chat/completions` → OpenAI-shape) regardless of what the upstream actually returned.

### 8. Auth header variants (x-api-key vs Authorization)
Most clients default to one but a few use the other. LiteLLM and Claude Code support both simultaneously; others do not.

**Mitigation for kiroxy:** Accept both on both endpoints. Strip both before forwarding if using the proxy's own upstream auth.

### 9. Tool-name case sensitivity
opencode has an `experimental_repairToolCall` that lowercases tool names mid-stream ([`session/llm.ts#L342-L363`](https://github.com/sst/opencode/blob/53a3f95088941aa0d3979af90b28b071c40fd866/packages/opencode/src/session/llm.ts#L342-L363)). Some proxies transform names differently.

**Mitigation for kiroxy:** Preserve tool names exactly. Never case-fold or namespace-prefix in proxy mode.

### 10. `/v1/models` endpoint expectations
- Claude Code with `CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY=1` → queries `/v1/models`, filters to IDs starting with `claude` or `anthropic`
- Zed queries `/v1/models?limit=1000` at [anthropic.rs#L230](https://github.com/zed-industries/zed/blob/917c6984834a2b6c08fd169520393b5187e33dda/crates/anthropic/src/anthropic.rs#L230) for model discovery (if configured)
- Cursor and Cline have hardcoded model lists; don't hit `/v1/models`
- Continue, opencode, aider get model metadata from separate sources (models.dev, LiteLLM registry)

**Mitigation for kiroxy:** Implement `/v1/models` returning `{"data":[{"id":"<model>","object":"model","display_name":"..."}]}`. List only model IDs kiroxy actually handles. Filter correctly for Anthropic pattern.

---

## kiroxy implementation checklist (derived from above)

- [ ] Expose `/v1/messages` with full Anthropic SSE protocol (message_start, content_block_*, message_delta, message_stop, ping, error)
- [ ] Expose `/v1/chat/completions` with full OpenAI SSE protocol (data/[DONE]/delta.content/delta.tool_calls)
- [ ] Expose `/v1/models` with both Anthropic-filter (claude/anthropic prefix) and OpenAI-filter support
- [ ] Accept `x-api-key` AND `Authorization: Bearer` on both endpoints
- [ ] Forward `anthropic-version` and `anthropic-beta` headers unchanged
- [ ] Normalize base URL variants internally (`{base}`, `{base}/v1`, `{base}/v1/messages`) to one canonical form
- [ ] Default read timeout ≥ 600s, enable chunked/unbuffered SSE
- [ ] Emit keepalive `ping` events every 15s during long thinking blocks
- [ ] Return `Retry-After` + `x-ratelimit-reset` + `ratelimit-reset` on 429s
- [ ] Never invent new SSE `type` values (use named `event:` or `:` comments for out-of-band data)
- [ ] Preserve tool names and cache_control placement (passthrough by default, transform only on explicit config)
- [ ] Match error-JSON shape to endpoint (Anthropic-shape on /v1/messages, OpenAI-shape on /v1/chat/completions)
- [ ] Emit usage fields: `input_tokens`, `output_tokens`, `cache_creation_input_tokens`, `cache_read_input_tokens` (Anthropic) / `prompt_tokens`, `completion_tokens`, `prompt_tokens_details.cached_tokens` (OpenAI)
- [ ] Set `includeUsage: true` behavior for OpenAI-compat → always include usage in final chunk
- [ ] Document explicit URL shape for each client in kiroxy docs (a per-client snippet table)

---

## Appendix: Source SHAs pinned

| Client | Commit | Date |
| --- | --- | --- |
| sst/opencode | `53a3f95088941aa0d3979af90b28b071c40fd866` | 2026-05-12 |
| cline/cline | `03f47045f338dcb6ac45b1ac1d6279a78be2b118` | 2026-05-11 |
| continuedev/continue | `cb273098d968906d25ee737b454f0b5f13ea2482` | 2026-04-16 |
| Aider-AI/aider | `3ec8ec5a7d695b08a6c24fe6c0c235c8f87df9af` | 2026-04-25 |
| zed-industries/zed | `917c6984834a2b6c08fd169520393b5187e33dda` | 2026-05-12 |
| BerriAI/litellm | `fc8a9a34067bb1571bb02bf6b9dc308f89ba168e` | 2026-05-12 |
| vercel/ai | `148f5fc3cf687ff205b2b001e1742880ccb8c5fc` | 2026-05-12 |
| anthropics/claude-code | (closed-source runtime; docs at docs.claude.com) | Living |

For claude-code and Cursor, citations are to the public docs at `docs.claude.com` and `docs.cursor.com` + issue tracker + community integration guides.
