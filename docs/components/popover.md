# popover

Lightweight non-modal overlay anchored to a trigger. Renders via the native `[popover]` attribute + CSS Anchor Positioning — zero JS for positioning, no Floating UI.

**Pattern inheritance:** Native web platform primitives (`popover` + anchor-positioning), Linear's restrained chrome, Vercel Geist `Context Card`. See `research-v3/REFERENCE_GALLERY.md` Tier D (`CSS Anchor Positioning`, `@starting-style`).

**Design system citation:** `docs/DESIGN_SYSTEM.md` §5.2 (@starting-style), §7.6 (decision tree vs dialog/drawer), `docs/INTERACTION_PATTERNS.md`.

---

## Anatomy

```
<Popover>
  ├── <PopoverTrigger>                ← any element with popovertarget attribute
  └── <PopoverContent>                 ← [popover] element, anchored via CSS
      ├── [optional] <PopoverArrow>   ← 8px pointer anchored to trigger centerline
      └── PopoverBody (free content)
</Popover>
```

Markup template:

```html
<button type="button" id="acct-peek-trigger"
        class="kx-button" data-variant="ghost" data-size="sm"
        popovertarget="acct-peek"
        aria-describedby="acct-peek">
  <svg aria-hidden="true" width="16" height="16"><!-- info --></svg>
  <span class="kx-button__label">acct_01H8</span>
</button>

<div id="acct-peek" popover="auto"
     anchor="acct-peek-trigger"
     class="kx-popover"
     data-placement="bottom"
     role="group" aria-labelledby="acct-peek-title">
  <header class="kx-popover__header">
    <strong id="acct-peek-title">acct_01H8XJK9M2</strong>
    <span class="kx-status-pill" data-intent="success">Healthy</span>
  </header>
  <dl class="kx-popover__body">
    <dt>Tier</dt><dd>Pro</dd>
    <dt>Last refresh</dt><dd class="mono">2m 14s ago</dd>
    <dt>Requests today</dt><dd class="mono" data-align="end">3,412</dd>
  </dl>
</div>
```

---

## API

| Attribute | Values | Default | Description |
|---|---|---|---|
| `popover` | `auto` \| `manual` \| `hint` | `auto` | Native. `auto` is light-dismiss (backdrop click + Esc). `hint` (Chrome 114+) for hover-triggered. |
| `data-placement` | `top` \| `bottom` \| `start` \| `end` | `bottom` | Preferred side; `position-try-fallbacks` handles flip. |
| `data-size` | `sm` \| `md` \| `lg` | `md` | Width caps: 240 / 320 / 480. |
| `data-arrow` | `true` \| `false` | `false` | Shows anchored pointer. |
| `anchor` | id-ref | — | Required. Matches trigger element id. |

---

## Variants

- **`info`** (default) — static read-only content; hover-triggered or click-triggered.
- **`menu`** — list of actions; upgrade to `role="menu"` + `<PopoverContent role="menu">`; items as `role="menuitem"`. If you have > 7 actions, use `CommandPalette` instead.
- **`form`** — small embedded form. Light-dismiss disabled (popover="manual"); explicit close required. Use sparingly — mostly a `Drawer` job.

---

## States

| State | Trigger | Visual |
|---|---|---|
| Closed | default | Hidden via native top-layer |
| Opening | `showPopover()` or click on trigger | Fade + `translateY(-4px → 0)` via `@starting-style` (if `data-placement="bottom"`) |
| Open | stable | Positioned relative to trigger via `anchor()` |
| Hover-peek (hint variant) | pointer over trigger; 500ms delay | Same as open; dismiss on pointer leave + 150ms |
| Closing | Esc, backdrop click (auto), explicit close | Fade out |

---

## Accessibility

WAI-ARIA:
- **Info popover** — `role="group"` or none; `aria-labelledby` pointing at the title element.
- **Menu popover** — `role="menu"`; items `role="menuitem"`; arrow-key navigation.
- **Tooltip popover** — use the dedicated `Tooltip` primitive (see `tooltip.md`), not Popover. Tooltip has different ARIA semantics (`role="tooltip"` + `aria-describedby`).

**Keyboard interactions (info variant):**

| Key (trigger focused) | Effect |
|---|---|
| `Enter` / `Space` | Toggle popover |
| `Escape` (when popover open) | Close popover; return focus to trigger |

**Keyboard interactions (menu variant):**

| Key (menu open) | Effect |
|---|---|
| `ArrowDown`/`ArrowUp` | Navigate items |
| `Home`/`End` | First/last |
| `Enter` / `Space` | Activate item |
| `Escape` | Close |
| Printable character | Typeahead |

**Focus management:**
- Trigger stays focused unless popover contains interactive content (menu items).
- On close, focus returns to trigger (native `popover` default).
- Light-dismiss via click-outside: native `popover="auto"` handles; click-outside events are NOT delivered to your app.

**Screen readers:**
- Trigger carries `aria-describedby` pointing at the popover id (for informational popovers).
- Menu-variant triggers carry `aria-haspopup="menu"` + `aria-expanded`.
- Popover content is announced on open (native).

