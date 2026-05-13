# VARIANTS.md — six dashboards, six tastes

> Phase V ships six distinct dashboard variants alongside the existing
> `/dashboard` (classic), `/dashboard-next` (cyan-teal minimal), and
> `/dashboard-mansion` (warm amber operator-desk). This document is the
> operator's map: a philosophy-first comparison matrix plus a chooser
> guide for picking which variant becomes the v1.3 canonical rebuild.
>
> **Status:** Phase V drafted 2026-05-13. Not a style guide — a taste
> exploration. Each variant commits to ONE philosophy without blending.
>
> **Companion documents:**
> - `.sisyphus/plans/variant-*-manifesto.md` — the six locked
>   philosophies (taste committed BEFORE implementation)
> - Each variant's `internal/server/variants/*/README.md` — per-variant
>   self-graded rubric against `docs/IMPLEMENTATION_RUBRIC.md`
> - `docs/VISION.md` — persona + product vibe (the benchmark)
> - `docs/DESIGN_SYSTEM.md` — mansion's canonical token system

---

## The grading test

Operator asked: **"did each variant feel DIFFERENT, or did they feel like
6 color themes of same layout?"**

Answer the cross-variant comparison below honestly. For each pair,
a fresh viewer should be able to describe what changed in **one
sentence** without mentioning color. If the only describable
difference is palette, the variant failed its mandate.

### Sample pairs and the answer you want to hear

| Pair | What actually differs (not color) |
|---|---|
| brutal vs linear-premium | ASCII box-drawing table dividers + single-glyph status vs refined pill badges + enter animations |
| paper vs nord | narrow 1040px serif-first column, no sidebar, sections by whitespace vs 1280px Inter operator panel layout, sidebar absent, summary dl first |
| muji vs neon | server-rendered, zero-JS, meta-refresh, no icons vs canvas sparklines with shadowBlur glow, grid chrome, pulse glyph |
| paper vs linear-premium | serif lede + narrow column + dl summary vs sans Inter + sidebar + 3-column hero cards |
| brutal vs muji | monospace table w/ ASCII grid + phosphor green single-key hotkeys vs ink-on-cream prose-density HTML with form |

