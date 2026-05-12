# empty-state

Centered prose + **copyable CLI command** shown when a section has no data. kiroxy's signature empty-state pattern comes from fly.io (Tier A): the CTA is a CLI command, NOT a big primary button.

**Pattern inheritance:** fly.io CLI-first empty states, LibreChat's restraint, Stripe's onboarding copy. See `research-v3/REFERENCE_GALLERY.md` Tier A + Tier E.

**Design system citation:** `docs/DESIGN_SYSTEM.md` §8 (EmptyState primitive), §11 (docs/empty-state copy tone), `docs/VISION.md` §Product-vibes-sound.

---

## Anatomy

```
<EmptyState>
  ├── [optional] <EmptyIcon>              ← 32x32 muted glyph
  ├── <EmptyTitle>                         ← 1 sentence, no exclamations
  ├── <EmptyDescription>                   ← 1-2 sentences explaining why empty
  ├── <EmptyAction>                         ← copyable CLI command (Snippet)
  └── [optional] <EmptyDocLink>            ← "Learn more →" link
</EmptyState>
```

Markup template:

```html
<div class="kx-empty" role="status" aria-live="polite">
  <svg class="kx-empty__icon" aria-hidden="true" width="32" height="32"><!-- stream icon --></svg>
  <h2 class="kx-empty__title">No accounts imported yet</h2>
  <p class="kx-empty__desc">
    kiroxy needs at least one Kiro account to proxy requests. Paste your
    refresh token:
  </p>
  <div class="kx-snippet" data-copyable="true">
    <pre class="mono"><code>kiroxy add-account --label=my-account --refresh-token=&lt;your-refresh-token&gt;</code></pre>
    <button type="button" class="kx-button" data-variant="ghost" data-size="sm"
            aria-label="Copy command">
      <svg aria-hidden="true"><!-- copy --></svg>
    </button>
  </div>
  <a class="kx-link" href="/docs/accounts">Read the account guide →</a>
</div>
```

---

## API

| Attribute | Values | Default | Description |
|---|---|---|---|
| `data-variant` | `first-time` \| `filtered` \| `error` \| `all-cooldown` | `first-time` | Drives icon + tone |
| `data-size` | `sm` (drawer) \| `md` (page) \| `lg` (route) | `md` | Spacing + icon size |
| `role` | `status` OR `alert` | `status` | `alert` only for `error` variant |
| `aria-live` | `polite` OR `assertive` | matches role | |

---

## Variants

- **`first-time`** — never had data; show the CLI command that creates it.
- **`filtered`** — data exists but filter returned zero; show the filter query + "Clear filter" ghost button. NO CLI command here.
- **`error`** — backend unreachable or auth failed; show the error phrasing + remediation command + "Retry" ghost button.
- **`all-cooldown`** — pool-specific: all accounts are in cooldown; show "Wait {N} seconds" + countdown (via `RelativeTimeCard`) + `Refresh pool` command.

---

## States

| State | Trigger | Visual |
|---|---|---|
| Idle | rendered | Centered content; icon `--color-text-dim`; title `--color-text-bright`; description `--color-text-default`; snippet with copy button |
| Copied | user clicked copy | Snippet `aria-live="polite"` announces "Command copied"; button shows check icon for 1.5s |
| Dismissed | filter cleared or data arrives | EmptyState unmounts; content fades in via `@starting-style` |

---

## Accessibility

WAI-ARIA:
- `role="status"` for first-time/filtered/all-cooldown; `role="alert"` for `error` variant.
- `aria-live` matches role (polite/assertive).
- The CLI command snippet is a `<pre><code>` block — already semantic.
- Copy button has `aria-label="Copy command"`.

**Keyboard:**
- Tab order: EmptyTitle is not focusable (prose); EmptyAction copy button IS; "Retry" or "Clear filter" button IS; doc link IS.
- `Enter` on focused Snippet copy button copies + announces.
- `Cmd+C` / `Ctrl+C` while the code block has focus also copies (native select + copy still works).

**Screen readers:**
- Announces title + description on mount.
- `aria-live="polite"` ensures later updates (e.g. cooldown countdown) don't spam.

---

## Motion

