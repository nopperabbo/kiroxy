# Variant 5 — `dashboard-muji` — "JAPANESE MINIMALISM"

## One-sentence philosophy
Nothing unnecessary — whitespace is the content, hover reveals, and the
dashboard achieves authority through absence rather than decoration.

## 3 visual signatures (5-second description)
1. **Near-white canvas `#FAFAFA`** with text `#1A1A1A`. Single 8-step
   gray ramp (`#FAFAFA, #F2F2F2, #E5E5E5, #C8C8C8, #888888, #555555,
   #2A2A2A, #1A1A1A`). **Zero accent color in the default state.** A
   link is underlined on hover, nothing more.
2. **Components are separated by whitespace (min 32px), NEVER by
   borders or cards.** No panel background, no shadow, no divider line.
   The page is one continuous sheet, sections implied by vertical rhythm.
3. **Numbers use tabular-nums, headings are 14px with 0.02em tracking,
   a single weight (500) stands for emphasis.** Empty states show one
   word or phrase, centered in a generous void. "No requests yet" —
   nothing else. This is the Muji "ma" (negative space) principle.

## 3 things EXPLICITLY NOT DOING
- **No JavaScript.** Live-refresh is `<meta http-equiv="refresh" content="5">`.
  Import UI is a plain HTML form posting to the backend. This is the
  purest expression of "nothing unnecessary."
- **No icons. No colors. No motion.** If an affordance needs an icon,
  it needed a better label. If a state needs a color, it needed a better
  sentence.
- **No fixed header/sidebar/footer.** The page scrolls as one. Navigation
  is three text links at the top, underlined on hover only.

## Reference citations
- **muji.com** — the brand's digital expression of its philosophy
- **Kinfolk magazine layouts** — generous whitespace, sans-serif dignity,
  single-axis rhythm
- **Kobo e-reader UI** — minimalism forced by hardware constraint
- **Apple's Reduced Motion aesthetic** — proving stillness reads as premium
- `docs/REFERENCE_GALLERY.md` future entry: "A Book Apart website" — serif
  wordmark on near-white, same restraint family

## Tech stack defense
**Pure server-rendered HTML + CSS, ZERO client JS.** The backend emits a
complete HTML page on `GET /dashboard-muji`; the browser refreshes every
5s via meta refresh. This is not a limitation — it is the philosophy in
code. Bundle target: < 6KB (HTML + inline CSS, no external assets; font
stack is system `Inter, -apple-system, sans-serif`). The only "framework"
is HTML. Import form posts to the existing `/dashboard/api/import`
endpoint (returns 404 today — v1.1 backend TODO documented in README).
