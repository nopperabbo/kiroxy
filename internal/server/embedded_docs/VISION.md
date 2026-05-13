# VISION.md — kiroxy

> What kiroxy is. Who kiroxy is for. Where kiroxy will not go.
>
> **Status:** v1.0 drafted 2026-05-13. This document drives product prioritization.
>
> **Companion documents:**
> - `docs/DESIGN_SYSTEM.md` — the visual and interaction language
> - `docs/ROADMAP.md` — trajectory v1.x → v2.0
> - `research-v3/REFERENCE_GALLERY.md` — the evidence behind every taste decision

---

## What kiroxy is (one sentence)

**kiroxy is a single-user, self-hosted proxy that turns your own Kiro IDE subscription into a local Anthropic and OpenAI API endpoint — built so you can point any AI-coding client at your own account without sending code through a third party.**

Seven-word version: *your Kiro subscription, as a local API.*

---

## Who kiroxy is for

### Primary persona — "The solo operator with a Kiro subscription"

A working engineer, likely SF Bay Area or equivalent global tech hub, who already has a Kiro subscription paid for personally or through work. They use Claude Code, Cursor, Continue, `opencode`, Zed's AI features, or another Anthropic-compatible client all day. Their problem is not finding a better LLM — they have the quota they need. Their problem is the **plumbing**: the client points at Anthropic's API, billing goes to Anthropic, but they're paying Amazon for Kiro already. kiroxy closes that loop.

Characteristics:
- Comfortable with YAML, env vars, `curl`, systemd, Docker Compose
- Runs kiroxy on `127.0.0.1` or inside a home Tailscale network
- Cares about: latency, reliability, not leaking code to a vendor, understanding exactly what's being sent upstream
- Does NOT care about: shared team UIs, RBAC, SAML/SSO, SOC 2
- Ships kiroxy in their dotfiles or Nix config; treats it as infrastructure, not a product

### Secondary persona — "The five-person trusted group"

A tight pod of engineers — co-founders, a small agency, a handful of friends — sharing a pooled set of 3-5 Kiro accounts via one kiroxy instance on a shared VPS or in a Tailscale tailnet. They trust each other completely. They do not need per-user quotas, but they benefit from request-level visibility ("whose Claude Code is hammering acct-2?"). For this persona, kiroxy is a **household utility**, not a product — like a shared Plex server.

Characteristics:
- One operator, four beneficiaries
- Shared API key or per-user kiroxy API keys with no enforcement
- Single source of telemetry (which account pulled how much)
- Pool management is the operator's job; beneficiaries don't see the dashboard

### Tertiary persona — "The open-source contributor"

Someone who finds kiroxy on GitHub, reads the README, thinks "oh, that's clever," and either forks, contributes, or adapts the architecture for a sibling tool. This persona matters because **kiroxy is how the operator introduces themselves as an engineer to the community.** The code has to be clean; the docs have to be good; the dashboard has to be impressive on first load.

Characteristics:
- Discovers kiroxy via HN, Twitter/X, kubeshark-style viral word-of-mouth, or opencode docs
- Judges kiroxy by the README hero image + the `/dashboard` screenshot before reading any code
- If they stay past the first 60 seconds, they read `docs/ARCHITECTURE.md` and `BUILD_LOG.md`
- May contribute bug fixes, adapters, doc improvements
- Does not operate kiroxy themselves long-term — but carries the aesthetic forward

### Explicit non-persona — "The enterprise multi-tenant operator"

kiroxy is **not** for operators running a shared gateway for hundreds of users, enforcing per-user quotas with billing, managing keys via SSO, exporting audit logs for compliance. That's LiteLLM, Portkey, or OpenRouter's job. We deliberately leave that market.

Characteristics of the market we reject:
- Multi-tenant auth (RBAC, SAML, SCIM)
- Per-user cost attribution and billing
- Compliance frameworks (SOC 2, HIPAA, ISO 27001)
- Multi-provider routing rules ("try Claude then fall back to GPT")
- Admin UI complexity (Cloudflare-dashboard sprawl)

