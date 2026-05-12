# INTERACTION_PATTERNS.md — kiroxy

> Decision trees and canonical flows for every interaction in the kiroxy dashboard.
>
> This document specifies HOW components combine. Individual primitives live in
> `docs/components/*.md`; this file explains when to use which, what to
> announce, and how timing integrates.
>
> **Status:** v1.0 drafted 2026-05-13.
>
> **Companion documents:**
> - `docs/DESIGN_SYSTEM.md` — principles, tokens, motion
> - `docs/KEYBOARD_SHORTCUTS.md` — canonical shortcut map
> - `docs/components/*.md` — per-primitive specs
> - `docs/VISION.md` §signature-thing — LiveRequestStream is the home page

---

## 1. Modal vs Drawer vs Popover vs Route — decision tree

```
Does the content require user response before continuing?
├── YES ──► Does losing the list/page context matter?
│           ├── NO  ──► Dialog (see components/dialog.md)
│           └── YES ──► Drawer (right slide-in; list stays visible)
└── NO ───► Is the content interactive (buttons, forms, nested links)?
            ├── YES ──► Popover (menu or form variant — §7.6)
            └── NO  ──► Tooltip (read-only hint, 500ms delay)
```

**Additional filter (layered on top of above):**

- Does the content deserve its own shareable URL?
  → Yes ⇒ **Route** (full page). e.g. `/requests/{id}` is a route, not a drawer.
  → No ⇒ pick per the tree above.
- Is the content multi-step (wizard)?
  → Yes ⇒ Dialog (`data-size="wizard"`) or Route. Never Drawer — users lose stepper context on resize.

**Default bias: DRAWER for inspection, DIALOG for commitment.** The list context is the operator's orientation anchor — keep it visible when possible. Dialogs interrupt; reserve for decisions.

**Examples:**

| Intent | Choice | Why |
|---|---|---|
| Inspect a request from the stream | Drawer | List context matters; no commitment |
| Confirm "Drop account" | Dialog (destructive variant) | Hard irreversible action; list blur is fine |
| Edit account label | Dialog (simple) | Commitment required; quick |
| "What is this cooldown duration?" | Tooltip | Read-only, 500ms delay |
| "Actions for acct-2" | Popover menu | Interactive, <7 items |
| "Request detail (shareable)" | Route `/requests/{id}` AND Drawer | Drawer for inline drill; Route for permalink |

---

## 2. Command palette invocation + behavior

### 2.1 Opening

- `⌘K` / `Ctrl+K` — global; opens root palette regardless of current focus.
- `/` — focuses the primary search input for the current view (scoped). Does NOT open the palette.
- `⌘K` while cursor is in a text input — STILL opens the palette (overrides native `⌘K` link insertion). This is the Superhuman discipline.
- Inside palette, typing `>` `/` `#` `@` `?` at position 0 enters a scope.

### 2.2 Fuzzy match algorithm (`fuzzysort`-style)

Scoring priority (higher = ranked higher):

1. **Exact prefix match on label.** Score 1000 × `(matched_chars / total_chars)`.
2. **Prefix match on any word boundary** (camelCase or space). Score 800.
3. **CamelCase initial match** (typing "grc" matches "GoRequestCount"). Score 600.
4. **Contiguous substring match.** Score 400.
5. **Character-subsequence match** (each query char appears in order). Score 200.
6. **Recents boost.** +150 if item used in last 5 sessions.
7. **Frecency boost.** +50 × log(uses) for items invoked > 3 times.

Ties broken by most-recent-use, then alphabetical.

**No magic weights beyond this.** Document the algorithm in a `docs/palette-ranking.md` when v1.3 ships.

### 2.3 Two-tier navigation

**Root palette (`data-tier="root"`):**
- Navigation (Go to Accounts, Go to Requests, …)
- Global actions (Refresh all accounts, Open settings, …)
- Recents
- Help (`?` scope)

**Item palette (`data-tier="item"`):**
- Triggered by `⌘K` on a focused row/block.
- Scope is the selected entity (account, request, model).
- Offers entity-scoped actions (Refresh THIS account, Copy THIS request ID, Replay THIS request).
- `Esc` pops back to root tier; second `Esc` closes entirely.

