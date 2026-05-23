# Contributing to kiroxy

<div align="center">
  <br>
  <strong>Thanks for taking interest in kiroxy.</strong>
  <br>
  <sub>Outside contributions are welcome but lightly gated — please read this document before opening a PR.</sub>
  <br>
  <br>
</div>

---

## Philosophy

kiroxy does **one thing well**: turn a Kiro IDE subscription into an Anthropic Messages API endpoint.

We deliberately **do not** want to become:

| ❌ Not this | ✅ Use instead |
|:---|:---|
| Multi-tenant gateway | [LiteLLM](https://github.com/BerriAI/litellm), [Portkey](https://github.com/Portkey-AI/gateway) |
| Multi-provider router | [OpenRouter](https://openrouter.ai) |
| SaaS service | — |
| General-purpose proxy framework | [Kong](https://github.com/Kong/kong), [Envoy](https://github.com/envoyproxy/envoy) |

If your contribution moves toward those directions, it will likely be rejected. **Open an issue first** to discuss scope.

---

## Before You Open a PR

1. **Open an issue first** for anything beyond a typo fix
2. **Check [existing issues](https://github.com/nopperabbo/kiroxy/issues)** — your idea may already be planned or explicitly rejected
3. **Read [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md)** — understand the boundaries
4. **Read [`CHANGELOG.md`](CHANGELOG.md)** — recent changes reveal current direction

---

## Development Setup

### Prerequisites

| Tool | Version | Purpose |
|:---|:---|:---|
| Go | 1.26+ | Backend (`GOEXPERIMENT=jsonv2` required) |
| Node | 20+ | Dashboard + landing page |
| pnpm | 9+ | Package manager |
| Make | any | Build orchestration |

### Build & Run

```bash
# Backend
make build                    # or: GOEXPERIMENT=jsonv2 go build -o ./kiroxy ./cmd/kiroxy

# Dashboard (rebuild before binary if you change Svelte source)
cd internal/server/mansion/client && pnpm install && pnpm run build

# Landing page
cd web/landing && pnpm install && pnpm run build

# Run
./kiroxy serve
open http://localhost:8787/dashboard-mansion
```

### Quality Gates

```bash
make gate          # fmt + vet + build + test (required before commits)
make test-race     # race detector
make lint          # golangci-lint (25+ linters)
make bench         # performance benchmarks
```

---

## PR Guidelines

### Commit Messages

[Conventional Commits](https://www.conventionalcommits.org/): `<type>(<scope>): <subject>`

**Types:** `feat` · `fix` · `docs` · `style` · `refactor` · `perf` · `test` · `chore` · `polish`

**Scopes:**

| Scope | Area |
|:---|:---|
| `kiroclient`, `pool`, `messages`, `auth` | Backend |
| `mansion` | Dashboard |
| `landing`, `web` | Landing page |
| `docs`, `ci` | Meta |

**Examples:**
```
feat(mansion): make logs histogram bins clickable
fix(pool): cool down failed accounts during rotation
polish(landing): hero topology pulse animation
```

### Code Style

<details>
<summary><strong>Go</strong></summary>

- `gofmt` everything (CI enforces)
- Use `slog` for structured logging
- Prefer typed errors with `errors.Is/As`
- Tests live alongside source: `foo.go` + `foo_test.go`

</details>

<details>
<summary><strong>TypeScript / Svelte</strong></summary>

- Run `pnpm exec svelte-check` — 0 errors required
- Use `$state` / `$derived` runes (Svelte 5)
- No Tailwind, no shadcn, no icon libraries — inline SVG only
- Brand identity is locked (see below)

</details>

<details>
<summary><strong>CSS / Brand Identity</strong></summary>

- Use `--c-*` design tokens from `tokens.css` — never hard-code colors
- **Palette:** warm charcoal + aged-brass amber
- **Typography:** JetBrains Mono (display) + Inter (text)
- **Borders:** hairline, max `border-radius: 6px`
- **Amber budget:** 5 roles only (wordmark cursor, live dot, primary CTA, keyboard pills, section underlines). Adding a 6th requires retiring one.
- **Motion budget:** 4 ambient idle-loop animations max

</details>

### Tests

- **Backend:** Unit tests for any logic change. Integration tests for protocol changes.
- **Frontend:** Playwright DOM probes for visual changes.
- **Mobile:** Test at iPhone 13 (390px viewport).

---

## Good First Issues

- Issues labeled [`good first issue`](https://github.com/nopperabbo/kiroxy/issues?q=is%3Aissue+is%3Aopen+label%3A%22good+first+issue%22)
- Add a missing test for a function in `internal/`
- Improve a documentation page in `docs/`
- Add a sample integration for an untested client

---

## What Will Be Closed

- ❌ PRs that change project scope (multi-tenant, multi-provider, SaaS)
- ❌ PRs adding dependencies without prior issue discussion
- ❌ PRs introducing UI frameworks (Tailwind, shadcn, etc.)
- ❌ PRs with build artifacts committed
- ❌ PRs without a corresponding issue for non-trivial changes
- ❌ PRs violating brand identity / motion budget without justification

---

## License

By contributing, you agree your contributions will be licensed under the [MIT License](LICENSE).

---

<div align="center">
  <sub>
    <a href="https://github.com/nopperabbo/kiroxy/discussions">💬 Questions? Open a Discussion</a>
    &nbsp;&nbsp;•&nbsp;&nbsp;
    <a href="SECURITY.md">🔒 Security concerns? See SECURITY.md</a>
  </sub>
</div>
