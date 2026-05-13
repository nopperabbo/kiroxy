# nord — arctic calm

> One of six Phase V dashboard variants. See
> `.sisyphus/plans/variant-nord-manifesto.md` for the locked
> philosophy this package commits to.

## Philosophy

Cold, composed, predictable — a palette calibrated for eight-hour
sessions in a dim room, every color contributing to calm rather than
urgency.

## Visual signatures

- **Literal Nord palette** (nordtheme.com, MIT-licensed): canvas `#2E3440`,
  panel `#3B4252`, foreground `#ECEFF4`, frost accent `#88C0D0`, aurora
  status colors muted. Every CSS var is named after its Nord original
  (`--nord0 … --nord15`) so the provenance is visible in one file.
- **6px radii, single Inter family (no mono — data cells use Inter with
  `font-variant-numeric: tabular-nums`).** Single 1px hairline
  `var(--hairline)` separates panels; no shadows anywhere except
  the palette's modal drop-shadow.
- **All motion: 240ms cubic-bezier(0.4, 0, 0.2, 1), period.** No
  bounce, no spring, no stagger. Status dots cross-fade. Rows fade on
  first appearance. The motion feels like breathing.

## Things explicitly NOT doing

- No accent color beyond the Nord palette.
- No shadows on panels — layering is via nord1/nord2/nord3 progressively
  lighter charcoal.
- No density toggle. One density: comfortable (40px rows, 24px gutters).
  Nord is for long sessions — we don't pretend compact is useful here.

## Data surface

Consumes `GET /dashboard/api/state`, polled every 2s. Import is
instruction-only (via palette hint `Import accounts`) because
`/dashboard/api/import` is a v1.1 backend TODO.

## Tech stack

Vanilla HTML + CSS + ES module, no build step. System font fallback
(Inter → system-ui). Bundle < 10KB gzipped total.

## References

- `nordtheme.com` — the palette itself, cited by CSS var names
- Arc Browser — calm predictable motion, muted everything
- Zed editor's Nord theme — proving the palette on a dev tool

## Bundle size

- HTML ≈ 1.3KB gzip
- CSS ≈ 2.5KB gzip
- JS ≈ 2.5KB gzip
- Total ≈ 6KB gzipped (under the 10KB target)

## Self-graded rubric (docs/IMPLEMENTATION_RUBRIC.md)

Nord commits to its own palette; the mansion design-system tokens don't
apply. Score reflects that.

- §1 Token compliance: **N/A** — nord uses Nord vars, not mansion tokens.
  All colors, spacing, motion are CSS custom properties (just named
  differently).
- §2 Interaction compliance: **Partial**. `⌘K` palette with arrow-key
  navigation; `r` refresh when not typing; `Esc` closes. Nord omits
  chord navigation (`g a / g r`) — not philosophically needed.
- §3 Accessibility: **Pass**. `--nord6` `#ECEFF4` on `--nord0` `#2E3440`
  ≈ 11:1 (AAA). `--nord8` frost accent on panel ≈ 4.8:1 (AA). Live dot
  has text-label companion (not color-only). `prefers-reduced-motion`
  collapses all transitions to 0ms via `transition-duration: 0ms`
  override. `:focus-visible` 2px frost outline with 2px offset.
- §4 Motion: **Pass**. One duration (`--dur: 240ms`), one easing
  (`--ease: cubic-bezier(0.4, 0, 0.2, 1)`). No libraries. One
  `@keyframes nord-fade` for new-row arrival.
- §5 Icons: **N/A** — nord uses a single `·` dot glyph + text labels.
- §6 Performance: inline CSS/JS (embedded), no CDN, no fonts downloaded.
- §7 Docs: this file + manifesto committed.
- §8 LiveRequestStream: **N/A** — backend has no `/api/requests` yet.

**Composite score:** graded against own palette ethos — see
`docs/VARIANTS.md`.
