# dialog

Modal overlay for confirmations, multi-step forms, and destructive actions. Uses the native `<dialog>` element with `@starting-style` for enter transitions. No JS focus-trap library — native `<dialog>` handles it.

**Pattern inheritance:** Radix UI `Dialog` compound API (trigger → overlay → content → close), Linear's quiet chrome, Superhuman's centered palette-style anchoring. See `research-v3/REFERENCE_GALLERY.md` Tier C + Tier B.

**Design system citation:** `docs/DESIGN_SYSTEM.md` §5 (motion via `@starting-style`), §7.6 (dialog vs drawer vs popover decision).

---

## Anatomy

```
<Dialog>
  ├── <DialogOverlay>              ← native ::backdrop; fades in
  └── <DialogContent>              ← the <dialog> element
      ├── <DialogHeader>
      │   ├── <DialogTitle>
      │   ├── [optional] <DialogDescription>
      │   └── <DialogClose>        ← "×" button; top-right
      ├── <DialogBody>             ← scrollable when > viewport height
      └── <DialogFooter>
          ├── [optional] <DialogCancel>
          └── <DialogConfirm>      ← primary or danger button
</Dialog>
```

Markup template:

```html
<dialog id="drop-account" class="kx-dialog"
        aria-labelledby="drop-title"
        aria-describedby="drop-desc">
  <header class="kx-dialog__header">
    <h2 id="drop-title" class="kx-dialog__title">Drop account</h2>
    <p id="drop-desc" class="kx-dialog__description">
      Removes <code class="mono">acct_01H8XJK9M2</code> from the pool. The
      refresh token is NOT revoked upstream; you can re-import it later.
    </p>
    <button type="button" class="kx-dialog__close"
            aria-label="Close dialog" formmethod="dialog" value="cancel">
      <svg aria-hidden="true" width="16" height="16"><!-- x icon --></svg>
    </button>
  </header>
  <section class="kx-dialog__body">
    <!-- confirm prompt, form fields, or diff view -->
  </section>
  <footer class="kx-dialog__footer">
    <button type="button" class="kx-button" data-variant="secondary"
            formmethod="dialog" value="cancel">Cancel</button>
    <button type="submit" class="kx-button" data-variant="danger"
            formmethod="dialog" value="confirm">Drop account</button>
  </footer>
</dialog>
```

Open via JS: `document.getElementById("drop-account").showModal()`.
Close by submitting a form with `method="dialog"` — native return-value support.

---

## API

| Attribute | Values | Default | Description |
|---|---|---|---|
| `data-size` | `simple` (480px) \| `wizard` (640px) \| `full` (800px) | `simple` | See `DESIGN_SYSTEM.md` §4.5. |
| `data-intent` | `confirm` \| `destructive` \| `form` \| `info` | `form` | Drives footer variant + default confirm button. |
| `data-state` | (read by CSS) `open` \| `closed` | — | Managed by `showModal()`/`close()` + `open` attribute on `<dialog>`. |

Native `<dialog>` attributes inherited: `open`, `showModal()`, `show()`, `close(returnValue)`, `returnValue`.

---

## Variants

- **`confirm`** — small (480px), one line of prose + Cancel + primary button. Example: "Save these changes?"
- **`destructive`** — same shape as `confirm` but confirm button is `danger` variant. For typed-confirmation (drop pool, reset vault), embed a `<TypedConfirm>` in the body that requires the user to type a specific word to enable the confirm button.
- **`form`** — medium (640px); embedded `Form`. Footer has Cancel + Submit.
- **`wizard`** — medium/large; multi-step. Footer has Back / Next / Finish. Step indicator in header.
- **`info`** — no footer except single "Got it" button. For changelog entries, onboarding notices.

---

## States

| State | Trigger | Visual |
|---|---|---|
| Closed | default | `display: none` via native `<dialog>` |
| Opening | `showModal()` called | Fade + `translateY(-6px → 0)` via `@starting-style`; backdrop fades in |
| Open | stable | Scroll-locked body; focus inside dialog |
| Submitting | inner form in flight | Confirm button shows `LoadingDots`; disabled Cancel |
| Error | inner form invalid | Inline `ErrorText` below the offending field; focus returns there |
| Closing | `close()` or form submit with method=dialog | Fade out |

---

## Accessibility

