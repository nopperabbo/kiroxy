# BACKLOG.md — kiroxy post-MVP

Moved from the build brief / caught by anti-scope-creep during MVP.

Last triaged: 2026-05-12 (post-Phase I).

---

## P0 — next release (v1.0.1)

- **CLOSED: Phase M — Prometheus metrics endpoint** (2026-05-12). `/metrics` served in Prometheus text format with request lifecycle histograms/counters, pool/vault GaugeFunc snapshots, and operator docs at `docs/METRICS.md` + starter Grafana dashboard at `docs/METRICS.grafana.json`. Cardinality bounded (5x status classes × 7 model aliases × 2 stream values). Loopback bypass + `KIROXY_METRICS_PUBLIC=1` for trusted private networks.
- **Upstream 403 with fresh credentials + new endpoint** — SURFACED 2026-05-12 live smoke. Symptoms: `/v1/messages` returns 502 (upstream 403 empty body) even with (a) freshly refreshed access_token verified valid via `kiroxy debug-refresh`, (b) migrated `runtime.us-east-1.kiro.dev` endpoint, (c) Phase 2.5 reactive refresh triggers 3x per request. Direct manual curl to `runtime.us-east-1.kiro.dev` with same access_token + profileArn + simpler payload shape returns 200 + valid EventStream SSE including `contextUsageEvent` and `meteringEvent`. Conclusion: kiroxy payload construction diverges from what the new endpoint accepts. Suspect: elaborate history/toolContext/agentTaskType fields that the manual minimal payload does not include. Next steps: (1) capture kiroxy's actual outbound body via the temp KIROXY_TAP, (2) diff against minimal-working curl body field-by-field, (3) identify which field triggers rejection. LoC estimate: 30-100 (likely field name or null-vs-omit issue in reqconv).
- **`added_at + expires_in` vs `expires_at` discrepancy** — SURFACED 2026-05-12. `/tmp/dash-preview.db` showed `expires_at` 1h **beyond** what `added_at + expires_in` math predicts. First import at 22:08 claimed expiry at 00:09 (2h later) instead of 23:08 (1h). Impact: Phase 2.5 proactive refresh trigger window is miscalibrated — either fires too early or too late. Needs deterministic test with mocked time + reconciliation between `cmd/kiroxy/import_json.go` (sets `expires_at = time.Now().Unix() + ExpiresIn`) and kiro_login.py output (sets `addedAt` at login time). LoC estimate: 20-50.
- **CLOSED: Workspace profileArn collision in dedupe key (BUG 4)** (2026-05-13 Phase G.BATCH). Google Workspace accounts within the same Kiro org share a `profileArn`, so the prior cascade silently overwrote earlier imports (10 operators → 1 vault entry). Resolution in `tools/onboard/onboard.py::_dedupe_key` + `tools/onboard/kiro_oauth.py::jwt_sub_or_email` + `cmd/kiroxy/import_json.go::deriveAccountID` as a 4-layer cascade (email → JWT claim → profileArn → token prefix). Schema gains an `email` field (via `--email` CLI flag). Legacy JSON files without `email` continue to import via fallback layers; operators on pre-v1.0.1 Workspace vaults should `rm tokens.db` and re-import. Collision detection at import time prevents silent overwrites of different tokens under the same id (requires `-allow-overwrite` to rotate in place). 16 new Go tests + 68 new Python tests cover the cascade, collision, state, and classification paths. LoC landed: ~1800 across 4 commits (c1–c5 of the Phase G.BATCH overnight run).
- **KIROXY_UPSTREAM_URL env var** — Phase L mock_kiro cannot be integration-tested through kiroxy without this. Simple baseURL override already exists at `kiroclient.WithBaseURL(...)`; just needs env plumbing in `internal/config/config.go`. LoC: 10-20.

## Phase G — Onboarder follow-ups (post G.0 + G.1 + G.FIX)

G.0 scaffold + G.1 single-account flow landed in v0.3.0.

### G.1 auto-login status: UPDATED by Phase G.FIX (2026-05-12 22:xx)

Phase G.FIX shipped layered stealth engineering targeting Google's
anti-bot defenses:

