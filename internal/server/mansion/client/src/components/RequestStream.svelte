<!--
  RequestStream — live feed of recent requests. Table-y but denser than
  AccountBoard. Each row:
    time · method · path · status · latency · account hint

  Clicking a row opens DetailDrawer with the lifecycle timeline.

  Virtualization: windowed at 120 visible rows max to keep scroll
  perf consistent even if the ring holds 500. If latency is unknown
  (synthetic delta record) we show "—".

  Filter bar: status range (all/2xx/4xx/5xx), toggle "errors only".
-->
<script lang="ts">
  import { store } from "../lib/store.svelte";
  import { shortTime, fmtMs, abbrId, truncate } from "../lib/format";
  import Icon from "./Icon.svelte";

  const VISIBLE = 120;

  let range = $derived(store.filters.statusRange ?? "all");
  let filtered = $derived(
    store.requests.filter((r) => {
      const q = store.filters.search.trim().toLowerCase();
      if (q && !r.path.toLowerCase().includes(q) && !r.id.toLowerCase().includes(q)) {
        return false;
      }
      if (range === "2xx" && !(r.status >= 200 && r.status < 300)) return false;
      if (range === "4xx" && !(r.status >= 400 && r.status < 500)) return false;
      if (range === "5xx" && r.status < 500) return false;
      if (store.filters.onlyErrors && r.status < 400) return false;
      return true;
    }),
  );
  let visible = $derived(filtered.slice(0, VISIBLE));

  function statusClass(s: number): string {
    if (s >= 500) return "st-5";
    if (s >= 400) return "st-4";
    if (s >= 300) return "st-3";
    if (s >= 200) return "st-2";
    return "st-0";
  }
  function setRange(r: typeof range): void {
    store.setFilter("statusRange", r);
  }
  function select(id: string): void {
    store.selectRequest(id);
  }
</script>