WAI-ARIA: [Dialog (Modal) Pattern](https://www.w3.org/WAI/ARIA/apg/patterns/dialog-modal/). Native `<dialog>` implements most of this for free.

**Required ARIA:**
- `aria-labelledby` pointing at `<DialogTitle>` id.
- `aria-describedby` pointing at `<DialogDescription>` id (optional but highly recommended).
- No `role="dialog"` — native `<dialog>` carries it implicitly (check your `<dialog>` polyfill coverage if targeting < 2023 browsers, though kiroxy does not).

**Keyboard interactions:**

| Key | Effect |
|---|---|
| `Escape` | Close dialog (native `<dialog>` default); returns focus to invoker |
| `Tab` | Focus trap within dialog (native handles this) |
| `Shift+Tab` | Reverse focus trap |
| `Enter` | Submit default button if a form is the body's default (native form behavior) |

**Focus management:**
- On open: focus moves to the first interactive element in `DialogBody` OR, for destructive dialogs, to the Cancel button (safer default).
- On close: focus returns to the element that triggered the dialog.
- Native `<dialog>` does both automatically when opened via `showModal()`.

**Screen readers:**
- Title and description are announced on open.
- `aria-live` is NOT needed inside a dialog (already a focused context).

---

## Motion

| Transition | Token | Notes |
|---|---|---|
| Dialog enter | `--dur-moderate` `--ease-default` | Fade + `translateY(-6px → 0)` via `@starting-style` |
| Backdrop enter | `--dur-moderate` | `background: oklch(0 0 0 / 0.45)` fade-in |
| Dialog exit | `--dur-quick` | Fade + `translateY(0 → -2px)` |
| Inner form validation shake | none | kiroxy does NOT shake inputs; error text is the signal |

```css
.kx-dialog {
  transition: opacity var(--dur-moderate) var(--ease-default),
              transform var(--dur-moderate) var(--ease-default),
              display var(--dur-moderate) allow-discrete,
              overlay var(--dur-moderate) allow-discrete;
  transition-behavior: allow-discrete;
}
.kx-dialog[open] { opacity: 1; transform: translateY(0); }
@starting-style {
  .kx-dialog[open] { opacity: 0; transform: translateY(-6px); }
}
```

`prefers-reduced-motion: reduce` → transitions collapse; dialog still opens/closes instantly.

---

## Composition

**Contains:** `DialogHeader` (required), `DialogBody`, `DialogFooter`.

**Contained by:** Any route. Dialogs are mounted at the end of `<body>` via the native top-layer — no portal needed.

**Paired with:** `Button` (footer actions), `Form` (body), `Toast` (post-close success announcement), `Drawer` (dialog closes, drawer may open — mutually exclusive).

---

## Anti-patterns

- ❌ **Dialog for row drill-down.** Use `Drawer` (§7.6) — the list context must stay visible.
- ❌ **Stacked dialogs.** If a confirm requires another confirm, your UX is broken. Refactor.
- ❌ **Closing on backdrop click for destructive dialogs.** Backdrop click only closes `confirm` / `form` / `info`; destructive requires explicit Cancel click or `Escape`.
- ❌ **Multi-step wizard without a step indicator.** Users lose place.
- ❌ **Auto-closing on success.** User doesn't see the success state. Show the outcome in the dialog, then auto-close only if the user has no follow-up action.
- ❌ **Focus-trap libraries** (focus-trap-react, etc.). Native `<dialog>` handles it; shipping a library is bundle bloat.
- ❌ **Dialogs taller than viewport without internal scroll.** `DialogBody` must be `overflow: auto` with `max-height: calc(100vh - header - footer - 32px)`.

---

## Differences from native `<dialog>`

- Enforces `aria-labelledby` / `aria-describedby`.
- Adds `@starting-style` entrance; native `<dialog>` opens instantly.
- Structured header/body/footer classes with opinionated padding + dividers.
- `data-intent="destructive"` unlocks the `TypedConfirm` pattern in the body; see `interaction-patterns.md`.

---

## Reference

- **Radix UI Dialog** — compound-component structure this spec mimics.
- **Linear** destructive dialogs — typed-confirmation pattern.
- **PlanetScale** deploy-request confirmations — explicit blockers surfaced in the body.
- **Stripe** save-changes dialogs — dismiss semantics.

---

## Example usage

**Destructive typed-confirm:**

```html
<dialog id="drop-pool" class="kx-dialog" data-size="simple" data-intent="destructive"
        aria-labelledby="dp-t" aria-describedby="dp-d">
  <form method="dialog" class="kx-dialog__form">
    <header class="kx-dialog__header">
      <h2 id="dp-t" class="kx-dialog__title">Drop all 12 accounts</h2>
      <p id="dp-d" class="kx-dialog__description">
        Removes every account from the pool. Refresh tokens are NOT revoked
        upstream. Type <code class="mono">drop pool</code> to confirm.
      </p>
      <button type="button" class="kx-dialog__close" aria-label="Close" value="cancel">
        <svg aria-hidden="true"><!-- x --></svg>
      </button>
    </header>
    <section class="kx-dialog__body">
      <div class="kx-field" data-size="md">
        <label class="kx-field__label" for="dp-confirm">Confirmation phrase</label>
        <div class="kx-field__row">
          <input id="dp-confirm" name="confirm" type="text"
                 class="kx-field__input mono"
                 autocomplete="off" data-1p-ignore>
        </div>
      </div>
    </section>
    <footer class="kx-dialog__footer">
      <button type="submit" class="kx-button" data-variant="secondary" value="cancel">Cancel</button>
      <button type="submit" class="kx-button" data-variant="danger" value="confirm"
              disabled data-enable-on="drop pool">Drop pool</button>
    </footer>
  </form>
</dialog>
```

Activator:

```html
<button type="button" class="kx-button" data-variant="danger"
        onclick="document.getElementById('drop-pool').showModal()">
  Drop pool
</button>
```
