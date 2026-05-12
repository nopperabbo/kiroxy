# INFORMATION_ARCHITECTURE.md — kiroxy

> Every screen, flow, URL, and state in the kiroxy dashboard. The map Track 3
> builds from; the grid a future maintainer consults when adding a feature.
>
> **Status:** v1.0 drafted 2026-05-13.
>
> **Companion documents:**
> - `docs/VISION.md` — who this is for; signature LiveRequestStream
> - `docs/DESIGN_SYSTEM.md` §4 (layout chassis)
> - `docs/INTERACTION_PATTERNS.md` — decision trees that back each screen
> - `docs/components/*.md` — per-primitive specs
> - `docs/KEYBOARD_SHORTCUTS.md` — nav chords reference below

---

## 1. Screen inventory

Three levels of navigational depth. Everything reachable from Level 1 via
either sidebar click, `g`-chord, or command palette.

### Level 1 — Primary navigation (five routes)

| Screen | Path | Signature content |
|---|---|---|
| **Home** | `/dashboard-mansion` | LiveRequestStream (`docs/components/live-request-stream-block.md`) — THE signature. |
| **Accounts** | `/dashboard-mansion/accounts` | Pool table (one row per account). |
| **Requests** | `/dashboard-mansion/requests` | Historical request log with DSL filter. Separate from LiveRequestStream (history, not live). |
| **Metrics** | `/dashboard-mansion/metrics` | Grafana-style panels (sparklines, heatmaps). |
| **Settings** | `/dashboard-mansion/settings` | Config inspector, env var explorer, theme + density. |

**Explicit non-inclusions at Level 1:**
- No "Dashboard" AND "Home" alias — they are the same. Sidebar labels "Home"; code route is `dashboard-mansion`.
- No "Logs" separate screen. Logs are a filtered view of Requests + Metrics panels. Operators type `g l` → it routes to `/dashboard-mansion/requests?stream=true`.
- No "Routes" configuration UI — declarative YAML, see `VISION.md` anti-goal.

### Level 2 — Drill-down drawers (four entities)

Drawers never lose list context (see `INTERACTION_PATTERNS.md` §1).

| Drawer | Opened from | URL (shareable) |
|---|---|---|
| **Account detail** | Accounts row click OR LiveRequestStream block's account link | `/dashboard-mansion/accounts/{id}` |
| **Request detail** | LiveRequestStream block click OR Requests row | `/dashboard-mansion/requests/{id}` |
| **Refresh-event detail** | Account timeline row click (nested) | `/dashboard-mansion/accounts/{id}#event-{ulid}` |
| **Replay workspace** | `⌘R` on a focused Request block | `/dashboard-mansion/requests/{id}/replay` |

Each Level-2 URL is a valid deep-link. Hitting it cold renders the Level-1
parent with the drawer pre-opened (SSR + client hydrate).

### Level 3 — Modals (three kinds)

| Modal | Trigger | Purpose |
|---|---|---|
| **Import accounts JSON** | Settings → Import OR empty-state CTA OR palette | Dialog `data-size="wizard"` with JSON paste + validation preview. |
| **Confirm destructive** | "Drop account", "Drop pool", "Reset vault" | Dialog `data-intent="destructive"` with typed confirmation. |
| **Command palette** | `⌘K` from anywhere | Not technically a modal — native `<dialog>` in top layer. See `docs/components/command-palette.md`. |

**No modals at Level 3 that don't fit one of these three kinds.** If you're
tempted to add a "Wizard walkthrough" modal, put it in a Drawer or a Route.

---

## 2. URL structure

Fully enumerated:

```
/dashboard-mansion                                Home (LiveRequestStream)
/dashboard-mansion/accounts                       Accounts table
/dashboard-mansion/accounts/{id}                  Account drawer open (deep-link)
/dashboard-mansion/accounts/{id}#event-{ulid}     Account drawer, scrolled to timeline event
/dashboard-mansion/accounts/{id}?edit=label       Account drawer with edit-label dialog
/dashboard-mansion/requests                       Requests history
/dashboard-mansion/requests/{id}                  Request drawer (inspect)
/dashboard-mansion/requests/{id}/replay           Request drawer with replay workspace
/dashboard-mansion/metrics                        Metrics panels
/dashboard-mansion/metrics?range=1h|24h|7d|30d    Metrics with time-range param
/dashboard-mansion/settings                       Settings
/dashboard-mansion/settings?section=theme         Settings scrolled to section
/dashboard-mansion/help                           Keyboard shortcut cheatsheet route (also `?` overlay)
```

### Query-param grammar

- `?q={dsl}` — DSL filter. URL-encoded. Example: `?q=status%3Ahealthy+tier%3Apro`.
- `?sort={column}.{asc|desc}[,...]` — multi-column sort.
- `?range={1h|24h|7d|30d|custom}&from={iso}&to={iso}` — time range.
- `?stream=true` — Requests view switches to live-stream mode.
- `?theme={light|dark|dark-dimmed|system}` — temporary theme override.

