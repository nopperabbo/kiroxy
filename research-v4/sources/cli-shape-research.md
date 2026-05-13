# Kiro Native Request Shape — Reverse Engineering Report

> Librarian report, compiled 2026-05-13 (Makassar).
> Feeds the "Kiro CLI Native Request Shape Analysis" section of ENOWX_STUDY.md.

## Summary

**Short answer: yes — but not uniformly.** Native-shaped traffic (correct User-Agent, `x-amzn-kiro-agent-mode`, `x-amzn-codewhisperer-optout`, lean payloads, synthetic system-prompt ack pair) does not demonstrably produce fewer 403s at steady-state vs proxy-shaped. What Kiro's server actually rejects is observably about payload correctness and size, not subtle fingerprinting:

- **Payloads >≈615 KB → 400 "Improperly formed request"** (not 413). `kiro-gateway` sets `KIRO_MAX_PAYLOAD_BYTES=600000` as a safe ceiling.
- **Tool-description length limits** (typically 1024 chars per `description`) → same 400.
- **Orphan `toolResult` / unmatched `toolUseId`** → 400.
- **Adjacent same-role messages** → 400 on new `runtime.*.kiro.dev` endpoint.
- **New `runtime.*.kiro.dev` endpoint strictly requires**: `Content-Type: application/x-amz-json-1.0`, `x-amz-target: AmazonCodeWhispererStreamingService.GenerateAssistantResponse`, `profileArn` in body regardless of auth type.
- **Model-ID format mismatch** (`anthropic.claude-sonnet-4-5-...` vs `claude-sonnet-4.5`) → 400 on new endpoint.
- **403s correlate primarily with auth state**, not request shape.

There is **no public evidence** that Kiro applies heuristics on history length, absence of IDE workspace context, or "proxy-ness" beyond the validation above.

## Source Availability

Official Kiro CLI source is **closed**. `kirodotdev/Kiro` is issues-only. All shape knowledge is reverse-engineered from six peer proxies:

- jwadow/kiro-gateway (Python/FastAPI, 979⭐) @ `0398d74f`
- d-kuro/kirocc (Go) @ `4ff8d812`
- Quorinex/Kiro-Go (Go) @ `1732b17f`
- petehsu/KiroProxy (Python) @ `9a91b9be`
- AntiHub-Project/Antigv-plugin (JS/Node) @ `06ad96f8`
- hj01857655/kiro-account-manager (Rust/Tauri) @ `6413d1ac`

## Native Headers

| Header | Value |
|---|---|
| Content-Type (new) | `application/x-amz-json-1.0` |
| X-Amz-Target | `AmazonCodeWhispererStreamingService.GenerateAssistantResponse` |
| User-Agent (CLI) | `aws-sdk-rust/1.3.14 ua/2.1 api/codewhispererstreaming/0.1.14474 os/macos lang/rust/1.92.0 md/appVersion-2.0.0 app/AmazonQ-For-CLI` |
| User-Agent (IDE 0.10+) | `aws-sdk-js/1.0.26 ua/2.1os/win32#10.0.26100 lang/js md/nodejs#22.21.1 api/codewhispererstreaming#1.0.26 m/E KiroIDE-0.10.32-<machineid>` |
| x-amz-user-agent | `<SDK UA> KiroIDE-<ver>-<sha256_machineid>` |
| x-amzn-kiro-agent-mode | **`vibe`** (default universal). `spec` documented but not seen on wire |
| x-amzn-codewhisperer-optout | `"true"` (most proxies) or `"false"` (kirocc). Both pass |
| amz-sdk-invocation-id | UUIDv4 per invocation |
| amz-sdk-request | `attempt=1; max=3` (static) or dynamic on retries |

Machine-ID suffix is **SHA-256 hex (64 chars)** from per-account stable input (profileArn > clientId > hardware UUID).

## Native Body Structure

```json
{
  "conversationState": {
    "chatTriggerType": "MANUAL" | "AUTO",
    "agentTaskType": "vibe",
    "conversationId": "<stable UUID per session>",
    "history": [
      // SYNTHETIC SYSTEM PROMPT PAIR when system is present:
      {"userInputMessage": {"content": "<system prompt>", "origin": "KIRO_CLI"}},
      {"assistantResponseMessage": {
        "messageId": "<SHA1-derived UUID>",
        "content": "I will fully incorporate this information when generating my responses, and explicitly acknowledge relevant parts of the summary when answering questions."
      }},
      // ... actual alternating turns
    ],
    "currentMessage": {
      "userInputMessage": {
        "content": "<user text or ''>",
        "modelId": "claude-sonnet-4.5",  // dot format
        "origin": "KIRO_CLI",  // or "AI_EDITOR" for IDE
        "userInputMessageContext": {
          "tools": [...],
          "toolResults": [...]
        }
      }
    }
  },
  "profileArn": "arn:aws:codewhisperer:us-east-1:..."  // REQUIRED on runtime.*
}
```

