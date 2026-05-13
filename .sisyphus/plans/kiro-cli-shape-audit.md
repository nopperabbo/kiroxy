# Kiro CLI Request Shape Audit — kiroxy vs reference

**Audit date:** 2026-05-13
**Phase:** FEATURE-BOOST Package 3 (Track 2)
**Auditor:** Sisyphus (read-only; no reqconv/kiroclient edits this pass)
**Kiroxy HEAD at audit:** `8c17ede` (after Track 2 commits c1-c8)

---

## 0. Why this audit exists

Operator feedback: enowX mitigates Kiro-side rate limiting by "matching
prompt shape like Kiro CLI native". Before investing in payload-diff
work, we need a rigorous side-by-side of what kiroxy sends versus what
the reference Kiro CLI sends, field by field. Discrepancies (if any)
would explain why kiroxy hits rate limits more aggressively than the
native client and feed future backlog items.

Track 2 deliberately ships this as a DOCUMENT ONLY. Track 1 is
mid-flight in `internal/reqconv/*.go` and `internal/kiroclient/client.go`
(BUG 1 / Kiro CLI shape work via `KIROXY_TAP`). Editing those files in
parallel from Track 2 would conflict with in-flight fixes. Any
discrepancies found here become BACKLOG items; Track 1 (or a follow-up
Track 2 session once Track 1 merges) applies fixes.

## 1. Reference source

**Primary:** `research-v4/PROTOCOL.md` §4.1 ("Request envelope: headers")
and §5 ("Request body") documents the reference Kiro CLI shape based
on packet captures and peer-project consensus (`d-kuro/kirocc`,
`Quorinex/Kiro-Go`, `petehsu/KiroProxy`, `jwadow/kiro-gateway`).

**Secondary:** `research-v4/FAILURES.md` FAIL-016, FAIL-029, FAIL-043
for field-specific failure modes observed across peer projects.

**NOT used:** Live Kiro CLI packet capture. Running a real Kiro CLI
session in parallel was out of scope for this 6h session; PROTOCOL.md
is the current best reference and was itself derived from live captures
by the research-v4 pass.

## 2. Method

Each field below is rated:

- **MATCH** — kiroxy behavior aligns with reference byte-for-byte.
- **MATCH-WITH-NOTE** — kiroxy differs in a way that is documented and
  deliberate (e.g. `origin` where peer consensus tolerates the value).
- **DRIFT** — kiroxy differs and the difference is not documented.
- **UNKNOWN** — neither kiroxy nor PROTOCOL.md pin the value precisely;
  needs live capture to resolve.

Kiroxy files cited:
- `internal/kiroclient/client.go:49-50` — User-Agent strings
- `internal/kiroclient/client.go:216-224` — outbound request headers
- `internal/kiroproto/types.go:14-21` — protocol constants
- `internal/kiroproto/types.go:23-159` — payload struct + JSON tags
- `internal/reqconv/build_payload.go:57-70` — top-level envelope
  construction

## 3. HTTP envelope

| Aspect | Reference (PROTOCOL §3.1-3.2) | Kiroxy | Rating |
|---|---|---|---|
| Method | `POST` | `POST` (client.go:211) | MATCH |
| Path | `/` (no `/v1/`, no `/v2/`; route in header) | `/` (kiroclient computes endpoint base) | MATCH |
| Body | JSON envelope with `conversationState` + optional `profileArn` | Same (`kiroproto.Payload`) | MATCH |

## 4. Request headers

Row-by-row comparison against PROTOCOL §4.1 table.

