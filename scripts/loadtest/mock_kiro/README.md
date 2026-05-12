# mock_kiro

A protocol-correct mock of the Kiro CodeWhisperer upstream. Emits AWS
EventStream binary frames (CRC-32 validated, big-endian, header-tagged)
matching the shape kiroxy's `internal/kiroproto` parses from the real
`q.us-east-1.amazonaws.com` endpoint.

Used by the load-test harness under `scripts/loadtest/` for isolated
performance testing without burning real Kiro quota, but the binary is
useful on its own for any CodeWhisperer client that wants a local
upstream to point at.

## Build & run

```bash
go build -o mock_kiro .
./mock_kiro --addr :9789 --stream-events 8
# {"status":"ok"} from GET /healthz
```

No deps, stdlib only.

## Flags

| flag | default | purpose |
|---|---|---|
| `--addr` | `:9789` | listen address |
| `--latency-ms` | `0` | fixed latency before first byte |
| `--error-rate` | `0` | 0.0–1.0 probability of 429 ThrottlingException |
| `--stream-events` | `8` | assistantResponseEvent frames per response |
| `--chunk-delay-ms` | `0` | delay between streamed frames |
| `--tokens-in` | `42` | reported input tokens in metadataEvent |
| `--tokens-out` | `64` | reported output tokens in metadataEvent |
| `--text` | `"mock kiroxy response"` | canned response text (split across events) |
| `--fail-after` | `0` | return 500 after N successes (0=off); for breaker tests |
| `--log` | `false` | log each request line |

Env-var aliases: `MOCK_KIRO_ADDR`, `MOCK_KIRO_LATENCY_MS`,
`MOCK_KIRO_ERROR_RATE`, `MOCK_KIRO_STREAM_EVENTS`, `MOCK_KIRO_CHUNK_DELAY_MS`.

## Endpoints

| method | path | purpose |
|---|---|---|
| POST | `/` | CodeWhisperer-shaped entry point |
| POST | `/generate` | alias of `/` for ad-hoc testing |
| GET | `/healthz` | `{"status":"ok"}` probe |
| GET | `/stats` | JSON counters: `{requests, errors, inflight}` |

## Response shape

One successful `POST /` yields the following event sequence (each event
is one AWS EventStream frame):

1. `initial-response` — `{"conversationId": "mock-conv-<n>"}`
2. `assistantResponseEvent` × N — `{"content": "<chunk>", "modelId": "mock-sonnet"}`
3. `metadataEvent` — full tokenUsage breakdown
4. `messageMetadataEvent` — `{"conversationId", "utteranceId"}`

The total frame count per response is `N + 3` (default: 11).

## Protocol notes

### Frame layout

```
+------------------+------------------+-----------+
| totalLen (u32 BE)| hdrLen   (u32 BE)| prelude   |
|                                     | CRC (u32) |
+------------------+------------------+-----------+
| headers block (variable)                        |
+-------------------------------------------------+
| payload (variable, typically JSON)              |
+-------------------------------------------------+
| message CRC (u32 BE)                            |
+-------------------------------------------------+
```

- `preludeCRC` covers bytes 0..7 only.
- `messageCRC` covers bytes 0..(totalLen-4) — the entire frame minus the
  CRC slot itself.
- Both CRCs use `crc32.IEEETable` (polynomial 0x04C11DB7, reversed).

### Header format

Each header: `nameLen(u8) | name | valueType(u8) | value`. For string
values (type 7), the value has a `u16 BE` length prefix. The mock emits
three headers per event:

- `:message-type` = `"event"`
- `:event-type`   = `"initial-response"` | `"assistantResponseEvent"` | ...
- `:content-type` = `"application/json"`

Other value types (bool, int, timestamp, uuid) exist in the AWS spec but
are not used by any real Kiro response.

### Validation

Output frames were validated against kiroxy's production parser
(`local/kiroxy/internal/kiroproto.ParseStream`) and decoded cleanly —
CRCs match, headers parse, event types round-trip.

## Failure-mode simulation

```bash
# Throttling — 10% of requests return 429
./mock_kiro --error-rate 0.10

# Upstream down — after 5 successes, always 500
./mock_kiro --fail-after 5

# Slow upstream — 200ms latency + 5ms between chunks
./mock_kiro --latency-ms 200 --chunk-delay-ms 5

# Hostile — all three combined
./mock_kiro --latency-ms 100 --error-rate 0.05 --chunk-delay-ms 10
```

Useful for exercising kiroxy's cooldown / strike / breaker logic without
triggering real AWS abuse detection.

## Pointing kiroxy at the mock

kiroxy's `internal/kiroclient` supports `WithBaseURL()` but exposes no
runtime env var to set it. So currently the mock is useful **directly**
(harness hits it in `--mode kiro`) but not transparently through kiroxy's
`/v1/messages` path.

To wire kiroxy through the mock you'd need a debug build that calls:

```go
kiroclient.NewHTTPClient(
    kiroclient.WithBaseURL("http://127.0.0.1:9789"),
    // ... other opts
)
```

This is a backlog item (`KIROXY_UPSTREAM_URL` env var). Track it in the
repo's `BACKLOG.md`.

## Limitations

- **No SigV4.** Kiro uses bearer tokens, not SigV4 — mock matches this.
  But it also does not verify the `Authorization: Bearer <token>` value.
- **No rate limiting.** Use `--error-rate` to simulate upstream throttle
  responses; the mock does not track per-client quotas.
- **No request validation.** Any POST body is accepted; the mock logs
  unexpected `Content-Type` / `X-Amz-Target` values but does not reject
  them. Real Kiro rejects malformed requests with
  ValidationException — the mock's permissiveness is deliberate for
  harness simplicity.
- **No TLS.** Plain HTTP only. Front with stunnel / nginx if you need
  TLS for a particular test.
