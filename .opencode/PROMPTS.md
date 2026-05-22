# kiroxy — Prompt Templates
# Copy-paste these into your AI session when needed.
# These supplement the global PROMPTS_STANDARD.md with kiroxy-specific workflows.

---

## 🚀 Kiroxy Improvement Sprint — Full Scope

```
You are improving the kiroxy project at this workspace. kiroxy is a self-hosted Go 1.26 proxy (GOEXPERIMENT=jsonv2) that exposes Kiro IDE subscriptions as an Anthropic Messages API.

Read .opencode/AGENTS.md first — it has architecture rules, import boundaries, and anti-patterns.

## Current State
- Version: v1.4.0 (production, MIT)
- Go packages: 23 in internal/
- Test files: 94
- Overall coverage: 66.7%
- All tests pass: `GOEXPERIMENT=jsonv2 go test ./...`
- Build: `make build` (pins GOEXPERIMENT + stamps version)
- Full gate: `make gate` (build + vet + fmt + test)

## Coverage Weak Spots (fix these FIRST)
| Package | Current | Target |
|---|---|---|
| cmd/kiroxy | 23.6% | 50%+ |
| internal/messages | 37.8% | 70%+ |
| internal/anthropic | 47.9% | 70%+ |
| internal/tokencount | 50.0% | 70%+ |
| internal/httpx | 66.7% | 80%+ |

## Open P1 Backlog (quick wins, do AFTER coverage)
1. **API key min length** — reject KIROXY_API_KEY < 16 chars at startup with WARN. ~10 LoC in internal/config/config.go
2. **OIDC rotation detection** — log warning when N consecutive IDC refresh 401s hit same clientId. ~15 LoC in internal/tokenvault/
3. **Ship docs/alerts.yml** — Prometheus alert rules for PoolDepleted, HighErrorRate, RefreshFailing, UpstreamLatencyHigh

## Open P1 Reliability Fix
4. **Singleflight around refreshOne** — wire singleflight.Group.Do in internal/pool/pool.go around refreshOne. Currently RefreshConfig.group field exists at internal/pool/refresh.go:60 but is NEVER invoked. Under burst load, N-1 requests fail with "reserve: another reservation in-flight". Fix: ~10-20 LoC. Tighten refresh_concurrent_test.go assertion from "at least 1 succeeded" to "all 50 succeeded".

## Open P3 Quick Wins (do if time permits)
5. **Wire RecordLatency** — call Pool.RecordLatency(id, dur) from kiroclient response path. ~10 LoC in internal/messages/execute.go
6. **Expose stickiness state** — add sticky_sessions_count to /dashboard/api/state. ~20 LoC

## Rules
- Write tests FIRST (TDD), then implement
- Run `make gate` after every change
- Update CHANGELOG.md for every feature/fix
- Follow conventional commits: feat(scope), fix(scope)
- Scopes: config, pool, messages, tokenvault, auth, mansion, docs
- NEVER violate architecture rules in .opencode/AGENTS.md

## Order of Execution
1. Coverage push (messages → cmd → anthropic → tokencount)
2. P1 quick wins (API key, OIDC, alerts.yml)
3. Singleflight fix
4. P3 if time permits
5. Final `make gate` + CHANGELOG update
```

---

## 🧪 Coverage Push — internal/messages

```
Increase test coverage for internal/messages/ from 37.8% to 70%+.

This is the CORE HOT PATH — every request through kiroxy goes through this package.

Key files to test:
- handler.go — Service.Handle (main request handler)
- execute.go — upstream invocation logic
- response.go — SSE response writing
- thinking.go — thinking budget resolution (already has resolveThinkingBudget)
- toolsearch.go — tool search proxy logic
- capture.go — request/response capture
- gate_writer.go — tier-gated streaming writer
- tier_gate.go — tier gating logic

Strategy:
1. Run `GOEXPERIMENT=jsonv2 go test -cover ./internal/messages/` to see current state
2. Identify uncovered functions via `go tool cover -func=coverage.out`
3. Write table-driven tests for each uncovered public function
4. Mock external deps (pool, kiroclient, tokenvault) using interfaces
5. Test error paths: upstream 429, 502, context canceled, malformed SSE
6. Test thinking paths: with/without budget, effort levels, empty thinking response
7. Run `make gate` after each batch of tests

Target: every public function has at least 1 happy path + 1 error path test.
```

---

## 🔐 Security Hardening Sprint

```
Security improvements for kiroxy. Read SECURITY.md and BACKLOG.md first.

Tasks (in order):
1. **API key min length** (P1)
   - File: internal/config/config.go
   - Reject KIROXY_API_KEY < 16 chars at startup
   - Log slog.Warn with reason, don't abort (operator might be on loopback)
   - Add test in internal/config/config_test.go
   - ~10 LoC

2. **Vault at-rest encryption** (P2)
   - File: internal/tokenvault/vault.go
   - User-supplied passphrase via KIROXY_VAULT_PASSPHRASE env
   - Key derivation: argon2id (time=1, memory=64MB, threads=4)
   - Cipher: XChaCha20-Poly1305 (via golang.org/x/crypto/chacha20poly1305)
   - Encrypt token fields before SQLite write, decrypt on read
   - Backward compatible: detect unencrypted vault, offer migration
   - Add tests for encrypt/decrypt round-trip, wrong passphrase, migration

3. **Cosign keyless signatures** (P2)
   - File: .github/workflows/release.yml
   - Add cosign sign step after goreleaser
   - Keyless (Fulcio + Rekor) — no private key to manage
   - Update README install instructions with verify step

4. **Audit zeroize previousRefreshToken** (P2)
   - File: internal/tokenvault/vault.go
   - Rotate out stale refresh tokens after N generations
   - Currently retained forever — limits blast radius

Run `make gate` after each task. Update CHANGELOG.md.
```

