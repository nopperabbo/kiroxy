# kiroxy Troubleshooting

Operator-facing diagnostics. If kiroxy misbehaves, start at the top and
work down. Each symptom links to the page it came from.

Last reviewed: Phase I (2026-05-12).

---

## Before anything else

1. **Version + commit**
   ```bash
   kiroxy version
   ```
   Make sure you're running the binary you think you are. `version`
   prints the `-ldflags -X main.version` stamp.

2. **Local build/test gate**
   ```bash
   make gate      # fmt + vet + build + test
   ```
   Green? Toolchain + module + test suite are healthy.

3. **Health + readiness**
   ```bash
   curl -s http://127.0.0.1:8787/healthz | jq .
   curl -s -H "X-Api-Key: $KIROXY_API_KEY" \
     http://127.0.0.1:8787/readyz | jq .
   ```
   `/healthz` always returns 200 when the server is up. `/readyz`
   returns 503 + a JSON error body when the vault or pool is down.

4. **Accounts**
   ```bash
   kiroxy list-accounts
   ```
   Confirm the account you expect is in the pool, and is `active`
   rather than `failed` / `disabled`.

---

## Common errors

| Symptom | Likely cause | Diagnostic | Fix |
|---|---|---|---|
| 401 on `/v1/messages` (inbound) | API key mismatch between client and server | `echo $KIROXY_API_KEY` on both sides; check `X-Api-Key` is sent | Align the values; or unset `KIROXY_API_KEY` on the server and keep loopback-only |
| 401 `missing_api_key` on `/v1/messages` | Server requires key, client omitted it | Check `Authorization` or `X-Api-Key` header | Send the key on every request |
| 502 + "profileArn required" | Account missing `profileArn` in vault metadata | `kiroxy list-accounts` shows `profileArn=<empty>` | Re-import via `import-accounts-json` (Desktop-flow token ships the profileArn) |
| 403 `UnauthorizedException` upstream | `refresh_token` revoked server-side | `kiroxy debug-refresh <account>` prints the refresh outcome | Re-onboard the account from the sidecar (`tools/onboard/`) |
| Model silently returns Sonnet when Opus requested | Model-ID resolver fell back to Sonnet | `grep "non-claude model" <logs>` | Use a canonical Claude API ID (`claude-opus-4-7`, etc.) or pick from `kiroxy opencode-config` output |
| Pool empty / "no accounts available" | All accounts in `failed` status | `kiroxy list-accounts` | Manually re-enable via re-import, or re-onboard |
| Slow first request | Cold refresh | Check logs for a refresh event | Normal; subsequent requests hit the cached token |
| SSE stream cuts mid-response | Upstream or network | `kiroxy` stderr + client logs | Retry; check upstream status; inspect for proxy buffering in front of kiroxy |
| Docker container unhealthy | Healthcheck probe failing | `docker logs kiroxy`, `docker inspect kiroxy` | Confirm `KIROXY_BIND=0.0.0.0`, `KIROXY_PORT=8787` match the exposed port |

---

## Per-error deep dives

### `POST /v1/messages -> 401 authentication_error`

You haven't added an account yet, or every account in the pool has
refresh_token trouble.

Diagnostic:
```bash
kiroxy list-accounts
# expect at least one account with status="active"
```

Fix options:
- Use the bundled Desktop-flow onboarder (`tools/onboard/`):
  `python3 tools/onboard/onboard.py --email you@example.com`
  then `kiroxy import-accounts-json --file kiro_tokens.json`.
- Or point at an existing `kiro-cli` install:
  `export KIROXY_KIRO_DB_PATH="$HOME/Library/Application Support/kiro-cli/data.sqlite3"`
  and restart kiroxy.

### `POST /v1/messages -> 401 missing_api_key`

`KIROXY_API_KEY` is set on the server but the client did not include
`X-Api-Key` (or `Authorization: Bearer`).

Fix options:
- Either unset `KIROXY_API_KEY` on the server (only safe when
  `KIROXY_BIND=127.0.0.1`).
- Or pass the key on every client request.

For `claude` CLI: `ANTHROPIC_AUTH_TOKEN=$KIROXY_API_KEY`.

For opencode: see `docs/OPENCODE.md` for the `api_key` field.

### `502 ... profileArn is required`

Your account was added from the old triplet flow or Builder ID
device-code flow and has no `profileArn` in `Bundle.Metadata`. The
Desktop-flow Kiro upstream (CodeWhisperer) requires this ARN.

Diagnostic:
```bash
kiroxy list-accounts --verbose  # or: sqlite3 ~/.kiroxy/tokens.db \
  'select connection_id, substr(metadata, 1, 120) from bundles;'
```

