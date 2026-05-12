# DESIGN_SYSTEM.md — kiroxy

> The design language that powers kiroxy's dashboard, CLI output, and documentation.
> Every decision here is grounded in `research-v3/REFERENCE_GALLERY.md` — if you see a
> number without a citation, it traces back to that dossier.
>
> **Status:** v1.0 (drafted 2026-05-13). Authoritative for dashboard-next rebuild (v1.3 target).
>
> **Companion documents:**
> - `docs/VISION.md` — what kiroxy is, who it's for, anti-goals
> - `docs/ROADMAP.md` — trajectory v1.x → v2.0
> - `research-v3/REFERENCE_GALLERY.md` — the evidence for every decision below

---

## 1. Design principles

Seven tenets, in order. Earlier beats later.

1. **Every interaction has a keyboard path.** The mouse is optional. If a feature requires pointing, it's a bug. Pattern inheritance: Linear, Superhuman, Raycast.
2. **Data density without visual noise.** The dashboard is for reading hundreds of requests per minute, not for looking good in a screenshot. 32-36px row height, tabular numerals, borders over shadows. Pattern inheritance: Supabase, Tailscale, Grafana.
3. **State is always visible, never inferred.** If a token is expired, show the countdown. If an account is in cooldown, show the duration. Never make the operator guess. Pattern inheritance: Grafana panel titles, PlanetScale deployability badges.
4. **Motion serves function, never decorates.** Every animation must justify its frame budget (Zed ethos). 120-200ms `cubic-bezier(0.16, 1, 0.3, 1)` max — the "Linear easing" that became universal. No bouncy springs. No decorative sparkles. No `@keyframes pulse` unless something is actually pulsing.
5. **Dark is default, not inverted-light.** kiroxy is an ops tool used for hours at a time. Dark mode is where the thought went; light mode is a considered port, not an afterthought. Pattern inheritance: Railway's commitment, Tailscale's dark-mode forcing function.
6. **One signature mechanic, not five features.** Warp has the Block. Linear has the command palette. Arc has the sidebar-instead-of-tabs. kiroxy has **the live request stream with cmd-click-to-attach-context** (see §12 — Signature primitive). Everything else orbits it.
7. **The integration list is not the product.** kiroxy routes to Kiro/CodeWhisperer via Anthropic or OpenAI shapes. That's scope. Do not let "here are 15 supported LLMs" become the landing-page hero. Pattern inheritance: anti-reference (Homepage, Open WebUI, LibreChat did this; LibreChat partially escaped; we don't fall in).

---

## 2. Color system

### 2.1 Core philosophy

- **OKLCH-first.** Native browser support in all evergreens since Firefox 113 (2023). Tailwind v4 and shadcn/ui v4 ship OKLCH defaults. No HSL fallbacks — they double the token surface and re-introduce the perceptual-lightness bug we're leaving behind.
- **Semantic tokens, not numeric scales.** `--bg`, `--surface`, `--elevated`, `--border`, `--text-dim`, `--text-default`, `--text-bright` — not `gray-50/100/.../900`. Pattern inheritance: Supabase (`background-surface-100/200/300`, `border-default/strong/stronger`), Vercel Geist (Color 1-10 as semantic bands).
- **Seven neutral stops for dark, five for light.** Fewer stops than Radix's 12 or Tailwind's 11 — kiroxy is not a consumer-SDK palette. Fewer stops = less "should this be step-3 or step-4?" paralysis.
- **One primary accent + four semantics.** Cyan-teal primary, plus success/warning/danger/info. Dual-accent (primary + secondary) is a marketing-brand pattern; it fights with status dots on an ops tool.
- **Depth comes from surface color and 1px borders, not drop shadows.** Pattern inheritance: Supabase (no shadows, only border-weight), Grafana (four-layer background model: canvas → primary → secondary → elevated), Linear (near-black canvas, 1px borders at `rgba(255,255,255,~0.06)`).

### 2.2 Dark mode tokens (primary theme)

Cool undertone, hue 285 (Tailwind v4 zinc hue) — faintly blue-tinted neutrals, reads as "terminal on a CRT" rather than pure black. Grafana's `rgb(204, 204, 220)` off-white text trick is the closest reference; Linear uses the same "not pure white" approach at `#e2e2e2`.

