# toast

Transient feedback stacked bottom-right. Auto-dismiss 4s; optional "Undo" per action. Never for errors that require operator intervention (see `empty-state.md` + `INTERACTION_PATTERNS.md`).

**Pattern inheritance:** Superhuman's success flashes (not modal); Linear's subtle toasts; `sonner` library shape. See `research-v3/REFERENCE_GALLERY.md` Tier B.

**Design system citation:** `docs/DESIGN_SYSTEM.md` §7.7 (toast vs inline decision), §5 (motion tokens).

---

## Anatomy

```
<ToastRegion>                          ← <ol role="region" aria-label="Notifications">
  └── <Toast>*
      ├── [optional] <ToastIcon>       ← status glyph matching data-intent
      ├── <ToastContent>
      │   ├── <ToastTitle>
      │   └── [optional] <ToastDescription>
      ├── [optional] <ToastAction>     ← single Button, ghost variant
      └── <ToastClose>                 ← "×"; keyboard accessible
</ToastRegion>
```

Markup template:

```html
<ol class="kx-toast-region" role="region" aria-label="Notifications">
  <li class="kx-toast" data-intent="success" role="status" data-state="open">
    <svg class="kx-toast__icon" aria-hidden="true"><!-- check-circle --></svg>
    <div class="kx-toast__content">
      <strong class="kx-toast__title">Account refreshed</strong>
      <span class="kx-toast__desc mono">acct_01H8XJK9M2 · new token expires in 59m</span>
    </div>
    <button type="button" class="kx-button" data-variant="ghost" data-size="sm">Undo</button>
    <button type="button" class="kx-toast__close" aria-label="Dismiss notification">
      <svg aria-hidden="true" width="14" height="14"><!-- x --></svg>
    </button>
  </li>
</ol>
```

---

## API

| Attribute (Toast) | Values | Default | Description |
|---|---|---|---|
| `data-intent` | `success` \| `info` \| `warning` \| `danger` | `info` | Drives icon + accent border-left |
| `data-state` | `open` \| `closing` | `open` | Drives enter/exit via `@starting-style` |
| `data-duration` | ms | `4000` | 0 = persistent until dismissed |
| `role` | `status` (success/info) \| `alert` (warning/danger) | — | Drives SR urgency |
| `aria-live` | inherited via role | — | Do NOT override |

---

## Variants

- **`success`** — green dot + check icon + title + optional Undo.
- **`info`** — blue dot + info icon + title + optional "Learn more" link.
- **`warning`** — amber dot + alert-triangle + title + optional "View" action.
- **`danger`** — red dot + alert-circle + title + optional "Retry" action. **For transient errors only** (e.g. network blip). Persistent errors go inline or in a banner.

---

## States

| State | Trigger | Visual |
|---|---|---|
| Enter | toast added | Fade + `translateY(8px → 0)` via `@starting-style`; 200ms |
| Visible | stable | Surface background; 2px left border in intent color |
| Hover | pointer over | Pause auto-dismiss timer |
| Action hover | pointer over action button | Normal button hover state |
| Dismissing | manual close OR timer elapsed | Fade + `translateX(8px → 0 reversed)`; 120ms |
| Stacked | > 1 toast | 4px gap; newest at bottom |

---

## Accessibility

WAI-ARIA: Not a dedicated pattern — uses `role="status"` / `role="alert"` + `aria-live` semantics.

**Required:**
- `<ol role="region" aria-label="Notifications">` container.
- Each toast: `role="status"` for success/info, `role="alert"` for warning/danger.
- `role="alert"` auto-implies `aria-live="assertive"`; `role="status"` implies `aria-live="polite"`.

**Keyboard:**
- Toast is NOT focusable by default (would interrupt current task).
- `F6` cycles focus into the toast region (browser convention for regions).
- Inside toast: `Tab` moves between Action and Close.
- `Escape` on focused close button dismisses the toast.

