# Phase J — OpenAI-Compatible API Surface

**Goal:** Add OpenAI Chat Completions + Models endpoints that translate to the
existing kiroxy Anthropic → Kiro → CodeWhisperer pipeline. Target clients:
Cursor, Continue, Cline, aider, any OpenAI SDK caller.

**Non-goal:** Become a multi-provider gateway. We are an OpenAI-compat *shim*
over our Anthropic surface; all semantics come from the Anthropic pipeline.

---

## 1. Architecture: adapter, not rewrite

```
OpenAI request
  → openai.TranslateRequest()  ─── builds anthropic.Request
    → messages.Service.HandleMessages path
      → reqconv → kiroclient → kiroproto events
        → respconv builds Anthropic {stream SSE | non-stream JSON}
      → openai.TranslateResponse()  ─── rewrites on the fly
OpenAI response
```

We do **not** duplicate the Kiro/Anthropic pipeline. We translate at the edges
only. The `messages.Service` stays untouched; we invoke it through a thin
in-process adapter that hands it a synthetic `http.ResponseWriter` intercepting
bytes, and we transform those bytes into OpenAI shape.

Two translation modes:

| Mode | Source | Target |
|---|---|---|
| **Non-streaming** | Anthropic response JSON (one shot) | OpenAI ChatCompletion JSON |
| **Streaming** | Anthropic SSE events | OpenAI SSE chunks + `data: [DONE]` |

---

## 2. Model alias table

Accept **both** OpenAI-flavor aliases and the existing Anthropic IDs. Unknown
models → 400 error listing available IDs.

| OpenAI alias (incoming) | Resolves to |
|---|---|
| `gpt-4o`, `gpt-4o-mini` | `claude-sonnet-4-6` (default best-cost) |
| `gpt-4-turbo`, `gpt-4` | `claude-opus-4-7` |
| `gpt-3.5-turbo` | `claude-haiku-4.5` |
| `openai/*` | strip `openai/` prefix then re-resolve |
| any `claude-*` | pass through to `models.Resolve` unchanged |
| any `claude-sonnet-4-6`, etc. | native Anthropic IDs — unchanged |

Alias resolution happens in `internal/openai/models.go` **before** we call
`models.Resolve`. `models.Resolve` already handles unknown-claude fallback.

---

## 3. Request translation (OpenAI → Anthropic)

```go
type ChatCompletionRequest struct {
    Model        string              `json:"model"`
    Messages     []ChatMessage       `json:"messages"`
    MaxTokens    *int                `json:"max_tokens,omitempty"`        // OpenAI default unlimited; we map to 4096
    Temperature  *float64            `json:"temperature,omitempty"`       // dropped (not in anthropic.Request)
    TopP         *float64            `json:"top_p,omitempty"`             // dropped
    Stream       bool                `json:"stream"`
    Stop         StopField           `json:"stop,omitempty"`              // string | []string
    Tools        []Tool              `json:"tools,omitempty"`
    ToolChoice   *ToolChoice         `json:"tool_choice,omitempty"`
    N            *int                `json:"n,omitempty"`                 // must be 1 or unset; reject otherwise
    PresencePenalty  *float64        `json:"presence_penalty,omitempty"`  // dropped
    FrequencyPenalty *float64        `json:"frequency_penalty,omitempty"` // dropped
    User         string              `json:"user,omitempty"`              // dropped
    ResponseFormat *ResponseFormat   `json:"response_format,omitempty"`   // dropped (v1.1 follow-up)
}

type ChatMessage struct {
    Role       string           `json:"role"`    // system | user | assistant | tool
    Content    MessageContent   `json:"content"` // string | []ContentPart
    Name       string           `json:"name,omitempty"`
    ToolCalls  []ToolCall       `json:"tool_calls,omitempty"`
    ToolCallID string           `json:"tool_call_id,omitempty"`
}
```

Rules:

1. `system` role messages (top of list) → concatenated into
   `anthropic.Request.System` (string form). Multiple system messages join
   with `\n\n`.
2. `user` / `assistant` → `anthropic.Message` with the same role.
3. `tool` role (OpenAI representation of a tool result) → `user` message
   containing a `tool_result` block referencing `tool_call_id`.