```css
:root, :root[data-theme="dark"] {
  color-scheme: dark;

  /* Neutrals — 7 semantic stops */
  --bg:              oklch(0.145 0.005 285);  /* canvas — the deepest */
  --surface:         oklch(0.205 0.006 285);  /* cards, table rows, sidebar */
  --elevated:        oklch(0.265 0.007 285);  /* hovered row, popover, modal */
  --border:          oklch(0.340 0.008 285);  /* default border */
  --text-dim:        oklch(0.660 0.015 285);  /* secondary, timestamps, metadata */
  --text-default:    oklch(0.830 0.012 285);  /* body */
  --text-bright:     oklch(0.970 0.003 285);  /* headings, emphasis */

  /* Accent — cyan-teal (reads "network/proxy", distinct from Vercel blue / Supabase green) */
  --accent:          oklch(0.720 0.130 200);
  --accent-hover:    oklch(0.780 0.130 200);
  --accent-pressed:  oklch(0.660 0.130 200);
  --accent-subtle:   color-mix(in oklch, var(--accent) 14%, transparent);
  --accent-border:   color-mix(in oklch, var(--accent) 30%, transparent);

  /* Semantics — tuned against --bg for perceptual contrast ≥ 0.5 ΔL */
  --success:         oklch(0.720 0.180 145);
  --warning:         oklch(0.800 0.165  85);
  --danger:          oklch(0.680 0.220  25);
  --info:            oklch(0.720 0.130 240);

  /* Derived utilities */
  --border-subtle:   color-mix(in oklch, var(--text-default) 8%, transparent);
  --text-muted:      color-mix(in oklch, var(--text-default) 62%, transparent);
  --shadow-pop:      0 8px 24px oklch(0.02 0 0 / 0.7);  /* Grafana's "shadow darker than surface" */
}
```

### 2.3 Dark-dimmed variant (GitHub Primer pattern)

Softer dark for reading sessions. Opt-in via `:root[data-theme="dark-dimmed"]`. Shipped alongside default dark from v1.3.

```css
:root[data-theme="dark-dimmed"] {
  --bg:          oklch(0.195 0.008 285);  /* lifted from pure black */
  --surface:     oklch(0.255 0.008 285);
  --elevated:    oklch(0.315 0.008 285);
  --border:      oklch(0.380 0.009 285);
  /* Semantic + accent tokens unchanged from dark */
}
```

### 2.4 Light mode tokens

5 neutral stops — fewer because dark has less contrast range to play with, so more stops add no perceptual information.

```css
:root[data-theme="light"] {
  color-scheme: light;

  --bg:              oklch(0.995 0     0  );
  --surface:         oklch(0.975 0.002 285);
  --elevated:        oklch(0.945 0.004 285);
  --border:          oklch(0.895 0.005 285);
  --text-dim:        oklch(0.500 0.015 285);
  --text-default:    oklch(0.180 0.010 285);
  --text-bright:     oklch(0.080 0.005 285);

  --accent:          oklch(0.500 0.155 200);
  --accent-hover:    oklch(0.440 0.155 200);
  --accent-pressed:  oklch(0.560 0.155 200);

  --success:         oklch(0.500 0.155 145);
  --warning:         oklch(0.540 0.170  60);  /* see §2.7 pitfall */
  --danger:          oklch(0.520 0.220  25);
  --info:            oklch(0.500 0.180 240);
}
```

### 2.5 High-contrast variants

Render-pattern: high-contrast is a first-class theme, not an accessibility settings fallback.

```css
:root[data-theme="dark-highcontrast"] {
  --bg:          oklch(0.08  0 0);
  --surface:     oklch(0.15  0 0);
  --border:      oklch(0.55  0 0);  /* 3× default border luminance */
  --text-dim:    oklch(0.800 0.010 285);
  --text-default:oklch(0.950 0.005 285);
  --text-bright: oklch(1.000 0     0  );
  --accent:      oklch(0.820 0.150 200);  /* higher L for AAA contrast */
}
```

