# Dashboard — Mansion

> Third operator-facing dashboard for kiroxy, mounted at `/dashboard-mansion`.
> Sibling of `/dashboard` (Phase H vanilla HTML) and `/dashboard-next`
> (experimental Svelte 5 P0 reference stack).

This document is the self-defense for Mansion's visual direction,
technology choices, feature set, and known rough edges. It was written
together with the initial implementation, not after, so it reflects the
decisions as they were made rather than as they might later be
rationalized.

---

## Why a third dashboard

Kiroxy already ships two dashboards:

- `/dashboard` (Phase H) — 133 lines of hand-authored HTML + vanilla JS.
  Zero build step, minimal polling, fits in one `<style>` tag. Exactly
  right as a no-dependency "can I see the pool?" UI.
- `/dashboard-next` — Svelte 5 + Vite, 84 KB bundle. Proves the
  2026-stack alternative works end to end, with components, SSE
  wiring, import, palette, and theme toggle. Operator feedback was
  *"kurang puas untuk front end"* — it felt like minimum-viable
  scaffolding rather than a signature product.

Mansion is what happens if we commit fully to a visual direction and a
density aesthetic, instead of aiming for "parity with dashboard-next in a
different framework." All three dashboards coexist on the same binary
so operators can pick; Mansion is the ambitious one.

## Visual direction — "Operator Desk — Warm Dense"

The short version: **reading pool telemetry at a desk lit by brass lamps.**

- **Warm dark-first.** Deep warm charcoal (oklch(14.5% 0.012 65))
  backgrounds, not true black. Light mode is warm ivory, not `#fff`. The
  whole palette has a low but non-zero chroma at hue 65–75 (warm neutral)
  so nothing ever looks sterile next to the amber accent.
- **Aged-brass amber accent** (oklch(78% 0.14 72) dark / oklch(58% 0.14 72) light).
  Chosen over the usual SaaS indigo because brass is the historical
  operator-tool hue — vintage terminals, meter faces, Zed's warm theme,
  Warp. Chroma held low (0.14) so amber can coexist with dense data
  without becoming shouty.
- **JetBrains Mono as the display face.** Not reserved for code — it
  runs the wordmark, section headers, counters, and timestamps.
  Inter is used only for body prose. This is deliberately the opposite
  of consumer SaaS practice, and it sells the operator framing more
  than any single visual choice.
- **Hairline ledger dividers.** Every rule between rows, between
  sections, under the topbar plaque is `1px` at a subtle warm-gray
  tone. No boxy cards-with-drop-shadow decoration — instead dense
  tables with thin rules, like a printed ledger.
- **No gradients, no glass.** Except two: a subtle radial warm wash on
  the body (barely perceptible) for paper grain, and a brass-gradient
  hairline under the topbar. That's it. No "glassmorphism."

**Rejected alternatives:**

- *Neo-brutalist terminal* — considered; too costume-y, doesn't scale to
  production.
- *Linear-style cool-mono minimal* — overdone and would be
  indistinguishable from dashboard-next.
- *Vercel/Supabase dark* — great references but monochrome-teal is
  exhausted.
- *Grafana density-first* — the inspiration for density but too utilitarian
  visually; Mansion borrows rhythm from Grafana without its chrome.

**Reference gallery (mental, not literal copies):**
Zed warm theme · Warp terminal statusbar · Linear density · Raycast
palette behavior · Superhuman keyboard discipline · Arc command bar ·
Fly.io density · Grafana panels.

## Tech stack + defense

