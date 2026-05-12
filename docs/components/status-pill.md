# status-pill

Text + colored dot used in row status columns and panel headers. Not clickable; read-only indicator.

**Pattern inheritance:** hj01857655/kiro-account-manager (4-color status taxonomy beyond 3-light), Vercel Geist `Status Dot`, Linear's tiny status glyphs.

**Design system citation:** `docs/DESIGN_SYSTEM.md` §8 (primitives list), §9 (color-is-never-the-only-signal).

---

## Anatomy

```
<StatusPill>
  ├── <StatusDot>         ← 8px circle in semantic color
  └── <StatusLabel>       ← text (always present; never color-only)
</StatusPill>
```

Markup template:

```html
<span class="kx-status-pill" data-intent="success">
  <span class="kx-status-dot" aria-hidden="true"></span>
  <span class="kx-status-pill__label">Healthy</span>
</span>
```

---

## API

| Attribute | Values | Default | Description |
|---|---|---|---|
| `data-intent` | `success` \| `warning` \| `danger` \| `info` \| `neutral` \| `cooldown` | `neutral` | Drives dot color + subtle bg |
| `data-size` | `sm` (18px tall) \| `md` (22px) | `md` | Compact table use vs comfortable |
| `data-filled` | `true` \| `false` | `false` | Filled pill (subtle bg) vs dot-only |
| `role` | `status` | `status` | SR semantics |

---

## Variants (taxonomy)

Per hj01857655 dossier (`REFERENCE_GALLERY.md`), 4+ states — not 3-light.

| Intent | Token | Label example | Use |
|---|---|---|---|
| `success` | `--color-success` | "Healthy" | Account ready, request succeeded |
| `cooldown` | `--color-warning` | "Cooldown 1m 45s" | Rate-limit backoff, NOT banned |
| `danger` | `--color-danger` | "Failed" | Auth error, upstream 5xx, revoked |
| `warning` | `--color-warning` | "Capped" | Quota limit reached (distinct from cooldown) |
| `info` | `--color-info` | "Refreshing" | In-flight operation |
| `neutral` | `--color-text-dim` | "Disabled" | Explicitly disabled, not an error |

**`cooldown` and `danger` are distinct** — the kiro-account-manager dossier called out that "capped" (quota hit) vs "banned" (account-level) should not collapse into one "unavailable" state. Apply the same to kiroxy: cooldown + capped + danger each get their own intent.

---

## States

Static — no hover, no focus, no disabled. Pills mirror data; interaction belongs on the containing row.

---

## Accessibility

- `role="status"` on the pill.
- `aria-hidden="true"` on the dot span (decorative).
- The label text IS the announcement; dot provides visual reinforcement.
- Never use color alone — label is always present. Colorblind users read "Healthy" / "Failed" exactly the same as sighted users.

---

## Motion

No inherent motion. If the intent changes (e.g. success → cooldown), the CONTAINING row plays the `@property --row-flash-progress` pulse; the pill content swaps atomically.

---

## Composition

**Contains:** `StatusDot`, label span.
**Contained by:** `TableCell`, `BlockHeader`, `PanelHeader`, `Drawer` body, `CardHeader`.
**Paired with:** `RelativeTimeCard` (for "Cooldown 1m 45s" dynamic countdown), `Tooltip` (for long descriptions attached to the pill).

---

## Anti-patterns

- ❌ **Pill without label** ("just the dot"). Fails 1.4.1 Use of Color.
- ❌ **Pill as a button.** Use `Button data-variant="ghost"` styled like a pill if clickable.
- ❌ **Collapsing cooldown + danger + capped into one "unavailable" intent.** Users need to diagnose. 4+ distinct intents.
- ❌ **Pill background fill for success** on light mode. Light warning/success fills reduce contrast below AA; use dot-only (`data-filled="false"`) unless the pill is on `--color-surface`.
- ❌ **Pulsing the dot.** Motion is row-level, not dot-level.

---

## Reference

- **Vercel Geist** `Status Dot` — the minimal version.
- **hj01857655/kiro-account-manager** dossier — 4-color taxonomy insight.
- **Linear** workflow states — bespoke glyphs per state (aspirational, not copied).

---

## Example usage

**Healthy, comfortable size:**
```html
<span class="kx-status-pill" data-intent="success" data-size="md">
  <span class="kx-status-dot" aria-hidden="true"></span>
  <span class="kx-status-pill__label">Healthy</span>
</span>
```

**Cooldown with live countdown:**
```html
<span class="kx-status-pill" data-intent="cooldown" data-size="sm" role="status" aria-live="polite">
  <span class="kx-status-dot" aria-hidden="true"></span>
  <span class="kx-status-pill__label">Cooldown</span>
  <time class="mono" datetime="PT105S">1m 45s</time>
</span>
```

**Refreshing (filled subtle bg for emphasis):**
```html
<span class="kx-status-pill" data-intent="info" data-filled="true" role="status">
  <span class="kx-status-dot" aria-hidden="true"></span>
  <span class="kx-status-pill__label">Refreshing…</span>
</span>
```
