# Anchor: kiroxy current state of art

> Compiled 2026-05-15 (post v1.4.0+3, after credit-monitoring + tier badge ship). This is the BASELINE for comparison against peer projects researched in Stream A/B/C/D dossiers.

---

## 1. Identity

| Field | Value |
|---|---|
| Repo | https://github.com/nopperabbo/kiroxy |
| Language | Go 1.26.2 |
| Build constraint | `GOEXPERIMENT=jsonv2` (uses `encoding/json/v2`) |
| License | MIT |
| Version | v1.4.0 (11 tags total: v0.1.0-mvp through v1.4.0) |
| Primary surface | `/v1/messages` (Anthropic) + `/v1/chat/completions` (OpenAI) |
| Total tests | 32 packages green (~1,200+ test cases) |
| Internal packages | 23 modules (anthropic, auth, builderid, config, doctor, httpx, kiroclient, kiroproto, logging, messages, metrics, models, openai, pool, reqconv, respconv, safego, server, testutil, tokencount, tokenvault, toolsearch, tracing) |

---

## 2. Feature Inventory

### Onboarding methods
- **Builder ID device flow** (cmd/kiroxy add-account, internal/builderid) — PKCE OAuth2 to AWS IDC
- **JSON paste import** (cmd/kiroxy import) — for kiro-cli-derived tokens
- **Bulk JSON import** (cmd/kiroxy import-json) — `kiro_tokens.json` format from batch onboarder
- **Camoufox-based onboarder** (tools/onboard/, Python) — anti-detection browser automation for Builder ID at scale. Single-threaded by design (Google rate limit).

### Token lifecycle (Phase 2.5)
- Auto-refresh via `pool.RefreshConfig.RefreshFn` (internal/pool/refresh.go)
- Singleflight coalescing (one refresh call per account even with concurrent requests)
- Vault-side optimistic concurrency (generation field, ErrLockHeld)
- Proactive refresh for social accounts when ExpiresAt < now+skew
- Reactive refresh on 401 from upstream
- Phase 6.2: auth_method=idc tagged for Builder ID accounts (v1.4.0)
- Phase 6.3: scaffold metadata fields for IdC refresh (deferred plumbing)

### Pool selection
- Weighted-LRU with health (success_rate × usage_remaining_pct × success_streak)
- Session stickiness (60s window, configurable)
- Capacity shortage (`INSUFFICIENT_MODEL_CAPACITY`) excluded from cooldowns to prevent stampede
- Structural error quarantine (Phase 6.1) for `UnknownOperationException`, `AccessDeniedException`, `ResourceNotFoundException`, `UnrecognizedClientException`, `InvalidSignatureException`
- Configurable upstream pool retries (`upstreamPoolRetries`)

### Credit monitoring (just shipped v1.4.0+2)
- 60s poll cycle of `q.<region>.amazonaws.com/getUsageLimits` per account
- ForcePoll hook on chat path 429
- Pool weight factors `PercentRemaining` (50% sisa = half pool weight, <10% = floor)
- Real-time fields surfaced: monthly_used / monthly_cap / overage / next_reset / days_until_reset
- `KIROXY_USAGE_POLL_DISABLED=1` kill-switch

### Subscription tier awareness (just shipped v1.4.0+2)
- Detects free/pro/pro+/power from `subscriptionInfo.type`
- Cap-based fallback (50/1000/2000/10000)
- Frontend tier badge in mansion dashboard
- NO tier-aware routing yet (pending feature)

### Anthropic API surface
- `POST /v1/messages` (sync + SSE streaming)
- `POST /v1/messages/count_tokens` (tiktoken estimator + actual via downstream)
- `GET /v1/models` (allow-list of supported Kiro models)
- Tool use: parse + sanitize + passthrough
- Schema sanitization for heterogeneous-anyOf branches (lossy fallback at DEBUG level since Phase 6 fix)

