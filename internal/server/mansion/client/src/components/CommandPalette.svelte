<!--
  CommandPalette — cmd-k surface. Fuzzy search across:
    - Static actions (reload, import, copy curl/link, navigate, clear filters)
    - Accounts by id
    - Recent requests by method/path/id

  Selection: arrow keys + Enter. Esc closes. The selected-item preview
  renders on the right pane — for actions it's a plain-text explanation,
  for account/request rows it's a mini status card.
-->
<script lang="ts">
  import { store } from "../lib/store.svelte";
  import { filterAndRank, type Scored } from "../lib/fuzzy";
  import { abbrId, fmtMs, shortTime } from "../lib/format";
  import { accountStatus } from "../lib/types";
  import Icon from "./Icon.svelte";

  interface Props {
    open: boolean;
    onClose: () => void;
    onAction: (id: string) => void;
  }
  let { open, onClose, onAction }: Props = $props();

  let query = $state("");
  let selectedIdx = $state(0);
  let input: HTMLInputElement | null = $state(null);

  interface Item {
    id: string;
    label: string;
    hint?: string;
    kind: "action" | "account" | "request";
    preview?: string;
  }

  const actions: Item[] = [
    { id: "action:reload", label: "reload snapshot", hint: "refresh /state + /requests", kind: "action", preview: "Re-fetches the current proxy snapshot and recent requests. Useful after importing accounts or restarting the server." },
    { id: "action:import", label: "import accounts", hint: "open the import drawer", kind: "action", preview: "Paste a JSON array exported from kiro-cli or another kiroxy vault and stage it for the pool." },
    { id: "action:copy-curl", label: "copy curl for state endpoint", hint: "clipboard", kind: "action", preview: "Copies a one-liner you can run from a terminal to inspect the state endpoint exactly as the UI sees it." },
    { id: "action:copy-link", label: "copy shareable view link", hint: "current filters in url", kind: "action", preview: "Your current filter and sort settings are encoded in the URL hash. Copy the link to share the same view." },
    { id: "action:clear-filters", label: "clear all filters", hint: "reset the board to defaults", kind: "action" },
    { id: "action:dashboard-next", label: "open dashboard-next", hint: "switch to experimental stack", kind: "action", preview: "Jumps to /dashboard-next — the minimum-viable alt stack you can compare side by side." },
    { id: "action:dashboard-v1", label: "open dashboard v1", hint: "phase H vanilla", kind: "action", preview: "Jumps to /dashboard — the phase-H vanilla HTML dashboard with simple polling." },
  ];

  let dynamicItems = $derived.by(() => {
    const accts: Item[] = store.snapshot.accounts.map((a) => ({
      id: "account:" + a.id,
      label: "account · " + abbrId(a.id, 10),
      hint: a.provider ? `${a.provider} · ${accountStatus(a)}` : accountStatus(a),
      kind: "account" as const,
    }));
    const reqs: Item[] = store.requests.slice(0, 30).map((r) => ({
      id: "request:" + r.id,
      label: `${r.method} ${r.path}`,
      hint: `${r.status} · ${fmtMs(r.latency_ms)} · ${shortTime(r.started_at)}`,
      kind: "request" as const,
    }));
    return [...actions, ...accts, ...reqs];
  });

  let ranked = $derived.by<Scored<Item>[]>(() =>
    filterAndRank(query, dynamicItems, (it) => `${it.label} ${it.hint ?? ""}`),
  );
  let selectedItem = $derived(ranked[selectedIdx]?.item);

  $effect(() => {
    // Reset selection when the query changes or palette opens.
    void query;
    void open;
    selectedIdx = 0;
  });

  $effect(() => {
    if (open) setTimeout(() => input?.focus(), 0);
  });

  function onKey(e: KeyboardEvent): void {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      selectedIdx = Math.min(ranked.length - 1, selectedIdx + 1);
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      selectedIdx = Math.max(0, selectedIdx - 1);
    } else if (e.key === "Enter") {
      e.preventDefault();
      const hit = ranked[selectedIdx];
      if (hit) onAction(hit.item.id);
    }
  }
</script>

