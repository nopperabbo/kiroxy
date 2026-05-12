# BACKLOG.md — kiroxy post-MVP

Moved from the build brief / caught by anti-scope-creep during MVP.

Last triaged: 2026-05-12 (post-v0.3.0).

---

## P0 — next release (v0.4.0)

- **Wire pool-mode token refresher** — see the P1 item below, promoted to
  P0 for the next release because without auto-refresh the v0.3.0
  stack is only usable for ~1h per onboarded account.

## Phase G — Onboarder follow-ups (post G.0 + G.1)

G.0 scaffold + G.1 single-account flow landed in v0.3.0.
Remaining work:

- **P2: G.2 — Credential encryption.** Current onboarder ingests `--password` via CLI arg (visible in `ps`) or stdin. Batch mode (G.3) needs persistent credential storage. Options: age (modern, portable, no keyring dependency) or macOS Keychain (OS-integrated; requires `security(1)` shelling). Preferred: age with password-derived keys; fall back to Keychain on macOS if user opts in. Deliverable: `tools/onboard/credentials.enc` generator + decrypter; `onboard.py --credentials-file credentials.enc --email you@…` path that reads the decrypted password for that entry only.
- **P2: G.3 — Batch mode with concurrency cap.** Accept `--credentials-file` (or plain `--accounts-file email:password`) and run N accounts through onboarder in parallel with a concurrency cap (default 3; Kiro/Google are both rate-sensitive). Atomic writes already in place; needs a lock file or fcntl around the output JSON for concurrent `_upsert`. Reuse profile rotation from `_pick_profile()`.
- **P2: G.4 — Retry logic + failure classification.** Classify failures: transient (network, timeout, Camoufox crash) → retry with backoff; hard (wrong password, Google block, consent declined) → fail fast + surface. Mirror kikirro's `_classify_error` shape. On classify=hard, add a `failed_accounts.json` sidecar so G.3 batch runs can resume.
- **P3: G.5 — Polish, progress UI, docs.** tqdm-style progress for batch, per-account status line, README expanded with success/failure rate tips, retry playbook, screenshots walkthrough.

## Phase 2 (post-v0.1.0)

- **P1 (PROMOTED from P2): Wire pool-mode token refresher for `source="import-accounts"` and `source="import-accounts-json"` accounts.** Current state: `pool.TokenGetter.GetToken()` reads `Bundle.Metadata` for `profile_arn` (v0.2.2) but does NOT trigger refresh when the access_token expires. Imported accounts work only during the `expires_in` window (~1h for Desktop-flow tokens) because the kiroclient used for the pool path has no `WithTokenRefresher` (only `KIROXY_KIRO_DB_PATH` mode does). Scope: extend `pool.TokenGetter` + `main.go` to call `auth.refreshSocialToken` for Desktop-flow accounts (`authMethod="social"` in metadata) when rotation is needed; persist rotated refresh_token + access_token back to the vault. This is the next release's defining feature — without it, v0.3.0 is usable only for short-lived testing.
- **P2: Pool tier-awareness — warn/error when a Pro-tier model is requested but the picked account is Free tier.** Needs tier metadata in vault schema or a runtime probe (one-shot call after first refresh).
- **`opencode-config` subcommand** — **CLOSED in v0.3.0 (Phase F).** Ships 7 resolver-verified Claude model IDs with a `-models` filter. The 13-label Pro tier list was *not* adopted literally: `kiro/*` display labels silently rewrite to `claude-sonnet-4-6` in `internal/models/models.go`, so emitting them in opencode config would cause silent-fallback billing misattribution. See OVERNIGHT_LOG Phase F entry for the mapping logic. If Anthropic-family non-Claude models (`kiro/deepseek-3.2`, `kiro/glm-5`, etc.) later appear in the resolver table, `opencode-config` can be extended to include them.
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
