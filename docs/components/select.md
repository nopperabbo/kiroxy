# select

Single-value selection from an enumerated list. Framework-agnostic; composes a native `<button>` trigger with a native `[popover]` listbox. No Portal, no Floating UI — CSS Anchor Positioning carries the placement.

**Pattern inheritance:** Radix UI `Select` API shape (compound component, slot-based), Linear's quiet chrome, Supabase's semantic border weight. See `research-v3/REFERENCE_GALLERY.md` Tier C (Radix) and Tier A (Supabase).

**Design system citation:** `docs/DESIGN_SYSTEM.md` §7.6 (modal/drawer/popover decision), §8 (primitive rules); `docs/components/popover.md` for the host primitive.

---

## Anatomy

```
<Select>
  ├── <SelectTrigger>                 ← button-shaped; shows current value
  │   ├── <SelectValue>
  │   ├── [optional] <SelectIcon>
  │   └── <ChevronDown>
  └── <SelectListbox>                 ← [popover], anchor-positioned
      ├── [optional] <SelectSearch>   ← embedded Input for >10 options
      ├── <SelectGroup> (optional, per category)
      │   ├── <SelectGroupLabel>
      │   └── <SelectOption>*
      └── <SelectEmptyState>          ← "No matches"
```

Markup template:

```html
<div class="kx-select" data-state="closed">
  <button type="button" class="kx-select__trigger"
          id="model-trigger"
          popovertarget="model-listbox"
          aria-haspopup="listbox"
          aria-expanded="false"
          aria-labelledby="model-label model-trigger">
    <span id="model-label" class="kx-field__label">Model</span>
    <span class="kx-select__value">claude-sonnet-4-5</span>
    <svg class="kx-select__chevron" aria-hidden="true"><!-- chevron-down --></svg>
  </button>
  <div id="model-listbox" popover="auto"
       anchor="model-trigger"
       role="listbox"
       aria-labelledby="model-label"
       class="kx-select__listbox"
       data-state="closed">
    <div class="kx-select__group" role="group" aria-labelledby="g-anthropic">
      <div id="g-anthropic" class="kx-select__group-label">Anthropic</div>
      <div class="kx-select__option" role="option" aria-selected="true"
           data-value="claude-sonnet-4-5" tabindex="0">
        claude-sonnet-4-5
      </div>
      <!-- … -->
    </div>
  </div>
</div>
```

---

## API

| Attribute (wrapper) | Values | Default | Description |
|---|---|---|---|
| `data-state` | `closed` \| `open` \| `loading` | `closed` | Drives trigger + listbox visibility. |
| `data-size` | `sm` \| `md` \| `lg` | `md` | Matches button/input. |
| `data-searchable` | `true` \| `false` | auto (`true` if ≥10 options) | Toggles the embedded search input. |
| `data-multiple` | `true` \| `false` | `false` | If `true`, see `select-multi.md` (v1.3 follow-up). |

| Attribute (option) | Values | Default | Description |
|---|---|---|---|
| `role` | `option` | `option` | Required for listbox semantics. |
| `aria-selected` | `true` \| `false` | `false` | Exactly one `true` per listbox. |
| `aria-disabled` | `true` \| `false` | — | Skip in keyboard nav. |
| `data-value` | string | — | Machine value; separate from visible label. |
| `data-group` | string | — | Optional; for keyboard-jump-to-group via initial letter. |

---

## Variants

- **`simple`** — no search, ≤10 options, no groups.
- **`searchable`** — embedded filter input at the top of the listbox. Filter matches option label + synonyms from `data-keywords`.
- **`grouped`** — options organized by `<SelectGroup>` with labels. `Home/End` jumps to first/last within the group.
- **`async`** — options load over SSE/fetch; shows `Skeleton` rows while loading, `SelectEmptyState` on empty result.

---

## States

| State | Trigger | Visual |
|---|---|---|
| Closed | default | Trigger shows current value; listbox hidden (via popover) |
| Open | trigger click OR `Enter`/`Space`/`ArrowDown` on trigger | Listbox visible, anchored below trigger via `anchor()`; first option focused |
| Searching | user types in search input | Options filtered; empty-state if 0 matches |
| Loading (async) | data in flight | Skeleton rows visible; trigger shows `LoadingDots` |
| Disabled | `aria-disabled` on trigger | Opacity 0.5, no pointer; does not open |

---

## Accessibility

