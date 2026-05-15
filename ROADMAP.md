# Roadmap

kiroxy ships in **focused, atomic releases**. Each entry below is a
direction — not a deadline. The single-user, self-hosted scope from the
[philosophy section][phil] of CONTRIBUTING.md is permanent; nothing
here violates it.

[phil]: ./CONTRIBUTING.md#philosophy

---

## Shipped

### v1.4.0 — Public release · 2026-05-15
- Mansion dashboard polish across 7 views (light theme contrast, motion
  budget, pool minimap, empty state warmth, mobile compact mode,
  microinteractions, post-wizard guide)
- Public OSS hygiene (SECURITY, CONTRIBUTING, CODE_OF_CONDUCT, issue +
  PR templates)
- See [CHANGELOG.md](./CHANGELOG.md) for the full list

### v1.3.0 — Mansion polish solo round
- WCAG AA contrast fix (`.faint` token in both schemes)
- Motion budget policy (4 ambient idle-loop animations max)
- Pool minimap density visualization (8×10 cell heatmap)

### v1.2.0 — Dashboard expansion
- All 7 views exposed in topbar (was 3 hidden behind ⌘K)
- Metrics charts + top-N tables + SLO gauges
- Live row click → drawer + time range + sort + CSV export
- Logs volume histogram + multi-level chips + facets
- Settings LOG LEVEL editable + theme switcher
- Tools run history + markdown export
- Models toggle / copy curl / probe

### v1.1.0 — Landing page + initial polish

### v1.0.0 — First production cut
- Anthropic + OpenAI shapes on inbound
- Multi-account pool with stickiness + cooldown + auto-refresh
- SQLite token vault (mode 0600)
- Mansion dashboard MVP

---

## Next — v1.5.x (Q3 2026)

Refinement, not expansion. The product surface is mostly complete.

- **Per-model usage stats in Models view** — count, p50/p95, error rate
  per Anthropic ID. Currently deferred behind `/metrics` parsing.
- **Tools Backup / Restore / Onboarder tab flesh-out** — privileged
  actions need auth + flow design beyond display-only.
- **Models CRUD drawers** — add / edit / delete model entries through
  UI. Currently config-file only.
- **Phase instrumentation** — per-phase latency in DetailDrawer
  (DNS / TLS / TTFB / body / total) instead of synthetic phases.
- **Distributed tracing (OTel)** — already wired, polish exporter UX
  in Settings.
- **Better empty-state for fresh installs** — guided first-account
  flow integrated with onboarder sidecar.

---

## Maybe — v2.0 (no commitment)

Larger structural changes. None of these violate single-user scope —
they make the existing scope better.

- **Pool federation** — share a pool across two laptops you own. Not
  multi-tenant. Not a SaaS. Cryptographically authenticated mesh of
  trusted peers.
- **Embedded Mansion as standalone binary** — ship the dashboard as a
  separate `kiroxy-mansion` binary so you can run the dashboard against
  a remote kiroxy without exposing it.
- **Web Crypto-based vault** — encrypt the SQLite vault with a
  passphrase derived via Argon2id. Currently relies on filesystem
  permissions (mode 0600).
- **Replay / dry-run mode** — record a request, replay against a
  different model or account for A/B testing.

---

## Will not ship

**Permanent non-goals.** PRs implementing these will be politely
closed.

- **Multi-tenant SaaS / hosted gateway** — use [LiteLLM][litellm],
  [Portkey][portkey], or [OpenRouter][or] for that.
- **Multi-provider routing** — kiroxy speaks Anthropic + OpenAI
  *shapes* but only routes to Kiro upstream. Need OpenAI/Gemini? See
  the gateways above.
- **Team collaboration features** (SSO, RBAC, audit logging beyond
  /logs, etc.) — single-user scope.
- **Pay-per-use billing** — defeats the point.
- **Browser extension** — out of scope.

[litellm]: https://github.com/BerriAI/litellm
[portkey]: https://portkey.ai
[or]: https://openrouter.ai

---

## How to influence this list

- **Open a [Discussion][disc]** with the `Ideas` category. Roadmap
  items often start there.
- **File an [Issue][iss]** with the feature_request template if it
  fits the philosophy.
- **Sponsor** if you want to fund a specific direction. Mention it in
  the sponsorship message.

[disc]: https://github.com/nopperabbo/kiroxy/discussions
[iss]: https://github.com/nopperabbo/kiroxy/issues/new/choose

> The roadmap moves when releases ship, not when we promise things.
> If something here doesn't show up for 6 months, that means it didn't
> earn its slot — not that we forgot.