### 2.6 Theme architecture

**`color-scheme: light dark` on `:root` is mandatory** for `light-dark()` to work.

```css
html { color-scheme: light dark; }
:root[data-theme="light"] { color-scheme: only light; }
:root[data-theme="dark"]  { color-scheme: only dark;  }
```

Theme is stored in `localStorage` + mirrored to `data-theme` attribute on `<html>`. System default = respect `prefers-color-scheme`.

**Zed-pattern: themes are a public API.** The dashboard ships a JSON schema at `/api/theme/schema.json` documenting every token; operators can write custom themes in JSON and hot-reload them via the command palette (`⌘K → Theme: Reload`). Ship Ayu / Gruvbox / One Dark ports as starter themes from v1.3.

### 2.7 Pitfalls to enforce in CI

- **Safari/Chrome gamut mapping** clips rather than perceptually reduces chroma. Keep baseline `C ≤ 0.25` for sRGB. Use `@media (color-gamut: p3)` to opt into richer chroma for P3 displays.
- **No `%` in `calc()` inside relative colors.** `oklch(from var(--accent) calc(l - 10%) c h)` is invalid. Use `calc(l - 0.1)`.
- **Warning in light mode is the soft spot** (~3.4:1 against `--bg`). Mitigation: use warning only as a background with dark text, or pair with `⚠` icon (WCAG 1.4.1 Use of Color). Never encode state by color alone.
- **Lint rule**: `stylelint-gamut` in CI catches out-of-sRGB accident.
- **Contrast verification**: every (fg, bg) pair goes through WebAIM before release. AAA for text-default on bg; AA for text-dim on bg; AA for accent on bg.

---

## 3. Typography

### 3.1 Choices

| Role | Pick | License | Why |
|---|---|---|---|
| **UI sans** | **Inter Variable** | SIL OFL 1.1 | Universal, tabular figures, zero licensing risk for OSS, metrics don't look dated in 3 years |
| **Mono (data)** | **JetBrains Mono Variable** | SIL OFL 1.1 | Purpose-built for code, unambiguous `0/O/1/l/I`, tabular by default, massive install base |

Both self-hosted as WOFF2 from `assets/fonts/`. No Google Fonts runtime. Subset to Latin + Latin Extended only → Inter drops from ~330KB to ~90KB.

**Rejected candidates** (all documented in `REFERENCE_GALLERY.md` → Typography picks):

- Geist Sans / Mono — too Vercel-branded; "looks like every dashboard built on Geist"
- IBM Plex — enterprise-IBM signal
- Berkeley Mono — paid ($75-200+), OSS contributors can't match style
- Söhne — paid + "looks like OpenAI" tell
- Monaspace — bold alternative with texture-healing, but mix-and-match is an unusual pattern kiroxy doesn't need to introduce

### 3.2 Type scale

Minor-third ratio (1.200), base `14px`. **Not fluid** for chrome — kiroxy is keyboard-first, fixed sizes beat clever sizes for muscle memory. Clamp only the welcome-screen hero.

```css
:root {
  --font-sans: "InterVariable", Inter, -apple-system, BlinkMacSystemFont, system-ui, sans-serif;
  --font-mono: "JetBrains Mono Variable", "JetBrains Mono", ui-monospace, "SF Mono", Menlo, monospace;

  /* Sizes — 1.200 ratio, base 14px */
  --type-11: 0.6875rem;  /* 11px — micro meta, keycap badges */
  --type-12: 0.75rem;    /* 12px — metadata, timestamps */
  --type-13: 0.8125rem;  /* 13px — compact body (Linear 510 weight pattern) */
  --type-14: 0.875rem;   /* 14px — default body */
  --type-16: 1rem;       /* 16px — h3 */
  --type-20: 1.25rem;    /* 20px — h2 */
  --type-24: 1.5rem;     /* 24px — h1 */
  --type-32: 2rem;       /* 32px — welcome hero (only) */

  /* Weights */
  --weight-regular: 400;
  --weight-solid:   510;  /* Linear body-text trick: slightly heavier than regular */
  --weight-medium:  560;  /* UI labels, buttons */
  --weight-semibold: 620;  /* headings */

  /* Line heights */
  --lh-tight:   1.20;   /* headings */
  --lh-snug:    1.40;   /* UI */
  --lh-normal:  1.55;   /* body prose */
  --lh-relaxed: 1.70;   /* empty-state prose */
}

/* Defaults */
html {
  font-family: var(--font-sans);
  font-size:   var(--type-14);
  line-height: var(--lh-snug);
  font-feature-settings: "cv05", "cv08", "cv11", "ss03"; /* Inter stylistic sets — "g" tell, round quotes */
  font-variant-numeric: tabular-nums;
}

code, pre, .mono {
  font-family: var(--font-mono);
  /* Disable ligatures in paths — `/`, `->`, `/=` rearrange incorrectly */
  font-feature-settings: "liga" 0, "calt" 0;
}
```