| Transition | Token | Notes |
|---|---|---|
| Mount (first-time) | `--dur-quick` | Fade only; no decorative entrance |
| Mount (filtered, after filter change) | `--dur-quick` | Fade |
| Unmount (data arrives) | `--dur-quick` | Fade; new content fades in above via `@starting-style` |
| Copy confirmation | `--dur-quick` | Icon swap to check; 1.5s revert |

---

## Composition

**Contains:** Icon (optional), title, description, CLI Snippet (primary), ghost Button (optional), doc link (optional).

**Contained by:** Any route body (page-level), `Drawer` body (section-level), `TableBody` (table-level — in place of rows).

**Paired with:** `Snippet` (primary action), `Button` (secondary Retry/Clear), `RelativeTimeCard` (for all-cooldown countdown).

---

## Anti-patterns

- ❌ **Big primary button as the CTA.** fly.io pattern: CLI command, not button. Operator is comfortable with a terminal; a Button here says "I don't think you are."
- ❌ **Apology copy.** "Sorry, no data yet!" is banned. DESIGN_SYSTEM.md §11 — voice is dry, technical, self-aware.
- ❌ **Emoji.** No 🎉 or ✨ or 🚀.
- ❌ **Large hero illustration.** kiroxy is not a consumer SaaS. An icon (24-32px) suffices.
- ❌ **Empty state without a remediation path.** Every empty state tells the user what to do next. "No data" alone is hostile.
- ❌ **Same empty state for first-time vs filtered.** They're semantically different. First-time = teach; filtered = suggest clear.
- ❌ **Empty state that doesn't include a doc link.** Users who want more context should have a path. The link is optional but strongly recommended.

---

## Reference

- **fly.io** — CLI-command-as-CTA pattern. See `REFERENCE_GALLERY.md → Tier A → fly.io`.
- **Supabase** table-editor empty state — inspires the filtered variant.
- **LibreChat** — restrained copy, no emoji.

---

## Example usage

**First-time (no accounts):**

```html
<div class="kx-empty" role="status" data-variant="first-time">
  <svg class="kx-empty__icon" aria-hidden="true"><!-- server --></svg>
  <h2 class="kx-empty__title">No accounts imported yet</h2>
  <p class="kx-empty__desc">
    kiroxy needs at least one Kiro account to proxy requests. Paste your
    refresh token:
  </p>
  <div class="kx-snippet"><pre class="mono"><code>kiroxy add-account --label=my-account --refresh-token=&lt;your-refresh-token&gt;</code></pre>
    <button type="button" class="kx-button" data-variant="ghost" data-size="sm" aria-label="Copy"><svg aria-hidden="true"><!-- copy --></svg></button>
  </div>
  <a class="kx-link" href="/docs/accounts">Read the account guide →</a>
</div>
```

**Filtered (zero results for current DSL):**

```html
<div class="kx-empty" role="status" data-variant="filtered">
  <svg class="kx-empty__icon" aria-hidden="true"><!-- filter --></svg>
  <h2 class="kx-empty__title">No requests match this filter</h2>
  <p class="kx-empty__desc">
    Filter: <code class="mono">model:claude-opus-4-7 status:429</code>.
    Try broadening the status or model.
  </p>
  <button type="button" class="kx-button" data-variant="secondary" data-size="sm">
    Clear filter <kbd class="kx-keycap">Esc</kbd>
  </button>
</div>
```

**All-cooldown:**

```html
<div class="kx-empty" role="status" data-variant="all-cooldown" aria-live="polite">
  <svg class="kx-empty__icon" aria-hidden="true"><!-- alert-triangle --></svg>
  <h2 class="kx-empty__title">All 3 accounts in cooldown</h2>
  <p class="kx-empty__desc">
    Pool resumes in <time class="mono" datetime="PT45S">45s</time>.
    You can force-refresh any single account:
  </p>
  <div class="kx-snippet">
    <pre class="mono"><code>kiroxy debug-refresh acct_01H8XJK9M2</code></pre>
    <button type="button" class="kx-button" data-variant="ghost" data-size="sm" aria-label="Copy"><svg aria-hidden="true"><!-- copy --></svg></button>
  </div>
</div>
```