If you need these things, kiroxy is actively the wrong tool. Use:
- **LiteLLM** — for multi-provider routing with OpenAI-compatible shape
- **Portkey** — for enterprise gateway with cost + compliance
- **OpenRouter** — if you want hosted multi-provider access

kiroxy is a *smaller* product. That's a feature.

---

## Product vibes

One paragraph per surface. These paragraphs are the taste brief — every design decision traces back here.

### It feels like: Grafana meets Linear meets fly.io, not Zapier

kiroxy's dashboard should feel like you're looking at an instrument panel, not a marketing page. Grafana taught us the four-layer background model (canvas → panel → elevated → popover) and the off-white-with-blue-cast text that reads as "terminal on a CRT." Linear taught us the keyboard-first command palette as primary navigation and the LCH-generated palette that stays perceptually clean. fly.io taught us that CLI-first empty states (`kiroxy add-account --refresh-token=...` as the CTA, not a button) communicate engineering confidence. **We are an operator tool with taste.** We are not a SaaS dashboard with a sidebar of integrations. We are not a "let me help you build your first prompt" AI chat interface. We are infrastructure that an engineer runs, observes, and occasionally reconfigures — rendered with care.

### It sounds like: "ops tool with taste," not "startup SaaS"

kiroxy's copy — in README, error messages, empty states, CLI output — has a specific register:

- Dry, technical, self-aware
- First-person singular only in docs narrator voice ("I decided to use Go because…"), never in product UI ("I couldn't find…" is banned)
- Uses `$29/month, Guaranteed response time, Run by Fly.io engineers, not chat bots` energy — kept plain-spoken, refuses hype
- Never apologizes for being small, single-user, or opinionated
- Error messages follow `{problem} — {cause} — {action}`. Example: `Upstream returned 403 — access token may be revoked — run 'kiroxy debug-refresh <account-id>'`
- Empty states follow `{what you're seeing} — {why you're seeing it} — {how to make it non-empty}`
- No "Oops! Something went wrong!" No "Great choice!" No "Coming soon!" (if it's not there, don't mention it)

### It looks like: dark-first, monospace for data, sans for prose, high density without claustrophobia

