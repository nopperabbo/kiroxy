# live-request-stream-block

kiroxy's **signature primitive.** The Block is the atomic unit of the LiveRequestStream on the dashboard home page — one request = one Block. Warp-inspired: scrollable, copyable, permalinkable, cmd-click-attachable for replay-with-context.

**Pattern inheritance:** Warp's block-as-unit-of-terminal-output invention, Linear's subtle row hierarchy, Vercel Geist `Entity` component. See `research-v3/REFERENCE_GALLERY.md` → Tier B → Warp; `docs/VISION.md` §signature-thing; `docs/DESIGN_SYSTEM.md` §12.

**Design system citation:** `docs/DESIGN_SYSTEM.md` §12 (LiveRequestStream signature), §5.3 (row flash animation), §13 (no decoration).

This primitive is unique to kiroxy. Operators recognize the dashboard by this Block.

---

## Anatomy

```
<LiveRequestBlock>
  ├── <BlockHeader>                                   ← 1-line summary; always visible
  │   ├── <StatusDot>                                 ← healthy/failed/cooldown
  │   ├── <BlockAccount>                              ← acct_01H8, clickable → drill drawer
  │   ├── <BlockModel>                                ← claude-sonnet-4-5
  │   ├── <BlockMethod>                               ← POST /v1/messages
  │   ├── <BlockStatusCode>                           ← 200 / 429 / 502
  │   └── <BlockLatency>                              ← 1.4s
  ├── <BlockMetadata>                                  ← 2nd line; visible in comfortable mode
  │   ├── <TokenCounts>                               ← 1,247 in · 389 out
  │   ├── <Cost>                                       ← $0.012
  │   ├── <StreamFlag>                                 ← stream|non-stream
  │   └── <Timestamp>                                  ← 11:42:18
  └── <BlockHints>                                     ← 3rd line; hover-only
      ├── ⌘↩ inspect
      ├── ⌘C copy ID
      ├── ⌘R replay
      └── ⌘L view logs
</LiveRequestBlock>
```

Markup template (comfortable density, healthy request):

```html
<article class="kx-block" data-state="idle" data-intent="success"
         data-request-id="req_01HKM9N2ZQ"
         aria-label="Request to claude-sonnet-4-5 via acct_01H8, 200 OK in 1.4 seconds"
         tabindex="0">
  <header class="kx-block__header">
    <span class="kx-status-dot" data-intent="success" aria-hidden="true"></span>
    <a class="kx-block__account mono" href="/accounts/acct_01H8XJK9M2">acct_01H8</a>
    <span class="kx-block__model mono">claude-sonnet-4-5</span>
    <span class="kx-block__method mono">POST /v1/messages</span>
    <span class="kx-block__status mono">200</span>
    <span class="kx-block__latency mono" data-align="end">1.4s</span>
  </header>
  <div class="kx-block__metadata">
    <span class="kx-block__tokens mono">1,247 in · 389 out</span>
    <span class="kx-block__cost mono">$0.012</span>
    <span class="kx-block__stream">stream</span>
    <time class="kx-block__ts mono" datetime="2026-05-13T11:42:18Z">11:42:18</time>
  </div>
  <div class="kx-block__hints" aria-hidden="true">
    <span><kbd class="kx-keycap">⌘↩</kbd> inspect</span>
    <span><kbd class="kx-keycap">⌘C</kbd> copy ID</span>
    <span><kbd class="kx-keycap">⌘R</kbd> replay</span>
    <span><kbd class="kx-keycap">⌘L</kbd> view logs</span>
  </div>
</article>
```

---

## API

| Attribute | Values | Default | Description |
|---|---|---|---|
| `data-state` | `idle` \| `hover` \| `selected` \| `attached` \| `drilled` \| `updating` | `idle` | Drives visual state |
| `data-intent` | `success` \| `warning` \| `danger` \| `info` | inferred from status-code | Drives `--status-dot` color |
| `data-density` | (inherited) | `comfortable` | Comfortable = 3 lines; Compact = 1 line |
| `data-request-id` | string | — | Required; powers permalink + cmd-C copy |
| `aria-label` | string | generated | Full request summary for SRs |
| `tabindex` | `0` | required | Block is a focus target |