### 3.3 Mono as the data layer (Vercel Geist pattern)

**All non-prose data runs in mono.** Request IDs, timestamps, account IDs, token counts, endpoints, model names, file paths, breadcrumbs. Named type tokens:

- `.text-label-13` — UI labels (sans)
- `.text-label-13-mono` — IDs, timestamps (mono)
- `.text-label-12-mono` — table cell data (mono, compact)
- `.text-copy-14` — prose (sans)
- `.text-copy-13-mono` — inline code (mono)

### 3.4 Pitfalls

- Use `font-display: optional` or `swap` + preload Variable files. Otherwise FOUT ~300ms at first paint.
- Inter Variable: `font-feature-settings` tokens are cumulative — setting them at `:root` doesn't cascade to components unless explicitly re-applied. Test at every site.
- JetBrains Mono ligatures are great for code, awful for paths. **Rule: always disable ligatures in any element that displays file paths, URLs, or shell commands.**

---

## 4. Spacing, density, layout

### 4.1 Grid basis: 4px

Rhythms: 4, 8, 12, 16, 24, 32, 48, 64 (no 36, no 40 — stick to multiples of 4 throughout).

### 4.2 Density modes

Two modes, user-toggleable via `⌘⇧T`:

| Dimension | Comfortable (default) | Compact |
|---|---|---|
| Table row height | 36px | 28px |
| Table cell padding (horizontal) | 12px | 8px |
| Button height | 32px | 28px |
| Input height | 32px | 28px |
| Card padding | 16px | 12px |
| Sidebar item height | 32px | 28px |

Persisted in `localStorage`. Applied via `data-density="compact"` attribute on `<html>`.

### 4.3 Container queries (component-level responsive)

- `container-type: inline-size` on dashboards, cards, tables.
- Break at 600px inline-size: collapse secondary columns.
- Break at 800px: show right-side inspector drawer; below, drawer becomes bottom-sheet.
- Viewport media queries only for global chrome (sidebar collapse at `< 960px`).

### 4.4 Layout chassis

The kiroxy default layout is the **Linear "inverted L"** — not the homelab "sidebar + top-nav + content-pane" that every Tier E reference uses.

```
┌────────────────────────────────────────────────────────────┐
│ sidebar (240px collapsible to 56px icon rail)              │
│ ├── logo + version                                         │
│ ├── [search] / [command palette affordance]                │
│ ├── nav                                                    │
│ │   ├── Dashboard                                          │
│ │   ├── Accounts (N healthy / M cooldown / K disabled)     │  ← live counts
│ │   ├── Requests                                           │
│ │   ├── Routes                                             │
│ │   ├── Metrics                                            │
│ │   ├── Logs                                               │
│ │   └── Settings                                           │
│ └── footer                                                 │
│     └── token-refresh-in-flight indicator                  │
└────────────────────────────────────────────────────────────┘
┌────────────────────────────────────────────────────────────┐
│ content pane                                               │
│ ├── breadcrumb + actions (right)                           │  ← thin, no box
│ ├── [optional] tab strip                                   │
│ ├── content (table, cards, or form)                        │
│ └── [optional right drawer] ← slides in on row click       │
└────────────────────────────────────────────────────────────┘
```

No top-nav bar. Breadcrumb carries the orientation load (Render pattern). Horizontal top-nav was rejected because kiroxy has 7+ sections — Tailscale's top-nav only works with ~8 sections.

