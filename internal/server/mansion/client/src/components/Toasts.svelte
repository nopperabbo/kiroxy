<!--
  Toasts — stacked transient notifications. Auto-dismiss at store level.
-->
<script lang="ts">
  import { store } from "../lib/store.svelte";
  import Icon from "./Icon.svelte";
</script>

{#if store.toasts.length > 0}
  <div class="toasts" aria-live="polite" aria-label="notifications">
    {#each store.toasts as t (t.id)}
      <div class="toast toast--{t.kind}" role="status">
        <Icon name={t.kind === "ok" ? "check" : t.kind === "err" ? "alert" : "dot"} size={12} />
        <span>{t.msg}</span>
      </div>
    {/each}
  </div>
{/if}

<style>
  .toasts {
    position: fixed;
    inset-block-end: var(--sp-6);
    inset-inline-end: var(--sp-6);
    z-index: var(--z-toast);
    display: flex;
    flex-direction: column;
    gap: var(--sp-3);
    pointer-events: none;
  }
  .toast {
    display: inline-flex;
    align-items: center;
    gap: var(--sp-3);
    padding: var(--sp-3) var(--sp-4);
    font-size: var(--fs-sm);
    border-radius: var(--r-sm);
    border: 1px solid var(--c-border);
    background: var(--c-surface);
    color: var(--c-text);
    box-shadow: var(--sh-2);
    pointer-events: auto;
    animation: slide-up var(--mo-med) var(--ease-out);
  }
  .toast--ok {
    color: var(--c-success);
    border-color: color-mix(in oklch, var(--c-success), transparent 70%);
  }
  .toast--err {
    color: var(--c-danger);
    border-color: color-mix(in oklch, var(--c-danger), transparent 60%);
  }
</style>
