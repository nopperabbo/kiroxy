# paper — ink on cream

> One of six Phase V dashboard variants. See
> `.sisyphus/plans/variant-paper-manifesto.md` for the locked
> philosophy this package commits to.

## Philosophy

This isn't a tool, it's a document. Quiet dignity, long-read focus —
a printed report from a thoughtful ops team rendered in HTML. Cream
paper background, ink text, serif headings, narrow column.

## Visual signatures

- **Cream paper `#f5f1e8`** with deep ink `#1a1713`. No dark mode — paper
  doesn't have a dark mode. Single muted accent: forest green
  `#2d5f3f` for links only.
- **Newsreader serif for headings** (declared via font-family stack with
  system fallback to Charter/Georgia), **Inter Tight for body** (system
  fallback). Body at 16.5px / 1.65 leading — actual reading size.
- **Narrow 1040px max-width container, 44px+ row spacing, 1px rules in
  `rgba(0,0,0,0.07)` for dividers.** No sidebar; nav is three serif
  links at the top.

## Things explicitly NOT doing

- No sidebar, no tabs, no cards. Sections separated by whitespace and a
  serif heading.
- No dark mode toggle. Paper is paper.
- No motion beyond a single 600ms row fade on newly-arrived rows and
  180ms link hover transitions. This is a document, not an animation.

## Data surface

Consumes `GET /dashboard/api/state`, polled every 2s. Import is
instruction-only (`kiroxy import-json < tokens.json`) because
`/dashboard/api/import` is a v1.1 backend TODO.

## Tech stack

Vanilla HTML + CSS + ES module. No build step — dist/ is source of
truth. Font stack uses system fallbacks (no CDN fetches, no self-hosted
woff2). This keeps the variant portable and honors the "document" ethos.

## References

- `stripe.com/docs` operator pages — serif + narrow + dignified density
- The New York Times reading mode — cream + ink typography
- Edward Tufte's `tufte-css` — margin notes, no-color data
- `The Economist` data graphics — low-saturation accents on warm paper

## Bundle size

- HTML ≈ 1.3KB gzip
- CSS ≈ 2.3KB gzip
- JS ≈ 2.4KB gzip
- Total ≈ 6KB gzipped

Well under the 12KB target in the manifesto.

## Self-graded rubric (docs/IMPLEMENTATION_RUBRIC.md)

Paper deliberately rejects the mansion rubric for fundamental tokens —
the mansion system is dark-first, accent-rich, data-dense ops aesthetic.
Paper is a different product. The score below grades paper against its
own philosophy.

- §1 Token compliance: **N/A** (paper uses its own cream/ink/forest
  palette, not mansion's OKLCH oklch dark tokens).
- §2 Interaction compliance: **Partial**. `⌘K` opens palette; arrow-key
  navigation; `Esc` closes. Paper deliberately omits `g a / g r / g m /
  g s` chords (the document ethos rejects chord navigation).
- §3 Accessibility: **Pass**. Body contrast #1a1713 on #f5f1e8 ≈ 13:1
  (AAA). Link accent #2d5f3f on #f5f1e8 ≈ 7:1 (AAA). `:focus-visible`
  2px forest green outline with 2px offset. `prefers-reduced-motion`
  kills the row fade.
- §4 Motion: **Pass**. Only animations are the 600ms row fade
  (`@keyframes paper-fade`) and 180ms link hover transition. No motion
  libraries.
- §5 Icons: **N/A** — paper uses glyphs (`▸` prompt, `·` separators)
  instead of SVG icons.
- §6 Performance: all inline. System font fallback, no CDN. Bundle < 8KB.
- §7 Docs: this file + manifesto + typography citations.
- §8 LiveRequestStream: **N/A** — paper renders a snapshot report, not
  a live stream. Table rows fade in on first appearance.

**Composite score:** graded against own ethos, not mansion — see
`docs/VARIANTS.md`.