<section class="stream" aria-label="recent requests">
  <header class="stream__head">
    <div class="stream__title">
      <span class="caps">requests</span>
      <span class="stream__count mono tabular">{filtered.length}</span>
      <span class="faint mono" style="font-size: var(--fs-xs)">
        {#if store.liveStatus === "stream"}
          <span class="pip pip--good" aria-hidden="true"></span>live
        {:else if store.liveStatus === "polling"}
          <span class="pip pip--accent" aria-hidden="true"></span>polling
        {:else if store.liveStatus === "offline"}
          <span class="pip pip--bad" aria-hidden="true"></span>offline
        {:else}
          <span class="pip pip--warn" aria-hidden="true"></span>{store.liveStatus}
        {/if}
      </span>
    </div>
    <div class="stream__filters" role="group" aria-label="status range">
      {#each ["all", "2xx", "4xx", "5xx"] as r}
        <button
          type="button"
          class="seg"
          class:seg--active={range === r}
          onclick={() => setRange(r as "all" | "2xx" | "4xx" | "5xx")}
        >{r}</button>
      {/each}
    </div>
  </header>

  <div class="stream__body" role="table">
    {#if visible.length === 0}
      <div class="stream__empty" role="row">
        <div class="stream__empty-icon" aria-hidden="true">
          <span class="stream__cursor">▍</span>
        </div>
        <div>
          <div class="stream__empty-title">no requests yet</div>
          <div class="stream__empty-hint faint">
            {#if store.filters.search || range !== "all" || store.filters.onlyErrors}
              your filters are hiding everything. <button type="button" class="stream__empty-btn" onclick={() => { store.setFilter('search',''); store.setFilter('onlyErrors', false); setRange('all'); }}>clear filters</button>
            {:else}
              the proxy hasn't served a request yet. try <code class="mono">curl -H "x-api-key: $KIROXY_API_KEY" http://127.0.0.1:8787/v1/models</code>
            {/if}
          </div>
        </div>
      </div>
    {/if}
    {#each visible as r (r.id)}
      {@const sel = store.selectedRequestId === r.id}
      <div
        class="entry entry--{statusClass(r.status)}"
        class:entry--selected={sel}
        role="row"
        tabindex="0"
        data-request-id={r.id}
        onclick={() => select(r.id)}
        onkeydown={(e) => {
          if (e.key === "Enter" || e.key === " ") {
            e.preventDefault();
            select(r.id);
          }
        }}
      >
        <span class="entry__time mono tabular">{shortTime(r.started_at)}</span>
        <span class="entry__method mono">{r.method}</span>
        <span class="entry__path mono" title={r.path}>{truncate(r.path, 48)}</span>
        <span class="entry__status mono tabular">{r.status}</span>
        <span class="entry__latency mono tabular">{fmtMs(r.latency_ms)}</span>
        <span class="entry__account mono faint" title={r.account_id ?? ""}>
          {r.account_id ? abbrId(r.account_id, 8) : "—"}
        </span>
        <Icon name="chevron-right" size={12} />
      </div>
    {/each}
    {#if filtered.length > VISIBLE}
      <div class="stream__more faint" role="row">
        <span>showing newest {VISIBLE} of {filtered.length} — refine filters to narrow the view</span>
      </div>
    {/if}
  </div>
</section>

<style>
  .stream {
    background: var(--c-surface);
    border: 1px solid var(--c-border);
    border-radius: var(--r-md);
    box-shadow: var(--sh-1);
    overflow: hidden;
    display: flex;
    flex-direction: column;
  }
  .stream__head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: var(--sp-4) var(--sp-5);
    border-block-end: 1px solid var(--c-rule);
    gap: var(--sp-4);
  }
  .stream__title {
    display: inline-flex;
    align-items: baseline;
    gap: var(--sp-3);
  }
  .stream__count {
    font-size: var(--fs-xs);
    color: var(--c-accent);
    padding: 1px 6px;
    border: 1px solid color-mix(in oklch, var(--c-accent), transparent 60%);
    border-radius: var(--r-pill);
    background: var(--c-accent-wash);
  }
  .stream__filters {
    display: inline-flex;
    gap: 1px;
    padding: 1px;
    background: var(--c-surface-sunken);
    border: 1px solid var(--c-rule);
    border-radius: var(--r-sm);
  }
  .seg {
    padding: 3px 8px;
    font-size: var(--fs-xs);
    font-family: var(--font-mono);
    letter-spacing: var(--tr-wide);
    text-transform: uppercase;
    color: var(--c-text-dim);
    border-radius: var(--r-xs);
  }
  .seg:hover {
    color: var(--c-text);
  }
  .seg--active {
    color: var(--c-accent);
    background: var(--c-surface);
    box-shadow: var(--sh-1), inset 0 0 0 1px color-mix(in oklch, var(--c-accent), transparent 60%);
  }

  .stream__body {
    flex: 1 1 auto;
    overflow-y: auto;
    max-block-size: min(640px, 70vh);
  }

  .pip {
    display: inline-block;
    inline-size: 6px;
    block-size: 6px;
    border-radius: var(--r-pill);
    margin-inline-end: var(--sp-2);
  }
  .pip--good {
    background: var(--c-success);
  }
  .pip--accent {
    background: var(--c-accent);
  }
  .pip--warn {
    background: var(--c-warn);
  }
  .pip--bad {
    background: var(--c-danger);
  }

  .entry {
    display: grid;
    grid-template-columns: 72px 44px minmax(0, 1fr) 42px 58px 70px 12px;
    align-items: center;
    gap: var(--sp-3);
    padding: var(--sp-2) var(--sp-5);
    border-block-end: 1px solid var(--c-rule);
    font-size: var(--fs-sm);
    cursor: pointer;
    transition: background var(--mo-fast) var(--ease-std);
  }
  .entry:hover {
    background: var(--c-surface-hover);
  }
  .entry:focus-visible {
    outline: none;
    box-shadow: inset 0 0 0 2px var(--c-accent);
  }
  .entry--selected {
    background: color-mix(in oklch, var(--c-accent-wash), var(--c-surface));
    box-shadow: inset 2px 0 0 0 var(--c-accent);
  }

  .entry__time {
    color: var(--c-text-faint);
    font-size: var(--fs-xs);
  }
  .entry__method {
    font-size: var(--fs-xs);
    color: var(--c-accent);
    letter-spacing: var(--tr-wide);
    text-transform: uppercase;
  }
  .entry__path {
    color: var(--c-text);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .entry__status {
    text-align: center;
    font-weight: var(--fw-semibold);
  }
  .entry--st-2 .entry__status {
    color: var(--c-success);
  }
  .entry--st-3 .entry__status {
    color: var(--c-info);
  }
  .entry--st-4 .entry__status {
    color: var(--c-warn);
  }
  .entry--st-5 .entry__status {
    color: var(--c-danger);
  }
  .entry--st-0 .entry__status {
    color: var(--c-text-faint);
  }
  .entry__latency {
    text-align: end;
    color: var(--c-text-dim);
    font-size: var(--fs-xs);
  }
  .entry__account {
    text-align: end;
    font-size: var(--fs-2xs);
  }

  .stream__empty {
    display: flex;
    align-items: flex-start;
    gap: var(--sp-4);
    padding: var(--sp-7) var(--sp-5);
    color: var(--c-text-faint);
  }
  .stream__empty-icon {
    color: var(--c-accent);
  }
  .stream__cursor {
    display: inline-block;
    font-family: var(--font-mono);
    font-size: var(--fs-xl);
    line-height: 1;
    animation: blink 1.1s steps(2, end) infinite;
  }
  @keyframes blink {
    from { opacity: 1; }
    50%  { opacity: 0.1; }
    to   { opacity: 1; }
  }
  .stream__empty-title {
    font-family: var(--font-display);
    font-size: var(--fs-md);
    color: var(--c-text);
    margin-block-end: 3px;
  }
  .stream__empty-hint {
    font-size: var(--fs-sm);
    max-inline-size: 44ch;
    line-height: var(--lh-snug);
  }
  .stream__empty-hint code {
    font-size: var(--fs-2xs);
    color: var(--c-accent);
    background: var(--c-accent-wash);
    padding: 1px 6px;
    border-radius: var(--r-sm);
    border: 1px solid color-mix(in oklch, var(--c-accent), transparent 70%);
  }
  .stream__empty-btn {
    color: var(--c-accent);
    border-block-end: 1px dashed color-mix(in oklch, var(--c-accent), transparent 60%);
    padding: 0;
    background: transparent;
    font: inherit;
    cursor: pointer;
  }
  .stream__empty-btn:hover {
    color: var(--c-accent-strong);
  }

  .stream__more {
    padding: var(--sp-3) var(--sp-5);
    font-size: var(--fs-xs);
    text-align: center;
    color: var(--c-text-faint);
  }
</style>
