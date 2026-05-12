# IMPLEMENTATION_RUBRIC.md — kiroxy

> Executable checklist for grading any kiroxy dashboard implementation against
> Part 1 + Part 2 design docs. Track 3 (mansion), any future iteration, or a
> fork should score itself before claiming parity.
>
> **Status:** v1.0 drafted 2026-05-13.
>
> **Companion documents:**
> - `docs/DESIGN_SYSTEM.md` — source of every token and rule
> - `docs/DESIGN_TOKENS_AUDIT.md` — contrast baseline
> - `docs/components/*.md` — per-primitive specs
> - `docs/INTERACTION_PATTERNS.md` — behavior contracts
> - `docs/KEYBOARD_SHORTCUTS.md` — shortcut map
> - `docs/ICONOGRAPHY.md` — icon inventory
> - `docs/INFORMATION_ARCHITECTURE.md` — route spec
>
> **How to use:** Run through each section. Mark ☑ only after running the
> verification command or checking the evidence. Target **>= 90%** for v1.3
> dashboard rebuild. Any < 100% items go to `BACKLOG.md` with a rationale.

---

## How to score

- `[ ]` — unchecked, not verified
- `[x]` — verified; include evidence in the PR description
- `[~]` — partially met; link to BACKLOG entry with deviation + rationale
- `[N/A]` — explicitly not applicable in this implementation; include reasoning

Final score: `checked + half(partial) / (total - N/A)`. Publish in the
implementation's `DASHBOARD.md` header.

---

## 1. Token compliance

Every visual value comes from `internal/server/assets/tokens/tokens.css`. Raw hex,
rgb(), hsl(), oklch() literals inside component CSS are a regression.

- [ ] All colors in component CSS use `var(--color-*)` — no raw `oklch()`, `hex`, `rgb()`, `hsl()`.
- [ ] No Tailwind utility classes in templates/components (or: if Tailwind is used, its palette maps 1:1 to `tokens.css` via a theme plugin — document).
- [ ] Typography uses `var(--font-sans | --font-mono)` and `var(--type-NN)`.
- [ ] Weights use `var(--weight-*)`.
- [ ] Spacing uses `var(--space-*)` or the density aliases.
- [ ] Radii use `var(--radius-*)`.
- [ ] Shadows use `var(--shadow-*)`.
- [ ] Motion uses `var(--dur-*)` + `var(--ease-*)`.
- [ ] Theme switching via `[data-theme]` attribute on `<html>`, NOT JS recomputing colors per element.
- [ ] `light-dark()` used OR `[data-theme]` attribute selectors override tokens — no third variant.
- [ ] Lint rule in place: `stylelint` catches raw color literals OR a `make tokens-lint` target exists.

**Verification:**
```bash
# List suspect raw color literals in dashboard component CSS.
rg -n "#[0-9a-fA-F]{3,8}|rgb\(|hsl\(|oklch\(" internal/server/next/client/src \
  | grep -v 'tokens.css' | grep -v 'var(--'
```

---

## 2. Interaction compliance

Per `INTERACTION_PATTERNS.md` + `KEYBOARD_SHORTCUTS.md`.