WAI-ARIA: [Listbox Pattern](https://www.w3.org/WAI/ARIA/apg/patterns/listbox/). Not [Combobox](https://www.w3.org/WAI/ARIA/apg/patterns/combobox/) — kiroxy's Select is button-trigger-opens-listbox, not an editable input. For editable combobox behavior, use `CommandPalette`.

**Keyboard interactions:**

| Key (trigger focused) | Effect |
|---|---|
| `Enter` / `Space` / `ArrowDown` / `ArrowUp` | Open listbox; focus first/current selected option |
| Any printable character | Open + filter (searchable variant) OR open + jump to option starting with that character (simple variant) |

| Key (option focused) | Effect |
|---|---|
| `ArrowDown` / `ArrowUp` | Move focus to next/previous option (wraps) |
| `Home` / `End` | First / last option |
| `PageDown` / `PageUp` | Jump 10 |
| `Enter` / `Space` | Select + close |
| `Escape` | Close without selecting; focus returns to trigger |
| `Tab` | Close + focus next element (treat as commit current) |
| Printable character | Typeahead within listbox; debounce 500ms |

**Focus management:**
- Trigger receives focus when listbox closes (always).
- When opening via keyboard, focus moves to currently-selected option OR first option.
- When opening via mouse, focus goes to the listbox body; keyboard users then arrow to options.

**Screen readers:**
- Trigger announces: "[label], combobox, [current value], collapsed."
- On open: "expanded, [N] options."
- Option focus announces: "[option label], [M of N]."
- `aria-activedescendant` pattern is NOT used; we use roving tabindex (simpler; matches native `<select>`).

---

## Motion

| Transition | Token | Notes |
|---|---|---|
| Listbox open (via `@starting-style`) | `--dur-quick` `--ease-default` | Fade + 4px `translateY(-4px → 0)` |
| Listbox close | `--dur-quick` | Fade |
| Chevron rotation on open | `--dur-quick` | 180° via `transform: rotate()` |
| Option hover | instant | Background `--color-elevated` |

---

## Composition

**Contains:** `SelectTrigger`, `SelectListbox`, `SelectOption`, `SelectGroup`, `SelectEmptyState`, optional `SelectSearch` (an `Input` variant).

**Contained by:** `Form`, `FormRow`, `Toolbar`, `TableColumnHeader` (filter mode), `Drawer`.

**Paired with:** `Tooltip` (for disabled options explaining why), `Toast` (confirming async selection).

---

## Anti-patterns

- ❌ **Using `Select` for >50 options.** Switch to `CommandPalette` with scoped sigil. See `command-palette.md`.
- ❌ **Building a custom dropdown without `role="listbox"`.** Screen-reader-invisible. Use native semantics.
- ❌ **Selecting on hover.** Selection must be explicit (click, Enter, Space).
- ❌ **Opening upward AND closing unexpectedly.** Let CSS Anchor Positioning's `position-try-fallbacks` handle flip; don't hand-roll.
- ❌ **Closing on first keypress when searchable.** The filter must stay open until Escape or selection.
- ❌ **Multi-select collapsed into a single chip** ("3 selected"). Show at most 3 chips + "+N more"; wider triggers; or switch to a Drawer-based multi-picker.
- ❌ **Async load with no skeleton.** Layout shift = CLS violation.

---

## Differences from native `<select>`

- Searchable / grouped variants that native `<select>` cannot do.
- Consistent visual with other kiroxy inputs (native `<select>` is UA-styled).
- Keyboard semantics match native `<select>` exactly; we gain, not lose.
- Native `<select>` still the correct choice inside *forms that ship outside a browser* (RSS readers, webhooks) — kiroxy's Select is a UI component, not a form fallback.

---

## Reference

- **Radix UI Select** — the compound-component API shape this primitive mimics.
- **Linear** project-selector — searchable + grouped + keyboard-first, direct influence.
- **Supabase** table-editor column-picker — option grouping and subtle borders.

---

## Example usage

**Simple model selector (≤10 options):**

```html
<div class="kx-select" data-state="closed" data-size="sm">
  <button type="button" class="kx-select__trigger" id="t1"
          popovertarget="l1" aria-haspopup="listbox" aria-expanded="false">
    <span class="kx-field__label">Model</span>
    <span class="kx-select__value">claude-sonnet-4-5</span>
    <svg class="kx-select__chevron" aria-hidden="true" width="12" height="12"><!-- … --></svg>
  </button>
  <div id="l1" popover="auto" anchor="t1" role="listbox" class="kx-select__listbox">
    <div class="kx-select__option" role="option" aria-selected="true" data-value="claude-sonnet-4-5" tabindex="0">claude-sonnet-4-5</div>
    <div class="kx-select__option" role="option" data-value="claude-opus-4-7" tabindex="-1">claude-opus-4-7</div>
    <div class="kx-select__option" role="option" data-value="claude-haiku-4" tabindex="-1">claude-haiku-4</div>
  </div>
</div>
```

**Grouped + searchable account picker:**

```html
<div class="kx-select" data-state="closed" data-searchable="true">
  <button type="button" class="kx-select__trigger" id="acct-t" popovertarget="acct-l"
          aria-haspopup="listbox" aria-expanded="false">
    <span class="kx-field__label">Account</span>
    <span class="kx-select__value">acct_01H8XJK9M2</span>
    <svg class="kx-select__chevron" aria-hidden="true"><!-- … --></svg>
  </button>
  <div id="acct-l" popover="auto" anchor="acct-t" role="listbox" class="kx-select__listbox">
    <div class="kx-field kx-select__search" data-size="sm">
      <input type="search" placeholder="Filter accounts…"
             class="kx-field__input mono" aria-label="Filter options">
    </div>
    <div class="kx-select__group" role="group" aria-labelledby="g-healthy">
      <div id="g-healthy" class="kx-select__group-label">Healthy (3)</div>
      <div class="kx-select__option" role="option" data-value="acct_01H8…" tabindex="0">acct_01H8XJK9M2</div>
      <!-- … -->
    </div>
    <div class="kx-select__group" role="group" aria-labelledby="g-cool">
      <div id="g-cool" class="kx-select__group-label">Cooldown (1)</div>
      <!-- … -->
    </div>
  </div>
</div>
```
