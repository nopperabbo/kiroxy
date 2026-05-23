<div align="center">

<a href="https://github.com/nopperabbo/kiroxy">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset=".github/assets/banner-dark.svg">
    <source media="(prefers-color-scheme: light)" srcset=".github/assets/banner-light.svg">
    <img alt="kiroxy" src=".github/assets/banner-light.svg" width="100%">
  </picture>
</a>

<br>

<p>
  <a href="https://github.com/nopperabbo/kiroxy/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/nopperabbo/kiroxy/ci.yml?branch=main&style=for-the-badge&label=CI&labelColor=1a1a1a&color=c9a34a" alt="CI"></a>
  &nbsp;
  <a href="https://github.com/nopperabbo/kiroxy/releases/latest"><img src="https://img.shields.io/github/v/release/nopperabbo/kiroxy?style=for-the-badge&label=Release&labelColor=1a1a1a&color=c9a34a" alt="Release"></a>
  &nbsp;
  <a href="https://codecov.io/gh/nopperabbo/kiroxy"><img src="https://img.shields.io/codecov/c/github/nopperabbo/kiroxy?style=for-the-badge&labelColor=1a1a1a&color=c9a34a" alt="Coverage"></a>
  &nbsp;
  <a href="https://scorecard.dev/viewer/?uri=github.com/nopperabbo/kiroxy"><img src="https://img.shields.io/ossf-scorecard/github.com/nopperabbo/kiroxy?style=for-the-badge&label=Scorecard&labelColor=1a1a1a&color=c9a34a" alt="OpenSSF Scorecard"></a>
  &nbsp;
  <a href="LICENSE"><img src="https://img.shields.io/github/license/nopperabbo/kiroxy?style=for-the-badge&labelColor=1a1a1a&color=c9a34a" alt="License"></a>
</p>

<p>
  <a href="#quick-start"><strong>Quick Start</strong></a> ·
  <a href="docs/ARCHITECTURE.md"><strong>Architecture</strong></a> ·
  <a href="docs/TROUBLESHOOTING.md"><strong>Troubleshooting</strong></a> ·
  <a href="https://github.com/nopperabbo/kiroxy/discussions"><strong>Discussions</strong></a> ·
  <a href="CHANGELOG.md"><strong>Changelog</strong></a>
</p>

</div>

<br>

## What is kiroxy?

A single-binary, self-hosted proxy that exposes your **Kiro IDE subscription** as a standard **Anthropic Messages API** endpoint. Point Claude Code, Cursor, or any Anthropic-compatible client at kiroxy — it handles credential pooling, token refresh, and request routing transparently.

<br>

<div align="center">
  <img src=".github/assets/demo.svg" alt="kiroxy demo" width="720">
</div>

<br>

## Why Kiroxy?

<table>
  <tr>
    <td width="50%">
      <h4>⚡ Blazing fast</h4>
      <p>23,810 req/s on a single core. Zero-copy SSE streaming. No GC pauses in the hot path.</p>
    </td>
    <td width="50%">
      <h4>📦 Single binary</h4>
      <p>One <code>go build</code>, one file, done. No runtime, no sidecar, no config server.</p>
    </td>
  </tr>
  <tr>
    <td width="50%">
      <h4>🏠 Truly self-hosted</h4>
      <p>Your credentials never leave your machine. Zero telemetry, zero phone-home.</p>
    </td>
    <td width="50%">
      <h4>🔄 Drop-in API</h4>
      <p>Standard Anthropic Messages API — existing SDKs and tools work unchanged.</p>
    </td>
  </tr>
</table>

<br>

## How it compares

| | kiroxy | LiteLLM | One-API | OpenRouter |
|:---|:---:|:---:|:---:|:---:|
| Self-hosted | ✅ | ✅ | ✅ | ❌ |
| Single binary | ✅ | ❌ | ❌ | — |
| Zero dependencies | ✅ | ❌ | ❌ | — |
| Kiro IDE support | ✅ | ❌ | ❌ | ❌ |
| Multi-account pool | ✅ | ❌ | ✅ | — |
| Memory footprint | ~30 MiB | ~500 MiB | ~200 MiB | — |
| No telemetry | ✅ | ❌ | ❌ | ❌ |

<sub>kiroxy is purpose-built for Kiro IDE credentials. For multi-provider routing, use LiteLLM or OpenRouter.</sub>

