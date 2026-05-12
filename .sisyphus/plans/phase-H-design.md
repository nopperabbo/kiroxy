# Phase H — Operator Dashboard v2 (Design Doc)

**Date:** 2026-05-12  
**Target:** land in current session (~4h wall), tag later by user.  
**Scope:** replace v1 dashboard HTML served at `GET /dashboard` with a mature, dense, operator-focused view with live SSE updates, command palette, and import UI.

---

## Non-goals / strict boundaries

- **No touch** to `internal/pool/*`, `internal/auth/*`, `internal/tokenvault/*`.
  Phase 2.5 token-refresh work may be in flight concurrently on these packages.
  ZERO collision tolerance means no additive changes either. We use only the
  already-exported APIs (`Pool.List`, `Pool.Remove`, `Vault.Save`, `Vault.Delete`,
  `Vault.ListByProvider`).
- **No touch** to `cmd/kiroxy/main.go` dispatch. No new CLI subcommands.
- **No new Go dependencies.** Standard library only, plus what's in go.mod today.
- **No runtime CDN fetches.** Everything goes through `go:embed`.
- **No new version tag.** Tagging is operator-controlled post-review.

---

## Aesthetic posture (defend-in-commit)

- **Dark-default, light-optional.** Developer tool; dark is first-class. Light
  mode must look intentional (different palette, not inverted).
- **Monospace UI for data; sans for prose.** Engineers read tables of tokens,
  IDs, timestamps — monospace is the correct typeface. Sans survives only in
  the command palette hint text and the empty-state copy.
- **Density over breathing room.** htop / Grafana panel target. Information
  hierarchy via weight + color + spacing, not boxes-and-shadows.
- **Two functional colors + neutrals.** Accent (indigo-mono, brand-ish), success
  (green), warn (amber), danger (red). No pastel palette.
- **Keyboard-first.** `cmd-k` opens palette, `/` focuses search, `j/k` scroll
  rows, `esc` closes overlays, `?` shows hotkey cheat sheet. Shortcuts labeled
  visibly in the UI (not hidden in a menu).
- **Meaningful motion only.** No decorative animations. Numbers tick, drawer
  slides, state pills transition — that's it.

---

## Tech stack decision (with rationale)

