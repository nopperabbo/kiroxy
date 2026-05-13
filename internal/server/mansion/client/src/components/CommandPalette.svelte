<!--
  CommandPalette — cmd-k surface. Fuzzy search across:
    - Static actions (reload, import, copy curl/link, navigate, clear filters)
    - Accounts by id
    - Recent requests by method/path/id
    - Operator docs (README, ARCHITECTURE, TROUBLESHOOTING, OPENCODE,
      OPENAI, METRICS, VISION) served by /dashboard/api/docs/index.

  Selection: arrow keys + Enter. Esc closes. The selected-item preview
  renders on the right pane — for actions it's a plain-text explanation,
  for account/request rows it's a mini status card, for docs it's the
  rendered markdown body so the operator can read inline without ever
  leaving the palette.
-->
<script lang="ts">
  import { store } from "../lib/store.svelte";
  import { filterAndRank, type Scored } from "../lib/fuzzy";
  import { abbrId, fmtMs, shortTime } from "../lib/format";
  import { accountStatus } from "../lib/types";
  import { api, type DocEntry } from "../lib/api";
  import { renderMarkdown } from "../lib/markdown";
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
  let docs: DocEntry[] = $state([]);
  let docsLoaded = $state(false);
  let docsLoading = $state(false);

  interface Item {
    id: string;
    label: string;
    hint?: string;
    kind: "action" | "account" | "request" | "doc";
    preview?: string;
    docIndex?: number;
    /** Group heading for the two-tier Raycast-style layout. Items with the
     *  same section are rendered under a single divider + caps header. */
    section?: "Navigate" | "Pool actions" | "Stream" | "Quick actions" | "Docs";
    /** Keyboard shortcut hint (e.g., "G L", "Space", "⌘I"). Rendered on the
     *  right side of each row, mono. */
    kbd?: string;
  }

  const actions: Item[] = [
    { id: "view:live",     label: "Live stream",  kind: "action", section: "Navigate", kbd: "G L", preview: "The Warp-inspired request feed. Holds the 50 most recent requests; rows get a thin amber edge marker when they arrive." },
    { id: "view:pool",     label: "Pool",         kind: "action", section: "Navigate", kbd: "G P", preview: "The ledger-style account table. Inline 5-min sparklines per row, health dots, countdown rings for auto-refresh." },
    { id: "view:metrics",  label: "Metrics",      kind: "action", section: "Navigate", kbd: "G M", preview: "Aggregate KPI tile grid. P50/P95/P99 histogram, model / client mix, upstream origin breakdown." },
    { id: "view:logs",     label: "Logs",         kind: "action", section: "Navigate", kbd: "G G", preview: "Live tail of structured slog records from /dashboard/api/logs/stream. Filter by level, source, or free-text search; expand rows to see attribute JSON." },
    { id: "view:settings", label: "Settings",     kind: "action", section: "Navigate", kbd: "G S", preview: "Runtime info, redacted env vars, inbound API keys CRUD, and vault stats. Generate a new inbound key to grant a client access to this proxy." },
    { id: "view:tools",    label: "Tools",        kind: "action", section: "Navigate", kbd: "G T", preview: "Diagnostic (kiroxy doctor), backup/restore instructions, and onboarder pointers." },
    { id: "view:models",   label: "Models",       kind: "action", section: "Navigate", kbd: "G D", preview: "Canonical Claude model table — Anthropic id to upstream Kiro SKU, context window, family, tier." },
    { id: "action:pause-feed",     label: "Pause request feed",      kind: "action", section: "Stream", kbd: "Space", preview: "Freezes the live request stream without dropping incoming rows — the ring keeps filling, UI just stops auto-scrolling." },
    { id: "action:clear-filters",  label: "Clear all filters",       kind: "action", section: "Stream", preview: "Resets search, status range, errors-only and cooldown-only toggles." },
    { id: "action:import",         label: "Import accounts…",        kind: "action", section: "Pool actions", kbd: "I", preview: "Paste accounts or drop a .jsonl. Existing IDs are updated in place." },
    { id: "action:reload",         label: "Reload snapshot",         kind: "action", section: "Pool actions", preview: "Re-fetches the current proxy snapshot and recent requests. Useful after importing accounts or restarting the server." },
    { id: "action:copy-curl",      label: "Copy curl · state",       kind: "action", section: "Quick actions", preview: "Copies a one-liner you can run from a terminal to inspect the state endpoint exactly as the UI sees it." },
    { id: "action:copy-link",      label: "Copy shareable view link", kind: "action", section: "Quick actions", preview: "Your current filter and sort settings are encoded in the URL hash. Copy the link to share the same view." },
    { id: "action:dashboard-next", label: "Open dashboard-next",     kind: "action", section: "Quick actions", preview: "Jumps to /dashboard-next — the minimum-viable alt stack you can compare side by side." },
    { id: "action:dashboard-v1",   label: "Open dashboard v1",       kind: "action", section: "Quick actions", preview: "Jumps to /dashboard — the phase-H vanilla HTML dashboard with simple polling." },
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
    const docItems: Item[] = docs.map((d, idx) => ({
      id: "doc:" + d.path,
      label: "docs · " + d.title.toLowerCase(),
      hint: `${d.path} · ${formatBytes(d.bytes)}`,
      kind: "doc" as const,
      section: "Docs",
      docIndex: idx,
    }));
    return [...actions, ...docItems, ...accts, ...reqs];
  });

  function formatBytes(n: number): string {
    if (n < 1024) return n + " B";
    return Math.round(n / 1024) + " KB";
  }

  let ranked = $derived.by<Scored<Item>[]>(() =>
    filterAndRank(query, dynamicItems, (it) => `${it.label} ${it.hint ?? ""}`),
  );
  let selectedItem = $derived(ranked[selectedIdx]?.item);
  let selectedDoc = $derived<DocEntry | null>(
    selectedItem?.kind === "doc" && typeof selectedItem.docIndex === "number"
      ? docs[selectedItem.docIndex] ?? null
      : null,
  );
  let selectedDocHtml = $derived(selectedDoc ? renderMarkdown(selectedDoc.content) : "");

  async function loadDocs(): Promise<void> {
    if (docsLoaded || docsLoading) return;
    docsLoading = true;
    const res = await api.docsIndex();
    docsLoading = false;
    if (res.ok && res.data && Array.isArray(res.data.docs)) {
      docs = res.data.docs;
      docsLoaded = true;
    }
  }

  $effect(() => {
    void query;
    void open;
    selectedIdx = 0;
  });

  $effect(() => {
    if (open) {
      setTimeout(() => input?.focus(), 0);
      void loadDocs();
    }
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

  function sectionFor(kind: Item["kind"]): string {
    if (kind === "account") return "Search account";
    if (kind === "request") return "Search request";
    if (kind === "doc") return "Docs";
    return "";
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
        placeholder="Ask kiroxy…"
        bind:value={query}
        onkeydown={onKey}
      />
      {#if docsLoading}<span class="pal__loading faint">docs…</span>{/if}
      <kbd class="pal__esc">esc</kbd>
    </div>
    <div class="pal__body">
      <ul class="pal__list" role="listbox">
        {#each ranked as s, i (s.item.id)}
          {@const prevSection = i > 0 ? (ranked[i - 1].item.section ?? "") : ""}
          {@const thisSection = s.item.section ?? sectionFor(s.item.kind)}
          {#if !query && thisSection && thisSection !== prevSection}
            <li class="pal__hdr caps">{thisSection}</li>
          {/if}
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
              {#if s.item.kind === "doc"}<Icon name="book" size={12} />{/if}
            </span>
            <span class="pal__label">{s.item.label}</span>
            {#if s.item.hint}<span class="pal__hint faint">{s.item.hint}</span>{/if}
            {#if s.item.kbd}<span class="pal__kbd mono faint">{s.item.kbd}</span>{/if}
          </li>
        {/each}
        {#if ranked.length === 0}
          <li class="pal__empty">
            <p class="empty-italic">No matches. Try fewer characters, or a path like <code class="mono">/v1/messages</code>.</p>
          </li>
        {/if}
      </ul>
      <aside class="pal__preview" aria-live="polite">
        {#if selectedItem}
          <div class="pal__preview-head">
            <span class="caps">{selectedItem.kind}</span>
            {#if selectedItem.kind === "doc" && selectedDoc}
              <span class="pal__preview-path mono faint">{selectedDoc.path}</span>
            {/if}
          </div>
          <div class="pal__preview-body" class:pal__preview-body--doc={selectedItem.kind === "doc"}>
            {#if selectedItem.kind === "doc" && selectedDoc}
              <!-- eslint-disable-next-line svelte/no-at-html-tags -->
              <article class="md">{@html selectedDocHtml}</article>
            {:else if selectedItem.preview}
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
      <span><kbd class="kbd-amber">↵</kbd> select</span>
      <span><kbd>→</kbd> drill</span>
      <span><kbd>esc</kbd> close</span>
      <span class="pal__foot-spacer"></span>
      <span class="faint mono">{ranked.length} items</span>
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
  .pal__input::placeholder {
    color: var(--c-text-faint);
    font-family: var(--font-text);
    font-style: italic;
    letter-spacing: 0;
  }
  .pal__esc {
    font-size: var(--fs-2xs);
  }

  .pal__hdr {
    list-style: none;
    padding: var(--sp-3) var(--sp-5) 4px;
    color: var(--c-text-faint);
    font-size: 10.5px;
    letter-spacing: 0.08em;
  }
  .pal__hdr:first-child {
    padding-block-start: var(--sp-2);
  }
  .pal__kbd {
    color: var(--c-text-faint);
    font-size: var(--fs-xs);
    letter-spacing: 0.04em;
  }

  .empty-italic {
    margin: 0;
    font-family: var(--font-text);
    font-style: italic;
    color: var(--c-text-faint);
    font-size: var(--fs-sm);
  }
  .empty-italic code {
    font-style: normal;
    font-size: var(--fs-xs);
    color: var(--c-accent);
    padding: 1px 4px;
    border: 1px solid color-mix(in oklch, var(--c-accent), transparent 70%);
    border-radius: var(--r-xs);
  }

  /* amber budget: role 5 of 5 — ↵ kbd pill in palette footer. */
  .kbd-amber {
    color: var(--c-accent);
    border: 1px solid color-mix(in oklch, var(--c-accent), transparent 60%);
    background: transparent;
    padding: 0 4px;
    font-size: var(--fs-xs);
    border-radius: var(--r-xs);
  }
  .pal__foot-spacer { flex: 1; }

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
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: var(--sp-3);
    padding-block-end: var(--sp-3);
    border-block-end: 1px solid var(--c-rule);
  }
  .pal__preview-path {
    font-size: var(--fs-2xs);
  }
  .pal__preview-body {
    padding-block-start: var(--sp-3);
    font-size: var(--fs-sm);
    color: var(--c-text-dim);
  }
  .pal__preview-body p {
    margin: 0 0 var(--sp-3);
  }
  .pal__preview-body--doc {
    padding-block-start: var(--sp-2);
  }

  .pal__loading {
    font-family: var(--font-mono);
    font-size: var(--fs-2xs);
    padding: 1px 6px;
    border-radius: var(--r-sm);
    border: 1px solid var(--c-rule);
  }

  .pal__item--doc .pal__label {
    font-family: var(--font-text);
  }

  .md {
    display: block;
    line-height: var(--lh-relaxed, 1.55);
    color: var(--c-text);
  }
  .md :global(.md-h1),
  .md :global(.md-h2),
  .md :global(.md-h3),
  .md :global(.md-h4),
  .md :global(.md-h5),
  .md :global(.md-h6) {
    margin: var(--sp-4) 0 var(--sp-2);
    font-family: var(--font-display);
    font-weight: var(--fw-semibold);
    letter-spacing: -0.005em;
  }
  .md :global(.md-h1) {
    font-size: var(--fs-lg);
    margin-block-start: 0;
  }
  .md :global(.md-h2) {
    font-size: var(--fs-md);
  }
  .md :global(.md-h3),
  .md :global(.md-h4),
  .md :global(.md-h5),
  .md :global(.md-h6) {
    font-size: var(--fs-sm);
    text-transform: uppercase;
    letter-spacing: var(--tr-caps);
    color: var(--c-text-dim);
  }
  .md :global(.md-p) {
    margin: 0 0 var(--sp-3);
    color: var(--c-text-dim);
  }
  .md :global(.md-ul),
  .md :global(.md-ol) {
    margin: 0 0 var(--sp-3);
    padding-inline-start: var(--sp-5);
    color: var(--c-text-dim);
  }
  .md :global(.md-ul li),
  .md :global(.md-ol li) {
    margin-block-end: 4px;
  }
  .md :global(.md-code) {
    font-family: var(--font-mono);
    font-size: 0.92em;
    color: var(--c-accent);
    background: var(--c-accent-wash);
    padding: 1px 5px;
    border-radius: var(--r-sm);
    border: 1px solid color-mix(in oklch, var(--c-accent), transparent 70%);
  }
  .md :global(.md-pre) {
    margin: 0 0 var(--sp-3);
    padding: var(--sp-3);
    background: var(--c-surface-sunken);
    border: 1px solid var(--c-rule);
    border-radius: var(--r-sm);
    overflow-x: auto;
    font-size: var(--fs-xs);
    line-height: var(--lh-snug);
  }
  .md :global(.md-pre code) {
    background: transparent;
    border: 0;
    padding: 0;
    color: var(--c-text);
    font-family: var(--font-mono);
  }
  .md :global(.md-strong) {
    color: var(--c-text);
    font-weight: var(--fw-semibold);
  }
  .md :global(.md-em) {
    font-style: italic;
  }
  .md :global(.md-a) {
    color: var(--c-accent);
    text-decoration: none;
    border-block-end: 1px dashed color-mix(in oklch, var(--c-accent), transparent 60%);
  }
  .md :global(.md-a:hover) {
    border-block-end-style: solid;
  }
  .md :global(.md-quote) {
    margin: 0 0 var(--sp-3);
    padding: var(--sp-2) var(--sp-4);
    border-inline-start: 2px solid var(--c-accent);
    background: var(--c-accent-wash);
    color: var(--c-text-dim);
    border-radius: 0 var(--r-sm) var(--r-sm) 0;
    font-style: italic;
  }
  .md :global(.md-hr) {
    border: 0;
    border-block-end: 1px solid var(--c-rule);
    margin: var(--sp-4) 0;
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