<br>

## Quick Start

```bash
# Pre-built binary (Linux/macOS, amd64/arm64)
VERSION=1.4.0 OS_ARCH=Darwin_arm64
curl -sSL "https://github.com/nopperabbo/kiroxy/releases/download/v${VERSION}/kiroxy_${VERSION}_${OS_ARCH}.tar.gz" | tar xz
./kiroxy version
```

```bash
# Or via Docker
docker pull ghcr.io/nopperabbo/kiroxy:latest
docker run --rm -p 127.0.0.1:8787:8787 -v kiroxy-data:/data ghcr.io/nopperabbo/kiroxy:latest
```

```bash
# Configure & run
export KIROXY_KIRO_DB_PATH="$HOME/Library/Application Support/kiro-cli/data.sqlite3"
./kiroxy serve
# → kiroxy listening on http://127.0.0.1:8787
```

```bash
# Connect any Anthropic-compatible client
export ANTHROPIC_BASE_URL=http://127.0.0.1:8787
export ANTHROPIC_AUTH_TOKEN=$(openssl rand -hex 32)
claude  # ← just works
```

<br>

## Dashboard

The bundled **Mansion** dashboard ships in the same binary. Seven views, dark + light themes, mobile-responsive.

<table>
  <tr>
    <td align="center" width="50%">
      <img src="docs/screenshots/dashboard-live.png" alt="Live stream" width="100%">
      <br><sub><strong>Live Request Stream</strong> — Real-time SSE feed with drill-down</sub>
    </td>
    <td align="center" width="50%">
      <img src="docs/screenshots/dashboard-metrics.png" alt="Metrics" width="100%">
      <br><sub><strong>Metrics</strong> — Time-series charts, SLO gauges</sub>
    </td>
  </tr>
  <tr>
    <td align="center" width="50%">
      <img src="docs/screenshots/dashboard-pool.png" alt="Pool" width="100%">
      <br><sub><strong>Account Pool</strong> — Density viz with anomaly emphasis</sub>
    </td>
    <td align="center" width="50%">
      <img src="docs/screenshots/dashboard-logs.png" alt="Logs" width="100%">
      <br><sub><strong>Logs</strong> — Volume histogram, facets, JSONL export</sub>
    </td>
  </tr>
</table>

<details>
<summary><strong>More screenshots</strong></summary>
<br>
<table>
  <tr>
    <td align="center" width="50%">
      <img src="docs/screenshots/dashboard-settings-light.png" alt="Light theme" width="100%">
      <br><sub><strong>Light Theme</strong> — WCAG AA contrast</sub>
    </td>
    <td align="center" width="50%">
      <img src="docs/screenshots/dashboard-mobile.png" alt="Mobile" width="280">
      <br><sub><strong>Mobile</strong> — All views adapt to 390px</sub>
    </td>
  </tr>
</table>
</details>

<br>

## Architecture

> Full component-by-component overview: [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md)

```
  Claude Code ─┐   ┌─────────────┐   ┌──────────┐   ┌──────┐
  Cursor       ├──▶│ /v1/messages │──▶│ reqconv  │──▶│ kiro │──▶ Kiro IDE
  any client  ─┘   │ auth · log   │   │ anthropic│   │client│
                    └──────┬───────┘   │ → kiro   │   └──────┘
                           │           └──────────┘       │
                           ▼                              ▼
                    ┌─────────────┐              ┌────────────┐
                    │ pool.Token  │              │  respconv  │
                    │ (LRU+cool) │              │ kiro → SSE │
                    └──────┬──────┘              └─────┬──────┘
                           │                           │
                           ▼                           ▼
                    ┌─────────────┐              your client
                    │ tokenvault  │           (streamed chunks)
                    │ SQLite+OAuth│
                    └─────────────┘
```

<br>

<details>
<summary><strong>Environment Variables</strong></summary>

| Variable | Default | Purpose |
|:---|:---|:---|
| `KIROXY_API_KEY` | _(empty = open)_ | Client auth key |
| `KIROXY_BIND` | `127.0.0.1` | Bind interface |
| `KIROXY_PORT` | `8787` | TCP port |
| `KIROXY_DB_PATH` | `~/.kiroxy/tokens.db` | Vault SQLite path |
| `KIROXY_KIRO_DB_PATH` | _(empty)_ | Read creds from kiro-cli DB |
| `KIROXY_KIRO_REGION` | `us-east-1` | AWS region |
| `KIROXY_LOG_LEVEL` | `info` | Log level |
| `KIROXY_SHUTDOWN_TIMEOUT` | `30` | Drain timeout (seconds) |