---

## 🎨 Mansion Dashboard Enhancement

```
Enhance the Mansion dashboard (Svelte 5) at internal/server/mansion/client/.

CRITICAL RULES (from .opencode/AGENTS.md):
- Svelte 5 runes ($state, $derived) — NOT legacy let/$:
- No Tailwind, no shadcn, no component libraries
- Inline SVG only (no icon libraries)
- Use --c-* CSS tokens from tokens.css
- Amber budget: 5 roles MAX (wordmark, live dot, CTA, keyboard, underlines)
- Motion budget: 4 ambient idle-loop animations MAX
- Max border-radius: 6px
- Fonts: JetBrains Mono (data), Inter (UI)
- WCAG AA contrast in both themes

Build: cd internal/server/mansion/client && pnpm install && pnpm run build
Check: pnpm exec svelte-check (0 errors required)
Then: make build (embeds bundle in Go binary)

Enhancement tasks:
1. Per-model usage stats in Models view — count, p50/p95, error rate per model
2. Stickiness state visualization — show which sessions are pinned to which accounts
3. Per-phase latency in DetailDrawer (DNS/TLS/TTFB/body/total)
```

---

## 🔧 Singleflight Fix (P1 Reliability)

```
Fix the singleflight gap in internal/pool/ that causes burst-load failures.

## Problem
internal/pool/refresh.go:60 has `group singleflight.Group` field but it's NEVER invoked.
Under burst load (e.g., Claude Code parallel tool calls), concurrent GetToken calls
against an expired account all call refreshOne independently. First to vault.Reserve
wins; N-1 callers get wrapped tokenvault.ErrLockHeld error.

Observable symptom: "reserve: another reservation in-flight" errors under load.

## Fix
1. In pool.go (or the file where GetToken calls refreshOne):
   - Wrap refreshOne invocation in cfg.group.Do(accountID, func() ...)
   - N-1 waiters get the winner's refreshed bundle instead of failing

2. Tighten test assertion in refresh_concurrent_test.go:
   - TestRefreshFn_ConcurrentCallsAreSerialized should assert "all 50 succeeded"
   - Currently only asserts "at least 1 succeeded"

## Verification
- Run: GOEXPERIMENT=jsonv2 go test -race -run TestRefreshFn_Concurrent ./internal/pool/
- All 50 goroutines should succeed
- Run: make gate

~10-20 LoC change + test assertion tightening.
```

---

## 📊 Observability Sprint

```
Improve kiroxy's observability for production operations.

1. **Ship docs/alerts.yml** — Prometheus alert rules
   - PoolDepleted: pool_active_accounts == 0 for 5m
   - HighErrorRate: rate(requests_total{status="5xx"}[5m]) > 0.1
   - RefreshFailing: increase(token_refresh_errors_total[15m]) > 5
   - UpstreamLatencyHigh: histogram_quantile(0.95, rate(request_duration_seconds_bucket[5m])) > 30

2. **Wire RecordLatency** — ~10 LoC in internal/messages/execute.go
   - After successful Kiro response, call pool.RecordLatency(accountID, duration)
   - This feeds the EWMA tracker already in pool/health.go
   - Dashboard AvgLatency field becomes accurate (currently always 0)

3. **Expose stickiness in /dashboard/api/state** — ~20 LoC
   - pool.stickiness.Snapshot() already exists
   - Add sticky_sessions_count to top-level state JSON
   - Add sticky_for_session_count per DashboardAccount row

Run `make gate` after each. Update CHANGELOG.
```

---

## 🧹 Housekeeping

```
Quick housekeeping tasks for kiroxy repo hygiene.

1. **SMOKE_TEST.md** — Prepend superseded banner:
   > ⚠️ **SUPERSEDED** — This smoke test was written for v0.2.1 and is no longer
   > accurate. kiroxy v1.4.0 passes all equivalent checks. A fresh smoke test
   > will be written for v1.5.0.

2. **Create .session-handoff.json** at project root:
   {
     "project": "kiroxy",
     "version": "1.4.0",
     "last_session": "YYYY-MM-DD",
     "current_branch": "main",
     "in_progress": [],
     "blocked_on": [],
     "recent_decisions": [],
     "next_tasks": ["Coverage push internal/messages", "API key min length"],
     "gotchas": ["GOEXPERIMENT=jsonv2 required for all Go commands", "pnpm build mansion before make build"]
   }

3. **Coverage badge** — Add to README.md:
   [![Coverage](https://img.shields.io/badge/coverage-66.7%25-yellow?style=flat-square)](internal/)

4. **Makefile: add test-coverage target**:
   test-coverage:
     go test -coverprofile=coverage.out ./...
     go tool cover -func=coverage.out | tail -1
     @echo "HTML report: go tool cover -html=coverage.out"
```
