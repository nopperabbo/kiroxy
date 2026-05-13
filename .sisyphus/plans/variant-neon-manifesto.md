# Variant 4 — `dashboard-neon` — "CYBERPUNK GRAFANA"

## One-sentence philosophy
Night-shift ops at 3am, data-dense and intentionally loud — the dashboard
for when you're the only engineer awake and the pool is getting hammered.

## 3 visual signatures (5-second description)
1. **Very dark blue-black canvas `#0A0E27`** with ONE committed neon
   accent: electric magenta `#FF006E`. Subtle 8% magenta glow
   (`box-shadow: 0 0 12px rgba(255,0,110,0.18)`) on interactive
   elements only — not decoration, a signal that the row is hot.
2. **JetBrains Mono for data + Orbitron for headers** (self-hosted, not
   Google CDN). Headers are ALL CAPS + 0.12em tracking, 13px, feel
   slightly geometric — Grafana dashboard header DNA, turned up 1 notch.
3. **Sparklines glow magenta with 1px drop-shadow, grids have faint
   `#1A1F3A` 1px lines every 24px.** Live-request rows have a 2-second
   `--row-flash` that pulses magenta once, never loops. The grid itself
   is the decoration.

## 3 things EXPLICITLY NOT DOING
- **No continuous animation loops.** No "pulse forever", no shimmer, no
  looping gradient. Every motion is single-shot, under 2 seconds. The
  "cyberpunk" is visual, not kinetic.
- **No second accent color.** Only magenta. Success states use magenta
  inverted to lime `#39FF14` only when status demands it; all neutral
  chrome is grayscale + magenta.
- **No rounded corners > 4px.** Cyberpunk doesn't do friendly pills. 2px
  for pills, 4px for cards, 0px for tables.

## Reference citations
- **Grafana dark theme** — the gold standard for data-dense ops dashboards
- **Tron Legacy UI (Christopher Balaskas)** — neon accent + geometric
  headers, restrained
- **Cyberpunk 2077's CRT overlay menus** — grid-as-chrome, glow as signal
- `docs/REFERENCE_GALLERY.md` future entry: "synthwave admin panels" —
  cited to Behance collection TBD

## Tech stack defense
**Vanilla HTML + CSS + ES2022 JS + Canvas 2D for sparklines.** Sparklines
hand-drawn on canvas rather than SVG because the glow (shadowBlur) is
native to canvas and cheaper than an SVG filter. No framework — Grafana
itself proves dashboards don't need React. Bundle target: < 14KB gz (most
of the budget is Orbitron subset).
