# ICONOGRAPHY.md — kiroxy

> kiroxy's icon system. Inline SVG only. No library imports. Hand-picked set
> of ~20 glyphs matching the dashboard's information needs exactly.
>
> **Status:** v1.0 drafted 2026-05-13.
>
> **Companion documents:**
> - `docs/DESIGN_SYSTEM.md` §6 (iconography rules)
> - `docs/components/*.md` — specs that reference icons
> - `tools/icons/` — source SVG files + typed export

---

## Principles

1. **Inline SVG, every time.** No icon fonts. No sprite sheets. No `<img src=".svg">`. Inline so `currentColor` works and the icon inherits text color.
2. **One set. No exceptions.** Mixing Lucide with Heroicons or Tabler with Phosphor is detectable within seconds and reads as template-core.
3. **Hand-picked.** kiroxy needs ~20 icons total across the whole dashboard. Bulk library imports are forbidden; trees-shaken imports are allowed when the tree is shaken to just the list below.
4. **Source of inspiration: Tabler Icons (MIT).** The design language (24×24 viewBox, 1.5px stroke default, round linecap/linejoin, pure outline) matches kiroxy's visual vocabulary. We adapt and hand-author to match kiroxy's exact stroke width preference.
5. **Currency: `currentColor`.** No fixed hues. Every icon inherits the text color of its containing element.

---

## Design constraints

| Constraint | Value |
|---|---|
| viewBox | `0 0 24 24` |
| Stroke width | `1.5` (default); adjustable via CSS `--icon-stroke-width` |
| Stroke linecap | `round` |
| Stroke linejoin | `round` |
| Fill | `none` (outlines only) |
| Pixel-grid alignment | All anchor points on integer coordinates where possible |
| Accessibility | `aria-hidden="true"` when decorative; `<title>` + `role="img"` when meaningful |

Rendered sizes:

| Size | Use |
|---|---|
| 14px | Inline in dense text (14/16px body) |
| 16px | Default in most components (button leading, status dot pair, table cell) |
| 20px | Sidebar nav, buttons at `md` size |
| 24px | Empty-state hero icon, full-nav icons at `lg` size |
| 32px | Empty-state hero only at section scale |

CSS helper:

```css
.kx-icon {
  width: 1em;
  height: 1em;
  stroke: currentColor;
  stroke-width: var(--icon-stroke-width, 1.5);
  stroke-linecap: round;
  stroke-linejoin: round;
  fill: none;
  flex-shrink: 0;
}
```

---

## Inventory

kiroxy's canonical icon list. Every icon here has a real home in the dashboard;
no speculation. Icons marked ✅ ship in v1.0 of Part 2 (source SVG committed in
`tools/icons/`). Icons marked ⏳ are deferred to implementation time — their
spec is locked, only the SVG path is pending.

### Status (5)

| Name | Purpose | Shipping v1.0 |
|---|---|---|
| `status-healthy` | Account / request OK | ✅ |
| `status-cooldown` | Account in backoff | ⏳ |
| `status-failed` | Auth error, upstream 5xx | ⏳ |
| `status-refreshing` | Token refresh in-flight | ⏳ |
| `status-unknown` | State indeterminate | ⏳ |

### Actions (8)

| Name | Purpose | Shipping v1.0 |
|---|---|---|
| `refresh` | Refresh pool / token | ✅ |
| `remove` | Delete / drop account | ⏳ |
| `disable` | Pause account (reversible) | ⏳ |
| `enable` | Resume account | ⏳ |
| `import` | Import accounts from JSON | ⏳ |
| `export` | Export accounts to JSON | ⏳ |
| `copy` | Copy to clipboard | ⏳ |
| `search` | Focus search input | ⏳ |

### Navigation (6)

| Name | Purpose | Shipping v1.0 |
|---|---|---|
| `chevron-left` | Back, previous | ⏳ |
| `chevron-right` | Forward, drill affordance | ✅ |
| `chevron-up` | Collapse, sort-asc glyph | ⏳ |
| `chevron-down` | Expand, sort-desc glyph | ⏳ |
| `close` | Dismiss dialog, clear filter | ✅ |
| `menu` | More actions trigger | ⏳ |

### Semantic (4)

| Name | Purpose | Shipping v1.0 |
|---|---|---|
| `info` | Tooltip trigger, info toast | ✅ |
| `warning` | Warning toast, cooldown | ⏳ |
| `error` | Error toast, validation | ⏳ |
| `question` | Help trigger, `?` cheatsheet | ⏳ |

