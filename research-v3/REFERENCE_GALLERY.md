# Reference Gallery — kiroxy v3 research

> A dossier of 30-50 dev tools, operator dashboards, design systems, and web-platform features studied for their visual, interaction, and aesthetic signatures. Each entry is a verbal screenshot-in-words: color palette, typography, density, motion, and a ranked take on what is worth borrowing and what is not.
>
> Compiled overnight 2026-05-13. Sources are cited inline per entry. No images committed — verbal description only.
>
> **Companion documents:**
> - `docs/VISION.md` — what kiroxy is, who it's for, anti-goals
> - `docs/DESIGN_SYSTEM.md` — tokens, typography, motion grounded in this gallery
> - `docs/ROADMAP.md` — where kiroxy goes from v1.x to v2.0

---

## How to read this gallery

Every entry uses one of these tiers:

| Tier | Meaning | Entries |
|---|---|---|
| A | Operator / developer-infra dashboards — closest match to kiroxy's genre | 11 targets |
| B | Developer tools with signature UI — power-user aesthetics worth studying | 9 targets |
| C | Design systems and component libraries — architectural primitives | 6 targets |
| D | Bleeding-edge 2026 web tech — features to use or intentionally avoid | 15 topics |
| E | Anti-reference — hobbyist / homelab UIs that kiroxy must not become | 7 targets |

Per-entry template (fields may be "unknown — why" when unverifiable):

```
### Name — Tier — Sub-category
URL, what it is, visual signature (3-4 sentences), extracted color, typography,
density, motion, key borrowable decisions, explicit NOT-to-borrow, source URLs.
```

## Category index

Use this index to jump by design dimension rather than tier.