| Decision | Rationale |
|---|---|
| **Go `html/template`** with `go:embed` | Same pattern we already use for v1 HTML. Zero new tools. |
| **Single embedded HTML shell** + JSON-over-SSE + vanilla JS DOM updates | Avoids Alpine+htmx entirely (saves ~60KB of vendored code for features we don't need). Our "views" are panels toggled by JS; no partial-swap use case that justifies htmx. Single-user tool; no SPA routing complexity. |
| **No Tailwind, no CSS-in-JS, no utility classes** | Tailwind CDN banned by brief. Local Tailwind would add a build step for ~6 screens; custom CSS with design tokens is mature and avoids utility-soup in templates. |
| **Design tokens via CSS custom properties** | First-class dark+light mode via `:root[data-theme]` override. `prefers-color-scheme` honored by default. |
| **SSE over `GET /dashboard/api/stream`** | Real-time updates without polling. 2s heartbeat snapshot + per-request events. |
| **No animation libraries** | Explicit brief requirement. CSS transitions for state pills only. |
| **Inline SVG icons (hand-authored)** | No Heroicons/Feather import. We need 8 icons; embedded as `<symbol>` in a single SVG sprite. |
| **Zero `go get`** | All functionality uses stdlib. |

---

## Type scale (1.125 ratio, base 13px for data density)

```
--font-mono: ui-monospace, 'SF Mono', Menlo, Monaco, 'JetBrains Mono', Consolas, monospace;
--font-sans: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Inter', sans-serif;

--text-xs:   11px  / 16px  line-height
--text-sm:   12px  / 18px
--text-base: 13px  / 20px   (tables, forms, most UI)
--text-md:   14px  / 22px
--text-lg:   16px  / 24px   (section headings)
--text-xl:   18px  / 26px
--text-2xl:  22px  / 30px   (one hero-number)
```

Spacing: 4px base → `2 4 8 12 16 24 32 48`.

---

## Color palette (developer terminal vibe, not pastel-SaaS)

### Dark (default)
```
--bg:        #0A0A0B      near-black page bg
--surface:   #111113      panel bg
--surface-2: #16161A      hover / nested panel
--border:    #1F1F23      subtle divider
--border-strong: #2A2A30  emphasized divider
--text:      #E6E6E7
--text-dim:  #8B8B92      muted copy
--text-faint:#5A5A62      placeholder, empty states
--accent:    #8B95F5      indigo-mono for interactive emphasis
--accent-dim:#4B56B3
--success:   #3DD68C      healthy / 200
--warn:      #F3B34C      cooldown
--danger:    #E85D5D      errors
--focus:     #7C8CFF      focus ring
```

### Light (opt-in)
```
--bg:        #FAFAFA
--surface:   #FFFFFF
--surface-2: #F3F3F5
--border:    #E5E5E7
--border-strong: #CFCFD3
--text:      #111113
--text-dim:  #5A5A62
--text-faint:#8B8B92
--accent:    #3B48CC
--accent-dim:#7780E0
--success:   #18A560
--warn:      #C47A0F
--danger:    #C93A3A
--focus:     #3B48CC
```

Both modes: tuned for WCAG AA contrast (4.5:1 on body text, 3:1 on large).

---

## Information architecture

```
┌─────────────────────────────────────────────────────────────────┐
│  KIROXY · v0.3.0 · uptime 2h14m    ● ready    127 req · 0.8% err│  top bar
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ACCOUNTS (3)                                   + import  ⟳     │  section header
│  ┌─────────────────────────────────────────────────────────┐    │
│  │ ID         status     req     err    cooldown   last   │    │
│  │ acct-1  ● healthy     47      0      —          11:42  │    │  account rows
│  │ acct-2  ● cooldown    91      3      1m 45s     11:40  │    │
│  │ acct-3  ● disabled    —       —      —          —      │    │
│  └─────────────────────────────────────────────────────────┘    │
│                                                                 │
│  RECENT REQUESTS (live)                                         │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │ 11:42:18  claude-sonnet-4-6   acct-1   200   1.4s  sse │    │  request rows
│  │ 11:42:03  claude-opus-4-7     acct-2   200   2.8s  sse │    │
│  │ 11:41:58  claude-sonnet-4-6   acct-1   429   0.2s      │    │  (SSE-pushed, ring-50)
│  └─────────────────────────────────────────────────────────┘    │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
  footer:  ⌘K palette · / search · ? keys · theme: auto | dark | light
```

Overlays (not separate pages):
- **Cmd-K palette** — full-screen overlay, fuzzy-search accounts + requests +
  nav actions.
- **Account drawer** — right-side slide-in; shows full account metadata on row
  click.
- **Import modal** — drag-drop / paste JSON; validates client-side, posts to
  import endpoint.
- **Hotkey cheat sheet** — keyboard `?` trigger.

---

## Feature scope (locked)

### P0 — must ship

1. **Account pool table** with: ID, status pill (healthy/cooldown/disabled),
   requests, errors, cooldown countdown, last-used timestamp. Sortable
   headers. Live-updated via SSE every ~2s.
2. **Recent requests feed** — ring buffer (last 50) of `/v1/messages` and
   `/v1/messages/count_tokens` requests. SSE-pushed on each request. Fields:
   timestamp, method, path, status, latency, stream-vs-not.
3. **System health top bar** — version, uptime, total requests (5m window),
   error rate, vault OK, pool readiness.
4. **Command palette (cmd-k)** — fuzzy search over accounts + request history,
   plus commands (Import, Refresh state, Toggle theme, Copy API key hint,
   Open docs).
5. **Import accounts UI** — modal with drag-drop or paste. Validates
   `kiroTokenEntry[]` shape client-side, POSTs to `/dashboard/api/import`.
6. **Remove account** — per-row menu; confirm-click removes from vault + pool.

### P1 — fit in remaining time

7. **Account drill-down drawer** — right-side slide-in with full metadata:
   provider, source, generation, updated-at, refresh-in-progress, previous
   tokens count. Read-only.
8. **opencode config inline** — modal showing the exact JSON the
   `kiroxy opencode-config` subcommand would emit, generated server-side.
9. **Log stream** — tail of structured JSON logs (needs a log-sink interface
   wired into the slog handler). This one is risky under time budget; may
   cut.

### P2 — defer, documented in BACKLOG

- Per-account force-refresh (requires pool API addition → phase 2.5 owner).
- Per-account disable/enable (requires `pool.SetEnabled` → phase 2.5 owner).
- Historical metrics charts (needs TSDB).
- Multi-user auth.
- Onboarder trigger (needs Phase G.2+).

---

## Files & routing plan

### New files
```
internal/server/
  dashboard_v2.go         v2 HTTP handlers (state, stream, import, accounts CRUD)
  dashboard_v2_test.go    handler tests
  dashboard_sink.go       RequestRecorder interface + in-memory ring buffer
  dashboard_sink_test.go  ring buffer tests
  ui/
    index.html            single-file HTML shell (go:embed)
    tokens.css            design tokens (dark + light)
    app.css               component styles
    dashboard.js          SSE + DOM + hotkeys + palette + import (vanilla)
    icons.svg             hand-authored SVG sprite
```

### Modified files
```
internal/server/dashboard.go   add new routes, keep v1 state endpoint shape
internal/server/server.go      wire RequestRecorder → logging middleware
internal/server/logging.go     call recorder on every request finish
cmd/kiroxy/dashboard.go        extend provider for import + remove + requests
```

### Routing (new endpoints prefixed `/dashboard/api/`)
```
GET  /dashboard                       HTML shell (replaces v1 content)
GET  /dashboard/api/state             JSON snapshot (backward-compatible keys preserved)
GET  /dashboard/api/stream            SSE stream of snapshots + request events
POST /dashboard/api/import            body: kiroTokenEntry[]; validates + saves
DELETE /dashboard/api/accounts/{id}   remove account from pool + vault
GET  /dashboard/api/opencode-config   returns opencode snippet JSON (uses existing generator)
GET  /dashboard/assets/{file}         static assets (css, js, svg); no-path-traversal
```

Auth: all inherit the existing `/dashboard`-prefix loopback-bypass + API-key
middleware. Non-loopback access requires API key.

---

## Backward compatibility

The brief mandates that `/dashboard/api/state` shape stays compatible. I will:
- Keep every existing JSON key (`version`, `uptime_s`, `ready`, `ready_detail`,
  `vault_ok`, `vault_path`, `accounts[]`, `account.id`, `account.enabled`,
  `account.requests`, `account.errors`, `account.cooldown_until`, `account.last_error`).
- Add new fields alongside (`total_requests_5m`, `error_rate_5m`,
  `account.last_used`, `account.provider`, `account.region`, etc.).
- Never remove existing fields this session.

External consumers of `/dashboard/api/state` continue to parse successfully.

---

## Test plan

| Test | File | What it exercises |
|---|---|---|
| v2 HTML served at `/dashboard` | `dashboard_v2_test.go` | GET returns HTML shell with CSP-safe markers |
| assets served with correct Content-Type | `dashboard_v2_test.go` | `/dashboard/assets/app.css` returns text/css; nonexistent returns 404 |
| state JSON backward-compatible | `dashboard_test.go` (kept) | existing test stays green |
| SSE stream emits snapshot on connect | `dashboard_v2_test.go` | first event within 1s |
| SSE stream emits request event on /v1/messages | `dashboard_v2_test.go` | integration; uses existing stubKiroClient |
| Import: valid JSON array adds accounts | `dashboard_v2_test.go` | vault.Save called; response count matches |
| Import: bad shape returns 400 with per-entry detail | `dashboard_v2_test.go` | |
| DELETE account removes from vault + pool | `dashboard_v2_test.go` | |
| Non-loopback without key → 401 | `dashboard_v2_test.go` | auth inheritance check for each new route |
| Ring buffer: cap enforced, FIFO eviction | `dashboard_sink_test.go` | |
| Ring buffer: concurrent writes safe | `dashboard_sink_test.go` | -race |

---

## 15-minute checkpoints (4h budget)

- **0:00–0:15**  Design doc + approve self. **(this file)**
- **0:15–0:45**  Ring buffer (`dashboard_sink.go`) + wiring through logging
  middleware. Tests. Commit.
- **0:45–1:15**  Scaffold v2: new handlers, route registration, empty HTML
  shell, design tokens CSS, basic asset serving. Tests. Commit.
- **1:15–1:45**  Account pool table + SSE snapshot stream. Live updates
  driven by vanilla JS. Commit.
- **1:45–2:15**  Recent requests feed via SSE per-request events. Commit.
- **2:15–2:45**  Command palette + keyboard shortcuts + theme toggle. Commit.
- **2:45–3:15**  Import accounts UI (modal + drag-drop + server endpoint).
  Commit.
- **3:15–3:30**  Remove account per-row action. Commit.
- **3:30–3:45**  P1 drill-down drawer + opencode config modal if time allows.
- **3:45–4:00**  Docs (README note, OVERNIGHT_LOG Phase H entry, BACKLOG close
  Dashboard v2 line). Self-review. Final commit.

Hard stop at 4h; if P1 not landed, note in OVERNIGHT_LOG under "Known rough edges".

---

## Rough-edge acknowledgment upfront

Going in with eyes open on these:

- **No force-refresh button.** Requires a Pool method we can't add under
  concurrency constraints. Users can trigger a refresh today by letting a
  request hit the account.
- **No per-account disable/enable toggle.** Same reason. Workaround: remove
  + re-import.
- **Request log is in-memory only.** Ring buffer evaporates on restart; no
  persistence. Matches the tool's "personal dashboard" posture; historical
  metrics are a known P2.
- **No websocket.** SSE is one-way, which is fine for a dashboard; writes go
  through regular `POST`/`DELETE`. Simpler to reason about.
