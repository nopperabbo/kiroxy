# kiroxy Architecture

This document is a living reference for how kiroxy is put together. It
complements `BUILD_LOG.md` (which is the chronological construction log)
and `NOTICE` (which carries the per-file licence attribution).

Last reviewed: Phase I (2026-05-12). Expect drift after each phase; update
as packages change shape.

---

## Overview

**kiroxy** is a single-user, self-hosted proxy that exposes an operator's
Kiro IDE subscription (Amazon Q Developer / AWS CodeWhisperer upstream) as
an **Anthropic Messages API** endpoint. Clients that speak the Anthropic
protocol — opencode, Claude Code, Cursor, raw `curl` — point at kiroxy
instead of `api.anthropic.com`; kiroxy rewrites each request into the
CodeWhisperer protocol, forwards it upstream, and rewrites the streaming
response back into Anthropic SSE.

### What kiroxy is

- A personal proxy. Single operator, 1–N accounts, one binary, one SQLite
  file, no external infrastructure.
- An Anthropic-compatible edge. Streaming, non-streaming, tool use,
  vision, thinking blocks, tool_search, count_tokens — all of it speaks
  the Anthropic wire format.
- A translator. Every request/response crosses the Anthropic↔CodeWhisperer
  boundary through `internal/reqconv` and `internal/respconv`.
- A multi-account pool with cooldowns, generation-locked OAuth refresh,
  and a per-request selection policy.

### What kiroxy is NOT

- Not a production hosting layer. No tenancy, no rate-limiting per-key
  beyond loopback trust, no TLS termination. Put Caddy or nginx in front
  if you expose it beyond localhost.
- Not an account manager. Onboarding lives in a separate Python sidecar
  under `tools/onboard/` (Phase G). The Go binary does not ship a
  browser.
- Not an inference engine. kiroxy forwards; Kiro decides the model.

### High-level data flow

```
                    +---------------------+
                    |  opencode / Claude  |
                    |  Code / curl / SDK  |
                    +---------+-----------+
                              |
                  Anthropic Messages API (HTTP/JSON)
                              |
                   +----------v----------+
                   |       kiroxy        |
                   |  (this repo)        |
                   +----------+----------+
                              |
           AWS SigV4-less bearer-token JSON-RPC
           (X-Amz-Target: ...GenerateAssistantResponse)
                              |
                 +------------v------------+
                 | Kiro CodeWhisperer API  |
                 |  codewhisperer.us-east- |
                 |  1.amazonaws.com        |
                 +-------------------------+
```

The choice of upstream host (`codewhisperer.*` vs `q.*`) is decided by
whether the account has a `profileArn` — see the Kiro client section.

---

## Components

Every package in `internal/` is listed below with its purpose, key types,
and provenance. The **Attribution** line maps to `NOTICE`.

### `internal/server` — HTTP surface

**Purpose.** Wire the HTTP mux, middleware stack, and route handlers;
serve the dashboard; expose health endpoints.

**Key types.**
- `server.Options`, `server.Server` — constructor-style wiring from `cmd/kiroxy/main.go`.
- Middleware: `auth.go` (SHA-256 constant-time inbound-key check),
  `logging.go` (per-request ULID, JSON slog logger, `X-Request-Id`).
- Routes:
  - `GET /healthz` — liveness (bypass auth).
  - `GET /readyz` — readiness with per-dep subchecks (`readiness.go`).
  - `POST /v1/messages` — Anthropic handler (`internal/messages`).
  - `POST /v1/messages/count_tokens` — token counting.
  - `GET /dashboard` + `GET /dashboard/api/state` — HTML + JSON snapshot.

**Attribution.** Original to kiroxy.

### `internal/auth` — credential sourcing

**Purpose.** Read credentials from the configured source (managed vault
OR kiro-cli SQLite DB) and produce `auth.Credentials` ready for the Kiro
client.

**Key types.**
- `Credentials` — `{AccessToken, RefreshToken, ExpiresAt, ProfileARN, AuthMethod, ClientID, ClientSecret}`.
- `RefreshFn` — callback invoked when `ExpiresAt <= now`.
- `refresh.go` — Builder ID device-code refresh (`auth_method="builderid"`).
- `refresh_social.go` — Desktop-flow refresh (`auth_method="social"`).

**Attribution.** Derived from kirocc; refresh paths kept structurally
identical so bug fixes upstream can be ported. See NOTICE for commit SHA.

### `internal/tokenvault` — SQLite credential store