| Choice | Why |
|---|---|
| **Svelte 5 (runes)** | Already in the repo (`dashboard-next`), so no new runtime. Runes let the single `store.svelte.ts` be the one source of truth for reactive state. Compiler output is small enough to stay under budget. |
| **Vite 6 + esbuild minify** | Stays lockstep with `dashboard-next`'s build graph. Deterministic filenames + no content hashing because the Go server serves from memory. |
| **No chart libraries** | Sparkline (catmull-rom SVG) and CountdownRing (animated `stroke-dashoffset`) are hand-rolled. Chart libraries would easily add 30–60 KB; we didn't want to spend that budget for two components. |
| **No icon libraries** | All icons are one `<Icon>` component with a switch over small hand-drawn SVG paths. 100 bytes per icon vs. ~40 KB for Lucide. |
| **No animation libraries** | Motion primitives are CSS-only (`@keyframes pulse-ring`, `@starting-style`, `cubic-bezier` ease tokens). Keeps the bundle tight and the motion feel consistent. |
| **@types/node only** | One devDep on top of dashboard-next's set, purely for `node:url` in `vite.config.ts`. No runtime polyfills. |
| **polling + opportunistic SSE** | The current Phase H backend exposes `/dashboard/api/state` but not `/dashboard/api/stream` or `/dashboard/api/requests`. LiveSource tries SSE, falls back to polling every 2s, and synthesizes RequestRecord entries from per-account counter deltas so the feed stays populated without backend changes. |

## Feature set

### Parity with `/dashboard-next`

- Account pool view with live updates
- Recent requests feed (synthesized from snapshot deltas until the
  `/dashboard/api/requests` endpoint lands backend-side)
- System health ribbon (live status, vault path, build, uptime)
- Command palette (⌘K) with fuzzy search across actions, accounts, and
  recent requests
- Import accounts UI (client-side JSON validation, honest error
  surfacing when the backend returns 404)
- Theme toggle (system / dark / light)
- Keyboard shortcut cheat sheet (?)

### Signature additions

1. **Refresh countdown rings.** Each account row shows a ring that
   animates its `stroke-dashoffset` at requestAnimationFrame rate; color
   interpolates accent → warn → danger as the fraction of TTL drops. The
   label inside the ring updates in realtime.
2. **Request lifecycle timeline.** Clicking a request opens
   `DetailDrawer` with proportional bars for each phase (inbound, pool
   pick, refresh, upstream, convert, flush). Timings are synthesized
   from total latency — the current backend doesn't emit per-phase
   telemetry, and the drawer labels that assumption honestly as a v1.3
   swap target.
3. **Per-account sparklines (rolling 5-min window).** Store maintains
   30 buckets of 10s each; `perAccountSpark[id]` updates on each
   snapshot and rolls over on bucket boundaries. Hand-rolled
   catmull-rom SVG with an auto-ranged y-axis and a latest-value dot.
4. **Pool Pulse hero strip.** Per-account compact card grid above the
   table so pool health reads at a glance. With fewer than 3 identities
   it fills with dashed "capacity slot" ghost cards for visual balance
   and a quiet onboarding CTA.
5. **Activity Ledger.** Synthesizes events from snapshot + request
   stream (account add/remove, cooldown arm, error bumps, 4xx/5xx
   responses). At rest it shows eight `--:-- waiting for event · ...`
   ghost rows that teach the event categories rather than leaving the
   panel empty.
6. **URL-persisted filter state.** The `Filters` object serializes to
   `#q=...&err=1&cool=1&status=5xx` so a given view is shareable by
   URL. The palette's `copy shareable view link` action copies the
   current URL.
7. **Command palette with live preview pane.** Selecting an action
   shows a short plain-text explanation; selecting an account or
   request shows a mini status card — no blind command runs.

### P2 features the timeline didn't fit

- Log stream with ANSI color (`--:-- waiting for event · vault rotation`
  in the ledger is a stub for this)
- Split-pane resizer (DetailDrawer is a fixed-width drawer for now)
- "What-if" config preview
- Inline docs on metric hover

## Comparison matrix

| | /dashboard (H) | /dashboard-next | /dashboard-mansion |
|---|---|---|---|
| LoC (JS + Svelte) | 0 + 0 (inline JS) | ~1 400 | ~2 600 |
| LoC (CSS) | ~70 inline | ~900 | ~1 800 |
| LoC (Go) | 133 | ~130 | ~126 + tests |
| Bundle (raw) | 5 KB html | 84 KB | 134 KB |
| Bundle (gzipped) | — | ~21 KB | **37.7 KB** |
| Dependencies (runtime) | 0 | svelte 5 | svelte 5 |
| Dev deps | 0 | 6 | 7 (adds @types/node) |
| Components | 0 | 8 | **14** |
| SSE support | no | yes | yes + polling fallback |
| Sparklines | no | no | **yes (hand-rolled SVG)** |
| Countdown rings | no | no | **yes (rAF animated)** |
| Lifecycle timeline | no | no | **yes (synthesized phases)** |
| URL-shareable views | no | no | **yes** |
| Themeable | no | yes (3 modes) | yes (3 modes, dark default) |
| a11y (WCAG 2.2) | AA-ish | AA | AA + focus rings, skip target, reduced motion, high-contrast media query |
| Empty-state polish | minimal | minimal | **curated (curl recipe, capacity slots, ghost ledger rows)** |

