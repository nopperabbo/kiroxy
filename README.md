# kiroxy

A single-user, self-hosted proxy that exposes your Kiro IDE subscription (Amazon Q Developer / AWS CodeWhisperer) as an **Anthropic Messages API** endpoint. Point your Claude Code, Cursor, or any Anthropic-compatible client at kiroxy, and it forwards requests to Kiro using your own credentials.

**Status:** v0.3.0 — personal-use, MIT-licensed. See `BUILD_LOG.md` for the construction log, `CHANGELOG.md` for release notes, `docs/ARCHITECTURE.md` for the engineering overview, and `docs/TROUBLESHOOTING.md` for operator diagnostics.

---

## Installation

Three ways to get kiroxy running.

### 1. Download a pre-built binary (recommended for end users)

Pre-built binaries for Linux and macOS (amd64 + arm64) are attached to every
[GitHub Release](../../releases/latest). Each archive ships the binary
plus `LICENSE`, `NOTICE`, `README.md`, `CHANGELOG.md`, `docs/ARCHITECTURE.md`,
`docs/TROUBLESHOOTING.md`, and `docs/OPENCODE.md`. A SHA-256 checksums file
(`kiroxy_<version>_checksums.txt`) is attached to verify download integrity.

```bash
# Pick the matching Os_Arch — Linux_amd64, Linux_arm64, Darwin_amd64, Darwin_arm64.
VERSION=0.3.0
OS_ARCH=Linux_amd64   # or Darwin_arm64, etc.

curl -sSL -o kiroxy.tar.gz \
  "https://github.com/nopperabbo/kiroxy/releases/download/v${VERSION}/kiroxy_${VERSION}_${OS_ARCH}.tar.gz"
curl -sSL -o checksums.txt \
  "https://github.com/nopperabbo/kiroxy/releases/download/v${VERSION}/kiroxy_${VERSION}_checksums.txt"

# Verify the archive matches the published checksum.
grep " kiroxy_${VERSION}_${OS_ARCH}.tar.gz$" checksums.txt | shasum -a 256 -c -

tar -xzf kiroxy.tar.gz
./kiroxy version
```

### 2. Run from Docker

