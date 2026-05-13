<!--
  EmptyState — shared shell for "nothing here yet" panels.

  The mandate asks for operator-grade voice instead of generic "No data"
  copy. Every view that can be empty (Live Stream, Pool board, Metrics,
  Logs, Settings inbound keys, Tools diagnostic, Models) gets a version
  of this component with:

    - philosophical one-line title (required)
    - optional secondary hint (multi-word guidance)
    - optional slot for a CTA or mono snippet

  Intentionally plain: the visual weight of the empty state should
  reinforce the product's "wire quiet" voice, not compete with it.
-->
<script lang="ts">
  import type { Snippet } from "svelte";

  interface Props {
    /** Short philosophical statement. One sentence, lowercase-optional. */
    title: string;
    /** Secondary explanation — multi-word, dim. */
    hint?: string;
    /** Optional rendered content (buttons, mono code blocks, etc.). */
    children?: Snippet;
    /** Visual density — "comfortable" (default) pads generously for full
     *  panels; "tight" for inline rows inside tables that already ship
     *  their own padding. */
    density?: "comfortable" | "tight";
    /** Optional icon/glyph character rendered large above the title.
     *  Keep it to a single mono char — this is not an icon library. */
    glyph?: string;
  }
  let { title, hint, children, density = "comfortable", glyph }: Props = $props();
</script>

<div class="empty empty--{density}" role="status">
  {#if glyph}
    <span class="empty__glyph mono" aria-hidden="true">{glyph}</span>
  {/if}
  <p class="empty__title">{title}</p>
  {#if hint}
    <p class="empty__hint faint">{hint}</p>
  {/if}
  {#if children}
    <div class="empty__slot">{@render children()}</div>
  {/if}
</div>

<style>
  .empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    text-align: center;
    color: var(--c-text-dim, #a0a0a0);
  }
  .empty--comfortable {
    gap: var(--sp-3, 10px);
    padding: var(--sp-8, 36px) var(--sp-6, 22px);
  }
  .empty--tight {
    gap: var(--sp-2, 8px);
    padding: var(--sp-5, 18px) var(--sp-4, 14px);
  }

  .empty__glyph {
    font-size: 28px;
    line-height: 1;
    color: var(--c-accent, #d5a048);
    opacity: 0.55;
    margin-block-end: var(--sp-2, 6px);
  }
  .empty__title {
    margin: 0;
    font-family: var(--font-display, var(--font-mono, monospace));
    font-size: var(--fs-md, 14px);
    color: var(--c-text, #eaeaea);
    font-weight: var(--fw-semibold, 500);
    letter-spacing: -0.005em;
  }
  .empty__hint {
    margin: 0;
    font-size: var(--fs-sm, 13px);
    max-inline-size: 44ch;
    line-height: var(--lh-relaxed, 1.5);
  }
  .empty__slot {
    margin-block-start: var(--sp-3, 10px);
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--sp-2, 8px);
  }
</style>
