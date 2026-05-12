# ROADMAP.md — kiroxy

> Trajectory from shipped v1.0 through v2.0's ambitious bet. Every item here answers:
> does this move us toward the `VISION.md` 6- and 12-month picture, against our vibes,
> without crossing our anti-goals?
>
> **Status:** v1.0 drafted 2026-05-13. Updated per release.
>
> **Companion documents:**
> - `docs/VISION.md` — what kiroxy is, who it's for, anti-goals (the filter)
> - `docs/DESIGN_SYSTEM.md` — visual/interaction language that v1.3 rebuild is measured against
> - `research-v3/REFERENCE_GALLERY.md` — the evidence behind design choices
> - `BACKLOG.md` — raw unsorted work; roadmap is the curated slice
> - `CHANGELOG.md` — what actually shipped, tag by tag

---

## v1.0 — shipped (2026-05-12)

Milestones A through M + Phase J + Phase H + Phase I + Phase G.0 + G.1 + G.FIX. See `CHANGELOG.md` and `BUILD_LOG.md` for the full shipped record. Summary of what a user gets today:

- `kiroxy serve` — Anthropic Messages API proxy (`/v1/messages`, `/v1/messages/count_tokens`) with streaming SSE
- `/v1/chat/completions` + `/v1/models` — OpenAI-compatible surface via Phase J translation shim
- Managed token vault (SQLite at `~/.kiroxy/tokens.db`, mode 0600, generation-locked OAuth refresh)
- Multi-source credentials: kiro-cli SQLite, kiro-auth-token.json, Desktop-flow JSON import, triplet import
- Pool with LRU selection, cooldowns, reactive + proactive token refresh
- CLI: `add-account`, `import-accounts(-json)`, `list-accounts`, `remove-account`, `status`, `debug-refresh`, `opencode-config`, `healthcheck`
- Phase H dashboard v1 at `/dashboard` — vanilla JS, no-build, SSE-driven live state
- Dashboard Next experimental alternative at `/dashboard-next` — Svelte 5 + Vite (not the v1.3 target, but proved the stack)
- `/metrics` Prometheus endpoint + starter Grafana dashboard JSON at `docs/METRICS.grafana.json`
- Phase L load-test harness with mock Kiro
- Phase I CI + goreleaser multi-platform release pipeline
- Onboarder (Python sidecar) with 6-layer stealth stack; 65-80% fresh-Gmail reliability per documented band
- Single distroless Docker image (~30MiB, nonroot, read-only FS)

Positioning: **kiroxy works.** It is observably reliable enough for single-operator daily use. It is not yet beautiful enough for the operator to link from a GitHub bio without caveats.

---

## v1.0.1 — bug fixes (2-3 weeks, target end of May 2026)

Purely fixes for issues surfaced during live smoke on v1.0. No new features.

**P0 items (must ship):**
- **Upstream 403 with fresh credentials + new endpoint** — Reqconv payload divergence. Capture outbound body via KIROXY_TAP, diff against minimal-working curl, identify the rejected field. See `BACKLOG.md` for full context. LoC: 30-100.
- **`added_at + expires_in` vs `expires_at` discrepancy** — Phase 2.5 refresh window miscalibrated by up to 1h. Deterministic test with mocked time + reconciliation between `cmd/kiroxy/import_json.go` and `kiro_login.py` output. LoC: 20-50.
- **Workspace profileArn collision in dedupe key** — 2+ Workspace users under one org share profileArn; current logic silently overwrites. Fix option C (capture email in onboarder, propagate through schema, use as dedupe key). LoC: 30-80.
- **`KIROXY_UPSTREAM_URL` env var** — enable Phase L mock_kiro integration testing through the real kiroxy binary. LoC: 10-20.

**P1 items (ship if time permits):**
- Pool-mode token refresher for `source="import-accounts"` accounts (currently only `KIROXY_KIRO_DB_PATH` path has `WithTokenRefresher`). Imported accounts expire after ~1h without refresh support.

**No scope creep.** If a new feature request arrives during v1.0.1, it goes to BACKLOG and ships in v1.1+.

---

## v1.1 — OpenAI + observability polish (1 month, target late June 2026)

Phase J + Phase M + Phase L already shipped infrastructure; v1.1 refines based on operator feedback.

