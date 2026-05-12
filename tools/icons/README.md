# kiroxy icon set

Hand-authored outline SVGs inspired by [Tabler Icons](https://tabler.io/icons)
(MIT). Each file carries a header comment crediting Tabler as the
design-language reference.

**Source of truth:** `docs/ICONOGRAPHY.md`.
**Typed manifest:** `icons.ts`.
**CSS helper:** `.kx-icon` defined in `internal/server/assets/tokens/tokens.css`.

## Shipped

These six SVGs are committed in Part 2 so Track 3 can render the dashboard's
navigation + status pills + LiveRequestStream block hints without blocking:

- `status-healthy.svg`
- `refresh.svg`
- `close.svg`
- `chevron-right.svg`
- `info.svg`
- `stream.svg`

## Deferred

The remaining ~21 icons are specified in `docs/ICONOGRAPHY.md` with locked
names, purposes, and file slots. Any of them can be sourced at implementation
time without returning to the design document.

## Adding an icon

See `docs/ICONOGRAPHY.md` → "How to add a new icon". Short version:

1. Author at 24×24 on a 1px grid, 1.5px stroke, round caps + joins, no fill.
2. Optimize with `svgo`.
3. Add header: `<!-- kiroxy — inspired by Tabler Icons (MIT). -->`.
4. Commit alongside an update to `icons.ts` flipping `shipped: false → true`.
