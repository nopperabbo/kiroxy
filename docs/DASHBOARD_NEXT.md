# Dashboard Next — parallel frontend (Phase H.alt)

**Date:** 2026-05-12
**Status:** Experimental alternative to Phase H dashboard v2.
**Mount point:** `/dashboard-next`
**Coexists with:** `/dashboard` (Phase H, htmx+Alpine-free vanilla JS, already shipped at commit 0564c26 era).

This document captures the design decisions of "Dashboard Next" and the
comparison artifact the operator asked for: LOC, bundle size, build time,
subjective wins/losses vs Phase H's approach.

---

## Charter

Ship a second, parallel operator dashboard using a 2026-state-of-art stack
so the operator can A/B compare with Phase H's vanilla-JS dashboard. Identical
P0 feature surface, intentionally different stack. Winner chosen post-session.

- Phase H ships: vanilla JS + hand-authored CSS + `go:embed` of raw `.html`/`.css`/`.js`.
- Dashboard Next ships: Svelte 5 (runes) + TypeScript + Vite bundle, built
  into `internal/server/assets/next/` and served via `go:embed`.

---

## Stack decisions (defend-in-commit)

### Svelte 5 + runes, NOT SvelteKit
- **Why Svelte 5:** smallest compiled runtime (~1.2KB/component avg) of any
  modern reactive framework, compile-time reactivity means no VDOM overhead,
  `$state` / `$derived` / `$effect` runes give React-hook ergonomics without
  the hook rules or stale-closure traps.
- **Why not SvelteKit:** heavy for an embedded single-page tool. We don't
  need SSR, file-based routing, or server endpoints — the Go server is the
  only backend. SvelteKit would add a ~50KB adapter shim and a build-pipeline
  layer we don't benefit from. Vanilla Svelte + Vite is the surgical choice.
