# kiroxy

A single-user, self-hosted proxy that exposes your Kiro IDE subscription (Amazon Q Developer / AWS CodeWhisperer) as an **Anthropic Messages API** endpoint. Point your Claude Code, Cursor, or any Anthropic-compatible client at kiroxy, and it forwards requests to Kiro using your own credentials.

**Status:** v0.1.0-mvp — personal-use, MIT-licensed. See `BUILD_LOG.md` for the construction log.

---

## Five-minute quickstart

### 1. Prereqs

- **Go 1.26+** (kiroxy uses `encoding/json/v2` via `GOEXPERIMENT=jsonv2`)
- **A Kiro account** — either:
  - the `kiro-cli` tool logged in (kiroxy can read its SQLite credentials directly), or
  - you're running Kiro IDE and have `~/.aws/sso/cache/kiro-auth-token.json`, or
  - you'll use `kiroxy add-account` to go through the AWS Builder ID device-code flow (M9, coming soon)

### 2. Build

```bash
git clone <your kiroxy remote>
cd kiroxy
make build          # or: GOEXPERIMENT=jsonv2 go build -o kiroxy ./cmd/kiroxy
```

### 3. Configure

Pick **one** of two credential sources:

**Option A — you already have kiro-cli installed and logged in** (easiest):

```bash
export KIROXY_KIRO_DB_PATH="$HOME/Library/Application Support/kiro-cli/data.sqlite3"   # macOS
# or:  export KIROXY_KIRO_DB_PATH="$HOME/.local/share/kiro-cli/data.sqlite3"          # Linux
```

kiroxy will read and refresh tokens from the kiro-cli database directly.

**Option B — managed token vault** (default, needs M9 `add-account` to be useful):

```bash
# Creates ~/.kiroxy/tokens.db with 0600 perms on first run.
# Use 'kiroxy add-account' to register Kiro accounts (M9, not yet shipped).
unset KIROXY_KIRO_DB_PATH
```

Optional but recommended:

```bash
export KIROXY_API_KEY="$(openssl rand -hex 32)"
```

### 4. Run

```bash
./kiroxy serve
# JSON log on stderr:
# {"time":"...","level":"INFO","msg":"kiroxy listening","version":"0.1.0-mvp","addr":"http://127.0.0.1:8787"}
```

### 5. First request

Non-streaming:

```bash
curl -sS http://127.0.0.1:8787/v1/messages \
  -H "X-Api-Key: $KIROXY_API_KEY" \
  -H "X-Claude-Code-Session-Id: $(uuidgen)" \
  -H "Content-Type: application/json" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model":"claude-sonnet-4-5",
    "max_tokens":1024,
    "messages":[{"role":"user","content":"Reply with just the word: kiroxy"}]
  }'
```

Streaming:

```bash
curl -sN http://127.0.0.1:8787/v1/messages \
  -H "X-Api-Key: $KIROXY_API_KEY" \
  -H "X-Claude-Code-Session-Id: $(uuidgen)" \
  -H "Content-Type: application/json" \
  -d '{
    "model":"claude-sonnet-4-5",
    "max_tokens":1024,
    "stream":true,
    "messages":[{"role":"user","content":"Write a haiku about proxies."}]
  }'
# Emits: event: message_start, content_block_delta ..., event: message_stop
```

### 6. Use with Claude Code

```bash
export ANTHROPIC_BASE_URL=http://127.0.0.1:8787
export ANTHROPIC_AUTH_TOKEN=$KIROXY_API_KEY   # any non-empty value if KIROXY_API_KEY unset
claude
```

---

## Architecture

```
                     KIROXY_API_KEY                                 KIROXY_KIRO_DB_PATH
                          |                                              (or vault)
                          v                                                  |
  Claude Code /-+   +-------------------+   +--------------+   +------+     v
  Cursor         +->|  /v1/messages     |-->| reqconv      |-->| kiro +--> Kiro IDE
  OpenAI SDK     |  |  /v1/messages/    |   | (anthropic - |   |client|   (CodeWhisperer
  your own curl /+  |    count_tokens   |   |  Kiro payload)|  |      |    generateAssistant
                    |                   |   +--------------+   +------+    Response)
                    |  auth MW          |          ^                           |
                    |  log MW (ULID)    |          |                           |
                    |  /healthz         |          |                           v
                    |  /readyz          |          |                     +-------------+
                    +----------+--------+          |                     | respconv    |
                               |                   |                     | (Kiro SSE - |
                               v                   |                     |  Anthropic  |
                     +----------------+            |                     |  SSE)       |
                     | pool.TokenGetter|           |                     +----+--------+
                     | (LRU selection  |           |                          |
                     |  + cooldowns)   |-----------+                          |
                     +----------+------+                                      v
                                |                                       your client
                                v                                    (streamed chunks)
                     +-----------------+
                     | tokenvault      |
                     | (SQLite +       |
                     |  gen-lock OAuth |
                     |  refresh)       |
                     +-----------------+
```

