# table

Dense, keyboard-navigable, subgrid-based data table. Core primitive for account pools, request logs, routes, metrics lists. Framework-agnostic; renders as native `<table>` with explicit roles where needed.

**Pattern inheritance:** Supabase table editor (direct-manipulation rows, right-side drawer drill-down), Tailscale machines table (search DSL over drop-down filters, single primary table per page), atopile LogViewer (subgrid alignment for log columns). See `research-v3/REFERENCE_GALLERY.md` Tier A/B.

**Design system citation:** `docs/DESIGN_SYSTEM.md` §1.2 (density), §4.2 (density modes), §7.4 (data-table interaction), §7.5 (search DSL).

---

## Anatomy

```
<Table>
  ├── <TableToolbar>                    ← filter + bulk-action slot
  │   ├── <InputField data-size="sm">  ← DSL filter
  │   └── <BulkActionBar>              ← appears when selection > 0
  ├── <TableContainer>                  ← overflow-x; sticky header; subgrid parent
  │   ├── <TableHeader>                 ← sticky top
  │   │   └── <TableColumnHeader>*     ← sortable; click toggles asc/desc/unsorted
  │   └── <TableBody>
  │       └── <TableRow>*               ← data-state="idle|selected|updating|drilled"
  │           ├── <TableCheckbox>       ← bulk selection
  │           └── <TableCell>*          ← mono for IDs/timestamps/numbers
  ├── <TableFooter>                     ← pagination OR "Showing N of M"
  └── <TableEmptyState>                 ← per-filter empty; see empty-state.md
</Table>
```

Markup template:

```html
<div class="kx-table" data-density="comfortable" data-state="ready">
  <div class="kx-table__toolbar">
    <!-- filter input + bulk action bar slot -->
  </div>
  <div class="kx-table__container" role="region" aria-labelledby="tbl-caption" tabindex="0">
    <table role="grid" aria-label="Accounts">
      <caption id="tbl-caption" class="visually-hidden">Accounts pool</caption>
      <thead>
        <tr>
          <th scope="col" class="kx-table__checkbox-col">
            <input type="checkbox" aria-label="Select all visible rows" class="kx-checkbox">
          </th>
          <th scope="col" aria-sort="none">
            <button type="button" class="kx-table__sort" data-column="id">
              ID <svg aria-hidden="true"><!-- sort glyph --></svg>
            </button>
          </th>
          <th scope="col" aria-sort="descending">…</th>
          <!-- … -->
        </tr>
      </thead>
      <tbody>
        <tr data-state="idle" tabindex="0" aria-selected="false">
          <td><input type="checkbox" class="kx-checkbox" aria-label="Select row acct-2"></td>
          <td class="mono">acct_01H8XJK9M2</td>
          <td>…</td>
        </tr>
      </tbody>
    </table>
  </div>
  <div class="kx-table__footer">Showing 12 of 12</div>
</div>
```

---

## API

| Attribute (wrapper) | Values | Default | Description |
|---|---|---|---|
| `data-density` | `comfortable` \| `compact` | (inherited from root) | Row height + cell padding |
| `data-state` | `ready` \| `loading` \| `empty` \| `error` | `ready` | Drives body content |
| `data-drill-target` | CSS selector | — | When present, rows open a `Drawer` at that selector on `Enter` |

| Attribute (row) | Values | Description |
|---|---|---|
| `data-state` | `idle` \| `hover` \| `selected` \| `updating` \| `drilled` | Row lifecycle |
| `aria-selected` | `true`/`false` | Mirrors checkbox state |
| `aria-rowindex` | number | Required when virtualized |

| Attribute (column header `<th>`) | Values | Description |
|---|---|---|
| `aria-sort` | `none` \| `ascending` \| `descending` | Required on sortable columns |
| `data-column` | string | Machine column name for sort/filter integration |
| `data-align` | `start` \| `end` \| `center` | Cell alignment (numbers go `end`) |

---

## Variants

- **`default`** — standard dense table; 36px rows (comfortable) / 28px (compact).
- **`virtualized`** — set `data-virtualized="true"` when > 200 rows. Renders only visible rows + buffer; requires `aria-rowindex` and `aria-rowcount` on `<table>`.
- **`grouped`** — rows grouped by `data-group`; header rows are `<tr role="rowgroup">` with `<th scope="colgroup">` — Supabase pattern.
- **`selectable` | `readonly`** — presence of checkbox column. `readonly` variant hides the column entirely.