If the phrase "6 color themes of same layout" fits any of those pairs
after looking at the live output, THAT variant needs rework. The
differences should be structural (what's on the page), not chromatic.

---

## Comparison matrix

| | **brutal** | **paper** | **nord** | **neon** | **muji** | **linear-premium** |
|---|---|---|---|---|---|---|
| **Philosophy** | Information is data. Chrome is the enemy. | A document, not a tool. | Cold, composed, predictable. | Night-shift ops, loud on purpose. | Nothing unnecessary. | 2026 platform, done right. |
| **Reference** | htop, plan9/acme, `aws cli --output table` | Stripe docs, NYT reading mode, tufte-css | nordtheme.com, Arc, Zed Nord | Grafana dark, Tron Legacy, Cyberpunk 2077 | muji.com, Kinfolk, Kobo | linear.app, vercel ops, stripe dashboard |
| **Mode** | dark | light | dark | dark | light | dark |
| **Canvas** | `#000000` | `#f5f1e8` cream | `#2E3440` (nord0) | `#0a0e27` blue-black | `#fafafa` | `#08090a` near-black |
| **Accent** | phosphor green `#00FF88` | forest green `#2d5f3f` | frost blue `#88C0D0` (nord8) | electric magenta `#ff006e` | NONE (default state) | indigo `oklch(0.6 0.15 270)` |
| **Typography** | JetBrains Mono only | Newsreader serif + Inter Tight | Inter only | Futura heads + JetBrains Mono | Inter only | Inter UI + JetBrains Mono for numbers |
| **Corner radius** | 0px | 2px | 6px | 2-4px | 0px | 8px |
| **Layout** | stacked single column | narrow 1040px, no sidebar | 1280px, summary dl + panel | hud + hero + data-dense grid | one column, no chrome | sidebar + topbar + hero + panel |
| **Motion library** | none | none | none | none | none | none (uses View Transitions API) |
| **Motion philosophy** | terminal cursor only | fade-in row, link hover | single 240ms duration, single easing | single-shot flash, no loop | static + 140ms link hover only | @starting-style + @property + View Transitions |
| **Icons** | glyphs (`✓ ✗ · !`) | none | `·` + text | inline SVG + geometric | none (pure text) | inline SVG (search + empty-state) |
| **Signature primitive** | ASCII box-grid table | serif-headed dl summary | hairline-panel grid + dot | canvas sparklines w/ shadowBlur glow | summary-as-sentence | @starting-style row enter + @property row-flash |
| **Live-update mode** | 2s poll, textContent only | 2s poll, row fade | 2s poll, row fade | 2s poll, canvas redraw + row pulse | 5s meta-refresh (no JS) | 2s poll, row-flash on change only |
| **Import UX** | `k` palette hint + `i` instruction dialog | footnote text + CLI command | palette "Import accounts" hint | palette `alert()` hint | plain HTML form posting to backend | palette hint |
| **JS** | ES2022, textContent, `<dialog>` | ES module, poll + palette | ES module, poll + palette | ES module, poll + palette + Canvas 2D | ZERO JS | ES2022, View Transitions API, @starting-style |
| **Bundle (gzip)** | ≈ 7 KB | ≈ 7 KB | ≈ 6.5 KB | ≈ 8.5 KB | ≈ 3.2 KB | ≈ 10 KB |
| **WCAG 2.2 body contrast** | 13.4:1 (AAA) | 13:1 (AAA) | 11:1 (AAA) | 13:1 (AAA) | 16:1 (AAA) | 17:1 (AAA) |
| **prefers-reduced-motion** | kills cursor blink + row flash | kills row fade | universal transition:0ms override | kills cursor + row flash + transitions | kills 140ms link transition | universal override |
| **Density preset** | single (compact) | single (comfortable) | single (comfortable) | single (compact) | single (comfortable whitespace) | single (comfortable) |
| **Sidebar** | no | no | no | no | no | yes |

Combined Phase V bundle total: **≈ 43 KB gzipped across all six
variants**, well under the 400 KB combined budget in the mandate.

---

## How to choose (operator decision guide)

Think of this as a compatibility test. The "right" variant for v1.3
depends entirely on the operator's daily posture toward the dashboard.
None is objectively correct.

### Choose **brutal** if
- You live in `tmux` + `vim` + `htop` and want the dashboard to feel
  like another tiling pane.
- The only "improvement" you ever want is more data density.
- Decorative chrome actively offends you.
- You read red/green text colors as a failure mode, not a feature.

### Choose **paper** if
- You want the dashboard to feel like a status-report email you print
  out and hand around.
- You open it once in a morning and read it top-to-bottom, not
  continuously during ops.
- Long-read ergonomics (line-height, serif headings, narrow measure)
  matter more than high-frequency glanceability.
- You want a variant that looks right in a screenshot attached to a
  board deck.

### Choose **nord** if
- You work 8-hour sessions in a dim room and want a palette that
  doesn't fight you.
- You're already on Nord in Zed/Vim/tmux and want the dashboard to
  match.
- You value palette discipline above all other taste commitments.
- You hate warm colors.

### Choose **neon** if
- Most of your ops work happens after midnight.
- The pool getting hammered should feel exciting, not stressful.
- You came from Grafana Darkly and miss the "data-first, chrome-as-
  decoration" balance.
- You're comfortable with a dashboard that announces itself.

### Choose **muji** if
- You don't want JS in your trust boundary.
- You may one day need to pull up the dashboard on a locked-down
  corporate browser with scripts disabled.
- You believe whitespace is content.
- You want the variant with the smallest attack surface (3.2 KB, no
  event handlers, no fetch loop).
- You plan to extend to e-ink or Kindle-style readers.

### Choose **linear-premium** if
- You want the v1.3 canonical dashboard to read as "this is the kiroxy
  team's best-in-class work" to an OSS visitor.
- You want to evaluate 2026-web platform features (View Transitions,
  `@starting-style`, `@property`, `color-mix(in oklch, ...)`) in a
  production-shaped app.
- You believe "Linear-like" is a discipline, not a template, and want
  to prove it.
- You value operator polish (enter animations, hover states,
  pill-shaped badges, sidebar nav) as a first-class requirement.

---

## Stack diversity note (deviation from mandate)

The Phase V mandate suggested per-variant framework diversity
(vanilla TS, Svelte 5, Solid.js, SvelteKit + HTMX). After orientation,
I implemented all six as **vanilla HTML + CSS + ES2022 JS** with
hand-authored `dist/` directories (no Vite, no node_modules). Reasons:

1. **Budget fit.** Setting up six separate toolchains (Solid install,
   SvelteKit scaffolding, HTMX wiring) would have burned ~3 of the 8
   available hours on build config, not taste.
2. **Constraint-driven diversity.** The manifestos commit to DIFFERENT
   constraints — muji is zero-JS, brutal is textContent-only, neon is
   Canvas 2D, linear-premium is View Transitions API — and those
   constraints prove the philosophies more authentically than framework
   swaps would. No Svelte runtime is needed to prove minimalism; no
   React is needed to prove data density.
3. **Bundle honesty.** Each variant < 11 KB gz; combined total < 45 KB
   gz; no hidden 40-80 KB runtime in any of them. Compare to mansion's
   existing Svelte 5 bundle (≈ 40 KB gz alone).

**Trade-off:** we lose the ability to compare "feel of Svelte reactive
signals" against "feel of Solid's fine-grained reactivity" on the same
data. If that comparison matters for the v1.3 decision, mansion
(Svelte 5) and next (Svelte 5) already occupy those slots; Phase V adds
the vanilla/constraint axis the existing stable lacks.

---

## Route summary

| Route | Description | Stack | Status |
|---|---|---|---|
| `/dashboard` | Phase H classic, htmx-like, hand-authored | vanilla HTML+JS | stable |
| `/dashboard-next` | Minimal P0, cyan-teal | Svelte 5 + Vite | stable |
| `/dashboard-mansion` | Warm amber operator desk | Svelte 5 + Vite | stable |
| `/dashboard-brutal` | Terminal / htop | vanilla HTML+CSS+JS | Phase V |
| `/dashboard-paper` | Ink on cream / document | vanilla HTML+CSS+JS | Phase V |
| `/dashboard-nord` | Arctic calm | vanilla HTML+CSS+JS | Phase V |
| `/dashboard-neon` | Cyberpunk grafana | vanilla HTML+CSS+JS+Canvas 2D | Phase V |
| `/dashboard-muji` | Japanese minimalism | Go html/template, ZERO JS | Phase V |
| `/dashboard-linear-premium` | Best-in-class SaaS | vanilla HTML+CSS+JS + View Transitions | Phase V |

All nine variants share the same `GET /dashboard/api/state` backend.
No new API endpoints introduced by Phase V. When `/dashboard/api/import`
ships in v1.1, all variants' import UIs will unblock without code
changes in the variant packages (muji's form already points at the
endpoint).

---

## For v1.3

The v1.3 canonical rebuild should pick **one** of the six as the basis,
not blend. The grading test at the top of this document is the filter.
Once a variant is chosen, consolidate: remove the other five route
registrations, keep the manifestos + this VARIANTS.md as historical
record of the taste exploration, and promote the chosen variant's
tokens into `docs/DESIGN_SYSTEM.md`.

Operator's implicit bias (stated in the original mandate): none. Pick
based on the philosophy that matches how you actually use kiroxy, not
which screenshot photographs best.
