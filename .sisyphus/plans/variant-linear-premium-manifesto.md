# Variant 6 — `dashboard-linear-premium` — "SIGNATURE SAAS DONE RIGHT"

## One-sentence philosophy
The best-in-class admin genre, executed with 2026-web platform primitives
— proving that "Linear-like" can be a discipline rather than a template.

## 3 visual signatures (5-second description)
1. **Near-black canvas `#08090A`** with a purple-indigo accent
   (`oklch(0.60 0.15 270)` ≈ `#5B5BD6`). Cards have a subtle top-to-bottom
   2% brightness gradient (ONE gradient, not multi-stop) — the only
   ornament, and it's almost invisible until you look for it.
2. **Inter Display for headers (18-24px), Inter Variable for UI (13-14px),
   JetBrains Mono for numerical cells only.** Weight discipline: 400 body,
   500 emphasis, 600 headers, never 700. 8px corner radius on everything
   consistently. Pills are 11px text in a 22px-tall pill shape.
3. **View Transitions API for cross-nav, @starting-style for enter, all
   240ms cubic-bezier(0.22, 1, 0.36, 1).** Command palette opens with
   a blur backdrop — real `backdrop-filter: blur(12px)`, used once, not
   everywhere. Focus rings use `:focus-visible` with 2px accent.

## 3 things EXPLICITLY NOT DOING
- **No motion libraries.** No framer-motion, no GSAP, no anime.js. Pure
  CSS transitions + View Transitions API + @starting-style. This is the
  2026 platform baseline, and we commit to it exclusively.
- **No decorative gradients.** One gradient, on cards, 2% shift, and
  that's it. No hero glow, no button gradient, no chart gradient fills
  beyond a single neutral 10%→0% opacity fade under the sparkline.
- **No glow effects or neon.** This is premium dark, not cyberpunk. The
  accent appears only on interactive states and the single logo letter.

## Reference citations
- **linear.app** — the genre leader; informs every structural decision
  (sidebar, palette, cards, typography hierarchy)
- **vercel.com operator console** — proving monochrome + 1 accent scales
  to 50+ screens
- **stripe.com/dashboard** — the refined version of "SaaS admin"
- **Raycast preferences window** — palette-first interaction language
- `docs/REFERENCE_GALLERY.md` entries `ref-linear-command-palette`,
  `ref-vercel-observability`, `ref-supabase-table-editor` already cited

## Tech stack defense
**Vanilla HTML + CSS + ES2022 JS + modern web APIs (View Transitions,
@starting-style, @property, :has()).** Deliberate choice: Linear-quality
UI does not require a framework — it requires discipline and modern
platform features. Svelte/React would add 20-40KB of runtime overhead that
doesn't earn its weight here. Bundle target: < 18KB gz (highest of the 6,
because this variant ships the richest interactions). Ships a self-hosted
Inter Variable subset + JetBrains Mono subset.