**Purpose.** Persist OAuth bundles at rest; mediate safe concurrent
refresh across goroutines and processes using the generation-lock
pattern.

**Key types.**
- `Vault` — SQLite handle, opened at `KIROXY_DB_PATH`.
- `Bundle` — `{Provider, ConnectionID, AccessToken, RefreshToken, ExpiresAt, Metadata, Generation}`.
- Generation-locked refresh: `ReserveRefresh()` captures generation,
  `CommitRefresh()` rejects if generation moved, `ReleaseRefresh()`
  rolls back a reservation.

**Design decisions.**
- **SQLite not Postgres** — single user, zero external deps, self-healing
  schema migration on `Open()`.
- **Mode 0600 enforced on `Open()`** — vault at rest should be
  user-private.
- **modernc.org/sqlite** — pure Go driver, no cgo, no libc surface area.
- **IMMEDIATE transactions** — every mutation runs in an IMMEDIATE tx so
  the DB lock is acquired up front, eliminating the "upgrade from read to
  write" SQLITE_BUSY pathology.

**Attribution.** Ported from `kadangkesel/hexos` (MIT); generation-lock
pattern preserved literally. See NOTICE.

### `internal/pool` — account selection

**Purpose.** Pick an account for each outbound request using an LRU
policy; cooldown accounts that return upstream errors; mark accounts
failed after 3 consecutive errors.

**Key types.**
- `Pool` — `{accounts []Account, cooldownUntil map[string]time.Time, strikes map[string]int}`.
- `Pick(ctx) (Account, error)` — returns `ErrNoAccount` if nothing is
  usable.
- `TokenGetter` — adapter that the Kiro client uses; reads the current
  access token from the vault, triggering refresh on expiry.

**Design decisions.**
- **LRU not round-robin** — even in a multi-account deployment the
  workload is typically one-user-at-a-time, so LRU just means "spread
  load evenly". Donor project was weighted RR.
- **Cooldowns track recency, strikes track repeat failures.** An account
  returning a 5xx once enters a short cooldown; 3 in a row marks it
  failed and it drops out of the pool until re-enabled.
- **`profileArn` threads from vault metadata** into `auth.Credentials`
  (closed in v0.2.2).

**Attribution.** Derived from `Quorinex/Kiro-Go` (MIT); selection policy
swapped to LRU.

### `internal/kiroclient` — upstream HTTP

**Purpose.** Make HTTP calls to the Kiro CodeWhisperer API; parse the
event stream; surface AWS-style errors in a Go-typed shape.

**Key types.**
- `HTTPClient` — `{http.Client, region, optional TokenRefresher}`.
- `GenerateAssistantResponse(ctx, creds, body) (*Response, error)` — the
  single upstream entry point.
- `aws_error.go` — typed `AWSError` with `Kind`, `Message`, `RequestID`.
- `X-Amz-Target` selection: `chooseAmzTarget` switches to the AmazonQ
  target when `ProfileARN` is empty (Builder ID path) and to the
  CodeWhisperer target otherwise (Desktop-flow path).

**Attribution.** Derived from kirocc.

### `internal/messages` — Anthropic handler

**Purpose.** Be the engine behind `POST /v1/messages`. Parses the
Anthropic request, picks an account via the pool, calls reqconv to build
the CodeWhisperer body, calls kiroclient, streams the response through
respconv, and writes SSE to the client.

**Key types.**
- `Service` — a stateless handler composed from auth, pool, kiroclient,
  reqconv, respconv.
- `capture.go` — optional payload capture for debugging (off by default).
- `toolsearch.go` — Anthropic tool_search support.

**Attribution.** Derived from kirocc.

### `internal/reqconv` — Anthropic → CodeWhisperer

**Purpose.** Translate Anthropic Messages API JSON into Kiro's
`ConverseRequest` shape.

**Key responsibilities.**
- Map `messages[]` → `conversationMessages[]`.
- Serialise vision, thinking, tool_use, tool_result blocks.
- Extract tool_reference blocks for tool_search resolution.
- Handle `system` prompts as a distinct top-level field.

**Attribution.** Derived from kirocc.

### `internal/respconv` — CodeWhisperer → Anthropic

**Purpose.** Translate Kiro's event stream back into Anthropic SSE. Runs
as a streaming accumulator: each upstream event produces zero or more
client events (`message_start`, `content_block_delta`, `message_delta`,
`message_stop`, …).

**Key types.**
- `responseAccumulator` — per-request state (token budget, content
  blocks, usage counters, stop reason).

**Attribution.** Derived from kirocc.

