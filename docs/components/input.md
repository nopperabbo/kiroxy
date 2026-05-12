# input

Single-line text input with optional leading affordance (icon, kbd hint) and trailing action (clear, toggle, copy). Framework-agnostic; renders as native `<input>`.

**Pattern inheritance:** Linear (tight chrome, `border` token as the dominant visual weight), Supabase (semantic error states), Raycast (keyboard-first invocation patterns — palette `⌘K`, search `/`). See `research-v3/REFERENCE_GALLERY.md`.

**Design system citation:** `docs/DESIGN_SYSTEM.md` §3 (typography), §7.3 (form interaction), §9 (accessibility).

---

## Anatomy

```
<InputField>
  ├── <Label>                        ← always visible OR visually-hidden but present
  ├── <InputRow>                     ← flex row that contains:
  │   ├── [optional] <LeadingSlot>   ← icon OR kbd hint
  │   ├── <input>                     ← the native control
  │   ├── [optional] <TrailingSlot>  ← clear-button, copy-button, toggle
  │   └── [optional] <HotkeyHint>    ← "/" or "⌘K" keycap
  ├── [optional] <HelperText>        ← always reserves space; collapses on error
  └── [optional] <ErrorText>         ← replaces HelperText on invalid
</InputField>
```

Markup template:

```html
<div class="kx-field" data-size="md" data-state="default">
  <label class="kx-field__label" for="account-search">Search accounts</label>
  <div class="kx-field__row">
    <svg class="kx-field__leading" aria-hidden="true" width="16" height="16"><!-- search icon --></svg>
    <input id="account-search" name="q" type="text" class="kx-field__input"
           placeholder="model:claude-sonnet status:429"
           aria-describedby="account-search-help"/>
    <kbd class="kx-keycap kx-field__hotkey" aria-hidden="true">/</kbd>
  </div>
  <p class="kx-field__help" id="account-search-help">Filter by the search DSL.</p>
</div>
```

---

## API

| Attribute (input) | Values | Default | Description |
|---|---|---|---|
| `type` | `text` \| `search` \| `email` \| `password` \| `url` \| `number` | `text` | Drives native UA behavior (autofill, virtual keyboard). |
| `name` | string | — | Required inside a form. |
| `value` / `defaultValue` | string | `""` | Controlled vs uncontrolled. |
| `placeholder` | string | — | Example query; never a proxy for a label. |
| `required` | presence | — | Native constraint. |
| `disabled` | presence | — | `aria-disabled` implicit. |
| `readonly` | presence | — | Visually distinct from `disabled`; focusable, copyable. |

| Attribute (field wrapper) | Values | Default | Description |
|---|---|---|---|
| `data-size` | `sm` \| `md` \| `lg` | `md` | Matches button scale. |
| `data-state` | `default` \| `focused` \| `invalid` \| `valid` \| `loading` | `default` | Drives ring + helper-text swap. |
| `data-density` | (inherited) | — | From `:root[data-density]`. |

---

## Variants

**Size × state matrix.** Every cell ships.

|  | `sm` (height 28/24) | `md` (32/28) | `lg` (36/32) |
|---|---|---|---|
| `default` | border + surface | border + surface | border + surface |
| `focused` | accent-border + ring | same | same |
| `invalid` | danger border + danger helper | same | same |
| `valid` | `success` subtle border (rare — only for async-verified fields like license keys) | same | same |
| `loading` | right-side inline `LoadingDots` | same | same |

**Slot variants:**
- **`search`** — leading search icon + trailing `/` hotkey.
- **`command`** — leading `>` sigil + trailing `⌘K`.
- **`readonly-copy`** — trailing copy-button; input `readonly`; font-family `mono` (for IDs, tokens).
- **`password`** — trailing eye/eye-off toggle.

---

## States

| State | Trigger | Visual |
|---|---|---|
| Default | idle | `border` token, `surface` background |
| Focused | `:focus-visible` OR `:focus-within` on wrapper | `accent-border` border, `--shadow-focus` ring, background stays |
| Invalid | form validation OR `data-state="invalid"` | `danger` 1px border, helper text replaced by `ErrorText` (`role="alert"`), leading icon swap to `alert-circle` |
| Valid | async verify confirms | success-subtle border; icon to `check`; auto-revert after 2s |
| Loading | async validation in-flight | trailing `LoadingDots`; `aria-busy="true"` on the input |
| Readonly | `readonly` attribute | border disappears, background `elevated`; copy action visible |
| Disabled | `disabled` | opacity 0.5, pointer-events none, text-dim |

---

## Accessibility