---

## Variants (by density)

**Comfortable mode (default):** 3-line block — header + metadata + (hover) hints. Row height ~64px.

**Compact mode (`data-density="compact"`):** 1-line block collapsing metadata inline:

```
● acct_01H8 claude-sonnet-4-5 POST /v1/messages 200 1.4s 1247/389 $0.012 11:42:18
```

Row height ~28px. Hints live in the palette `⌘K` sub-palette on cmd-click instead of inline hover.

---

## States

| State | Trigger | Visual |
|---|---|---|
| Idle | default | `surface` background; no border |
| Hover | pointer over | Background `elevated`; hints row becomes visible |
| Focus-visible | keyboard | Ring on the block |
| Selected | `Space` on focused block | Background `accent-subtle`; left 2px `accent` border |
| Attached (cmd-click) | `⌘` held + click | Background `accent-subtle`; left 2px accent border; checkmark icon; ready to be context for next palette action. Multiple blocks can be attached. |
| Drilled | `Enter` or click → opens drawer | Background `elevated`; left 2px accent border; ARIA `aria-expanded="true"` |
| Updating (SSE arriving/mutating) | live event | 600ms green border flash via `@property --row-flash-progress`; no other change |
| New (just arrived) | appended to stream | Enter animation via `@starting-style`: fade + `translateY(8px → 0)`, 200ms |

---

## Accessibility

WAI-ARIA:
- `role` defaults to `article` (native for `<article>`).
- `aria-label` must summarize the request (status + account + model + latency).
- `tabindex="0"` makes the block reachable.
- Stream container uses `role="feed"` + `aria-busy="true"` while fetching, `aria-live="polite"` when SSE-active.

**Keyboard:**

| Key (block focused) | Effect |
|---|---|
| `Enter` | Open inspect drawer |
| `Space` | Toggle selection |
| `Cmd+Click` / `Ctrl+Click` (mouse) OR `A` (keyboard) | Toggle attach-for-context |
| `⌘C` / `Ctrl+C` | Copy request ID to clipboard + Toast |
| `⌘R` / `Ctrl+R` | Open replay drawer pre-filled with this request |
| `⌘L` / `Ctrl+L` | Open logs filtered to this request_id |
| `⌘K` / `Ctrl+K` | Open item-tier CommandPalette scoped to this request |
| `Esc` | Clear selection; close drilled drawer |

**Focus management:**
- Stream container maintains a focus-visible block.
- Arrow keys move focus to previous/next block.
- On SSE append: new block appears at top; focus DOES NOT move (preserves scroll position of reading user).

**Screen readers:**
- Each block announces via its `aria-label`.
- SSE-append events batch: every 5s, the feed container announces "N new requests"; individual block changes use the row-flash as the visual cue only.
- Status codes translated: "200" announces as "200 OK"; "429" as "429 rate limited"; "502" as "502 bad gateway".

---

## Motion

| Transition | Token | Notes |
|---|---|---|
| Enter (new block arrives) | `--dur-moderate` `--ease-default` | Fade + translateY(8→0) via `@starting-style` |
| Hover | instant | Background shift only |
| Selection | `--dur-quick` | Background + left border in one transition |
| Attach | `--dur-quick` | Left border + checkmark fade in |
| Update flash | `--dur-flash` (600ms) | `@property --row-flash-progress` 0→1→0; single-shot |
| Drill open | `--dur-moderate` | Drawer slides in; block stays focused with `data-state="drilled"` |

`prefers-reduced-motion: reduce` → enter animation collapsed to instant append; flash still changes color but without animation; drawer opens instantly.

---

## Composition

**Contains:** `StatusDot`, `BlockAccount` (a link), `BlockModel`/`BlockMethod`/`BlockStatus`/`BlockLatency` (all mono spans), `BlockMetadata` (grouping container), `BlockHints` (hover-visible only), `Keycap` (inside hints).

**Contained by:** `LiveRequestStream` feed (a `<ol role="feed">` with subgrid for column alignment).