Fix: re-onboard via the Desktop-flow path. `tools/onboard/` emits
`kiro_tokens.json` with `profile_arn` set. Import via
`kiroxy import-accounts-json --file kiro_tokens.json`.

### `403 UnauthorizedException` from upstream

The refresh token has been revoked server-side (password change,
account suspension, client-ID rotation). Refresh will keep failing
until you get a fresh one.

Diagnostic:
```bash
kiroxy debug-refresh <connection_id>
```
Look for `code: NotAuthorizedException` or `code: InvalidGrantException`.

Fix: re-onboard the account. Desktop-flow refresh tokens live for
weeks at most.

### Model silently returns Sonnet when Opus requested

`internal/models` has a fallback: unknown model names get rewritten to
`claude-sonnet-4-6`. This is intentional (avoids 400 errors for the
typo case) but silent.

Diagnostic:
```
# look in kiroxy's logs for:
level=WARN  msg="non-claude model requested, falling back" \
  requested="kiro/opus-claude4" using="claude-sonnet-4-6"
```

Fix: use canonical IDs. The resolver-verified 7 are emitted by
`kiroxy opencode-config -output -`; copy that list into your client
config. See `docs/OPENCODE.md`.

### Pool empty / "no accounts available"

Every account is in `failed` state (3 strikes). Happens when every
upstream call is 5xx for a while.

Diagnostic:
```bash
kiroxy list-accounts
# status="failed" for all rows
```

Fix options:
- Wait out a transient Kiro outage; re-enable via re-import.
- Re-onboard if the token family is revoked.
- Check your IP/region: AWS upstream is sensitive to region mismatch.

### Slow first request

"First request after start takes 2+ seconds" — that's the initial
refresh. The vault's cached access_token ExpiresAt was past; kiroxy
synchronously refreshed before forwarding.

Diagnostic: logs show `msg="refreshing expired bundle"` immediately
before the upstream call.

Fix: nothing to fix; subsequent requests use the cached token. If
every first request is slow (multiple cold starts per day), lower
`KIROXY_SHUTDOWN_TIMEOUT` or use a process supervisor that keeps
kiroxy running.

### SSE stream cuts mid-response

Usually a reverse proxy in front of kiroxy buffering the response.
See the SSE note in `README.md` for proxy-specific flush settings.

Diagnostic:
- `curl -N` directly against kiroxy (bypass your proxy). If it
  streams cleanly, the proxy is the issue.
- Check the upstream: `kiroxy debug-refresh` won't help here;
  look at the raw `kiroxy` logs for `msg="upstream stream closed"`.

Fix:
- **nginx:** `proxy_buffering off;`
- **Caddy:** `reverse_proxy { flush_interval -1 }`
- **Cloudflare:** cannot be fully disabled; consider a Worker or a
  direct tunnel.

### Docker container unhealthy

Container starts but the orchestrator marks it unhealthy.

Diagnostic:
```bash
docker logs kiroxy
docker inspect --format='{{json .State.Health}}' kiroxy | jq .
```

Likely causes:
- `KIROXY_BIND` set to `127.0.0.1` inside the container (should be
  `0.0.0.0` — the network namespace is the boundary).
- `KIROXY_PORT` changed but not in `EXPOSE` / `ports:`.
- Vault permissions: the distroless container runs as UID 65532; if
  you bind-mount a host directory, make sure it's writeable by that
  UID (`chown 65532:65532 /host/path` or use a named volume).

### `go build` fails with `encoding/json/v2: build constraints exclude`

You're missing `GOEXPERIMENT=jsonv2`.

Fix:
```bash
make build   # Makefile pins GOEXPERIMENT=jsonv2
# or:
GOEXPERIMENT=jsonv2 go build ./cmd/kiroxy
```

Also requires Go 1.26+ (`go.mod` floor).

### Running multiple kiroxy instances from the same vault

Safe. `internal/tokenvault` uses generation-locked refresh; at most
one successful upstream refresh per (provider, account) happens at
any instant across processes. See `BUILD_LOG.md` M4 for the test
that hammers 50 goroutines against the same bundle.

### AWS suspended my Kiro account

Multi-account pooling against consumer Builder IDs can trigger abuse
detection. Personal use only. 1–3 accounts is fine; keep request
cadence reasonable; don't share tokens across machines.

### MCP servers / `mcp_servers` field in /v1/messages