- **Phase R P0: contextUsageEvent accurate input_tokens** — the `meteringEvent` contains real token counts that are more accurate than the tiktoken estimate. Pipe them through to the dashboard + metrics. Closes the "how much did that request actually cost?" question.
- **P1 backlog: Prompt caching** — adopt d-kuro/kirocc's `cache_points` pattern. Signal inbound cache hints to Kiro via the appropriate upstream fields; expose cache-hit-rate on the dashboard.
- **P1 backlog: Session stickiness** — maintain account affinity for a client session to maximize prompt-cache reuse and reduce context-switching costs upstream.
- **Grafana dashboard JSON refinement** — based on production usage by the operator + trusted pod. Drop panels that never lit up; add ones that surfaced gaps (per-account cooldown duration, per-model cost trend).
- **OpenAI follow-ups from Phase J** — `tool_choice: "required"` + function-name forcing (today: 400); `response_format: json_object` passthrough; `stream_options.include_usage` opt-in.
- **Dashboard v1 polish only** — v1.3 is the full rebuild; v1.1 just fixes known v1.0 dashboard regressions (if any), doesn't redesign.

**Gate criterion for v1.1 ship:** operator runs kiroxy as primary Claude Code + Cursor backing for 2 weeks without diagnosing a v1.0 bug.

---

## v1.2 — Onboarder maturity (2 months, target August 2026)

All remaining Phase G work, promoting onboarder from "honest best-effort with stealth" to "configurable batch tool operators actually use."

- **G.2 — Credential encryption.** Current onboarder ingests `--password` via CLI arg or stdin. Replace with age-based encrypted credentials file (passphrase or detached keyfile), Keychain opt-in on macOS. Deliverable: `tools/onboard/credentials.enc` generator/decrypter; `onboard.py --credentials-file credentials.enc --email you@…` path. LoC: 100-200.
- **G.3 — Batch mode with concurrency cap.** `--credentials-file` or `--accounts-file email:password` → N accounts through onboarder in parallel (default 3). fcntl-locked JSON output for concurrent `_upsert`. Reuses G.FIX per-account profile dirs. LoC: 150-250.
- **G.4 — Retry classification.** Transient (network, timeout, Camoufox crash) → retry with backoff; hard (wrong password, Google block, consent declined) → fail fast + `failed_accounts.json` sidecar for G.3 resume. Mirror kikirro's `_classify_error`. LoC: 100-150.
- **G.5 — Polish + progress UI.** tqdm-style progress for batch, per-account status line, README expanded with success/failure tips + retry playbook. LoC: 50-100.
- **Live reliability % targets** documented in `tools/onboard/README.md` with dated measurements. If reliability drops below 50% for fresh-Gmail + residential-proxy + warmed-profile + no-2FA, ship a stealth-layer update within 2 weeks.

**Gate criterion for v1.2 ship:** operator successfully runs `onboard.py --credentials-file creds.enc --batch 5` end-to-end against 5 fresh Gmail accounts on a residential proxy without manual intervention, documented reliability band ≥ 65%.

---

## v1.3 — Dashboard v3 (the signature UI) (3 months, target November 2026)

**The big design push.** Full rebuild of the dashboard surface based on `DESIGN_SYSTEM.md`. This is where kiroxy moves from "works well" to "is something to be proud of."

Stack:
- Svelte 5 + runes (proven in Dashboard Next; keeps the winning choice)
- Vite 6 + TypeScript strict
- Native CSS primitives (OKLCH, `@starting-style`, anchor positioning, subgrid, view transitions) — no CSS-in-JS runtime, no Framer Motion
- `cmdk` (pacocoursey) for the command palette; `sonner` for toasts; no other runtime deps

Feature surface:
- **Home page = LiveRequestStream** (the signature). Reverse-chron stream of requests as Warp-inspired Blocks. SSE-driven. `⌘K` on any block opens per-request action sub-palette. Cmd-click-to-attach-context. Shareable per-request permalinks (`/requests/<id>`).
- **Command palette as primary navigation** (`⌘K`). Two-tier (nav palette + per-item action palette, Raycast-style). Scope sigils (`/` accounts, `#` requests, `>` commands, `@` models, `?` help).
- **Sidebar as secondary, Linear-inverted-L layout.** Collapsible to 56px icon rail. Live counts in section labels (e.g. "Accounts (3 healthy / 1 cooldown)").
- **Per-account drill-down** (right drawer, 560px) with token-refresh timeline, recent requests scoped to that account, live status including Kiro quota remaining.
- **Request replay + debug view.** Select a request, click `⌘R`, replay against a different model or prompt in an isolated sandbox (no effect on prod traffic). Implementation: clone the request, mutate, re-dispatch through a debug pool that cannot touch other accounts.
- **Config inspector** — `kiroxy opencode-config` output rendered as a syntax-highlighted copy-to-clipboard block.
- **Theme system**: Light, Dark (default), Dark Dimmed, High Contrast Dark, High Contrast Light. JSON schema published at `/api/theme/schema.json` for custom theme authoring. Hot-reload via command palette.
- **Density toggle**: Comfortable (default) / Compact via `⌘⇧T`.
- **Full keyboard shortcut map** per `DESIGN_SYSTEM.md §7.2`.
- **View Transitions API** for cross-page navigation.
- **Search DSL** for request log — `model:claude-sonnet status:429 latency:>2s user:alice`. Tailscale-inspired, typeahead on colon.
- **Accessibility audit pass**: WCAG 2.2 AA verified against default themes; AAA on text colors. Screen-reader tested with VoiceOver + NVDA.

