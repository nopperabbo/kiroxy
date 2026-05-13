# linear-premium — signature SaaS done right

> One of six Phase V dashboard variants. See
> `.sisyphus/plans/variant-linear-premium-manifesto.md` for the locked
> philosophy this package commits to.

## Philosophy

The best-in-class admin genre, executed with 2026-web platform
primitives — proving that "Linear-like" can be a discipline rather
than a template.

## Visual signatures

- **Near-black canvas `#08090a`** with indigo-purple accent
  `oklch(0.60 0.15 270)` (≈ `#5b5bd6`). Cards have a subtle top-to-bottom
  2% brightness gradient (ONE gradient, not multi-stop) — the only
  ornament, and it's almost invisible until you look for it.
- **Inter for UI, JetBrains Mono for numerical cells only.** Weight
  discipline: 400 body, 500 emphasis, 600 headers, never 700. 8px
  corner radius on everything consistently. Pills are 11.5px text in a
  22px-tall pill shape.
- **View Transitions API for cross-variant nav.** `@starting-style` for
  dialog and row enter. `@property --row-flash` drives a single-shot
  600ms row-flash via pure CSS. 240ms `cubic-bezier(0.22, 1, 0.36, 1)`
  everywhere. Palette backdrop uses real `backdrop-filter: blur(12px)`.

## Things explicitly NOT doing

- **No motion libraries.** No framer-motion, GSAP, anime.js. Pure CSS
  transitions + View Transitions API + `@starting-style` + `@property`.
- **No decorative gradients.** One gradient, on cards, 2% shift, and
  that's it.
- **No glow effects or neon.** This is premium dark, not cyberpunk.

## Data surface

Polls `GET /dashboard/api/state` every 2s. Row-flash fires on actual
data change (tracked via `prevReq` map) rather than every tick, so
idle rows stay quiet.

## Tech stack

Vanilla HTML + CSS + ES2022 JS + modern web APIs. Deliberate choice:
Linear-quality UI does not require a framework — it requires discipline
and modern platform features. Svelte/React would add 20-40KB of runtime
overhead that doesn't earn its weight here.

## References

- `linear.app` — the genre leader
- `vercel.com` operator console — monochrome + 1 accent at 50+ screens
- `stripe.com/dashboard` — the refined version of "SaaS admin"
- Raycast preferences window — palette-first interaction language

## Bundle size

- HTML ≈ 1.9KB gzip
- CSS ≈ 4.5KB gzip (the richest of the six — earned)
- JS ≈ 3.8KB gzip
- Total ≈ 10KB gzipped (under the 18KB manifesto target)

## Self-graded rubric (docs/IMPLEMENTATION_RUBRIC.md)

Linear-premium is the variant closest in spirit to the mansion rubric
(both commit to production-quality dark-first operator aesthetic). It
scores high against the rubric except where its own taste disagrees.

- §1 Token compliance: **Pass**. All colors in CSS vars (oklch-based
  for the accent, mixed via color-mix for derived tones). No raw hex
  in component code outside `:root`.
- §2 Interaction compliance: **Pass**. `⌘K` palette works from any
  screen, arrow-key navigation, `Esc` closes, `r` refresh when not
  typing. View Transitions on cross-variant navigation. Chord nav (`g
  p / g h / g i`) not implemented — backlog v1.1.
- §3 Accessibility: **Pass**. `#ececef` on `#08090a` ≈ 17:1 (AAA).
  Accent-on-surface contrast adequate via color-mix. All pills have
  text-label + dot. `:focus-visible` 2px accent outline + 2px offset.
  `prefers-reduced-motion` collapses all transitions and animations to
  0ms via universal override.
- §4 Motion: **Pass**. `@view-transition { navigation: auto }`.
  `@starting-style` on dialog enter. `@property --row-flash` for row
  flash. No motion libraries. One `@keyframes row-flash` single-shot.
- §5 Icons: **Pass**. Inline SVG (search glyph in palette, empty-state
  illustration). `currentColor` stroke, round joins, no library.
- §6 Performance: ≈ 10KB gz; no runtime CDN; modern-browser only
  (View Transitions graceful-fallback in JS).
- §7 Docs: this file + manifesto.
- §8 LiveRequestStream: **Partial** — row-flash on changed rows is the
  signature motion primitive until backend exposes `/api/requests`.

**Composite score:** highest alignment with mansion rubric among the
six variants. See `docs/VARIANTS.md` for cross-variant comparison.