### `internal/kiroproto` — wire types

**Purpose.** Go structs matching the Kiro CodeWhisperer JSON schema
(request + streaming response). Uses `encoding/json/v2` (behind
`GOEXPERIMENT=jsonv2`).

**Attribution.** Derived from kirocc.

### `internal/models` — model ID resolver

**Purpose.** Map display labels to canonical Anthropic model IDs.
Silently rewrites unknown `kiro/*` labels to `claude-sonnet-4-6`.

**Known quirk.** This silent fallback is why `kiroxy opencode-config`
only emits the 7 resolver-verified IDs; emitting the Pro-tier `kiro/*`
display labels would cause silent-fallback billing misattribution.

**Attribution.** Derived from kirocc.

### `internal/tokencount` — tiktoken wrapper

Backs `POST /v1/messages/count_tokens`. Uses the cl100k_base encoding
from `github.com/pkoukk/tiktoken-go`.

**Attribution.** Derived from kirocc.

### `internal/tracing` — OpenTelemetry

Standard otel HTTP instrumentation. Tracing wires exist but are not
enabled by default; see the BACKLOG for the exporter landing item.

**Attribution.** Derived from kirocc.

### `internal/logging` — slog + JSON + ULID

**Purpose.** Structured logs on stderr. Every request gets a ULID, emitted
in the `X-Request-Id` response header so clients can correlate.

**Attribution.** Derived from kirocc.

### `internal/anthropic`, `internal/httpx`, `internal/toolsearch`

Shared types and helpers. Each is small and focused; see the files for
specifics.

**Attribution.** Derived from kirocc.

### `internal/config` — env + flag parsing

**Purpose.** Single source of truth for all `KIROXY_*` env vars. No
external dependency.

**Attribution.** Original to kiroxy.

### `cmd/kiroxy` — CLI

**Purpose.** The executable's subcommand dispatch:
- `serve` (default) — run the HTTP proxy.
- `add-account` — Builder ID device-code OAuth (Phase B).
- `import-accounts` — line-delimited triplet format (Phase A).
- `import-accounts-json` — Desktop-flow tokens (Phase C.2b).
- `list-accounts` / `remove-account` / `status` — vault admin.
- `debug-refresh` — force a refresh, dump result (Phase C.2b).
- `healthcheck` — in-binary `/healthz` probe (Phase D Docker).
- `opencode-config` — emit opencode provider JSON (Phase F).
- `version` / `help`.

### `tools/onboard/` — Python sidecar (Phase G)

**Purpose.** Automate Kiro Desktop OAuth acquisition end-to-end: PKCE
generation, login URL construction, Camoufox browser drive with
humanised typing, callback capture, token exchange, and output JSON
matching the `kiroxy import-accounts-json` schema.

**Why external.** Keep the Go binary small and CGO-free. The Python
sidecar is optional; operators who already have refresh tokens from
`kiro-cli` or `kikirro` do not need it.

---

## Request Lifecycle

```
client                kiroxy                    upstream (Kiro)
  |                     |                                |
  |--POST /v1/msgs----->| auth MW (SHA-256 key check)    |
  |                     | logging MW (assign ULID)       |
  |                     | messages.Service.Handle        |
  |                     |   |                            |
  |                     |   +-- pool.Pick() ------+      |
  |                     |                         |      |
  |                     |   +-- vault.Get(acct) --+      |
  |                     |     |                          |
  |                     |     +-- refresh if expired -->|
  |                     |                                |
  |                     |   +-- reqconv.Build()          |
  |                     |   +-- kiroclient.Generate*()-->|
  |                     |                                | 200 +SSE
  |                     |   <-- SSE event loop ----------|
  |                     |   +-- respconv.Accumulate()    |
  |<--SSE events--------| write Anthropic SSE chunks     |
  |                     | on client disconnect: cancel ctx
  |                     | on upstream error: surface to pool
  |                     |   (cooldown / strike / mark failed)
```

Happy path: one upstream request per client request, one access-token
lookup, zero refreshes.

Refresh-triggered path: vault.Get detects `ExpiresAt <= now`, calls the
auth refresh fn (one successful refresh per provider+connection thanks
to the generation lock), writes the new bundle, returns the fresh token.

Failure path: upstream 4xx/5xx surfaces to messages.Service; the account
is cooldowned or strike-counted by the pool; the client receives an
Anthropic-shaped error.

---

## Account Lifecycle