WAI-ARIA: [Input patterns](https://www.w3.org/WAI/ARIA/apg/patterns/) via native controls. Prefer native semantics — do not role-override.

**Keyboard:**

| Key | Effect |
|---|---|
| Global (when not focused in a textarea/editor) `/` | Focus the *primary* search input of the current view |
| `Escape` | Clear input if non-empty; blur if already empty; close palette if palette open |
| `Enter` | Submit parent form OR trigger associated action (e.g. palette selection) |
| `Cmd/Ctrl+C` on readonly input | Native copy; additionally triggers a 1.5s Toast "Copied" from `CopyableValue` wrapping if present |

**Focus management:**
- `:focus-visible` only for the ring — mouse focus gets no ring.
- When entering invalid state, focus stays on the input; do not auto-blur.
- Leading/trailing slot elements are NOT in the tab order (`tabindex="-1"`); clicking them focuses the input.

**Screen readers:**
- Visible `<label>` linked via `for`/`id` is required. `aria-label` only if the label would be visually redundant (search field with an icon + known context).
- `ErrorText` is linked via `aria-describedby` AND carries `role="alert"` so it's announced on invalidation.
- `HelperText` is linked via `aria-describedby` but does NOT use a live role — only announced when the user enters the field.
- Loading state announces via `aria-busy`; avoid double-announcements.

---

## Motion

| Transition | Token | Notes |
|---|---|---|
| Border color on focus/blur | `--dur-quick` | No box-shadow animation — the ring is instant |
| Ring appearance | `--dur-instant` | Keyboard accessibility demands instant visibility |
| Error shake (optional) | `--dur-moderate` × 2 iterations | **Opt-in only**, and only for `type="password"` on auth failure. Never on validation errors in general. |
| Valid checkmark fade-in | `--dur-quick` | Auto-revert after 2s |

---

## Composition

**Contains:** `<label>`, `<input>`, `LeadingSlot` (icon or hint), `TrailingSlot` (clear/copy/toggle), `HotkeyHint`, `HelperText` / `ErrorText`.

**Contained by:** `Form`, `FormRow`, `CommandPalette` (as the query input), `TableHeader` (as column filter), `Drawer` (edit forms).

**Paired with:** `Button` (form submit), `Select` (filter row), `Toast` (validation success).

---

## Anti-patterns

- ❌ **Placeholder as label.** Placeholder disappears on type; sighted users lose context, screen-reader users never hear it.
- ❌ **Validation on every keystroke.** DESIGN_SYSTEM.md §7.3 — on-blur + on-submit. Per-keystroke validation is anxiety-inducing.
- ❌ **Auto-clearing input on error.** Destructive; user loses their typed value.
- ❌ **Disabled inputs with no explanation.** Always include `aria-describedby` pointing at a help text explaining *why* the field is disabled.
- ❌ **Mixing leading icon + leading kbd hint.** Pick one; both is noisy.
- ❌ **Native browser autocomplete on search DSL fields.** Disable `autocomplete="off"` plus `data-1p-ignore` (1Password) — the DSL values aren't personal data and autofill is distracting.
- ❌ **Typing into a button styled as an input.** If it needs text entry, use `<input>`, not a faux-input div.

---

## Differences from native `<input>`

- Wrapping `<div class="kx-field">` carries state attributes; the input itself is unmodified native.
- Error state is explicit (`data-state="invalid"` + `role="alert"` on ErrorText) rather than relying on `:invalid` pseudo alone (which triggers on empty required fields before user interaction).
- Leading/trailing slots are in-flow; there is no absolute-positioning hack. The input's internal padding is adjusted by the wrapper's grid-template-columns.

---

## Reference

- **Linear** inputs: 510-weight labels, `border` as visual weight carrier.
- **Supabase** inputs: error-state semantics, `destructive` subtle border.
- **Raycast** search input: `/` focus convention, inline kbd hint.

---

## Example usage

**Filter input with DSL placeholder:**

```html
<div class="kx-field" data-size="md" data-state="default">
  <label class="kx-field__label" for="req-filter">Filter requests</label>
  <div class="kx-field__row">
    <svg class="kx-field__leading" aria-hidden="true" width="16" height="16"><!-- filter icon --></svg>
    <input id="req-filter" name="q" type="search"
           class="kx-field__input mono"
           placeholder="model:claude-sonnet status:429"
           autocomplete="off" data-1p-ignore>
    <kbd class="kx-keycap" aria-hidden="true">/</kbd>
  </div>
  <p class="kx-field__help" id="req-filter-help">
    Combine <code>field:value</code> filters or type free text.
  </p>
</div>
```

**Invalid API-key field:**

```html
<div class="kx-field" data-size="md" data-state="invalid">
  <label class="kx-field__label" for="api-key">API key</label>
  <div class="kx-field__row">
    <input id="api-key" name="key" type="password"
           class="kx-field__input"
           required
           aria-describedby="api-key-error">
  </div>
  <p class="kx-field__error" id="api-key-error" role="alert">
    Key must start with <code>sk-kx-</code>.
  </p>
</div>
```

**Readonly copyable token:**

```html
<div class="kx-field" data-size="sm" data-state="default">
  <label class="kx-field__label" for="acct-id">Account ID</label>
  <div class="kx-field__row">
    <input id="acct-id" readonly value="acct_01H8XJK9M2"
           class="kx-field__input mono">
    <button type="button" class="kx-button" data-variant="ghost" data-size="sm"
            aria-label="Copy account ID">
      <svg aria-hidden="true" width="14" height="14"><!-- copy icon --></svg>
    </button>
  </div>
</div>
```