Covered in the [Run with Docker](#run-with-docker) section below. `gcr.io`
or `ghcr.io` image hosting is on the roadmap; for now, build locally with
`make docker-build`.

### 3. Build from source

Covered in the [Five-minute quickstart](#five-minute-quickstart) below.
`make build` pins `GOEXPERIMENT=jsonv2` and stamps `main.version` from
`git describe`.

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

**Option B — managed token vault** (recommended for multi-account):

Seed the vault with one or more accounts:

```bash
# Single account
./kiroxy add-account --label=my-account --refresh-token=<your-refresh-token>

# Or bulk: line-delimited email:refresh_token:signature triplets
./kiroxy import-accounts --file=triplets.txt

# Or pipe triplets:
cat triplets.txt | ./kiroxy import-accounts --stdin
```

The managed vault lives at `~/.kiroxy/tokens.db` (mode 0600).

**About the triplet format:** `email:refresh_token:signature` — only the
refresh_token is sent upstream; the email is used as the account identifier
and the optional signature is stored in `metadata` for reference (kiroxy
never transmits it).

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

> **For the full component-by-component overview, see
> [docs/ARCHITECTURE.md](./docs/ARCHITECTURE.md).** Read it before
> modifying anything in `internal/`. The diagram below is the high-level
> sketch only.

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
| GET | `/dashboard` | loopback bypass | 302 redirect to the canonical `/dashboard-mansion` |
| GET | `/dashboard-mansion` | loopback bypass | Canonical operator dashboard (Mansion) |
| GET | `/dashboard/api/state` | loopback bypass | Pool + request snapshot JSON (shared by all dashboard variants) |
| GET | `/_variants/<slug>` | loopback bypass | Archived dashboards: `brutal`, `paper`, `nord`, `neon`, `muji`, `linear-premium`, `dashboard-next`, `dashboard-legacy` |

Pass API key as either `X-Api-Key: <key>` or `Authorization: Bearer <key>`.

Every response carries `X-Request-Id`; clients may set their own via the same header.

---

## Dashboard

kiroxy ships a single canonical dashboard plus an archive of historical
design variants.

- **Canonical:** `GET /dashboard-mansion` — the "operator desk" UI.
  Warm charcoal + aged-brass amber, JetBrains Mono for data density,
  Linear-grade motion discipline (View Transitions, @starting-style
  enters, amber focus ring). Built with Svelte 5 + Vite; the bundle is
  embedded in the binary so a fresh clone works with `go build` alone.
- **Redirect:** `GET /dashboard` — 302s to `/dashboard-mansion` so
  existing bookmarks and scripts keep working.
- **Archive:** `GET /_variants/<slug>` — eight historical variants
  (`brutal`, `paper`, `nord`, `neon`, `muji`, `linear-premium`,
  `dashboard-next`, `dashboard-legacy`) kept fully functional for
  reference. Each one commits to one taste without blending; see
  [`docs/VARIANTS.md`](./docs/VARIANTS.md) for the philosophy matrix.

Auth behavior for all dashboard routes: loopback requests skip the
`KIROXY_API_KEY` check (personal-use UX). Non-loopback access still
requires the key via `X-Api-Key` or `Authorization: Bearer`.

---

## Troubleshooting

> **For the full diagnostics playbook, see
> [docs/TROUBLESHOOTING.md](./docs/TROUBLESHOOTING.md).** The sections
> below cover the handful of errors most operators hit first.

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

## CLI Reference

All kiroxy functionality ships as a single binary. Top-level shortcuts
(`--version`, `-v`, `--help`, `-h`, `help`) work without a subcommand.
Subcommand `--help` prints per-command usage.

| Subcommand | One-liner |
|---|---|
| `serve` (default) | Run the HTTP proxy. Respects all `KIROXY_*` env vars. |
| `add-account` | Register a new account via AWS Builder ID device-code OAuth. |
| `import-accounts` | Bulk-import accounts from a line-delimited triplet file (`email:refresh_token:signature`). |
| `import-accounts-json` | Import accounts from Desktop-flow JSON (the `tools/onboard/` sidecar emits this format). |
| `list-accounts` | Print the managed vault's accounts with status, last-used, strikes. |
| `remove-account <id>` | Delete an account from the vault. |
| `status` | Pool + request snapshot (mirrors the dashboard JSON state). |
| `debug-refresh <id>` | Force a refresh against the configured source; print outcome. Useful for diagnosing revoked tokens. |
| `healthcheck` | In-binary `/healthz` probe. Used by the Docker `HEALTHCHECK` directive. |
| `opencode-config` | Emit an `opencode.json` provider snippet (7 resolver-verified Claude model IDs). |
| `version` | Print the `-ldflags -X main.version` stamp. |
| `help [subcommand]` | Print top-level or per-subcommand usage. |

Examples:

```bash
# First-time setup
kiroxy import-accounts-json --file tools/onboard/kiro_tokens.json
kiroxy list-accounts

# Operational
kiroxy status                            # pool snapshot
kiroxy debug-refresh $(kiroxy list-accounts -ids | head -1)
kiroxy opencode-config -output opencode.snippet.json

# Container-friendly
kiroxy healthcheck && echo ok
```

For the full per-subcommand flag list: `kiroxy help <subcommand>`.

---

## Run with Docker

A multi-stage `Dockerfile` (distroless, nonroot, ~30 MiB) and `docker-compose.yml` ship with the repo. The token vault lives in a named volume at `/data/tokens.db` so it survives container restarts.

### Quick start

```bash
cp .env.example .env                         # fill in KIROXY_API_KEY
make docker-compose-up                       # build + start in background
docker compose logs -f kiroxy                # tail JSON logs
curl http://127.0.0.1:8787/healthz           # {"status":"ok",...}
```

Add an account (running container has the same CLI surface as the native binary):

```bash
docker compose exec kiroxy kiroxy add-account \
  --label=my-account --refresh-token=<your-refresh-token>
docker compose exec kiroxy kiroxy list-accounts
```

### Manual `docker run`

```bash
make docker-build                            # tags kiroxy:<git-describe> + kiroxy:local
docker run --rm \
  -p 127.0.0.1:8787:8787 \
  -v kiroxy-data:/data \
  --read-only --cap-drop=ALL \
  --security-opt=no-new-privileges:true \
  --tmpfs /tmp:size=16m,mode=1777 \
  -e KIROXY_API_KEY="$KIROXY_API_KEY" \
  kiroxy:local
```

### Security posture inside the container

| Control | Setting |
|---|---|
| Base image | `gcr.io/distroless/static-debian12:nonroot` (no shell, no package manager) |
| User | `nonroot` (UID 65532) |
| Root FS | read-only; only `/data` (vault) and `/tmp` (tmpfs) are writable |
| Capabilities | all dropped |
| Privilege escalation | `no-new-privileges:true` |
| Healthcheck | in-binary `kiroxy healthcheck` subcommand (no curl, no shell) |

### Gotchas

- **`KIROXY_BIND` inside the container is `0.0.0.0`** \u2014 the network namespace IS the boundary. Control host-side exposure via `docker run -p` / compose's `ports:` mapping (defaults to `127.0.0.1:8787`).
- **`docker compose down` keeps the volume**; `docker compose down -v` wipes it (including `tokens.db`). Back up the volume before running `-v`.
- **Image tags never use `:latest`**; set `IMAGE=foo:bar` on `make docker-build` if you want a custom tag.

---

## Contributing

kiroxy is personal-use software, but PRs that fix bugs, add tests, or
improve docs are welcome.

- **CI.** The repository ships a [GitHub Actions workflow](./.github/workflows/ci.yml)
  that runs `make gate` on Ubuntu + macOS on every PR. A separate
  [govulncheck workflow](./.github/workflows/vuln.yml) runs daily.
- **Pre-commit.** Run `make gate` locally before pushing. Set
  `KIROXY_CI_STRICT=1` to also require `govulncheck`.
- **Releases.** Tag-triggered via the [release workflow](./.github/workflows/release.yml).
  Local dry-run: `make release-dry-run` (requires goreleaser).
- **Commit style.** Conventional commits (`feat:`, `fix:`, `docs:`,
  `ci:`, `build:`, `chore:`). The goreleaser changelog groups by prefix.
- **Anti-scope-creep.** New features land in `BACKLOG.md` first. Core
  proxy path stays small.

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
