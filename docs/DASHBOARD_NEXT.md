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
| FCP | < 400ms | not measured in-session (no headless chrome) |
| LCP | < 800ms | not measured in-session |
| INP | < 100ms | not measured in-session |
| CLS | 0 | visual audit: 0 (no async layout shift paths) |
| JS gzipped | < 50KB | **20.7 KB** ✓ |
| CSS gzipped | < 15KB | **5.0 KB** ✓ |
| Initial HTML | < 3KB | **0.5 KB** (gzipped) ✓ |
| Build time | — | **555 ms** (vite build, cold) |

All "not measured" items need a headless-chrome invocation not available in
this build environment. The bundle-size budgets are the ones that
genuinely gate shippability, and all pass comfortably.

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

## Comparison matrix (measured at close-out)

Phase H's UI was stashed out of the tree mid-session by a concurrent agent
(see commit b8f9acd HALT report), so exact parity comparison isn't possible
in this snapshot. Numbers below are for Dashboard Next only; Phase H's
measured counterparts live in the operator's A/B review alongside this.

| Dimension | Dashboard Next (Svelte 5) | Phase H baseline (from stash) |
|---|---|---|
| Client LOC (Svelte) | 2,352 | — |
| Client LOC (TypeScript) | 606 | — |
| Client LOC (CSS) | 300 | — |
| Client LOC total | **3,258** | ~1,650 (from stash inspection: 778 JS + 670 CSS + 200 HTML) |
| Go handler LOC | 125 (+ 36 embed.go) | 298 (dashboard_v2.go) + 102 (dashboard.go) |
| Test LOC | 161 | ~300 (dashboard_v2_test.go in stash) |
| Bundle JS (gzip) | **20.7 KB** | ~8 KB (uncompiled source served raw) |
| Bundle CSS (gzip) | **5.0 KB** | ~4 KB |
| Build time | 555 ms | 0 ms (no build step) |
| Dev dependencies | 5 (vite, svelte, svelte-check, tsc, plugin-svelte) | 0 |
| Runtime dependencies | 0 | 0 |
| Change amplification | `.svelte` edit → component-level | hand-sync'd DOM queries in app.js |

### Notes on the numbers

- **Svelte LOC is higher** because each component embeds its own
  component-scoped `<style>`. That CSS compiles down into 5 KB gzipped,
  which is where the actual-shipping-size comparison matters.
- **Phase H's "0 build time"** is a real win if you value zero-setup forks.
  Dashboard Next requires `pnpm install && pnpm build` for a fresh clone
  to produce working assets.
- **Change amplification** favors Svelte: editing a row's behavior is a
  one-file concern (`AccountTable.svelte`) that includes its own styles
  and markup. Phase H's equivalent requires coordinated edits across
  `app.js` (DOM mutation) + `app.css` (styling) + `index.html` (markup
  shape).

## Subjective wins / losses

### Wins over Phase H
1. **Type safety end-to-end**: the `Account`, `Snapshot`, `RequestRecord`
   types are declared once in `types.ts` and enforced at both the fetch
   layer and every template expression. Phase H uses `escapeHtml(s)` on
   every access — correct but easy to forget.
2. **Component encapsulation**: CSS scoping comes free from Svelte; no
   accidental selector collisions. Phase H's BEM-ish naming is disciplined
   but not compiler-enforced.
3. **Reactive primitives feel native**: `$state`, `$derived`, `$effect`
   read like the code they replace. The `HealthBar`'s rAF-driven uptime
   ticker is 12 lines in `$effect`; the Phase H equivalent would require
   manual state reconciliation.
4. **Modern CSS applied rigorously**: `light-dark()` means zero JS
   touches style on theme toggle; `container queries` give
   per-component responsiveness that media queries can't. Phase H uses
   1.5-year-old CSS because that was portable in vanilla JS.
5. **Accessibility muscle memory**: Svelte's compile-time A11y lints
   surfaced three issues we fixed instantly (redundant role, click-only
   handlers). Phase H relies on eyeballing.

### Losses vs Phase H
1. **Onboarding cost**: contributors need Node + pnpm just to make a one-
   character UI change visible. Phase H contributors need nothing beyond
   Go.
2. **Bundle is 2.5× larger gzipped** (20.7 KB vs ~8 KB). Neither matters
   at localhost scale, but at ~$∞/GB egress this would be worth watching.
3. **Committed `dist/`** couples review overhead — every source change
   requires reviewing both input and a rebuild diff. Mitigated by
   deterministic filenames but not eliminated.
4. **Meta-complexity**: Svelte 5 runes, Vite bundling, CSS cascade layers,
   View Transitions — these are all genuinely great but compound the
   mental load for a reviewer new to any one of them.

### Recommendation for operator
For a **personal operator tool shipped as a single Go binary**, Phase H's
vanilla approach is probably the right long-term choice: no build step,
no Node dep, minimum contribution friction.

Dashboard Next earns its place if the project grows into a multi-view app
(P1 drill-down pages, historical metrics, multi-user auth) — at that
scale Svelte's type safety, component encapsulation, and reactivity pay
for themselves several times over.

My blunt take: **ship Phase H now, keep Dashboard Next in the tree as the
proven escape hatch for when the UI surface genuinely grows**. Both
coexist cleanly under `/dashboard` and `/dashboard-next`; no need to
delete either.

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