---

## States

| State | Trigger | Visual |
|---|---|---|
| Row hover | mouse-over | Background `--color-elevated`; no transform |
| Row selected | checkbox OR `Space` on focused row | Accent-subtle background + left accent-border 2px; `aria-selected="true"` |
| Row updating (SSE) | row arrives/mutates via stream | 600ms green border flash via `@property --row-flash-progress` |
| Row drilled | `Enter` / click opens drawer | Background stays `elevated`; subtle left accent border; drawer URL updates |
| Row loading | per-row optimistic action | Inline `LoadingDots` in the first cell of the row |
| Column sorted | `aria-sort` not `none` | Sort glyph filled; chevron up/down |
| Table loading (initial) | first render | Skeleton rows matching the expected column layout |
| Table empty | filtered result or first-time | `TableEmptyState`; no header row |
| Table error | backend unreachable | Inline banner above the header |

---

## Accessibility

WAI-ARIA: [Grid Pattern](https://www.w3.org/WAI/ARIA/apg/patterns/grid/) for interactive tables; [Table Pattern](https://www.w3.org/WAI/ARIA/apg/patterns/table/) for read-only.

Use `role="grid"` when rows are navigable/selectable; keep default `<table>` semantics when read-only. kiroxy's account/request tables are always grids.

**Keyboard interactions:**

| Key | Effect |
|---|---|
| `Tab` | Focus enters the table container; subsequent `Tab` leaves |
| `ArrowDown` / `ArrowUp` / `j` / `k` | Move row focus |
| `ArrowLeft` / `ArrowRight` | Move cell focus within a row (in `grid` mode) |
| `Home` / `End` | First / last row |
| `PageDown` / `PageUp` | Jump 10 rows |
| `Space` | Toggle selection of focused row |
| `Enter` | Open drill-down drawer for focused row |
| `Cmd+Enter` / `Ctrl+Enter` | Open drill-down in full page (URL navigation) |
| `Cmd+A` / `Ctrl+A` | Select all visible rows |
| `Escape` | Clear selection; if drawer open, close drawer (delegated to `Drawer`) |
| `/` (when table in focus) | Focus the DSL filter input |

**Focus management:**
- Roving tabindex on rows: one row has `tabindex="0"`, rest `tabindex="-1"`.
- Column-header buttons are in their own tab stop above the grid.
- Drawer open does NOT blur the row; the row retains `data-state="drilled"` + returns focus when drawer closes.

**Screen readers:**
- `<caption>` required (can be visually hidden) describing the table purpose.
- Column headers use `<th scope="col">`; row headers if any use `<th scope="row">`.
- `aria-rowcount` / `aria-rowindex` required in virtualized mode.
- Live-update announcements batched: every 5 seconds a polite summary ("3 new requests since last summary") — not per-row chatter.

---

## Motion

| Transition | Token | Notes |
|---|---|---|
| Row hover | instant | Background-only shift |
| Row selection | `--dur-quick` | Background + left border together |
| Row SSE flash | `--dur-flash` (600ms) | `@property --row-flash-progress` 0→1→0; single-shot |
| Sort toggle | `--dur-quick` | Chevron rotation |
| Virtualized scroll | native | No custom momentum |
| Empty-state fade-in | `--dur-quick` | After filter returns zero |

---

## Composition

**Contains:** `TableToolbar`, `TableHeader`, `TableBody`, `TableRow`, `TableCell`, `TableCheckbox`, `TableColumnHeader`, `TableEmptyState`, `BulkActionBar`.

**Contained by:** A primary view route (Accounts, Requests, Routes, Logs). Never inside a `Dialog` or `Drawer` — tables are page-level.

**Paired with:** `Drawer` (row drill-down), `CommandPalette` (search triggered by `/`), `Toast` (bulk-action confirmation).

---

## Anti-patterns

- ❌ **Modal for row drill-down.** Use `Drawer` — list context must stay visible (Supabase pattern).
- ❌ **Row-level toolbars that appear on hover.** Poor for keyboard users. Put row actions in the `Drawer` OR the `CommandPalette` `⌘K` scope.
- ❌ **Pagination for < 500 rows.** Virtualize instead — operators want Cmd-F.
- ❌ **Sortable columns without `aria-sort`.** Screen readers can't announce sort state.
- ❌ **Filter via drop-downs instead of search DSL.** DESIGN_SYSTEM.md §7.5 specifies DSL; drop-downs lose compositional filtering.
- ❌ **Empty-state card with a big CTA button.** DESIGN_SYSTEM.md §12 / §7 — empty state shows the CLI command that changes state (fly.io pattern), not a primary button.
- ❌ **Numbers in sans-font cells.** Every number (ID, timestamp, count, latency) in `mono` with `tabular-nums`. Scannable columns matter.
- ❌ **Row background changing on every SSE update.** Use the single-shot border flash via `@property`; never re-render the row's surface color on update.

---

## Differences from native `<table>`

- Interactive tables upgrade to `role="grid"` semantics for keyboard navigation.
- Sticky header + overflow container; raw `<table>` can't scroll horizontally without wrapping.
- Subgrid for column alignment across sticky-header + virtualized body (Safari 16+, Chrome 117+ — see `REFERENCE_GALLERY.md` Tier D).
- Row `data-state` + `@property --row-flash-progress` for SSE updates — native has no comparable mechanism.

---

## Reference

- **Supabase** table editor — direct-manipulation rows, keyboard navigation, right drawer on row click.
- **Tailscale** machines table — search DSL, semantic tokens, one table per page.
- **atopile LogViewer** — CSS subgrid column alignment (cited in `REFERENCE_GALLERY.md` → Tier D Subgrid).

---

## Example usage

**Accounts table with DSL filter + bulk-action:**

```html
<div class="kx-table" data-density="comfortable" data-state="ready">

  <div class="kx-table__toolbar">
    <div class="kx-field" data-size="sm">
      <label class="visually-hidden" for="accts-filter">Filter accounts</label>
      <div class="kx-field__row">
        <svg class="kx-field__leading" aria-hidden="true"><!-- filter --></svg>
        <input id="accts-filter" type="search" class="kx-field__input mono"
               placeholder="status:healthy tier:pro">
        <kbd class="kx-keycap" aria-hidden="true">/</kbd>
      </div>
    </div>
    <div class="kx-bulk" hidden>
      <span class="kx-bulk__count">0 selected</span>
      <button type="button" class="kx-button" data-variant="secondary" data-size="sm">Refresh</button>
      <button type="button" class="kx-button" data-variant="danger"    data-size="sm">Drop</button>
    </div>
  </div>

  <div class="kx-table__container" role="region" aria-labelledby="tc" tabindex="0">
    <table role="grid" aria-rowcount="12" aria-label="Accounts">
      <caption id="tc" class="visually-hidden">Accounts pool, 12 rows</caption>
      <thead>
        <tr>
          <th scope="col" class="kx-table__checkbox-col">
            <input type="checkbox" aria-label="Select all visible rows" class="kx-checkbox">
          </th>
          <th scope="col" aria-sort="none">
            <button class="kx-table__sort" data-column="id">ID <svg aria-hidden="true"><!-- --></svg></button>
          </th>
          <th scope="col" aria-sort="none">
            <button class="kx-table__sort" data-column="status">Status</button>
          </th>
          <th scope="col" aria-sort="descending" data-align="end">
            <button class="kx-table__sort" data-column="req">Requests</button>
          </th>
        </tr>
      </thead>
      <tbody>
        <tr data-state="idle" tabindex="0" aria-rowindex="1">
          <td><input type="checkbox" class="kx-checkbox" aria-label="Select acct_01H8"></td>
          <td class="mono">acct_01H8XJK9M2</td>
          <td>
            <span class="kx-status-pill" data-intent="success">
              <span class="kx-status-dot" aria-hidden="true"></span> Healthy
            </span>
          </td>
          <td class="mono" data-align="end">3,412</td>
        </tr>
        <!-- … -->
      </tbody>
    </table>
  </div>

  <div class="kx-table__footer">Showing 12 of 12</div>
</div>
```
