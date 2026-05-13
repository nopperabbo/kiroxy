# kiroxy docs/ — index

This directory holds the design + product documentation that drives kiroxy's
dashboard, CLI, and contributor experience. Every doc is opinionated, every
decision is cited back to its source (Part 1 research, external spec, or
an explicit rationale here in the repo).

**Layers (authoritative top-to-bottom):**

1. **Vision** — what kiroxy is and isn't.
2. **Design system** — tokens, typography, motion, primitives.
3. **Interaction** — decision trees + shortcut maps.
4. **Information architecture** — screens, routes, empty states.
5. **Implementation** — the rubric an implementation is scored against.

---

## Part 1 — Product identity (fixed; operator-reviewed)

| Doc | Purpose |
|---|---|
| [VISION.md](./VISION.md) | What kiroxy is. Who it's for. Anti-goals. Signature primitive. |
| [ROADMAP.md](./ROADMAP.md) | Trajectory v1.x → v2.0. |
| [DESIGN_SYSTEM.md](./DESIGN_SYSTEM.md) | Color, typography, motion, layout, primitives, accessibility. |
| `../research-v3/REFERENCE_GALLERY.md` | 43 references studied; the evidence under every Part 1 decision. |

Editing Part 1 documents requires a PR with operator review.

---

## Part 2 — Executable design foundation (this week)

### Tokens

| Artifact | Role |
|---|---|
| [`internal/server/assets/tokens/tokens.css`](../internal/server/assets/tokens/tokens.css) | Runtime CSS custom properties. All five themes. |
| [`internal/server/assets/tokens/tokens.ts`](../internal/server/assets/tokens/tokens.ts) | Typed TypeScript exports mirroring the CSS. |
| [`internal/server/assets/tokens/tokens.json`](../internal/server/assets/tokens/tokens.json) | W3C DTCG machine-readable mirror (dark theme). |
| [DESIGN_TOKENS_AUDIT.md](./DESIGN_TOKENS_AUDIT.md) | WCAG 2.2 contrast audit for every fg/bg pair. |
| [`scripts/contrast.py`](../scripts/contrast.py) | Reproducible auditor script. |

### Interaction

| Doc | Purpose |
|---|---|
| [INTERACTION_PATTERNS.md](./INTERACTION_PATTERNS.md) | Decision trees for dialog vs drawer vs popover, forms, tables, live data, errors, loading, confirmations. |
| [KEYBOARD_SHORTCUTS.md](./KEYBOARD_SHORTCUTS.md) | Canonical shortcut map. Also the content of the `?` cheatsheet overlay. |

### Component primitives (18 specs)

Every primitive listed in `DESIGN_SYSTEM.md` §8 has a framework-agnostic
spec in [`components/`](./components/). Radix UI documentation style:
Anatomy → API → Variants → States → Accessibility → Motion → Composition
→ Anti-patterns → Reference → Example.

**Core controls:**
- [button.md](./components/button.md)
- [input.md](./components/input.md)
- [select.md](./components/select.md)
- [table.md](./components/table.md)

**Overlays:**
- [dialog.md](./components/dialog.md)
- [popover.md](./components/popover.md)
- [tooltip.md](./components/tooltip.md)
- [toast.md](./components/toast.md)

**State & shells:**
- [skeleton.md](./components/skeleton.md)
- [empty-state.md](./components/empty-state.md)

**Signature & affordances:**
- [command-palette.md](./components/command-palette.md)
- [live-request-stream-block.md](./components/live-request-stream-block.md) — ⭐ signature primitive
- [status-pill.md](./components/status-pill.md)
- [copyable-value.md](./components/copyable-value.md)
- [hotkey-hint.md](./components/hotkey-hint.md)

**Data visualization:**
- [timeline.md](./components/timeline.md)
- [sparkline.md](./components/sparkline.md)
- [heatmap.md](./components/heatmap.md)

### Information architecture

| Doc | Purpose |
|---|---|
| [INFORMATION_ARCHITECTURE.md](./INFORMATION_ARCHITECTURE.md) | Every screen, every URL, every empty state, every responsive breakpoint. |

### Iconography