### 2.4 Keyboard flow

Full flow documented in `docs/KEYBOARD_SHORTCUTS.md`. Summary inside palette:

- `↓` / `↑` navigate items.
- `Enter` execute default action; `⌘Enter` execute in new-window context.
- `Tab` reveals inline sub-actions (for items that have them — e.g. nav items have "open in new tab" secondary).
- `Backspace` on empty input pops scope to parent.
- `Esc` closes (or pops scope if nested).

### 2.5 Action execution timing

- **Synchronous actions** (navigate, copy, toggle theme) — execute on `Enter`, close palette immediately.
- **Async actions** (refresh token, import accounts) — execute on `Enter`, close palette immediately, show `Toast` in ~200ms with pending state, flip to success/error when complete. Never show a loading spinner in the palette itself.
- **Actions with confirmation** (Drop account) — execute on `Enter`, close palette, open `Dialog` with destructive confirm. Don't confirm inside the palette.
- **Debounce**: none. Every keystroke updates the filter; no typing lag.

---

## 3. Form interaction

### 3.1 Validation timing

| When | Behavior |
|---|---|
| `input` event (each keystroke) | **No validation.** Anxiety-inducing. |
| `blur` event | Run field-level validators; show `ErrorText` below field if invalid. |
| `submit` | Run all validators; focus first invalid field; show all `ErrorText`. |
| Server-side return | Inline field errors via `ErrorText`; form-level errors via banner above submit button. |

Exception: **async-verified fields** (e.g. refresh-token validity) may validate on blur after a 600ms debounce. Show `data-state="loading"` with inline `LoadingDots`; flip to `valid` or `invalid` on response.

### 3.2 Error display position and tone

- `ErrorText` sits directly below the field. NOT in a toast. NOT in a modal.
- Copy: `{what}: {why} — {how to fix}`. Example: "Label: already in use — pick a different one."
- Color: `--color-danger`. Icon: `alert-circle` (16px). Both required (WCAG 1.4.1).
- `role="alert"` so screen readers announce on invalidation.

### 3.3 Submit button states

| State | Visual |
|---|---|
| Idle | Primary variant, enabled |
| Disabled (form invalid) | `disabled` attr, opacity 0.5 |
| Submitting | `data-state="loading"`, `LoadingDots` replaces leading icon, label stays, `aria-busy="true"` |
| Success | Green 600ms flash via `--row-flash-progress`, then idle. Optional toast with Undo. |
| Error | Red 600ms flash, stay idle, error banner above the form |

### 3.4 Multi-step (wizard) forms

Rare. When needed:
- Dialog `data-size="wizard"` (640px).
- Stepper in header: "Step N of M".
- Footer: `Back` (ghost) + `Next` (primary) — Next promotes to `Save` on final step.
- `Back` preserves field values; `Next` validates current step only.
- User may jump back via stepper clicks; forward jumps disabled until validation passes.

---

## 4. Data table interaction

### 4.1 Sort

- Click column header: cycles `unsorted → ascending → descending → unsorted`.
- `⌘Click` on a second column: multi-column sort; secondary indicator shown.
- Keyboard: `Enter` or `Space` on a focused column header.
- Sort state persists in URL params: `?sort=status.asc,latency.desc`.

### 4.2 Filter

Per DESIGN_SYSTEM.md §7.5: search DSL, not drop-downs.

- Focus DSL input via `/` from anywhere in the view.
- Parse: `field:value` | `field:operator value` | `is:state` | `has:attribute` | free-text.
- Typeahead after `:` shows valid values for that field (from server catalog).
- Invalid filter shows inline warning pill below the input; doesn't block submit.
- Filter persists in URL: `?q=model:claude-sonnet+status:429`.
- "Clear filter" surfaces as a ghost button in the empty-state when filter returns zero.

### 4.3 Selection