4. Assistant messages with `tool_calls` → assistant message with `tool_use`
   content blocks (one per tool_call).
5. Content parts:
   - `{type: "text", text: "..."}` → `{type:"text", text:"..."}`
   - `{type: "image_url", image_url: {url}}` — if `data:` URI, decode to
     `anthropic.ImageSource`; if `https:` URI, reject (Anthropic base64 only).
6. `tools[].function` → `anthropic.Tool` with `input_schema = function.parameters`.
7. `tool_choice`:
   - `"auto"` / unset → no tool_choice (default)
   - `"none"` → strip tools from request (effectively disables)
   - `"required"` → dropped (Anthropic doesn't require same; graceful pass)
   - `{type:"function", function:{name:X}}` → dropped (v1.1 follow-up — log warn)
8. `stop` string or array → `StopSequences`.
9. `max_tokens` → `MaxTokens` (Anthropic requires it; if unset default 4096).
10. `n > 1` → 400 error (Anthropic doesn't support multiple choices).

---

## 4. Response translation (Anthropic → OpenAI)

**Non-streaming:**

```json
{
  "id": "chatcmpl-<anthropic-msg-id-stripped>",
  "object": "chat.completion",
  "created": 1699999999,
  "model": "<original openai alias>",
  "choices": [{
    "index": 0,
    "message": {
      "role": "assistant",
      "content": "<concatenated text blocks>",
      "tool_calls": [ {"id":..., "type":"function", "function":{"name":..., "arguments":"<json>"}} ]
    },
    "finish_reason": "stop" | "length" | "tool_calls" | "content_filter"
  }],
  "usage": {
    "prompt_tokens": input_tokens,
    "completion_tokens": output_tokens,
    "total_tokens": input+output
  }
}
```

`stop_reason` mapping:
- `end_turn` → `"stop"`
- `max_tokens` → `"length"`
- `stop_sequence` → `"stop"`
- `tool_use` → `"tool_calls"`

Thinking blocks are **not** exposed in OpenAI format (not part of their API).
We drop them silently. `signature` is not preserved.

**Streaming chunks:**

Each chunk shape:
```json
{
  "id": "chatcmpl-<id>",
  "object": "chat.completion.chunk",
  "created": 1699999999,
  "model": "<alias>",
  "choices": [{
    "index": 0,
    "delta": { ... },
    "finish_reason": null | "stop" | "length" | "tool_calls"
  }]
}
```

Anthropic SSE event → OpenAI chunk mapping:
- `message_start` → one chunk with `delta: {role:"assistant", content:""}`
- `content_block_start` of type `text` → (no emission)
- `content_block_delta` of type `text_delta` → `delta: {content: <text>}`
- `content_block_start` of type `tool_use` → `delta: {tool_calls:[{index:N, id, type:"function", function:{name:<n>, arguments:""}}]}`
- `content_block_delta` of type `input_json_delta` → `delta: {tool_calls:[{index:N, function:{arguments:<partial_json>}}]}`
- `content_block_stop` → (no emission)
- `message_delta` with stop_reason → final chunk with `finish_reason`
- `message_stop` → emit `data: [DONE]\n\n`
- `thinking_delta`, `redacted_thinking`, `content_block_*` for thinking → dropped

Termination: always emit `data: [DONE]\n\n` then close.

Usage stats: OpenAI emits usage in the **final chunk** when the client opts in
via `stream_options: {include_usage: true}`. We'll include it unconditionally
when we have numbers (most OpenAI clients tolerate extra fields).

---

## 5. Error shape

OpenAI error:
```json
{
  "error": {
    "message": "...",
    "type": "invalid_request_error" | "api_error" | "authentication_error",
    "code": "..." ,
    "param": "..."
  }
}
```

We mirror this exactly. Our existing `httpx.WriteError` writes Anthropic shape
(`{type:"error", error:{type,message}}`). For OpenAI routes we use
`openai.WriteError` which writes the OpenAI shape.

Type mapping:
- 400 → `invalid_request_error`
- 401 → `authentication_error`
- 5xx → `api_error`

---

## 6. Tool calls — v1.0 includes with limits

Tool calls are in scope. Translation:

- `openai.tools[].function.{name, description, parameters}` →
  `anthropic.Tool{Name, Description, InputSchema}`.
- `tool_choice` support: `"auto"`, `"none"`. `"required"` and specific
  function-name choice emit a `400` with a clear message (documented).

Response tool_calls: accumulate the Anthropic `tool_use` blocks, serialize
`input` map as JSON string for `function.arguments`.

Streaming: emit `tool_calls` delta progressively as Anthropic emits
`input_json_delta`. OpenAI spec requires each `tool_calls` chunk to set
`index` matching the order.

---

## 7. Models endpoint — `GET /v1/models`

OpenAI format:
```json
{
  "object": "list",
  "data": [
    {"id": "claude-sonnet-4-6", "object": "model", "created": <unix>, "owned_by": "kiroxy"},
    {"id": "gpt-4o", "object": "model", "created": <unix>, "owned_by": "kiroxy"}
  ]
}
```

Data source: `models.ListModels()` + alias table keys. Deduplicate.

---

## 8. File layout

```
internal/openai/
    types.go                   OpenAI types + JSON (un)marshal for unions
    errors.go                  OpenAI error response + WriteError
    models.go                  alias table + ListModels builder
    translate_request.go       OpenAI → anthropic.Request
    translate_response.go      Anthropic JSON → OpenAI JSON (non-stream)
    translate_stream.go        Anthropic SSE → OpenAI SSE chunks
    types_test.go
    translate_request_test.go
    translate_response_test.go
    translate_stream_test.go

internal/server/
    openai.go                  handlers: handleChatCompletions, handleListModels
    openai_test.go             handler integration tests using stub kiro client
    server.go                  wire routes (MINIMAL delta at END of handler block)
```

No change to:
- `internal/messages/*`
- `internal/reqconv/*`
- `internal/respconv/*`
- `internal/kiroclient/*`
- `internal/pool/*`, `internal/auth/*`, `internal/tokenvault/*`
- `cmd/kiroxy/main.go` (Server.Options unchanged; handler composition
  pulls existing fields)

---

## 9. How we invoke the existing pipeline

The cleanest path is to reuse `messages.Service.HandleMessages` directly by
constructing a synthetic `*http.Request` with the translated Anthropic body
and an interceptor `http.ResponseWriter` that buffers or streams the
Anthropic response, feeding it into the OpenAI translator.

Steps:

1. Parse OpenAI request.
2. Resolve OpenAI alias → canonical model ID.
3. Translate to `anthropic.Request`.
4. Marshal translated request back to JSON.
5. Build `httptest.NewRequest("POST", "/v1/messages", body)`, copy
   `Authorization` / `X-Claude-Code-Session-Id` (or generate one).
6. Wrap response writer:
   - Streaming: wrapping writer reads our own SSE chunks as they are
     emitted, parses each Anthropic `event: ...\ndata: ...\n\n` frame,
     translates, writes OpenAI SSE to the real client.
   - Non-streaming: buffer all bytes, unmarshal Anthropic JSON, translate,
     write OpenAI JSON.
7. Invoke `msgSvc.HandleMessages(wrapped, syntheticReq)`.

Session ID: if client provides `X-Claude-Code-Session-Id`, pass through; if
not, generate a per-request UUID (OpenAI clients don't set this header).

---

## 10. Authentication

Same Bearer/X-Api-Key as `/v1/messages`. Reuses the server's `authMiddleware`
wrap automatically (we register routes on the same mux).

---

## 11. Commit plan

- c1: scaffold types + errors
- c2: request translation + tests
- c3: non-streaming response translation + tests
- c4: streaming translation + tests
- c5: models alias table + `/v1/models` endpoint
- c6: wire `/v1/chat/completions` + `/v1/models` routes in server
- c7: docs/OPENAI.md + CHANGELOG + BACKLOG update

Each commit must pass `make gate`.

---

## 12. Out of scope (v1.1+ follow-ups — file in BACKLOG)

- `response_format: {type: "json_object"}` / JSON mode
- Specific function-name `tool_choice`
- `stream_options.include_usage` explicit opt-in
- `/v1/completions` (legacy, deprecated)
- `/v1/responses` (newer OpenAI Assistants-style surface)
- `/v1/embeddings`
- OpenAI logprobs / logit_bias
- Vision: https URLs for images (Anthropic wants base64)