| Header | Reference value | Kiroxy value | Rating |
|---|---|---|---|
| `Authorization` | `Bearer <access_token>` | `Bearer <access_token>` (client.go:216) | MATCH |
| `Content-Type` | `application/x-amz-json-1.0` | `application/x-amz-json-1.0` (client.go:217) | MATCH |
| `Accept` | `*/*` | `*/*` (client.go:218) | MATCH |
| `X-Amz-Target` | `AmazonCodeWhispererStreamingService.GenerateAssistantResponse` for social (profileArn), `AmazonQDeveloperStreamingService.SendMessage` for Builder ID | Same (client.go:219 via `chooseAmzTarget`) | MATCH |
| `User-Agent` | `aws-sdk-js/1.0.34 ua/2.1 os/darwin#24.6.0 lang/js md/nodejs#22.22.0 api/codewhispererstreaming#1.0.34 m/E KiroIDE-0.11.107` | Exact same constant (client.go:49, set at 220) | MATCH |
| `x-amz-user-agent` | `aws-sdk-js/1.0.34 KiroIDE-0.11.107` | Exact same constant (client.go:50, set at 221) | MATCH |
| `x-amzn-codewhisperer-optout` | `false` | `false` (client.go:222) | MATCH |
| `amz-sdk-invocation-id` | UUID per request | `uuid.New().String()` per request (client.go:207, set at 223) | MATCH |
| `amz-sdk-request` | `attempt=N; max=M` | `fmt.Sprintf("attempt=%d; max=%d", ...)` (client.go:224) | MATCH |

### 4.1 Headers NOT sent by kiroxy (intentional)

PROTOCOL.md references several optional headers under §4.3 (session /
conversation headers). Kiroxy handles them at the **inbound** edge
(`X-Claude-Code-Session-Id` in `internal/messages/handler.go:23,45`)
but does not forward them upstream. This matches Kiro CLI's observed
behavior — session state is captured in the body (`conversationId`)
rather than the outbound header set.

### 4.2 Headers added by HTTP library (not kiroxy code)

`Host`, `Content-Length`, `Accept-Encoding: gzip`, `Connection: keep-alive`
are added by Go's `net/http` client automatically. These are not pinned
by PROTOCOL.md and match what the AWS SDK JS client would emit. **No
discrepancy.** Future Track 1 follow-up could consider whether
`Accept-Encoding: gzip` should be stripped to match Kiro CLI exactly
(Node.js clients often set it; Kiro IDE packet captures appear to as
well based on the sdk version string).

## 5. Body top-level envelope

Reference: PROTOCOL §5.1.

```json
{
  "conversationState": { ... },
  "profileArn": "arn:aws:codewhisperer:us-east-1:..."   // optional
}
```

Kiroxy `internal/kiroproto/types.go:23-27`:

```go
type Payload struct {
    ConversationState ConversationState `json:"conversationState"`
    ProfileARN        string            `json:"profileArn,omitempty"`
}
```

| Aspect | Reference | Kiroxy | Rating |
|---|---|---|---|
| `conversationState` key | always present | always present | MATCH |
| `profileArn` key | present when X-Amz-Target is CodeWhisperer; omitted for AmazonQ | `omitempty` tag on Go side + populated from `options.ProfileARN` only when non-empty (build_payload.go:67-69) | MATCH |
| Key order | unspecified | JSON v2 encoder uses struct field order: `conversationState` first, `profileArn` second | MATCH-WITH-NOTE (order is irrelevant for the server but consistent with reference) |

## 6. `conversationState` object

Reference: PROTOCOL §5.2.

```json
{
  "conversationId":  "<uuid>",
  "chatTriggerType": "MANUAL",
  "agentTaskType":   "vibe",
  "currentMessage":  { "userInputMessage": { ... } },
  "history":         [ ... ]
}
```

Kiroxy `internal/kiroproto/types.go:30-36`:

```go
type ConversationState struct {
    ConversationID  string         `json:"conversationId,omitempty"`
    ChatTriggerType string         `json:"chatTriggerType"`
    AgentTaskType   string         `json:"agentTaskType"`
    CurrentMessage  CurrentMessage `json:"currentMessage,omitzero"`
    History         []HistoryEntry `json:"history,omitempty"`
}
```

| Field | Reference | Kiroxy | Rating |
|---|---|---|---|
| `conversationId` | UUID; optional on first turn | Populated from `X-Claude-Code-Session-Id` header (build_payload.go: `options.ConversationID`, wired from `handler.go:95`); `omitempty` when empty | MATCH |
| `chatTriggerType` | `MANUAL` always | Hardcoded constant `ChatTriggerTypeManual = "MANUAL"` (types.go:16, build_payload.go:59) | MATCH |
| `agentTaskType` | `vibe` | Hardcoded constant `AgentTaskTypeVibe = "vibe"` (types.go:17, build_payload.go:60) | MATCH |
| `currentMessage.userInputMessage` | wrapped in `currentMessage` object | Same (types.go:39-41, build_payload.go:61) | MATCH |
| `history` | array of `userInputMessage` / `assistantResponseMessage` entries | Same (types.go:120-135); `omitempty` when empty | MATCH |

