# kiroxy — Project Instructions
# This file is loaded automatically when working in this project directory.
# It supplements the global AGENTS.md with kiroxy-specific rules.

---

## Project Identity

- **Name**: kiroxy
- **Type**: Single-user, self-hosted HTTP proxy (Kiro → Anthropic API)
- **Language**: Go 1.26+, `GOEXPERIMENT=jsonv2`
- **Frontend**: Svelte 5 (Mansion dashboard), Astro (landing page)
- **Build**: `make build` (pins GOEXPERIMENT + stamps version)
- **Test**: `make test` / `go test ./...`
- **Full gate**: `make gate` (build + vet + fmt + test)
- **Version**: v1.4.0 (public release, MIT)
- **Repo**: https://github.com/nopperabbo/kiroxy

---

## Architecture Rules (NEVER violate)

### Package Boundaries
```
cmd/kiroxy/          CLI entry points (subcommands)
        ↓
internal/server/     HTTP server, routing, middleware, dashboard embedding
        ↓
internal/messages/   Core request handler (the hot path)
        ↓
internal/reqconv     Anthropic → Kiro payload conversion
internal/kiroclient  Upstream Kiro HTTP client
internal/respconv    Kiro SSE → Anthropic SSE conversion
        ↓
internal/pool        Multi-account LRU selection + cooldown
internal/tokenvault  SQLite credential vault + gen-lock OAuth refresh
        ↓
internal/config      Environment variable configuration
internal/logging     slog structured logging
internal/metrics     Prometheus-style metrics
internal/tracing     OpenTelemetry distributed tracing
internal/auth        API key middleware
internal/httpx       HTTP utilities
internal/safego      Safe goroutine helpers
internal/testutil    Test helpers (NEVER import in production code)
```

### Import Rules

- `cmd/` → `internal/*` (one direction only)
- `internal/messages` → `reqconv`, `respconv`, `kiroclient`, `pool`
- `internal/pool` → `tokenvault`
- `internal/server` → `messages`, `auth`, `logging`, `metrics`
- **NEVER** circular imports
- **NEVER** import `testutil` from production code
- **NEVER** import `server` from any other internal package

### Mandatory Patterns

| Pattern | Rule |
|---|---|
| Error handling | Use `fmt.Errorf("context: %w", err)` with sentinel errors + `errors.Is/As` |
| Logging | Use `slog`, NEVER `fmt.Println` or `log.*` |
| HTTP responses | Always set headers before writing body, handle `context.Canceled` |
| Config | `KIROXY_*` env vars, validated in `internal/config/` |
| Tests | Table-driven, alongside source (`foo.go` + `foo_test.go`) |
| Concurrency | `context.Context` for cancellation, `safego` for goroutines |
| Token refresh | Generation-lock pattern (see tokenvault) — NEVER bypass |
| SSE streaming | Flush after every event, respect `context.Done()` |
| Commit messages | Conventional: `feat(scope)`, `fix(scope)`, `docs`, `polish`, `chore` |
| Commit scopes | `kiroclient`, `pool`, `messages`, `auth`, `mansion`, `landing`, `docs`, `ci` |

---

## Frontend Rules (Mansion Dashboard)

### Tech Stack
- Svelte 5 with `$state` / `$derived` runes (NOT legacy `let` / `$:`)
- No Tailwind, no shadcn, no component libraries
- Inline SVG only (no icon libraries)
- Bundle embedded in Go binary via `go:embed`

### Brand Identity (LOCKED — do not change)
- **Colors**: Warm charcoal + aged-brass amber, via `--c-*` CSS tokens
- **Typography**: JetBrains Mono (data), Inter (UI text)
- **Borders**: Hairline, max border-radius 6px
- **Themes**: Dark + Light, both WCAG AA compliant

### Design Budgets
- **Amber budget**: 5 roles MAX — wordmark cursor, live dot, primary CTA, keyboard pills, section underlines. Adding a 6th requires retiring one.
- **Motion budget**: 4 ambient idle-loop animations MAX. See `motion-budget.css`.
- **CSS**: Use `--c-*` tokens from `tokens.css`, NEVER hard-code colors

### Build
```bash
cd internal/server/mansion/client
pnpm install && pnpm run build   # must run before `make build`
```

---

## Anti-Patterns (STOP and correct)

- ❌ Adding Tailwind, shadcn, or any CSS framework
- ❌ Adding icon libraries (use inline SVG)
- ❌ Multi-tenant or SaaS features (permanent non-goal)
- ❌ Multi-provider routing (only Kiro upstream)
- ❌ New npm or Go dependencies without issue discussion
- ❌ Build artifacts in git (`dist/`, `node_modules/`, binaries)
- ❌ `fmt.Println` for logging (use `slog`)
- ❌ Raw string error matching (use `errors.Is/As`)
- ❌ Skipping `GOEXPERIMENT=jsonv2` (will fail to build)
- ❌ Importing `testutil` in production code
- ❌ Exceeding amber or motion budgets without justification

---

## Key Files Reference

| File | Purpose | When to touch |
|---|---|---|
| `cmd/kiroxy/main.go` | CLI entry + subcommands | Adding new subcommands |
| `internal/server/server.go` | HTTP routing + middleware | Adding routes |
| `internal/messages/handler.go` | Core request handler | Modifying request flow |
| `internal/messages/execute.go` | Request execution | Modifying upstream calls |
| `internal/reqconv/` | Anthropic → Kiro conversion | Protocol changes |
| `internal/respconv/` | Kiro → Anthropic conversion | Protocol changes |
| `internal/pool/pool.go` | Account selection (LRU) | Pool behavior |
| `internal/tokenvault/vault.go` | Credential storage + refresh | Auth/token changes |
| `internal/config/config.go` | Configuration | Adding `KIROXY_*` env vars |
| `internal/server/mansion/` | Dashboard source (Svelte) | UI changes |
| `web/landing/` | Landing page (Astro) | Marketing page |
| `Makefile` | Build targets | Build changes |
| `CHANGELOG.md` | Release history | After every feature/fix |
| `BACKLOG.md` | Future work tracking | When adding/completing items |
| `docs/ARCHITECTURE.md` | Engineering overview | Architecture changes |
| `.sisyphus/plans/` | AI session plans | Phase planning |

---

## Documentation Sync (MANDATORY after every feature)

1. `CHANGELOG.md` — add entry under current version
2. `README.md` — update if user-facing
3. `docs/ARCHITECTURE.md` — update if architecture changes
4. `docs/TROUBLESHOOTING.md` — update if new failure modes
5. `BACKLOG.md` — mark items as done / add new items

---

## Current State (update periodically)

- Version: v1.4.0
- Go packages: 23
- Test files: 94
- Total Go LOC: ~41K
- Dashboard: Svelte 5, 7 views
- Upstream: Kiro (AWS CodeWhisperer)
- Accounts supported: 78 (pool)
- Credential storage: SQLite vault (mode 0600)