---

## Motion

| Transition | Token | Notes |
|---|---|---|
| Open fade | `--dur-quick` `--ease-default` | `@starting-style` drives entrance |
| `translateY` offset | 4px toward anchor | Reverses direction if `position-try-fallbacks` flips |
| Arrow animation | none | Arrow is static; no "extend" animation |
| Close fade | `--dur-quick` | Out |

```css
.kx-popover {
  transition: opacity var(--dur-quick) var(--ease-default),
              transform var(--dur-quick) var(--ease-default),
              display var(--dur-quick) allow-discrete,
              overlay var(--dur-quick) allow-discrete;
  transition-behavior: allow-discrete;
}
.kx-popover:popover-open { opacity: 1; transform: translateY(0); }
@starting-style {
  .kx-popover[data-placement="bottom"]:popover-open {
    opacity: 0; transform: translateY(-4px);
  }
  .kx-popover[data-placement="top"]:popover-open {
    opacity: 0; transform: translateY(4px);
  }
}
```

---

## Composition

**Contains:** Free content; no required children.

**Contained by:** The document body's top layer — native `popover` handles rendering. Do NOT nest inside `overflow: hidden` ancestors (top layer escapes them).

**Paired with:** `Tooltip` (alternative for short hover hints), `Drawer` (escalation when content grows beyond popover size), `CommandPalette` (escalation when actions exceed 7).

---

## Anti-patterns

- ❌ **Popover as a confirmation dialog.** Popovers are light-dismiss; destructive actions need the Dialog's typed-confirm. DESIGN_SYSTEM.md §7.6.
- ❌ **Popover containing scrollable content > 320px tall.** Escalate to Drawer.
- ❌ **Multiple popovers open simultaneously from the same view.** `popover="auto"` closes the previous one when a new one opens — relying on that is fine, hand-rolling layered popovers is not.
- ❌ **Popover that traps focus.** Popovers don't trap; only Dialog and Drawer do.
- ❌ **Hover-triggered popover without pointer-leave dismiss.** Use `hint` variant or stick with click-triggered.
- ❌ **Popover anchored via JavaScript `getBoundingClientRect()` measurement.** Use CSS anchor-positioning. Safari 26+ supports it natively; oddbird polyfill covers older browsers.
- ❌ **Skipping the `anchor` attribute.** Without it, the popover renders at viewport origin.

---

## Differences from `Tooltip`

- `Tooltip` is hover-only, 500ms delay, no interactive content, `role="tooltip"`.
- `Popover` can be click-triggered, contains interactive content (buttons, links), `role="group"` or `"menu"`.
- Use Tooltip for "what does this icon mean"; use Popover for "what is acct-2 right now."

---

## Reference

- **MDN** — [Popover API](https://developer.mozilla.org/en-US/docs/Web/API/Popover_API).
- **MDN** — [CSS Anchor Positioning](https://developer.mozilla.org/en-US/docs/Web/CSS/CSS_anchor_positioning).
- **Vercel Geist** `Context Card` component — inspiration for the info-popover content shape (label-value dl).
- **Linear** avatar peek — pattern for an info popover on hover.

---

## Example usage

**Info popover with anchored arrow:**

```html
<button type="button" id="last-refresh-trigger"
        class="kx-button" data-variant="ghost" data-size="sm"
        popovertarget="last-refresh">
  <span class="mono">2m 14s ago</span>
</button>

<div id="last-refresh" popover="auto"
     anchor="last-refresh-trigger"
     class="kx-popover" data-placement="top" data-size="sm" data-arrow="true"
     role="group" aria-labelledby="lr-t">
  <strong id="lr-t">Last refresh</strong>
  <p class="kx-popover__body">
    <time datetime="2026-05-13T12:42:18Z" class="mono">2026-05-13 12:42:18 UTC</time>
  </p>
</div>
```

**Menu variant (row actions):**

```html
<button type="button" id="row-acct-2-more"
        class="kx-button" data-variant="ghost" data-size="sm"
        popovertarget="row-acct-2-menu"
        aria-haspopup="menu" aria-expanded="false"
        aria-label="More actions for acct-2">
  <svg aria-hidden="true"><!-- more-horizontal --></svg>
</button>

<div id="row-acct-2-menu" popover="auto"
     anchor="row-acct-2-more"
     class="kx-popover" data-size="sm" role="menu"
     aria-label="Actions for acct-2">
  <button type="button" role="menuitem" class="kx-menuitem">
    <span>Refresh token</span><kbd class="kx-keycap" aria-hidden="true">⌘R</kbd>
  </button>
  <button type="button" role="menuitem" class="kx-menuitem">
    <span>Copy ID</span><kbd class="kx-keycap" aria-hidden="true">⌘C</kbd>
  </button>
  <hr class="kx-menu__divider">
  <button type="button" role="menuitem" class="kx-menuitem" data-intent="destructive">
    <span>Drop account</span><kbd class="kx-keycap" aria-hidden="true">⌫</kbd>
  </button>
</div>
```