- **Typography exemplars:** Linear, Vercel, Geist, Stripe, Warp, Notion, Monaspace repo
- **Color / dark-first operators:** fly.io, Supabase, Vercel, Railway, Zed, Raycast, Warp
- **Keyboard-first / command palette:** Linear, Raycast, Superhuman, VSCode, Vercel, Notion, Figma, Arc, Zed, Warp, cmdk lib
- **Information density (data tables + dense ops):** Grafana, Stripe Dashboard, Netlify, Cloudflare, PlanetScale, Tailscale
- **Information hierarchy (non-tables):** Notion, Figma, Tailscale, Replit
- **Motion / premium feel:** Linear, Arc, Superhuman, Vercel, Apple
- **Anti-patterns (so we don't look like these):** *arr stack, Homepage dashboard, Portainer, Jellyfin admin, Open WebUI, LibreChat
- **Component architecture references:** Radix UI, Ark UI, shadcn/ui, Primer, Geist, Tailwind UI
- **Modern web-platform features:** View Transitions, @scope, @container, :has, OKLCH+color-mix+light-dark, P3, anchor-positioning, @starting-style, @property, subgrid, container style queries, Svelte 5 / Solid / Qwik patterns, mini-apps aesthetic, utopian type, icon trend 2026

---

## Tier A — Operator / Developer-Infra Dashboards

> These are the closest genre match to kiroxy. Study deeply.

**Overall takeaway from the Tier A group:** The five references converge on a shared operator-tool grammar even though their visual personalities diverge wildly. All five use a token-based color system with three to four background "elevation" layers (canvas → panel → elevated → overlay) rather than drop shadows, because shadows read as muddy in dark mode. All five reserve chromatic color almost exclusively for semantics (status dots, accents, destructive actions) and keep 90%+ of the surface in neutral grays with a faint color cast (Grafana cools its grays toward blue via `rgba(204,204,220,...)`; Supabase keeps them pure neutral; Linear's LCH generation ensures perceptual uniformity). Typography-wise, Inter and its derivatives (Geist Sans, Inter Display) dominate — always paired with a mono for data, always with tabular numerals for aligned columns. Density is comfortable, not dense: row heights around 28–32px, sidebar widths 220–280px, max content widths 1200–1280px. The four borrowable primitives that repeat everywhere are: command palette (⌘K), status dot, sparkline/mini-chart panel, and a sidebar with collapsible sections. The anti-pattern they all avoid is chromatic gradients on chrome — gradients show up only in marketing art, brand moments, or intentional product flourishes (Grafana's orange selection, Linear's hero).

### Grafana — Tier A — Operator Dashboard

**URL:** https://grafana.com/grafana/ · live demo: https://play.grafana.org/

**What it is:** Open-source observability dashboard builder; the canonical "panel grid of time-series over a dark canvas" operator UI.

**Visual signature:** Grafana's dark theme doesn't use pure neutrals — it uses a cool off-white text color `rgb(204, 204, 220)` (a blue-tinted gray) against near-black layered backgrounds, giving the whole UI a faintly cyan cast that reads as "terminal on a CRT" rather than "bright chat app." The signature element is the panel grid: draggable rectangles with thin 1px borders separating sparse titles, one big visualization, and a muted footer of units/timestamps — no cards-with-shadows, just borders. Orange is used sparingly but loudly as the brand accent (selected menu items, brand gradient `#F55F3E → #FF8833`), which contrasts beautifully against the blue-tinted grays. Time is a first-class citizen — every panel has an implied time axis and the global time-range picker lives in the top-right corner.

**Color (extracted):**
- Canvas (darkest): `#0b0c0e` (palette.gray05)
- Background primary (panels): `#141619` (palette.gray10)
- Background secondary: `#202226` (palette.gray15)
- Primary accent (info/links): `#5794f2` (blue95)
- Text primary: `rgb(204, 204, 220)` — signature off-white
- Text secondary: `rgba(204, 204, 220, 0.65)`
- Border weak: `rgba(204, 204, 220, 0.12)`
- Semantic: success `#1a7f4b`, error `#d10e5c`, warning `#ff9900`, yellow `#ecbb13`
- Brand gradient: `linear-gradient(270deg, #F55F3E 0%, #FF8833 100%)` (marketing/selection only)

**Typography:**
- UI sans: Inter (system fallback in Grafana OSS); default 14px, secondary text at 0.65 opacity
- Mono/data: Roboto Mono (query editors, PromQL, log lines)

**Density / layout:** Comfortable-dense. 24-column fluid panel grid; panel titles ~14px; axis labels ~11–12px. 4px spacing base with 8/16px rhythm. Sidebar ~56px collapsed / ~260px expanded.

**Motion:** Minimal. Panel hover elevates border weakly; dropdowns/popovers use `0px 8px 24px rgb(1, 4, 9)` shadow — a near-black shadow darker than the background, which genuinely works in dark mode. Hover opacity factor 0.08; tonal offset 0.15.

**Key borrowable decisions:**
- **Four-layer background model** (`canvas → primary → secondary → elevated`). A principled depth system that never needs shadows. Steal this naming verbatim.
- **Off-white text with a blue cast** (`204, 204, 220`) instead of pure white. Reduces eye strain in long ops sessions; gives dark theme a signature tone.
- **Warm accent over cool neutrals**. Creates unmistakable anchors for "active/selected" without polluting the rest of the UI with saturated color.
- **Dark-mode shadow darker than the surface** (`rgb(1, 4, 9)`). Most dark themes fail at this; Grafana nails it.

**Explicit NOT-to-borrow:**
- The 24-column draggable panel grid. Kiroxy is not a dashboard builder.
- The orange brand gradient as load-bearing UI color. Too Grafana-specific.
- Plugin-gallery density on settings screens. Kiroxy has one subscription, not hundreds of integrations.

**Source URLs:**
- https://github.com/grafana/grafana/blob/main/packages/grafana-data/src/themes/createColors.ts
- https://github.com/grafana/grafana/blob/main/public/sass/_variables.dark.generated.scss
- https://grafana.com/grafana/ · https://play.grafana.org/

---

### Linear — Tier A — Operator Dashboard

**URL:** https://linear.app

**What it is:** Keyboard-first product-development tracker; the canonical "quiet, fast, dense-but-airy" modern app aesthetic.

**Visual signature:** Linear reads as "an expensive blank page that moves." Canvas is near-black (`#080808`–`#0e0e10`), not Apple-system dark gray, and almost every surface shares the same color — hierarchy comes from 1px borders at `rgba(255,255,255,~0.06)` and from type weight, not from fills. Text is `#e2e2e2`, never pure white. The famous "inverted L" chrome (≈240px left sidebar with nested collapsible sections plus a thin tab bar along the top of the content pane) is the single most-copied layout in operator tooling. Issue IDs like `ENG-2703` render in slightly-smaller mono; status is a tiny bespoke SVG glyph (circle-with-tick, dashed donut, half-circle), not Material icons. Motion is fast and short (~100ms). The command palette (`⌘K`) is so central it effectively replaces menus.

**Color (extracted):**
- Canvas: `#080808`
- Surface/panel: `#141414`
- Text primary: `#e2e2e2` (intentionally not `#fff`)
- Text secondary/muted: `#878787`
- Primary brand (desaturated indigo): `#5e6ad2` — buttons, focus rings, selection
- Light mode page: `#f7f8f8`; panels `#f3f4f5`; border `#e6e6e6`
- **Color system uses LCH** (not HSL) generated from 3 inputs: base color, accent color, contrast — yielding ~98 tokens

**Typography:**
- UI sans body: **Inter Variable**, 14–15px, mixed 400/510/590 weights (the 510 weight on body text is an unusual Linear choice — slightly heavier than regular)
- UI sans display: **Inter Display** (opsz 32) for larger headings, added in 2024 redesign
- Scale (observed): Display 48px/510/-1.056px tracking · H1 32/400 · H2 24/400 · H3 20/590 · Body 15-16px · Caption 14/510/-0.182px
- Mono: issue IDs, shortcuts, code blocks — likely Berkeley Mono or system mono

**Density / layout:** Dense but airy. 32px row heights on issue lists; 8px row padding; sidebar 240–260px. 4px grid basis with dominant 8px rhythm. Split panes (issue list + detail) with persistent right-side properties panel.

**Motion:** ~100–150ms ease-out. Keyboard-triggered actions feel instant. Command palette slides up with slight overshoot. Hover states mostly do nothing — no fills, just cursor change. Click is where state lives.

**Key borrowable decisions:**
- **LCH-based theme generation from 3 inputs** (base, accent, contrast). Perceptual uniformity means no "muddy" colors when theming. Linear's [design blog post](https://linear.app/blog/how-we-redesigned-the-linear-ui) explicitly contrasts LCH with HSL.
- **Inverted-L chrome layout**: sidebar + thin top tab bar + main pane + optional right properties panel. The operator-tool layout of the 2020s.
- **Command palette as primary nav**. `⌘K` opens a fuzzy-matched action launcher that replaces menus.
- **Status as bespoke 12px SVG donut glyph** (empty → partial → full), not a colored pill. Communicates state more densely than a word + color.
- **510 weight as body text default** (not 400). Subtle but makes dark-background text feel more solid.

**Explicit NOT-to-borrow:**
- Marketing hero animations (scrolling agent-issue mockups). Template-y on a single-user tool.
- In-app triage rituals (cycles, projects, initiatives). Issue-tracker-specific metaphors.
- Growing AI-agent UI in the activity feed. Doesn't fit an ops tool.

**Source URLs:**
- https://linear.app/blog/how-we-redesigned-the-linear-ui
- https://linear.app/brand
- https://colorpickercode.com/color-palette/dark-mode-palettes/linear-dark/
- https://design.hagicode.com/previews/linear.app/light.html

---

### fly.io — Tier A — Operator Dashboard

**URL:** https://fly.io/dashboard (auth-walled) · public: https://fly.io/ · blog: https://fly.io/blog/

**What it is:** Developer cloud platform with a deliberately CLI-first philosophy; dashboard is second-class, marketing is first-class and weird.

**Visual signature:** fly.io is the one reference here whose website looks nothing like its dashboard — and that's the point. Public marketing uses a whimsical "graphic-novel" aesthetic with hand-drawn illustrations (hot-air balloons, crabs, robot chefs by Annie Ruygt), warm off-white backgrounds, the Mackinac serif for headings, Fricolage Grotesque for body. The login screen contains user-submitted haikus. The dashboard itself is restrained: near-black canvas, sidebar of apps, a world map showing machine deployments as colored dots, and pronounced CLI-first ethos — empty states literally say `fly launch` rather than showing a big button. Brand palette is unusual for infra: purple + hot pink + neon green, used sparingly in the app but loudly in marketing.

**Color (extracted):**
- Canvas (dashboard dark): `#1A1A2E` estimated — deep navy rather than neutral black (per DesignMD analysis; dashboard behind auth so unverified)
- Primary brand purple: `#7C3AED`
- Secondary accent hot pink: `#F0047F`
- Success/running neon green: `#00FF85`
- Marketing background: warm off-white ~`#fbf9f4`
- **Exact dashboard CSS variables: unknown — fly.io does not publish a public design system**

**Typography:**
- Marketing display: **Mackinac** (quirky geometric serif)
- Marketing body: **Fricolage Grotesque** (variable weights, positive letter spacing)
- Dashboard: **Inter** at 14px (third-party observation, not documented)
- Mono: heavy in CLI output blocks; likely system mono stack

**Density / layout:** Marketing is spacious (1200px max-width, 96px+ section padding, print-like). Dashboard has 220px sidebar; machine list is tabular with 8px row padding; world map occupies full-width 300px-tall panel (signature visual). Blog posts use ~680–720px narrow column and are often 2000+ words.

**Motion:** Minimal in the product. Marketing pages have subtle scroll-triggered fades; primary motion is the world map's pulsing region dots.

**Key borrowable decisions:**
- **CLI-first empty states**. Don't put "Create New App" as a big button. Put `fly launch` as copyable code. Massive vibe-setter for kiroxy — empty states show `kiroxy add-account --refresh-token=...` rather than a primary-button-with-gradient.
- **Personality-first copy**. Error messages and empty states with a consistent voice (dry, technical, self-aware), not generic "Something went wrong."
- **Single ambient signature visual** (world map for fly.io; for kiroxy, a token-throughput sparkline that never leaves the screen).
- **A blog/docs surface that doubles as technical receipts** — long, smart, first-person posts over SaaS marketing-speak.

**Explicit NOT-to-borrow:**
- Mackinac + hand-drawn illustrations. fly.io brand equity; cosplay elsewhere.
- Haikus on login. Charming for fly, precious elsewhere.
- Purple/pink/green palette. Signals fly too strongly.
- The visual split between marketing and dashboard. kiroxy is small enough that landing and dashboard should share one visual language.

**Source URLs:**
- https://fly.io/ · https://fly.io/blog · https://fly.io/dashboard
- https://fly.io/blog/command-lines-flyctl-and-fly/
- https://www.designmd.co/d/fly (third-party analysis; unverified against live dashboard)
- https://styles.refero.design/style/0c77bb2a-c7cd-499b-b5cd-90268eefe906 (Mackinac + Fricolage Grotesque confirmed from live CSS)

---

### Vercel (Geist Design System) — Tier A — Operator Dashboard

**URL:** https://vercel.com/dashboard · design system: https://vercel.com/geist

**What it is:** Deployment platform whose dashboard is the best-documented public operator-console design system on the web.

**Visual signature:** Vercel's Geist aesthetic is "black and white and mono." Dashboard is near-black (Background 1 is essentially pure neutral, no color cast), borders are ~1px light neutrals, text is near-pure-white. The only chromatic colors appear in status indicators, destructive buttons (red), and linked text (blue). What makes it distinctive is how much it relies on the mono typeface for non-code data: deployment IDs, commit SHAs, timestamps, branch names — anything not prose runs in Geist Mono. Status is a tiny colored dot next to a label, never a pill. The `Entity` and `Relative Time Card` components (yes, they have named components for these) are the atomic units of deployment lists. It looks like `ls -la` with taste.

**Color (extracted):**
- Background 1 (default): near-black dark / pure white light
- Background 2 (secondary): subtly offset from Background 1
- Colors 1–3: component backgrounds (default / hover / active)
- Colors 4–6: borders (default / hover / active)
- Colors 7–8: high-contrast backgrounds (for emphasis bands)
- Color 9: secondary text/icons · Color 10: primary text/icons
- 10 color scales: Gray, Gray-alpha, Blue, Red, Amber, Green, Teal, Purple, Pink (+ Backgrounds)
- **P3 color space support on compatible displays** (enumerated in site but hex values only exposed via right-click-copy)

**Typography:**
- UI sans: **Geist Sans** (custom-designed by Vercel for developer UIs)
- Mono/data: **Geist Mono**
- Explicit type scale (Tailwind classes): `text-heading-72/64/…/14`, `text-button-16/14/12`, `text-label-20/18/16/14/13/12` + `-mono` variants at 14/13/12, `text-copy-24/20/…/13` + `text-copy-13-mono`
- Subtle/Strong modifiers via `<strong>` nesting
- **Tabular numerals** as a first-class variant (`Label 13 Tabular`)

**Density / layout:** Comfortable. Dashboard rows ~44–48px. `Entity` component = avatar + two-line stack + right-aligned timestamp. 4px spacing base; 8/12/16/24 rhythm. Sidebar ~240px.

**Motion:** Restrained. `Skeleton` during loads; `Loading Dots` for smaller indicators. Toast/drawer ease-out ~200ms. No "wow" motion — motion is "you didn't notice anything just happened."

**Key borrowable decisions:**
- **Two-background + 10-step semantic color-scale architecture** (`Background 1/2` + `Color 1–10`). Cleaner mental model than numeric `gray-50/.../900`. Color 1/2/3 = default/hover/active, 4/5/6 = borders at those states, 7/8 = emphasis, 9/10 = text. Adopt this exact scaffold.
- **Mono for all non-prose data** — deployment IDs, timestamps, commit hashes, file paths, breadcrumbs. For kiroxy: request IDs, token counts, timestamps, endpoints all go mono.
- **Tabular numeral variant as a typography token** — `Label 13 Tabular`. 10 API requests with token counts should align on the right.
- **Named primitives for operator UIs**: `Entity`, `Status Dot`, `Gauge`, `Relative Time Card`, `Context Card`, `Loading Dots`, `Empty State`. Use these as the kiroxy component list, not generic "Card/Badge/Button."
- **Keyboard Input** and **Snippet** as first-class components — shortcuts render as styled `⌘ K` visual tokens, not just text.

**Explicit NOT-to-borrow:**
- Full 10-scale chromatic system. Kiroxy doesn't need Purple or Pink; strip to Gray + Blue + Green + Amber + Red. Extra scales = maintenance burden without payoff for a single-user tool.
- Geist Sans/Mono themselves if you want to avoid Vercel-adjacent brand recognition. Inter + JetBrains Mono gets 90% of the effect without the visual-fork vibe.
- Marketing-scale type (`text-heading-72`). A single-user self-hosted tool shouldn't have 72px heroes.

**Source URLs:**
- https://vercel.com/geist · https://vercel.com/geist/introduction
- https://vercel.com/geist/colors · https://vercel.com/geist/typography
- https://vercel.com/

---

### Supabase Dashboard — Tier A — Operator Dashboard

**URL:** https://supabase.com/dashboard · design system: https://supabase.com/design-system

**What it is:** Postgres-centric backend platform whose dashboard is the single closest spiritual match to kiroxy — dev tool, data-heavy, self-hostable, dark-first, table-UX-centric.

**Visual signature:** Supabase's dashboard feels like somebody built pgAdmin with taste and shipped it in 2024. Canvas is near-black (`#171717`), surfaces are `#242424`, and critically there are **no drop shadows anywhere**. Elevation is exclusively a border-weight story: `#2e2e2e` for default surfaces, `#363636` for hover/interactive, with a Supabase-green `rgba(62,207,142,0.3)` border at 30% opacity as the topmost elevation signal for brand moments. The signature is the left sidebar organized by product area (Table Editor, SQL Editor, Auth, Storage, Edge Functions) with a nested inner side-menu inside each product — a two-column sidebar that is unusual and distinctive. The Table Editor is the crown jewel: Airtable-style direct editing on Postgres rows, in-place cell editing, keyboard row navigation, right-side row-detail panel.

**Color (extracted):**
- Canvas (dashboard dark): `#0f0f0f` or `#171717`
- Background surface 100 (panels): `#242424`
- Background surface 200 (overlap): lighter (exact hex not inlined)
- Border default: `#2e2e2e` · hover: `#363636` · strong: `#434343`
- Text muted: `#898989` · subtle: `#b4b4b4`
- Brand (Supabase Green): `#3ecf8e` · interactive: `#00c573` · brand-accent-30% border: `rgba(62,207,142,0.3)`
- Light mode: `#fafafa` canvas, `#efefef` borders, `#f7f8f8` alternates
- Tailwind semantic tokens: `background`, `foreground`, `border`, `brand`, `warning`, `destructive`

**Typography:**
- UI sans: **Inter** at 13–14px for dense dashboard; custom Circular-style display font for marketing
- Mono/data: **JetBrains Mono** or similar for SQL editor, code samples, request IDs, env vars
- Tabular numerals in table cells

**Density / layout:** Dense. Table rows ~32px, cells 28px tall with 8px horizontal padding — Airtable-style. Two-column sidebar: outer product rail (~56–64px with icons) + inner menu (~200px) — total ~260px left chrome. 4px grid base; 8/12/16 rhythm.

**Motion:** Restrained. Cell focus ~100ms. Realtime presence avatars fade in/out on collaborative editing.

**Key borrowable decisions:**
- **No shadows, only borders — layered by border weight**: `border-default → border-hover → brand-accent-30%-opacity`. Three-level elevation all expressed with 1px borders. The cleanest dark-mode depth system of the five references. Copy exactly.
- **Tailwind semantic token naming** (`background-surface-100/200/300`, `border-default/strong/stronger`, `brand`, `destructive`, `warning`) instead of numeric scales (`gray-50/100/.../900`). Maps to what the UI *means*, not to color.
- **Shorthand utilities** like `text-muted`, `bg-surface`. Reduces class noise.
- **Brand-color-at-30%-opacity-as-border** as highest-elevation signal. For kiroxy: "currently active" or "ready" indicator gets a `kiroxy-accent/30` border rather than a filled background.
- **Direct-manipulation table editor** as primary UI pattern for rows of data. Request-log viewer should steal this: keyboard nav, cell-level focus, right-panel row detail.
- **Three dark themes**: Light / Classic Dark / Deep Dark. Pure-black OLED-friendly variant is a nice niche without complicating the default.

**Explicit NOT-to-borrow:**
- Supabase-green brand color `#3ecf8e` itself. Identifiable as Stripe's `#635bff` — kiroxy needs its own accent. Pick one with similar *role* (single punchy semantic) but different *hue*.
- Two-column sidebar (product rail + inner menu). Kiroxy has one product area; a flat single-column sidebar is correct. Adopting two-column creates empty chrome.
- The chat-assistant panel and shipping AI UI patterns — Supabase's ongoing product bet, not a timeless pattern.
- "Build in a weekend, scale to millions" marketing tone. Kiroxy is deliberately single-user.

**Source URLs:**
- https://supabase.com/design-system/docs/theming
- https://supabase.com/design-system/docs/tailwind-classes
- https://github.com/supabase/supabase/pull/42649
- https://github.com/supabase/supabase/pull/45214
- https://supabase.com/blog/supabase-ui-library
- https://getdesign.md/design-md/supabase/preview

---

## Tier B — Developer Tools with Signature UI

> Power-user aesthetics, keyboard-first interactions, command palettes, distinctive motion.

<!-- Populated by Tier B librarian subagents. -->

_TBD — pending research._

---

## Tier C — Design Systems

> Primitives, tokens, theming, docs patterns. Borrow architecture, reject aesthetic clichés.

<!-- Populated by Tier C librarian subagent. -->

_TBD — pending research._

---

## Tier D — Bleeding-Edge 2026 Web Tech

> Platform features to adopt, avoid, or use carefully. Grounded in real production examples.

<!-- Populated by Tier D librarian subagent. -->

_TBD — pending research._

---

## Tier E — Anti-reference: Homelab / Hobbyist UIs

> kiroxy must not look like another *arr-stack tool. These are what we're transcending.

<!-- Populated by anti-reference librarian subagent. -->

_TBD — pending research._

---

## Appendix — Topical deep dives

### Command palette and keyboard shortcut deep dive

_TBD — pending librarian research on specific invocation keys, palette layouts, keyboard maps across 10+ tools._

### Typography picks 2026 — candidates, evidence, recommendation

**Candidates evaluated:**

| Pick | License | Notable users (verified) | Verdict for kiroxy |
|---|---|---|---|
| **Inter Variable** | SIL OFL 1.1 | Figma, Supabase, countless YC dashboards; rsms.me/inter | ✅ **Primary UI sans recommendation** |
| **Geist Sans / Mono** | SIL OFL 1.1 | vercel.com, v0.dev, nextjs.org, turbo.build | Strong branded alt — but signals "looks like Vercel" |
| **IBM Plex** | SIL OFL | IBM's [Carbon DS](https://carbondesignsystem.com/), ibm.com | Skip — "enterprise" feel |
| **JetBrains Mono Variable** | SIL OFL | JetBrains IDEs, Hugging Face, countless OSS dashboards | ✅ **Primary mono recommendation** |
| **Berkeley Mono (TX-02)** | **Paid** ($75–200+ via berkeleygraphics.com) | Perplexity (Phi Hoang), Cartesia (Kabir Goel), Shopify CEO personally, Axiom.co (Dan Newman), SerenityOS (Andreas Kling) | Skip for OSS — contributors can't match style without buying |
| **Commit Mono** | SIL OFL | — | Strictly worse than JBM for kiroxy's use case |
| **Monaspace** (Neon/Argon/Xenon/Radon/Krypton) | SIL OFL | GitHub Copilot inline, Obsidian/VS Code community | Bold alternative — texture-healing is real, but mix-and-match is unusual |
| **Söhne** (Klim) | **Paid** (~$50/weight) | OpenAI product, anthropic.com, NYT, The New Yorker | Skip — paid + "looks like OpenAI" tell |
| **Pretendard / Recursive** | OFL | — | Skip — pan-Unicode / morphing-axis showpieces, not kiroxy's need |

**🎯 Typography recommendation:**

| Role | Pick | Why |
|------|------|-----|
| **UI sans** | **Inter Variable** | OFL, universal, tabular figures via `font-variant-numeric: tabular-nums`, zero licensing risk for OSS, metrics don't look dated in 3 years |
| **Mono (data + code)** | **JetBrains Mono Variable** | OFL, purpose-built for code legibility, unambiguous `0/O/1/l/I`, tabular by default, massive install base |

```css
:root {
  --font-sans: "InterVariable", Inter, -apple-system, BlinkMacSystemFont, system-ui, sans-serif;
  --font-mono: "JetBrains Mono Variable", "JetBrains Mono", ui-monospace, "SF Mono", Menlo, monospace;
  font-feature-settings: "cv05", "cv08", "cv11", "ss03";
  font-variant-numeric: tabular-nums;
}
code, pre, .mono { font-feature-settings: "liga" 0, "calt" 0; } /* kill ligatures in paths */
```

**Alternative branded combo:** Geist Sans + Geist Mono if you want a distinct "modern web ops" aesthetic — both OFL, metric-compatible, work out of the box with `next/font`. Use this if you'd rather not "look like every Inter dashboard."

**Pitfalls:**
- Use `font-display: optional` or `swap` + preload the Variable files; otherwise first paint shows ~300ms FOUT.
- Inter Variable is ~330KB WOFF2; subset to Latin + Latin Ext only → ~90KB.
- JetBrains Mono Variable: `->`, `/=` ligatures cause mid-word rearrangement. **Disable for path columns**: `font-feature-settings: "liga" 0, "calt" 0;`.

### Color system 2026 — OKLCH adoption, neutral ramps, accent strategy

**OKLCH has won for new design systems.** Evidence:

1. **Tailwind CSS v4.0** (shipped Jan 22, 2025) — "upgraded the entire default color palette from `rgb` to `oklch`" ([blog](https://tailwindcss.com/blog/tailwindcss-v4#modernized-p3-color-palette); verified [theme.css](https://github.com/tailwindlabs/tailwindcss/blob/main/packages/tailwindcss/theme.css))
2. **Vercel Geist** — Ships P3 colors on supported displays, 10 color scales ([vercel.com/geist/colors](https://vercel.com/geist/colors))
3. **Radix Colors** — 12-step scale with semantic use per step ([docs](https://www.radix-ui.com/colors/docs/palette-composition/understanding-the-scale))
4. **Browser support** — Full in Chrome, Safari, Firefox per [caniuse](https://caniuse.com/mdn-css_types_color_oklch). Known caveat: Safari/Chrome gamut mapping uses naive channel clipping, not OKLCH chroma reduction. Mitigation: keep baseline `C ≤ 0.25`.

**Neutral ramp depth:** 7 semantic steps for kiroxy. Defense: You're not building a consumer SDK. 7 steps cover background → surface → elevated → border → dim-text → default-text → bright-text with no redundancy. More stops invite "should this be step-3 or step-4?" paralysis.

**Accent strategy:** Single primary + 4 semantics. Dual-accent is a marketing-brand tool, not an ops-tool pattern — it fights with status dots.

**🎯 Color recommendation — concrete OKLCH tokens:**

**Dark mode (primary, 7-step neutral, cool undertone hue 285):**

```css
:root[data-theme="dark"] {
  /* Neutrals (derived from Tailwind v4 zinc at hue 285) */
  --bg:           oklch(0.145 0.005 285);  /* canvas */
  --surface:      oklch(0.205 0.006 285);  /* card, table rows */
  --elevated:     oklch(0.265 0.007 285);  /* hover row, popover, modal */
  --border:       oklch(0.340 0.008 285);  /* default border */
  --text-dim:     oklch(0.660 0.015 285);  /* secondary, timestamps */
  --text-default: oklch(0.830 0.012 285);  /* body */
  --text-bright:  oklch(0.970 0.003 285);  /* headings, emphasis */

  /* Primary accent — cyan-teal, reads "network/proxy" (not Vercel blue, not Supabase green) */
  --accent:         oklch(0.720 0.130 200);
  --accent-hover:   oklch(0.780 0.130 200);
  --accent-pressed: oklch(0.660 0.130 200);

  /* Semantic — tuned against --bg for perceptual contrast ≥ 0.5 ΔL */
  --success: oklch(0.720 0.180 145);  /* green */
  --warning: oklch(0.800 0.165  85);  /* amber */
  --danger:  oklch(0.680 0.220  25);  /* red */
  --info:    oklch(0.720 0.130 240);  /* blue */
}
```

**Light mode (5-step neutral):**

```css
:root[data-theme="light"] {
  --bg:           oklch(0.995 0     0  );
  --surface:      oklch(0.975 0.002 285);
  --border:       oklch(0.895 0.005 285);
  --text-dim:     oklch(0.500 0.015 285);
  --text-default: oklch(0.180 0.010 285);

  --accent:  oklch(0.500 0.155 200);
  --success: oklch(0.500 0.155 145);
  --warning: oklch(0.540 0.170  60);  /* see pitfall */
  --danger:  oklch(0.520 0.220  25);
  --info:    oklch(0.500 0.180 240);
}
```

**P3 wide-gamut progressive enhancement:**

```css
@media (color-gamut: p3) {
  :root[data-theme="dark"] {
    --accent:  oklch(0.720 0.170 200);
    --success: oklch(0.720 0.220 145);
    --danger:  oklch(0.680 0.270  25);
  }
}
```

**Contrast verification (WCAG 2.2 approximation):**

| Pair | Approx ratio | Status |
|---|---|---|
| `--text-default` on `--bg` (dark) | ~11:1 | AAA |
| `--text-dim` on `--bg` (dark) | ~5.8:1 | AA |
| `--accent` on `--bg` (dark) | ~6.3:1 | AA normal, AAA large |
| `--warning` on `--bg` (light) | ~3.4:1 | ⚠ AA Large only — pair with icon |
| `--danger` on `--bg` (light) | ~5.1:1 | AA |
| `--text-default` on `--bg` (light) | ~14:1 | AAA |

> Warning in light mode is the soft spot. Mitigation: use warning only as a background with dark text, or always pair with `⚠` icon so the indicator isn't carried by color alone (WCAG 1.4.1 Use of Color).

**Pitfalls to flag in DESIGN_SYSTEM.md:**
- Safari/Chrome gamut-map clips rather than perceptually reduces chroma. Keep baseline `C ≤ 0.25` for sRGB; use `@media (color-gamut: p3)` for richer chroma.
- No `%` in `calc()` inside relative colors: `oklch(from var(--accent) calc(l - 10%) c h)` is invalid. Use `calc(l - 0.1)`.
- **Don't ship HSL anywhere as a fallback.** Doubles token surface and re-introduces perceptual-lightness bug. Browser support is universal in 2026.
- Don't rely on ΔL alone for contrast compliance. Final palette through [WebAIM](https://webaim.org/resources/contrastchecker/) before merging.

---

## Changelog

- `2026-05-13` — Scaffold created; 9 parallel librarian subagents firing for Tier A/B/C/D/E + command-palette + typography + color deep dives.