All params are **bookmarkable** and **server-respected**. Don't hide state
in localStorage when it could live in the URL.

### Route-level params vs localStorage

| Goes in URL (shareable) | Goes in localStorage (user preference) |
|---|---|
| Filter DSL (`?q=...`) | Theme preference |
| Sort state | Sidebar collapsed vs expanded |
| Time range | Density mode |
| Drawer open state | Recent palette items |
| Modal open state | Saved filter sets (named) |

---

## 3. Navigation model

### 3.1 Primary — Linear "inverted L"

See `docs/DESIGN_SYSTEM.md` §4.4 for the ASCII diagram. The kiroxy default is
**sidebar + thin breadcrumb**, not top-nav. kiroxy has 5 Level-1 sections;
horizontal nav was rejected because the footer still needs more chrome
(status indicator, token-refresh-in-flight, version).

**Sidebar contents (top-to-bottom):**

1. kiroxy wordmark + version (mono).
2. Search / `⌘K` affordance (a button styled like an Input, clicking opens palette).
3. Nav items:
   - `Home` (`g h`)
   - `Accounts (3 / 1 / 0)` — live counts `{healthy}/{cooldown}/{disabled}` in the label (Homepage-pattern but applied to OUR data, not third-party integrations).
   - `Requests` (`g r`)
   - `Metrics` (`g m`)
   - `Settings` (`g s`)
4. Divider.
5. Sidebar footer:
   - Token-refresh-in-flight status indicator (shows which account is refreshing; empty when idle).
   - Theme toggle (cycles via `⌘/`).
   - Connection status (●  Live or ● Reconnecting or ● Offline).

Collapsed rail (56px): just the icons + live-count badge on Accounts.

### 3.2 Breadcrumb bar (top of content)

Replaces a top-nav. Shows: `Accounts → acct_01H8… → Refresh history`. Each
segment is a link. Far right: contextual actions (usually 1-2 buttons + `⌘K`
hint).

Height: 40px (see `tokens.css` → `--layout-breadcrumb-height`). Thin; no box
shadow; separated from content by a 1px border.

### 3.3 Modal URL pattern (shareable drill-down)

When a drawer opens, the URL updates via `history.pushState` to the
Level-2 deep-link. Closing the drawer with `Esc` or back-button returns to
the Level-1 URL. A teammate pasted the deep-link into chat loads it cold;
the server serves the Level-1 skeleton + drawer pre-hydrated.

### 3.4 Focus-return on Esc

Per `INTERACTION_PATTERNS.md`:
- `Esc` on palette → close, focus invoker.
- `Esc` on dialog → close, focus invoker.
- `Esc` on drawer → close, focus row that opened it.
- `Esc` on tooltip → dismiss.
- `Esc` inside input → clear if non-empty, blur if empty.

The cascade is top-down: popover → tooltip → dialog → drawer → palette.
Pressing `Esc` with multiple overlays stacked closes the outermost first.

---

## 4. State persistence

| Key | Where | Scope | Lifetime |
|---|---|---|---|
| `kiroxy:theme` | localStorage | Whole app | Until user changes |
| `kiroxy:density` | localStorage | Whole app | Until user changes |
| `kiroxy:sidebar-collapsed` | localStorage | Whole app | Until user changes |
| `kiroxy:palette-recents` | localStorage | Palette | 30 most-recent; FIFO |
| `kiroxy:saved-filters` | localStorage | Per-view | Named sets; user-managed |
| Filter DSL | URL `?q=` | Per-route | URL lifetime |
| Sort state | URL `?sort=` | Per-route | URL lifetime |
| Time range | URL `?range=` | Per-route | URL lifetime |
| Drawer open | URL path segment | Level 2 | URL lifetime |
| Auth cookie / API key header | HttpOnly cookie | Whole app | Session (see server) |

**Never** store real data in localStorage. Only UI preferences + palette recents.

---

## 5. Empty states per screen

### 5.1 Home (LiveRequestStream)

| Variant | When | Shown |
|---|---|---|
| `first-time` | No accounts imported | CLI snippet: `kiroxy add-account --refresh-token=...` + doc link |
| `no-traffic` | Accounts exist but no requests in buffer | "No requests in the last 60 minutes. Point Claude Code at `http://127.0.0.1:8787` to see activity." + `Snippet` |
| `filtered` | User filtered down to zero | "No blocks match `status:429`. Clear filter?" |
| `error` | SSE connection failed after 3 retries | Banner at top of feed + cached last state + Retry button |

### 5.2 Accounts

| Variant | When | Shown |
|---|---|---|
| `first-time` | No accounts | CLI snippet for `kiroxy add-account` |
| `all-cooldown` | Every account in cooldown | Countdown until first account resumes + `debug-refresh` snippet |
| `all-disabled` | Every account disabled | Explainer: "All accounts are disabled. Enable one from settings or import fresh." |
| `filtered` | DSL returned zero | Clear filter |
| `error` | Can't load pool | Retry + contact server logs link |

### 5.3 Requests

