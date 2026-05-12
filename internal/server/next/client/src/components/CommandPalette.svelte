<!--
  CommandPalette — cmd-k. Fuzzy-filters across [accounts, requests, actions].
  Uses hand-rolled scorer from lib/fuzzy.

  Keyboard:
    cmd/ctrl + K  toggle open
    Arrow up/down  navigate
    Enter          execute
    Esc            close
-->
<script lang="ts">
  import { store } from "../lib/stores.svelte";
  import { fuzzyFilter } from "../lib/fuzzy";
  import Icon from "./icons/Icon.svelte";

  interface Props {
    open: boolean;
    onClose: () => void;
    onAction: (id: string) => void;
    actions: Array<{ id: string; label: string; hint?: string }>;
  }
  let { open, onClose, onAction, actions }: Props = $props();

  let query = $state("");
  let selectedIdx = $state(0);
  let inputEl: HTMLInputElement | undefined = $state();
  let dialogEl: HTMLDialogElement | undefined = $state();

  interface Item {
    id: string;
    group: "action" | "account" | "request";
    label: string;
    hint?: string;
  }

  const items = $derived.by<Item[]>(() => {
    const out: Item[] = [];
    for (const a of actions) {
      out.push({
        id: `action:${a.id}`,
        group: "action",
        label: a.label,
        ...(a.hint !== undefined ? { hint: a.hint } : {}),
      });
    }
    for (const acct of store.snapshot.accounts) {
      out.push({
        id: `account:${acct.id}`,
        group: "account",
        label: acct.id,
        hint: acct.provider || "kiro",
      });
    }
    for (const r of store.requests.slice(0, 20)) {
      out.push({
        id: `request:${r.id}`,
        group: "request",
        label: `${r.method} ${r.path}`,
        hint: `${r.status} · ${r.started_at.slice(11, 19)}`,
      });
    }
    return out;
  });

  const filtered = $derived(
    fuzzyFilter(items, query, (it) => `${it.label} ${it.hint ?? ""}`).slice(0, 30),
  );

  $effect(() => {
    if (open) {
      queueMicrotask(() => {
        dialogEl?.showModal();
        inputEl?.focus();
      });
    } else {
      dialogEl?.close();
    }
  });

  $effect(() => {
    // Reset selection when query changes
    void query;
    selectedIdx = 0;
  });

  function onKeydown(e: KeyboardEvent): void {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      selectedIdx = Math.min(filtered.length - 1, selectedIdx + 1);
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      selectedIdx = Math.max(0, selectedIdx - 1);
    } else if (e.key === "Enter") {
      e.preventDefault();
      const hit = filtered[selectedIdx];
      if (hit) execute(hit.item.id);
    } else if (e.key === "Escape") {
      e.preventDefault();
      onClose();
    }
  }

  function execute(id: string): void {
    onAction(id);
    query = "";
    selectedIdx = 0;
  }

  function groupLabel(g: Item["group"]): string {
    return g === "action" ? "action" : g === "account" ? "account" : "request";
  }
</script>

<dialog
  bind:this={dialogEl}
  class="palette"
  aria-labelledby="palette-heading"
  onclose={onClose}
>
  <div class="palette__box" role="combobox" aria-expanded="true" aria-controls="palette-list">
    <div class="palette__input-row">
      <Icon name="search" size={16} aria-label="search" />
      <input
        bind:this={inputEl}
        class="palette__input"
        type="text"
        placeholder="search accounts, requests, actions…"
        aria-label="command palette search"
        bind:value={query}
        onkeydown={onKeydown}
      />
      <kbd class="palette__kbd">esc</kbd>
    </div>

    <ol id="palette-list" class="palette__list" role="listbox">
      {#if filtered.length === 0}
        <li class="palette__empty faint">no matches</li>
      {:else}
        {#each filtered as hit, i (hit.item.id)}
          <!-- svelte-ignore a11y_click_events_have_key_events -->
          <li
            class="palette__item"
            class:palette__item--sel={i === selectedIdx}
            role="option"
            aria-selected={i === selectedIdx}
            onclick={() => execute(hit.item.id)}
            onmouseenter={() => (selectedIdx = i)}
          >
            <span class="palette__group">{groupLabel(hit.item.group)}</span>
            <span class="palette__label mono">{hit.item.label}</span>
            {#if hit.item.hint}
              <span class="palette__hint mono faint">{hit.item.hint}</span>
            {/if}
          </li>
        {/each}
      {/if}
    </ol>

    <div class="palette__foot">
      <span><kbd>↑</kbd> <kbd>↓</kbd> navigate</span>
      <span><kbd>↵</kbd> execute</span>
      <span><kbd>esc</kbd> close</span>
    </div>
  </div>
</dialog>

<style>
  .palette {
    inline-size: min(640px, 92vw);
    max-block-size: 72vh;
    border: 1px solid var(--c-border-strong);
    border-radius: var(--r-lg);
    background: var(--c-surface);
    color: var(--c-text);
    padding: 0;
    margin: 10vh auto auto;
  }
  .palette::backdrop {
    background: color-mix(in oklch, var(--c-bg), transparent 20%);
    backdrop-filter: blur(3px);
  }

  .palette__box {
    display: flex;
    flex-direction: column;
    min-block-size: 0;
    max-block-size: inherit;
  }

  .palette__input-row {
    display: flex;
    align-items: center;
    gap: var(--sp-3);
    padding: var(--sp-4) var(--sp-5);
    border-block-end: 1px solid var(--c-border);
    color: var(--c-text-dim);
  }
  .palette__input {
    flex: 1 1 auto;
    inline-size: 100%;
    font-size: var(--fs-md);
    color: var(--c-text);
    outline: none;
  }
  .palette__input::placeholder {
    color: var(--c-text-faint);
  }
  .palette__kbd {
    margin-inline-start: auto;
  }

  .palette__list {
    overflow-y: auto;
    flex: 1 1 auto;
    padding: var(--sp-2) 0;
  }
  .palette__empty {
    padding: var(--sp-5) var(--sp-5);
    text-align: center;
    font-size: var(--fs-sm);
  }
  .palette__item {
    display: grid;
    grid-template-columns: 80px 1fr auto;
    align-items: center;
    gap: var(--sp-4);
    padding: var(--sp-3) var(--sp-5);
    font-size: var(--fs-sm);
    cursor: pointer;
    border-inline-start: 2px solid transparent;
  }
  .palette__item--sel {
    background: var(--c-surface-hover);
    border-inline-start-color: var(--c-accent);
  }
  .palette__group {
    color: var(--c-text-faint);
    font-size: var(--fs-xs);
    text-transform: uppercase;
    letter-spacing: 0.06em;
    font-family: var(--font-mono);
  }
  .palette__label {
    color: var(--c-text);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .palette__hint {
    color: var(--c-text-faint);
    font-size: var(--fs-xs);
  }

  .palette__foot {
    display: flex;
    gap: var(--sp-5);
    padding: var(--sp-3) var(--sp-5);
    border-block-start: 1px solid var(--c-border);
    font-size: var(--fs-xs);
    color: var(--c-text-faint);
  }

  .faint {
    color: var(--c-text-faint);
  }
</style>
