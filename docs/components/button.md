# button

Primary affordance for user-triggered actions. Framework-agnostic spec — renders as a native `<button>` element with explicit type and role management.

**Pattern inheritance:** Linear (tight chrome, 510-weight labels), Raycast (accent rarity — primary variant used ~5% of any view), Supabase (borders over shadows for secondary tier). See `research-v3/REFERENCE_GALLERY.md` → Tier A / Tier B.

**Design system citation:** `docs/DESIGN_SYSTEM.md` §1 (keyboard-first), §5 (motion tokens), §8 (primitive rules).

---

## Anatomy

```
<Button>
  ├── [optional] <LeadingIcon />   ← 16×16 svg, aria-hidden
  ├── <Label>                       ← required unless aria-label provided
  └── [optional] <TrailingIcon />   ← 16×16, usually chevron for menu triggers
</Button>
```

Markup template (framework-agnostic):

```html
<button type="button"
        class="kx-button"
        data-variant="primary"
        data-size="md"
        data-state="idle">
  <svg class="kx-button__icon" aria-hidden="true" width="16" height="16"><!-- … --></svg>
  <span class="kx-button__label">Refresh pool</span>
</button>
```

---

## API

| Attribute | Values | Default | Description |
|---|---|---|---|
| `type` | `button` \| `submit` \| `reset` | `button` | Required on every kiroxy button to prevent form-submit accidents. |
| `data-variant` | `primary` \| `secondary` \| `ghost` \| `danger` | `secondary` | Visual intent. See *Variants* below. |
| `data-size` | `sm` \| `md` \| `lg` | `md` | Height + padding scale. See *Variants*. |
| `data-state` | `idle` \| `loading` \| `success` \| `error` | `idle` | Lifecycle marker; drives motion + ARIA announcements. |
| `data-density` | (inherited from root) | `comfortable` | Consumed from `:root[data-density]`; do not set per-button. |
| `disabled` | presence | — | Native disabled attribute. Sets `aria-disabled` implicitly. |
| `aria-label` | string | — | Required when `<Label>` is absent (icon-only buttons). |
| `aria-describedby` | id | — | For inline helper text. |
| `aria-pressed` | `true` \| `false` | — | Only on toggle buttons; never on action buttons. |

---

## Variants

**Intent × size matrix** — 4 × 3 = 12 combinations. Every cell ships.

|  | `sm` (height 28px, compact 24px) | `md` (32/28) | `lg` (36/32) |
|---|---|---|---|
| `primary` | accent fill, `text-bright` | accent fill | accent fill, used for welcome CTA |
| `secondary` | surface fill, `border` | same | same |
| `ghost` | transparent, `text-default` | same | same |
| `danger` | `danger-subtle` fill, `danger` border + text | same | same |

**Rule of thumb (Raycast pattern):** A screen should contain **at most one** `primary` button. Every additional primary dilutes the signal. Form screens with Save + Cancel → Save is `primary`, Cancel is `secondary`.

---

## States

| State | Trigger | Visual |
|---|---|---|
| Idle | default | per variant |
| Hover | mouse-over | background shift to `--color-accent-hover` (primary) / `--color-elevated` (secondary/ghost) / `--color-danger-subtle` intensified (danger); duration `--dur-quick` |
| Focus-visible | keyboard focus | outline `var(--ring-width)` `--color-focus-ring` offset `var(--ring-offset)`. Never use `outline: none`. |
| Pressed | active / `:active` | background shift to `--color-accent-pressed`; no scale animation |
| Loading | `data-state="loading"` | label stays; LeadingIcon swaps to inline `LoadingDots`; pointer-events disabled; `aria-busy="true"` |
| Success | `data-state="success"` | 600ms green border flash via `@property --row-flash-progress`; returns to idle. Triggered after async submit. |
| Error | `data-state="error"` | 600ms danger border flash; returns to idle. Accompanying toast carries the message. |
| Disabled | `disabled` | opacity 0.5, pointer-events none, `cursor: not-allowed`. `aria-disabled="true"`. |

---

## Accessibility

