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

### Stripe Dashboard — Tier A — Operator console / payments

**URL:** https://dashboard.stripe.com

**What it is:** Stripe's logged-in admin where businesses run transactions, customers, billing, Workbench.

**Visual signature:** Near-white canvas with a slim left sidebar split into primary resources (Home, Balances, Transactions, Customers, Product catalog) and a "Shortcuts" zone for pinned/recent pages. Content area uses flat cards, quiet dividers, and lots of tabular data rendered with the Sail design system. Type is Stripe's custom sans at ~14px body, with distinctive indigo-purple `#635BFF` ("Blurple") reserved for links and primary actions ONLY. **"Spreadsheet with taste."**

**Color (extracted):**
- Background: light (#FFFFFF canvas, ~#F6F9FC sidebar chrome — estimated)
- Primary accent: `#635BFF` (Blurple, confirmed Stripe brand token)
- Text: near-black (`#1A1F36` estimated from Sail)
- Semantic: green for succeeded, red for disputes/refunds, amber for pending — applied as pill badges, not full-row fills

**Typography:** Sohne-family-adjacent proprietary sans, 400/500/600, body ~13-14px. Mono inside Workbench for request/response payloads.

**Density / layout:** Medium-high density, 12-col content, fixed-width left rail ~240px, breadcrumb + tabs header pattern.

**Motion:** Minimal — subtle fades, ~150ms ease-out on tab switches; no flourish.

**Key borrowable decisions:**
- Sidebar is *resource-oriented* (Home, Balances, Transactions), not feature-oriented — maps 1:1 to objects in the system.
- Dedicated "Shortcuts" section for pinned + recently-visited pages; solves deep navigation without submenus.
- Global `?` opens keyboard shortcut sheet; **`⌘+I` copies current object ID from anywhere.**
- **Workbench is a separate pane for API/webhook logs** — logs live *next to* the entity, not in a detached "developers" app.

**Explicit NOT-to-borrow:**
- Settings split across Personal/Account/Product categories — overkill for single-user kiroxy.
- Product catalog, Connect, Atlas, Capital surfacing in "More" — enterprise sprawl.
- Light-only default; dark mode lands awkwardly on Sail.

**Source URLs:**
- https://docs.stripe.com/dashboard/basics
- https://docs.stripe.com/dashboard/search
- https://stripe.com/blog/dashboard-updates-oct-2020

---

### Netlify app — Tier A — Deploy/hosting console

**URL:** https://app.netlify.com

**What it is:** Netlify's logged-in operator panel for sites, deploys, teams.

**Visual signature:** Dual left navigation — team-level rail and site-level rail coexist, with the team rail collapsing to a compact strip when you're inside a site. Top breadcrumb carries you back out. Sites list supports grid (screenshot thumbnails) and table (sort/filter chips). Deploys screen is a vertical stack of "Deploy Cards" with color strips: **yellow indicates main/production branch, grey all others.** React + Tailwind + Storybook.

**Color:** Light default (#FFFFFF) / dark mode secondary. Primary accent: Netlify teal (~#00C7B7 estimated). Semantic green for success, yellow for main-branch marker, red for build-failed.

**Typography:** Inter-like sans (estimated from screenshots), 400/500/600. Mono in build logs.

**Density / layout:** Medium density, card-based deploy list, table + grid toggle for sites.

**Motion:** Restrained; Chromatic-backed visual regression implies they guard motion tightly.

**Key borrowable decisions:**
- Breadcrumb plus compact secondary rail lets you keep team context while deep in a site — elegant for any tool with "global vs scoped" views.
- Color strip + icon on each deploy card encodes branch type at a glance, no text needed.
- Filters and sorts at top of list are URL-serializable and shareable.

**Explicit NOT-to-borrow:**
- Dual-context navigation overkill for single-user.
- Grid view with site thumbnails — pure visual candy with no signal for a proxy/ops tool.
- Teal-heavy marketing-adjacent accent tends to look "SaaS-y."

**Source URLs:**
- https://netlify.com/blog/contextual-improvements-to-netlify-navigation
- https://storybook.js.org/blog/netlify-rebrand-storybook-chromatic
- https://netlify.com/blog/2016/09/01/a-game-of-cards

---

### Railway — Tier A — Infra canvas / PaaS

**URL:** https://railway.com (dashboard at https://railway.app)

**What it is:** Dark-first infra console where "services" live on a pannable Canvas graph.

**Visual signature:** The Canvas is the signature — a dark-navy workspace where service nodes float as rounded rectangles, connected by faint edges, pannable like Figma. Left sidebar is terse: projects, environments, settings. Typography leans generous and confident; the whole tone is "terminal-adjacent but with actual taste." Built on Vite + TanStack Router after moving off Next.js to prioritize a rich client-side interface.

**Color:** Dark (~#0B0D0F estimated) from screenshots + railway.app/design palette. Primary accent: violet/purple (~`#8B5CF6` estimated). Semantic dots on the canvas node (green healthy, red failed, amber building).

**Typography:** Inter (estimated — confirmed by VSCodeThemes screenshots). Mono: JetBrains Mono / Geist Mono for logs.

**Density / layout:** Medium density in sidebars, low density on Canvas (intentional — graph wants breathing room).

**Motion:** Pan/zoom Canvas with physics, smooth 200ms panel transitions, realtime multiplayer cursors.

**Key borrowable decisions:**
- **Dark-first commitment** — never apologizes, no "light mode is default" hedge.
- **`/design` page as public artifact**: palette, type, banner, buttons, charts — design system as marketing.
- **Typographic confidence** — uses type size and weight for hierarchy before resorting to color.
- `⌘K` as a first-class UI element, not a hidden power-user feature.

**Explicit NOT-to-borrow:**
- Canvas itself — kiroxy has one service, not a graph; canvas metaphor is pure theater.
- Realtime multiplayer awareness — single-user tool.
- Heavy purple brand saturation can tip into "gamer aesthetic."

**Source URLs:**
- https://railway.app/design · https://blog.railway.app/p/moving-railways-frontend-off-nextjs
- https://railway.com/changelog/2026-03-20-new-dashboard-layout
- https://nicelydone.club/apps/railway

---

### Render — Tier A — Deploy/hosting console

**URL:** https://dashboard.render.com

**What it is:** Services-and-datastores operator console, Stripe-influenced in restraint.

**Visual signature:** **Quiet, grown-up, slightly boring in the best sense.** Left pane lists Services, Blueprints, Env Groups; top strip has a breadcrumb and a prominent `+ New` button. Content is mostly clean tables with status dots, tabbed detail views per service (Events, Logs, Metrics, Settings). Provides light, dark, and **high-contrast** variants of each. Log explorer has its own independent theme setting. No ornamental animation.

**Color:** Light default / dark (~#0F1115 estimated) / high-contrast tokens. Primary accent: Render purple (~#7C3AED range, quiet). Semantic: green for live, grey for suspended, red for failed — pill badges.

**Typography:** Inter (estimated). Mono: exclusively inside log explorer.

**Density / layout:** Medium, table-first, breadcrumb-heavy navigation.

**Motion:** Near-zero — this is a tool for people who want to forget the UI exists.

**Key borrowable decisions:**
- `⌘+K` workspace-wide search with keyboard-only navigation.
- **Breadcrumb primary, sidebar secondary** — breadcrumbs carry the orientation load.
- **Separate theme setting for the log explorer, independent of the dashboard** — recognizes logs are a different mode.
- **High-contrast theme as a first-class citizen**, not an afterthought.

**Explicit NOT-to-borrow:**
- Heavy reliance on "Workspace" construct — irrelevant for single-user.
- Somewhat generic — doesn't have a strong visual fingerprint; easy to mistake for any React + Tailwind admin.
- `+ New` top-right assumes you create things often; kiroxy creates almost nothing.

**Source URLs:**
- https://render.com/docs/render-dashboard.md
- https://render.com/changelog

---

### PlanetScale — Tier A — Database admin / branching UI

**URL:** https://planetscale.com (console at app.planetscale.com)

**What it is:** Git-metaphor database admin — branches, deploy requests, schema diffs.

**Visual signature:** **Dense, deliberately GitHub-shaped UI.** Database dashboard is a tab bar (Overview, Branches, Deploy Requests, Insights, Settings) over a content well. Deploy Request pages are the showpiece: Summary tab shows deployability with specific blockers; Schema changes tab renders a line-by-line diff in green additions / red deletions, exactly like a code PR. "Instantly deployable" and "Approved" badges. Gated deployment toggles inline. Branches page is a nested tree visualization. Dark-forward with selective magenta/pink accents. **"Code review, not database admin."**

**Color:** Dark default (~#0C0D0F estimated) / light mode supported. Primary accent: magenta/pink historically (~#E5267A estimated), currently toned down. Semantic: green/red for schema diffs; amber "potential data loss" warnings.

**Typography:** Inter-likely. **Mono is prominent** — used wherever SQL, table names, or identifiers appear; schema diff is entirely mono.

**Density / layout:** High density, multi-tab detail pages, embedded code diffs.

**Motion:** Minimal — this tool is not trying to delight, it's trying to not let you delete prod.

**Key borrowable decisions:**
- **Git PR metaphor applied to a non-git domain** — the strongest move on this list. Deploy requests have Summary / Changes / Comments tabs. Changes are diffed. **For kiroxy: routing-rule changes are schema changes; proxy-config updates are deploy requests.**
- Non-deployable states surface *specific* blockers (invalid charset, missing unique key), not a generic "error."
- **Gated deployment** = "start the migration, pause before cutover, human clicks final Apply." Perfect pattern for risky ops actions.
- "Instantly deployable" badge as trust signal — auto-computed state communicated visually.

**Explicit NOT-to-borrow:**
- Branch tree visualization — too database-specific.
- Sheer quantity of tabs per page; kiroxy can't sustain that surface area.
- Magenta/pink brand saturation if you want to read as "infra" rather than "database vendor."

**Source URLs:**
- https://planetscale.com/docs/concepts/deploy-requests
- https://planetscale.com/docs/concepts/branching

---

### Cloudflare Dashboard — Tier A — Dense ops tool / sprawling product

**URL:** https://dash.cloudflare.com

**What it is:** Cloudflare's logged-in admin for DNS, Workers, R2, Zero Trust, and ~60 other products.

**Visual signature:** **The counterexample and the case study.** Left sidebar expands into nested product trees (each with 4-8 children). Top strip is account/zone selector. Content pages are tab-heavy, dense tables (DNS records, firewall rules, worker deployments) with inline edit. Cloudflare orange `#F6821F` is reserved almost entirely for the logo and primary CTAs; the working surface is restrained grey-on-white with a mediocre dark mode. Feels powerful when you know where you're going, overwhelming otherwise.

**Color:** Light default (#FFFFFF / ~#F6F6F7), dark mode available but visibly less polished. Primary accent: Cloudflare orange #F6821F (confirmed). Text near-black. Semantic: green active / red error / grey paused.

**Typography:** Inter or similar (estimated), 400/500/600. Mono for DNS records, worker code, API tokens.

**Density / layout:** Very high density, nested sidebar, deep tabbed detail pages.

**Motion:** Functional only — no decorative animation.

**Key borrowable decisions:**
- Dense, **editable tables with inline status, no modals to view a single row** — scan and act without navigation.
- Account/zone selector at the top of the sidebar is a permanent-context-switcher; no ambiguity where you are.
- **Restraint on the orange** — using brand color only for CTAs and the logo keeps the working surface neutral.

**Explicit NOT-to-borrow:**
- **Product sprawl** — nested sidebar with 60 children is the anti-pattern for a single-service tool.
- Dark mode clearly a second-class citizen; inconsistent token application.
- Inconsistent page layouts — different products feel like different apps (Workers looks different from DNS which looks different from Zero Trust).

**Source URLs:**
- https://dash.cloudflare.com
- https://developers.cloudflare.com/fundamentals/setup/manage-members/roles

---

**Synthesis pointers for kiroxy DESIGN_SYSTEM.md:**
- Camp vote: **dark-first like Railway/PlanetScale, typographic hierarchy like Stripe, breadcrumb + ⌘K skeleton like Render.**
- Steal PlanetScale's diff-and-badge pattern for config changes (routing rules are a lot like schema migrations).
- Steal Stripe's Workbench pattern — logs pane lives *next to* the thing it's about.
- Steal Tailscale's search DSL for the request log.
- Avoid Cloudflare's nested sprawl, Netlify's dual-context, and Railway's Canvas — all solve problems kiroxy doesn't have.

---

## Tier B — Developer Tools with Signature UI

> Power-user aesthetics, keyboard-first interactions, command palettes, distinctive motion.

**Overall takeaway:** Across these nine products, the signature move is the same in spirit but radically different in execution — each has taken a different aesthetic stance on "the operator deserves a tool that rewards expertise." Raycast weaponizes macOS vibrancy and keeps accent-red below 10% of any view. Superhuman treats `⌘K` as a training wheel that self-deprecates (every row shows the direct shortcut, so after 3-4 uses muscle memory replaces the palette). Arc bet the whole product on a sidebar + a command bar that replaces the URL bar. Zed treats the theme system as a public API, not a skin. Warp solved a decade-old terminal problem with one invention — the Block — and built everything around it. Notion teaches hierarchy is a *data model*, not decoration. Figma's UI3 is the cautionary tale about floating panels (they publicly reverted to docked). Tailscale shows restraint is the aesthetic — search DSL over drop-downs. Replit demonstrates the Kanban task board as an operational view. For kiroxy, the borrowable pattern: pick ONE signature mechanic (blocks? command-bar-as-URL? accent used only at 5%?) and let the rest orbit around it. The trap: thinking "command palette + dark mode + monospace numerals" is enough. Each of these products has one **irreducible idea** that everything else supports.

### Raycast — Tier B — macOS-native launcher / command palette

**URL:** https://raycast.com

**What it is:** A Spotlight replacement for macOS — floating 750px-wide window summoned by `Opt+Space`, wraps a plugin SDK, grown into productivity surface (AI chat, clipboard history, window management, notes).

**Visual signature:** A small floating rounded-rectangle window (continuous corners, `.ultraThinMaterial` / `NSVisualEffectView` blur) that sits over whatever you were doing — never full-screen. Dark-only interior with a 4-step surface ladder (`#1C1C1E → #242424 → #2C2C2E → #3C3C3C`), hairline borders, and a signature red-orange accent (`#FF6363`) that never appears on more than ~5-10% of any view. Keycap hints (⌘1, ⌘K) live on the right edge of every row in muted rounded rectangles — **the UI itself teaches its own shortcuts**. Brand's three diagonal red stripes are used once per marketing page, max.

**Color / Typography / Density / Motion:**
- **Color:** Dark-only (`#0d0d0d → #1111 → #1C1C1E → #242424 → #2C2C2E` elevated overlay), Raycast Red (`#FF6363` / `#FF4D4D`) as sole accent, category accents for extension icons only. Elevation through surface luminance steps, NOT drop shadows.
- **Typography:** Inter with `font-feature-settings: "calt", "kern", "liga", "ss03"` — `ss03` alternate "g" is a brand tell. Primary labels 14-15px medium; metadata 11-12px regular muted. SF Symbols for iconography.
- **Density:** ~36-40px row height, ~12px horizontal padding. Fits 7-8 result rows without scrolling.
- **Motion:** Window fades in with tiny scale-up (spring). Row selection instant. List re-sorts on filter with crossfade, not slide.

**Signature interactions:**
- `Opt+Space` summons from anywhere in macOS — the hotkey is sacred.
- Every action row shows its shortcut as a keycap on the right (`⌘1` for first result, `⌘K` for action menu).
- **Two-tier palette**: navigation palette (`Opt+Space`) + action palette (`⌘K` on selected item). Avoids command-list bloat.
- Extension commands and app-launcher results share the same list — no mode switching.
- Window is NEVER resizable — fixed floating surface.

**Key borrowable decisions:**
- **Accent rarity.** Red on ~5% of any view. Signal, not decoration. THIS is what makes it feel premium.
- **Elevation through surface color, not shadows.** 4-step ladder + 1px hairline borders. Crisp in dark mode.
- **Keycap hints embedded in every row.** UI is its own documentation — no separate "shortcuts help" modal needed.
- **Two-tier palette pattern** (navigation + action).
- **Fixed window size.** Removes "what size should this be" cognitive tax. Ops tools rarely need full-screen.

**Explicit NOT-to-borrow:**
- Native macOS vibrancy doesn't translate to a web dashboard; `backdrop-filter` impostors chug at scale and look cheap.
- Small floating window is wrong for ops — kiroxy needs dense tables that want full viewport.
- Raycast's marketing 64px display type + 96px section rhythm is consumer SaaS spacing, not ops-tool density.

**Source URLs:**
- https://raycast.com · https://developers.raycast.com
- https://seedflip.co/blog/raycast-design-system-dark-ui (surface ladder + accent %)
- https://www.dembrandt.com/explorer/raycast · https://getdesign.md/design-md/raycast/preview

---

### Superhuman — Tier B — command palette as training system

**URL:** https://superhuman.com

**What it is:** Email client whose entire product thesis is "speed is the feature." Onboarded via a mandatory 30-min human-led training; every action has a keyboard shortcut; `⌘K` is both the universal palette and the **shortcut-teaching mechanism**.

**Visual signature:** Three-pane layout (folders / list / reading), dense rows, elegant display type on marketing but **the command palette specifically uses a monospaced font** to "evoke the feeling of directing a powerful machine" (their words). Palette takes over the center at invocation — visually imposing, not a subtle dropdown.

**Color / Typography / Density / Motion:**
- **Color:** Light-mode-first (surprisingly). Near-pure white, deep gray text, one blue accent for links, purple/violet for reminders. Signal colors are tiny colored dots, never fills.
- **Typography:** Sans UI, but **palette uses mono** — deliberate "now you're driving the machine" signal.
- **Density:** Readable-not-punishing — 14-15px labels, 12px metadata muted.
- **Motion:** Absent where it would delay. Palette appears instantly. "Done" confirmations = green flash, not toast.

**Signature interactions:**
- `⌘K` opens the palette from ANYWHERE, including inside the compose editor (they override default text-editor `⌘K → insert-link` to keep palette sacred).
- Every palette row shows its shortcut on the right — **explicit pedagogy**: the palette is designed to make itself obsolete for you.
- Vim-inspired nav: `J/K` next/prev, `E` archive, `R` reply, `F` forward, `C` compose, `Z` undo, `/` search, `G` + letter for folders.
- Splits = `⌘K → Create Split` (VIP contacts, team domain, unread). Essentially saved filter views as first-class UI.
- Vocabulary as design: "Mark Done" instead of "Archive." The words you pick in the palette shape how users feel.

**Key borrowable decisions:**
- **Command palette as training wheel.** Every row shows its shortcut. After N uses, muscle memory forms and palette becomes vestigial. **Single highest-leverage pattern in the entire dossier.**
- **Monospace font INSIDE the palette specifically.** Visual signal: "you're driving the machine now."
- **Palette is central, not peripheral.** Center of viewport, large area. Not a dropdown, not a top-right search.
- **Vocabulary as design.** Pick kiroxy's verbs carefully ("Kill connection" vs. "Close", "Drain" vs. "Stop").
- **Every action, no exceptions.** If mouse can do it, `⌘K` must do it. No second-tier palette.

**Explicit NOT-to-borrow:**
- Mandatory 30-min human onboarding — not replicable at single-user self-hosted scale.
- Light-mode-first — wrong for an ops tool operators stare at for hours.
- Consumer-email restraint (white, airy, therapeutic) undersells kiroxy's power-tool stance.

**Source URLs:**
- https://superhuman.com · https://blog.superhuman.com/how-to-build-a-remarkable-command-palette/
- https://download.superhuman.com/Superhuman%20Keyboard%20Shortcuts.pdf

---

### Arc — Tier B — command bar replaces URL bar, sidebar replaces tabs

**URL:** https://arc.net

**What it is:** A Chromium browser from The Browser Company that made two bets: (1) the URL bar should be replaced by a centered command bar, (2) horizontal tabs should be replaced by a vertical sidebar with Spaces. (Note: company pivoted to Dia; Arc in maintenance but UI language is still the reference.)

**Visual signature:** Chrome-less content area with a vertical sidebar on the left (collapsible via `⌘S`) containing pinned tabs, folders, and today's tabs in three horizontal zones. **The sidebar itself can be themed per Space** — switching Spaces swipes the whole sidebar contents. No URL bar at the top of the window — `⌘T` summons a centered command bar overlaid on the current page. Tabs auto-archive after 12 hours.

**Color / Typography / Density / Motion:**
- **Color:** Per-Space theming. Each Space has its own palette that tints the sidebar. Content area stays neutral.
- **Typography:** System UI font (SF on macOS). Distinctiveness is spatial, not typographic.
- **Density:** Sidebar rows ~28-32px. Three zones (pinned, folders, today's) share row style with visual separators.
- **Motion:** Spring-based transitions everywhere. Space switching = sidebar slide. "Little Arc" mini-window has a distinctive bounce-open. This is Arc's whimsy — motion does more work than color.

**Signature interactions:**
- `⌘T` opens the Command Bar — universal entry: new tab, search, switch to open tab, run extension, navigate, create Space, open Notion doc. NO traditional address bar.
- `⌘S` toggles sidebar entirely. Arc "seems to really want you to close your sidebar" (Verge).
- `⌘Option N` opens "Little Arc" — frameless mini-window for quick lookups, doesn't pollute main sidebar. **A weight-class below a new tab.**
- `⌘Option ←/→` switches Spaces; `Ctrl+1/2/3` jumps to Space N.
- `⌘1..9` jumps to pinned tab N.
- Drag tab to middle of content = instant Split View (up to 4-way).

**Key borrowable decisions:**
- **Command bar replaces primary navigation.** Arc's move: delete the URL bar entirely, make `⌘T` the front door. For kiroxy: **what if there's no top nav at all — just a palette front door?**
- **Tiered window weight-classes.** Full window vs Little Arc vs Split View — each a different intensity for a different task. For kiroxy: full dashboard vs "peek at connection X" inspector vs side-by-side diff of two requests.
- **Auto-archive as default.** Tabs vanish after 12h. For ops: old sessions, idle connections, closed tunnels — archive automatically.
- **Per-workspace color theming.** For kiroxy: dev/staging/prod environments each with a signature tint. "Am I looking at prod?" becomes pre-attentive.
- **Sidebar as stacked zones with separators**, not a flat list.

**Explicit NOT-to-borrow:**
- Whimsy (bouncy springs, frosted colored sidebars, playful empty states) reads as consumer — dated for a post-2025 ops tool that should feel surgical.
- Arc's "Boost" (user-injected CSS/JS on any website) — scope creep.
- Little Arc's lack of forward button and single-tab constraint is a novelty that confuses users.

**Source URLs:**
- https://arc.net · https://start.arc.net/command-bar-actions
- https://start.arc.net/master-multitasking
- https://www.theverge.com/23462235/arc-web-browser-review

---

### Zed — Tier B — dark-first native editor, themes as public API

**URL:** https://zed.dev

**What it is:** Code editor written from scratch in Rust on a custom GPU-accelerated framework (GPUI), dark-first, Vim-compatible, collaborative. Built by ex-Atom/Tree-sitter founders. Homepage itself showcases the UI.

**Visual signature:** Dense three-pane IDE layout (project tree / editor / panels), chrome almost absent — no heavy frames, thin 1px separators, tabs are minimal text-only. Default font is **Lilex** (`.ZedMono`) — programming ligature font with distinct geometric feel. **Themes are first-class**: app ships with Ayu, Gruvbox, One family, all defined in structured JSON following a published schema (`zed.dev/schema/themes/v0.2.0.json`).

**Color / Typography / Density / Motion:**
- **Color:** Dark-first with 3 theme families (Ayu/Gruvbox/One), light + dark variants. Theme JSON exposes ~50+ semantic tokens per language capture. Marketing: near-pure black (`#0a0a0a`) with one warm red highlight (`~#D64545`).
- **Typography:** Lilex (programming ligatures, humanist geometry) for editor. UI labels system sans.
- **Density:** High. Small tab labels, thin scrollbars, minimal panel padding. Line-height ~1.4.
- **Motion:** Fanatical about input latency. GPU-smooth scrolling. No decorative animations. Pane splits animate ~120ms. **Motion as a performance advertisement** — every animation must justify its frame budget.

**Signature interactions:**
- `⌘Shift P` command palette (VS Code muscle memory). Commands namespaced: `editor: toggle format on save`, `theme selector: toggle` — **namespace is searchable and acts as categorization**.
- `⌘Alt ,` opens settings **as JSON** directly — config-as-code by default, not a GUI.
- `⌘P` file quick-open, `⌘Shift F` project search.
- Themes hot-reload — edit JSON, save, window restyles instantly.
- Vim mode is first-class, modal editing with text objects and marks.

**Key borrowable decisions:**
- **Themes as structured JSON schema with a published spec.** Operators can remix/fork; product treats theming as an API, not a setting. **For kiroxy: publish a theme schema, ship 2-3 defaults, let people write their own.**
- **Config-as-code by default.** `⌘Alt ,` opens raw JSON with autocomplete. No settings GUI for 90% of knobs. Dev-tool operators expect this.
- **Dense chrome, generous content.** Thin 1px separators, text-only tabs. The content (logs/requests/tables) gets the pixels.
- **Namespaced command palette.** `editor:`, `workspace:`, `theme:`. Searchable even when you forget the exact command.
- **Theme Builder.** Zed's `/theme-builder` previews every surface as you tweak. **Even a stripped-down kiroxy theme editor (tokens live-preview on current view) is a massive signal of respect for the operator.**

**Explicit NOT-to-borrow:**
- GPUI / native GPU rendering — kiroxy is a web dashboard. Borrow the *ethos* (every animation must justify itself) without the tech stack.
- "Paper-white on black" marketing + big serif-adjacent display = consumer-tech; works for Zed because they sell to individuals.
- Multibuffer UI — unique but confuses first-timers. For kiroxy, don't invent novel multi-pane metaphors.

**Source URLs:**
- https://zed.dev · https://docs.zed.dev/configuration/themes
- https://zed.dev/docs/extensions/themes · https://zed.dev/schema/themes/v0.2.0.json
- https://zed.dev/theme-builder
- https://github.com/zed-industries/zed/tree/main/assets/themes

---

### Warp — Tier B — the Block as the signature primitive

**URL:** https://warp.dev

**What it is:** Rust-built modern terminal (now open-source Apr 2026) whose core invention is the **Block** — every command + its output is grouped into a scrollable, filterable, copyable, shareable unit. AI agents layer on top, but the Block is load-bearing.

**Visual signature:** Looks like a terminal at first glance but is profoundly different on second look — each command is wrapped in a visually separated Block (left-edge accent bar, rounded container, timestamp, exit-code badge). **Scroll through Blocks with `⌘↑/↓` as if they were messages in a chat.** Cmd-click to attach a Block as context to an AI prompt. Dark-first, warm-off-black background, pink/magenta brand accent. **Feels like Slack + iTerm had a baby.**

**Color / Typography / Density / Motion:**
- **Color:** Dark warm (~`#161618`), monospace body, muted block separators, one signature pink-magenta accent (`~#E84393`) for agent affordances. Exit codes colorized (green zero, red non-zero) on block header.
- **Typography:** Monospace terminal surface; UI chrome sans-serif. Block headers use smaller muted label.
- **Density:** Blocks have visible padding (8-12px internal) — costs vertical vs classic scrollback, but tradeoff is navigability.
- **Motion:** Blocks fade inline as commands execute. Ghost-text italic preview for AI suggestions — accept with `Tab`. Minimal otherwise.

**Signature interactions:**
- `⌘P` (macOS) / `Ctrl+Shift+P` — Command Palette for the whole Warp app (settings, navigation, toggles). Separate from the terminal.
- `Ctrl+R` — Search across command history AND saved Workflows (goes beyond `history | grep`).
- `⌘D` split panes; `Opt+⌘I` sync input across multiple panes (magical for multi-server ops).
- `⌘↑/↓` — Attach previous block as context to AI query / clear attached blocks. **Block is first-class AI context.**
- `⌘⏎` — with text "Send to agent"; with error block selected, "Attach 'npm install' output as context."
- **Block permalinks** — every Block generates a shareable URL (command+output).
- Workflows — parameterized saved commands in the palette; user-authored runbooks living in the terminal.

**Key borrowable decisions:**
- **Invent one primitive that replaces a commodity.** Block replaced scrollback. **For kiroxy: what's the one primitive that replaces "the connections table"? A Request Block (request + response + timing + upstream logs, cmd-clickable, shareable)?** Highest-leverage idea.
- **Every unit is shareable via permalink.** Every request/connection/error row should have a shareable link — teammate debugging goes 10x faster.
- **Contextual palette hints at the input.** Warp's prompt line shows inline: `⌘↩ for new agent`, `⌘↑ attach 'npm install' output`. UI surfaces what you can do with current state. **For kiroxy: when a user has a connection selected, show inline `⌘K actions / ⌘D drop / ⌘L logs`.**
- **Separate Command Palette (`⌘P`) from context-driven action hints.** Warp doesn't make `⌘P` do everything.
- **Ghost-text autocomplete.** Non-intrusive; you can keep typing over it.

**Explicit NOT-to-borrow:**
- Pink-magenta accent — Warp brand. Don't copy.
- Block-level padding costs vertical real estate — fine for terminals, bad for dense dashboards with tables of connections. Use blocks for the *hero* surface only (request inspector), keep tables dense.
- Conversation chip + follow-up arrow UX specific to multi-turn AI. kiroxy doesn't need an LLM chat surface.

**Source URLs:**
- https://warp.dev · https://warp.dev/modern-terminal · https://warp.dev/all-features
- https://docs.warp.dev/agent-platform/local-agents/interacting-with-agents/terminal-and-agent-modes/

---

### Notion — Tier B — information hierarchy / block system

**URL:** https://notion.so

**What it is:** Document+database workspace built on a graph of "blocks" — every text line, row, page, even the workspace is a block with a parent pointer and content array. **The UI is a direct render of that tree.**

**Visual signature:** Extremely restrained chrome. Left sidebar is a tree of pages (split into 4 tabs as of 3.4 — pages, agent chats, meetings, notifications), center is a single scrollable document column with generous margins, right side is contextual (comments, properties, AI). Zero decorative color — accent comes from emoji/custom icons users bring themselves. **Typography carries 90% of the visual load.**

**Color / Typography / Density / Motion:**
- **Color:** Mostly greyscale with a muted blue accent.
- **Typography:** One sans font (Inter-like) with tight weight hierarchy (400 body, 600 headings, 500 UI).
- **Density:** *Paragraph-spacing-adaptive* — different padding for list items vs paragraph blocks based on adjacent-block type. Lists compact, prose breathes.
- **Motion:** Near-invisible — only drag-and-drop, toggle open/close, slash-menu slide.

**Signature layout:** Three-zone shell — sidebar (~240px, resizable), doc column (max-width ~900px centered), optional right rail. Database inline views nest INSIDE document columns — **the database UI is subordinate to the doc, not the other way around.**

**Key borrowable decisions:**
- **Adjacency-based spacing.** Tight padding between items of the same type (table rows, log lines), loose between different types. Rhythm without strict baseline grid.
- **Structural indentation.** Nesting means *ownership*, not just visual offset — a child metric inherits its parent's scope (permissions, tags, time window). Works beautifully for kiroxy's account → route → request hierarchy.
- **Right-side contextual rail** opens/closes without reflowing main content. Good home for "inspect a single request" without a modal.
- **Slash-menu for actions** — keyboard-first command palette anchored to cursor position.
- **Custom icons as the only source of color.** Chrome stays neutral.

**Explicit NOT-to-borrow:**
- Loud empty-state cards. An ops dashboard with zero traffic should show zeros, not a marketing card.
- Crowded sidebar (pre-3.4 era) — junk-drawer risk.
- Inventing new glyph for "settings" loses decade of muscle memory. Stick to gear/three-dots.

**Source URLs:**
- https://www.notion.com/blog/data-model-behind-notion
- https://www.notion.com/blog/updating-the-design-of-notion-pages
- https://theorganizednotebook.com/blogs/blog/notion-new-ui-design-update-june-2025

---

### Figma — Tier B — canvas + panel layout

**URL:** https://figma.com

**What it is:** Browser-native design tool with infinite canvas flanked by left (layers/assets) and right (properties) panels, a floating bottom toolbar, and top menu bar. **UI3 (GA Oct 2024) is the current generation.**

**Visual signature:** Canvas is king — center 60-80% of the screen is always the artboard, never chrome. Slim bottom toolbar replaced old top-heavy toolbar. **Panels are docked and resizable after Figma publicly walked back their floating-panel experiment** — explicit admission that floating "slowed people down" and "cramped the canvas."

**Color / Typography / Density / Motion:**
- **Color:** Near-monochrome (white/light-grey panels, dark-grey text) + single blue accent for selection/primary actions.
- **Typography:** Inter throughout.
- **Density:** Very high — right properties panel can show 20+ input fields in a single screen.
- **Motion:** Functional — property-panel sections expand/collapse 150ms ease. No decorative motion on canvas.

**Signature layout:** Four-zone shell — top menu bar (~40px), left panel (docked, resizable), canvas (flexes), right properties panel (docked, resizable, ~240-280px default). Bottom floating toolbar (~48px) with tool palette. **Properties panel reordered components to the TOP above x/y/w/h** because design-systems work became the majority use case.

**Key borrowable decisions:**
- **Docked + resizable as default, floating as exception.** Figma paid in engineer-years to learn this — do not re-learn it.
- **Thin bottom toolbar for primary actions.** Frees vertical space where users look most.
- **Properties panel ordered by frequency of use, not by convention.** For kiroxy: show `status / latency / tokens / cost` first, not raw headers first.
- **Minimize UI** (collapse both panels to slivers) as a power-user mode for focus-reading dashboards during incident response.
- **One accent color only.** Blue for selection/active. Everything else is semantic state.

**Explicit NOT-to-borrow:**
- Floating panels on hover — already proven to fail. Skip.
- Icon-only controls without labels as default — new users can't discover features.
- Canvas as a metaphor for a dashboard. Works for spatial artifacts, not for lists and tables.

**Source URLs:**
- https://figma.com/blog/our-approach-to-designing-ui3
- https://www.figma.com/blog/behind-our-redesign-ui3/
- https://ux-news.com/figma-has-updated-to-ui3-for-all-users/

---

### Tailscale admin — Tier B — beautiful-ops aesthetic

**URL:** https://login.tailscale.com/admin

**What it is:** Zero-trust network admin console — devices (tailnet machines), ACLs, DNS, users. Dense infrastructure operation, **rendered with unusual restraint.**

**Visual signature:** Looks more like a marketing page than an admin panel — deliberately. Generous white space, one primary table (machines), **aggressive use of search-as-filter rather than drop-down menus**, typography-forward rather than chart-forward. Built in React with Radix primitives and Tailwind, with a custom design-system layer on top for semantic tokens.

**Color / Typography / Density / Motion:**
- **Color:** Light mode: off-white background (~#FAFAFA), near-black text, one blue for links, semantic green/amber/red dots. **Dark mode (May 2024)** shipped after multi-year delay — used as forcing function to replace ad-hoc Tailwind classes with semantic tokens (`text-base`, `text-muted`, `text-disabled`). Chose `outline` over `ring` for accessibility.
- **Typography:** Inter-like sans at 14px body.
- **Density:** Medium-low for an ops tool — row breathing room over cramming info, because search DSL lets users filter rather than scan.
- **Motion:** Almost none — only table row hover and dropdown open.

**Signature layout:** Top horizontal navigation (~56px) with tabs (Machines, Users, Access Controls, DNS, Settings, Logs). **No left sidebar at top level** — navigation is horizontal because there are 6-8 sections. Inside a tab: search bar pinned to top, single dense data table, right-side detail drawer slides in on row click. **Search DSL** is the power user's path: `is:internal`, `has:update-available`, `lastseen:<10m`, `os:macos`, `tag:server`, `owner:shreya@...` — they explicitly chose filtering over sorting because sorting is usually a proxy for "find a device."

**Key borrowable decisions:**
- **Search DSL over dropdowns.** For kiroxy's request log: `model:claude-sonnet status:429 latency:>2s user:alice` beats six drop-down filter controls. Typeahead on the colon unlocks the whole filter vocabulary.
- **Horizontal top nav, not left sidebar**, when top-level nav is small (6-10 items). Saves 240px of horizontal space for actual data.
- **Semantic color tokens from day one.** `text-muted`, `bg-raised`, `border-subtle` — not `gray-500`. Pays for itself the moment dark mode ships.
- **One primary table per page, right-side drawer for detail.** Row click opens drawer, doesn't navigate away — keeps list context visible.
- **Ship a slimmer "mini" variant.** Their macOS windowed-app has "mini player" mode. For kiroxy: `/mini` or menubar-sized view showing just live throughput + top 3 errors.

**Explicit NOT-to-borrow:**
- Marketing-page whitespace in dense ops flows. Tailscale has low device counts (tens-hundreds); kiroxy will have thousands/min. Use Tailscale's *typography and color discipline* but pack denser — 32-36px row height, not 48-56px.
- Hiding sorting entirely — defensible for devices, too purist for a request log where `order by cost desc` is legitimate first-use.
- Horizontal-only nav — works for Tailscale's ~8 top-level sections but kiroxy's scope (Dashboard, Requests, Accounts, Routes, Settings, Logs, Metrics) already pushes that limit.

**Source URLs:**
- https://tailscale.com/blog/heart-of-dark-mode
- https://tailscale.com/blog/windowed-macos-ui-beta
- https://github.com/tailscale/tailscale/issues/2540 · https://login.tailscale.com/admin

---

### Replit workspace — Tier B — split-pane + Kanban task board

**URL:** https://replit.com

**What it is:** Browser IDE + hosting + AI agent, pivoted toward "describe it and Agent builds it" with Agent 4. **Workspace is the unified shell: chat thread + live preview + file editor + shared Kanban task board, all rearrangeable via Splits.**

**Visual signature:** Dark by default, high-density, dev-tool DNA but softened — rounded corners (~8px), subtle inner borders instead of hard separators, colorful accent per project type. The **panel system is the product**: users split any pane into 5 directions using conical drop zones. Underneath: panels are a multi-node tree — split same direction = new child; split opposite = new subtree.

**Color / Typography / Density / Motion:**
- **Color:** Dark neutral background, one brand orange-red accent, semantic colors for run states (green=running, amber=building, red=error).
- **Typography:** Monospace for code, sans for chat/UI.
- **Density:** Very high in editor, medium in chat thread.
- **Motion:** The standout: drag-and-drop is "fluid and interruptible" — mid-drag cancel returns pane to origin, panes shrink continuously as you drag. Inspired by Apple's Fluid Interfaces WWDC talk.

**Signature layout:** Workspace = freely arrangeable multi-node tree of panes. Default template: left (files), center (editor), right (preview), bottom (console). **Task board = Kanban: Drafts → Active → Ready → Done.** Each task gets its own isolated project copy until merged.

**Key borrowable decisions:**
- **Kanban task board as the operational view.** For kiroxy: each long-running request/batch/refresh moves through `Queued → Running → Succeeded / Failed`. Multi-task overview without a polling loop.
- **Isolated project copy per task.** Replit dispatches each agent task into "an exact copy of your current project." For kiroxy: each debug-a-request action forks to a read-only environment (replay that request with a different model/prompt) without touching prod traffic.
- **Multi-node tree for panel layout.** Not pixel-positioned — a tree means "put a webview in top-right" is a simple "insert at rightmost leaf." Serializable, restorable, shareable via deep-link.
- **Per-thread chat per task.** Instead of one god-chat, each background task gets its own thread.
- **Fluid interruptible drag.** If panel rearrangement ever ships, Apple-fluid-interfaces model is the reference.

**Explicit NOT-to-borrow:**
- Infinite canvas as first-class surface. Works for Replit (visual apps). Kiroxy is tabular ops data — canvas adds zero value.
- Agent chat as the primary entry point. Replit Agent 4 reduces the home screen to a prompt box. For kiroxy, live metrics and active alerts must be primary.
- Dark-only — reasonable for a code editor, but an ops tool used during daytime incident response needs both.

**Source URLs:**
- https://blog.replit.com/splits · https://blog.replit.com/whats-changed-agent3-to-agent4
- https://blog.replit.com/introducing-agent-4-built-for-creativity

---

## Tier C — Design Systems

> Primitives, tokens, theming, docs patterns. Borrow architecture, reject aesthetic clichés.

**Overall takeaway:** The design-system question for kiroxy is architectural, not aesthetic. There are three viable philosophies in 2026, each with a lineage. **Radix UI primitives** (now folded into [shadcn/ui primitives](https://ui.shadcn.com/docs/primitives) and [Ark UI](https://ark-ui.com)) give you unstyled, accessible behavior — you own every visual pixel. **shadcn/ui copy-paste** ships pre-styled Radix wrappers via the registry CLI — you own the source. **Bundled libraries** (Mantine, Chakra, HeroUI) ship complete styled components you import — you own the consumption. For kiroxy (opinionated aesthetic, zero marketing pressure, single-operator, open-source), the correct path is **Radix-primitives-via-Ark-UI with hand-authored styles in a kiroxy-specific token layer**. Copy-paste (shadcn) risks "looks like every other shadcn dashboard" and the operator has explicitly rejected that. Bundled libraries lock you into their aesthetic. Radix primitives alone give you behavior + accessibility + ARIA — and you do the aesthetic work that makes the mansion feel like yours.

### Radix UI Primitives — Tier C — Unstyled accessible primitives

**URL:** https://www.radix-ui.com/primitives

**What it is:** An unstyled, accessible component library maintained by Jenna Smith at WorkOS (formerly Modulz → WorkOS). The gold standard for accessible primitives: Dialog, Popover, DropdownMenu, Select, Tooltip, Toggle, Switch, Accordion, Tabs, Toast, Slider, NavigationMenu, Combobox (Command), and ~30 more. Each primitive handles WAI-ARIA, focus management, keyboard nav, portal rendering, and composition-by-slot.

**Visual signature:** None — primitives are headless. Every kiroxy styling decision is yours.

**Primitive / component architecture:** **Unstyled primitives via compound components with slot-based composition.** Example (`Dialog.Root` → `Dialog.Trigger` + `Dialog.Portal` → `Dialog.Overlay` + `Dialog.Content` → `Dialog.Title` + `Dialog.Description` + `Dialog.Close`). Data-state attributes (`data-state="open"`, `data-disabled`) drive styling via CSS selectors — no className juggling. `asChild` prop to compose behavior with any element.

**Tokens / theming approach:** **No theming — that's not Radix's job.** Radix Colors (a sister project at https://www.radix-ui.com/colors) provides 30-step (12 light + 12 dark + 6 alpha) semantic color scales: app-bg, subtle-bg, component-bg-default/hover/active, subtle-border, border, hover-border, solid (for saturated fills), low-text, high-text. This semantic ladder is the 2020s ops-tool palette architecture.

**Docs patterns worth stealing:**
- **Per-primitive page structure**: Overview → Features → Installation → Anatomy → API Reference → Examples → Accessibility. Every primitive ships a complete docs page.
- **Anatomy diagram shows the slot tree**: `<Dialog.Root>` → `<Dialog.Trigger />` → `<Dialog.Portal>` with child slots enumerated.
- **Accessibility section is mandatory** per primitive — lists ARIA roles, keyboard interactions, screen-reader announcements.

**Key borrowable decisions:**
- **Slot-based compound components** over monolithic props. `<Dialog title="..." description="..." />` locks you out; `<Dialog.Root>` with named children scales forever.
- **Data-state attributes for styling hooks** instead of class-toggling. CSS reads as `[data-state="open"] { ... }`.
- **`asChild` composition pattern** lets users substitute their own element without breaking behavior. Kiroxy's primitives should all support this.
- **The 12-step semantic scale is a mental model worth adopting** (even if kiroxy uses 7 steps — see `Tier A → Supabase` and `Appendix → Color System 2026`).

**Explicit NOT-to-borrow:**
- The Radix Colors default palette itself — it's tied to the Modulz/WorkOS aesthetic. Use the semantic structure, generate your own OKLCH values.
- Radix Themes (the styled wrapper at https://www.radix-ui.com/themes) — it's fine for prototypes but has a distinctive look that signals "I used Radix Themes." kiroxy skips it.

**Source URLs:**
- https://www.radix-ui.com/primitives
- https://www.radix-ui.com/colors
- https://www.radix-ui.com/colors/docs/palette-composition/understanding-the-scale

---

### Ark UI — Tier C — Cross-framework accessible primitives

**URL:** https://ark-ui.com

**What it is:** Cross-framework (React/Vue/Solid) accessible primitive library from the Chakra team (Segun Adebayo), built on top of Zag.js state machines. Spiritual successor to Radix for the non-React world, or a Radix alternative for teams wanting machine-backed state logic.

**Visual signature:** None (headless).

**Primitive / component architecture:** Same slot-based compound pattern as Radix, but state is explicitly machine-driven via Zag. Every primitive has a documented state machine with named states, guards, and transitions. This makes edge cases (typeahead buffer flushing, focus trap restoration, escape cascades) reproducible and testable.

**Tokens / theming approach:** None — inherits from whatever token system you bring. Pairs well with Park UI (a Panda CSS theme layer built on Ark) if you want a preset.

**Docs patterns worth stealing:**
- **Framework-specific installation + usage tabs** (React / Vue / Solid) on every primitive page. Same API across all three.
- **State machine diagrams** documented per primitive — you can see the lifecycle.
- Uses Ariakit's `@container style(...)` pattern for theme variants — see the `style()` query reference in the Tier D section.

**Key borrowable decisions:**
- **State machines for complex interaction logic.** When kiroxy's command palette gets nested (search → action menu → confirm dialog → error), explicit state machines beat useState chains.
- Cross-framework neutrality is insurance — kiroxy's current stack (Svelte) could migrate without abandoning primitive logic.

**Explicit NOT-to-borrow:**
- Park UI's aesthetic ships a specific look. Skip for kiroxy.
- Ark's abstraction is denser than Radix's — for a single-framework (Svelte) kiroxy, Radix's simpler slot composition wins unless you need the state-machine tooling.

**Source URLs:**
- https://ark-ui.com
- https://zagjs.com
- https://park-ui.com

---

### shadcn/ui — Tier C — Copy-paste Radix wrappers

**URL:** https://ui.shadcn.com

**What it is:** A registry of pre-styled Radix primitive wrappers. Copy via CLI (`npx shadcn@latest add dialog`) — the source lands in your repo, yours to modify. Tailwind + CVA (class-variance-authority) for variants. Not a dependency — a code-gen registry.

**Visual signature:** **The shadcn look**: slate-gray backgrounds, subtle rounded corners (~6-8px default), thin borders, geometric sans (Inter/Geist), subtle shadows, subtle hover states. It has become the 2024-2026 "AI dashboard template" aesthetic — and is precisely what the kiroxy operator wants to transcend ("not AI-slop, not template").

**Primitive / component architecture:** Styled copy-paste wrappers over Radix. Each component ships ~30-80 LOC of Tailwind classes. You own the source file.

**Tokens / theming approach:** **CSS custom properties with HSL values** in a `:root` / `.dark` dual-declaration pattern. Default tokens: `--background`, `--foreground`, `--card`, `--popover`, `--primary`, `--secondary`, `--muted`, `--accent`, `--destructive`, `--border`, `--input`, `--ring`. Tailwind v4 registry variant (`ui.shadcn.com/colors`) ships these in OKLCH.

**Docs patterns worth stealing:**
- **Live + code + copy-button on every component** — the demo is next to the implementation.
- **Component registry model** — install via CLI, source lands in your repo. Operators own their UI code.
- **Theme generator** at `ui.shadcn.com/themes` — visual palette picker that emits tokens for your `globals.css`.

**Key borrowable decisions:**
- **CLI-driven copy-paste distribution** — this is a genuinely better model than npm-install-a-component-library. Kiroxy primitives should ship via an installer that drops source into the repo.
- **CVA (class-variance-authority) for variant management** — typed variants, no runtime penalty. `button({ variant: "destructive", size: "sm" })`.
- **HSL → OKLCH migration path** — shadcn's v4 shows how to evolve tokens without breaking consumers.

**Explicit NOT-to-borrow:**
- **The default look.** Operator explicitly rejects this. Install shadcn's primitives, then **rewrite every component's classes** using kiroxy's OKLCH tokens and typography scale. Kill the default radius, adjust shadow spec, reconsider border weight (lean Supabase — borders over shadows).
- **The default marketing-card aesthetic** (gradient hero, stat grid with six pastel icon backgrounds). Operator rejects this by name.
- **Treating shadcn as a dependency**. If you upgrade a primitive via the CLI without preserving your overrides, you inherit the template look overnight. Commit hard to your rewrites.

**Source URLs:**
- https://ui.shadcn.com
- https://ui.shadcn.com/docs/installation
- https://ui.shadcn.com/themes
- https://github.com/shadcn-ui/ui

---

### GitHub Primer — Tier C — Product design system for ops/dev tools

**URL:** https://primer.style

**What it is:** GitHub's official design system — the one powering github.com, GitHub Desktop, and GitHub CLI output. Open-source under GitHub org. Includes Primer CSS, Primer React, Primer ViewComponents (Rails), Octicons icon set, Primer Brand (marketing surface).

**Visual signature:** **Dense, text-forward, octicon-heavy.** Chrome is minimal, content is dense tables + code + diff highlights + avatars. Uses GitHub's proprietary "Mona Sans" for display and "Hubot Sans" as a display companion (both OFL), paired with a system mono for code. Primary accent is GitHub Green (`#2DA44E` light, `#3FB950` dark) used sparingly for primary buttons and "merged" badges — status greens (open) and purples (merged) are semantic-heavy.

**Primitive / component architecture:** Full styled library (not headless). Import `@primer/react` and get opinionated components: `Button`, `IconButton`, `Label`, `Octicon`, `Box`, `Timeline`, `ActionList`, `Pagehead`. Polymorphic via `as` prop.

**Tokens / theming approach:** **CSS-in-JS with Primer Primitives as tokens.** 6 theme variants (`light`, `light_high_contrast`, `light_colorblind`, `dark`, `dark_dimmed`, `dark_high_contrast`, `dark_colorblind`). Dark Dimmed is GitHub's answer to "pure dark mode is too aggressive" — softened backgrounds (~`#22272e`) for reading sessions. Tokens defined as CSS custom properties in `@primer/primitives`.

**Docs patterns worth stealing:**
- **6-theme matrix** (light/dark × default/dimmed/high-contrast) shipped out of the box. **For kiroxy: offer light, dark, and dark-dimmed from v1.3.**
- **Status-specific color semantics** (issue-open green, issue-closed red, PR-merged purple, PR-draft gray). Your ops states deserve this treatment.
- **Octicons** (https://primer.style/octicons) — hand-tuned 12px/16px/24px grid icons with consistent stroke. Open-source MIT.

**Key borrowable decisions:**
- **Dark Dimmed as an explicit theme variant** — pure black is aggressive for reading sessions. Offer a softer dark (~#1A1B1E backgrounds, slightly lifted from pure black) as an opt-in.
- **Octicons as inline SVG with `sx` sizing** — don't ship a font icon set.
- **Semantic status colors** over generic success/warning/danger — kiroxy's "account cooldown" is not "warning"; it's its own state.

**Explicit NOT-to-borrow:**
- The Primer CSS class names (`Box`, `Pagehead`, `Timeline`) — too GitHub-flavored for a proxy dashboard.
- Mona Sans / Hubot Sans — too GitHub-branded.
- The `@primer/react` component palette — opinionated in a GitHub-specific way (issue cards, PR files diff); primitive architecture gives less starting point for kiroxy than Radix.

**Source URLs:**
- https://primer.style
- https://primer.style/foundations/primitives
- https://primer.style/octicons
- https://github.com/primer

---

### Vercel Geist — Tier C — Design system (already covered under Tier A)

See **Tier A → Vercel (Geist Design System)** for full treatment. Summary:
- Structured 10-step semantic color scale (Color 1-10 + Background 1/2)
- Geist Sans + Geist Mono (OFL)
- Named operator primitives (Entity, Status Dot, Gauge, Relative Time Card, Context Card, Loading Dots, Empty State)
- Tabular numeral variant as a first-class type token
- Keyboard Input + Snippet as first-class components

**Adopt:** the Color 1-10 architecture and the named operator primitives. **Skip:** Geist Sans (too Vercel-branded) and the full 10 chromatic scales (kiroxy strips to ~5).

---

### Tailwind UI catalog — Tier C — Production house style

**URL:** https://tailwindui.com

**What it is:** Adam Wathan / Steve Schoger's paid template catalog of pre-designed components (marketing, application UI, ecommerce). Not a design system — a marketplace of Tailwind-styled patterns. Runs on Tailwind v4 + Headless UI (Tailwind Labs' own unstyled primitives).

**Visual signature:** **Clean, confident, reads as 2023-2025 Tailwind-dashboard.** Generous whitespace, flat cards with 1px borders, rounded 6-8px, Inter body, indigo/purple accents by default (overridable). Every template demonstrates Schoger's "Refactoring UI" principles: correct whitespace rhythm, text color hierarchy via gray scale, intentional alignment.

**Primitive / component architecture:** Headless UI primitives (Menu, Listbox, Combobox, Dialog, Disclosure, Popover, Switch, Tabs, Transition) + RadixUI-like slot composition. Tailwind UI itself is templates built on Headless UI + Tailwind.

**Tokens / theming approach:** Tailwind's semantic tokens via `@theme` in v4 — `--color-*`, `--font-*`, `--spacing-*`. Fully customizable.

**Docs patterns worth stealing:**
- **Every component page has 3-5 variants** (simple / with-header / with-avatar / dense / split) — shows the same component under multiple conditions.
- **Copy-the-HTML-directly** affordance (no framework lock-in).

**Key borrowable decisions:**
- **Refactoring UI principles** (whitespace rhythm, text-color hierarchy, intentional alignment) are timeless. Worth reading Schoger's book regardless of aesthetic direction.
- **Headless UI primitives** are MIT and can replace Radix if you want Tailwind-Labs-maintained alternatives.

**Explicit NOT-to-borrow:**
- **The default aesthetic is the shadcn look's older sibling.** Same trap — "looks like every dashboard built in 2024." Operator rejects.
- Indigo/purple default accents.
- Marketing-template density (large hero images, testimonial carousels, pricing tables with checkmarks) — kiroxy is a dashboard, not a landing page.

**Source URLs:**
- https://tailwindui.com
- https://headlessui.com
- https://www.refactoringui.com

---

## Tier D — Bleeding-Edge 2026 Web Tech

> Platform features to adopt, avoid, or use carefully. Grounded in real production examples.

**Overall takeaway:** The 2026 baseline has shifted hard. OKLCH, `color-mix()`, `light-dark()`, `:has()`, `@starting-style`, subgrid, anchor positioning, cross-document view transitions — **all in all four evergreens with global usage above 82%.** The story is no longer "this is experimental." It's "this replaces a library." Tailwind v4 shipped OKLCH-only tokens. shadcn/ui generates OKLCH palettes. VSCode, pdf.js, Mozilla, Apache Airflow, Three.js, and Signal all use `light-dark()` in production. `@starting-style` has displaced half the JS-driven enter/exit animation code in real codebases. For kiroxy — loopback, single-user, you control the browser — **skip the polyfill tier, skip the PostCSS transform chain, skip CSS-in-JS runtimes, skip JS popover libraries.** The browser has become the framework. The one holdout is Container Style Queries (Firefox 0% in 2026). Everything else: ship native. The premium signal in 2026 is *not* using frosted-glass, *not* using rainbow gradients, and having exactly one accent color that sings.

### View Transitions API (cross-document, Level 2) — Tier D

**What it is:** CSS-only opt-in that makes full-page MPA navigations crossfade/morph between documents via a browser-native snapshot-and-interpolate pass.

**Browser support state:** Chrome 126+, Edge 126+, Safari 18.2+/iOS 18.2+, Firefox 144+ (partial — does not yet fully match cross-doc types). Global usage ~85%. [caniuse](https://caniuse.com/cross-document-view-transitions).

**Production examples:**
- WebKit's own blog ships it: [webkit.org/blog/16967](https://webkit.org/blog/16967/two-lines-of-cross-document-view-transitions-code-you-can-use-on-every-website-today/)
- [GoogleChromeLabs/view-transitions-toolkit](https://github.com/GoogleChromeLabs/view-transitions-toolkit) — official reference

**Key usage:**
```css
@view-transition { navigation: auto; }
::view-transition-old(root) { animation: 120ms ease-out fade-out; }
::view-transition-new(root) { animation: 200ms ease-out fade-in; }
```

**Pitfalls:** `view-transition-name` must be unique on the page at any time — duplicates silently drop. Don't use the default root fade to paper over 400ms FCP jank.

**Verdict for kiroxy:** **Use.** Two lines of CSS, fallback is "nothing happens." Zero-risk polish.

---

### CSS `@scope` — Tier D

**Browser support:** Chrome/Edge 118+, Safari 17.4+, Firefox 146+ (shipped Feb 2026). Global 88.4%. [caniuse](https://caniuse.com/css-cascade-scope).

**Verdict for kiroxy:** **Use carefully.** Good for theme-scoping root-level styles or dropping in a single vendor widget. Don't use as your primary organization strategy — a Tailwind v4 setup is less cognitive load.

---

### CSS `@container` Queries (size + style) — Tier D

**Browser support:**
- **Size queries**: all evergreens since 2023, global 93%+.
- **Style queries**: Chrome/Edge 111+, Safari 18+ (partial — only custom properties, not arbitrary props), **Firefox still 0% in 2026**.

**Verdict for kiroxy:** **Use size queries freely. Use style queries only with custom-prop values** (`@container style(--var: value)`). Ariakit production pattern. Avoid any design that *requires* Firefox style-query support.

---

### `:has()` selector — Tier D

**Browser support:** Chrome/Edge 105+, Safari 15.4+, Firefox 121+. Global 93.5%. Fully Baseline.

**Production:** Statamic CMS nav, Jenkins credentials plugin, Spree admin, VS Code sidebar — all ship `:has(input:checked)` or `:has([data-open])` patterns.

**Verdict for kiroxy:** **Use everywhere.** Single biggest CSS quality-of-life win of the last five years. Kill `useState` for "is this dropdown open" — mark the element `[data-open]` and style parent via `:has([data-open])`.

---

### OKLCH + `color-mix()` + `light-dark()` — Tier D

**Browser support:** OKLCH Chrome/Edge 111+, Safari 15.4+, Firefox 113+ (global 92%); `color-mix()` universal since 2023 (Baseline); `light-dark()` all evergreens since mid-2024 (global 88.6%).

**Production examples:**
- Tailwind CSS v4 core theme — entire default palette is OKLCH
- shadcn/ui v4 — OKLCH tokens for every theme
- VSCode, Mozilla pdf.js, Deno coverage viewer, Three.js devtools, Signal Desktop, Apache Airflow — all `light-dark()` in production

**Verdict for kiroxy:** **Use, mandatory.** Three tokens (`--bg`, `--fg`, `--accent`) + `color-mix()` derivations + `light-dark()` = an entire dashboard's color system. You get free hover states, free borders, free disabled states, free transparent washes.

---

### P3 gamut / `display-p3` — Tier D

**Browser support:** Safari led since v10 (2017); Chrome/Edge 111+, Firefox 113+. `color()` function global 92%. Display requirement is the real constraint — P3 only renders on wide-gamut displays (Apple Retina 2015+, modern iPhones, most recent laptops).

**Verdict for kiroxy:** **Use via OKLCH.** Don't write `color(display-p3 ...)` directly. Pick OKLCH values with chroma 0.15-0.22 for accents and let browsers+displays resolve. kiroxy users are developers on modern Macs/laptops — 80%+ have P3 displays.

---

### CSS Anchor Positioning — Tier D

**Browser support:** Chrome/Edge 125+, Safari 26.0+, Firefox 147+ (Feb 2026). Global 82.8%. [oddbird polyfill](https://github.com/oddbird/css-anchor-positioning) covers older browsers.

**Production:** scikit-learn HTML repr, HumanSignal Label Studio, Statamic CMS nav indicator, Ariakit segmented-button, MS Bot Framework Web Chat.

**Verdict for kiroxy:** **Use.** This is the Floating UI killer. kiroxy ships no Popper. Dashboard popovers, dropdowns, menus, tooltips — all native now. Use with `@supports` feature gate for older browsers.

---

### CSS `@starting-style` — Tier D

**Browser support:** Chrome 117+, Edge 117+, Safari 17.5+, Firefox 129+. Global 88.6%.

**Production:** Microsoft VSCode sidebar, PocketBase modals, Mozilla pdf.js, cheeaun/phanpy, Cockpit CMS — `@starting-style` + `transition-behavior: allow-discrete` for popover/dialog enter/exit.

**Verdict for kiroxy:** **Use everywhere.** Kill Framer Motion / react-spring for enter/exit on popovers, menus, toasts, modals. This + `<dialog>` / `[popover]` + `allow-discrete` = native animated overlays with zero JS state.

---

### View Transition Types — Tier D

**Browser support:** Level 2 spec — Chrome/Edge 126+, Safari 18.2+. Firefox 144+ ships same-doc, cross-doc type matching lags.

**Production:** Adobe React Spectrum (S2) Toasts — `active-view-transition-type(toast-add, toast-remove, toast-expand, toast-collapse, toast-clear)`.

**Verdict for kiroxy:** **Use carefully.** Basic crossfade covers 95% of need. Reach for types only if you need directional semantics (log drill-down slides right, breadcrumb-up slides left).

---

### CSS Subgrid — Tier D

**Browser support:** Firefox 71+ (shipped 2019), Safari 16+, Chrome/Edge 117+. Global 89.8%.

**Production:** atopile LogViewer (dev-tool log viewer aligns column headers and log rows via subgrid — **exact kiroxy use case**), pierre diff viewer, vkurko calendar, Google security research dashboard.

**Verdict for kiroxy:** **Use, mandatory for tables.** kiroxy's core UI is dense tables. Subgrid is the right tool.

---

### Container Style Queries — Tier D

**Browser support:** Chrome/Edge 111+, Safari 18+ (custom props only), **Firefox 0% in 2026** ([bug 1795622](https://bugzilla.mozilla.org/show_bug.cgi?id=1795622)).

**Verdict for kiroxy:** **Skip, or use with clear fallback.** If users on Firefox must be supported, prefer `:has()` + data attributes: `.group:has([data-open]) .content { ... }`.

---

### Fine-Grained Reactivity (Svelte 5 / Solid / Qwik) — Tier D

**Svelte 5 runes** — signals under the hood, Rich Harris at Vercel. Production: Apple Music Web, Apple Podcasts, IKEA, NYT, 1Password, Square, Hugging Face. Pattern: `$state`, `$derived`, `$effect`.

**Solid.js / SolidStart** — purest signal model. Production: Cloudflare Pages docs uses SolidStart. Pattern: `createSignal`, `createMemo`, `createEffect`.

**Qwik resumability** — Builder.io's approach. Production: builder.io itself. Adoption outside Builder.io remains small.

**Verdict for kiroxy:**
- **Svelte 5** — strong fit. Apple-quality UX, small bundle, keyboard-first ergonomics. Already the Dashboard Next choice.
- **Solid** — equally valid. Smallest runtime.
- **Qwik** — skip. Resumability's big win is first-paint over slow networks; kiroxy is localhost.

---

### "Refined-Minimal / Warm-Modern" 2026 Aesthetic — Tier D

**What it is:** Dominant dev-tool aesthetic — warm off-whites, true-but-not-pure blacks, generous-yet-dense, micro-shadows over hero-shadows, one accent color, monospaced numerics, 8-12px border-radius.

**Named products:** Linear, Raycast, Vercel dashboard (2025 refresh), Resend, Cal.com, Attio, Railway, Neon, Planetscale, Dia Browser.

**Key patterns:**
- Background: `oklch(11% 0.005 260)` (cool) or `oklch(14% 0.01 80)` (warm) — never pure 0/0
- Off-white: `oklch(99% 0 0)` or `oklch(98% 0.008 80)` — never `#fff`
- Borders: 0.5-1px, `color-mix(in oklch, var(--fg) 8-12%, transparent)` — whisper-thin
- Radii: 6px inputs, 8px cards, 10-12px panels, 16px modals. Never uniform.
- **One accent with chroma 0.15-0.22. Primary CTA, focus ring, "selected" state, nothing else.**
- Motion: 120-200ms, `cubic-bezier(0.16, 1, 0.3, 1)` (the "Linear easing" that became universal)

**Verdict for kiroxy:** **Use.** This palette is the correct target. Copy Linear's color discipline, Raycast's typography rhythm, Vercel's grid density.

---

### "Utopian" Typography / Fluid Type Scales — Tier D

**Browser support:** `clamp()` universal since 2020, Baseline.

**Verdict for kiroxy:** **Use selectively, not dogmatically.** kiroxy's main UI is a dashboard — dense, predictable, NOT marketing. Clamp the rare heading and the hero of the welcome screen. Chrome (buttons, tables, sidebar) should be fixed `px`/`rem`. Fluid everywhere makes the interface feel squishy and less like a professional tool.

---

### 2025-2026 Icon Trend — Tier D

**What's premium:** 24px grid, 1.5-2px stroke, rounded linejoin.

**Premium choices:**
- **Lucide** — shadcn/ui default, Tailwind default. 1704+ icons, 24px, 2px stroke (override to 1.5px for dense dashboards). MIT.
- **Radix Icons** — 15×15px grid, 1px stroke — Linear-adjacent.
- **Phosphor** — six weights (Thin, Light, Regular, Bold, Fill, Duotone) — the premium weight-variation choice.
- **Iconoir** — Apache 2.0, ~1600, 24px, 1.5px stroke — premium feel on free license.
- **Hand-rolled**: Linear, Raycast, Vercel Geist Icons all maintain their own to control style.

**Verdict for kiroxy:** **Lucide with `strokeWidth={1.5}` and `absoluteStrokeWidth`.** Override the default 2px. Consistent, 1704 icons covers every dashboard need, MIT, tree-shakeable, zero bundle cost for unused. Save hand-rolling for kiroxy's own logo/brand mark.

**⚠️ Rule: Mixing icon libraries looks amateur instantly. ONE library, no exceptions.**

---

## Tier E — Anti-reference: Homelab / Hobbyist UIs

> kiroxy must not look like another *arr-stack tool. These are what we're transcending.

**Overall takeaway:** The "self-hosted infra tool" genre has a recognizable visual tell: built by engineers solving their own problem, not to be looked at. Patterns repeat across the whole category — sidebar + dense stat grid + table list + colored status dots + logo bar. When one project looks like another, it's because they both adopted the path of least resistance (Bootstrap → MUI → shadcn).

**What reads as "dated / homelab-amateur":**
- Multi-hue stat cards (one color per stat — blue/violet/emerald/pink/cyan) — **single most overused dashboard trope of the 2020s.**
- Zero customization of shadcn defaults (default radius, default spacing, default everything).
- Logo-lists-as-features (100+ integration logos paraded on the homepage).
- No opinion on typography (Inter, system-ui, line-heights never considered).
- Port numbers visible in the URL (`:8989`, `:3000`, `:7470`).
- Dark mode as afterthought — a global `.dark` class flipping tokens.

**What kiroxy can steal without becoming one of them:**
- Typography as the primary signal — a specific display face + tabular-nums for data is worth more than any color token.
- Restraint in the stat row — one neutral treatment, not six. Motion and typography for emphasis, not color.
- Density with air — Homepage actually gets this right on the grid. Card isn't fighting you.
- Named aesthetic point of view — Open WebUI (neutral minimal) and LibreChat (apple-dev-tools warm) each have one. Arr-stack doesn't. **Having a point of view is the only way to escape the genre.**

### hexos — Tier E — Archived AI proxy + dashboard (upstream of kiroxy)

**URL:** https://github.com/kadangkesel/hexos · https://hexos.kadangkesel.net (archived 2026-05-10)

**What it is:** Lightweight AI API proxy (CodeBuddy/Cline/Kiro multi-provider), bundles a Next.js dashboard at `:7471`. Archived with "All providers are dead and no longer maintained!" banner. **kiroxy's direct ancestor.**

**Visual signature:** Dark-first shadcn dashboard with a single Halloween-orange accent (`oklch(0.837 0.128 66.29)`) and zero border radius (`--radius: 0rem`) everywhere. Sidebar + sticky navbar + content-pane-with-scroll layout, lifted from shadcn `dashboard-01` nearly verbatim. Landing page is a 6-card stat grid driving area/pie/bar charts from `recharts`. **Vibe: "crypto-tool dark dashboard" more than "infra admin."**

**Color / Typography / Density / Motion:**
- Warm sepia light theme (`oklch(0.98 0.005 48.998)`, all hues anchored to 48.998 for uniform tint)
- Dark: near-black `oklch(0.147 0.004 49.25)` with glass cards
- **Six hard-coded per-stat icon hues: blue/violet/emerald/pink/cyan/emerald — the classic trope**
- Bricolage Grotesque as both `--font-heading` and `--font-sans`
- Framer-Motion successor with staggered card intros

**Key borrowable decisions:**
- **Hue-locked OKLCH palette** (everything at hue 48.998) — automatic harmony without a designer.
- **Zero radius commits to a look.** It's opinionated. kiroxy should keep (or move to a specific non-zero like 8px/12px) — don't drift back to shadcn default.
- **Isolated re-render leaf for live counters** — good engineering pattern worth porting.
- **Route groups** `(dashboard)` / `(chat)` — clean URL architecture.

**What it gets wrong (opinionated):**
- **The six-color stat row is exactly the "looks like every admin dashboard" trap.** Reads as template, not product.
- **Dark theme labeled "Halloween inspired" in the code** — the aesthetic is themed, not designed. A primary should express a stance, not a season.
- **Bricolage is 2023's dashboard default.** Readable but generic.
- **Sidebar collapsed-by-default** — common homelab move that says "I don't trust my own nav labels."

**Source URLs:**
- https://github.com/kadangkesel/hexos
- https://github.com/kadangkesel/hexos/blob/d4c0d1c/dashboard/src/app/globals.css
- https://github.com/kadangkesel/hexos/blob/d4c0d1c/dashboard/src/app/(dashboard)/page.tsx

---

### Homepage (gethomepage) — Tier E — Homelab landing dashboard

**URL:** https://gethomepage.dev · https://github.com/gethomepage/homepage

**What it is:** YAML-configured personal landing page. You point it at your services (Sonarr/Plex/Proxmox/Portainer/etc.) and it renders a tile grid of bookmark cards plus live widgets from each service's API. 150+ supported integrations.

**Visual signature:** Tile grid of service cards, each with a logo-on-colored-background and a live status strip (queue counts, CPU, disk). Quiet neutral background — services are the visual events. **The whole genre of "homelab landing page" basically means this look.**

**Key borrowable decisions:**
- **Live values embedded in navigation** — the sidebar isn't just links, it's links-with-telemetry. kiroxy's account sidebar could do this.
- YAML-configurable everything — clean decoupling.

**What it gets wrong:**
- **The aesthetic IS other people's logos.** The page is a lobby for 100 tools with zero identity of its own. Avoid leaning on provider brand color to do your design work.
- Widget list as homepage feature (docs sidebar 150 items of "Adguard Home, APC UPS, Arcane, ArgoCD…"). **kiroxy should not let its integration list become its identity.**

---

### Portainer — Tier E — Docker/K8s management UI (enterprise-homelab hybrid)

**URL:** https://portainer.io · https://github.com/portainer/portainer (37.4k stars)

**What it is:** Web UI for Docker / Swarm / Kubernetes. Community edition free; business bolts on RBAC, SSO, support.

**Visual signature:** Chrome-heavy enterprise-admin feel. Top nav, left sidebar with expandable sections, deep breadcrumbs, big data tables with bulk-action checkboxes. Teal brand (`#13BEF9`-ish) on white/gray. **Designed to be screenshotted into corporate-IT compliance decks.**

**What it gets wrong:**
- **Looks like enterprise VMware-era software** — dense tables, gray-on-gray, zero confidence in whitespace. Most dated direction available.
- Stack is a visible scar — TypeScript/Go/JavaScript/HTML mixed repo, Angular-legacy + React-new running side-by-side. You can feel it in the UI.
- **"Get 3 free nodes of Business Edition" in README** — marketing inside the README is the vibe-tell. UX is already an upsell funnel.

**Source URLs:** https://github.com/portainer/portainer · https://www.portainer.io/features

---

### Sonarr — Tier E — The *arr-stack archetype

**URL:** https://sonarr.tv · https://github.com/Sonarr/Sonarr

**What it is:** PVR/indexer for TV series. Canonical member of the "arr-stack" (Sonarr/Radarr/Lidarr/Prowlarr/Readarr) that defines what homelab tool UIs look like.

**Visual signature:** Dark navy (`~#20282E`), accent blue (`~#5D9CEC`), poster-wall series view (which actually works because posters do the work), multi-tab detail views, tables with tiny icons, legacy-Bootstrap-era paddings. Icons look like Font Awesome 4. **This is the look kiroxy should most consciously reject.**

**Key borrowable decisions (limited):**
- Poster wall as default view — when your data has a cover image, use it at scale.
- Manual Search flow — Sonarr doesn't pretend to be magic. kiroxy's account screens could adopt the same honesty.

**What it gets wrong:**
- Navy + blue + dense tables + FontAwesome 4 = "written in 2014, never rethemed." Functional software with no visual point of view.
- Settings pages are a tree of tabs-within-tabs. Any time kiroxy reaches for "tabs inside tabs," pause.

---

### Jellyfin — Tier E — Self-hosted media server admin

**URL:** https://jellyfin.org · https://github.com/jellyfin/jellyfin

**What it is:** Open-source media server (Emby fork on .NET Core). Admin dashboard is a separate web UI.

**Visual signature:** Dark UI with purple-gradient accent that dates to the 2019 Emby fork. Hero-tile "continue watching" for client side; admin side is utilitarian sidebar + form. Big library tiles win; admin forms look like a 2018 Material dashboard.

**What it gets wrong:**
- **Admin looks like a different product from client.** Two aesthetic voices in one codebase is the universal self-host stumble.
- Gradient as brand — 2019 artifact. A single hue + supporting neutral ages far better.
- Plugin install pages are dense dropdowns on gray — the "I gave up trying to design the config surface" look.

---

### Open WebUI — Tier E — Self-hosted LLM chat frontend (current-gen)

**URL:** https://openwebui.com · https://github.com/open-webui/open-webui (137k stars, Svelte + FastAPI)

**What it is:** ChatGPT-shaped UI for Ollama and OpenAI-compatible APIs. Self-hosted, RAG, web search, RBAC, LDAP, SCIM.

**Visual signature:** **Faithful ChatGPT clone.** Left sidebar with conversation list, centered chat stream, composer at bottom, avatar-bubble messages. Neutral grayscale, very restrained color. Built on Svelte + Tailwind.

**Key borrowable decisions:**
- **Restraint wins.** Grayscale base + one accent reads as "tool" not "toy." **Biggest lesson in this tier.**
- Settings as modal over content, not a separate route tree.
- Admin surface is spatially separated from chat but uses **same chrome language** — same fonts, same radius, same spacing. No aesthetic bifurcation.

**What it gets wrong:**
- **It's a ChatGPT clone.** Fastest way to ship something legible, and also fastest way to be visually invisible. Users recognize the layout and stop seeing it.
- Giant feature-list README with 30 emoji bullets — self-host tell. Explains what to use it for, not what it is.

---

### LibreChat — Tier E — Positive reference (tier above the genre)

**URL:** https://librechat.ai · https://github.com/danny-avila/LibreChat (36.9k stars)

**What it is:** Multi-model chat (Anthropic/OpenAI/Azure/Bedrock) with agents, code interpreter, artifacts, MCP support, OAuth/SAML/LDAP/2FA. Pitches at companies.

**Visual signature:** Marketing site that reads **more Vercel/Resend than homelab**. Clean product hero with light+dark screenshots side-by-side, neutral palette, tight copy blocks, real typography. **The one in this list that doesn't announce itself as self-hosted software.**

**Key borrowable decisions:**
- **Dual light/dark hero screenshot** as the opening gesture — shows confidence in both themes. kiroxy should do this in its README.
- **Enterprise logos without losing indie vibe** — logos in monochrome on a neutral strip. They don't hijack the page.
- **"Everything you need" grid as 3×3 of one-line features** — each says the value, not the integration name. "Memory: persistent context across conversations so your AI remembers you." Product copy, not config docs.

**What it gets wrong:**
- Mild — marketing site leans Vercel-template; in-app is ChatGPT derivative. Both legible but neither is a new direction.

---

## Genre Conventions kiroxy Should Actively Break

Breaking any ONE of these will immediately distinguish kiroxy:

1. **Multi-color stat row.** Six pastel icon backgrounds is the single most overused move. Pick one treatment. Let motion and typography do the hierarchy.
2. **Sidebar + top-nav + content-pane.** Five of seven Tier E references use this exact chassis. A command-palette-first approach, or a single vertical feed, is instantly non-generic.
3. **Provider/integration logo wall as feature.** Homepage, Open WebUI, LibreChat all do it. kiroxy integrates things too — resist making the integrations be the product.
4. **"Dark mode is just tokens flipped."** Five of seven treat dark as a toggle over the same layout. A dark mode with different density or different information hierarchy reads as designed.
5. **Default shadcn radius + default Inter.** When tokens go unconsidered, the product looks like `npx shadcn@latest add *`. hexos at least picked `0rem`; any specific number (including 0) beats the default.
6. **Port number and `install.sh` in the hero.** Says "homelab" before saying what the product does. Lead with an outcome, not a curl command.
7. **README as feature-list emoji changelog.** Seven-of-seven do this. A one-paragraph README that says what kiroxy *is* — before listing what it does — would be visually anomalous.

**The anti-goal is clear: everything in Tier E targets the user who is already sold. kiroxy can target the user who is visiting for the first time and doesn't know they wanted a proxy yet. That audience responds to restraint, typography, and a single specific stance — not feature inventories.**

---

## Appendix — Topical deep dives

### Command palette and keyboard shortcut deep dive

**Invocation keys across 11 references (all primary-sourced):**

| Product | Primary key | Scope | Position |
|---|---|---|---|
| Raycast | `Opt+Space` (configurable) | Commands/apps/ext/AI/files/clipboard | Floating centered ~20% from top |
| Linear | `⌘K` / `Ctrl+K` | Actions + nav + entities (everything) | Centered modal ~640px |
| VS Code | `⌘Shift+P` / `Ctrl+Shift+P` | Mode-switched via sigil (`>`, `@`, `#`, `:`, `?`) | Drops from top center, ~600px |
| Superhuman | `⌘K` / `Ctrl+K` | Every action (palette IS the menu bar) | Centered modal with heavy backdrop blur |
| Slack | `⌘K`/`⌘T` (conversations), `⌘/` (cheatsheet) | Channels + DMs only (NOT actions) | Centered modal ~500px |
| Notion | `⌘P` or `⌘K` | Pages primarily; `/` for blocks; `@` mentions | Centered modal ~600px |
| Figma | `⌘/` or `⌘K` (historically `⌘/`); `Ctrl+Shift+?` for shortcuts | Menu items + plugins + AI tools (not layers) | Anchored to canvas center |
| GitHub | `⌘K` search, `⌘Shift+K` command mode | Multi-mode sigils (`>`, `#`, `@`, `/`, `!`) | Centered modal |
| Vercel Dashboard | `⌘K` | Projects/deployments/teams/actions | Centered modal ~600px |
| Stripe Dashboard | `/` focuses search, `?` opens shortcut sheet | **Entity search with operator syntax** (`amount:>149.99 email:...`) | Persistent top search bar (not modal) |
| Warp | `⌘P` (app palette); `⌘↑/↓` (block nav) | App settings (NOT terminal) | Centered modal |
| cmdk (pacocoursey) | `⌘K` by convention | Whatever you scope | Centered modal; nested sub-pages |

**Keyboard shortcut conventions 2026 (observed across the products above):**

| Action | Convention | Exceptions |
|---|---|---|
| Command palette | `⌘K` / `Ctrl+K` | VS Code uses `⌘Shift+P`; Arc uses `⌘T` |
| Quick file search | `⌘P` / `Ctrl+P` | Notion conflates it with `⌘K` |
| Keyboard shortcut cheatsheet | `?` (universal) | Figma `Ctrl+Shift+?`, Superhuman `?` |
| Focus search | `/` (universal — GitHub, Stripe, Linear) | — |
| Toggle sidebar | `⌘B` (VS Code, Linear) or `⌘S` (Arc) | — |
| New tab / nav forward | `⌘T` / `⌘Enter` (new tab) | — |
| Settings | `⌘,` (universal macOS) | — |
| Close/Back | `Esc` or `⌘W` | — |
| Submit + new | `⌘Enter` | — |

**🎯 Concrete recommendations for kiroxy:**

**Invocation keys:**
- **Primary**: `⌘K` (Mac) / `Ctrl+K` (Linux/Windows) — the 2026 default, don't fight it.
- **Alternate (focus search only)**: `/` — universal focus-search idiom from GitHub/Stripe/Linear.
- **Cheatsheet**: `?` — universal "show me the shortcuts" key.

**Palette layout (the Raycast-Linear hybrid):**
- Centered modal, ~640px wide, top-anchored ~20% from viewport top.
- Input row at top with subtle placeholder showing current mode (`Type a command, or / for accounts…`).
- **Empty state is a CURATED list, not empty** — show recents + 5 common actions (à la Vercel).
- **Every row shows its direct shortcut on the right as a keycap badge** — this is the Linear/Superhuman teaching pattern that makes the palette self-documenting.
- **Footer hints** show available modifiers for selected row: `↩ run · ⌘↩ run in new window · ⌘C copy ID · ? help`.
- **Two-tier palette** (Raycast pattern): root palette for navigation/actions; `⌘K` on a selected account opens an action sub-palette (refresh, copy ID, view logs, disable, delete).

**Navigation scheme within palette:**
- `↑/↓` — navigate
- `Enter` — execute (default action)
- `⌘Enter` — secondary action (open in new tab / view details without navigating)
- `⌘C` — copy the currently-selected item's ID to clipboard (Stripe pattern)
- `Tab` — narrow scope (GitHub pattern: `Tab` into a repo from org level)
- `Backspace` on empty input — pop to parent palette / widen scope
- `Esc` — close palette entirely

**Scope sigils (VS Code / GitHub mode-switching):**
- `/` — accounts
- `#` — requests (log entries)
- `>` — commands (actions)
- `@` — models
- `?` — help

**Full kiroxy keyboard cheatsheet (v1.3 target):**

| Key | Action |
|---|---|
| `⌘K` | Open command palette |
| `/` | Focus search (scoped to current view) |
| `?` | Open keyboard shortcut cheatsheet |
| `⌘,` | Open settings |
| `⌘B` | Toggle sidebar |
| `⌘1..9` | Jump to section N (Dashboard, Accounts, Requests, Routes, Settings, Logs, Metrics) |
| `G` then `D/A/R/S/L/M` | Vim-style section navigation (Linear-pattern alternative) |
| `J/K` | Row down/up in any table |
| `Enter` | Open selected row in right drawer |
| `⌘Enter` | Open selected row in full page |
| `⌘C` | Copy selected row ID |
| `E` | Edit selected row inline |
| `X` | Toggle selection checkbox |
| `⌘A` | Select all visible rows |
| `⌘D` | Drop/disable selected (with confirm) |
| `⌘R` | Refresh selected account token |
| `⌘.` | Toggle account status filter (active/cooldown/disabled/all) |
| `⌘⇧L` | Jump to live log tail |
| `⌘\` | Toggle dark/light (respects system by default) |
| `⌘⇧T` | Toggle density (comfortable/compact) |
| `Esc` | Close palette / drawer / modal; pop scope |

**Accessibility notes:**
- All palette interactions work keyboard-only (no mouse hover required).
- `aria-live="polite"` announces palette result count as user types.
- Focus trap inside palette when open; returns to previous focus on close.
- Keycap badges have `aria-hidden="true"` — screen readers hear the command, not "command K."
- High-contrast mode override: keycap badges get 2px borders and solid fills instead of subtle translucent.

---

## Changelog

- `2026-05-13` — Scaffold created; 9 parallel librarian subagents fired for Tier A/B/C/D/E + command-palette + typography + color deep dives.
- `2026-05-13` — Tier A (Grafana, Linear, fly.io, Vercel, Supabase, Stripe, Netlify, Railway, Render, PlanetScale, Cloudflare) complete. Typography + OKLCH color recipe committed.
- `2026-05-13` — Tier B (Raycast, Superhuman, Arc, Zed, Warp, Notion, Figma, Tailscale, Replit) complete.
- `2026-05-13` — Tier C (Radix, Ark, shadcn, Primer, Geist, Tailwind UI) synthesized from existing gallery evidence (subagent failed mid-run).
- `2026-05-13` — Tier D (15 web-platform features + aesthetic trend) complete.
- `2026-05-13` — Tier E (hexos, Homepage, Portainer, Sonarr, Jellyfin, Open WebUI, LibreChat) complete.
- `2026-05-13` — Command palette + keyboard shortcut deep dive complete with concrete kiroxy shortcut map.

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
