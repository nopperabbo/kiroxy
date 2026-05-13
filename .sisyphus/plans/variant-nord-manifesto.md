# Variant 3 — `dashboard-nord` — "ARCTIC CALM"

## One-sentence philosophy
Cold, composed, predictable — a palette calibrated for eight-hour sessions
in a dim room, every color contributing to calm rather than urgency.

## 3 visual signatures (5-second description)
1. **Literal Nord palette** (from nordtheme.com): canvas `#2E3440`,
   panel `#3B4252`, foreground `#ECEFF4`, frost accent `#88C0D0`, aurora
   status colors (`#A3BE8C` success, `#BF616A` error — *muted*, never
   alarm-red, `#EBCB8B` warn). All tokens named after Nord's originals
   so the provenance is visible in the CSS.
2. **6px radii everywhere** — noticeable but not playful, softer than
   mansion's 8px. Inter everywhere, single family (no mono — data cells
   use Inter with `font-variant-numeric: tabular-nums`). Single
   hairline border `#434C5E` separates panels; no shadows.
3. **Motion is slow and uniform: 240ms ease-in-out, period.** No bounce,
   no spring, no stagger. Status dots fade in, rows cross-fade on update,
   palette uses @starting-style. The motion feels like breathing.

## 3 things EXPLICITLY NOT DOING
- **No accent color beyond the Nord palette.** No brand purple, no neon.
  The palette's discipline is the product.
- **No shadows.** Anywhere. Panels layer by using `nord1 → nord2 → nord3`
  progressively lighter charcoal.
- **No density toggle.** One density: comfortable (40px rows, 24px gutters).
  Nord is for long sessions — we don't pretend compact is useful here.

## Reference citations
- **nordtheme.com** — the palette itself, MIT-licensed, cited directly
- **Arc Browser** — calm predictable motion, muted everything
- **Zed editor's Nord theme** — proving the palette on a dev tool
- `docs/REFERENCE_GALLERY.md` future entry: "Obsidian's default dark" —
  similar restraint

## Tech stack defense
**Vanilla HTML + CSS + ES2022 JS.** The palette is the entire point —
any framework overhead (Svelte's runtime, Solid's signals) would be
wasted on a UI this constrained. CSS custom properties for every Nord
color so the palette is auditable in one file. Bundle target: < 10KB gz.
Polls `/dashboard/api/state` at 2s cadence.