WAI-ARIA pattern: [Button](https://www.w3.org/WAI/ARIA/apg/patterns/button/).

**Keyboard interactions:**

| Key | Effect |
|---|---|
| `Enter` / `Space` | Activate the button (native behavior) |
| `Tab` | Move focus to next interactive element |
| `Shift+Tab` | Move focus to previous |

**Focus management:**
- Always focusable when not disabled.
- `:focus-visible` only — never show a ring on mouse-click focus.
- Focus trap in a parent modal must include this button.

**Screen readers:**
- Icon-only button MUST set `aria-label="Refresh pool"`.
- Button with text label does NOT set `aria-label` (would override the label for SRs).
- Loading state announces via `aria-busy`; success/error states announce via an associated toast with `role="status"` (success) or `role="alert"` (error).

**Color contrast:** Every variant's text-on-fill passes AA per `docs/DESIGN_TOKENS_AUDIT.md`. `ghost` on `elevated` is the lowest-contrast pairing at 4.93:1 — AA pass but do not use `ghost` buttons on `elevated` surfaces for critical actions.

---

## Motion

| Transition | Token | Notes |
|---|---|---|
| Background on hover | `--dur-quick` `--ease-default` | Color shifts only; no transform. |
| Focus ring appearance | `--dur-instant` | Must be immediate — no fade-in. |
| Loading swap | `--dur-quick` | Icon crossfade; label stays anchored. |
| Success/error flash | `--dur-flash` | Single-shot via `@property`. |

`prefers-reduced-motion: reduce` → all transitions 0ms. State changes remain; only the animation collapses.

---

## Composition

**Contains:** `LeadingIcon` (optional), `Label` (required unless `aria-label`), `TrailingIcon` (optional).

**Contained by:** `Dialog` footer, `Drawer` footer, `Toolbar`, `CardHeader`, `FormRow`. **Never** inside a `StatusPill`, `Keycap`, or `HotkeyHint` — those are read-only surfaces.

**Paired with:** `LoadingDots` (inside loading state), `Toast` (announces success/error), `Tooltip` (for icon-only, with `delay` 500ms).

---

## Anti-patterns

- ❌ **Multiple `primary` buttons in one view.** Dilutes the signal. Raycast breaks this rarely; kiroxy never.
- ❌ **Icon-only button without `aria-label`.** Screen readers announce "button" — useless.
- ❌ **`onClick` on a `<div>`.** Use a native `<button>` for the free focus, keyboard, and ARIA.
- ❌ **Setting `outline: none` on `:focus-visible`.** Kills keyboard accessibility silently.
- ❌ **Using `primary` for destructive actions.** Use `danger` variant. The cyan-teal accent should never imply "Delete."
- ❌ **Loading-state spinner inside the button while leaving the label mutable.** Lock the label during loading so users don't type into a button on its way to submitting.
- ❌ **Full-width buttons in tables.** Table row actions use `ghost` or icon-only; full-width belongs in forms.
- ❌ **Gradient backgrounds** (the shadcn-dashboard template look). DESIGN_SYSTEM.md §13 forbids.

---

## Differences from native `<button>`

- Adds `data-variant` / `data-size` / `data-state` attributes for styling hooks (Radix pattern).
- Enforces `type="button"` default (native defaults to `submit` inside `<form>` — common footgun).
- Loading state is a first-class lifecycle, not an ad-hoc `disabled` toggle.
- Focus ring specified via tokens, not browser-default.

---

## Reference

- **Linear** buttons (REFERENCE_GALLERY.md → Tier A → Linear): 510-weight labels, accent reserved for primary.
- **Raycast** buttons (Tier B → Raycast): accent rarity ~5% of any view.
- **Vercel Geist** buttons: `Snippet` inspiration for trailing icon use.

---

## Example usage

**Primary action (form submit):**

```html
<button type="submit"
        class="kx-button"
        data-variant="primary"
        data-size="md"
        data-state="idle">
  <span class="kx-button__label">Save account</span>
</button>
```

**Icon-only ghost (table row action):**

```html
<button type="button"
        class="kx-button"
        data-variant="ghost"
        data-size="sm"
        aria-label="Refresh acct-2">
  <svg aria-hidden="true" width="16" height="16" viewBox="0 0 24 24">
    <!-- refresh icon -->
  </svg>
</button>
```

**Danger with keyboard hint:**

```html
<button type="button" class="kx-button" data-variant="danger" data-size="md"
        aria-keyshortcuts="Meta+Shift+D">
  <span class="kx-button__label">Drop account</span>
  <kbd class="kx-keycap" aria-hidden="true">⌘⇧D</kbd>
</button>
```