- **Layer 1** — warm profile persistence (Camoufox `persistent_context=True` + YouTube/Google/GitHub pre-warmup with 7-day marker).
- **Layer 2** — residential proxy support (`KIROXY_ONBOARD_PROXY` env + `--proxy` flag + egress validation + geoip pass-through).
- **Layer 3** — human-like interaction (burst-pause typing, typo injection, curved mouse drift).
- **Layer 4** — challenge detection + manual-solve recovery (`--challenge-mode {auto,manual,skip}`, 7 challenge kinds with localized phrase patterns).
- **Layer 5** — session reuse (free via Layer 1; profile dirs persist forever).
- **Layer 6** — fingerprint diagnostic tool (`fingerprint_check.py` against sannysoft + creepjs).

Honest reliability band (documented in `tools/onboard/README.md`):

- Fresh Gmail + residential proxy + warmed profile + no 2FA: **65-80%**
- Fresh Gmail, no proxy: **25-45%**
- Previously-automated account + proxy: **30-50%**
- Account with 2FA: **0% full-auto**; challenge-mode=auto prompts the operator and resumes.
- Google-flagged account: **5-15%**; fall back to manual sign-in via `kiro_login.py` or `--challenge-mode manual`.

85 unit tests pass in `tools/onboard/`. Live-Google validation is the operator's responsibility — TESTING.md has the checklist.

`onboard.py` is no longer a "best-effort" tool with an empty toolkit; it ships a real layered stealth stack. It is still NOT a guarantee. Google's arms race continues; these numbers may drift.

### Remaining work:

- **P2: G.2 — Credential encryption.** Current onboarder ingests `--password` via CLI arg (visible in `ps`) or stdin. Batch mode (G.3, now shipped) reads plain `email:password` lines. Options: age (modern, portable, no keyring dependency) or macOS Keychain (OS-integrated; requires `security(1)` shelling). Preferred: age with password-derived keys; fall back to Keychain on macOS if user opts in. Deliverable: `tools/onboard/credentials.enc` generator + decrypter; `batch.py --credentials-file credentials.enc` path that reads the decrypted password for each entry only in-memory.
- **CLOSED: G.3 — Batch mode with state + rate limit + classification** (2026-05-13 Phase G.BATCH). `tools/onboard/batch.py` drives N single-account onboards single-threaded with `batch_state.json` for resume, `--cooldown-s` per account (default 60s), failure classification (transient vs hard), safety aborts (3 consecutive hard fails in last 5 attempts OR Camoufox crash rate > 20% on ≥ 5 samples), per-account log files under `batch_logs/`, collision detection against the output JSON, and a loud warning when `KIROXY_ONBOARD_PROXY` is unset. 48 tests pass without spawning browsers (injected `FakeRunner` + `FakeSleep`). Parallel onboards explicitly deferred — Google's per-IP rate-limit tripping would cook accounts; single-threaded is intentional for v1.
- **CLOSED: G.4 — Retry logic + failure classification** (2026-05-13 Phase G.BATCH, folded into G.3). Classification table in `batch.py` maps exit code + stderr substrings to transient (network/browser/timeout, retry up to `--max-retries`) vs hard (blocked/2FA/wrong_pass/consent/unknown, sticky). 13 classification-unit tests + 7 integration tests.
- **P3: G.5 — Polish, progress UI, docs.** tqdm-style progress for batch, rich status lines, README walkthrough with screenshots, retry playbook. (G.3 ships plain text output — functional but spartan.)

## Phase 2 (post-v0.1.0)

