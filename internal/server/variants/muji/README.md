# muji — Japanese minimalism

> One of six Phase V dashboard variants. See
> `.sisyphus/plans/variant-muji-manifesto.md` for the locked
> philosophy this package commits to.

## Philosophy

Nothing unnecessary — whitespace is the content, hover reveals, and
the dashboard achieves authority through absence rather than
decoration.

## Visual signatures

- **Near-white canvas `#fafafa`** with ink `#1a1a1a`. Single 8-step
  gray ramp, no accent color in default state.
- **Sections separated by whitespace (min 56px), NEVER by borders or
  cards.** No panel background, no shadow, no divider line. The page
  is one continuous sheet, sections implied by vertical rhythm.
- **Summary is a single sentence** (e.g. "49 requests · 2 errors · 3
  accounts"), not a grid of stat cards. Empty states show one phrase
  in a generous void.

## Things explicitly NOT doing

- **NO JavaScript.** Live-refresh is `<meta http-equiv="refresh"
  content="5">`. Import is a plain HTML form. The variant's test
  asserts the rendered HTML contains no `<script>` tag.
- **No icons, no colors, no motion.** If an affordance needed an icon,
  it needed a better label.
- **No fixed header/sidebar/footer.** The page scrolls as one.

## Data surface

This is the ONLY variant rendered server-side. `muji.Register()` takes
a `SnapFn` that returns the current snapshot; `server.Server.mujiSnap`
(in `internal/server/muji_bridge.go`) maps the existing
`DashboardStateProvider` into the muji-local `Snapshot` / `Account`
types. The page refreshes every 5s via meta-refresh — no JS poll loop.

Import is a plain HTML form posting to `/dashboard/api/import` (404
today; the form will work when the endpoint lands in v1.1).

## Tech stack

Pure server-rendered HTML + CSS. ZERO client JS. Uses Go's
`html/template` with two tiny helpers (`uptime`, `state`). This is not
a limitation — it is the philosophy in code. Bundle target: < 6KB.

## References

- `muji.com` — the brand's digital expression of its philosophy
- Kinfolk magazine layouts — whitespace rhythm, sans dignity
- Kobo e-reader UI — minimalism by hardware constraint
- Apple's Reduced Motion aesthetic — stillness reads as premium

## Bundle size

- rendered HTML ≈ 1.4KB gzip (varies with account count)
- CSS ≈ 1.7KB gzip
- Total ≈ 3.2KB gzipped — smallest of the six variants

## Self-graded rubric (docs/IMPLEMENTATION_RUBRIC.md)

- §1 Token compliance: **N/A** — muji uses its own `--g0..--g7` ramp.
- §2 Interaction compliance: **Minimal**. No `⌘K`, no chord nav, no
  live region — philosophy refuses them. `Tab` reaches every link and
  form control in source order.
- §3 Accessibility: **Pass**. `#1a1a1a` on `#fafafa` ≈ 16:1 (AAA).
  `:focus-visible` 1px outline with 4px offset. `prefers-reduced-motion`
  disables the 140ms link transitions. Form has explicit `<label>`.
- §4 Motion: **Pass**. Only motion is 140ms opacity/color transitions
  on links; silenced by `prefers-reduced-motion`.
- §5 Icons: **N/A** — no icons by philosophy.
- §6 Performance: HTML + CSS only, ≈ 3.2KB gz. SSR means FCP = TCP
  establish + first-paint (no hydration).
- §7 Docs: this file + manifesto.
- §8 LiveRequestStream: **N/A** — refresh-by-meta is the live surface.

**Composite score:** graded against own ethos — see `docs/VARIANTS.md`.
