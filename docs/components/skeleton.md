# skeleton

Placeholder shape shown while data is loading. Component knows its own dimensions. No spinner, no shimmer animation — DESIGN_SYSTEM.md §13 explicitly forbids spinners.

**Pattern inheritance:** Vercel Geist `Skeleton`, GitHub Primer skeleton loaders. See `research-v3/REFERENCE_GALLERY.md` Tier C.

**Design system citation:** `docs/DESIGN_SYSTEM.md` §13 (no spinners), §8 (primitive rules).

---

## Anatomy

Skeletons mirror the shape of the content they replace. Every primary component (Table, Card, Sparkline, StatusPill) ships a matching `Skeleton` sub-variant.

```
<Skeleton>                              ← a single block OR a composite
  ├── [for table]   <SkeletonRow>       ← replicates N rows of column widths
  ├── [for card]    <SkeletonCard>      ← replicates the header/body/footer shape
  ├── [for chart]   <SkeletonSparkline> ← flat 60-point baseline + dim hash
  └── [generic]     <SkeletonBlock>     ← solid `--color-elevated` rectangle
</Skeleton>
```

Markup template (generic block):

```html
<div class="kx-skeleton" role="status" aria-label="Loading content"
     data-shape="block" style="width: 120px; height: 14px;"></div>
```

Markup template (table row):

```html
<tr class="kx-skeleton-row" aria-hidden="true">
  <td><div class="kx-skeleton" data-shape="block" style="width: 16px; height: 16px;"></div></td>
  <td><div class="kx-skeleton" data-shape="block" style="width: 160px; height: 14px;"></div></td>
  <td><div class="kx-skeleton" data-shape="block" style="width: 72px;  height: 14px;"></div></td>
</tr>
```

---

## API

| Attribute | Values | Default | Description |
|---|---|---|---|
| `data-shape` | `block` \| `circle` \| `text` \| `row` \| `card` \| `sparkline` | `block` | Shape preset |
| `data-size` | `sm` \| `md` \| `lg` | inherited | Sizes text skeletons to 12/14/16 height |
| `role` | `status` OR `presentation` | — | `status` on the wrapping container announcing "Loading {section}"; child shapes are `presentation` |
| `aria-label` | string | `"Loading"` | Required on the wrapping status element |
| `aria-busy` | `true` | required | On the element the skeleton is replacing |

---

## Variants

- **`block`** — solid rectangle; generic.
- **`circle`** — `border-radius: var(--radius-full)` for avatar placeholders.
- **`text`** — block with 14px height + randomized widths across siblings.
- **`row`** — matches a table-row layout; one `<div>` per column.
- **`card`** — composite: header line + body lines + optional footer.
- **`sparkline`** — baseline + dim hash pattern for chart placeholders.

---

## States

| State | Trigger | Visual |
|---|---|---|
| Loading | `aria-busy="true"` on parent | Skeleton visible; `--color-elevated` fill |
| Resolved | data arrives | Skeleton unmounts; content fades in via `@starting-style` (120ms) |
| Error | fetch fails | Skeleton replaced with inline error; see `empty-state.md` for full-section error |

---

## Accessibility

WAI-ARIA:
- Parent container (the element the skeleton replaces) carries `aria-busy="true"`.
- The skeleton wrapper carries `role="status"` + `aria-label="Loading {section name}"`.
- Individual shape elements carry `aria-hidden="true"` — they're decorative; the `status` role handles announcement.

**Screen readers:**
- "Loading accounts pool" announced once when the skeleton mounts.
- On resolve, nothing announced — the new content takes over; subsequent SSE updates use the table's live region.

**Keyboard:**
- Skeletons are NOT focusable. Tab skips them.
- The parent container retains its tab-stop; focus returns to it after skeleton unmount if the user was focused there.

---

## Motion

**No shimmer, no pulse, no gradient sweep.** DESIGN_SYSTEM.md §5 prohibits decorative motion.

| Transition | Token | Notes |
|---|---|---|
| Mount (on parent busy) | instant | No fade; skeleton is not "pretty" — it's a placeholder |
| Unmount (data arrives) | `--dur-quick` | Content fades in via `@starting-style`; skeleton removal is instant |
| `prefers-reduced-motion` | same | Already minimal |

If a Track 3 implementation adds a shimmer: **reject the PR**. A shimmer is decorative motion (§1 tenet 4 violation) — it tells the user "I know nothing more than 1ms ago," which is true but redundant to the visible skeleton block itself.

---

## Composition

**Contains:** Nothing user-relevant; children are decorative.

**Contained by:** The component whose data is loading. `SkeletonRow` inside `TableBody`, `SkeletonCard` inside a card grid, `SkeletonSparkline` inside a metrics panel.

**Paired with:** `aria-busy` on parent; `EmptyState` (if load resolves to zero); `Toast` (if load fails).

---

## Anti-patterns

- ❌ **Spinner instead of skeleton.** DESIGN_SYSTEM.md §13. The operator's eye is drawn to motion — use it only for things that are actually progressing (e.g. the `@property` row-flash).
- ❌ **Shimmer gradient animation** (the "AI dashboard template" signature). Static fill only.
- ❌ **Skeleton dimensions that don't match the real content.** Causes CLS (content layout shift) when data arrives — DESIGN_SYSTEM.md IMPLEMENTATION_RUBRIC demands CLS = 0.
- ❌ **Skeleton without `aria-busy` on parent.** SR users don't know the page is loading.
- ❌ **Indefinite skeleton.** Timeout at 10s → switch to error empty-state.
- ❌ **Skeleton for <100ms loads.** Flashes on/off and looks broken. Only render skeleton if load takes > 100ms.

---

## Differences from Spinner

- Skeleton occupies the final layout size; spinner does not. Skeleton prevents CLS; spinner causes it.
- Skeleton is aware of shape; spinner is generic.
- Skeleton is static; spinner is animated. kiroxy's motion-discipline prefers static.

---

## Reference

- **Vercel Geist** `Skeleton` primitive — shape-matched placeholders, no shimmer.
- **GitHub Primer** skeleton loader — animation opt-in only.
- **REFERENCE_GALLERY.md → hexos** — the cautionary tale of `animate-stagger` + `animate-float` + `animate-bounce-in` stacked (what NOT to do).

---

## Example usage

**Table loading skeleton (5 placeholder rows):**

```html
<tbody role="status" aria-label="Loading accounts" aria-busy="true">
  <tr class="kx-skeleton-row" aria-hidden="true">
    <td><div class="kx-skeleton" data-shape="block" style="width: 16px; height: 16px;"></div></td>
    <td><div class="kx-skeleton" data-shape="block" style="width: 160px; height: 14px;"></div></td>
    <td><div class="kx-skeleton" data-shape="block" style="width: 72px;  height: 14px;"></div></td>
    <td><div class="kx-skeleton" data-shape="block" style="width: 48px;  height: 14px;"></div></td>
  </tr>
  <!-- 4 more rows with slight width variation for a natural look -->
</tbody>
```

**Sparkline skeleton:**

```html
<div class="kx-skeleton" data-shape="sparkline" role="status" aria-label="Loading chart"
     style="width: 100%; height: 48px;"></div>
```
