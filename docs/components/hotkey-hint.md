# hotkey-hint

A small visible "kbd" badge showing a keyboard shortcut. Lives on palette items, button row hints, tooltips. Cosmetic — the actual key listener lives on a global `KeyboardShortcuts` dispatcher.

**Pattern inheritance:** Raycast (keycap badges on every palette row), Linear (shortcut hints in menus), Superhuman (palette-as-training-wheel). Also called "Keycap" interchangeably — same primitive.

**Design system citation:** `docs/DESIGN_SYSTEM.md` §8 (`Keycap` primitive entry), §3.2 (tracking for keycaps), `docs/KEYBOARD_SHORTCUTS.md`.

---

## Anatomy

```
<HotkeyHint>                    ← rendered as <kbd>
  └── keys joined with +        ← e.g. "⌘⇧R" or "g then h"
</HotkeyHint>
```

Markup template:

```html
<kbd class="kx-keycap" aria-hidden="true">⌘K</kbd>
```

Multi-chord (Linear-style "g then h"):

```html
<span class="kx-keycap-chord" aria-hidden="true">
  <kbd class="kx-keycap">g</kbd>
  <span class="kx-keycap-chord__then">then</span>
  <kbd class="kx-keycap">h</kbd>
</span>
```

---

## API

| Attribute | Values | Default | Description |
|---|---|---|---|
| `data-size` | `sm` (20px tall) \| `md` (22px) | `md` | Match row density |
| `data-variant` | `default` \| `inverted` | `default` | Inverted for use on `--color-accent` backgrounds |
| `aria-hidden` | `true` | `true` | Always hidden from SRs; the containing element announces the shortcut via `aria-keyshortcuts` |

---

## Variants

- **`single`** — one key: `⌘`, `K`, `?`, `Esc`, `↩`.
- **`combo`** — modifier + key, rendered as a single `<kbd>` with no spaces: `⌘K`, `⌘⇧R`.
- **`chord`** — two keys in sequence ("g then h"): two `<kbd>` with "then" between.
- **`sequence`** — three+ keys in sequence (rare; `⌘K then ?` opens help scope from palette).

---

## States

Static. Does not react to pointer or focus. If the user has pressed the modifier (e.g. `Cmd`), the containing element (palette item) may add `data-pressed="true"` to its Keycap for transient visual feedback — NOT the Keycap itself.

---

## Accessibility

- `<kbd>` is semantic; no `role` needed.
- `aria-hidden="true"` — SRs hear the action name from the parent, not "Command K".
- **`aria-keyshortcuts` on the parent** is the authoritative SR announcement path per WAI-ARIA: `<button aria-keyshortcuts="Meta+K" aria-label="Open command palette">`.
- Cross-platform: display `⌘` on macOS, `Ctrl` on Windows/Linux. Detect via `navigator.platform` or `navigator.userAgentData` and render accordingly.

---

## Motion

None. Keycaps don't animate. If implementation adds a "press" animation on real keypress, reject — it's decorative.

---

## Composition

**Contains:** Text (the key character or modifier glyph).
**Contained by:** `CommandPaletteItem`, `Tooltip`, `Popover menuitem`, `Button` (trailing hint), `EmptyState` button.
**Paired with:** `Button` (provides the `aria-keyshortcuts`), `Tooltip` (hosts the keycap).

---

## Symbol conventions (macOS)

| Key | Glyph |
|---|---|
| Command | ⌘ |
| Control | ⌃ |
| Option/Alt | ⌥ |
| Shift | ⇧ |
| Caps Lock | ⇪ |
| Return/Enter | ↩ |
| Tab | ⇥ |
| Escape | Esc |
| Backspace/Delete | ⌫ |
| Arrow keys | ↑ ↓ ← → |
| Space | ␣ or literal "Space" |
| Page | ⇞ ⇟ |

---

## Windows/Linux rendering

Same structure, different glyphs:

| Mac glyph | Win/Linux rendering |
|---|---|
| ⌘ | Ctrl |
| ⌥ | Alt |
| ⇧ | Shift |
| ↩ | Enter |
| ⌫ | Backspace |
| Esc | Esc |

---

## Anti-patterns

- ❌ **Custom CSS making keycaps look like fake 3D keys** (the Apple System Prefs aesthetic from 2012). Flat monospace text + 1px border only.
- ❌ **Chords that require 3+ modifiers.** Muscle-memory ceiling. Two modifiers max.
- ❌ **Keycap without corresponding listener.** A visible hint that does nothing is worse than no hint.
- ❌ **Animating on press.** Decorative motion, violates §5.
- ❌ **Icons inside keycaps** (🎹, keyboard icons). Text is clearer.
- ❌ **`aria-hidden="false"`.** SR users hear both the action name AND "Command K"; redundant.

---

## Reference

- **Raycast** keycap badges on every palette row.
- **Linear** shortcut menus — chord rendering ("g then h").
- **Vercel Geist** `Keyboard Input` primitive.

---

## Example usage

**Single key:**
```html
<kbd class="kx-keycap" aria-hidden="true">/</kbd>
```

**Combo:**
```html
<kbd class="kx-keycap" aria-hidden="true">⌘K</kbd>
```

**Chord:**
```html
<span class="kx-keycap-chord" aria-hidden="true">
  <kbd class="kx-keycap">g</kbd>
  <span class="kx-keycap-chord__then">then</span>
  <kbd class="kx-keycap">h</kbd>
</span>
```

**Inside a palette item:**
```html
<div class="kx-palette__item" role="option">
  <svg aria-hidden="true"><!-- refresh --></svg>
  <span>Refresh all accounts</span>
  <kbd class="kx-keycap" aria-hidden="true">⌘⇧R</kbd>
</div>
```

**Inside a button (action + hint):**
```html
<button type="button" class="kx-button" data-variant="secondary"
        aria-keyshortcuts="Slash" aria-label="Focus search">
  <span class="kx-button__label">Search</span>
  <kbd class="kx-keycap" aria-hidden="true">/</kbd>
</button>
```
