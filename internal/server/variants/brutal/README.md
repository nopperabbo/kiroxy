# brutal — terminal variant

> One of six dashboard taste-exploration variants shipped in Phase V.
> See `.sisyphus/plans/variant-brutal-manifesto.md` for the locked
> philosophy this package commits to.

## Philosophy

Information is the product. Chrome is the enemy. The dashboard is a
monospaced data readout — a `htop` for a pool of Kiro accounts, no more,
no less.

## Visual signatures

- **Pure black `#000000`** canvas, single signal color (phosphor green
  `#00FF88`), 8-step grayscale for everything else. No gradient, no
  shadow, no border-radius anywhere (`border-radius: 0`).
- **JetBrains Mono single weight (400), single body size (13px).**
  Headers same size, uppercase, `letter-spacing: 0.08em`.
- **ASCII box-drawing characters (`─ ═ │ ┃ ┌ ┐ └ ┘`)** as dividers,
  never CSS `border`. Status is a single glyph (`✓ ✗ · !`), not a pill.

## Things explicitly NOT doing

- No rounded corners anywhere.
- No color beyond grayscale + one signal green. Errors use the same
  green prefixed with `✗` — no red/green ambiguity.
- No icons, no SVG, no raster images. Character glyphs only.

## Data surface

Consumes `GET /dashboard/api/state` (polled every 2s). No new backend
endpoints. Import is instruction-only (`kiroxy import-json < tokens.json`)
because `/dashboard/api/import` is a v1.1 backend TODO.

## Tech stack

Vanilla HTML + CSS + ES2022 JS. Zero dependencies, no build step — the
dist/ files are the source of truth. This is deliberate: a terminal UI
run through Vite would betray its own philosophy.

## References

- htop (ncurses TUI)
- plan9 `acme` editor
- AWS `cli --output table`

## Bundle size

Target: `< 8KB gzipped`. Achieved with inline CSS tokens, no external
fonts (uses system `JetBrains Mono` fallback chain), no JSON parsers
beyond built-in `fetch` + `JSON.parse`.

## Self-graded rubric (docs/IMPLEMENTATION_RUBRIC.md)

Many rubric items intentionally fail — brutal rejects the project's
default design-system tokens. This is a feature, not a bug. Score is
recorded here for transparency rather than parity.

- §1 Token compliance: **N/A** — brutal refuses `tokens.css`. Deliberate
  rejection in service of a different taste. ~0% compliance by design.
- §2 Interaction compliance: **Partial**. `⌘K`-equivalent uses `k`
  (single key, no modifier — fits the aesthetic). `i` import, `r`
  refresh, `?` help, `Esc` close dialogs. Tab order trivial (everything
  in source order).
- §3 Accessibility: **Pass**. Contrast #d0d0d0 on #000 = 13.4:1 (AAA);
  phosphor green on black = 12.4:1 (AAA). `aria-live="polite"` on the
  pool table. `prefers-reduced-motion` kills the cursor blink + row
  flash. `:focus-visible` 2px green outline.
- §4 Motion: **Pass**. No motion libraries. Only animations: cursor
  blink (1s steps) and single-shot 600ms row-flash on new rows. Both
  silenced under `prefers-reduced-motion`.
- §5 Icons: **N/A** — philosophy forbids icons entirely.
- §6 Performance: inline CSS + single `<script type=module>`; no CDN
  fetches; measured bundle ≈ 7KB gzip.
- §7 Docs: this file + manifesto committed.
- §8 Signature primitive LiveRequestStream: **N/A** — backend has no
  `/api/requests` yet. Pool table fills the signature slot.

**Composite score:** not meaningful — brutal is scored against its own
philosophy, not the mansion rubric. See `docs/VARIANTS.md` for the
cross-variant comparison.
