# Variant 2 — `dashboard-paper` — "INK ON CREAM"

## One-sentence philosophy
This isn't a tool, it's a document — quiet dignity, long-read focus, a
printed report from a thoughtful ops team rendered in HTML.

## 3 visual signatures (5-second description)
1. **Cream paper background `#F5F1E8`** with deep ink text `#1A1713`. No
   dark mode — paper doesn't have a dark mode, the operator reads this in
   daylight or a warm-lit room. Single muted accent: forest green
   `#2D5F3F` for links only.
2. **Serif heading face (Newsreader, self-hosted)** + sans data face
   (Inter Tight). Body copy is 16.5px / 1.65 leading — actual reading
   size. Data cells use tabular-nums on Inter Tight.
3. **Narrow 960px max-width container, generous 44px table rows, 1px
   rules in `rgba(0,0,0,0.07)` instead of borders.** No sidebar. Nav is
   a top bar of three serif links. Command palette via `⌘K` is the
   only operator shortcut — the UI refuses to decorate itself with
   affordances.

## 3 things EXPLICITLY NOT DOING
- **No sidebar, no tabs, no pills, no cards.** Sections separated by
  whitespace and a serif heading, the way a report is organized.
- **No dark mode toggle.** Paper is paper. Commitment to one palette is
  part of the taste.
- **No motion beyond `<details>` expand and a single 200ms opacity fade
  on new request rows.** This is a document, not an animation demo.

## Reference citations
- **Stripe operator docs** (stripe.com/docs pages) — serif headings,
  narrow width, dignified density
- **The New York Times reading mode** — cream background, ink typography
- **Edward Tufte's `tufte-css`** — margin notes, no-color data
- `docs/REFERENCE_GALLERY.md` entry: "The Economist data graphics" —
  low-saturation accents on warm paper (to be added v1.1)

## Tech stack defense
**Vanilla HTML + CSS + minimal ES module for polling.** No SPA, no
hydration — the page server-renders the latest snapshot on first paint
and refreshes only the data region. This matches the "document, not
application" philosophy. Bundle target: < 12KB gz (most of it is font
subset). Uses `Newsreader-latin-subset.woff2` + Inter Tight variable.