### OpenAI API surface (already shipped, recently added)
- `POST /v1/chat/completions` (sync + SSE streaming)
- `GET /v1/models`
- Translation shim: OpenAI request → Anthropic Messages → Kiro upstream → Anthropic response → OpenAI response

### Observability
- **Prometheus metrics**: `/metrics` endpoint (request count, latency histograms, pool state, errors by kind, cooldown reasons, refresh outcomes)
- **OpenTelemetry tracing**: `internal/tracing/` package fully implemented (NOT YET wired to main.go — open backlog item)
- **Structured logs**: `slog` with account_id, request_id, model, kind, latency injected into all error paths
- **Mansion dashboard** (`/dashboard-mansion`): Svelte SPA with views: live stream, account board (77 rows), pool pulse, models view, logs tail, tools view. Theme system (paper/nord/neon/muji/brutal/linearpremium variants).
- **Legacy dashboard** (`/dashboard`): vanilla JS, simpler view
- **JSON API**: `/dashboard/api/state` (full snapshot), `/dashboard/api/logs` (tail), `/dashboard/api/settings`

### Configuration
- 9 env vars + flag override
- Loopback-only by default (KIROXY_BIND=127.0.0.1)
- Auth: X-Api-Key OR `Authorization: Bearer <token>` header
- Vault perms enforced at chmod 0600 (boot-time check is OPEN backlog item)
- Graceful shutdown: 60s default for SSE drain (Phase 6.4)

### Testing
- Unit: 32 packages
- Integration: refresh_e2e (vault → pool → kiroclient), refresh_concurrent (50-goroutine concurrency), import_json (full vault round-trip)
- Mock kiro server: `scripts/loadtest/mock_kiro/` — load testing target
- No fuzz tests yet

### Reliability features
- Generated tag stamps (`v1.4.0-N-gSHA-dirty`) via Makefile + ldflags
- Goreleaser config for multi-platform binaries
- Docker support: Dockerfile + docker-compose.yml
- Healthcheck endpoint pattern (TODO — open backlog)

### Notable gaps (from BACKLOG.md)
- **P0**: SSE keepalive pings every 15s during slow thinking (~10 LoC)
- **P1**: OTel tracing wire-up (`tracing.Init` never called from main.go) (~20 LoC)
- **P1**: OIDC client-secret rotation detection (FAIL-006) (~15 LoC)
- **P1**: Boot-time vault file mode check (FAIL-035) (~5 LoC)
- **P1**: Tests for `internal/config` (~100 LoC test coverage)
- **P1**: `docs/alerts.yml` Prometheus alert rules
- **P2**: Pool tier-aware routing (warn/error if Pro-only model picks Free account)

---

## 3. License-relevant fact

**MIT licensed** — kiroxy can borrow patterns from MIT/BSD/Apache peers. CANNOT incorporate code from AGPL (`chaogei/Kiro-account-manager`), GPL, or CC BY-NC-SA peers (`hj01857655/kiro-account-manager`). Pattern study is allowed; verbatim copy is not.

---

## 4. Comparison Slots

This anchor document feeds these comparison axes into peer dossiers:

1. **Onboarding diversity**: Does peer have unique flows kiroxy doesn't?
2. **Pool selection strategy**: Does peer have unique scoring kiroxy doesn't?
3. **Token refresh patterns**: Singleflight? Optimistic concurrency? Vault generation?
4. **Credit/usage polling**: Endpoint shape, frequency, pool weight integration
5. **Tier detection / routing**: How do peers handle free vs pro?
6. **API surfaces exposed**: Anthropic + OpenAI? Either? Custom?
7. **Observability breadth**: Metrics? Traces? Dashboards? Logs?
8. **Operational reliability**: Restart safety, vault perms, secret hygiene
9. **OSS hygiene**: License clarity, docs, CI, test coverage
10. **Unique innovations**: What does each peer do that no one else does?