### 4.5 Content max-widths

- Dashboard content: no max (fills available).
- Prose (docs, empty states, tooltips): 72ch.
- Modal (dialog): 480px (simple), 640px (wizard), 800px (full-form).
- Right drawer: 400px default, 560px for request-inspector.

---

## 5. Motion

### 5.1 Easing and duration

One easing curve, four durations. **Linear's easing became universal in 2026 ops-tools**; we inherit it.

```css
:root {
  --ease: cubic-bezier(0.16, 1, 0.3, 1);

  --dur-instant:  0ms;    /* selection, row highlight */
  --dur-quick:    120ms;  /* hover, popover, tooltip */
  --dur-moderate: 200ms;  /* modal open/close, drawer */
  --dur-slow:     320ms;  /* cross-page view transition */
}
```

**`prefers-reduced-motion: reduce` collapses all durations to `0ms`.** Non-negotiable.

### 5.2 Patterns

- **View Transitions API (cross-document)** — ship `@view-transition { navigation: auto; }` for free crossfades on navigation. Two-line opt-in; fallback is no animation.
- **`@starting-style` for popovers, dialogs, drawers** — `transition-behavior: allow-discrete` + `@starting-style { opacity: 0; transform: translateY(-6px); }`. Zero JS for enter animations.
- **`:has([data-open])`** for parent-state styling. Kill `useState({ isOpen: false })` for CSS-only state reflection.
- **No decorative motion.** No `@keyframes pulse`, no `animate-float`, no `animate-stagger` stacks (the hexos mistake). Every animation must have a functional reason: "this row updated just now," "this dialog is opening," "this value is changing."

### 5.3 Specific interactions

| Interaction | Animation | Duration |
|---|---|---|
| Row hover | border color swap | instant |
| Row selected (click) | background shift to `--elevated` | 80ms |
| Row value updated (SSE) | `@property --row-flash` animates green border 0→1→0 | 600ms once |
| Dialog open | fade + translateY(-6px → 0) via `@starting-style` | 200ms |
| Drawer open | slideIn from right via `@starting-style` | 200ms |
| Popover (tooltip) open | fade | 120ms |
| Command palette open | fade + slight scale (0.98 → 1) | 120ms |
| Theme toggle | `document.startViewTransition()` | 320ms |

---

## 6. Iconography

**Lucide** at `strokeWidth={1.5}` with `absoluteStrokeWidth`. Override the default 2px — 1.5px reads correctly at 16px display, 2px is chunky.

- Sizes: 14px inline · 16px default · 20px buttons · 24px nav / empty states
- Color: always `currentColor` — never a fixed hue
- Inline SVG only — no icon fonts, no sprite sheets for core icons
- Hand-rolled exception: the **kiroxy logo mark** (small monospaced-letter "k" inside a rounded rectangle — keeps the CLI-tool feel) — maintained as a single inline SVG in `src/components/icons/Brand.tsx`

**Rule: one icon library, no exceptions.** Mixing Lucide with Heroicons or Radix Icons is detectable by eye within 3 seconds and reads as amateur.

Required icon inventory for v1.3 dashboard (all present in Lucide):

```
search, command, settings, plus, x, check, alert-circle, alert-triangle,
info, chevron-down, chevron-right, chevron-up, arrow-right, arrow-left,
copy, clipboard, external-link, download, upload, refresh-cw, key, lock,
unlock, user, users, activity, zap, server, database, terminal, moon,
sun, monitor, filter, eye, eye-off, more-horizontal, trash, edit,
check-circle, x-circle, pause, play, skip-forward, help-circle
```

---

## 7. Interaction patterns

### 7.1 Command palette (primary navigation)

Full spec in `research-v3/REFERENCE_GALLERY.md → Command palette and keyboard shortcut deep dive`. Summary:

- **Invocation**: `⌘K` / `Ctrl+K` (primary), `/` focuses scoped search, `?` opens shortcut sheet.
- **Layout**: centered modal, ~640px, top-anchored ~20% from viewport top.
- **Empty state is CURATED** — recents + 5 common actions (never empty).
- **Every row shows its direct shortcut as a keycap badge** (Linear/Superhuman teaching pattern).
- **Footer hints** show available modifiers on selected row.
- **Two-tier palette**: root for nav/actions; `⌘K` on selected row opens sub-palette for item-specific actions (Raycast pattern).
- **Scope sigils** (VS Code / GitHub): `/` accounts · `#` requests · `>` commands · `@` models · `?` help.

### 7.2 Full keyboard shortcut map (v1.3 target)

See `REFERENCE_GALLERY.md → kiroxy keyboard cheatsheet`. Canonical — do not diverge.

### 7.3 Form interaction

- **Inline validation** on blur + on submit. Not on every keystroke (anxiety-inducing).
- **Submit state** via button disabled + spinner inline. No full-page overlay.
- **Errors** appear directly below the field in `--danger` text with an `alert-circle` icon. Not in a toast.
- **Success** after async submit shown as a subtle green flash on the submit button + toast with optional "Undo" (Superhuman pattern).

### 7.4 Data table interaction

Pattern inheritance: Supabase table editor + Tailscale machines table.

- **Keyboard navigation**: `J/K` row down/up, `Enter` opens right drawer, `⌘Enter` opens full page.
- **Selection**: `X` toggles checkbox; `⌘A` selects all visible; click-drag range selection.
- **Sort**: `⌘↑/↓` on header sorts; click-toggles asc/desc; tri-state (asc → desc → unsorted).
- **Filter**: search input above table is a DSL (Tailscale pattern). Typeahead on the colon unlocks vocabulary.
- **Bulk actions**: top-bar transforms on selection — "N selected · refresh · disable · delete." Disappears on deselect.
- **Row updates** (live SSE) flash the updated cell with a 600ms green border via `@property`.

### 7.5 Search DSL grammar

```
<field>:<operator><value>   e.g. latency:>2s
<field>:<value>             e.g. status:429
is:<state>                  e.g. is:active, is:cooldown, is:disabled
has:<attribute>             e.g. has:error
<free-text>                 e.g. claude (matches model name or error message)
```

Combinable: `model:claude-sonnet status:429 latency:>2s user:alice`

Fields + operators documented in `/help → search syntax` (accessible via `?` from anywhere).

### 7.6 Modal vs drawer vs popover decision tree

| Kind | When | Dismiss |
|---|---|---|
| **Popover** (native `[popover]`) | hover-triggered info, tooltips, meta | Esc, click-outside, auto after 4s |
| **Right drawer** (400-560px from right) | row drill-down, inspect details without losing list context | Esc, click-outside, navigation |
| **Dialog** (native `<dialog>`) | confirmations, multi-step forms, destructive actions | Esc, `Cancel` button, explicit `Close` |
| **Full page route** | config, new-entity wizards, detailed drill-down with own URL | Browser back, breadcrumb |

**Default to drawer over dialog** for inspect operations. The list stays visible, the operator doesn't lose orientation.

### 7.7 Toast vs inline status decision tree

| Kind | Placement | When |
|---|---|---|
| **Inline status** (below form, in card header) | persistent until resolved | Validation errors, ongoing state (cooldown, refreshing) |
| **Toast** (bottom-right, auto-dismiss 4s) | transient success + optional Undo | "Account refreshed," "Request replayed" |
| **Banner** (top of content pane) | persistent until dismissed | Warnings that affect whole page (upstream outage, rate-limited) |

**No modal for errors.** If a user saw an error in a toast and missed it, the inline status has it too. Redundancy beats interruption.

---

## 8. Component primitives (the kiroxy component list)

Named after Vercel Geist's operator-centric vocabulary — not generic "Card/Badge/Button."