- [ ] `⌘K` opens palette from any screen (verify in /accounts, /requests, /metrics, /settings, /home).
- [ ] `/` focuses scoped search on current view.
- [ ] `?` opens keyboard-shortcut cheatsheet overlay.
- [ ] `Esc` closes top overlay in correct order: popover → tooltip → dialog → drawer → palette.
- [ ] Every action exposed via palette has a matching `Keycap` rendered on its palette row.
- [ ] Tab order flows left-to-right, top-to-bottom per the primitives' spec.
- [ ] `:focus-visible` outline visible on all interactive elements; mouse-focus never shows ring.
- [ ] Table sort tri-state: `unsorted → asc → desc → unsorted` on repeated click; `aria-sort` reflects state.
- [ ] Bulk action bar appears when `selection > 0`, disappears when selection cleared.
- [ ] Row `Enter` opens drawer; `⌘Enter` navigates to route.
- [ ] Navigation chords (`g a`, `g r`, `g m`, `g s`, `g h`) work with 700ms window.
- [ ] Theme cycle (`⌘/`) works.
- [ ] Density toggle (`⌘⇧T`) works and persists in localStorage.
- [ ] Sidebar toggle (`⌘\`) works and persists.
- [ ] Every item in `KEYBOARD_SHORTCUTS.md` has an implementation.

**Verification:** Run the keyboard-only smoke test in `docs/SMOKE_TEST.md`.

---

## 3. Accessibility (WCAG 2.2 AA)

Per `DESIGN_SYSTEM.md` §9 + `DESIGN_TOKENS_AUDIT.md`.

- [ ] Every interactive element has an accessible label (`aria-label`, `<label>`, or text content).
- [ ] Color contrast passes the values committed in `DESIGN_TOKENS_AUDIT.md` — re-run `python3 scripts/contrast.py` and confirm no regression.
- [ ] `prefers-reduced-motion: reduce` collapses all durations to 0ms; verified by running with the OS setting enabled.
- [ ] Focus trap in modals; focus returns to invoker on close.
- [ ] Focus trap in drawer; focus returns to the row that opened it.
- [ ] Live regions on SSE-driven sections (`aria-live="polite"` on LiveRequestStream feed; `aria-live="assertive"` on error banners).
- [ ] Keyboard-only flow verified end-to-end (palette → nav → drill → drawer → action → back).
- [ ] Screen-reader smoke test with VoiceOver or NVDA; every primary action announceable.
- [ ] Color never the sole signal — every status state carries dot + text.
- [ ] Icons have `aria-hidden="true"` when decorative, `<title>` + `role="img"` when meaningful.
- [ ] High-contrast theme (`data-theme="dark-highcontrast"`) ships and is reachable from the theme toggle.
- [ ] `prefers-contrast: more` auto-selects high-contrast theme on first load (v1.3 target; see `DESIGN_TOKENS_AUDIT.md` §6).

**Verification:**
```bash
python3 scripts/contrast.py      # exits 0
# Run axe-core or pa11y against /dashboard-mansion:
npx pa11y http://127.0.0.1:8787/dashboard-mansion
```

---

## 4. Motion compliance

Per `DESIGN_SYSTEM.md` §5 + `INTERACTION_PATTERNS.md` §9.

- [ ] View Transitions (`@view-transition { navigation: auto }`) used for cross-page navigation.
- [ ] `@starting-style` used for dialog / drawer / popover entrance; no JS entrance libraries.
- [ ] `transition-behavior: allow-discrete` wrapped around `display` / `overlay` transitions.
- [ ] Row-update flash via `@property --row-flash-progress`; single-shot 600ms; no looping.
- [ ] No `framer-motion`, `gsap`, `anime.js`, `motion-one`, `popmotion` in `package.json` (or equivalent).
- [ ] No `@keyframes pulse`, `bounce`, `float`, `shimmer`, `stagger` in component CSS (except the `LoadingDots` stagger which is inherent).
- [ ] `prefers-reduced-motion: reduce` kills all transitions/animations.
- [ ] Theme toggle uses `document.startViewTransition()` where supported; falls back gracefully.

**Verification:**
```bash
rg -n "framer-motion|gsap|anime\.js|motion-one|popmotion" package.json internal/server/next
rg -n "animation:\s*\w+" internal/server/next/client/src | grep -v 'dur-' | grep -v 'LoadingDots'
```

---

## 5. Icon compliance

Per `docs/ICONOGRAPHY.md`.

- [ ] All icons inline SVG.
- [ ] No bulk library import (`import * from 'lucide-react'`, full sprite sheet, or icon font).
- [ ] `aria-hidden` OR `aria-label` / `<title>` applied correctly.
- [ ] Stroke width adjustable via `--icon-stroke-width` custom property.
- [ ] Icon set used matches `tools/icons/icons.ts` manifest — no rogue icons.
- [ ] Stroke cap + join `round` everywhere.
- [ ] `currentColor` stroke; no hard-coded hex.

**Verification:**
```bash
rg -n "lucide-react|heroicons|radix-icons|phosphor-react|tabler-icons" package.json internal/server/next
```

---

## 6. Performance budgets

Per `docs/DASHBOARD_NEXT.md` budgets (v1.0 mansion baselined; v1.3 must hit same
or better).

- [ ] JS bundle < **100 KB** gzipped.
- [ ] CSS bundle < **25 KB** gzipped.
- [ ] Font WOFF2 subset to Latin + Latin Extended only; Inter + JetBrains Mono combined < **200 KB**.
- [ ] FCP < **400 ms** on localhost (measured via Chrome DevTools Lighthouse).
- [ ] LCP < **800 ms**.
- [ ] INP < **100 ms** (measured over 1 min of interaction).
- [ ] CLS = **0** (skeleton shape-matched to final content).
- [ ] Zero runtime CDN fetches — all assets `go:embed`.
- [ ] SSE reconnect works after simulated network drop (devtools offline toggle).

**Verification:** `pnpm build` output shows size summary; run Lighthouse in
CI with budget thresholds.

---

## 7. Documentation compliance

- [ ] Every component used in the dashboard has a corresponding spec in `docs/components/*.md`.
- [ ] Every design decision in the implementation cites `REFERENCE_GALLERY.md` entry OR `DESIGN_SYSTEM.md` section.
- [ ] Known deviations documented in `BACKLOG.md` with `rationale:` field.
- [ ] Deferred items tracked in `BACKLOG.md` with owner + target version.
- [ ] Implementation's own `DASHBOARD.md` header shows this rubric's score.
- [ ] Any newly-introduced primitive not in `DESIGN_SYSTEM.md` §8 is added via PR to both `DESIGN_SYSTEM.md` and `docs/components/{new}.md`.

---

## 8. Signature primitive — LiveRequestStream

Per `docs/VISION.md` §signature-thing + `docs/components/live-request-stream-block.md`.

- [ ] Home page (`/dashboard-mansion`) renders the LiveRequestStream feed — NOT a stat grid.
- [ ] New blocks arrive via SSE and animate in via `@starting-style` fade + 8px translateY.
- [ ] Row-update flash on existing blocks triggers on state change (cost update, retry).
- [ ] `⌘K` on a focused block opens item-tier palette with request-scoped actions.
- [ ] `⌘C` copies `request_id` to clipboard + shows `Toast`.
- [ ] `Enter` opens inspect drawer; `⌘R` opens replay drawer.
- [ ] Shareable permalink `/dashboard-mansion/requests/{id}` loads drawer open on cold navigation.
- [ ] Compact density mode collapses each block to single line with inline metadata.
- [ ] Subgrid aligns timestamps across blocks.
- [ ] No "activity pulse" animation on headers (only per-block single-shot flash).
- [ ] Attach-for-context (`a` on focused block) marks block with accent border + checkmark.

---

## 9. Data model contract (for Track 3 server coupling)

Track 3 server endpoints must conform to this shape for client rendering:

- [ ] `GET /dashboard-mansion/api/stream` — `text/event-stream`; events are `request_arrived`, `request_updated`, `account_state_changed`, `pool_snapshot`, `error`.
- [ ] Each `request_arrived` event carries `{ id, ts, account_id, model, method, path, status, latency_ms, tokens_in, tokens_out, cost, stream }`.
- [ ] `POST /dashboard-mansion/api/import` — accepts `application/json` body `{ accounts: [...] }`; returns `{ imported: N, skipped: N, errors: [...] }`.
- [ ] `DELETE /dashboard-mansion/api/accounts/:id` — soft-delete by default; `?purge=true` for hard.
- [ ] Server rejects writes on `KIROXY_READONLY_DASHBOARD=1`.
- [ ] Server `ETag`s the `/api/state` response so polling fallback is cheap.

---

## 10. Anti-pattern guard rails

These are explicit rejections. If any bullet below is observed in the
implementation, the PR is blocked pending fix.

- [ ] No six-color stat row with pastel icon backgrounds on any page.
- [ ] No drop shadows used as the primary depth cue (borders + surface colors only).
- [ ] No gradient backdrops on chrome.
- [ ] No rounded-2xl + shadow-xl + backdrop-blur "glass" cards.
- [ ] No default shadcn radius anywhere (kiroxy uses the scale from `tokens.css`).
- [ ] Inter and JetBrains Mono only — no Geist, Söhne, Berkeley, Monaspace, Bricolage, Mackinac.
- [ ] No `pastel` hue-shifted variants anywhere.
- [ ] No `jQuery`, `htmx`, `tailwind CDN runtime`.
- [ ] No Framer Motion / GSAP / anime.js / motion-one / popmotion.
- [ ] No bulk icon library import.
- [ ] No TanStack Query / SWR / Redux / Zustand / nanostores (SSE + Svelte stores are enough).
- [ ] No shimmer, pulse, float, stagger, bounce keyframes.
- [ ] No "welcome wizard" on first load.
- [ ] No marketing gradient hero on any page.

---

## Scoring rubric interpretation

| Score | Status |
|---|---|
| ≥ 95% | Ship candidate. Operator-grade. |
| 90-94% | Ship with BACKLOG entries for the gaps. |
| 80-89% | Needs work before merge; focus on accessibility and performance. |
| 70-79% | Major regressions; revisit DESIGN_SYSTEM.md compliance. |
| < 70% | Rejected; start over or use a different implementation. |

**A 100% score is not the goal.** Honest partial scores with documented
rationale beat over-claimed full scores. If an item is impossible in the
current stack, mark `[N/A]` with a defense.

---

## Self-score template

Paste this at the top of the implementation's `DASHBOARD.md`:

```markdown
## Implementation rubric score (v{VERSION} at {DATE})

Source: docs/IMPLEMENTATION_RUBRIC.md

Tokens:         x/11      (% )
Interaction:    x/15      (% )
Accessibility:  x/12      (% )
Motion:         x/8       (% )
Icons:          x/7       (% )
Performance:    x/9       (% )
Documentation:  x/6       (% )
Signature:      x/11      (% )
Data contract:  x/6       (% )
Anti-patterns:  x/14      (% )

TOTAL:         xx/99      (xx%)

Known deviations: see BACKLOG.md#dashboard-v{VERSION}.
Upcoming work:    see ROADMAP.md.
```
