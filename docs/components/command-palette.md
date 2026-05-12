# command-palette

Primary navigation mechanism. `⌘K` / `Ctrl+K` from anywhere. Two-tier: root palette (nav + global actions) → per-item action palette when invoked on a selected entity. Scope sigils (`/`, `#`, `>`, `@`, `?`) pivot the search target.

**Pattern inheritance:** Linear command menu (primary-nav idiom), Raycast two-tier (`⌘K` on selected item opens contextual sub-palette), Superhuman (every row shows its shortcut — the palette teaches itself obsolete), `cmdk` library shape. See `research-v3/REFERENCE_GALLERY.md` → Command palette deep dive.

**Design system citation:** `docs/DESIGN_SYSTEM.md` §7.1, §12; `research-v3/REFERENCE_GALLERY.md → Command palette and keyboard shortcut deep dive`.

---

## Anatomy

```
<CommandPalette>                         ← <dialog> with popover semantics
  ├── <PaletteInput>                     ← InputField variant, auto-focused
  ├── [optional] <PaletteScopeBadge>     ← shows current sigil scope
  ├── <PaletteList>                      ← role="listbox"
  │   ├── <PaletteGroup>* (virtualized when > 20 groups)
  │   │   ├── <PaletteGroupLabel>
  │   │   └── <PaletteItem>*             ← role="option"; shows action + Keycap
  │   └── <PaletteEmptyState>
  └── <PaletteFooter>
      ├── <FooterHint>*                  ← ⏎ run · ⌘⏎ run in new · ? help
      └── <ScopeTip>                     ← e.g. "Type / to search accounts"
</CommandPalette>
```

Markup template:

```html
<dialog id="cmdk" class="kx-palette"
        aria-label="Command palette"
        data-state="open" data-scope="root">
  <div class="kx-palette__input-row">
    <svg class="kx-palette__leading" aria-hidden="true" width="16" height="16"><!-- command --></svg>
    <input type="text" class="kx-palette__input"
           aria-autocomplete="list" aria-controls="cmdk-list"
           aria-activedescendant="cmdk-opt-1"
           placeholder="Type a command, or / for accounts…">
    <kbd class="kx-keycap" aria-hidden="true">Esc</kbd>
  </div>

  <ul id="cmdk-list" class="kx-palette__list" role="listbox" aria-label="Commands">
    <li class="kx-palette__group">
      <div class="kx-palette__group-label">Recents</div>
      <div id="cmdk-opt-1" class="kx-palette__item" role="option" aria-selected="true" tabindex="-1">
        <svg class="kx-palette__item-icon" aria-hidden="true"><!-- activity --></svg>
        <span class="kx-palette__item-label">Go to LiveRequestStream</span>
        <kbd class="kx-keycap" aria-hidden="true">g h</kbd>
      </div>
    </li>
    <li class="kx-palette__group">
      <div class="kx-palette__group-label">Actions</div>
      <div class="kx-palette__item" role="option" tabindex="-1">
        <svg class="kx-palette__item-icon" aria-hidden="true"><!-- refresh --></svg>
        <span class="kx-palette__item-label">Refresh all accounts</span>
        <kbd class="kx-keycap" aria-hidden="true">⌘⇧R</kbd>
      </div>
    </li>
  </ul>

  <footer class="kx-palette__footer">
    <span class="kx-palette__hint"><kbd class="kx-keycap">↩</kbd> run</span>
    <span class="kx-palette__hint"><kbd class="kx-keycap">⌘↩</kbd> new window</span>
    <span class="kx-palette__hint"><kbd class="kx-keycap">?</kbd> help</span>
  </footer>
</dialog>
```

---

## API

| Attribute | Values | Default | Description |
|---|---|---|---|
| `data-state` | `closed` \| `open` \| `loading` | `closed` | Drives `@starting-style` enter |
| `data-scope` | `root` \| `accounts` \| `requests` \| `commands` \| `models` \| `help` \| `item:{id}` | `root` | Sigil-driven scope |
| `data-tier` | `root` \| `item` | `root` | `item` = Raycast sub-palette on a selected entity |

| Scope sigil | Trigger | Searches |
|---|---|---|
| (none) | cold open | Recents + top commands + fuzzy across all scopes |
| `/` | typed as first char | Accounts |
| `#` | typed as first char | Requests (log entries) |
| `>` | typed as first char | Commands (actions) |
| `@` | typed as first char | Models |
| `?` | typed as first char | Help topics |