```
  onboard (G)  ---+
                   \
  import-json -----+---> vault.Save
  import-triplet --+       |
                           v
  add-account -----+---> pool: account visible
                           |
                           | first request:
                           |   access_token expired? refresh
                           |
                           | upstream error: strike++
                           |   strikes >= 3 → status = failed
                           |   (falls out of pool)
                           v
                  manual re-enable / re-import / remove-account
```

Account states in the vault: `active`, `failed`, `disabled`. Pool only
picks `active` accounts.

---

## Design Decisions

### Why SQLite, not Postgres
Single operator, zero deploy dependencies. A single `~/.kiroxy/tokens.db`
file stores everything. Backup = copy the file. Multi-process safety is
handled by the OS file lock + SQLite's own WAL.

### Why Go stdlib-heavy, minimal deps
Reduce supply-chain surface area. The entire deps list is small and
audited: UUID, slog colour, tiktoken, otel, sync, lumberjack, sqlite.
No framework, no router, no ORM.

### Why the pool uses LRU, not round-robin
Single-caller workload. Weighted RR adds complexity without measurable
benefit when N=1 concurrent requests. LRU keeps even spread on the
multi-request case.

### Why refresh is both proactive and reactive
Belt-and-suspenders. The vault proactively refreshes when `ExpiresAt
<= now`. The kiroclient reactively retries once on `UnauthorizedException`
upstream. Either alone would be enough; both together survive clock
skew and upstream token-expiry ambiguity.

### Why the Python onboarder is external
Separation of concerns. The Go binary is ~30 MiB and CGO-free. Adding
Playwright / Camoufox / Chromium would inflate the binary by 10x and
pull in a large surface area. Operators who already have refresh tokens
never need the Python tool.

### Why distroless, not Alpine
Smaller attack surface. Distroless has no shell, no package manager, no
libc. The container runs as `nonroot` (UID 65532), read-only root FS,
all caps dropped. The only writable mount is the named volume at
`/data` for the SQLite file.

### Why GOEXPERIMENT=jsonv2
Kiro's response streams can exceed the 6.5 MB that `encoding/json` v1
buffers before returning a parse error. `encoding/json/v2` fixes the
bufferless streaming path and is a fixed-feature in Go 1.26+. The
tradeoff is a `go 1.26` module floor.

---

## Security Model

### Trust boundaries

- **Operator trusts the host.** The vault is readable by the OS user
  who owns `~/.kiroxy/`. If the host is compromised, the vault is
  compromised. There is no process-level sandbox.
- **kiroxy trusts the filesystem.** `KIROXY_DB_PATH` is readable and
  writeable by the process user; the file is chmod'd to 0600 on
  `vault.Open()`.
- **kiroxy trusts the network.** TLS-only to `codewhisperer.*` and
  `q.*` upstream endpoints. No certificate pinning, but the system
  trust store is the only anchor.

### Inbound authentication

- `KIROXY_API_KEY` unset → loopback-bypass mode. Any request accepted.
  Only safe if `KIROXY_BIND` is `127.0.0.1` (default).
- `KIROXY_API_KEY` set → required on every `/v1/*` and
  `/dashboard/api/*` request via `X-Api-Key` or
  `Authorization: Bearer`. Comparison is SHA-256 constant-time.
- `/healthz` always bypasses auth (intended for container orchestrators).
- `/readyz` requires auth.

### Outbound

- TLS 1.2+ to Kiro upstream (enforced by stdlib default).
- Connections pooled via `http.Transport` default keepalive.
- Request IDs propagated from client to upstream via `X-Request-Id`.

### Vault at rest

- SQLite file at `KIROXY_DB_PATH` (default `~/.kiroxy/tokens.db`).
- Parent dir auto-created at mode 0700.
- File chmod'd to 0600 on open.
- **Plaintext today.** Encryption-at-rest is a roadmap item (Phase G.2
  for the onboarder credentials file; vault-side encryption is open).

### Secrets in logs

- Access tokens and refresh tokens are never logged in full.
- Redaction is handled at the structured-log layer (`internal/logging`
  logs by field, and credential fields are never added to the record).
- Request bodies may contain chat content; by default only headers and
  metadata are logged. The capture path in `internal/messages` is
  opt-in for debugging.

---

## Further reading

- `BUILD_LOG.md` — chronological phase-by-phase log.
- `CHANGELOG.md` — user-facing release notes.
- `BACKLOG.md` — open items.
- `NOTICE` — per-file attribution to donor projects (kirocc,
  Quorinex/Kiro-Go, kadangkesel/hexos).
- `TROUBLESHOOTING.md` — operator-facing diagnostics.
