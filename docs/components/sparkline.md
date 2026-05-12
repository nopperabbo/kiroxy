# sparkline

Hand-rolled inline SVG line chart, 60 points max. No library. Used in account rows (requests/min trend), metrics panels, and small "glance" tiles.

**Pattern inheritance:** hj01857655/kiro-account-manager `UsageTrendChart` (hand-rolled SVG precedent cited in `REFERENCE_GALLERY.md`), Grafana's single-stat-with-sparkline panels, Vercel Geist data viz discipline.

**Design system citation:** `docs/DESIGN_SYSTEM.md` §8 (Sparkline primitive "hand-rolled SVG, no library"), §13 (no chart libraries).

---

## Anatomy

```
<Sparkline>                      ← <svg> role="img"
  ├── <PathFill>                 ← optional gradient/solid under the line
  ├── <PathLine>                 ← the line itself; currentColor
  ├── [optional] <HighlightDot>  ← latest point marker
  └── <TextAlt>                  ← <title> element for SRs
</Sparkline>
```

Markup template:

```html
<svg class="kx-sparkline"
     viewBox="0 0 120 32" preserveAspectRatio="none"
     role="img" aria-labelledby="sp-title-1 sp-desc-1" width="120" height="32">
  <title id="sp-title-1">Requests per minute, last 60 minutes</title>
  <desc id="sp-desc-1">Peak 42 at 11:15; current 18.</desc>
  <path class="kx-sparkline__fill" d="M0,28 L0,20 L2,18 L4,15 … L120,22 L120,32 Z"/>
  <path class="kx-sparkline__line" d="M0,20 L2,18 L4,15 … L120,22" fill="none"/>
  <circle class="kx-sparkline__dot" cx="120" cy="22" r="2"/>
</svg>
```

---

## API

| Attribute | Values | Default | Description |
|---|---|---|---|
| `data-points` | `60` \| `30` | `60` | Max points rendered; older data dropped |
| `data-intent` | `neutral` \| `success` \| `warning` \| `danger` \| `accent` | `neutral` | Stroke color token |
| `data-fill` | `true` \| `false` | `false` | Render translucent area under line |
| `data-highlight` | `true` \| `false` | `true` | Show dot at latest point |
| `data-interactive` | `true` \| `false` | `false` | Enable hover crosshair + value readout |
| `width` / `height` | number (attr) | 120 × 32 | SVG dimensions |

---

## Variants

- **`line`** — default; single stroke line.
- **`area`** — line + translucent fill below.
- **`bars`** — 60 thin vertical rects; for discrete events (errors/min). Uses same API.
- **`pulse`** — line + animated trailing dot; the dot position uses `@property --sp-progress` to track the newest point across sparkline updates. Reserved for live streams where the LATEST value is operationally critical (incident dashboard).

---

## States

| State | Trigger | Visual |
|---|---|---|
| Idle | data rendered | Static line |
| Hover (interactive variant) | pointer crosshair over x-axis | Vertical guide line + value readout `Tooltip` at crosshair |
| Updating | new point arrives | Line re-renders with new point + dot animates to new position in 200ms |
| Loading | no data yet | See `skeleton.md` `data-shape="sparkline"` |
| Empty | truly no data | Flat dim line + "No data" micro-label below |

---

## Accessibility

- Root `<svg>` is `role="img"`.
- `<title>` element labeled with a summary ("Requests per minute, last 60 minutes").
- `<desc>` element with the key insight ("Peak 42 at 11:15; current 18.").
- Both linked via `aria-labelledby` + `aria-describedby`.
- Interactive variant has focus-visible: the svg becomes focusable (`tabindex="0"`), arrow keys move the crosshair, `Enter` copies the current point's value.
- `prefers-reduced-motion: reduce` → update animation collapses to instant.

---

## Motion

| Transition | Token | Notes |
|---|---|---|
| Update (new point) | `--dur-moderate` | Line path morph; dot translates |
| Load | instant | Rendered fully on first paint (no "grow from left" — decorative) |
| Hover crosshair | instant | Follows pointer |

---

## Composition

**Contains:** `<path>` elements; optional `<circle>` dot.
**Contained by:** `TableCell` (per-row trend), `Dialog` body, `Drawer` (per-account detail), `MetricsPanel`.
**Paired with:** `Tooltip` (on hover), `RelativeTimeCard` (for "last 60m" label), `StatusPill` (intent coloring signal).

---

## Anti-patterns

- ❌ **Chart library import** (recharts, chart.js, d3). DESIGN_SYSTEM.md §13 forbids. Hand-rolled SVG is required.
- ❌ **Y-axis labels or ticks.** Sparklines are sparse by definition. The `<desc>` carries the key value.
- ❌ **Multiple series in one sparkline.** One series only. Multi-series is a full chart — escalate to a metrics panel.
- ❌ **Animated "draw in" on mount.** Decorative motion violation.
- ❌ **Tooltip for every point.** Interactive variant only; default is static.
- ❌ **Stroke width > 1.5px.** Crowds the tiny canvas. Use 1.25 or 1.5.
- ❌ **Fill with high alpha** (> 0.2). Heavy fill reads as a bar chart; keep area variants at ~0.14 (`color-mix(... 14%, transparent)`).

---

## Reference

- **hj01857655/kiro-account-manager** `UsageTrendChart` — hand-rolled SVG precedent.
- **Grafana** single-stat-with-sparkline panels.
- **Vercel Geist** metric cards.

---

## Example usage

**Line sparkline in a table cell (static, 120×32):**

```html
<td>
  <svg class="kx-sparkline" viewBox="0 0 120 32" preserveAspectRatio="none"
       role="img" aria-labelledby="sp-t-2" width="120" height="32"
       data-intent="success">
    <title id="sp-t-2">Requests per minute, last hour — current 18</title>
    <path class="kx-sparkline__line" fill="none"
          d="M0,20 L2,18 L4,15 L6,18 L8,14 L10,12 L12,10 L14,13 L16,11 L18,9 L20,7 L22,10 L24,13 L26,11 L28,14 L30,16 L32,18 L34,21 L36,19 L38,22 L40,24 L42,22 L44,20 L46,18 L48,16 L50,14 L52,12 L54,10 L56,8 L58,6 L60,5 L62,7 L64,9 L66,11 L68,13 L70,15 L72,17 L74,19 L76,21 L78,23 L80,22 L82,20 L84,18 L86,16 L88,14 L90,12 L92,10 L94,12 L96,14 L98,16 L100,18 L102,20 L104,22 L106,24 L108,22 L110,20 L112,18 L114,20 L116,22 L118,20 L120,22"/>
    <circle class="kx-sparkline__dot" cx="120" cy="22" r="2"/>
  </svg>
</td>
```

**Area variant (account drill-down header):**

```html
<svg class="kx-sparkline" viewBox="0 0 240 48" preserveAspectRatio="none"
     role="img" aria-labelledby="sp-t-3 sp-d-3" width="240" height="48"
     data-intent="accent" data-fill="true">
  <title id="sp-t-3">Tokens per hour, last 24h</title>
  <desc id="sp-d-3">Peak 18,400 at 14:00 yesterday; current 7,200.</desc>
  <path class="kx-sparkline__fill" d="M0,40 L0,30 L8,28 … L240,32 L240,48 Z"/>
  <path class="kx-sparkline__line" fill="none" d="M0,30 L8,28 … L240,32"/>
</svg>
```
