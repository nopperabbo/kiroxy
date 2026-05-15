# Contributing to kiroxy

Thanks for taking interest. kiroxy is a single-user, self-hosted tool maintained by [@nopperabbo](https://github.com/nopperabbo); the project is public so others can audit, fork, and learn from it. Outside contributions are welcome but lightly gated — please read this document before opening a PR.

## Project Philosophy

kiroxy does **one thing well**: turn a Kiro IDE subscription into an Anthropic Messages API endpoint. We deliberately **do not** want to become:

- A multi-tenant gateway (use [LiteLLM](https://github.com/BerriAI/litellm) or [Portkey](https://github.com/Portkey-AI/gateway))
- A multi-provider router (Gemini, OpenAI, etc.)
- A SaaS service
- A general-purpose proxy framework

If your contribution moves the project toward those directions, it will likely be rejected. Open an issue first to discuss scope.

## Before You Open a PR

1. **Open an issue first** for anything beyond a typo fix or one-line bug. Sync on the approach.
2. **Check the BACKLOG.md** — your idea may already be planned, deferred, or explicitly rejected with reasoning.
3. **Read `docs/ARCHITECTURE.md`** — the engineering overview explains the boundaries and constraints.
4. **Read `CHANGELOG.md`** — recent changes reveal the project's current direction and conventions.

## Development Setup

### Prerequisites

- **Go 1.26+** with `GOEXPERIMENT=jsonv2` enabled
- **Node 20+** + **pnpm 9+** (for the Mansion dashboard)
- **Astro 5+** (for `web/landing/`)
- **Python 3.10+** (only if working on the onboarder tool)
- **Make** (or run commands directly from the Makefile)

### Build

```bash
# Backend
GOEXPERIMENT=jsonv2 go build -o ./kiroxy ./cmd/kiroxy

# Mansion dashboard (rebuild before binary if you change Svelte source)
cd internal/server/mansion/client
pnpm install
pnpm run build

# Landing page (independent of binary)
cd web/landing
pnpm install
pnpm run build  # output to web/landing/dist/

# Run all tests
go test ./...

# Lint
gofmt -d .
go vet ./...
```

### Run Locally

```bash
./kiroxy serve -port 8787
# Health check
curl http://localhost:8787/healthz
# Dashboard
open http://localhost:8787/dashboard-mansion
```

## PR Guidelines

### Commit Messages

Follow conventional commits: `<type>(<scope>): <subject>`

Types: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `chore`, `polish`

Scopes vary by area:
- `kiroclient`, `pool`, `messages`, `auth` — backend
- `mansion` — dashboard
- `landing` or `web` — landing page
- `docs`, `ci`, `gitignore` — meta

Examples:
- `feat(mansion): make logs histogram bins clickable`
- `fix(pool): cool down failed accounts during rotation`
- `polish(landing): hero topology pulse animation`

### Commit Hygiene

- **One logical change per commit.** Don't bundle a feature with a refactor.
- **Keep dependencies minimal.** No new npm or Go modules without discussion in the issue.
- **No build artifacts.** `dist/`, `node_modules/`, and binaries are gitignored — keep them that way.

### Code Style

**Go:**
- `gofmt` everything (CI will fail otherwise)
- Use `slog` for structured logging, not `fmt.Println`
- Prefer typed errors with `errors.Is/As` over string matching
- Tests live alongside source: `foo.go` + `foo_test.go`

**TypeScript / Svelte:**
- Run `pnpm exec svelte-check` before committing — 0 errors required
- Use `$state` / `$derived` runes (Svelte 5), not legacy `let` / `$:` syntax
- No Tailwind, no shadcn, no icon libraries — inline SVG only
- Brand identity is locked: warm charcoal + amber, JetBrains Mono + Inter, hairline borders, max border-radius 6px

**CSS:**
- Use `--c-*` design tokens from `tokens.css` — never hard-code colors
- The amber budget has 5 roles: wordmark cursor, live dot, primary CTA, keyboard pills, section underlines. Adding a 6th requires retiring one and justifying the swap.
- Motion budget: 4 ambient idle-loop animations max. See `motion-budget.css`.

### Tests

- **Backend:** Go unit tests for any logic change. Integration tests (using real or mocked Kiro endpoint) for protocol changes.
- **Frontend:** Visual changes verified via Playwright DOM probes; the multimodal review path is encouraged but not required.
- **Mobile:** Test at iPhone 13 emulation (390px viewport) — Pool view must stay under 6000px height.

## What Counts as a Good First Issue

If you want to contribute but don't have a specific feature in mind:

- Check issues labeled [`good first issue`](https://github.com/nopperabbo/kiroxy/issues?q=is%3Aissue+is%3Aopen+label%3A%22good+first+issue%22)
- Add a missing test for a function in `internal/`
- Improve a documentation page in `docs/`
- Translate a section of the landing page (currently English-only)
- Add a sample integration in `docs/SAMPLE_RUN.md` for a client kiroxy hasn't been tested with

## What Will Be Closed

- PRs that change the project scope (multi-tenant, multi-provider, SaaS direction)
- PRs adding new dependencies without prior issue discussion
- PRs introducing UI frameworks (Tailwind, shadcn, etc.)
- PRs with build artifacts committed
- PRs without a corresponding issue for non-trivial changes
- PRs that violate the brand identity / motion budget without justification

## License

By contributing, you agree your contributions will be licensed under the [MIT License](LICENSE). The `NOTICE` file tracks meaningful upstream attributions.

## Questions?

Open a [Discussion](https://github.com/nopperabbo/kiroxy/discussions) (Q&A category) or file an [Issue](https://github.com/nopperabbo/kiroxy/issues). For sensitive matters, see [SECURITY.md](SECURITY.md).