- **Why not Solid:** close competitor but smaller ecosystem, less typing
  support in some surfaces, and we wanted to test Svelte 5's runes migration
  specifically (the killer feature of this year's release).
- **Why not Lit:** web-component overhead (shadow DOM, light-DOM composition
  tradeoffs) isn't paying for itself here. We're not distributing.
- **Why not vanilla JS (what Phase H did):** that IS Phase H. The point of
  this build is to test whether a reactive framework pays for itself at this
  feature surface.

### TypeScript strict
- `strict: true`, `exactOptionalPropertyTypes: true`, `noUncheckedIndexedAccess: true`,
  `verbatimModuleSyntax: true`. Catches the optional-chain bugs that plague
  SSE payload handling.
- `.svelte` files typed via `svelte-check` at build time (part of `pnpm build`).

### Vite 6 + ESM target
- Vite is the default-choice 2026 bundler — Rolldown-backed, sub-second warm
  builds. Hashed asset filenames are irrelevant for us (we `go:embed` the
  whole dist and paths are stable) so we set `build.assetsInlineLimit` low
  and keep filenames deterministic via `rollupOptions.output`.
- Target `esnext` — we ship to local browsers only, modern syntax is free.

### CSS: 2026 primitives, zero framework
- **Cascade layers** — `@layer reset, tokens, base, theme, components, utilities;`
  lets us predict specificity without `!important` ever.
- **OKLCH colors** with P3 gamut — perceptually uniform lightness gives us
  "one click darker" adjustments via `calc()` on the L channel. `color-mix()`
  derives hover/active states from base tokens.
- **`light-dark()` function** — zero-JS theming. We set `color-scheme` on
  `<html>` and every token is defined once as
  `--bg: light-dark(oklch(98% 0 0), oklch(12% 0 250));`.
  Manual override via `color-scheme: only dark`/`only light` on the root.
- **Container queries** — `AccountTable` collapses its cooldown column at
  container width < 600px regardless of viewport. Viewport-size media
  queries are for page layout only.
- **`:has()`** — `.row:has(input:checked) { background: …; }` style-drives
  selection state. Way cleaner than `classList.toggle`.
- **Native CSS nesting** — no Sass, no PostCSS nesting plugin. Vite passes
  through natively.
- **View Transitions** — used for theme toggle and account-drawer open/close.
  `document.startViewTransition()` with a fallback branch.
- **`@starting-style`** for dialog/popover enter transitions without JS.
- **Scroll-driven animations** — progress indicator on the request feed uses
  `animation-timeline: scroll(nearest)` — decorative only, no CLS impact.
- **`@property`** — types `--hue`, `--accent-l` so they animate. Required
  for the "live-update pulse" micro-interaction on data rows.
- **Native `<dialog>`** for the request-detail modal, **popover attribute**
  for tooltips. Zero positioning library.
- **`text-wrap: balance`** on headings, `pretty` on empty-state prose.
- **`field-sizing: content`** on the import JSON textarea — it auto-grows.
- **Subgrid** for the account drawer's aligned label/value pairs.

### Typography: variable fonts, self-hosted
- **JetBrains Mono Variable** for IDs, timestamps, request paths.
- **Inter Variable** for UI chrome and section headings.
- Both served as woff2 from `assets/next/fonts/`. No Google Fonts runtime.
- Type scale: 1.200 ratio (minor third), base `14px`.

### Motion
- **View Transitions** for cross-state changes (theme, drawer, modal).
- **Scroll-driven** for decorative progress indicators only.
- Absolute respect for `prefers-reduced-motion`: all non-essential motion
  collapses to `animation: none; transition: none;` inside
  `@media (prefers-reduced-motion: reduce)`.
- **Zero** Framer Motion / GSAP / Motion One / anime.js.

### Accessibility: WCAG 2.2 AA
- `:focus-visible` 3px outline, high-contrast (meets 2.2 SC 2.4.11).
- Full keyboard flow: Tab order, arrow keys in tables, Esc closes dialogs,
  Cmd-K opens palette, `/` focuses search, `?` shows cheat sheet.
- `aria-live="polite"` on the request feed's status summary for SSE updates.
- `prefers-contrast: more`, `prefers-reduced-transparency` respected.

### Data / state
- Svelte class stores with `$state` runes. Scoped, typed, simple.
- SSE via native `EventSource` wrapped in a class that exposes three
  `$state`-backed reactive fields (`snapshot`, `requests`, `status`).
- No Redux / Zustand / nanostores / TanStack Query. The scale doesn't need
  a cache coordinator — we have one source of truth (SSE), and drawer/modal
  state is local to the component.

### Fetch
- Native `fetch` with a typed wrapper (`lib/api.ts`). All responses have a
  discriminated-union return type: `{ ok: true, data } | { ok: false, error }`.
- No retry library; SSE reconnect is native EventSource behavior.

---

## What we intentionally DON'T use

React, Vue, Angular, Solid, SvelteKit, htmx (Phase H's territory), Alpine,
Tailwind, Bootstrap, Material, Chakra, Ant Design, DaisyUI, shadcn, Framer
Motion, GSAP, anime.js, Motion One, Popmotion, Lucide/Heroicons/Feather
bundles, TanStack Query, SWR, any CDN runtime, any gradient backdrop, any
rounded-2xl shadow-xl card soup, any pastel SaaS palette, any spinner, jQuery.

---

## Performance budgets

| Metric | Budget | Measured |
|---|---|---|
| FCP | < 400ms | TBD |
| LCP | < 800ms | TBD |
| INP | < 100ms | TBD |
| CLS | 0 | TBD |
| JS gzipped | < 50KB | TBD |
| CSS gzipped | < 15KB | TBD |
| Initial HTML | < 3KB | TBD |

Measured via `vite build` stats + `gzip -9c` on final bundles.
Filled in at close-out.

---

## Information architecture

```
┌─────────────────────────────────────────────────────────────────┐
│  KIROXY · NEXT    v0.3.0    uptime 2h14m    ready    127req/0.8%│  topbar
│                                                          theme ⎇│
├─────────────────────────────────────────────────────────────────┤
│  POOL  (3 accounts · 2 healthy · 1 cooldown)          + import  │  section
│  ┌──────────────────────────────────────────────────────────┐   │
│  │  ID         status     req    err    cooldown    last    │   │  header
│  │  acct-1  ● healthy      47     0     —           11:42   │   │  row (✳pulse on update)
│  │  acct-2  ● cooldown     91     3     1m 45s      11:40   │   │
│  │  acct-3  ○ disabled     —      —     —           —       │   │
│  └──────────────────────────────────────────────────────────┘   │
│                                                                 │
│  REQUESTS  (live feed)                                          │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │  11:42:18  POST /v1/messages      acct-1  200   1.4s    │   │
│  │  11:42:03  POST /v1/messages      acct-2  200   2.8s    │   │
│  │  11:41:58  POST /v1/messages      acct-1  429   0.2s    │   │
│  └──────────────────────────────────────────────────────────┘   │
│                                                                 │
│  ⌘K palette  ·  /  search  ·  ?  keys                           │  footer
└─────────────────────────────────────────────────────────────────┘
```

Overlays:
- `<dialog>` for account drill-down + request detail (native, no library)
- Popover `<div popover>` for tooltips (native, anchor-positioned)
- Cmd-K full-screen overlay with fuzzy search

---

## API contract

Consumes endpoints added by Phase H (already mounted):

| Endpoint | Verb | Used for |
|---|---|---|
| `/dashboard/api/state` | GET | Initial snapshot / SSE fallback polling |
| `/dashboard/api/stream` | GET (SSE) | Live snapshot + request events |
| `/dashboard/api/requests` | GET | Back-fill last 50 on cold load |
| `/dashboard/api/import` | POST | Import accounts modal |
| `/dashboard/api/accounts/{provider}/{id}` | DELETE | Row remove action |
| `/dashboard/api/opencode-config` | GET | Config inspector |

**We do not create duplicate endpoints.** If Phase H hadn't shipped yet,
we would have added them under `/dashboard/api/*` (same namespace) because
they're proxy-wide concerns, not dashboard-version concerns. That scenario
didn't materialize.

Added under our namespace:

| Endpoint | Verb | Used for |
|---|---|---|
| `/dashboard-next` | GET | HTML shell |
| `/dashboard-next/assets/{path...}` | GET | Vite-bundled JS/CSS/fonts/sprites |

---

## File structure

```
internal/server/next/
  embed.go                 // go:embed all:../assets/next
  handlers.go              // HTTP handlers: HTML shell + asset serving
  handlers_test.go         // happy + auth denied + bad input + path traversal
  client/
    package.json
    pnpm-lock.yaml (committed)
    vite.config.ts
    tsconfig.json
    svelte.config.js
    index.html
    src/
      main.ts              // bootstrap + mount
      App.svelte           // root
      lib/
        sse.ts             // EventSource manager
        fuzzy.ts           // hand-rolled subsequence scorer
        api.ts             // typed fetch
        theme.ts           // theme persistence + system detection
        types.ts           // Account, RequestRecord, Snapshot, ImportEntry
        stores.svelte.ts   // global reactive state via $state
      routes/
        Pool.svelte
        Requests.svelte
        AccountDrill.svelte
      components/
        AccountTable.svelte
        RequestFeed.svelte
        HealthBar.svelte
        CommandPalette.svelte
        ImportModal.svelte
        ThemeToggle.svelte
        StatusPill.svelte
        KeyHint.svelte
        icons/Icon.svelte  // 12 hand-picked inline SVG glyphs
      styles/
        tokens.css         // OKLCH + light-dark() design tokens
        reset.css
        base.css           // cascade layer setup
        theme.css
        components.css     // imported CSS for .svelte components that use globals

internal/server/assets/next/   // go:embed target, vite build output
```

Server integration:
- `internal/server/next/embed.go` — `//go:embed all:../assets/next`
- `internal/server/next/handlers.go` — `Register(mux *http.ServeMux)` registers
  `GET /dashboard-next` + `GET /dashboard-next/assets/{path...}`.
- `internal/server/server.go` — one line added: `next.Register(mux)` after
  existing dashboard registration. This is the only shared-file touch.

---

## Feature scope (P0 locked; P1 if time)

### P0
1. **AccountTable** — dense, live SSE, sortable, click-to-drill. Row flash
   on update via `@property` + `view-transition-name`.
2. **RequestFeed** — rolling 50, live SSE append, click for detail `<dialog>`.
3. **HealthBar** — version, uptime (ticking), total requests (animated
   counter via `@property`), error-rate (inline sparkline SVG).
4. **CommandPalette** — cmd-k, fuzzy over [accounts, requests, actions].
5. **ImportModal** — drag-drop JSON file OR paste, client-side validation
   with per-entry preview, POST to `/dashboard/api/import`.
6. **ThemeToggle** — three-way: system / dark / light, persisted in
   localStorage, `light-dark()` CSS does the work.

### P1 (if time permits in 4h budget)
7. AccountDrill.svelte route (`/dashboard-next/#/account/:id`).
8. Config inspector modal (GET `/dashboard/api/opencode-config`, copy-to-clipboard).

### P2 (documented, not built)
- Historical metrics (needs TSDB).
- Multi-user auth.
- Onboarder trigger.
- Log streaming.

---

## Milestones (4h budget, 15-min granularity)

- **0:00 – 0:30** Design doc + tooling verification. ← **CURRENT**
- **0:30 – 1:00** Scaffold Vite+Svelte+TS, design tokens, base CSS, bootstrap.
- **1:00 – 1:30** Go handlers, embed, route registration, first commit.
- **1:30 – 2:15** AccountTable + SSE wiring + HealthBar + StatusPill.
- **2:15 – 2:45** RequestFeed + detail dialog.
- **2:45 – 3:15** CommandPalette + hotkeys + ThemeToggle.
- **3:15 – 3:40** ImportModal.
- **3:40 – 3:55** Handler tests, build verification, measurements.
- **3:55 – 4:00** Commit, BUILD_LOG, docs close-out.

---

## Comparison matrix (to fill in at close-out)

| Dimension | Phase H (vanilla JS) | Dashboard Next (Svelte 5) |
|---|---|---|
| LOC: client JS/TS | 778 lines (app.js) | TBD |
| LOC: client CSS | 670 lines (tokens.css + app.css) | TBD |
| LOC: client HTML | 200 lines (index.html) | TBD |
| LOC: Go handlers | 298 lines (dashboard_v2.go) | TBD |
| Bundle JS (min+gzip) | ≈ 8KB (uncompiled source embedded raw) | TBD |
| Bundle CSS (min+gzip) | ≈ 4KB | TBD |
| Build time | 0ms (no build step) | TBD |
| Node dependencies | 0 | 5 (dev only: vite, svelte, svelte-check, tsc, plugin-svelte) |
| Runtime dependencies | 0 | 0 (Svelte compiles away) |
| Cold FCP | TBD | TBD |
| Change amplification (edit one component) | grep+edit vanilla | edit .svelte only |

---

## Subjective wins/losses (filled at close-out)

### Wins of this stack over Phase H
TBD

### Losses vs Phase H
TBD

### Recommendation for operator
TBD

---

## Rough-edge acknowledgment

Going in with eyes open:
- **Build step required.** A fresh clone now needs `pnpm install && pnpm build`
  before `go build` produces a working dashboard-next. Makefile target added
  under a clear "=== Dashboard Next ===" separator at EOF (does not conflict
  with Phase I append point).
- **Committed dist.** We commit `internal/server/assets/next/*` so `go build`
  works without Node installed (matches the brief's constraint). This means
  reviewing a client change requires reviewing both the source AND the
  rebuild diff. Accepted tradeoff.
- **Reactive framework for a single-user tool.** Svelte's value prop is
  hardest to justify at this scale — it's genuinely a test of whether
  compile-time reactivity pays off even here.