- Checkbox column at position 0. `X` or `Space` toggles focused row. Click-drag selects range.
- `⌘A` selects all visible (max 200 for safety; scope to filter).
- `⌘⇧A` deselects all.
- Range selection: click row A, shift-click row B, everything between selects.
- Selection persists across sort/filter when the same items remain visible; cleared on navigation.

### 4.4 Bulk actions

- Action bar appears at TOP of table (above toolbar) when `selection.count > 0`.
- Shows: "N selected" + primary actions (Refresh, Export) + danger action (Drop…).
- Clears on deselect or navigation.
- Bulk destructive action → Dialog with typed confirmation showing the count + first 3 item previews.

### 4.5 Drill-down

- `Enter` on focused row OR click row → opens Drawer. List stays visible.
- `⌘Enter` OR `⌘Click` → opens full-page route. List navigates away.
- Row carries `data-state="drilled"` while drawer open; `aria-expanded="true"`.

### 4.6 Empty states per filter combination

Three distinct empty variants:

1. **First-time** (no data ever existed): "No accounts imported yet." + CLI command.
2. **Filtered** (data exists, filter returned zero): "No accounts match `status:healthy tier:pro`. Clear filter?"
3. **All-cooldown** (data exists, state prevents display): "All 3 accounts in cooldown — resumes in 45s" + countdown.

See `docs/components/empty-state.md` for variants.

---

## 5. Live data updates (SSE)

### 5.1 Connection model

- **One SSE connection per view.** Home (LiveRequestStream), Accounts, Requests, Metrics each open their own `EventSource`.
- NOT global — avoids dead connections on background tabs; browser manages backpressure per view.
- Reconnect: native `EventSource` retries at `retry:` interval from server (default 3000ms). On ≥3 consecutive failures, switch to polling `/state` every 5s + show "Reconnecting…" banner.

### 5.2 Update strategies

| Strategy | When | Visual |
|---|---|---|
| **Append** | New event (request arrived, refresh happened) | New block fades in at top via `@starting-style` |
| **Prepend** | Same as append, explicit for reverse-chron streams | Same |
| **Morph** | Existing entity updates (account cooldown countdown tick) | 600ms green flash via `@property --row-flash-progress`; no layout shift |
| **Replace** | Full pool snapshot (after reconnect) | Crossfade the whole list; no per-item flash |

### 5.3 Conflict resolution

Server-authoritative. If the dashboard has a pending optimistic action (e.g. "Refreshing acct-2…") and the server-pushed state disagrees:
- Drop the optimistic local state.
- Show the server state.
- If the action failed, show a `Toast` with retry.

No client-side merge logic; kiroxy's scope is single-user + trusted-pod, server is truth.

### 5.4 Stale indicator (SSE disconnected)

- Top-of-content banner: `● Live updates paused — reconnecting…` (info intent, pulsing dot).
- Content below continues to render cached data; timestamps gain a dim "(as of Xs ago)" label.
- On reconnect, banner dismisses + full snapshot replaces cache via `replace` strategy.

---

## 6. Error display hierarchy

Four tiers, decision tree:

```
Is the error from an async action the user just initiated?
├── YES ──► Is the action reversible/retryable?
│           ├── YES ──► Toast (danger intent) with Retry
│           └── NO  ──► Dialog (error variant)
└── NO ───► Is the error scoped to a form field?
            ├── YES ──► Inline ErrorText below field
            └── NO  ──► Is the error scoped to a section/panel?
                        ├── YES ──► Empty-state (error variant) replacing section
                        └── NO  ──► Banner at top of route
```

Tiers spelled out:

