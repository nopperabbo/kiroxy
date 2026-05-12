# PROTOCOL.md — Kiro / AWS CodeWhisperer wire reference

> Comprehensive reference for the protocol that a Kiro proxy must speak.
> Assembled from kiroxy's own `internal/kiroproto/`, `internal/kiroclient/`,
> `internal/auth/`, `internal/reqconv/`, and peer projects (d-kuro/kirocc,
> Quorinex/Kiro-Go, jwadow/kiro-gateway, petehsu/KiroProxy,
> hj01857655/kiro-account-manager, AntiHub-Project/Antigv-plugin).
>
> Every field, endpoint, event type, and error code listed here is used or
> handled in kiroxy today. Citations follow the convention
> `file:line` (kiroxy path) or `repo/path:line` (peer) or `peer-issue#NNN`.
>
> Kiro's API surface is not publicly documented by AWS. This file is the
> result of reverse-engineering by the Kiro-proxy community, with peer
> cross-validation. It is the closest thing to an authoritative reference
> that exists.
>
> Target audience: contributors fixing bugs, operators debugging live
> requests, maintainers preparing for upstream changes.

---

## Table of contents

1. [Domain layout](#1-domain-layout)
2. [Authentication endpoints](#2-authentication-endpoints)
3. [Runtime endpoints](#3-runtime-endpoints)
4. [Request envelope: headers](#4-request-envelope-headers)
5. [Request body (`conversationState`)](#5-request-body-conversationstate)
6. [Response EventStream](#6-response-eventstream)
7. [Error catalog](#7-error-catalog)
8. [Model IDs](#8-model-ids)
9. [Token formats](#9-token-formats)
10. [Profile ARN](#10-profile-arn)
11. [Quirks & undocumented behavior](#11-quirks--undocumented-behavior)
12. [Sources](#12-sources)

---

## 1. Domain layout

Kiro as a product distinguishes between **auth** (token mint + refresh),
**runtime** (chat generation), and **management** / **telemetry**
(referenced but undocumented). AWS is actively migrating every runtime
endpoint from legacy `q.<region>.amazonaws.com` to first-class
`runtime.<region>.kiro.dev` as of 2026-05-15.

| Domain | Purpose | Auth required? |
|---|---|---|
| `auth.desktop.kiro.dev` / `prod.<region>.auth.desktop.kiro.dev` | Kiro Desktop / social-flow token issuance + refresh | refresh token on refresh; none on login |
| `oidc.<region>.amazonaws.com` | Builder ID / IDC device-code flow; OAuth client register; token refresh for `aoa*`-prefix credentials | client id + client secret (device registration); refresh token on refresh |
| `runtime.<region>.kiro.dev` | **New**: chat generation, SendMessage / GenerateAssistantResponse | bearer access_token |
| `q.<region>.amazonaws.com` | **Legacy**, deprecated 2026-05-15 (documented 2026-08-15 sunset). Same surface as runtime.kiro.dev | bearer access_token |
| `management.<region>.kiro.dev` | Referenced in peer code but kiroxy does not use. Purpose unclear; believed to be IDE settings sync | bearer token |
| `telemetry.<region>.kiro.dev` | Referenced in IDE but kiroxy explicitly does not proxy | n/a |

Regions currently served (runtime): **us-east-1**, **eu-central-1**.
Peer code in `Quorinex/Kiro-Go` and `jwadow/kiro-gateway` hardcode both.
Expansion expected with AWS regional rollout cadence; no public date.

**Kiroxy endpoint selection:**
- `internal/kiroclient/client.go:167` (`endpointURL`) defaults to
  `runtime.<region>.kiro.dev`; env `KIROXY_USE_LEGACY_ENDPOINT=1` flips
  to legacy.
- Migration reference: `jwadow/kiro-gateway#146` (deprecation), PR `#155`
  (bhaskoro's migration in jwadow; kiroxy commit `8b20a1f`).

---

## 2. Authentication endpoints

### 2.1 Social flow (Kiro Desktop / `aor*` tokens)

**Login** (not consumed by proxies — operator does this in Kiro Desktop):
```
POST https://auth.desktop.kiro.dev/login
```
Returns an interactive OAuth flow in a browser. Kiroxy's onboarder
(`tools/onboard/`) automates this via Camoufox to scrape the resulting
credentials from the Kiro Desktop SQLite DB.

**Refresh** (called by proxies on access_token expiry):
```
POST https://prod.<region>.auth.desktop.kiro.dev/refreshToken
Content-Type: application/json

{"refreshToken": "<aor-prefixed-refresh-token>"}
```

Successful response (cited from `internal/auth/refresh.go:205`):
```json
{
  "accessToken":  "aoa...",
  "refreshToken": "aor...",   // may be omitted — caller coalesces
  "expiresIn":    3600,       // seconds
  "profileArn":   "arn:aws:codewhisperer:us-east-1:..."
}
```

Errors observed (from `internal/auth/refresh_social.go:81`):
- `401 Unauthorized` / `403 Forbidden` → refresh_token revoked.
  Non-retryable; requires operator to re-onboard.
- `5xx` → transient; retry with backoff.
- `200` with `accessToken == ""` or `expiresIn <= 0` → malformed; treat
  as transient.

**Refresh token rotation:**
The social endpoint sometimes returns the same `refreshToken`, sometimes a
new one. Kiroxy coalesces the new value with the old via
`auth.coalesce(result.RefreshToken, creds.RefreshToken)`
(`internal/auth/refresh.go:266,287`). Peers jwadow and Quorinex
implement the same coalesce; skipping it causes a silent
"refresh-token zeroed out" bug surfaced in `bhaskoro/kiro-gateway#155`.

### 2.2 IDC / Builder ID flow (`aoa*` tokens)

**Device registration** (one-time, done by Kiro CLI / kiro-cli tool):
```
POST https://oidc.<region>.amazonaws.com/client/register
```
Produces `clientId` + `clientSecret` persisted in kiro-cli's SQLite DB.

**Device authorization** (one-time, interactive):
```
POST https://oidc.<region>.amazonaws.com/device_authorization
```
Produces a user_code + verification_uri.

**Token refresh** (called by proxies):
```
POST https://oidc.<region>.amazonaws.com/token
Content-Type: application/json

{
  "grantType":    "refresh_token",
  "clientId":     "...",
  "clientSecret": "...",
  "refreshToken": "aoa..."
}
```
Cited: `internal/auth/refresh.go:249`.

Response shape is the same as social refresh minus `profileArn`
(Builder ID accounts don't carry a profileArn — they route to the
AmazonQ target; see §4.2).

### 2.3 Refresh semantics: validity buffer

Both paths cache the access_token and only refresh when
`expiresAt - 5min < now`. Cited: `internal/auth/refresh.go:23`
(`tokenValidityBuffer = 5 * time.Minute`). Peer Quorinex uses 60s
buffer; jwadow uses 5 min. Shorter buffers mean more refreshes; longer
mean rare in-flight token rejection. 5 min is the conservative choice.

### 2.4 Concurrent refresh handling

Kiroxy uses `golang.org/x/sync/singleflight` on key `"refresh"` to
dedupe N concurrent callers into one HTTP refresh
(`internal/auth/refresh.go:100`). Peer jwadow uses an asyncio.Lock;
peer Quorinex uses sync.Mutex + a shared channel. All three converge
on the same guarantee: only one outbound refresh request per access
window, regardless of caller concurrency.

The caller's context is **detached** for the refresh call because a
short-lived request (e.g. claude-code's 30s connect timeout) must not
cancel the refresh for all waiting goroutines
(`internal/auth/refresh.go:101`: `context.WithoutCancel(ctx)`).
A bounded 35s timeout is applied separately. This is a subtle fix
kiroxy derives from kirocc commit 5633c47f.

---

## 3. Runtime endpoints

### 3.1 URL pattern

```
POST https://runtime.<region>.kiro.dev/
POST https://q.<region>.amazonaws.com/       # legacy
```

Both paths accept an identical envelope. There is no path component —
routing is done entirely by the `X-Amz-Target` header (AWS JSON 1.0
convention).

Cited: `internal/kiroclient/client.go:174,172`.

### 3.2 Request path

Every chat request is a single `POST /` with:
- JSON body containing `conversationState` + optional `profileArn`
- Eight required headers (see §4)
- Response is either `application/vnd.amazon.eventstream` (success) or
  `application/json` (AWS exception envelope, even on HTTP 200)

Kiro does NOT use path-based versioning (no `/v1/`, no `/v2/`). Version
negotiation happens via `Content-Type` (`application/x-amz-json-1.0`)
and `X-Amz-Target`.

---

## 4. Request envelope: headers

### 4.1 Required headers

Cited: `internal/kiroclient/client.go:216-224`.

| Header | Value | Source of truth |
|---|---|---|
| `Authorization` | `Bearer <access_token>` | Access token from §2 |
| `Content-Type` | `application/x-amz-json-1.0` | AWS JSON 1.0 protocol |
| `Accept` | `*/*` | Matches Kiro IDE |
| `X-Amz-Target` | see §4.2 | Routes to SendMessage vs GenerateAssistantResponse |
| `User-Agent` | `aws-sdk-js/1.0.34 ua/2.1 os/darwin#24.6.0 lang/js md/nodejs#22.22.0 api/codewhispererstreaming#1.0.34 m/E KiroIDE-0.11.107` | Matches Kiro Desktop |
| `x-amz-user-agent` | `aws-sdk-js/1.0.34 KiroIDE-0.11.107` | Matches Kiro Desktop |
| `x-amzn-codewhisperer-optout` | `false` | Telemetry opt-in |
| `amz-sdk-invocation-id` | UUID per request | AWS SDK convention |
| `amz-sdk-request` | `attempt=N; max=M` | AWS SDK retry bookkeeping |

**User-Agent matters.** Peer code in `kirocc` originally used a Rust-SDK
UA string; that fails for Builder ID accounts at the gateway with
"credential is invalid". Switching to the Kiro IDE `aws-sdk-js` UA
format fixed both auth paths — kiroxy took this change verbatim from
Quorinex (`internal/kiroclient/client.go:43-50`).

### 4.2 X-Amz-Target values

Kiro routes requests to one of two backend services via
`X-Amz-Target`:

| Target | When to use | Credential source |
|---|---|---|
| `AmazonCodeWhispererStreamingService.GenerateAssistantResponse` | account has `profileArn` | social flow (`aor*` tokens) |
| `AmazonQDeveloperStreamingService.SendMessage` | account has no `profileArn` | Builder ID / IDC (`aoa*` tokens) |

Cited: `internal/kiroclient/client.go:32,38`, `internal/kiroclient/target.go:19`.

Sending the CodeWhisperer target without a `profileArn` produces the
infamous `UnauthorizedException: "profileArn is required for this
request"`. Conversely, sending the AmazonQ target with a profileArn is
tolerated in practice but undocumented; kiroxy strictly routes by
credential type.

Peer evidence: `Quorinex/Kiro-Go` uses the same dual-target dispatch
(`handler.go` pre-MIT commit). `jwadow/kiro-gateway` uses only
CodeWhisperer (social-only) and so cannot service Builder ID accounts,
which was a reported limitation in that project.

### 4.3 Session / conversation headers (optional)

| Header | Purpose |
|---|---|
| `X-Claude-Code-Session-Id` | Per-session stable UUID claude-code sends; kiroxy forwards unchanged. Referenced `internal/server/openai.go:99` |
| `X-Request-Id` | Trace ID on inbound; kiroxy accepts user-supplied value or mints one. Referenced `internal/server/logging.go:37` |
| `X-Forwarded-For` | Logged, not forwarded to upstream. `internal/server/logging.go:158` |

---

## 5. Request body (`conversationState`)

### 5.1 Top-level envelope

Cited: `internal/kiroproto/types.go:23-27`.

```json
{
  "conversationState": { ... },
  "profileArn": "arn:aws:codewhisperer:us-east-1:..."   // optional; required when X-Amz-Target is CodeWhisperer
}
```

`profileArn` is omitted when `X-Amz-Target` is the AmazonQ target.

### 5.2 `conversationState`

Cited: `internal/kiroproto/types.go:30-36`.

```json
{
  "conversationId":  "<uuid, optional on first turn>",
  "chatTriggerType": "MANUAL",
  "agentTaskType":   "vibe",
  "currentMessage":  { "userInputMessage": { ... } },
  "history":         [ { "userInputMessage": {...} }, { "assistantResponseMessage": {...} }, ... ]
}
```

**`chatTriggerType`** observed values across peer code:
- `MANUAL` — standard user prompt (kiroxy always sends this)
- `AUTO` — IDE-internal auto-suggest
- `CONTEXT_MENU` — IDE right-click action
- `INLINE_CHAT` — IDE inline completion

Kiroxy only sends `MANUAL`. Sending other values from a proxy is
unnecessary and not observed to change behavior.

**`agentTaskType`** observed values:
- `vibe` — default free-form coding (kiroxy)
- `CODE_REVIEW` — Kiro IDE "review this code" entrypoint
- `GENERATE_UNIT_TESTS` — Kiro IDE "generate tests" entrypoint
- Undocumented: `DOC_GEN`, `TRANSFORM`, others visible in peer logs

Kiroxy hardcodes `vibe` (`internal/kiroproto/types.go:17`,
`ChatTriggerTypeManual + AgentTaskTypeVibe`). This matches
kiro-cli's default chat mode; switching to other values can invoke
Kiro's specialist sub-prompts and is rarely wanted from a proxy.

### 5.3 `userInputMessage`

Cited: `internal/kiroproto/types.go:44-51`.

```json
{
  "content":                 "<user prompt text>",
  "modelId":                 "claude-sonnet-4.6",
  "origin":                  "KIRO_CLI",
  "userInputMessageContext": { ... },
  "images":                  [ { "format":"png", "source":{"bytes":"<base64>"} }, ... ],
  "cachePoint":              { "type": "default" }
}
```

**`origin`** values seen:
- `KIRO_CLI` (kiroxy, kirocc, Quorinex, jwadow — consensus)
- `KIRO_IDE` (Kiro Desktop itself)
- `KIRO_WORKSPACE` (Workspace org subscribers)

Kiroxy always sends `KIRO_CLI` (`internal/kiroproto/types.go:15`).
Peer evidence: `jwadow/kiro-gateway` uses `KIRO_CLI`. `petehsu/KiroProxy`
uses `KIRO_IDE`. Both work; no observable behavior difference.

**`modelId`** is the **Kiro SKU**, not the Anthropic alias. See §8.

**`cachePoint`** with `{"type":"default"}` tells Kiro to insert an
implicit prompt cache boundary before this message. Kiroxy places cache
points at specific locations governed by
`internal/reqconv/cache_points.go`. See §5.5.

### 5.4 `userInputMessageContext`

Cited: `internal/kiroproto/types.go:54-57`.

```json
{
  "tools": [
    { "toolSpecification": { "name": "...", "description": "...", "inputSchema": { "json": { ... } } } },
    { "cachePoint": { "type": "default" } }
  ],
  "toolResults": [
    { "toolUseId": "tool_use_12345", "status": "success", "content": [ { "text": "..." } ] }
  ]
}
```

**`tools`** is a mixed array of `toolSpecification` entries and
`cachePoint` markers. Each entry is exactly one or the other — never
both. The marshal helper at `internal/kiroproto/types.go:66` enforces
this discriminant.

**`toolResults`** status values observed:
- `"success"` — tool call completed
- `"error"` — tool call errored (kiroxy sets this when the downstream
  client passes `is_error: true`)

Only ONE of `text` or `json` should be set in each `content` element
(`internal/kiroproto/types.go:97`). Peer bugs: jwadow sometimes sends
both; Kiro tolerates it but may treat as undefined behavior.

### 5.5 Cache-point placement

Kiro supports **up to 4 prompt-cache breakpoints per request** (AWS
convention). Kiroxy places them at:
1. End of tools array (so tool definitions are cached once across a
   session).
2. End of each message in history after the system prompt.

Cited: `internal/reqconv/cache_points.go`. Peer Quorinex uses the same
pattern since PR #34.

Cache hits show up as `cacheReadInputTokens` in the `metadataEvent`
(§6.4). Missing a cache point is not an error, just a missed saving.

### 5.6 `history` format

Cited: `internal/kiroproto/types.go:120-135`.

```json
[
  { "userInputMessage":         { "content": "...", "origin": "KIRO_CLI", "modelId": "...", "userInputMessageContext": { ... } } },
  { "assistantResponseMessage": { "messageId": "uuid", "content": "...", "toolUses": [...], "cachePoint": {...} } }
]
```

Each history element is a discriminated union — exactly ONE of
`userInputMessage` or `assistantResponseMessage` is present. Marshal
helper: `internal/kiroproto/types.go:126`.

**`messageId`** on the assistant message is optional but kiroxy sets it
deterministically for the synthetic ack after system prompt
(`internal/reqconv/build_payload.go:112`, UUID-v5 derived from the
ack content so the same ack always gets the same ID — crucial for
prompt caching).

### 5.7 The synthetic system-prompt ack

kiroxy inserts a fixed user/assistant pair at the top of `history`
whenever a system prompt is present:

```
user:      {system prompt}
assistant: "I will fully incorporate this information when generating
            my responses, and explicitly acknowledge relevant parts of
            the summary when answering questions."
```

Cited: `internal/reqconv/build_payload.go:109,122-132`.

This mimics the kiro-cli behavior observed in packet captures. Every
production Kiro proxy that matches kiro-cli's behavior does this. The
ack is sent even when history was previously empty — this is load-bearing;
Kiro treats it as part of the prompt cache and skipping it
reduces cache hit rates.

### 5.8 The synthetic "Continue" user message

When the last message in an Anthropic request has role `assistant`,
kiroxy synthesizes a trailing user message with content `"Continue"`
(`internal/reqconv/build_payload.go:98-102`). Kiro expects every
turn to end with a user input; peers jwadow, Quorinex, AntiHub do the
same. claude-code sometimes sends assistant-terminal history when
resuming a stream.

### 5.9 Tool-result-only turns

When the user's current turn is purely a tool result (no text), kiroxy
keeps `content: ""` rather than forcing a "Continue" text
(`internal/reqconv/build_payload.go:163`). This matches kiro-cli's
observed continuation shape. Sending "Continue" here causes Kiro to
echo a human-readable "let me continue" message instead of actually
processing the tool result.

### 5.10 `thinking` mode injection

When thinking is enabled, kiroxy injects XML tags inline in the user
content rather than as a separate field:
```
<thinking_mode>enabled</thinking_mode>
<max_thinking_length>{budget}</max_thinking_length>

{user content}
```
Cited: `internal/reqconv/build_payload.go:179-183`.

This matches Quorinex/Kiro-Go's PR #40. Kiro's own SendMessage API
does NOT expose a `thinking` boolean at the top level — the XML tag
format is the only documented way to opt in.

### 5.11 Size limits

Request body limit: 4 MB observed in peer handling, enforced by
client at `internal/kiroproto/frame.go:17` (max frame size, though that
applies to response — request limits are server-side). Payloads over
~3 MB routinely get `413 Payload Too Large` or
`ValidationException`. Kiroxy does not enforce a client-side limit;
this is a BACKLOG item.

---

## 6. Response EventStream

### 6.1 Framing

Kiro uses the AWS EventStream binary framing:

```
[ total_length : 4 bytes big-endian ]
[ headers_length: 4 bytes big-endian ]
[ prelude_crc:   4 bytes big-endian ]  ← CRC32(bytes[0:8])
[ headers:       headers_length bytes ]
[ payload:       (total_length - headers_length - 16) bytes ]
[ message_crc:   4 bytes big-endian ]  ← CRC32(bytes[0:total_length-4])
```

Cited: `internal/kiroproto/frame.go:25-86`.

Both CRCs use CRC-32 IEEE polynomial. Violations = drop the connection.

**Max frame size:** 4 MiB (`internal/kiroproto/frame.go:17`). AWS
appears to enforce a smaller cap upstream (~1 MiB observed for
cumulative streams) but per-frame ceiling is 4 MiB.

### 6.2 Frame headers

Each frame has typed headers. Only two are load-bearing:
- `:message-type` (type 7 = string) — `"event"` or `"exception"`
- `:event-type` / `:exception-type` (type 7 = string) — event name

Cited: `internal/kiroproto/frame.go:105-154`. Other header types
(bool, int, short, long, byte-array, timestamp, uuid) are accepted
by the parser but Kiro doesn't currently emit them in load-bearing
positions.

### 6.3 Event type catalog

Every event type kiroxy parses, from `internal/kiroproto/eventstream.go:60-78`:

| Event type | Purpose | Fields |
|---|---|---|
| `initial-response` | First frame; handshake | `conversationId` observed |
| `assistantResponseEvent` | Text delta | `content`, `modelId` |
| `reasoningContentEvent` | Thinking text delta | `text`, `signature`, `redactedContent` |
| `toolUseEvent` | Tool call delta | stream of deltas accumulated via `toolUseAccumulator` |
| `metadataEvent` | Token accounting | `tokenUsage.uncachedInputTokens`, `outputTokens`, `totalTokens`, `cacheReadInputTokens`, `cacheWriteInputTokens` |
| `meteringEvent` | Cost accounting | `usage` (credits), `inputTokens`, `outputTokens` |
| `invalidStateEvent` | Stream-level fault | `reason`, `message` |
| `exception` | `:message-type=exception` frame | `message` string |
| `messageMetadataEvent` | Conversation IDs | `conversationId`, `utteranceId` |
| `followupPromptEvent` | Suggested follow-ups | ignored by kiroxy |
| `citationEvent` | Inline citations | ignored |
| `codeEvent` / `codeReferenceEvent` | Code attribution | ignored |
| `supplementaryWebLinksEvent` | Sources | ignored |
| `intentsEvent` | Intent inference | ignored |
| `interactionComponentsEvent` | IDE interaction widgets | ignored |
| `dryRunSucceedEvent` | Dry-run mode ack | ignored |
| `contextUsageEvent` | **Context % remaining** | `contextUsagePercentage` |

**`contextUsageEvent`** was the critical Phase-R finding: Kiroxy needed
it to compute accurate `input_tokens` in response to claude-code's
context-window math. Quorinex PR #37 shipped this first; kiroxy
adopted it. Peer evidence: `Quorinex/Kiro-Go` PR #37.

### 6.4 `metadataEvent` vs `meteringEvent`

Both carry token counts but from different ledgers:
- `metadataEvent` = operational: what Kiro billed against your prompt
  cache.
- `meteringEvent` = financial: credit units consumed.

Fields overlap but are not identical:

| Field | `metadataEvent` | `meteringEvent` |
|---|---|---|
| `inputTokens` | derived = `uncachedInputTokens + cacheReadInputTokens` | raw from server |
| `outputTokens` | yes | yes |
| `totalTokens` | yes | no |
| `cacheReadInputTokens` | yes | no |
| `cacheWriteInputTokens` | yes | no |
| `usage` (credits) | no | yes |

kiroxy emits both; `/v1/messages` uses metadataEvent for `usage.input_tokens`
and prefers meteringEvent for operator-visible credit count
(`internal/kiroproto/eventstream.go:163-203`).

### 6.5 Tool-use accumulation

`toolUseEvent` is streamed as deltas — kiroxy accumulates them with
`toolUseAccumulator` (`internal/kiroproto/tooluse.go`). The
accumulator:
1. Gathers `input` JSON deltas per `toolUseId` until the stop flag.
2. Flushes an event with the full tool call on stop or EOF.
3. Handles out-of-order/interleaved deltas for parallel tool calls.

If a stream EOFs without a stop frame, the accumulator flushes any
in-progress tool to the callback (`eventstream.go:96-99`). This
prevents tool calls from being silently dropped.

### 6.6 Ordering guarantees

Observed order from Kiro (across kiroxy + peer captures):

```
initial-response
[messageMetadataEvent]
[contextUsageEvent]
{ assistantResponseEvent | reasoningContentEvent | toolUseEvent (streaming deltas) }*
metadataEvent
meteringEvent
```

The stream terminates cleanly with a zero-byte body close after the
last metering event. No explicit `end` marker. Kiroxy handles EOF
gracefully at `eventstream.go:95` (flush any in-progress tool-use,
return nil).

---

## 7. Error catalog

### 7.1 HTTP-level status codes

Cited: `internal/kiroclient/client.go:248-346`.

| Status | Content-Type | Meaning | Kiroxy action |
|---|---|---|---|
| `200 OK` | `application/vnd.amazon.eventstream` | Successful stream | Parse events |
| `200 OK` | `application/json` | **AWS exception envelope on 200** | Read JSON, extract `__type`, retry if transient, else `UpstreamError` |
| `401 Unauthorized` | varies | Token invalid at auth layer | Not handled directly (refresh layer returns this) |
| `403 Forbidden` | often empty body | Profile ARN required / credential-gateway reject | Invalidate cache, refresh, retry once |
| `429 Too Many Requests` | JSON | Quota / throttling | Backoff + retry (3 attempts max) |
| `5xx` | JSON or empty | Upstream internal error | Backoff + retry (3 attempts max) |

**The 200-with-JSON quirk** is critical: Kiro returns `HTTP 200` with
`Content-Type: application/json` body containing `{"__type": "ThrottlingException"}`
or similar, instead of promoting to a 4xx/5xx. Kiroxy checks
Content-Type at `client.go:263` and treats this as an error. Without
that check, the eventstream parser reads the JSON bytes as frames and
errors with a confusing "prelude CRC mismatch". Peer evidence:
`jwadow/kiro-gateway` shipped the same detection in early April 2026
(cited in the `research-v2/` v1 delta recheck).

### 7.2 AWS exception classes

Extracted via `parseAWSExceptionType` from `__type` / `type` / `code`
fields in body, or `X-Amzn-ErrorType` response header. Normalized to
strip `com.amazon...#` prefix and `:hostname` suffix
(`internal/kiroclient/aws_error.go:78-91`).

**Retryable** (cause a retry attempt):
- `ThrottlingException`
- `TooManyRequestsException`
- `ServiceUnavailableException`
- `InternalServerException`
- `InternalFailureException`
- `InternalServerError`

Cited: `aws_error.go:60-68`.

**Not retryable** (propagate to caller as `UpstreamError`):
- `ValidationException` — bad request shape
- `UnauthorizedException` — auth rejected
- `AccessDeniedException` — policy
- `ResourceNotFoundException`
- `ConflictException`
- any class not in the retry list above

### 7.3 Error shape observed in peer captures

Full exception body:
```json
{
  "__type": "com.amazon.coral.service#ThrottlingException",
  "message": "Rate exceeded"
}
```

Header variant:
```
X-Amzn-ErrorType: ThrottlingException:http://internal.amazon.com/coral/com.amazon.coral.service/
```

The `:hostname` suffix and the `com.amazon...#` prefix are both
stripped by normalization (`aws_error.go:82-89`).

### 7.4 403 with empty body

Kiroxy has seen this in production during the endpoint migration
(repo BACKLOG: "Upstream 403 with fresh credentials + new endpoint",
2026-05-12 live smoke). Symptoms: fresh refreshed token + correct
profileArn + new endpoint produce 403 with no body. Hypothesis: the
request body shape diverges slightly from what `runtime.kiro.dev`
accepts. Investigation ongoing.

### 7.5 Auth-layer vs service-layer errors

Distinguishing auth-layer (auth.desktop.kiro.dev returned 401)
from service-layer (runtime.kiro.dev returned 401):

| Observable | Auth layer | Service layer |
|---|---|---|
| URL path | `/refreshToken` or `/token` | `/` (POST) |
| Response wrapper | JSON | JSON or eventstream |
| Typical body | `{"message": "Refresh token is invalid"}` | `{"__type": "UnauthorizedException", "message": "..."}` |
| Recoverable? | Requires re-onboard | Refresh once, then give up |

Kiroxy handles these in separate packages (`auth/` vs
`kiroclient/`) with typed errors (`ErrRefreshUnauthorized`,
`UpstreamError{Status: 401}`) so callers can route correctly.

---

## 8. Model IDs

### 8.1 Canonical Kiro SKUs

As of 2026-05-13, kiroxy's `internal/models/models.go:34-43`:

| Anthropic alias | Kiro SKU | 1M variant | Notes |
|---|---|---|---|
| `claude-opus-4-7` | `claude-opus-4.7` | `claude-opus-4.7` | always 1M context |
| `claude-opus-4-7[1m]` | `claude-opus-4.7` | `claude-opus-4.7` | same — the suffix is an advertisement, not a routing flag |
| `claude-opus-4-6` | `claude-opus-4.6` | `claude-opus-4.6` | always 1M |
| `claude-sonnet-4-6` | `claude-sonnet-4.6` | `claude-sonnet-4.6-1m` | separate 1M SKU |
| `claude-sonnet-4.5` | `claude-sonnet-4.5` | `claude-sonnet-4.5-1m` | separate 1M SKU |
| `claude-opus-4.5` | `claude-opus-4.5` | n/a | 200k only |
| `claude-haiku-4.5` | `claude-haiku-4.5` | n/a | 200k only |

**Dot vs dash format:**
- Anthropic convention uses dashes (`claude-opus-4-7`).
- Kiro's new runtime endpoint **accepts dot-format** (`claude-opus-4.7`).
- Kiro's legacy endpoint also accepts dot-format but historically
  tolerated dashes in some contexts.

Kiroxy always sends dot-format to upstream
(`models.go:130`, the `Kiro` field is the source of truth).

### 8.2 Context window sizes

- `DefaultContextWindowSize = 200_000` (`models.go:27`)
- `ThinkingContextWindowSize = 1_000_000` (`models.go:28`)

The `[1m]` suffix on the **response** model ID signals to claude-code
(via its `mR()` / `A2()` functions) that the model has 1M context.
This lets claude-code adjust its prompt-planning accordingly. Without
the suffix, claude-code assumes 200k and truncates prompts
aggressively. Kiroxy always appends `[1m]` to
`anthropicModel` when context is 1M
(`models.go:195-197`).

### 8.3 Thinking opt-in

Two ways to enable thinking:
1. **Model suffix**: request model name ends in `[1m]` AND a separate
   `-1m` SKU exists → Kiroxy sends the `-1m` SKU and adds the XML
   injection (§5.10).
2. **Explicit request field**: Anthropic request has
   `thinking: {type: "enabled", budget_tokens: N}` → handled in
   the request-conversion layer, not the model resolver.

### 8.4 Free tier vs Pro tier

Kiro gates some models to Pro subscribers:
- Pro-only: `claude-opus-4.5`, `claude-opus-4.6`, `claude-opus-4.7`
- Free + Pro: `claude-sonnet-4.5`, `claude-sonnet-4.6`, `claude-haiku-4.5`

Gating enforcement happens server-side. Sending an Opus request from
a free account returns `ValidationException` with a "subscription required"
message. No public documentation; inferred from peer issue reports.

### 8.5 Silent fallback

When Kiroxy receives an unknown model name:
- If it starts with `claude-` → pass through unchanged (best effort)
  (`models.go:162`)
- Otherwise → fall back to `DefaultModel = "claude-sonnet-4.6"` with a
  slog.Warn (`models.go:166`)

Peer jwadow silently falls back with no warning. Peer Quorinex
rejects unknown models with a 400. Kiroxy's compromise (warn + fall
back) matches real claude-code behavior where users sometimes request
non-existent model IDs.

### 8.6 Non-Claude models

Kiro has experimented with DeepSeek, GLM, Minimax, Qwen model support
(confirmed in jwadow/kiro-gateway README and Kiro IDE model picker).
These are gated by region + subscription tier and are not universally
available. Kiroxy doesn't map them explicitly — they'd fall through
the default model fallback.

---

## 9. Token formats

### 9.1 Prefix meaning

| Prefix | Meaning | Flow |
|---|---|---|
| `aor...` | **A**uth **O**Auth **R**efresh? or "auth origin refresh"? Public name unknown | Social (Kiro Desktop) |
| `aoa...` | **A**uth **O**Auth **A**ccess? or "auth origin access"? | Either flow can mint these as access tokens |
| `eyJ...` | JWT (base64url header `{"alg":...,"typ":"JWT"...}`) | Neither — this would be a different system's token |

Kiroxy does NOT decode or validate the JWT structure of these tokens —
they are opaque to the proxy. The only use is:
- Send as `Authorization: Bearer <token>` to upstream
- Store in vault
- Refresh before `expiresAt - 5min`

### 9.2 Token lifetime

- Social: `expiresIn = 3600` seconds typical (1h). Refresh rotation
  is opaque.
- IDC/OIDC: `expiresIn` varies by IDC config, typically 1h also.

Refresh tokens themselves don't document an explicit TTL but revoke
on:
- Account password change / account recovery
- AWS admin action
- `>90 days` idle (reported by peers, not formally verified)
- Concurrent refresh from multiple IPs (suspected; not verified)

### 9.3 JWT introspection (not performed)

Peer `jwadow/kiro-gateway` decodes the JWT payload for logging
purposes. Kiroxy deliberately does not do this — any info leaked to
logs from a decoded token is redactable only by prefix-matching, which
is brittle.

---

## 10. Profile ARN

### 10.1 Structure

```
arn:aws:codewhisperer:<region>:<account_id>:profile/<profile_id>
```

Examples seen in peer captures:
```
arn:aws:codewhisperer:us-east-1:123456789012:profile/EXAMPLE1PROF
```

### 10.2 Where it comes from

- Social flow: returned inline in `profileArn` field of the refresh
  response (§2.1).
- IDC flow: NOT returned. Accounts using IDC/Builder ID lack
  profileArn and route to AmazonQ target (§4.2).

### 10.3 Storage format in vault

Kiroxy parses profile values in two forms
(`internal/auth/credentials_parser.go:59-70`):
- Plain ARN string
- JSON object `{"arn": "...", "profile_name": "..."}` (some kiro-cli
  versions store it wrapped)

The JSON form is robust to future schema extensions.

### 10.4 Workspace org subtlety

**Peer issue** (partial, from BACKLOG): Workspace-org Kiro
subscribers have profileArns that are **shared** across multiple users.
If multiple kiroxy instances authenticate with different Workspace
members' credentials but the same profileArn, request logs are
commingled on the Workspace audit trail. Kiroxy does not de-dup by
profileArn; this is a BACKLOG item ("Workspace profileArn collision"
— see BACKLOG.md).

---

## 11. Quirks & undocumented behavior

### 11.1 Token propagation delay after refresh

After the refresh endpoint returns a new access_token, there is a
**short observable delay (~1-3 sec)** before runtime.kiro.dev accepts
it. Peer evidence: `bhaskoro/kiro-gateway PR #155`.

Kiroxy's retry-on-403 logic
(`internal/kiroclient/client.go:296-312`) handles this implicitly: on
a 403, it refreshes and retries. A fresh refreshed token sometimes gets
rejected once, then succeeds on the retry. The 3-attempt retry budget
absorbs this.

### 11.2 X-Claude-Code-Session-Id requirement

claude-code sends this header; kiroxy accepts it and logs it.
Observation from peer logs: omitting it is tolerated but some Kiro
rate-limit paths appear to use it as an idempotency key. Passing it
through is safe.

### 11.3 Loopback vs remote auth differences

Peer jwadow reports that `127.0.0.1` requests to Kiro runtime endpoints
(not kiroxy, direct) can sometimes bypass normal auth in development
builds. This is not observable from userspace kiroxy and is out of
scope.

### 11.4 EventStream truncation mid-stream

Very rarely, Kiro returns a complete prelude + partial payload and
EOFs. Kiroxy's parser treats this as an error ("truncated prelude: read
X/12 bytes", `frame.go:35`) because the CRC cannot be validated.
Downstream clients see a truncated response. Idle-reader timeout
(`idle_reader.go`, 180s default) protects against the inverse: silent
hangs where headers arrive but no frames do.

### 11.5 200 with non-eventstream Content-Type

See §7.1. The 200 + JSON exception case is the single most common
source of confusion in Kiro-proxy code. Kiroxy's
`isEventStreamContentType()` at `aws_error.go:104` is load-bearing.

### 11.6 conversationId persistence

Kiroxy does NOT send `conversationId` on new conversations — it lets
Kiro mint one and echoes it back via `messageMetadataEvent`. Peer
jwadow does the same; peer Quorinex mints client-side UUIDs and
reuses them, which works but offers no benefit.

### 11.7 Region mismatch errors

Sending a us-east-1-issued token to eu-central-1 upstream, or vice
versa, returns `UnauthorizedException`. Kiroxy carries `region` in
`Credentials` and uses it at `internal/kiroclient/client.go:167`.
Peers sometimes hardcode us-east-1; kiroxy is region-aware.

### 11.8 Refresh token re-rotation timing

Social refresh sometimes returns an empty refreshToken (meaning
"reuse the one you already have"), sometimes returns a NEW
refreshToken. Kiroxy coalesces
(`internal/auth/refresh.go:266,287`) so the stored value is always
non-empty. Pattern observed:
- First refresh after login: usually same refreshToken
- Subsequent refreshes: frequently new refreshToken with rotation
- Peer evidence: Quorinex comments on this in their refresh.go

### 11.9 Token-at-rest encryption

Kiroxy stores access_token and refresh_token **unencrypted** in
`internal/tokenvault/vault.go` SQLite. Peer jwadow stores in
plaintext too. Peer Quorinex supports optional encryption with a
user-supplied key (not default). This is a SECURITY.md audit item.

### 11.10 The "agentContinuationId" mystery

Peer code in `AntiHub-Project/Antigv-plugin` and some older jwadow
versions set `agentContinuationId` on the currentMessage. kiroxy does
NOT set it. Observable behavior: no difference detected. Believed
deprecated or IDE-internal.

---

## 12. Sources

### Primary (kiroxy source)

- `internal/kiroproto/types.go` — all request body struct definitions
- `internal/kiroproto/frame.go` — EventStream binary framing
- `internal/kiroproto/eventstream.go` — event type parsing
- `internal/kiroclient/client.go` — request dispatch, header set
- `internal/kiroclient/aws_error.go` — error classification
- `internal/kiroclient/target.go` — X-Amz-Target selection
- `internal/kiroclient/backoff.go` — retry policy
- `internal/auth/refresh.go` — OIDC refresh
- `internal/auth/refresh_social.go` — social refresh
- `internal/auth/credentials_parser.go` — token/profile parsing
- `internal/reqconv/build_payload.go` — Anthropic → Kiro payload mapping
- `internal/reqconv/cache_points.go` — cache-point placement
- `internal/models/models.go` — model ID resolution

### Peer sources

- `d-kuro/kirocc` commit `5633c47f0d65aaef748728bae1c68160b0ea538d` —
  the base kiroxy derived from; Apache-2.0
- `Quorinex/Kiro-Go` commit `940dc782cb0a9a0d095abc6f407adf21ccc24ae2`
  — pool dispatch pattern; MIT
- `jwadow/kiro-gateway` — Python/FastAPI reference; AGPL-3.0
- `petehsu/KiroProxy` — TypeScript peer; no license
- `hj01857655/kiro-account-manager` — account management patterns
- `AntiHub-Project/Antigv-plugin` — browser plugin peer
- `bhaskoro/kiro-gateway PR #155` — endpoint migration bible

### External

- `https://kiro.dev/docs` — AWS public Kiro docs (sparse on runtime
  API; richer on IDE usage)
- AWS SDK for JavaScript / `aws-sdk-js` — `user-agent` format
  convention
- AWS EventStream binary format spec (internal AWS SDKs)
- `jwadow/kiro-gateway#146` — endpoint migration deadline issue
- `jwadow/kiro-gateway#153` — Write tool truncation bug
- `Quorinex/Kiro-Go PR #37` — contextUsageEvent parsing
- `Quorinex/Kiro-Go PR #40` — thinking-config routing

### Dead-ends and unverifiable claims

Marked in-text as "observed in peer code, not formally documented" or
"inferred from peer issue reports". Examples:
- Regional expansion timing (§1)
- Refresh token TTL (§9.2)
- Non-Claude model gating details (§8.6)
- agentContinuationId purpose (§11.10)

These are unknowns — documented as such rather than guessed.

---

*Document maintained alongside kiroxy source. When the protocol
changes (as it did 2026-05-15 with the endpoint migration), update
this file and cite the new peer evidence.*