**Paired with:** `Drawer` (inspect/replay drill-down), `Toast` (copy confirmation), `CommandPalette` item-tier (action sub-palette scoped to this request).

---

## Anti-patterns

- ❌ **Block without tabular-nums.** Latencies and token counts must scan vertically; `tabular-nums` is non-negotiable.
- ❌ **Block that re-renders on every SSE event.** Only the flash animation changes. Mutating the layout or shifting sibling blocks = CLS + confusion.
- ❌ **Hints visible without hover/focus.** Noise. Show on hover, focus, or selection.
- ❌ **Stat-grid replacement on the home page.** DESIGN_SYSTEM.md §12 explicitly: stat grids, charts, settings are auxiliary. The Block feed is the home.
- ❌ **Block with embedded interactive buttons.** The whole block is the focus target. Secondary actions come from the `⌘K` item palette, not inline buttons that fight for hit area.
- ❌ **Block content that changes width.** Column widths via subgrid stay aligned across all blocks; a cost going from $0.01 to $12.34 pushes downstream alignment if widths aren't reserved.
- ❌ **Shimmer animation on pending blocks.** See `skeleton.md` anti-patterns. Static skeleton only.
- ❌ **Permalink that points to a global log view with a URL fragment.** `/requests/{id}` is a first-class route, shareable, with the drill-down rendered via the same drawer opened inline. Test the permalink in a fresh incognito tab before merging.

---

## Differences from Warp Block

- Warp Blocks are terminal output (command + stdout/stderr + exit code). kiroxy Blocks are HTTP requests.
- Warp Blocks are user-typed; kiroxy Blocks are server-generated via SSE.
- Warp Blocks have inline content editing (rerun with edit). kiroxy Blocks are immutable; replay opens a new drawer with editable fields.
- Both share the "unit of context you can attach to an action" DNA.

---

## Reference

- **Warp** terminal — the Block-as-primitive original. `REFERENCE_GALLERY.md → Tier B → Warp`.
- **Vercel Geist** `Entity` — inspires the avatar+two-line layout.
- **Linear** row hierarchy — subtle left-border-for-state pattern.
- **kiroxy** `docs/VISION.md` §signature-thing — the mansion's front door.

---

## Example usage

**Healthy request, comfortable mode:**

(See "Markup template" above.)

**Failed request with attach-for-context highlighted:**

```html
<article class="kx-block" data-state="attached" data-intent="danger"
         data-request-id="req_01HKM9N2ZQ"
         aria-label="Request to claude-opus-4-7 via acct_01H8, 502 bad gateway in 340 milliseconds, attached for context"
         tabindex="0">
  <header class="kx-block__header">
    <span class="kx-status-dot" data-intent="danger" aria-hidden="true"></span>
    <a class="kx-block__account mono" href="/accounts/acct_01H8XJK9M2">acct_01H8</a>
    <span class="kx-block__model mono">claude-opus-4-7</span>
    <span class="kx-block__method mono">POST /v1/messages</span>
    <span class="kx-block__status mono">502</span>
    <span class="kx-block__latency mono" data-align="end">340ms</span>
    <svg class="kx-block__attached-mark" aria-hidden="true"><!-- check-circle accent --></svg>
  </header>
  <div class="kx-block__metadata">
    <span class="kx-block__error mono">upstream 403</span>
    <span class="kx-block__stream">non-stream</span>
    <time class="kx-block__ts mono" datetime="2026-05-13T11:41:58Z">11:41:58</time>
  </div>
</article>
```

**Compact mode (single-line block for dense view):**

```html
<article class="kx-block" data-density="compact" data-state="idle" data-intent="success"
         data-request-id="req_01HKM9N2ZQ" tabindex="0"
         aria-label="…">
  <span class="kx-status-dot" data-intent="success" aria-hidden="true"></span>
  <span class="mono">acct_01H8 claude-sonnet-4-5 POST /v1/messages 200 1.4s 1247/389 $0.012</span>
  <time class="mono" datetime="2026-05-13T11:42:18Z">11:42:18</time>
</article>
```