- **P1 (PROMOTED from P2): Wire pool-mode token refresher for `source="import-accounts"` and `source="import-accounts-json"` accounts.** Current state: `pool.TokenGetter.GetToken()` reads `Bundle.Metadata` for `profile_arn` (v0.2.2) but does NOT trigger refresh when the access_token expires. Imported accounts work only during the `expires_in` window (~1h for Desktop-flow tokens) because the kiroclient used for the pool path has no `WithTokenRefresher` (only `KIROXY_KIRO_DB_PATH` mode does). Scope: extend `pool.TokenGetter` + `main.go` to call `auth.refreshSocialToken` for Desktop-flow accounts (`authMethod="social"` in metadata) when rotation is needed; persist rotated refresh_token + access_token back to the vault. This is the next release's defining feature — without it, v0.3.0 is usable only for short-lived testing.
- **P2: Pool tier-awareness — warn/error when a Pro-tier model is requested but the picked account is Free tier.** Needs tier metadata in vault schema or a runtime probe (one-shot call after first refresh).
- **`opencode-config` subcommand** — **CLOSED in v0.3.0 (Phase F).** Ships 7 resolver-verified Claude model IDs with a `-models` filter. The 13-label Pro tier list was *not* adopted literally: `kiro/*` display labels silently rewrite to `claude-sonnet-4-6` in `internal/models/models.go`, so emitting them in opencode config would cause silent-fallback billing misattribution. See OVERNIGHT_LOG Phase F entry for the mapping logic. If Anthropic-family non-Claude models (`kiro/deepseek-3.2`, `kiro/glm-5`, etc.) later appear in the resolver table, `opencode-config` can be extended to include them.
- **AWS Builder ID device-code OAuth inside `kiroxy add-account`** — **CLOSED in v0.2.0 (Phase B).**
- **`--json` flag for `list-accounts` and `status`** — machine-readable output. — P3
- **Interactive `--yes` / `-y` flag on `remove-account`** when/if multi-user lands. — P3
- **OpenAI-compatible surface** `/v1/chat/completions` + `/v1/models` — **CLOSED in Phase J (Unreleased).** Translation shim in `internal/openai` over the existing Anthropic pipeline; see `docs/OPENAI.md`. Follow-ups filed below as P2.
- **OpenAI surface follow-ups (Phase J → v1.1+)** — P2:
  - `tool_choice: "required"` and specific function-name forcing (today: 400).
  - `response_format: json_object` / JSON mode passthrough (today: silently ignored).
  - `stream_options.include_usage` opt-in (today: usage always included in final chunk; no client has complained but spec-strict clients may).
  - `/v1/responses` (Assistants-style API surface) — larger scope, separate effort.
  - `/v1/completions` legacy text endpoint (deprecated upstream; include only if a dependent client requires it).
  - `/v1/embeddings` — out of scope without an embeddings-capable backend.
  - https:// image URLs — would require server-side fetch + base64 re-encode; consider if Cursor or Continue start sending them.
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

- **CLOSED in v0.3.x (Phase I).** GitHub Actions CI (`make gate` +
  race + coverage on ubuntu-latest + macos-latest Go 1.26.x).
- **CLOSED in v0.3.x (Phase I).** goreleaser multi-platform binaries
  (linux/darwin × amd64/arm64 tarballs + SHA-256 checksums, tag-triggered).
- **NEW (P1, follow-up from Phase I).** Dedicated `Dockerfile.release`
  that packages the pre-built goreleaser binary, plus a goreleaser
  `dockers:` block that publishes `ghcr.io/<owner>/kiroxy:{{.Version}}`
  and `:latest`. The current `Dockerfile` is a from-source multi-stage
  build and does not fit the release-consume pattern.
- **NEW (P3, follow-up from Phase I).** Collapse the CI strict lane.
  The daily `vuln.yml` workflow handles govulncheck separately from
  `ci.yml`; the opt-in `KIROXY_CI_STRICT=1` path is local-only today.
  Future iteration: add a `strict` matrix cell to `ci.yml`.
- Homebrew tap (and the release-workflow glue to publish to it).
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
- **Phase 2.5.2: wire singleflight.Group.Do around refreshOne** (SURFACED 2026-05-12 Phase 2.5.1 concurrency test). Current state: `RefreshConfig.group singleflight.Group` field exists at `internal/pool/refresh.go:60` but is NEVER invoked. Concurrent `TokenGetter.GetToken` calls against an expired account all call `refreshOne` independently — first to `vault.Reserve` wins; losing callers receive a wrapped `tokenvault.ErrLockHeld` error instead of waiting for the winner's result. Observable symptom: under sudden burst load (e.g. Claude Code parallel tool calls), N-1 requests of a batch fail with "reserve: another reservation in-flight". Fix: wrap `refreshOne` invocation in `cfg.group.Do(key, func() ...)` inside `TokenGetter.GetToken`; N-1 waiters get the winner's refreshed bundle instead of attempting their own Reserve. `refresh_concurrent_test.go::TestRefreshFn_ConcurrentCallsAreSerialized` logs the observed failure and should tighten from "at least 1 succeeded" to "all 50 succeeded" after the fix. LoC: 10-20 in pool.go + test assertion tightening.

