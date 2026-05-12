# BACKLOG.md — kiroxy post-MVP

Moved from the build brief / caught by anti-scope-creep during MVP:

## Phase 2 (post-v0.1.0)

- **P1 (PROMOTED from P2): Wire pool-mode token refresher for source="import-accounts" accounts.** Current state: `pool.TokenGetter.GetToken()` doesn't read `Bundle.Metadata` nor trigger refresh. Triplet-imported accounts work only while the stored access_token is fresh; they break after ~1h when it expires because the kiroclient used for the pool path has no `WithTokenRefresher` (only `KIROXY_KIRO_DB_PATH` mode does). Scope: extend `pool.TokenGetter` + `main.go` to call `auth.refreshSocialToken` when triplet accounts need rotation; persist rotated refresh_token + access_token back to the vault.
- **P2: Pool tier-awareness — warn/error when a Pro-tier model is requested but the picked account is Free tier.** Needs tier metadata in vault schema or a runtime probe (one-shot call after first refresh).
- **P2: `opencode-config` subcommand should emit all 13 canonical Kiro models** (Free + Pro Anthropic + Others from the tier display), with a `--models` filter for subset emission. Reference list:
  - Free Anthropic: `kiro/auto`, `kiro/haiku-4.5`, `kiro/sonnet-4`, `kiro/sonnet-4.5`
  - Pro Anthropic:  `kiro/sonnet-4.6`, `kiro/opus-4.5`, `kiro/opus-4.6`, `kiro/opus-4.7`
  - Free Others:    `kiro/deepseek-3.2`, `kiro/glm-5`, `kiro/minimax-m2.1`, `kiro/minimax-m2.5`, `kiro/qwen3-coder-next`
  - Output cap: 64K all models.
- **AWS Builder ID device-code OAuth inside `kiroxy add-account`** — **CLOSED in v0.2.0 (Phase B).**
- **`--json` flag for `list-accounts` and `status`** — machine-readable output. — P3
- **Interactive `--yes` / `-y` flag on `remove-account`** when/if multi-user lands. — P3
- **OpenAI-compatible surface** `/v1/chat/completions` + `/v1/models` — P1
- **Prompt/response caching** (see kirocc `cache_points` / Quorinex `cache_tracker`) — P2
- **Prometheus metrics exporter** — P2
- **OTel tracing** (kirocc already has the wiring) — P2
- **Inbound rate limiting** (token bucket per API key) — P3 (single-user doesn't need it yet)
- **Dashboard v2** — Vue/React + live charts. v1 (M10) is plain HTML — P3
- **Cost tracking / usage analytics** — P2

## Post-Phase-2

- **Hexos outbound proxy pool** (HTTP/SOCKS5 rotation for VPN users) — P3
- **Hexos provider registry pattern** — groundwork for multi-provider (CodeBuddy revival? Qoder?) — P3
- **Context compression** (aliom-v 3-layer cache + AI summary) — P3
- **Tool Search proxy-side impl** (kirocc has it; requires Anthropic `defer_loading` support) — P3
- **Cloudflare Tunnel integration** (hexos pattern) — P3

## Infra / release

- GitHub Actions CI (go test, go vet, gofmt, govulncheck)
- goreleaser multi-platform binaries
- Homebrew tap
- Docker Hub image + `docker-compose.yml` polish
- Fly.io / Railway / Render one-click deploy guide
- HTTPS via Caddy sidecar config

## Security hygiene

- **Chmod tokens.db to 0600 on Open** — hexos trusted filesystem; we should enforce mode explicitly. Fold into M5 when Vault gets wired from main.go.
- **Audit-zeroize previousRefreshToken on TTL** — currently retained forever; rotate out after N generations to limit blast radius.

## Engineering hygiene

- Replace simple LRU pool with weighted LRU (Quorinex's weight-based expansion)
- Add integration tests that run against a real Kiro test account
- govulncheck in gate
- Race-condition test covers account-swap-mid-stream
- Refactor kirocc's reqconv into smaller files (most are already small but some test files are huge)
