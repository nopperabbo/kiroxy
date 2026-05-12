# copyable-value

Mono-formatted ID/token/URL with an inline copy button. Primary home for every long opaque string kiroxy surfaces: `request_id`, `account_id`, `refresh_token` prefix, `/v1/messages` endpoint URL, `opencode-config` JSON snippet.

**Pattern inheritance:** Stripe's `⌘+I` copy-ID idiom (the copy affordance lives next to the value, not hidden in a menu); Vercel Geist `Snippet`; Warp's permalinkable block pattern.

**Design system citation:** `docs/DESIGN_SYSTEM.md` §3.3 (mono as data layer), §11 (error/copy-paste commands), `docs/VISION.md` §signature-thing (per-request permalinks).

---

## Anatomy

```
<CopyableValue>
  ├── <Value>             ← mono text; `user-select: all` for double-click-select
  └── <CopyButton>        ← ghost Button; 14-16px copy icon; aria-label
</CopyableValue>
```

Markup template:

```html
<span class="kx-copyable" data-copied="false">
  <code class="mono kx-copyable__value" data-value="acct_01H8XJK9M2">acct_01H8XJK9M2</code>
  <button type="button" class="kx-button" data-variant="ghost" data-size="sm"
          aria-label="Copy account ID">
    <svg aria-hidden="true" width="14" height="14"><!-- copy --></svg>
  </button>
</span>
```

---

## API

| Attribute | Values | Default | Description |
|---|---|---|---|
| `data-copied` | `false` \| `true` | `false` | Flips to `true` for 1.5s after copy; drives icon swap |
| `data-truncate` | `none` \| `start` \| `mid` \| `end` | `none` | Ellipsize display; `data-value` carries full value |
| `data-value` | string | required | The actual value copied |
| `data-size` | `sm` \| `md` | `md` | Matches row density |

---

## Variants

- **`id`** — short opaque string (`acct_…`, `req_…`). Not truncated; `user-select: all`.
- **`url`** — full URL; may truncate with `data-truncate="mid"`; hover or focus reveals the full URL via a `Tooltip`.
- **`token`** — sensitive token; always truncated `data-truncate="mid"`; copy copies the full value but display is masked like `sk-kx-…9fA2`. Visually signals "treat me as secret."
- **`snippet`** — multi-line code block (e.g. `kiroxy add-account --…`). Different primitive — see `Snippet`; CopyableValue is single-line.

---

## States

| State | Trigger | Visual |
|---|---|---|
| Idle | default | Mono value + ghost copy button |
| Hover | pointer over | Copy button background `--color-elevated` |
| Copying | click/Enter on copy button | Icon swaps to `check`; button `aria-label="Copied"`; Toast fires |
| Copied | for 1.5s | `data-copied="true"`; check icon visible |
| Reset | after 1.5s | `data-copied="false"`; copy icon returns |
| Disabled | `data-value=""` | Button hidden; value still visible |

---

## Accessibility

- Value uses `<code class="mono">` — semantically marks the text as code.
- Copy button carries `aria-label` specific to the value ("Copy account ID", not generic "Copy").
- On copy, SR announces the `Toast` — no `aria-live` on the button itself (double-announcement).
- `user-select: all` on the value enables drag-select without hitting the button; native `Cmd+C` works as a fallback.

**Keyboard:**
- Tab reaches the copy button (value span is not focusable).
- `Enter` / `Space` on the button copies.
- When value span has focus (via outer component's logic), `Cmd+C` copies natively.

---

## Motion

| Transition | Token | Notes |
|---|---|---|
| Icon swap (copy → check) | `--dur-quick` | Crossfade |
| Check → copy reset | `--dur-quick` | 1.5s delay, then crossfade back |
| Toast entrance | `--dur-moderate` | Per `toast.md` |

---

## Composition

**Contains:** `<code>` value + `Button` copy.
**Contained by:** `Entity` (Geist pattern), `TableCell`, `BlockHeader`, `Drawer` fields, `Dialog` body.
**Paired with:** `Toast` (copy confirmation), `Tooltip` (truncated URL peek).

---

## Anti-patterns

- ❌ **Copy button hidden behind a more-menu.** Stripe exposes it inline; kiroxy does too.
- ❌ **Copy silently** with no Toast. Users lose confidence their action took effect.
- ❌ **`user-select: none` on the value.** Breaks native Cmd+C for power users.
- ❌ **Showing full token without masking.** Tokens in plaintext violate sec hygiene when operators screen-share.
- ❌ **Copying whitespace or formatting characters.** `data-value` is the canonical source of truth; display may have ellipsis — copy must not include it.
- ❌ **No `aria-label`** on icon-only copy button.

---

## Reference

- **Stripe** `⌘+I` copy-ID pattern.
- **Vercel Geist** `Snippet` primitive.
- **kiroxy** `docs/VISION.md` — every request gets a permalink; CopyableValue is how the permalink is surfaced.

---

## Example usage

**Account ID in a table cell:**
```html
<td>
  <span class="kx-copyable" data-copied="false">
    <code class="mono" data-value="acct_01H8XJK9M2">acct_01H8XJK9M2</code>
    <button type="button" class="kx-button" data-variant="ghost" data-size="sm" aria-label="Copy acct_01H8XJK9M2">
      <svg aria-hidden="true" width="14" height="14"><!-- copy --></svg>
    </button>
  </span>
</td>
```

**Truncated URL with mid-ellipsis + tooltip:**
```html
<span class="kx-copyable" data-truncate="mid">
  <code class="mono" data-value="https://runtime.us-east-1.kiro.dev/generateAssistantResponse">
    https://runtime.us-east-1.kiro.dev/generate…sponse
  </code>
  <button type="button" class="kx-button" data-variant="ghost" data-size="sm"
          aria-label="Copy upstream URL"
          aria-describedby="u-full">
    <svg aria-hidden="true"><!-- copy --></svg>
  </button>
  <div id="u-full" popover="manual" anchor="…" role="tooltip">
    https://runtime.us-east-1.kiro.dev/generateAssistantResponse
  </div>
</span>
```

**Masked token:**
```html
<span class="kx-copyable" data-truncate="mid" data-variant="token">
  <code class="mono" data-value="sk-kx-9vEj2pQXmRzBTY09fA2">sk-kx-…9fA2</code>
  <button type="button" class="kx-button" data-variant="ghost" data-size="sm" aria-label="Copy API key">
    <svg aria-hidden="true"><!-- copy --></svg>
  </button>
</span>
```