Dark is the default — the operator lives in this dashboard for hours. Pure black is too aggressive (Grafana's off-white blue-tinted neutrals teach this); pure Inter UI with no mono reads as blog-template. Data in mono (JetBrains Mono Variable), prose in sans (Inter Variable), tabular numerals everywhere. Borders do the work of shadows (Supabase pattern). One accent color (cyan-teal, distinct from Vercel blue and Supabase green) used at ~5-10% of any view (Raycast pattern). Density is Supabase-dense (32-36px rows) with Tailscale-level breathing room in margins. The eye should be able to scan hundreds of requests a minute without fatigue — the signal is the data, not the chrome.

### It moves like: View Transitions for state changes, no decorative animations, perceived-instant response

Zed's ethos: every animation justifies its frame budget. The dashboard uses `@starting-style` + `transition-behavior: allow-discrete` for popovers/dialogs/drawers (zero JS). The cross-page view transition is `@view-transition { navigation: auto }` — two lines of CSS. Row updates flash a 600ms green border via `@property` on the specific row that changed (not a full-table re-render). Motion curve is Linear's (`cubic-bezier(0.16, 1, 0.3, 1)`) at 120-200ms. `prefers-reduced-motion: reduce` collapses everything to 0ms. There is no `@keyframes pulse`. There is no `animate-stagger` chain (the hexos mistake). If you're reaching for Framer Motion, you're solving the wrong problem.

---

## The signature thing about kiroxy

A new user lands on the dashboard for the first time. What do they see in the first 30 seconds that makes them go **"wait, this is different"**?

Candidates considered:

| Candidate | Verdict |
|---|---|
| Command palette as primary navigation | ✅ Adopt — but not the signature (too common post-Linear) |
| Real-time account pool visualization with live refresh countdown | Adopted as ambient element |
| Zero-chrome dashboard — pure data | Adopted as design principle |
| "Single binary in my homelab" feel | Adopted as distribution principle |
| Honest operator ergonomics: every action has a keyboard shortcut | Adopted but not THE signature |
| **The LiveRequestStream as the home page** — reverse-chron block list of requests, each a Warp-style Block, `⌘K` on any block opens per-request actions (replay, attach context, view upstream raw), shareable permalinks per request | **⭐ This is the signature** |

### The LiveRequestStream is the mansion's front door

The kiroxy dashboard's home page is **not a stat grid with six pastel cards**. It's a live reverse-chronological stream of requests arriving as they happen, rendered as Warp-inspired Blocks:

```
┌───────────────────────────────────────────────────────────────┐
│ ● acct-2  claude-sonnet-4-5  POST /v1/messages  200  1.4s     │
│    1,247 in · 389 out · $0.012 · stream · 11:42:18            │
│    ⌘↩ inspect · ⌘C copy ID · ⌘R replay · ⌘L view logs         │
└───────────────────────────────────────────────────────────────┘
```

Three reasons this is the signature:

1. **It's immediately legible as infrastructure telemetry** — a first-time visitor sees live data, not marketing copy. They understand what kiroxy does before they read a word.
2. **It introduces the Block primitive** — and with it, the cmd-click-to-attach pattern, shareable per-request permalinks, the replay workflow. One primitive carries multiple features.
3. **It's zero-decoration** — the data is the page. No stat-grid nostalgia, no "quick-start wizard," no "Tip of the day" card. The operator's time is respected.

Everything else (Accounts list, Routes config, Metrics charts, Settings, Logs) is a navigation target. The LiveRequestStream is the resting state.

**This is the single design decision that distinguishes kiroxy from every Tier E anti-reference.** Homepage, Open WebUI, LibreChat, hexos, and the *arr-stack all default to a stat-grid or tile-grid or service-catalog on their home page. kiroxy's home is a live feed. That's the mansion.

---

## Anti-goals

What kiroxy will NEVER become, expressed as commitments:

- **Multi-provider gateway.** We do not route to OpenAI or Gemini directly. kiroxy speaks Anthropic and OpenAI *shapes* but only routes to Kiro/CodeWhisperer upstream. If you want multi-provider, use LiteLLM.
- **Multi-tenant SaaS.** No per-user quotas with enforcement, no billing, no admin UI for onboarding users via email. kiroxy is single-operator or trusted-pod scope.
- **AI chat interface.** We are not a ChatGPT clone. We are not building a front-end for talking to an LLM. We are infrastructure; a client like Claude Code is the product surface.
- **Marketing site with gradient hero.** The README and the `/` landing page are dev-focused. The dual-mode screenshot + one-paragraph description + copy-paste install. No scroll-triggered testimonials. No product tour. No pricing table (kiroxy is free; if it becomes commercial, that section lives elsewhere).
- **Mobile-first.** kiroxy is designed for a 13-inch+ screen. Responsive down to tablet for incident-response glance-mode, but not optimized for phones. We run on loopback or a tailnet; there is no mobile use case.
- **Integration sprawl.** kiroxy integrates with Kiro, Claude Code, opencode, Cursor, Continue, Zed — via Anthropic and OpenAI API shapes. That's it. We do not add Langfuse, Helicone, Portkey, or OpenRouter as integrations. If you want those, run them next to kiroxy; they speak the same shapes.
- **Subscriber-funded community (Discord, forum, paid support).** kiroxy is OSS. Issues go to GitHub. There is no Discord with #announcements. There is no Patreon.
- **Config GUI for routing rules.** If you want to change routing behavior, you edit a YAML file or export an env var. The dashboard surfaces state; it does not let you write code in a form. (If v2.0 adds declarative routing, the DSL is a config file, not a form builder.)
- **Agent framework.** kiroxy is a proxy. We do not orchestrate agents, we do not implement tool-use loops, we do not spin up sandboxes. The client does that.
- **Collaborative real-time features in v1.x.** No multiplayer cursors, no shared debug sessions, no "invite a friend to look at this request with me." (v2.0 candidate E explores a five-person collaborative mode, but only if it fits without breaking the single-user primary persona.)

---

## 6-month picture (≈ v1.3 ship)

- kiroxy dashboard v3 ships on Svelte 5 + Vite, grounded in DESIGN_SYSTEM.md, with the LiveRequestStream as the home page.
- The command palette replaces the sidebar as primary navigation; sidebar becomes the collapsible Linear-style secondary.
- Per-account drill-down with token-refresh timeline, shareable request permalinks, cmd-click-to-attach-context replay workflow.
- Phase R contextUsageEvent P0 fix shipped (accurate input_tokens); prompt caching shipped (P1 backlog); session stickiness shipped (P1 backlog).
- Grafana dashboard JSON refined based on 6 months of production usage by the operator + trusted pod.
- Onboarder G.2 (age-encrypted credentials), G.3 (batch), G.4 (retry classification), G.5 (polish) all shipped.
- README looks like LibreChat's (dual-mode screenshot + one-paragraph + copy-paste install).
- Docs site at `kiroxy.dev` or similar — or lives in GitHub Pages as a single-file, no-build static site. No VitePress, no Docusaurus.
- First external contributor shipped — doc fix or bug fix, reviewed by the operator.

### Positioning at 6 months

kiroxy is known in a small circle as "the tidy Go proxy for Kiro — the one with the nice dashboard." No marketing budget. All growth is word-of-mouth: one opencode tutorial mentioning `kiroxy opencode-config`, one HN post about the dashboard design, one reference in a Tailscale community homelab thread. Star count ~1-3k. GitHub issues in single-digit weekly cadence, mostly from the operator and two friends.

The operator has something they can link to their GitHub bio without cringing.

---

## 12-month picture (≈ v2.0 ship)

- One of Roadmap v2.0 candidates has shipped (most likely Candidate C: Native MCP server integration, or Candidate D: Privacy-first audit trail).
- kiroxy is the reference implementation for "personal AI infrastructure" — how a single engineer runs their own AI plumbing.
- A sibling project exists (fork, adaptation) — evidence the architecture is sound.
- The dashboard has been featured somewhere design-adjacent (Mobbin-style screenshot collection, Refactoring UI newsletter, a "beautiful Go dashboards" roundup).
- The operator is invited to speak or write about it — a blog post, a podcast appearance, a GopherCon lightning talk.
- v2.0 ships with a public design system JSON schema so operators can fork the look while keeping the engineering.

### Positioning at 12 months

kiroxy is *the* Kiro proxy anyone would recommend. It's opinionated, it's small, it's beautifully rendered. It is explicitly NOT trying to be the general-purpose gateway — that market belongs to LiteLLM and OpenRouter and we stay out. We are the small precise tool for the engineer who already has a Kiro subscription and wants to use it locally.

Star count ~5-15k. One or two production deployments in small companies that share an operator. Operators occasionally ask the primary maintainer to accept a feature that would break the anti-goals — the maintainer says no politely and points at the anti-goals section of this document. That's how a product stays itself.

---

## How this document changes

- Anti-goals: **never without PR + operator review + a reason better than "someone asked for it."**
- Personas: refine based on actual user reports. If the trusted-pod persona never materializes in real feedback, demote it to non-persona.
- Signature thing: the LiveRequestStream is locked for v1.x. If v2.0 changes the signature, this document explains *why* the change improves the product against the personas, not just the features.
- Vibes: refined to match reality. If "Grafana meets Linear meets fly.io" turns out to feel like just "Linear," say so and pick a more honest reference.

**This document is the north star. Every roadmap item should answer: does this move us toward the 6- or 12-month picture for our personas, against our vibes, without crossing our anti-goals?** If not, it belongs in BACKLOG or HALL_OF_SHAME.
