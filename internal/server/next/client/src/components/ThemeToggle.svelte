<!--
  ThemeToggle — three-way cycle: system / dark / light.
  Uses View Transitions API for a smooth color swap when available.
-->
<script lang="ts">
  import { loadTheme, nextTheme, applyThemeWithTransition } from "../lib/theme";
  import type { Theme } from "../lib/types";
  import Icon from "./icons/Icon.svelte";

  let theme = $state<Theme>(loadTheme());

  $effect(() => {
    applyThemeWithTransition(theme);
  });

  function cycle(): void {
    theme = nextTheme(theme);
  }

  function label(t: Theme): string {
    return t === "system" ? "auto" : t;
  }
</script>

<button
  type="button"
  class="themebtn"
  aria-label="toggle theme (currently {label(theme)})"
  title="theme: {label(theme)}"
  onclick={cycle}
>
  <Icon name="theme" size={13} />
  <span class="themebtn__text">{label(theme)}</span>
</button>

<style>
  .themebtn {
    display: inline-flex;
    align-items: center;
    gap: var(--sp-2);
    padding: var(--sp-2) var(--sp-3);
    border-radius: var(--r-sm);
    border: 1px solid var(--c-border);
    background: var(--c-surface-2);
    color: var(--c-text-dim);
    font-size: var(--fs-xs);
    transition:
      background var(--mo-fast),
      color var(--mo-fast);
  }
  .themebtn:hover {
    background: var(--c-surface-hover);
    color: var(--c-text);
  }
  .themebtn__text {
    text-transform: uppercase;
    letter-spacing: 0.08em;
    font-weight: var(--fw-medium);
  }
</style>
