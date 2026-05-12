# KEYBOARD_SHORTCUTS.md — kiroxy

> Canonical keyboard shortcut map for the kiroxy dashboard. Every action here
> is also exposed as a command palette entry with the matching `Keycap` hint
> displayed inline — the palette teaches itself obsolete.
>
> **Status:** v1.0 drafted 2026-05-13.
>
> **Companion documents:**
> - `docs/DESIGN_SYSTEM.md` §7 (interaction patterns)
> - `docs/INTERACTION_PATTERNS.md` — decision trees
> - `docs/components/hotkey-hint.md` — visual badge
> - `docs/components/command-palette.md` — scope sigils + behavior
> - `research-v3/REFERENCE_GALLERY.md → Command palette deep dive` — 11 products compared
>
> **Displayed in-app:** This entire shortcut map is the content of the
> keyboard-shortcut cheatsheet overlay triggered by `?` from any screen.

---

## Conventions

- macOS shown first. Windows/Linux equivalent in parentheses. The app detects
  `navigator.userAgentData.platform` or `navigator.platform` and renders the
  correct glyph.
- `⌘` = Cmd on macOS, Ctrl on Windows/Linux.
- `⌥` = Opt on macOS, Alt on Windows/Linux.
- Chord sequences ("g then a") use a 700ms timeout window between keys.
- Everything in this document has an `aria-keyshortcuts` binding on the
  invoking element; screen readers announce shortcuts.
- Shortcuts that conflict with native text-input (`⌘A`, `⌘C`, arrow keys) are
  intercepted ONLY when the focus is outside a text-editable element.

Attribution: keyboard taxonomy influenced by Linear, Raycast, Superhuman, and
`cmdk`. See `REFERENCE_GALLERY.md → Command palette deep dive` for the full
cross-product comparison.

---

## Global (anywhere in the UI)

| Key | Action |
|---|---|
| `⌘K` (`Ctrl+K`) | Open command palette |
| `/` | Focus primary search input of current view |
| `?` | Open keyboard-shortcut cheatsheet overlay |
| `Esc` | Close top overlay: popover → tooltip → dialog → drawer → palette → cheatsheet |
| `⌘/` (`Ctrl+/`) | Toggle light / dark / dark-dimmed theme (cycle) |
| `⌘\` (`Ctrl+\`) | Toggle sidebar (collapsed icon-rail ↔ expanded) |
| `⌘⇧L` (`Ctrl+Shift+L`) | Toggle live log stream panel |
| `⌘⇧T` (`Ctrl+Shift+T`) | Toggle density (comfortable ↔ compact) |
| `⌘,` (`Ctrl+,`) | Open settings route |

## Navigation chords (Linear-style `g then X`)

| Chord | Destination |
|---|---|
| `g` then `h` | Home — LiveRequestStream (signature) |
| `g` then `a` | Accounts |
| `g` then `r` | Requests (history) |
| `g` then `m` | Metrics |
| `g` then `s` | Settings |
| `g` then `l` | Logs |
| `g` then `d` | Dashboard overview (alias for `g h`) |
| `g` then `?` | Help |

Chord window: 700ms. Pressing any non-chord key cancels the pending `g`.

## List navigation (tables, feeds, timelines)

| Key | Action |
|---|---|
| `j` / `↓` | Focus next row / block |
| `k` / `↑` | Focus previous row / block |
| `Home` | First row |
| `End` | Last row |
| `PageDown` | Jump 10 rows down |
| `PageUp` | Jump 10 rows up |
| `Enter` | Open drill-down drawer for focused row |
| `⌘Enter` (`Ctrl+Enter`) | Open drill-down as route (new tab / new window context) |
| `Space` / `x` | Toggle row selection |
| `⌘A` (`Ctrl+A`) | Select all visible rows (capped at 200 for safety) |
| `⌘⇧A` (`Ctrl+Shift+A`) | Deselect all |
| `⇧Click` | Range-select from last-clicked row |
| `⌘Click` (`Ctrl+Click`) | Toggle selection without clearing others |
| `r` | Refresh focused row (per-entity action — e.g. refresh token on focused account) |
| `e` | Edit focused row inline (if editable) |
| `.` or `⌘K` on focused row | Open item-tier command palette scoped to row |
| `c` | Copy focused row's primary ID to clipboard |

## Drill-down drawer

| Key | Action |
|---|---|
| `Esc` | Close drawer; focus returns to list row |
| `j` / `k` | Navigate between detail fields |
| `⌘C` (`Ctrl+C`) on focused field | Copy field value |
| `⌘R` (`Ctrl+R`) | Refresh drawer content (re-fetch entity) |
| `⌘⇧←` (`Ctrl+Shift+Left`) | Previous entity in list (navigate without closing drawer) |
| `⌘⇧→` (`Ctrl+Shift+Right`) | Next entity in list |

## Command palette (open)

| Key | Action |
|---|---|
| `↓` / `↑` | Move active item |
| `Home` / `End` | First / last item |
| `PageDown` / `PageUp` | Jump 10 |
| `Enter` | Execute default action |
| `⌘Enter` (`Ctrl+Enter`) | Execute in "new window" context (e.g. route opens new tab) |
| `Tab` | Reveal / enter inline sub-actions for active item |
| `Backspace` on empty input | Pop scope to parent (sigil-scope ↔ root; item-tier ↔ root) |
| `Esc` | Close palette (or pop one tier if nested) |
| `/` typed at position 0 | Enter accounts scope |
| `#` typed at position 0 | Enter requests scope |
| `>` typed at position 0 | Enter commands scope |
| `@` typed at position 0 | Enter models scope |
| `?` typed at position 0 | Enter help scope |