**Status: not supported.** Anthropic's [MCP connector
beta](https://docs.claude.com/en/docs/agents-and-tools/mcp-connector)
(`anthropic-beta: mcp-client-2025-11-20`) lets a client send `mcp_servers[]`
in a Messages API request and have Anthropic do the MCP server connection
+ tool dispatch + result streaming server-side.

kiroxy proxies to AWS Q Developer (Kiro), **not** Anthropic. The Kiro
upstream protocol does not implement MCP semantics — it accepts only a
flat tool definition list and returns `tool_use`/`server_tool_use`
content blocks. There is no upstream `mcp_tool_use`/`mcp_tool_result`
support to forward.

If your client sends `mcp_servers`, kiroxy will currently strip the
field at the request boundary (the `anthropic.Request` struct does not
unmarshal it) and the request proceeds as a normal /v1/messages call.
You will not get MCP tools.

**Workarounds for MCP-style workflows today:**

1. Run an MCP-aware orchestrator in front of kiroxy (e.g.
   [musistudio/claude-code-router](https://github.com/musistudio/claude-code-router)
   ~34k stars, MIT) that handles MCP server connections itself and
   feeds resolved tools to kiroxy as plain `tools[]` entries.
2. Use `claude-code` directly with kiroxy as `ANTHROPIC_BASE_URL`. The
   CLI handles MCP stdio servers locally; kiroxy never sees the MCP
   plumbing.
3. Track [issue #(TODO)](#) for kiroxy-native MCP support — a clean-room
   Go MCP client (~700 LoC, can't lift code from AGPL peers like
   jwadow/kiro-gateway) is on the roadmap but not in v1.4.

### Stream truncated mid-output (client shows partial response then hangs)

Symptom: `claude-code` or `opencode` prints some output then sits forever
without ever showing a final/done state. `kiroxy.log` may show
`upstream stream error` or `upstream exception` mid-stream.

Cause: Kiro upstream sometimes severs long streams without emitting a
proper `messageStop` event — common with very long thinking blocks or
near-cap-exhausted accounts. Without a `message_stop` SSE event the
client cannot finalize the response and waits for more deltas that
never arrive.

Mitigation (since v1.4 + Path-C work): kiroxy now detects this case
when the stream has already promoted (visible content reached the
client) and synthesizes a clean stream-close with
`stop_reason: max_tokens`. The client sees a valid SSE protocol
envelope and can show its normal "response was truncated, ask
'continue' to resume" UX instead of hanging.

The synthetic stop reason is logged at WARN with the original upstream
reason in `upstream_reason` so you can still diagnose what tripped it:

```
WARN  stream truncated, finalizing as max_tokens
      upstream_reason=upstream_severed
      stop_reason_emitted=max_tokens
      input_tokens=823
      output_tokens=12047
```

If you see these warns frequently, check pool health for
near-exhausted accounts (low `usage_percent_used`) and inspect the
GetUsageLimits poller logs for refresh failures.

---

## Diagnostic tools

### `kiroxy debug-refresh <connection_id>`

Forces a refresh against the configured source. Prints the pre-refresh
bundle, the refresh outcome, and the post-refresh bundle. Useful for
isolating "is this account even usable?" from "is the proxy flow
broken?".

### `kiroxy healthcheck`

In-binary `/healthz` probe. Used by the distroless HEALTHCHECK
directive (Phase D). Exits 0 when the local server returns 200; exits
1 otherwise. Good for supervisord / systemd-notify as well.

### `kiroxy opencode-config --output -`

Emits the provider-config JSON snippet for opencode. Dry-run by
writing to stdout with `-`. See `docs/OPENCODE.md`.

### `curl /dashboard/api/state | jq`

Snapshot of the dashboard's live state: account pool, per-account
strike counts, recent requests, upstream latency histograms.
Authenticated the same way as `/v1/*`.

### Trace-level logging

```bash
KIROXY_LOG_LEVEL=debug kiroxy serve
```

Surfaces per-refresh decisions, per-request pool picks, and upstream
HTTP request/response headers (bodies are NOT included to avoid
leaking chat content).

### Isolate the vault for debugging

```bash
KIROXY_DB_PATH=/tmp/kiroxy-debug.db kiroxy serve
```

Starts against a fresh vault so you can reproduce issues without
touching your real one. Import a test account, reproduce, delete the
file.

---

## When all else fails

1. Collect:
   - `kiroxy version`
   - `go version`, `go env GOEXPERIMENT`
   - `kiroxy list-accounts` (redact tokens before sharing)
   - recent kiroxy logs at `debug` level (redact tokens)
   - the exact client command / HTTP request that failed
2. File an issue with the above, plus a minimal reproduction.
3. If the issue is time-sensitive and clearly upstream (everything
   was working, then nothing works), check for:
   - AWS / Kiro status updates
   - a recent `kiroxy` version bump
   - host clock drift (TLS + OAuth are both sensitive)