Key patterns donated from:
- **d-kuro/kirocc** (Apache-2.0) — everything in `internal/{reqconv, respconv, kiroproto, kiroclient, anthropic, logging, httpx, tokencount, toolsearch, tracing, messages, models, testutil, auth}`. See `NOTICE`.
- **Quorinex/Kiro-Go** (MIT) — `internal/pool/pool.go` (adapted to LRU).
- **kadangkesel/hexos** (MIT) — `internal/tokenvault/vault.go` (ported from TypeScript, generation-lock pattern preserved).

---

## Environment variables

| Variable | Default | Purpose |
|---|---|---|
| `KIROXY_API_KEY` | (empty = open mode) | Required for clients; empty disables inbound auth (only safe on loopback) |
| `KIROXY_BIND` | `127.0.0.1` | Interface. Set `0.0.0.0` only behind a TLS reverse proxy |
| `KIROXY_PORT` | `8787` | TCP port |
| `KIROXY_DB_PATH` | `~/.kiroxy/tokens.db` | Managed-vault SQLite path (mode 0600 enforced) |
| `KIROXY_KIRO_DB_PATH` | (empty) | If set, read creds from this kiro-cli SQLite DB instead of the managed vault |
| `KIROXY_KIRO_REGION` | `us-east-1` | AWS region for the upstream |
| `KIROXY_LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error` |
| `KIROXY_SHUTDOWN_TIMEOUT` | `30` | Seconds to wait for in-flight SSE to drain on SIGTERM |

---

## Endpoints

| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/healthz` | bypass | Liveness JSON `{"status":"ok","version":...}` |
| GET | `/readyz`  | required | Readiness JSON; 503 if vault/pool down |
| POST | `/v1/messages` | required | Anthropic Messages API (streaming + non-streaming) |
| POST | `/v1/messages/count_tokens` | required | tiktoken-based token count |

Pass API key as either `X-Api-Key: <key>` or `Authorization: Bearer <key>`.

Every response carries `X-Request-Id`; clients may set their own via the same header.

---

## Troubleshooting

### `POST /v1/messages -> 401 authentication_error`

You haven't added an account yet. Either set `KIROXY_KIRO_DB_PATH` to your kiro-cli database, or wait for M9's `kiroxy add-account` subcommand.

### `POST /v1/messages -> 401 missing_api_key`

You set `KIROXY_API_KEY` on the server but didn't send `X-Api-Key` (or `Authorization: Bearer`) from the client. Either unset `KIROXY_API_KEY` on the server (safe only on loopback) or pass the key on every request.

### SSE chunks arrive all at once

A reverse proxy in front of kiroxy may be buffering. Disable proxy buffering:
- **nginx**: `proxy_buffering off;`
- **Caddy**: `reverse_proxy { flush_interval -1 }`
- **Cloudflare**: enable "Early Hints" on the route, or use a worker

### `go build` fails with `encoding/json/v2: build constraints exclude all Go files`

Missing `GOEXPERIMENT=jsonv2`. Use `make build` (which sets it for you) or `export GOEXPERIMENT=jsonv2`.

### Running multiple kiroxy instances from the same vault

Safe. `internal/tokenvault` uses generation-locked OAuth refresh (see M4 in `BUILD_LOG.md`). At most one successful upstream refresh per (provider, account) at any instant.

### AWS suspended my Kiro account

Multi-account pooling against consumer Builder IDs can trigger abuse detection. Personal use only; 1-3 accounts is fine, keep request cadence reasonable.

---

## Build and test

```bash
make build          # single binary at ./kiroxy
make test           # go test ./...
make gate           # build + vet + fmt + test \u2014 required before commits
make test-race      # race-mode test run
```

Pins `GOEXPERIMENT=jsonv2` automatically.

---

## Licensing and attribution

- **kiroxy** itself is MIT (see `LICENSE`).
- **NOTICE** enumerates donor projects, pinned commit SHAs, and per-donor licenses.
- Every ported file carries a header comment citing its origin.
- Personal, non-distributed use: AGPL contamination from any reference-only material (not present in this repo) does not attach.

---

## Links

- **Research**: `../research/` — 170 KB of repo evaluations, comparison tables, extraction cookbook, recommendation.
- **Build plan**: `../BUILD_PLAN.md` — the milestone decomposition.
- **Build log**: `./BUILD_LOG.md` — append-only record of each milestone's gate.
- **Backlog**: `./BACKLOG.md` — Phase 2 and hygiene items.