## Forms (focus inside form)

| Key | Action |
|---|---|
| `Tab` | Move to next field |
| `⇧Tab` | Move to previous field |
| `⌘Enter` (`Ctrl+Enter`) | Submit form (shortcut for Submit button) |
| `Esc` | Cancel / revert changes (warns if unsaved) |
| `⌘Z` (`Ctrl+Z`) | Undo last field change (limited to text inputs) |

## LiveRequestStream — signature primitive

See `docs/components/live-request-stream-block.md`.

| Key (block focused) | Action |
|---|---|
| `Enter` | Inspect — opens drawer |
| `Space` | Toggle selection |
| `a` | Toggle attach-for-context (cmd-click equivalent) |
| `⌘C` (`Ctrl+C`) | Copy request ID |
| `⌘R` (`Ctrl+R`) | Replay — opens replay drawer |
| `⌘L` (`Ctrl+L`) | View logs filtered by this request ID |
| `⌘K` (`Ctrl+K`) | Open item-tier palette for this request |
| `⌘⇧S` (`Ctrl+Shift+S`) | Copy shareable permalink |

## Dialog / modal

| Key | Action |
|---|---|
| `Esc` | Close (Cancel) |
| `Enter` | Submit default button (form-native) |
| `Tab` / `⇧Tab` | Focus trap cycles within dialog |
| `⌘Enter` (`Ctrl+Enter`) | Confirm primary action (e.g. submit destructive dialog) |

## Theme & appearance

| Key | Action |
|---|---|
| `⌘/` (`Ctrl+/`) | Cycle theme: dark → light → dark-dimmed → system → dark |
| `⌘⇧D` (`Ctrl+Shift+D`) | Force dark (skips cycle) |
| `⌘⇧T` (`Ctrl+Shift+T`) | Toggle density mode |

## Accessibility and escape hatches

| Key | Action |
|---|---|
| `Esc` (on any overlay) | Close top overlay; focus returns to invoker |
| `F6` | Cycle focus between landmark regions (sidebar ↔ content ↔ toast) |
| `Tab` | Move through visible interactive elements |
| `⇧Tab` | Reverse |
| `Alt+F4` (Win) / `⌘Q` (macOS) | Native — quit browser; not intercepted |

---

## Rendering conventions

Every shortcut above is rendered in-app as a `Keycap` (`docs/components/hotkey-hint.md`). Two visual styles:

```html
<!-- Single combo -->
<kbd class="kx-keycap" aria-hidden="true">⌘K</kbd>

<!-- Chord sequence -->
<span class="kx-keycap-chord" aria-hidden="true">
  <kbd class="kx-keycap">g</kbd>
  <span class="kx-keycap-chord__then">then</span>
  <kbd class="kx-keycap">h</kbd>
</span>
```

All `aria-hidden="true"` — screen readers are informed via `aria-keyshortcuts` on the invoking element, not via the visible keycap.

---

## Implementation contract

A Track 3 implementation that claims compliance MUST:

1. **Register each shortcut in a single `KeyboardShortcuts` module** with priority scoping so palette-open handlers preempt list handlers preempt global handlers.
2. **Respect the native text-input escape rule**: `⌘A`, `⌘C`, arrow keys, `Enter`, `Esc` pass through to the browser when focus is inside `input`/`textarea`/`[contenteditable]` EXCEPT where this doc explicitly overrides (e.g. `⌘K` always opens palette).
3. **Render every executable shortcut as a palette entry** — so a user who doesn't know the shortcut discovers it through the palette and learns the keycap hint.
4. **Document every deviation** from this map in `BACKLOG.md`. Do not silently deviate.
5. **Implement `?` cheatsheet** that renders exactly this file's content (server-rendered from the markdown, not a hand-maintained duplicate).

---

## Rationale summary (inheritance)

| Pattern | Source |
|---|---|
| `⌘K` universal palette | Linear, Superhuman, Vercel, cmdk |
| `/` focus search | GitHub, Stripe, Linear |
| `?` cheatsheet overlay | Stripe (`docs.stripe.com/dashboard/basics` confirms), Linear |
| `g` then `X` navigation chords | Linear (documented in Linear guide) |
| `j` / `k` Vim-style list nav | Superhuman, Linear |
| Item-tier palette `.` or `⌘K` on row | Raycast two-tier |
| `⌘Enter` for new-window context | GitHub, VS Code, Vercel |
| `Backspace` pops scope | cmdk library convention |
| `⇧Click` range select | native platform |
| `⌘/` theme toggle | VS Code |

**If Track 3 wants to introduce a shortcut NOT in this document:** update this
file first, via PR. The shortcut map is the contract.