| Artifact | Role |
|---|---|
| [ICONOGRAPHY.md](./ICONOGRAPHY.md) | Icon system spec. 27-icon curated inventory. |
| [`tools/icons/`](../tools/icons/) | 6 shipping SVGs + typed manifest + README. |

### Implementation rubric

| Doc | Purpose |
|---|---|
| [IMPLEMENTATION_RUBRIC.md](./IMPLEMENTATION_RUBRIC.md) | Executable checklist for grading any dashboard implementation. Track 3 (mansion) and any future iteration self-score here. |

---

## Operator-facing docs (unchanged by Part 2)

| Doc | Purpose |
|---|---|
| [ARCHITECTURE.md](./ARCHITECTURE.md) | System overview + package boundaries. |
| [TROUBLESHOOTING.md](./TROUBLESHOOTING.md) | Runbook for common failures. |
| [METRICS.md](./METRICS.md) + [METRICS.grafana.json](./METRICS.grafana.json) | Prometheus endpoint + starter dashboard. |
| [OPENAI.md](./OPENAI.md) | `/v1/chat/completions` integration guide. |
| [OPENCODE.md](./OPENCODE.md) | `kiroxy opencode-config` walkthrough. |
| [BENCHMARKS.md](./BENCHMARKS.md) | Latency baselines + methodology. |
| [DASHBOARD_NEXT.md](./DASHBOARD_NEXT.md) | Phase H.alt Svelte 5 dashboard doc. |
| [DASHBOARD_MANSION.md](./DASHBOARD_MANSION.md) | Track 3 dashboard implementation doc. |

---

## Cross-reference map

Each Part 2 document cites Part 1 by section. The reverse links:

| If you changed… | Re-audit these Part 2 docs: |
|---|---|
| `DESIGN_SYSTEM.md` §2 (color) | `tokens.css`, `tokens.ts`, `tokens.json`, `DESIGN_TOKENS_AUDIT.md`, every `components/*.md` |
| `DESIGN_SYSTEM.md` §3 (typography) | `tokens.css`, `components/*.md` (font specs) |
| `DESIGN_SYSTEM.md` §5 (motion) | `tokens.css`, `INTERACTION_PATTERNS.md` §9, `components/*.md` (motion tables) |
| `DESIGN_SYSTEM.md` §7 (interactions) | `INTERACTION_PATTERNS.md`, `KEYBOARD_SHORTCUTS.md`, `components/*.md` |
| `DESIGN_SYSTEM.md` §8 (primitives) | `components/*.md`, `IMPLEMENTATION_RUBRIC.md` §7 |
| `VISION.md` signature-primitive | `components/live-request-stream-block.md`, `INFORMATION_ARCHITECTURE.md` §1, `IMPLEMENTATION_RUBRIC.md` §8 |
| `VISION.md` anti-goals | `IMPLEMENTATION_RUBRIC.md` §10 |
| Adding a new icon | `ICONOGRAPHY.md`, `tools/icons/icons.ts`, any `components/*.md` that needs it |
| Adding a new shortcut | `KEYBOARD_SHORTCUTS.md` first, THEN implementation |

---

## Workflow for contributors

1. **Read `VISION.md` first.** If what you want to build crosses an anti-goal,
   stop and open an issue.
2. **Check `DESIGN_SYSTEM.md` + `components/`** for an existing primitive or
   pattern. Reuse before inventing.
3. **Follow the decision trees** in `INTERACTION_PATTERNS.md`.
4. **Keyboard-first.** Every action has a shortcut in `KEYBOARD_SHORTCUTS.md`;
   if you're adding a new one, update that doc first.
5. **Token-only styling.** Raw hex/rgb/hsl in component CSS is a regression.
6. **Run `scripts/contrast.py`** before merging color changes.
7. **Self-score** against `IMPLEMENTATION_RUBRIC.md` in your PR description.

---

## Authoring conventions

- **No emoji in product copy.** Tone is "ops tool with taste" per `VISION.md` §vibes.
- **Error messages:** `{problem} — {cause} — {action}`.
- **Empty states:** copyable CLI command as the CTA, not a button (fly.io pattern).
- **Code blocks** in docs use triple-backtick + language hint.
- **Tables** over prose when enumerating options.
- **Cite sources.** Every external pattern cites `REFERENCE_GALLERY.md` or an
  external URL.
