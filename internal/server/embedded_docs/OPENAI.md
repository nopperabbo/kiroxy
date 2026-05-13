# OpenAI-Compatible API

kiroxy ships an OpenAI Chat Completions surface alongside the Anthropic
Messages API, so any tool that already speaks OpenAI — Cursor, Continue,
Cline, aider, the OpenAI SDK in any language — can route to the Kiro
backend with a one-line change.

## Endpoints

| Endpoint | Status |
|---|---|
| `POST /v1/chat/completions` | streaming + non-streaming |
| `GET /v1/models` | list available models |

Authentication reuses the `/v1/messages` bearer key: set
`Authorization: Bearer <YOUR_KIROXY_API_KEY>` (or `X-Api-Key: <key>`).

## Quick start

```bash
curl http://localhost:8787/v1/chat/completions \
  -H "Authorization: Bearer $KIROXY_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o",
    "messages": [{"role": "user", "content": "hello"}]
  }'
```

## Model aliases

You can send either an OpenAI-style alias or a native Claude model ID. The
alias resolves before the request hits the Kiro pipeline.

| OpenAI alias | Resolves to |
|---|---|
| `gpt-4o`, `gpt-4o-mini` | `claude-sonnet-4-6` (default, best cost) |
| `gpt-4-turbo`, `gpt-4` | `claude-opus-4-7` |
| `gpt-3.5-turbo` | `claude-haiku-4.5` |
| `o1`, `o1-mini` | `claude-opus-4-7` / `claude-sonnet-4-6` |
| `openai/<anything>` | stripped, then re-resolved |
| `claude-<any>` | passed through unchanged |

The response `model` field echoes back exactly what the client sent, so
integrations that display the model stay coherent.

## Client integrations

### Cursor

Settings → Models → **OpenAI API Key** → set base URL to your kiroxy
instance (`http://localhost:8787/v1`) and paste your `KIROXY_API_KEY`.
Add a custom model and pick any alias (`gpt-4o` is the best default).

### Continue (VS Code / JetBrains)

`~/.continue/config.json`:

```json
{
  "models": [
    {
      "title": "Kiroxy (Sonnet 4.6)",
      "provider": "openai",
      "model": "gpt-4o",
      "apiBase": "http://localhost:8787/v1",
      "apiKey": "YOUR_KIROXY_API_KEY"
    }
  ]
}
```

### Cline

Settings → API Provider → **OpenAI Compatible** → set base URL
`http://localhost:8787/v1`, API key, and model `gpt-4o`.

### aider

```bash
export OPENAI_API_KEY=$KIROXY_API_KEY
export OPENAI_API_BASE=http://localhost:8787/v1
aider --model gpt-4o
```

### OpenAI Python SDK

```python
from openai import OpenAI

client = OpenAI(
    api_key="YOUR_KIROXY_API_KEY",
    base_url="http://localhost:8787/v1",
)

resp = client.chat.completions.create(
    model="gpt-4o",
    messages=[{"role": "user", "content": "hello"}],
)
print(resp.choices[0].message.content)
```

## Feature support

| Feature | Status | Notes |
|---|---|---|
| Streaming (`stream: true`) | ✓ | Emits `data: [DONE]` terminator |
| Tool calls (`tools`, `tool_calls`) | ✓ | OpenAI function tools → Anthropic tools |
| `tool_choice: "auto"` | ✓ | Default |
| `tool_choice: "none"` | ✓ | Strips tools |
| `tool_choice: "required"` | silently ignored | Upstream doesn't enforce |
| `tool_choice: {name: X}` | **400** | Planned for v1.1 |
| Image parts | ✓ (data: URIs only) | `https://` URLs rejected (upstream limitation) |
| `max_tokens` | ✓ | Also accepts `max_completion_tokens` |
| `stop` (string or array) | ✓ | Maps to Anthropic `stop_sequences` |
| `n` | must be 1 | Upstream is single-choice only |
| `response_format: json_object` | silently ignored | Planned for v1.1 |
| `temperature`, `top_p` | silently ignored | Not in Anthropic Messages API |
| `presence_penalty`, `frequency_penalty` | silently ignored | Not in Anthropic Messages API |
| `logprobs`, `logit_bias` | silently ignored | Not supported upstream |
| Usage stats | ✓ | `prompt_tokens`, `completion_tokens`, `total_tokens` |

"Silently ignored" means the field is accepted without error so OpenAI-SDK
clients that always send it keep working; the translation just does not
use it.

## Error shape

Errors come back in OpenAI format:

```json
{
  "error": {
    "message": "...",
    "type": "invalid_request_error | api_error | authentication_error",
    "param": "...",
    "code": "..."
  }
}
```

Type mapping:

- `400` → `invalid_request_error`
- `401`, `403` → `authentication_error`
- `5xx` → `api_error`

## Not implemented (yet)

- `/v1/responses` (Assistants-style API)
- `/v1/completions` (legacy text completions)
- `/v1/embeddings`
- `/v1/audio`, `/v1/images`
- Specific function-name `tool_choice`
- JSON-mode `response_format`
- `stream_options.include_usage` explicit opt-in (usage is always included
  in the final chunk today, which OpenAI SDKs tolerate)

File feature requests against the project tracker if any of these block
your use case.