## Proxy-vs-Native Tells

### Proxies emit, native does NOT:
1. `"Continue"` placeholder on tool-result-only continuation (native: `""`)
2. Random `conversationId`/`agentContinuationId` per request (native: stable per session)
3. Merged adjacent same-role blobs (native: always alternating)
4. `"(empty)"` literal for empty assistant turn (native: omits)
5. `<thinking_mode>` XML injection (native: uses reasoningContentEvent)
6. `Content-Type: application/json` on new endpoint (native on new: `x-amz-json-1.0`)
7. `Connection: close` forced (native: keep-alive)

### Native emits, proxies MISS:
1. Synthetic system-prompt ack pair (deterministic text) → most proxies concatenate instead
2. Deterministic `messageId` (SHA1-based) on assistant history → 5/6 proxies random
3. Empty string `content: ""` on tool-result continuation → most use "Continue"
4. Kiro CLI v3 tool-result shape: `{json:{exit_status,stdout,stderr}}` → most use `{text}`
5. `origin: "KIRO_CLI"` vs `"AI_EDITOR"` matched to UA flavor
6. `x-amz-user-agent` with stable machine-id sha256

## Server-Side Detection: What We Can & Can't Conclude

- **403 = auth state, nearly always** (stale token, suspended account, wrong `profileArn`)
- **400 "Improperly formed request"** is overloaded: payload > 615KB, tool-desc > 1024 chars, orphan toolResults, missing profileArn on new endpoint, wrong modelId format, adjacent same-role, shape issues
- **429 = real rate limit** (both HTTP 429 AND HTTP 200 with AWS exception envelope)
- **No public evidence** of server-side heuristics detecting "proxy-shaped" traffic beyond validation rules

## Top-11 Mimicry Moves for kiroxy (ranked by impact)

1. Migrate to `runtime.us-east-1.kiro.dev` with correct headers (before May 15 sunset). Content-Type `x-amz-json-1.0`, X-Amz-Target set, profileArn always, dot-format modelId.
2. CLI-flavored Rust UA for Claude Code / CLI clients; IDE-flavored JS UA otherwise.
3. Set `x-amzn-kiro-agent-mode: vibe` universally.
4. Match `origin` field to UA flavor.
5. Stable `conversationId` per session (hash of first 2 messages). Stable `messageId` per assistant history.
6. Stop prepending `"Continue"` on tool-result-only continuation turns. Use `content: ""`.
7. Inject exact synthetic system-prompt ack pair (verbatim text) instead of concatenation.
8. Emit Kiro CLI v3 tool-result shape `{json:{exit_status,stdout,stderr}}` when UA=CLI.
9. Guard payload at 600KB. Trim oldest history before sending.
10. Enforce user/assistant alternation. First history entry must be user.
11. Don't change what's working. `x-amzn-codewhisperer-optout` either value passes.

## kiroxy Status

kiroxy already implements most of these correctly (see PROTOCOL.md §5):
- ✅ `runtime.*.kiro.dev` with correct Content-Type (post-Phase E migration)
- ✅ X-Amz-Target set
- ✅ profileArn always on new endpoint
- ✅ Dot-format modelId
- ✅ Synthetic system-prompt ack pair (`build_payload.go:109,122-132`)
- ✅ Stable `messageId` (UUID-v5 from content, `build_payload.go:112`)
- ✅ `content: ""` on tool-result turns (`build_payload.go:163`)
- ✅ Alternating history enforcement
- ⚠️ User-Agent flavor matching — **currently uses only IDE-style UA; CLI-flavored Rust UA for Claude Code clients is a BACKLOG candidate**
- ⚠️ Tool-result v3 shape (`{json:{exit_status,...}}`) — **currently uses `{text}`; v3 shape is a BACKLOG candidate**
- ⚠️ Stable `conversationId` per session — **not yet keyed per session; currently may be per-request**

## Open Questions (unverifiable from public sources)

- Does tool-turn density affect server-side routing?
- Is `x-amz-user-agent` machine-id cross-account validated?
- What does `spec` agent-mode unlock vs `vibe`?
- What is `q-developer-converse` agent-mode (seen only in kiro-account-manager)?