{#if open}
  <div class="pal-backdrop" onclick={onClose} role="presentation"></div>
  <div class="pal" role="dialog" aria-modal="true" aria-label="command palette">
    <div class="pal__head">
      <Icon name="search" size={14} />
      <input
        bind:this={input}
        type="text"
        class="pal__input"
        placeholder="Run a command · Search accounts · Jump to a request"
        bind:value={query}
        onkeydown={onKey}
      />
      <kbd class="pal__esc">esc</kbd>
    </div>
    <div class="pal__body">
      <ul class="pal__list" role="listbox">
        {#each ranked as s, i (s.item.id)}
          <li
            class="pal__item pal__item--{s.item.kind}"
            class:pal__item--selected={i === selectedIdx}
            role="option"
            aria-selected={i === selectedIdx}
            onmouseenter={() => (selectedIdx = i)}
            onclick={() => onAction(s.item.id)}
            onkeydown={(e) => {
              if (e.key === "Enter" || e.key === " ") {
                e.preventDefault();
                onAction(s.item.id);
              }
            }}
          >
            <span class="pal__glyph">
              {#if s.item.kind === "action"}<Icon name="bolt" size={12} />{/if}
              {#if s.item.kind === "account"}<Icon name="layers" size={12} />{/if}
              {#if s.item.kind === "request"}<Icon name="arrow-right" size={12} />{/if}
            </span>
            <span class="pal__label">{s.item.label}</span>
            {#if s.item.hint}<span class="pal__hint faint">{s.item.hint}</span>{/if}
          </li>
        {/each}
        {#if ranked.length === 0}
          <li class="pal__empty faint">no matches. try fewer characters, or try a path like <code>/v1/messages</code>.</li>
        {/if}
      </ul>
      <aside class="pal__preview" aria-live="polite">
        {#if selectedItem}
          <div class="pal__preview-head">
            <span class="caps">{selectedItem.kind}</span>
          </div>
          <div class="pal__preview-body">
            {#if selectedItem.preview}
              <p>{selectedItem.preview}</p>
            {:else if selectedItem.kind === "account"}
              <p class="mono">id = {selectedItem.id.replace("account:", "")}</p>
              <p class="faint">Enter to focus this row on the board.</p>
            {:else if selectedItem.kind === "request"}
              <p class="mono">{selectedItem.label}</p>
              <p class="faint">{selectedItem.hint}</p>
              <p class="faint">Enter to open lifecycle timeline.</p>
            {/if}
          </div>
        {:else}
          <p class="faint">pick an item to see a quick description.</p>
        {/if}
      </aside>
    </div>
    <div class="pal__foot">
      <span><kbd>↑↓</kbd> navigate</span>
      <span><kbd>Enter</kbd> run</span>
      <span><kbd>Esc</kbd> close</span>
    </div>
  </div>
{/if}

<style>
  .pal-backdrop {
    position: fixed;
    inset: 0;
    background: color-mix(in oklch, var(--c-bg), transparent 30%);
    z-index: var(--z-overlay);
    animation: fade-in var(--mo-fast) var(--ease-std);
    cursor: default;
  }
  .pal {
    position: fixed;
    inset-block-start: 14vh;
    inset-inline: 0;
    margin-inline: auto;
    inline-size: min(720px, 92vw);
    z-index: var(--z-dialog);
    background: var(--c-surface);
    border: 1px solid var(--c-border-strong);
    border-radius: var(--r-lg);
    box-shadow: var(--sh-3);
    display: flex;
    flex-direction: column;
    overflow: hidden;
    animation: slide-up var(--mo-med) var(--ease-out);
  }
  .pal__head {
    display: flex;
    align-items: center;
    gap: var(--sp-3);
    padding: var(--sp-4) var(--sp-5);
    border-block-end: 1px solid var(--c-rule);
    color: var(--c-text-dim);
  }
  .pal__input {
    flex: 1 1 auto;
    font-size: var(--fs-md);
    font-family: var(--font-mono);
    color: var(--c-text);
  }
  .pal__esc {
    font-size: var(--fs-2xs);
  }

  .pal__body {
    display: grid;
    grid-template-columns: minmax(0, 1.35fr) minmax(0, 1fr);
    max-block-size: 48vh;
  }

  .pal__list {
    list-style: none;
    margin: 0;
    padding: var(--sp-2) 0;
    overflow-y: auto;
    border-inline-end: 1px solid var(--c-rule);
  }
  .pal__item {
    display: grid;
    grid-template-columns: 20px 1fr auto;
    align-items: center;
    gap: var(--sp-3);
    padding: 6px var(--sp-5);
    font-size: var(--fs-sm);
    color: var(--c-text);
    cursor: pointer;
  }
  .pal__item--selected {
    background: var(--c-surface-hover);
    box-shadow: inset 2px 0 0 0 var(--c-accent);
  }
  .pal__glyph {
    color: var(--c-accent);
    display: inline-grid;
    place-items: center;
  }
  .pal__label {
    min-inline-size: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .pal__hint {
    font-size: var(--fs-xs);
    font-family: var(--font-mono);
  }
  .pal__item--account .pal__label,
  .pal__item--request .pal__label {
    font-family: var(--font-mono);
  }

  .pal__empty {
    padding: var(--sp-5);
    font-size: var(--fs-sm);
  }
  .pal__empty code {
    font-size: var(--fs-xs);
    color: var(--c-accent);
  }

  .pal__preview {
    padding: var(--sp-4) var(--sp-5);
    overflow-y: auto;
  }
  .pal__preview-head {
    padding-block-end: var(--sp-3);
    border-block-end: 1px solid var(--c-rule);
  }
  .pal__preview-body {
    padding-block-start: var(--sp-3);
    font-size: var(--fs-sm);
    color: var(--c-text-dim);
  }
  .pal__preview-body p {
    margin: 0 0 var(--sp-3);
  }

  .pal__foot {
    display: flex;
    gap: var(--sp-5);
    padding: var(--sp-2) var(--sp-5);
    border-block-start: 1px solid var(--c-rule);
    background: var(--c-surface-sunken);
    font-size: var(--fs-xs);
    color: var(--c-text-faint);
  }

  @media (max-width: 640px) {
    .pal__body {
      grid-template-columns: 1fr;
    }
    .pal__preview {
      display: none;
    }
  }
</style>
