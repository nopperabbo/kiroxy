<!--
  ShortcutSheet — the keyboard cheatsheet (press ?). Grouped by context.
-->
<script lang="ts">
  import Icon from "./Icon.svelte";

  interface Props {
    open: boolean;
    onClose: () => void;
  }
  let { open, onClose }: Props = $props();
</script>

{#if open}
  <div class="sheet-scrim" onclick={onClose} role="presentation"></div>
  <div class="sheet" role="dialog" aria-modal="true" aria-label="keyboard shortcuts">
    <header class="sheet__head">
      <h2 class="sheet__title">keyboard shortcuts</h2>
      <button type="button" class="iconbtn" aria-label="close" onclick={onClose}>
        <Icon name="x" size={14} />
      </button>
    </header>
    <div class="sheet__body">
      <section>
        <h3 class="caps">global</h3>
        <dl class="sheet__list">
          <dt><kbd>⌘</kbd><kbd>K</kbd></dt><dd>open / close command palette</dd>
          <dt><kbd>/</kbd></dt><dd>focus the search input</dd>
          <dt><kbd>i</kbd></dt><dd>open import drawer</dd>
          <dt><kbd>?</kbd></dt><dd>show this cheat sheet</dd>
          <dt><kbd>Esc</kbd></dt><dd>close overlay / clear selection</dd>
        </dl>
      </section>
      <section>
        <h3 class="caps">palette</h3>
        <dl class="sheet__list">
          <dt><kbd>↑</kbd><kbd>↓</kbd></dt><dd>navigate results</dd>
          <dt><kbd>Enter</kbd></dt><dd>run the highlighted action</dd>
        </dl>
      </section>
      <section>
        <h3 class="caps">board</h3>
        <dl class="sheet__list">
          <dt><kbd>Tab</kbd></dt><dd>move focus between rows and controls</dd>
          <dt><kbd>Enter</kbd></dt><dd>open details on the focused row</dd>
        </dl>
      </section>
    </div>
  </div>
{/if}

<style>
  .sheet-scrim {
    position: fixed;
    inset: 0;
    background: color-mix(in oklch, var(--c-bg), transparent 30%);
    z-index: var(--z-overlay);
    animation: fade-in var(--mo-fast);
    cursor: default;
  }
  .sheet {
    position: fixed;
    inset-block-start: 20vh;
    inset-inline: 0;
    margin-inline: auto;
    inline-size: min(460px, 92vw);
    z-index: var(--z-dialog);
    background: var(--c-surface);
    border: 1px solid var(--c-border-strong);
    border-radius: var(--r-lg);
    box-shadow: var(--sh-3);
    overflow: hidden;
    animation: slide-up var(--mo-med) var(--ease-out);
  }
  .sheet__head {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: var(--sp-4) var(--sp-5);
    border-block-end: 1px solid var(--c-rule);
  }
  .sheet__title {
    margin: 0;
    font-size: var(--fs-md);
    font-family: var(--font-display);
    font-weight: var(--fw-semibold);
  }
  .iconbtn {
    display: inline-grid;
    place-items: center;
    inline-size: 28px;
    block-size: 28px;
    border-radius: var(--r-sm);
    color: var(--c-text-dim);
  }
  .iconbtn:hover {
    color: var(--c-text);
    background: var(--c-surface-hover);
  }

  .sheet__body {
    padding: var(--sp-5);
    display: grid;
    gap: var(--sp-5);
  }
  h3 {
    margin: 0 0 var(--sp-2);
  }
  .sheet__list {
    display: grid;
    grid-template-columns: max-content 1fr;
    gap: var(--sp-2) var(--sp-5);
    margin: 0;
    font-size: var(--fs-sm);
  }
  .sheet__list dt {
    display: inline-flex;
    gap: var(--sp-2);
  }
  .sheet__list dd {
    margin: 0;
    color: var(--c-text-dim);
  }
</style>