### Data (4)

| Name | Purpose | Shipping v1.0 |
|---|---|---|
| `stream` | LiveRequestStream nav, activity | ✅ |
| `metric` | Metrics route nav | ⏳ |
| `log` | Logs route nav, view-logs action | ⏳ |
| `token` | Token / key affordance | ⏳ |

**Total inventory: 27 icons. 6 shipping source SVGs in Part 2; 21 deferred
with locked specs.** The six chosen for v1.0 give Track 3 enough to render the
navigation + status pills + LiveRequestStream block hints without blocking.

---

## Usage guide

### When to use icons vs text vs both

- **Icon only** — inside focusable icon-only buttons (e.g. close "×", refresh). ALWAYS pair with `aria-label`. Reinforce with a `Tooltip` showing the label on hover.
- **Icon + text** — primary navigation, primary buttons, empty-state heroes. Icon carries genre recognition; text carries semantics.
- **Text only** — body prose, row cells, form labels. Don't stud icons throughout prose — noise.
- **Never color-only** — every state indicator carries BOTH a status-dot color AND a text label (`docs/components/status-pill.md`).

### Accessibility

```html
<!-- Decorative (inside a button that already has text or aria-label) -->
<svg class="kx-icon" aria-hidden="true" width="16" height="16" viewBox="0 0 24 24"
     stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" fill="none">
  <!-- paths -->
</svg>

<!-- Meaningful (stands alone, no accompanying text) -->
<svg class="kx-icon" role="img" aria-labelledby="refresh-t" width="16" height="16" viewBox="0 0 24 24"
     stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" fill="none">
  <title id="refresh-t">Refresh pool</title>
  <!-- paths -->
</svg>
```

### Sizing

Use `width` and `height` attributes equal to each other (never non-square). Prefer `em` units if the icon should scale with surrounding text, absolute `px` if the icon sits in a fixed-height control.

### Color

Always `stroke="currentColor"`. Wrap the icon in a span with the intended color:

```html
<span class="kx-text-danger">
  <svg class="kx-icon" ...><!-- error icon --></svg>
</span>
```

---

## File layout

```
tools/icons/
  ├── status-healthy.svg     ← shipping v1.0
  ├── refresh.svg            ← shipping v1.0
  ├── close.svg              ← shipping v1.0
  ├── chevron-right.svg      ← shipping v1.0
  ├── info.svg               ← shipping v1.0
  ├── stream.svg             ← shipping v1.0
  ├── icons.ts               ← typed manifest (includes all 27 with paths where present)
  └── README.md              ← per-file attribution + how to add a new icon
```

Each SVG:
- Includes a license/attribution header comment citing Tabler Icons as
  design-language inspiration (MIT).
- Optimized via `svgo` before commit. No editor metadata, no `xmlns:sodipodi`.
- Uses `currentColor` for stroke; no hard-coded hex.

---

## How to add a new icon

1. Draw the icon at 24×24 on a 1px pixel grid (Figma / Illustrator / hand).
2. Keep strokes at 1.5px; rounded caps + joins; no fill.
3. Export optimized SVG — run through `svgo --config=tools/icons/svgo.config.mjs`.
4. Add header comment with attribution: `<!-- kiroxy — inspired by Tabler Icons (MIT). -->`.
5. Save to `tools/icons/{kebab-name}.svg`.
6. Add entry to `tools/icons/icons.ts` with `{ name, source }`.
7. Add row to §"Inventory" above.
8. If replacing a deferred icon, flip its ✅ / ⏳ status here.

**Never** import a full icon library and tree-shake. The ~20 kiroxy icons are
a curated set, not a slice of a bigger one.

---

## Anti-patterns

- ❌ **`<i class="lucide-refresh">…</i>`** — icon fonts break `currentColor` and force a runtime CSS.
- ❌ **`import * from 'lucide-react'`** — bulk import. Even with tree-shaking, it signals template.
- ❌ **Mixed stroke widths** — some 1.5px, some 2px. Inconsistent aesthetic.
- ❌ **Fills for status pills** — status is communicated by the pill's background and text, not by the glyph's fill.
- ❌ **Color-only icons for state** — e.g. "the icon is red means danger". Fails 1.4.1 Use of Color.
- ❌ **Custom illustrations for empty states** — DESIGN_SYSTEM.md §13 forbids; icons only.
- ❌ **Animated icons** — except the inline `LoadingDots` which is itself a component. No spinning refresh glyphs.