| Variant | When | Shown |
|---|---|---|
| `first-time` | No requests in history DB | "Request history appears here after traffic hits kiroxy. Historic data retains 7 days." |
| `filtered` | DSL returned zero | Clear filter |
| `range` | Time range returned zero | "No requests in the last {range}. Try a wider range." |
| `error` | Can't load history | Retry |

### 5.4 Metrics

| Variant | When | Shown |
|---|---|---|
| `first-time` | No metric history | "Metrics populate after 60 seconds of operation." |
| `error` | Can't load panels | "Metrics backend unreachable. Check `/metrics` endpoint." |

### 5.5 Settings

No empty states needed — settings always have current values to display.

---

## 6. Responsive behavior

kiroxy is desktop-first. Per `VISION.md` anti-goal: NOT mobile-first.

| Viewport | Behavior |
|---|---|
| `≥ 1280px` | Primary design target. Full sidebar (240px) + content + optional drawer side-by-side. |
| `≥ 1024px` | Sidebar stays expanded. Drawer overlays content instead of splitting when opened. |
| `≥ 768px` | Sidebar collapses to 56px icon rail. Drawer overlays. Tables retain all columns. |
| `< 768px` (mobile) | Graceful not-supported message: "kiroxy's dashboard is designed for tablet and larger. Run `kiroxy status` or see `/healthz`." — with link to docs. Nothing else. |

No phone UI; no mobile-optimized tables; no swipe gestures. Operators run
kiroxy on laptop or desktop. The CLI handles the phone use case.

---

## 7. Progressive disclosure

Information depth, Level 1 → Level 3:

1. **Level 1** — just the data. No edit affordances inline; the row is the row. Drill-down on Enter.
2. **Level 2** — full detail of one entity. Edit controls visible. Related data (timeline, associated requests) nested.
3. **Level 3** — actions that need confirmation. Destructive things only.

Operators should be able to answer 90% of their questions at Level 1 without
drilling. Drilling is for debugging, not for daily monitoring.

---

## 8. Help and discoverability

- **`?` cheatsheet** — always available; renders `KEYBOARD_SHORTCUTS.md`.
- **Tooltips** — on every icon-only button.
- **Palette footer** — rotating tips ("Type `/` to filter accounts", "Press `⌘K` on a row to see actions", "Press `?` for shortcuts").
- **Empty-state CTAs** — teach via CLI command, not via "Click here to get started".
- **docs link** — in the sidebar footer, always reachable.
- **Error messages** — `{problem} — {cause} — {action}` format per `DESIGN_SYSTEM.md` §11.

---

## 9. Route specification for Track 3

Track 3's router must expose these routes exactly:

| Method | Path | Handler behavior |
|---|---|---|
| GET | `/dashboard-mansion` | Home skeleton + SSR LiveRequestStream initial state |
| GET | `/dashboard-mansion/accounts` | Accounts table skeleton + SSR pool snapshot |
| GET | `/dashboard-mansion/accounts/:id` | Accounts + drawer pre-opened for `:id` |
| GET | `/dashboard-mansion/requests` | History skeleton + initial page |
| GET | `/dashboard-mansion/requests/:id` | Requests + drawer pre-opened for `:id` |
| GET | `/dashboard-mansion/requests/:id/replay` | Requests + drawer with replay view |
| GET | `/dashboard-mansion/metrics` | Metrics panels |
| GET | `/dashboard-mansion/settings` | Settings |
| GET | `/dashboard-mansion/help` | Cheatsheet content |
| GET (SSE) | `/dashboard-mansion/api/stream` | LiveRequestStream events |
| GET (SSE) | `/dashboard-mansion/api/pool` | Pool state changes |
| GET | `/dashboard-mansion/api/state` | Polling fallback |
| POST | `/dashboard-mansion/api/import` | Import accounts |
| DELETE | `/dashboard-mansion/api/accounts/:id` | Drop account |
| GET | `/dashboard-mansion/api/opencode-config` | Emit opencode snippet |
| GET | `/dashboard-mansion/assets/{path...}` | Vite-bundled assets |

**Rule:** every page URL is a first-class navigable state. Browsers Back/Forward
move between them without losing position.

---

## 10. Anti-patterns

- ❌ **Hash-only routing** (`/#/accounts`). Breaks SSR + breaks server-side
  auth; use real paths.
- ❌ **Modal over modal.** If a confirm needs a confirm, redesign.
- ❌ **Drawer that doesn't deep-link.** Every drawer gets a URL.
- ❌ **"Back" button inside the page chrome.** Browsers have it.
- ❌ **Sidebar with "last session restore" animations.** Sidebar restores
  collapsed-vs-expanded instantly from localStorage; no animation.
- ❌ **Tabs inside a drawer inside a route.** Limit of three nesting levels.
- ❌ **Dashboard that requires login on loopback.** See `VISION.md` — single
  user + `KIROXY_API_KEY` optional on `127.0.0.1`. Login flow only appears
  when `KIROXY_BIND=0.0.0.0` + key configured.