Non-goals for v1.3:
- Cost tracking UI (lives in v1.4)
- Multi-node federation (lives in v2.0 Candidate A)
- Declarative routing rules (lives in v2.0 Candidate B)

Gate criterion for v1.3 ship: operator screenshots the dashboard + posts to HN/Twitter with specific pride. Dashboard renders cleanly on the operator's 16" MacBook + a 4K external display + an iPad (glance mode). Dashboard Next gets renamed to the primary and the old `/dashboard` is retired behind a `KIROXY_LEGACY_DASHBOARD=1` env flag.

---

## v1.4 — Cost + observability maturity (4 months, target March 2027)

- **Per-account cost tracking** using the `meteringEvent` tokens from v1.1. Display cost-to-date per account, per model, per day/week/month. Dashboard card + Prometheus gauge.
- **Usage analytics dashboard** — rollup views: top consumers, cost trend, error-rate trend, latency p50/p95/p99. One page, 4-6 charts max, all using the same data flow as the LiveRequestStream.
- **Prometheus integration maturity** — refined dashboards, alert rule examples, docs for operators who want to pipe kiroxy metrics into their existing Grafana.
- **OTel tracing** — kirocc already has the wiring; turn it on behind `KIROXY_OTEL_EXPORTER=otlp`. Default off.
- **Weighted LRU pool** (Quorinex's weight-based expansion) — replaces the simple LRU when per-account tier (Free/Pro/Enterprise) matters for weighting.
- **Race-condition test covering account-swap-mid-stream** — caught in review but not systematically tested.
- **Integration test suite against a real Kiro test account** — the operator maintains one permanent test account; CI runs a smoke subset against it weekly (not per-PR, avoids rate limits).

Gate criterion for v1.4 ship: operator looks at their monthly Amazon bill, looks at the kiroxy cost dashboard, and the two reconcile within 5%.

---

## v2.0 — The Mansion (6-12 months out, target mid-late 2027)

**The ambitious bet.** Five candidates considered; the roadmap commits to ONE headline feature for v2.0, the others either ship as v2.x increments or graduate to a sibling project or get killed.

### Candidate evaluation

Every candidate is measured against `VISION.md`:
- Does it serve the primary/secondary persona?
- Does it break any anti-goal?
- Is it the kind of feature an operator would be proud to open-source?

#### Candidate A — Multi-node kiroxy with federation

**Pitch:** Operator runs kiroxy on their laptop, their homelab NAS, and a small cloud VPS. All three nodes share one account pool via a lightweight gossip/consensus layer. Each node handles requests from its local network; accounts are leased across nodes with eventual consistency. One dashboard federates the view.

**For:** Primary persona's homelab dream. The "I have kiroxy on my laptop at a coffee shop, but when I'm home it seamlessly hands off to my NAS" story is genuinely compelling.

**Against:** Distributed consensus is hard. Current single-binary simplicity is the operator's favorite thing. Risk: we spend 4 months building Raft and the operator prefers `ssh homelab 'kiroxy status'`.

**Verdict:** Strong but risky. If v2.0 picks this, the operator should be prepared to trade simplicity for ambition.

#### Candidate B — Declarative routing rules

**Pitch:** OpenRouter-style body-aware routing: "requests with `model:claude-opus-4-7` go to account pool A; everything else goes to pool B." Rules in a YAML file; hot-reloaded. Kiroxy stays Kiro-only (no other providers) — rules apply within Kiro.

**For:** Solves a real problem for the trusted-pod persona (5-person group routing by caller ID or by model preference).

**Against:** LiteLLM does this for multi-provider. Doing it for single-provider (Kiro) feels like pre-optimization. Risk: we add complexity without a user demanding it.

**Verdict:** Skip as v2.0 headline. Revisit as v2.x minor if trusted-pod persona actually surfaces the need.

#### Candidate C — Native MCP server integration ⭐

**Pitch:** kiroxy becomes an MCP (Model Context Protocol) host. It exposes Kiro as MCP tools that Claude Desktop (or any MCP client) can call. Now kiroxy isn't just a proxy — it's the **bridge between your Kiro subscription and the MCP ecosystem.** Operators can write MCP tools (e.g. "search my codebase," "query my Linear," "refactor this function") and kiroxy routes them through their Kiro quota.

**For:**
- MCP is the 2026-2027 protocol; shipping a native MCP host in the kiroxy space is a defensible position before LiteLLM/Portkey pivot.
- Matches kiroxy's single-operator personality — MCP servers are overwhelmingly per-user.
- Opens the door for third-party operator contributions (each operator contributes an MCP server that speaks to their own tools).
- Doesn't break anti-goals: kiroxy stays Kiro-only; MCP is just another shape (alongside Anthropic and OpenAI).

**Against:**
- MCP surface is still evolving; shipping too early means maintenance burden as spec changes.
- Requires new conceptual load in the dashboard ("what is an MCP tool, and why do I care?").

**Verdict:** ⭐ **Primary recommendation for v2.0 headline.** Strongest alignment with primary persona + OSS contribution opportunity + aesthetic positioning (kiroxy as "personal AI infrastructure").

#### Candidate D — Privacy-first audit trail

**Pitch:** Every request kiroxy handles is logged locally, encrypted at rest (age), queryable via the dashboard. The operator gets perfect recall of "what did my AI see about my codebase this month?" — self-sovereignty over the prompt+response history Anthropic/Amazon could otherwise reconstruct from billing logs.

**For:**
- Differentiated: no competing proxy ships encrypted local history.
- Aligned with "ops tool with taste" tone — dry, technical, self-aware, privacy-conscious.
- Unlocks workflows (replay yesterday's request with today's model; diff two prompts that led to different outcomes; export full history when leaving a job).

**Against:**
- Storage grows without bound; need retention policies.
- Encryption adds operational burden (key management, recovery stories).
- Potentially scary legal implications in some jurisdictions (EU's right-to-be-forgotten if kiroxy's operator is also a data controller).

**Verdict:** Strong v2.0 candidate but narrower appeal than Candidate C. **Recommend as v2.x follow-up after MCP ships.**

#### Candidate E — Collaborative mode (five-person team)

**Pitch:** The secondary persona gets first-class support: kiroxy runs on a shared VPS, each user gets their own API key with per-user quotas + telemetry. End-to-end encrypted access to the shared account pool.

**For:**
- Formalizes the existing secondary persona.
- Could ship as "kiroxy-pro" — paid tier — if the operator ever wants a revenue stream.

**Against:**
- **Crosses the "multi-tenant SaaS" anti-goal.** Even five-person scope starts to creep: "what if it's six?" "what if we add a sixth with different quotas?" "what about audit logs?" This is how LiteLLM's complexity compounded.
- Revenue stream is a value-add for the operator, but changes the OSS contract.

**Verdict:** ⛔ **Reject as a v1.x or v2.0 goal.** If the operator ever wants a commercial tier, it ships as a sibling project (`kiroxy-team` or `kiroxy-pro`) with its own repo, its own docs, its own license. kiroxy-the-OSS-tool stays single-operator.

### v2.0 commitment

**Headline feature: Candidate C — Native MCP server integration.**

- `/mcp` endpoint on kiroxy serving as a local MCP host.
- Dashboard gets an "MCP Tools" section — list of registered tools, per-tool invocation history, per-tool Kiro cost attribution.
- Onboarder adds `kiroxy mcp-add <name> <command>` and `kiroxy mcp-list`.
- Reference MCP servers ship with kiroxy: one for local-filesystem read, one for opencode-config lookup. Each a separate module under `tools/mcp/`.
- Docs: `docs/MCP.md` with operator-focused walkthrough ("run kiroxy as an MCP host for Claude Desktop in 5 minutes").
- Dashboard design: MCP Tools lives as a top-level sidebar item alongside Accounts and Requests. Uses existing Block primitives; no new visual vocabulary.

v2.0 also folds in polish/maturity items that landed during v1.4:
- Declarative config via `kiroxy.yml` (optional; env vars still work).
- First theme fork in the wild — kiroxy ships an official "Ayu" port to seed the ecosystem.
- Docs site graduates from GitHub Pages to a proper static site (Astro or equivalent — still no Docusaurus).

### v2.x increments (post-v2.0)

Candidates B and D slotted in as minor versions if the operator's day-to-day validates them:

- **v2.1** — Candidate D partial: encrypted request archive with retention policy + export. Dashboard gets a "History" tab with date-range + search DSL over past requests.
- **v2.2** — Candidate B partial: body-aware routing rules in `kiroxy.yml` (within Kiro only, no multi-provider). Useful when the trusted-pod persona needs "acct-A for Pro-tier, acct-B for Free-tier" split.
- **v2.3** — Candidate A experimental: two-node federation (laptop ↔ homelab) with a single operator and no consensus (active-standby, fcntl-over-tailnet for account leases). If it works at 2 nodes, 3+ is a bigger v2.x or v3.0 push.

---

## Post-v2.0 — aspirational (v3.0+, 18+ months out)

Pure speculation; reality will correct this list hard.

- **hexos outbound proxy pool** (HTTP/SOCKS5 rotation for VPN users) — the onboarder already has `KIROXY_ONBOARD_PROXY` infrastructure; the proxy path could adopt the same.
- **Context compression** (3-layer cache + AI summary, aliom-v style) — interesting for reducing repeated-context costs but unclear fit for a proxy.
- **Tool Search proxy-side implementation** (kirocc has it; requires Anthropic `defer_loading` support) — depends on Anthropic API evolution.
- **Cloudflare Tunnel integration** — make kiroxy reachable from anywhere the operator's machines are, without opening ports. Fits the "personal AI infrastructure" positioning.
- **Canvas-of-environments metaphor for v3.0 dashboard** — if the multi-node federation ships and operators run 3+ kiroxy nodes, a Railway-style canvas view of "where my accounts live, where my requests flow" could finally justify a canvas. (But only if federation ships. Never adopt canvas first and justify later.)

---

## Infra / release (ongoing across all versions)

Already shipped in v1.0 Phase I:
- GitHub Actions CI (`make gate` + race + coverage on ubuntu-latest + macos-latest Go 1.26.x)
- goreleaser multi-platform binaries + SHA-256 checksums, tag-triggered
- Daily govulncheck workflow

Outstanding:
- **P1 (v1.1):** Dedicated `Dockerfile.release` + goreleaser `dockers:` block for `ghcr.io/<owner>/kiroxy:{{.Version}}` + `:latest`
- **P2 (v1.2):** Homebrew tap + release-workflow glue
- **P3 (v1.3):** Fly.io / Railway / Render one-click deploy guide (even though kiroxy is loopback-primary, a cloud deploy path seeds the "remote access via Tailscale" story)
- **P3 (v1.3):** HTTPS via Caddy sidecar config + `docker-compose.caddy.yml`
- **P3 (v1.4):** Strict CI lane collapse — merge `ci.yml` + `vuln.yml` into a single workflow with strict as a matrix cell
- **P3 (v2.0):** Signed releases (cosign) for reproducibility-conscious users

---

## Security hygiene (continuous)

- **Chmod tokens.db to 0600 on Open** — enforce mode explicitly (hexos-inherited pattern). Fold into any vault touch PR.
- **Audit-zeroize previousRefreshToken on TTL** — rotate out after N generations to limit blast radius.
- **Rate limit inbound** (token bucket per API key) — ship when the trusted-pod persona surfaces the need, not before. Single-user doesn't.

---

## Anti-goals, re-stated (filter every future work item)

A future work item is **rejected** if it:
- Adds a non-Kiro upstream provider
- Adds multi-tenant auth (RBAC, SAML, SCIM, per-user billing)
- Adds a chat interface on the dashboard
- Adds a gradient hero to the README or landing
- Adds a pricing table
- Breaks the single-binary distribution
- Requires a JS runtime dependency beyond Svelte 5 + Vite + cmdk + sonner without operator sign-off
- Adds a decorative animation
- Crosses the "kiroxy is personal AI infrastructure" framing into "kiroxy is a gateway"

These aren't suggestions; they're commitments. Features pass through this filter before entering any version's scope.

---

## How this document changes

- **Per release**: amend with the actually-shipped scope + the actually-gated criteria. What slipped? What accelerated?
- **Per quarter**: review the v2.0 candidates; if reality diverged (e.g. MCP ecosystem stalled), rebalance.
- **Never**: add a feature without tracing it to VISION.md personas + vibes + anti-goals.

**The roadmap is a filter, not a wishlist.** Every item earns its slot by serving the operator (primary persona) and honoring the anti-goals. If a PR has a feature that doesn't map to this document, either the document is wrong (amend via PR) or the feature is wrong (close as "doesn't fit").