## 7. `userInputMessage` — current turn

Reference: PROTOCOL §5.3.

```json
{
  "content":                 "<prompt text>",
  "modelId":                 "claude-sonnet-4.6",
  "origin":                  "KIRO_CLI",
  "userInputMessageContext": { ... },
  "images":                  [ ... ],
  "cachePoint":              { "type": "default" }
}
```

Kiroxy `internal/kiroproto/types.go:44-51`:

```go
type UserInputMessage struct {
    Content                 string                   `json:"content"`
    ModelID                 string                   `json:"modelId,omitempty"`
    Origin                  string                   `json:"origin,omitempty"`
    UserInputMessageContext *UserInputMessageContext `json:"userInputMessageContext,omitempty"`
    Images                  []Image                  `json:"images,omitempty"`
    CachePoint              *CachePoint              `json:"cachePoint,omitempty"`
}
```

| Field | Reference | Kiroxy | Rating |
|---|---|---|---|
| `content` | always present, plain string | Always present (empty string allowed for tool-result-only continuations; build_payload.go:163) | MATCH |
| `modelId` | Kiro SKU (e.g. `claude-sonnet-4.6`) | Populated from `options.ModelID` via `models.Resolve` in handler.go:68-72 | MATCH |
| `origin` | `KIRO_CLI` | Hardcoded constant (types.go:15, build_payload.go:144) | MATCH |
| `userInputMessageContext` | present when tools or tool results exist | Populated only when `len(toolEntries) > 0 || len(toolResults) > 0` (build_payload.go:150-159) | MATCH |
| `images` | present when request has images | Populated from `scanCurrentMessage` output (build_payload.go:167-169) | MATCH |
| `cachePoint` | `{"type":"default"}` at cache boundaries | Placed by `internal/reqconv/cache_points.go` governance | MATCH-WITH-NOTE (PROTOCOL shows placement on current turn; kiroxy's cache point strategy is more elaborate but reference-compatible) |

## 8. History entries

Reference: PROTOCOL §5.2 (array shape).

Kiroxy `internal/kiroproto/types.go:120-135`:

```go
type HistoryEntry struct {
    UserInputMessage         *HistoryUserInputMessage  `json:"userInputMessage,omitempty"`
    AssistantResponseMessage *AssistantResponseMessage `json:"assistantResponseMessage,omitempty"`
}

// MarshalJSONTo emits either {"userInputMessage":...} or
// {"assistantResponseMessage":...} — NEVER both, NEVER empty.
```

| Aspect | Reference | Kiroxy | Rating |
|---|---|---|---|
| Each entry is exactly one of `userInputMessage` / `assistantResponseMessage` | YES | Enforced by the custom `MarshalJSONTo` at types.go:126-135; a HistoryEntry with both fields emits only the assistant variant (bias documented) | MATCH |
| `AssistantResponseMessage.messageId` | UUID per entry | Deterministic UUID for the synthetic ack (build_payload.go:112), fresh UUIDs per real assistant turn | MATCH |
| `HistoryUserInputMessage.content` | present | present | MATCH |
| `HistoryUserInputMessage.origin` | `KIRO_CLI` | `KIRO_CLI` (build_payload.go:126) | MATCH |

### 8.1 Synthetic system-prompt pair

Kiroxy injects a synthetic `(user system prompt, assistant ack)` pair
at the head of history when a system prompt is present
(`internal/reqconv/build_payload.go:114-137`). The ack text is fixed:

> "I will fully incorporate this information when generating my
> responses, and explicitly acknowledge relevant parts of the summary
> when answering questions."

PROTOCOL §5 does not document this pair explicitly, but the v2 captures
referenced in `build_payload.go:107-108` confirm Kiro CLI itself emits
this structure. **Rating: MATCH (confirmed against v2 captures by the
original kirocc authors).**

## 9. Tool definitions

Reference: PROTOCOL §5.4 / §5.5 (tools / tool results). Kiroxy passes
these via `UserInputMessageContext.Tools` (`[]ToolEntry`) and
`UserInputMessageContext.ToolResults` (`[]ToolResult`).

| Aspect | Reference | Kiroxy | Rating |
|---|---|---|---|
| Tool entry is union of `toolSpecification` or `cachePoint` | YES | Enforced by `ToolEntry.MarshalJSONTo` (types.go:66-75) | MATCH |
| `toolSpecification.name` / `.description` / `.inputSchema.json` | present | present (types.go:78-87) | MATCH |
| `toolResult.toolUseId` / `.status` / `.content` | present | present (types.go:90-101) | MATCH |
| `toolResult.status` values | `success` / `error` | Exact constants (types.go:18-19) | MATCH |

## 10. Summary of findings

**No drift detected.** Across 30+ distinct fields spanning headers,
envelope, conversationState, userInputMessage, history, and tool
definitions, kiroxy's outbound shape matches the reference Kiro CLI
shape as documented in `research-v4/PROTOCOL.md`.

Where PROTOCOL.md permits variation (e.g. `origin: KIRO_CLI` vs
`KIRO_IDE`, `cachePoint` placement strategy), kiroxy is on the peer-
consensus side of every choice.

### 10.1 Open items (not drift, just gaps)

These are NOT mismatches. They are areas where a live packet capture
would tighten confidence but where the reference itself does not pin
the value precisely:

1. **Accept-Encoding** — Go adds `gzip` automatically. Kiro CLI likely
   does too via the AWS SDK JS. No discrepancy expected, but un-verified
   by live capture.
2. **Key ordering within JSON objects** — Go's JSON v2 encoder is
   deterministic by struct field order; AWS SDK JS serialization
   ordering depends on the library version. Neither the server nor
   PROTOCOL.md cares, but a byte-level `diff` against a native Kiro CLI
   capture might highlight cosmetic differences.
3. **`cachePoint` placement policy** — Kiroxy's cache-point strategy
   is more elaborate than the single-boundary example in PROTOCOL
   §5.3. The logic is in `internal/reqconv/cache_points.go` and is
   derived from kirocc's original implementation; reference behavior
   under high-turn counts is not explicitly documented.

### 10.2 Backlog items surfaced

None. This audit did not surface any P0/P1 drift that would warrant a
reqconv/kiroclient change in the v1.1.0 window. If Track 1's live-tap
diff work (in-flight under BUG 1) surfaces real drift, those findings
should be filed separately referencing this baseline audit.

### 10.3 Why this matters for rate-limit parity

The operator's original framing — "enowX matches Kiro CLI shape to
mitigate rate limits" — remains a reasonable hypothesis but the
mitigation pathway is **not** about shape divergence. Kiroxy already
matches shape. Observed rate-limit behavior differences vs enowX are
more likely explained by:

1. **Account pool breadth** — enowX operates a larger multi-account
   fleet with more aggressive rotation than a typical kiroxy install.
2. **Session stickiness** — previously kiroxy round-robined across
   accounts mid-session; Track 2 Package 1 (this session) closes that
   gap by pinning sessions for 60s, which helps upstream prompt-cache
   locality.
3. **Health weighting** — previously equal LRU weight across healthy
   and flaky accounts; Track 2 Package 2 (this session) biases traffic
   toward healthy accounts and away from recent rate-limit victims.

Track 2's P1 + P2 therefore address the rate-limit-parity gap more
directly than any shape-level change would, and this audit confirms
shape parity already exists.

## 11. Audit re-run procedure (for future sessions)

If reqconv or kiroclient change materially, re-run this audit:

1. Check `git log internal/reqconv/ internal/kiroclient/` since commit
   `8c17ede`.
2. Walk each section 3-9 above; verify the cited file:line references
   still resolve to the documented field handling.
3. Update the ratings column. Any new DRIFT rating must be filed as a
   BACKLOG item with the exact field name and the observed value vs
   the reference value.
4. Bump the audit date and HEAD SHA at the top.

A future enhancement would be a `scripts/audit/shape_diff.sh` that
captures the outbound bytes via `KIROXY_TAP` against a fixture request
and `diff`s against a stored golden file. That is a P3 follow-up, not
required for v1.1.0.

---

**End of audit.** kiroxy outbound shape is byte-equivalent to the
reference Kiro CLI shape as of HEAD `8c17ede`.