</details>

<details>
<summary><strong>API Endpoints</strong></summary>

| Method | Path | Auth | Purpose |
|:---|:---|:---|:---|
| `POST` | `/v1/messages` | ✓ | Anthropic Messages API |
| `POST` | `/v1/messages/count_tokens` | ✓ | Token count |
| `GET` | `/healthz` | — | Liveness probe |
| `GET` | `/readyz` | ✓ | Readiness probe |
| `GET` | `/dashboard-mansion` | loopback | Operator dashboard |

</details>

<details>
<summary><strong>CLI Reference</strong></summary>

```bash
kiroxy serve              # Run the proxy
kiroxy add-account        # Register via AWS Builder ID OAuth
kiroxy import-accounts    # Bulk import from triplet file
kiroxy list-accounts      # Show vault accounts
kiroxy remove-account     # Delete an account
kiroxy status             # Pool + request snapshot
kiroxy debug-refresh      # Force token refresh
kiroxy healthcheck        # Docker HEALTHCHECK probe
kiroxy opencode-config    # Emit opencode.json snippet
kiroxy version            # Print version
```

</details>

<details>
<summary><strong>Docker</strong></summary>

```bash
docker pull ghcr.io/nopperabbo/kiroxy:latest
docker run --rm \
  -p 127.0.0.1:8787:8787 \
  -v kiroxy-data:/data \
  --read-only --cap-drop=ALL \
  --security-opt=no-new-privileges:true \
  -e KIROXY_API_KEY="your-key" \
  ghcr.io/nopperabbo/kiroxy:latest
```

| Control | Setting |
|:---|:---|
| Base image | `gcr.io/distroless/static-debian12:nonroot` |
| User | `nonroot` (UID 65532) |
| Root FS | Read-only |
| Capabilities | All dropped |
| Healthcheck | In-binary `kiroxy healthcheck` |

</details>

<details>
<summary><strong>Supply Chain Security</strong></summary>

Releases are signed with [Sigstore cosign](https://docs.sigstore.dev/) and include SBOM (SPDX format).

```bash
cosign verify-blob \
  --certificate kiroxy_1.4.0_Darwin_arm64.tar.gz.cert \
  --signature kiroxy_1.4.0_Darwin_arm64.tar.gz.sig \
  --certificate-identity-regexp "github.com/nopperabbo/kiroxy" \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
  kiroxy_1.4.0_Darwin_arm64.tar.gz
```

</details>

<details>
<summary><strong>Troubleshooting</strong></summary>

> Full diagnostics: [`docs/TROUBLESHOOTING.md`](docs/TROUBLESHOOTING.md)

| Symptom | Fix |
|:---|:---|
| `401 authentication_error` | No account added. Set `KIROXY_KIRO_DB_PATH` or run `kiroxy add-account` |
| `401 missing_api_key` | Client missing `X-Api-Key` header |
| SSE chunks arrive all at once | Disable proxy buffering (`proxy_buffering off`) |
| `go build` fails with json/v2 | Missing `GOEXPERIMENT=jsonv2` — use `make build` |

</details>

<br>

## Contributing

PRs welcome. See [`CONTRIBUTING.md`](CONTRIBUTING.md) for guidelines.

```bash
make gate       # fmt + vet + build + test
make lint       # golangci-lint (25+ linters)
make test-race  # race detector
```

<br>

## License

[MIT](LICENSE)

<br>

---

<div align="center">
  <sub>Built with care by <a href="https://github.com/nopperabbo">@nopperabbo</a></sub>
  <br>
  <sub>
    <a href="https://github.com/sponsors/nopperabbo">💛 Sponsor</a> ·
    <a href="https://github.com/nopperabbo/kiroxy/issues">Report Bug</a> ·
    <a href="https://github.com/nopperabbo/kiroxy/discussions">Discuss</a>
  </sub>
  <br><br>
  <a href="https://repobeats.axiom.co/api/embed/kiroxy">
    <img src="https://repobeats.axiom.co/api/embed/placeholder.svg" alt="Repobeats analytics" width="100%">
  </a>
</div>
