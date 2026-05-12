<script lang="ts">
  interface Props {
    keys: string[];
    label?: string;
  }
  let { keys, label }: Props = $props();
</script>

<span class="khint">
  {#each keys as k, i (k + i)}
    {#if i > 0}<span class="khint__plus" aria-hidden="true">+</span>{/if}
    <kbd>{k}</kbd>
  {/each}
  {#if label}<span class="khint__label">{label}</span>{/if}
</span>

<style>
  .khint {
    display: inline-flex;
    align-items: center;
    gap: var(--sp-2);
    color: var(--c-text-dim);
    font-size: var(--fs-xs);
  }
  .khint__plus {
    color: var(--c-text-faint);
  }
  .khint__label {
    margin-inline-start: var(--sp-2);
  }
</style>