### Tier 1 — Toast (transient, dismissible)
For action-outcome errors that can be retried silently. Auto-dismiss 6s (longer than success toast's 4s). Optional Retry. Example: "Refresh failed — upstream 502."

### Tier 2 — Inline (form-scoped)
`ErrorText` inside `Input`. For validation and input-bound server errors. Persists until the user fixes it.

### Tier 3 — Empty-state (section-scoped)
Replaces the section body when a whole section can't render. Example: "Can't load requests — gateway unreachable" with Retry button. Rest of the dashboard remains usable.

### Tier 4 — Banner (page-scoped)
Top of the route, warning/danger intent. For cross-cutting issues: "Upstream outage affecting all accounts since 11:42." Dismissible but re-appears if the condition persists. Never auto-dismisses.

### Never
- Modal blocking the whole app for a background error.
- Toast for persistent errors (user may miss).
- Error sound.

---

## 7. Loading states

Four kinds, decision tree:

```
Is the loading duration known/expected?
├── YES — <100ms  ──► No indicator. Just render when ready.
├── YES — <500ms  ──► No indicator. (Below perception threshold.)
├── YES — >500ms  ──► Skeleton (component shape) OR inline LoadingDots (button)
└── UNKNOWN       ──► Skeleton with 10s timeout → error empty-state
```

### Skeleton
For section/page loads. Shape-matched to final content. See `components/skeleton.md`. No shimmer, no pulse.

### LoadingDots (inline)
For button submissions. 3-dot bounce, 120ms stagger. Replaces the button's leading icon while `aria-busy="true"`.

### Progress bar (determinate)
For long operations where progress IS known (import 50 accounts, 30% done). Bar + percentage + current/total count. Inline in the section, not in a toast.

### Optimistic
Submit-succeeds-shown-before-confirmation. Used for:
- Selection toggle (checkbox flips immediately; server ack tail).
- Theme switch (visual applies immediately; localStorage persists).
- `CopyableValue` (success toast before clipboard ack).

**Revert on failure:** if server rejects, undo the optimistic change + show toast with retry.

---

## 8. Confirmation patterns

### 8.1 Destructive action → Dialog with typed confirmation

Drop account, delete pool, reset vault: `Dialog` `data-intent="destructive"` with:
- Clear title: "Drop account `acct_01H8XJK9M2`"
- Description: what happens, what's preserved, what's revoked
- **Typed confirmation input** (user types a specific phrase like "drop acct_01H8") to enable the Confirm button
- Confirm button is `danger` variant
- Cancel button is `secondary`, focused by default (safer)

### 8.2 Non-destructive state change → Toast with Undo

Pause account, rename label, toggle theme: no modal. Apply immediately + show success Toast with "Undo" action (Superhuman pattern).

- Undo window: 4s (matches toast duration).
- Undo is optimistic too — applies immediately + re-toasts "Undone."

### 8.3 Bulk destructive → Dialog with count + preview

"Drop 12 accounts": Dialog showing:
- Count: "You're about to drop 12 accounts."
- Preview: first 3 items + "…and 9 more"
- Typed confirmation: "drop 12 accounts"
- Confirm and Cancel buttons

Never execute bulk destructive actions without preview.

---

## 9. Timing summary (token references)

| Interaction | Duration token | Ease |
|---|---|---|
| Focus ring | `--dur-instant` | — |
| Hover color | `--dur-quick` (120ms) | `--ease-default` |
| Popover/Tooltip open | `--dur-quick` | `--ease-default` |
| Dialog/Drawer open | `--dur-moderate` (200ms) | `--ease-default` |
| Toast enter | `--dur-moderate` | `--ease-default` |
| Row SSE flash | `--dur-flash` (600ms) | — (`@property`-driven) |
| Cross-page transition | `--dur-slow` (320ms) | `--ease-default` |
| Palette open | `--dur-quick` | `--ease-default` |
| Theme toggle | `--dur-slow` | via `document.startViewTransition()` |

All durations collapse to 0ms under `prefers-reduced-motion: reduce`. State changes still occur; animation does not.

---

## 10. Accessibility integration

Every interaction in this document implies:

- **Focus management**: clear rules about where focus moves on each action. Spelled out per primitive.
- **Announcements**: live regions (`aria-live="polite"` for SSE updates; `aria-live="assertive"` only for errors requiring attention).
- **Keyboard-only path**: every interaction reachable by keyboard; verified end-to-end per `docs/IMPLEMENTATION_RUBRIC.md`.
- **Reduced motion**: timing tokens collapse; state changes still occur.

See `docs/DESIGN_SYSTEM.md` §9 and the per-component spec's "Accessibility" section.