## Known rough edges

These are things Mansion is honest about rather than hiding:

1. **Backend endpoint gaps.** `/dashboard/api/requests`,
   `/dashboard/api/stream`, `/dashboard/api/import`,
   `/dashboard/api/accounts/{p}/{id}`, and
   `/dashboard/api/opencode-config` are all consumed by the frontend
   but return 404 on the current Phase H backend. Mansion degrades
   gracefully: polls `/state`, synthesizes request deltas, surfaces
   import errors honestly. Tracked as v1.3 "backend parity" work.
2. **Request-phase timings are synthesized.** DetailDrawer clearly
   labels the assumption. Real per-phase telemetry is a v1.3 target
   for `internal/messages` + `internal/kiroclient`.
3. **Account `expires_at` field is optional.** The current
   DashboardAccount Go shape doesn't expose it. CountdownRing shows
   `--:--` until the backend adds it.
4. **Synthetic request records.** When `/dashboard/api/requests` is
   404, the client generates up to 3 RequestRecord entries per
   snapshot from per-account counter deltas. They're clearly marked
   with `latency_ms=0` so the UI renders `—` for latency, and their
   paths default to `/v1/messages`.
5. **Single-identity zero-state.** The UI gets denser when
   `accounts.length >= 3` because the ghost capacity slots drop. At
   one identity the pool-pulse row is deliberately padded with ghost
   cards — the alternative (shrinking the card) felt less balanced in
   testing.
6. **Layout assumes viewport width ≥ 1120px** for the two-column
   split. Below that the stream stacks under the board. Tested
   visually down to 720px; below 640px the pool-pulse grid collapses
   to a single column.
7. **ANSI color log stream is stubbed.** Shown as a ghost row in the
   ledger (`waiting for event · vault rotation`) but the rendering
   path doesn't exist yet.

## Build

```bash
# one-time
cd internal/server/mansion/client && pnpm install

# rebuild assets (committed dist tree keeps `go build` green on fresh clones)
cd internal/server/mansion/client && pnpm build:fast

# full typecheck + build
cd internal/server/mansion/client && pnpm build
```

The dist tree is committed for the same reason dashboard-next's is:
fresh-clone `go build` must not require Node. The Vite config emits
deterministic filenames (`app.js`, `app.css`, `index.html`) so diffs
stay reviewable.

## Routes

| Path | Handler | Notes |
|---|---|---|
| `GET /dashboard-mansion` | `mansion.handleIndex` | serves `index.html` with `Cache-Control: no-cache` |
| `GET /dashboard-mansion/assets/{path...}` | `mansion.handleAsset` | serves embedded `dist/` with explicit content-type whitelist + traversal guard |

Both are registered via `mansion.Register(mux)` in `server.go` next to
`next.Register(mux)` and `s.registerDashboard(mux)`. The existing
logging + auth middleware wraps them automatically.

## Where to look first

- **Visual direction:** `client/src/styles/tokens.css` — the palette
  and type scale with OKLCH + light-dark() machinery.
- **Data layer:** `client/src/lib/live.ts` — polling + SSE upgrade +
  delta synthesis.
- **Store:** `client/src/lib/store.svelte.ts` — single reactive source
  of truth with rolling sparkline buckets.
- **Pool Pulse:** `client/src/components/PoolPulse.svelte`.
- **Lifecycle timeline:** `client/src/components/DetailDrawer.svelte`
  with the phase synthesizer in `lib/phases.ts`.
- **Server mount:** `internal/server/mansion/handlers.go`.