| Primitive | Description | Pattern inheritance |
|---|---|---|
| **`Entity`** | Avatar + two-line name/metadata stack + right-aligned timestamp | Vercel Geist |
| **`StatusDot`** | 8px colored circle with optional outline-ring for loading | Vercel Geist |
| **`StatusPill`** | Text + dot, for row status columns | Radix + hexos learned |
| **`Keycap`** | Styled `⌘K` visual token, monospace inside a bordered rounded rect | Raycast + Geist `Keyboard Input` |
| **`Snippet`** | Monospace code block with copy button | Geist |
| **`Gauge`** | Progress bar + numerical value + max | Geist |
| **`RelativeTimeCard`** | "2h 14m ago" with absolute-time tooltip on hover | Geist |
| **`Sparkline`** | 60-point mini-line, hand-rolled SVG, no library | hexos `UsageTrendChart` precedent |
| **`ContextCard`** | Stack of label/value pairs, for drill-down panels | Geist |
| **`EmptyState`** | Centered icon + prose + CTA — **CTA is a copyable CLI command**, not a button (fly.io pattern) | fly.io |
| **`LoadingDots`** | 3-dot animated, 120ms stagger | Geist |
| **`Skeleton`** | Per-component shimmer during load | Geist |
| **`CommandPalette`** | Two-tier, `⌘K`, `cmdk` library base | Linear, Raycast, Vercel |
| **`DataTable`** | Subgrid-based, keyboard-navigable, DSL-search | Supabase, Tailscale, atopile LogViewer |
| **`Drawer`** | Right slide-in, `<dialog>` under the hood for focus trap | Radix primitive |
| **`Dialog`** | Native `<dialog>` with `@starting-style` fade | Web platform |
| **`Popover`** | Native `[popover]` + anchor-positioning API | Web platform |
| **`Toast`** | Stacked bottom-right, auto-dismiss, optional Undo | Sonner (OSS toast lib) |
| **`DensityToggle`** | Comfortable / Compact | kiroxy specific |
| **`ThemeToggle`** | Light / Dark / Dark-Dimmed / System | GitHub Primer pattern |
| **`Keyboard`** | Keyboard shortcut cheatsheet modal (`?` opens) | Linear / Superhuman |

**Rules:**

- Every primitive has a `data-state` attribute for styling hooks (Radix pattern).
- Every primitive supports `asChild` composition (Radix pattern).
- Every primitive ships with Storybook-equivalent stories (we use Ladle — lighter than Storybook).
- No primitive is exported until it has keyboard-interaction tests (vitest + @testing-library).

---

## 9. Accessibility (WCAG 2.2 AA baseline)

- **Focus visible**: 2px outline in `--accent`, offset 2px. Use `:focus-visible` only (keyboard-triggered). High-contrast mode: 3px outline.
- **Hit targets**: minimum 32px (compact) / 36px (comfortable). Icon-only buttons: 32×32 minimum.
- **Contrast**: AAA for default text on background; AA minimum for secondary text and icons.
- **Announcements**: `aria-live="polite"` on SSE-updated status regions; `aria-live="assertive"` on error banners.
- **Motion**: `prefers-reduced-motion: reduce` disables all transitions and animations.
- **Color**: Never the only signal. Status = dot + text + color. Validation errors = icon + text + color.
- **Screen reader**: every icon-only button has `aria-label`. Keycap badges are `aria-hidden="true"` (screen reader hears the action name, not "command K").

---

## 10. CLI output design

Not just a dashboard thing — kiroxy's CLI gets the same treatment.

- **Color scheme respects `$NO_COLOR`**, `$FORCE_COLOR`, and `$TERM=dumb`.
- **Status indicators**: `●` (colored dot) for state — green/amber/red/gray matching dashboard.
- **Table output**: mono-aligned columns, tabular-nums style, via the `tabwriter` Go package.
- **Errors**: `✗` prefix in red; `✓` prefix in green for success.
- **Copy the CLI palette from the dashboard tokens** — when `$TERM` supports 24-bit color, emit OKLCH-equivalent hex values so `kiroxy status` looks like the dashboard it represents.

---

## 11. Documentation design (README, docs site)

Pattern inheritance: LibreChat (Tier E positive reference) — the one homelab tool that escaped the genre.

