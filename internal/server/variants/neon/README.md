# neon — cyberpunk grafana

> One of six Phase V dashboard variants. See
> `.sisyphus/plans/variant-neon-manifesto.md` for the locked philosophy
> this package commits to.

## Philosophy

Night-shift ops at 3am, data-dense and intentionally loud. The
dashboard for when you're the only engineer awake and the pool is
getting hammered.

## Visual signatures

- **Very dark blue-black canvas `#0a0e27`** with ONE committed neon
  accent: electric magenta `#ff006e`. Lime `#39ff14` used only for
  "success" state contrast. Subtle magenta glow
  (`box-shadow: 0 0 8px rgba(255,0,110,0.35)`) on interactive elements.
- **JetBrains Mono for body + geometric sans (Futura/Trebuchet MS
  fallback) for ALL CAPS headers** with 0.22em tracking. Numbers use
  tabular-nums + slashed-zero.
- **Grid-as-chrome: `bg-grid` fixed layer with 24px cross-hatch at 10%
  alpha**, faded bottom-mask. Sparklines drawn on `<canvas>` with
  native `shadowBlur` glow (not SVG filter — canvas is cheaper).

## Things explicitly NOT doing

- **No continuous animation loops.** Cursor blink is the only recurring
  anim (1.2s step). Row-flash is single-shot 1.2s, no looping. Chrome
  is static.
- **No second accent color.** Magenta only; lime appears only for
  successful state and error sparkline when errors are zero.
- **No rounded corners > 4px.** Cyberpunk doesn't do friendly pills.

## Data surface

Polls `GET /dashboard/api/state` every 2s. Sparklines use a rolling
30-sample history kept client-side (nothing persisted). No new backend
endpoints — all 6 variants share `/dashboard/api/state`.

## Tech stack

Vanilla HTML + CSS + ES2022 JS + Canvas 2D. Sparklines hand-drawn on
canvas rather than SVG because `shadowBlur` is native to canvas and
cheaper than an equivalent SVG filter.

## References

- Grafana dark theme — the gold standard for data-dense ops dashboards
- Tron Legacy (Christopher Balaskas) — restrained neon accent, geometric
  headers
- Cyberpunk 2077 CRT overlay menus — grid-as-chrome, glow as signal

## Bundle size

- HTML ≈ 1.4KB gzip
- CSS ≈ 3.4KB gzip
- JS ≈ 3.6KB gzip
- Total ≈ 8.5KB gzipped (under 14KB manifesto target)

## Self-graded rubric (docs/IMPLEMENTATION_RUBRIC.md)

- §1 Token compliance: **N/A** — neon rejects mansion's restrained
  palette. Uses its own `--magenta/--lime/--amber` namespace.
- §2 Interaction compliance: **Partial**. `⌘K` palette with arrow-key
  navigation. `r` refresh. `Esc` close. No chord navigation.
- §3 Accessibility: **Pass with caveats.** `#d7dcf5` on `#0a0e27` ≈
  13:1 (AAA). Magenta on canvas ≈ 5.2:1 (AA). Status dots have text
  labels; `aria-live="polite"` on rows. `prefers-reduced-motion`
  disables the cursor blink, row-flash, and all transitions.
  **Caveat:** the bg-grid pattern and glow effects may be disorienting
  for some users; `prefers-reduced-motion` keeps glow but kills motion.
- §4 Motion: **Pass**. No motion libraries. One `@keyframes
  neon-cursor` (cursor blink, single-stop), one `@keyframes
  neon-rowflash` (single-shot). Both silenced under
  `prefers-reduced-motion`.
- §5 Icons: **N/A** — uses glyphs (`⌘`, `→`, `_`).
- §6 Performance: inline CSS + JS; canvas 2D sparkline is O(n) per
  draw with n=30 samples.
- §7 Docs: this file + manifesto committed.
- §8 LiveRequestStream: **Partial proxy** — the sparklines under the
  TOTAL.REQUESTS / TOTAL.ERRORS hero stats act as the live-motion
  signature primitive until backend exposes `/api/requests` (v1.1).

**Composite score:** graded against own cyberpunk ethos — see
`docs/VARIANTS.md`.