**Screen readers:**
- `role="status"` announces politely (after current speech finishes).
- `role="alert"` announces immediately (can interrupt).
- Do NOT put Undo as the first announced element; SRs should hear "Account refreshed. Undo, button. Dismiss, button."
- Timer-based dismiss is announced as the toast's removal from the live region; no special announcement needed.

---

## Motion

| Transition | Token | Notes |
|---|---|---|
| Enter | `--dur-moderate` `--ease-default` | Fade + translateY 8→0 via `@starting-style` |
| Pause on hover | instant | Timer pause; no visual change |
| Resume on leave | instant | Timer resumes |
| Exit | `--dur-quick` | Fade |
| Stack reflow | `--dur-quick` | Adjacent toasts slide up when one above dismisses |

---

## Composition

**Contains:** Icon, title, optional description, optional single action, close button.

**Contained by:** `<ToastRegion>`, mounted once near end of `<body>`. Position fixed bottom-right, respecting `env(safe-area-inset-*)` on mobile.

**Paired with:** `Button` (as action), `Snippet` (for copy-confirmations showing the copied value).

---

## Anti-patterns

- ❌ **Toast for errors requiring intervention.** Use inline error (form) or banner (page-level). Operator may miss a 4s toast.
- ❌ **Toast with multiple actions.** Pick one. Stacking actions is a dialog, not a toast.
- ❌ **Toast longer than 80 characters.** Longer belongs in the drawer or log.
- ❌ **Toast for ongoing state** (e.g. "Refreshing…"). Use inline status in the card header. Toast is for completion.
- ❌ **Toast without Undo for destructive actions.** Superhuman pattern: if an action is reversible, offer Undo. If not, it shouldn't have happened without a Dialog confirm.
- ❌ **Multiple toasts from the same action.** Collapse into one with a count ("3 accounts refreshed").
- ❌ **Toast that can block UI.** Never position-center or add backdrop; toasts stack at the corner only.
- ❌ **`alert` role for success.** Success is `status` (polite). Interrupting SR users is a design error.

---

## Differences from native browser notifications

- Toasts live inside the app; browser notifications live in the OS tray.
- Toasts have no persistence; browser notifications do.
- Use browser notifications via the Notifications API ONLY for events that happen while the tab is backgrounded (incident alerts). In-app = toast.

---

## Reference

- **Superhuman** success flashes — Undo action, 4s duration.
- **Linear** toasts — subtle, left-border in intent color.
- **sonner** (`pacocoursey`) — bottom-right stacking with 4px gap; the runtime library kiroxy uses.

---

## Example usage

**Success with Undo:**

```html
<ol class="kx-toast-region" role="region" aria-label="Notifications">
  <li class="kx-toast" data-intent="success" role="status" data-duration="4000">
    <svg class="kx-toast__icon" aria-hidden="true" width="16" height="16"><!-- check-circle --></svg>
    <div class="kx-toast__content">
      <strong class="kx-toast__title">3 accounts refreshed</strong>
    </div>
    <button type="button" class="kx-button" data-variant="ghost" data-size="sm">Undo</button>
    <button type="button" class="kx-toast__close" aria-label="Dismiss">
      <svg aria-hidden="true" width="14" height="14"><!-- x --></svg>
    </button>
  </li>
</ol>
```

**Transient error with retry:**

```html
<li class="kx-toast" data-intent="danger" role="alert" data-duration="6000">
  <svg class="kx-toast__icon" aria-hidden="true"><!-- alert-circle --></svg>
  <div class="kx-toast__content">
    <strong class="kx-toast__title">Refresh failed</strong>
    <span class="kx-toast__desc">Upstream returned 502. Retry?</span>
  </div>
  <button type="button" class="kx-button" data-variant="secondary" data-size="sm">Retry</button>
  <button type="button" class="kx-toast__close" aria-label="Dismiss"><svg aria-hidden="true"><!-- x --></svg></button>
</li>
```
