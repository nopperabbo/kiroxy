# tooltip

Short hover/focus hint for icon-only controls and truncated values. Role `tooltip`, 500ms show delay, never contains interactive content.

**Pattern inheritance:** Linear (tight tooltips with 500ms delay), Raycast (keyboard-shortcut hints in tooltips), Vercel Geist. See `research-v3/REFERENCE_GALLERY.md` Tier A/B.

**Design system citation:** `docs/DESIGN_SYSTEM.md` §9 (accessibility), §7.6 (vs popover).

---

## Anatomy

```
<Tooltip>
  ├── <TooltipTrigger>       ← any element; carries aria-describedby
  └── <TooltipContent>       ← [popover="manual"] OR anchored div; role="tooltip"
</Tooltip>
```

Markup template (native popover-manual approach):

```html
<button type="button" id="refresh-btn"
        class="kx-button" data-variant="ghost" data-size="sm"
        aria-label="Refresh pool"
        aria-describedby="refresh-tip"
        onmouseenter="setTimeout(() => this.nextElementSibling.showPopover(), 500)"
        onmouseleave="this.nextElementSibling.hidePopover()"
        onfocus="this.nextElementSibling.showPopover()"
        onblur="this.nextElementSibling.hidePopover()">
  <svg aria-hidden="true"><!-- refresh --></svg>
</button>
<div id="refresh-tip" popover="manual"
     anchor="refresh-btn"
     role="tooltip"
     class="kx-tooltip" data-placement="top">
  Refresh pool <kbd class="kx-keycap" aria-hidden="true">⌘R</kbd>
</div>
```

---

## API

| Attribute | Values | Default | Description |
|---|---|---|---|
| `data-placement` | `top` \| `bottom` \| `start` \| `end` | `top` | `position-try-fallbacks` handles flip. |
| `data-delay` | ms | `500` | Show delay. Always 500ms per Linear convention. |
| `data-hide-delay` | ms | `150` | Grace period on leave. |
| `role` | `tooltip` | required | Announces as a tooltip to screen readers. |

---

## Variants

- **`label`** (default) — short text + optional `Keycap`. Max 40 chars.
- **`truncated-value`** — shown when a cell is truncated via `overflow: hidden`; reveals the full value on hover. Auto-detected: if the element's `scrollWidth > clientWidth`, attach the tooltip on hover. Otherwise no tooltip.

---

## States

| State | Trigger | Visual |
|---|---|---|
| Hidden | default | Not rendered in top layer |
| Delaying | pointer enters trigger | 500ms timer; nothing visible |
| Visible | 500ms elapsed OR focus-visible on trigger | Fade + 4px offset toward trigger |
| Hiding | pointer leaves OR blur | 150ms grace; fade out |

---

## Accessibility

WAI-ARIA: [Tooltip Pattern](https://www.w3.org/WAI/ARIA/apg/patterns/tooltip/).

**Required:**
- `role="tooltip"` on the content.
- `aria-describedby` on the trigger pointing at the tooltip id.

**Keyboard:**
- Tooltip appears on `:focus-visible` of the trigger (no delay for keyboard).
- `Escape` dismisses the tooltip without blurring the trigger.

**Screen readers:**
- Announced as tooltip on trigger focus.
- Content must NOT duplicate the element's accessible name (both would be announced; redundant).
- Good: icon button `aria-label="Refresh pool"` + tooltip "Refresh pool ⌘R" — the `aria-label` wins announcement; the keyboard hint is a visible affordance only.

---

## Motion

| Transition | Token | Notes |
|---|---|---|
| Show fade | `--dur-quick` | + 4px offset from trigger |
| Hide fade | `--dur-quick` | |

---

## Composition

**Contains:** Text + optional single `Keycap`. No buttons, no links.

**Contained by:** Top layer (native popover). Attached to any interactive element.

**Paired with:** `Keycap` (inline keyboard hint), `Button` (icon-only needs a tooltip).

---

## Anti-patterns

- ❌ **Tooltip with interactive content.** Use `Popover` with `role="menu"` or `role="group"`.
- ❌ **Tooltip as the only label for an icon button.** Mouse-only users miss it; non-visual users miss it unless `aria-describedby` is set. Always pair with `aria-label`.
- ❌ **Tooltip on text that's already readable.** Noise.
- ❌ **Tooltip arrow animated on show.** No — fade only.
- ❌ **Delay < 500ms.** Eager tooltips interrupt reading. 500ms is the industry convention for a reason.
- ❌ **Tooltip wrapping to 3+ lines.** Truncate the trigger label, don't stretch the tooltip. Max 40 chars; 2 lines absolute max.
- ❌ **Tooltip on touch devices.** Touchscreen has no hover; tooltip either pops on tap-and-hold (bad UX) or not at all. Design for the keyboard-visible affordance.

---

## Differences from `Popover`

- Tooltip is hover-only (+ keyboard focus). Popover is click-triggered.
- Tooltip has no interactive content. Popover can.
- Tooltip role is `tooltip`. Popover is `group` or `menu`.
- Tooltip auto-hides on leave. Popover requires explicit dismiss.

---

## Reference

- **Linear** tooltips — 500ms delay, monochrome surface, optional kbd hint.
- **Raycast** tooltips — inline keyboard shortcut displayed.
- **MDN** [Tooltip pattern](https://www.w3.org/WAI/ARIA/apg/patterns/tooltip/).

---

## Example usage

**Icon button with keyboard-shortcut hint:**

```html
<button type="button" id="cmd-palette-trigger"
        class="kx-button" data-variant="ghost" data-size="sm"
        aria-label="Open command palette"
        aria-describedby="cmdk-tip">
  <svg aria-hidden="true"><!-- command --></svg>
</button>
<div id="cmdk-tip" popover="manual" anchor="cmd-palette-trigger"
     role="tooltip" class="kx-tooltip" data-placement="bottom">
  Command palette <kbd class="kx-keycap" aria-hidden="true">⌘K</kbd>
</div>
```