`Backspace` on empty input AND non-root scope pops scope back to root.

---

## Variants

- **`root`** — primary palette; nav + actions + search across scopes.
- **`item`** — opened via `⌘K` on a selected row; actions scoped to that entity (refresh this acct, drop this acct, copy this ID, view this request). Raycast pattern.
- **`filtered`** — user typed; results ranked by fuzzysort (prefix > camelCase > substring); empty state shows "No results. Press `?` for help."

---

## States

| State | Trigger | Visual |
|---|---|---|
| Closed | default | Not rendered |
| Opening | `⌘K` pressed | Fade + scale 0.98→1 via `@starting-style`, 120ms |
| Open (cold) | after open | Input empty + focused; list shows Recents + top 5 commands |
| Open (typing) | user typed ≥1 char | Results filtered + re-ranked; active option at top |
| Scope entered | sigil typed | Scope badge appears; list filters to that scope; sigil consumed from input |
| Loading (async) | fetching options | Input stays; list shows `SkeletonRow` × 3 |
| Empty | no results | PaletteEmptyState with tip |
| Closing | Esc or click-outside | Fade out 120ms |

---

## Accessibility

WAI-ARIA: [Combobox Pattern](https://www.w3.org/WAI/ARIA/apg/patterns/combobox/) — the palette is functionally a combobox with an attached listbox, not a simple input.

**Required:**
- Input has `aria-autocomplete="list"`, `aria-controls="{list-id}"`, `aria-activedescendant="{option-id}"`.
- List has `role="listbox"` and `aria-label="Commands"`.
- Items have `role="option"` and `aria-selected` reflecting focus.
- `aria-activedescendant` pattern is PREFERRED over roving tabindex for comboboxes — the input stays focused while active item changes.

**Keyboard:**

| Key | Effect |
|---|---|
| `⌘K` / `Ctrl+K` | Open (from anywhere) / close (from within palette — toggle) |
| `Escape` | Close palette; if in `item` tier, pop to `root` tier first |
| `ArrowDown` / `ArrowUp` | Move active option |
| `Enter` | Execute active option (default action) |
| `⌘+Enter` / `Ctrl+Enter` | Execute in "new window" context (e.g. open drill-down in a new route) |
| `Tab` | Navigate into item-level secondary actions (if option has sub-actions, tab reveals them) |
| `Home` / `End` | First / last option |
| `PageDown` / `PageUp` | Jump 10 options |
| `Backspace` (empty input, non-root scope) | Pop scope to root |
| `?` (typed first) | Enter help scope |
| `/` (typed first) | Enter accounts scope |

**Focus management:**
- On open: focus moves to the input.
- On close: focus returns to the element that triggered the palette (or body if opened globally).
- Dialog focus-trap wraps tab.
- Keyboard-only flow tested: user can open palette, navigate, and execute without a mouse.

**Screen readers:**
- Palette announces: "Command palette, dialog."
- Active option announces as the user arrows: "Go to LiveRequestStream, g h, 1 of 12."
- Scope changes announce: "Accounts scope, 8 results."
- Empty state announces: "No results found."

---

## Motion

| Transition | Token | Notes |
|---|---|---|
| Open | `--dur-quick` `--ease-default` | Fade + scale 0.98→1 via `@starting-style` |
| Close | `--dur-quick` | Reverse |
| Active option move | instant | Background shift only |
| Scope switch | `--dur-quick` | Badge fades in; list contents crossfade |

---

## Composition

**Contains:** PaletteInput (a specialized Input), PaletteList, PaletteGroup, PaletteItem, PaletteFooter, PaletteScopeBadge, PaletteEmptyState.

**Contained by:** Top layer; mounted near end of `<body>` as a `<dialog>`.

**Paired with:** `Keycap` (heavily — every item shows its shortcut), `StatusPill` (for account/request items), `Toast` (for action confirmations).

---

## Anti-patterns

- ❌ **Palette without visible keyboard hints on items.** The palette must teach itself obsolete (Superhuman). Every item displays its direct shortcut.
- ❌ **Empty palette on cold open.** DESIGN_SYSTEM.md §7.1 — empty state is CURATED. Recents + 5 common actions, never nothing.
- ❌ **Palette that doesn't support async loading.** Accounts/requests come from SSE; show skeleton rows, not a full-screen spinner.
- ❌ **Multiple modifier chords in palette items** (e.g. "Shift+Alt+Cmd+R"). One-modifier shortcuts + mnemonic `g x` chords only. Anything more complex belongs in a config file, not muscle memory.
- ❌ **Palette scope that requires learning sigil syntax upfront.** First-time user types nothing and gets full root + recents; sigils are a power-user acceleration, not a gate.
- ❌ **Palette closing on execute for nav actions.** If user picks "Go to Requests," palette closes + route changes. If user picks "Copy ID," palette closes + Toast confirms. Always close on execute.
- ❌ **Palette with > 3 footer hints.** Users can't parse more. Rotate hints based on scope if needed.
- ❌ **Custom palette for one-off actions that already exist in a keyboard map.** If `⌘R` refreshes the focused account, the palette entry "Refresh acct-2" should do exactly the same thing.
- ❌ **Fuzzy match with weights that feel magical.** Use the widely-understood algorithm (prefix > camelCase > substring). Document it.

---

## Reference

- **Linear** command menu — the modern primary-nav palette reference.
- **Raycast** two-tier palette (`⌘K` opens action palette for selected result).
- **Superhuman** — every row shows its shortcut.
- **cmdk** library (pacocoursey/cmdk) — the runtime we use.
- `research-v3/REFERENCE_GALLERY.md → Command palette and keyboard shortcut deep dive` — 11 products compared.

---

## Example usage

**Cold open (recents + top actions):**

```html
<dialog id="cmdk" class="kx-palette" aria-label="Command palette"
        data-state="open" data-scope="root">
  <div class="kx-palette__input-row">
    <svg class="kx-palette__leading" aria-hidden="true"><!-- command --></svg>
    <input type="text" class="kx-palette__input"
           aria-autocomplete="list" aria-controls="cmdk-list"
           placeholder="Type a command, or / for accounts…">
  </div>
  <ul id="cmdk-list" class="kx-palette__list" role="listbox" aria-label="Commands">
    <li class="kx-palette__group">
      <div class="kx-palette__group-label">Recents</div>
      <div class="kx-palette__item" role="option" aria-selected="true">
        <svg aria-hidden="true"><!-- activity --></svg>
        <span>Go to LiveRequestStream</span>
        <kbd class="kx-keycap">g h</kbd>
      </div>
    </li>
    <li class="kx-palette__group">
      <div class="kx-palette__group-label">Accounts</div>
      <div class="kx-palette__item" role="option">
        <svg aria-hidden="true"><!-- refresh --></svg>
        <span>Refresh all accounts</span>
        <kbd class="kx-keycap">⌘⇧R</kbd>
      </div>
    </li>
  </ul>
  <footer class="kx-palette__footer">
    <span class="kx-palette__hint"><kbd class="kx-keycap">↩</kbd> run</span>
    <span class="kx-palette__hint"><kbd class="kx-keycap">⌘↩</kbd> new window</span>
    <span class="kx-palette__hint"><kbd class="kx-keycap">?</kbd> help</span>
  </footer>
</dialog>
```

**Item tier (sub-palette for selected account):**

```html
<dialog id="cmdk-item" class="kx-palette" aria-label="Actions for acct_01H8"
        data-state="open" data-scope="item:acct_01H8XJK9M2" data-tier="item">
  <div class="kx-palette__input-row">
    <span class="kx-palette__scope-badge mono">acct_01H8XJK9M2</span>
    <input type="text" class="kx-palette__input" placeholder="Action…">
  </div>
  <ul class="kx-palette__list" role="listbox" aria-label="Account actions">
    <div class="kx-palette__item" role="option" aria-selected="true">
      <svg aria-hidden="true"><!-- refresh --></svg>
      <span>Refresh token</span>
      <kbd class="kx-keycap">⌘R</kbd>
    </div>
    <div class="kx-palette__item" role="option">
      <svg aria-hidden="true"><!-- copy --></svg>
      <span>Copy ID</span>
      <kbd class="kx-keycap">⌘C</kbd>
    </div>
    <div class="kx-palette__item" role="option" data-intent="destructive">
      <svg aria-hidden="true"><!-- trash --></svg>
      <span>Drop account…</span>
      <kbd class="kx-keycap">⌫</kbd>
    </div>
  </ul>
</dialog>
```
