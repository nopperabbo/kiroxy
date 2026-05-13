# Variant 1 — `dashboard-brutal` — "TERMINAL"

## One-sentence philosophy
Information is the product. Chrome is the enemy. The dashboard is a monospaced
data readout — a htop for a pool of Kiro accounts, no more, no less.

## 3 visual signatures (5-second description)
1. **Pure black background (`#000000`)** with a single signal color
   (phosphor green `#00FF88`) and otherwise 8-step grayscale. No gradient,
   no shadow, no border-radius anywhere (0px corners).
2. **JetBrains Mono everywhere, single weight (400), single size (13px) for
   body.** Headers are same size but uppercase + tracking +0.08em. Tables
   use ASCII box-drawing characters (`─ ═ │ ┃ ┌ ┐ └ ┘`) as dividers,
   never CSS `border`.
3. **Status is one glyph** — `✓` success, `✗` error, `·` idle, `!` warn —
   not colored pills, not icons, just a character in the signal color.
   Live data flicker = single-character blinking cursor `▌`.

## 3 things EXPLICITLY NOT DOING
- **No rounded corners.** Anywhere. `border-radius: 0` is the law.
- **No color beyond grayscale + one signal green.** Error states use the
  same green prefixed with `✗` or `ERR`, not red. (Rationale: real ops
  engineers read text, not color codes. Red/green ambiguity is a trope.)
- **No icons, no SVG, no images.** Character glyphs only. "Settings" is
  the word `settings`, not a gear.

## Reference citations
- **htop** (ncurses TUI) — ASCII grid, terminal aesthetic as operator UX
- **plan9 acme editor** — text as UI; chrome is syntax
- **AWS cli-v2 `--output table`** — hash-rule dividers; data density
- New reference entry: `docs/REFERENCE_GALLERY.md` extension — "TUI in the
  browser" category. To be added in v1.1 gallery pass.

## Tech stack defense
**Vanilla HTML + CSS + ES2022 JS, no build step.** A terminal UI that ran
through Vite would betray its own philosophy. Bundle target: < 8KB gzipped
total. `fetch` polls `/dashboard/api/state` every 2s; DOM updates use
`textContent` only (no innerHTML, no framework diffing). Zero dependencies.