- **Dual-mode hero screenshot** (light + dark side-by-side) at the top of the README. Shows confidence in both themes.
- **One-paragraph "what kiroxy is"** before any feature list. Name what it *is*, then what it *does*.
- **No emoji-feature-list**. Feature inventories belong in the CHANGELOG.
- **Copy-paste commands**, always with the expected output inline. fly.io "CLI-first" aesthetic.
- **Empty-state copy**: dry, technical, self-aware. No "Oops! Something went wrong." No "Great choice!"
- **Error messages**: `{problem} - {cause} - {action}`. Example: `Upstream returned 403 — access token may be revoked — run 'kiroxy debug-refresh <account-id>'`.

---

## 12. Signature primitive — the LiveRequestStream

One mechanic that everything orbits (§1 tenet 6).

**What it is:** The dashboard's home page is not a stat grid. It's a **live request stream** — a reverse-chronological list of requests as they happen, updating via SSE, with each request rendered as a Block (Warp-inspired primitive) showing:

```
┌───────────────────────────────────────────────────────────────┐
│ ● acct-2  claude-sonnet-4-5  POST /v1/messages  200  1.4s     │
│    1,247 in · 389 out · $0.012 · stream · 11:42:18            │
│    ⌘↩ inspect · ⌘C copy ID · ⌘R replay · ⌘L view logs         │  ← hints on hover
└───────────────────────────────────────────────────────────────┘
```

- **Cmd+click to attach a request as context** to an inline action (replay with different model, diff against another request, escalate to debug view). Pattern: Warp's block-as-AI-context.
- **Permalinks**: every request has a shareable URL (`/requests/<request-id>`). Teammate debugging goes 10x faster.
- **Density adaptive**: comfortable mode shows the 3-line block; compact mode collapses to single-line with inline metadata.
- **`:has([data-selected])` on parent**: selecting a block highlights it without JS.
- **Subgrid** on the block list ensures timestamps align across rows.
- **`@starting-style`** animates each new block in (fade + 2px translateY).
- **`@property`** drives the 600ms green-border pulse when a block arrives.

Stat grids, charts, and settings are auxiliary pages. **The request stream is the product's visible surface.**

---

## 13. What kiroxy explicitly does NOT use

Per operator instruction and per Tier E anti-reference:

- Six-color stat row with pastel icon backgrounds (hexos pattern — the "AI slop" signal)
- Drop shadows for elevation (use borders + surface color, Supabase-style)
- Gradient backdrops on chrome (only on marketing art if any, and only intentional)
- Rounded-2xl + shadow-xl + backdrop-blur "glass" cards (the shadcn-dashboard template look)
- Default shadcn radius, default Tailwind colors, default Inter without stylistic sets
- Mackinac / Söhne / Berkeley Mono (paid or brand-identified fonts)
- Framer Motion, GSAP, anime.js, Motion One, Popmotion (all JS motion libs replaced by CSS native)
- Lucide/Heroicons/Feather imported as a bulk bundle (hand-pick, tree-shake)
- TanStack Query, SWR, Redux, Zustand, nanostores (SSE + Svelte stores is enough at this scale)
- Any spinner (`LoadingDots` + `Skeleton` only)
- Any "pastel SaaS palette" (Notion-pastel / Linear-pastel / Supabase-pastel — all specific hue commitments)
- jQuery, htmx (Phase H's territory; Dashboard Next is Svelte 5)

---

## 14. How new designs are added

1. **Read `REFERENCE_GALLERY.md` first.** If a pattern exists there, follow it.
2. **If a new primitive is needed**, add to §8 via PR. Include: Storybook story, keyboard-interaction test, screenshot diff.
3. **If a new token is needed**, add to §2. Include: WCAG contrast check against all background tokens.
4. **If a new motion pattern is needed**, it must justify itself against §1 tenet 4 ("motion serves function"). Functional reason gets documented in the PR.
5. **No new dependencies without a paragraph justification** in the PR description. Current allowlist: `cmdk`, `sonner` (toast), Svelte 5, Vite 6. Adding to that list requires operator review.

---

## 15. Versioning this document

- v1.0 — 2026-05-13 — Initial draft from `research-v3/REFERENCE_GALLERY.md`.
- (future) v1.1 — after v1.3 dashboard ships; incorporate lessons from live deployment.

**Changes to this document are first-class PRs.** A commit message of `design: rename --elevated → --surface-hover` belongs in the kiroxy repo, not a sketchbook. The design system is code.
