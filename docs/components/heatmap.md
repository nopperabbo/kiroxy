# heatmap

Grid of small cells colored by intensity. Shows per-account usage density across time (hour × day) and request-type distribution. Hand-rolled SVG, no library.

**Pattern inheritance:** GitHub contributions graph (the canonical activity heatmap), Grafana status-history panels, Linear insights heatmaps.

**Design system citation:** `docs/DESIGN_SYSTEM.md` §8 (Heatmap primitive), §13 (no chart libraries).

---

## Anatomy

```
<Heatmap>                             ← <svg> role="img"
  ├── <HeatmapAxis data-side="x">    ← hour labels (e.g. 0…23)
  ├── <HeatmapAxis data-side="y">    ← day labels (e.g. Mon…Sun)
  ├── <HeatmapGrid>
  │   └── <HeatmapCell>*             ← <rect> per cell; aria-label with value
  └── <HeatmapLegend>                 ← 5-step intensity legend, lives below
</Heatmap>
```

Markup template:

```html
<figure class="kx-heatmap">
  <figcaption class="visually-hidden" id="hm-t-1">
    Requests per hour, last 7 days. Peak 342 on Tuesday at 14:00. Current week total 8,420.
  </figcaption>
  <svg class="kx-heatmap__svg" role="img" aria-labelledby="hm-t-1"
       viewBox="0 0 264 84" width="264" height="84">
    <!-- 7 rows × 24 columns = 168 cells, each 10×10 with 1px gap -->
    <rect x="0"  y="0"  width="10" height="10" class="kx-heatmap__cell" data-intensity="0"></rect>
    <rect x="11" y="0"  width="10" height="10" class="kx-heatmap__cell" data-intensity="2"></rect>
    <rect x="22" y="0"  width="10" height="10" class="kx-heatmap__cell" data-intensity="1"></rect>
    <!-- … 168 total … -->
  </svg>
  <ol class="kx-heatmap__legend" aria-label="Intensity legend">
    <li data-intensity="0">0</li>
    <li data-intensity="1">1-20</li>
    <li data-intensity="2">21-100</li>
    <li data-intensity="3">101-200</li>
    <li data-intensity="4">200+</li>
  </ol>
</figure>
```

---

## API

| Attribute (wrapper) | Values | Default | Description |
|---|---|---|---|
| `data-scale` | `linear` \| `log` | `linear` | How raw counts map to 5 intensity buckets |
| `data-intent` | `accent` \| `success` \| `neutral` | `accent` | Base hue for intensity gradient |
| `data-interactive` | `true` \| `false` | `false` | Hover tooltip per cell |
| `data-dimensions` | `"rows x cols"` | `"7 x 24"` | Grid geometry |

| Attribute (cell) | Values | Default | Description |
|---|---|---|---|
| `data-intensity` | `0` \| `1` \| `2` \| `3` \| `4` | `0` | Discrete bucket |
| `aria-label` | string | generated | SR announcement per cell |

---

## Variants

- **`week-hour`** — 7 rows × 24 cols; the GitHub-contributions clone.
- **`day-minute`** — 60 cols × 24 rows for a single day's minute-by-hour breakdown.
- **`model-account`** — rows by account, columns by model; surface quota concentration.
- **`static`** vs **`interactive`** — latter adds hover tooltip + cell focus via `tabindex`.

---

## States

| State | Trigger | Visual |
|---|---|---|
| Idle | default | Grid rendered |
| Hover (interactive) | pointer over cell | Cell outline `--color-accent-border`; tooltip shows exact value + timestamp |
| Focus (interactive) | keyboard nav | Cell outline `--color-focus-ring` |
| Loading | `aria-busy` | Skeleton grid (all cells `data-intensity="0"` and slightly muted) |
| Empty | `data-dimensions="0 x 0"` or no data | "No activity in the selected range" prose |

---

## Accessibility

- `<figure>` wrapper with `<figcaption>` describing the chart purpose + top-line insight.
- `<svg>` has `role="img"` + `aria-labelledby` pointing at figcaption.
- Cells carry `aria-label` in interactive mode ("Tuesday 14:00, 342 requests"); static mode uses figcaption alone.
- Intensity legend uses `<ol>` with `aria-label="Intensity legend"`.
- Color alone NEVER conveys intensity; the legend labels (`0`, `1-20`, `21-100`, `101-200`, `200+`) are mandatory.
- `prefers-contrast: more` switches the scale to patterns (diagonal/dot overlays) layered atop color — intensity readable without perception of hue.

**Keyboard (interactive):**
- Grid is a single focus stop; arrow keys move within.
- `Arrow` keys navigate cells.
- `Enter` opens drill-down dialog for that cell's timeslice.
- `Home` / `End` — first/last cell of current row.

---

## Motion

- **No animation on mount.** Whole grid renders in one frame.
- **No animation on update.** If data refreshes, cells swap intensity atomically.
- Hover outline transition: `--dur-quick`.

---

## Composition

**Contains:** `<svg>` with `<rect>` cells, axis labels, legend.
**Contained by:** `MetricsPanel`, `Dialog` body (zoomed drill-down), `Drawer` section.
**Paired with:** `Tooltip` (cell details), `Dialog` (cell drill-down to per-minute view).

---

## Anti-patterns

- ❌ **Continuous color scale.** 5 discrete buckets; continuous scales are harder to compare cells to legend.
- ❌ **Red-green gradient alone.** Colorblind hostile. kiroxy uses a single-hue intensity ramp (dim → accent saturated) regardless of semantic.
- ❌ **Tooltip without value.** Must include both the value AND the timestamp.
- ❌ **Cells smaller than 8×8 at effective density.** Below that, individual cells become invisible; switch to `Sparkline` or aggregate.
- ❌ **3D "heatmap cube".** No.
- ❌ **Animation on update.** Cell intensity changes atomically; any transition is decorative.

---

## Reference

- **GitHub** contributions heatmap (the canonical week-hour example).
- **Grafana** status-history panel.
- **Linear** insights heatmaps.

---

## Example usage

**Week × hour grid with legend:**

```html
<figure class="kx-heatmap" data-scale="log" data-intent="accent"
        data-dimensions="7 x 24" data-interactive="true">
  <figcaption id="hm-caption" class="visually-hidden">
    Requests per hour, last 7 days. Peak 342 on Tue 14:00. Week total 8,420.
  </figcaption>
  <svg class="kx-heatmap__svg" role="img" aria-labelledby="hm-caption"
       viewBox="0 0 264 84" width="264" height="84" tabindex="0">
    <!-- 168 <rect> elements with class kx-heatmap__cell and data-intensity 0..4 -->
  </svg>
  <ol class="kx-heatmap__legend" aria-label="Intensity legend">
    <li data-intensity="0">0</li>
    <li data-intensity="1">1-20</li>
    <li data-intensity="2">21-100</li>
    <li data-intensity="3">101-200</li>
    <li data-intensity="4">200+</li>
  </ol>
</figure>
```

**Static preview (in account drill-down drawer):**

```html
<figure class="kx-heatmap" data-interactive="false">
  <figcaption class="kx-heatmap__caption">
    Activity by hour (last 7 days) · <span class="mono">acct_01H8XJK9M2</span>
  </figcaption>
  <svg class="kx-heatmap__svg" role="img" viewBox="0 0 264 84" width="100%" height="84">
    <!-- … -->
  </svg>
</figure>
```
