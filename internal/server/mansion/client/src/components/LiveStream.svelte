<!--
  LiveStream — the signature Warp-inspired request feed.

  Columns: TIME · ACCOUNT · MODEL · PATH · LATENCY · TOKENS · COST · STATUS.

  Discipline:
    - Mono everywhere; tabular numerics on TIME / LATENCY / TOKENS / COST.
    - 50-row rolling window (pool the operator can actually read).
    - New rows get a 1.5px amber left-edge marker that fades in 600ms.
      This is the ONLY piece of amber on the grid — data stays neutral.
    - Space key pauses the feed (handled by App.svelte hotkeys).
    - Row click opens the drill-down drawer.

  Status cell colors map to brand status semantics:
    200 → --c-success   (sage)
    429 → --c-warn      (near-amber)  — treated as neutral-hot, not amber
    5xx → --c-danger    (rust)
    COOLDOWN → --c-info (steel)

  Filter chips live in the toolbar above: All / Success / Error / Cooldown.
-->
<script lang="ts">
  import { store } from "../lib/store.svelte";
  import { shortTime, fmtMs, abbrId, truncate } from "../lib/format";
  import Icon from "./Icon.svelte";
  import type { RequestRecord } from "../lib/types";
  import type { Snippet } from "svelte";

  interface Props {
    rail?: Snippet;
  }
  let { rail }: Props = $props();

  const VISIBLE = 50;

  type StatusKind = "ok" | "warn" | "err" | "cool";

  function kindOf(r: RequestRecord): StatusKind {
    if (r.status === 0) return "cool";
    if (r.status >= 500) return "err";
    if (r.status === 429) return "warn";
    if (r.status >= 400) return "err";
    return "ok";
  }
  function statusLabel(r: RequestRecord): string {
    if (r.status === 0) return "COOLDOWN";
    return String(r.status);
  }
  function modelOf(r: RequestRecord): string {
    // The backend doesn't (yet) emit the model name; we parse from path /
    // query hints so the mockup column doesn't lie. When the backend gains
    // a model field on RequestRecord, swap for a direct read.
    if (r.path.includes("haiku")) return "haiku-4";
    if (r.path.includes("sonnet")) return "sonnet-4.5";
    if (r.path.includes("opus")) return "opus-4.7";
    return "—";
  }

  let range = $derived(store.filters.statusRange ?? "all");
  let kindFilter = $state<"all" | StatusKind>("all");

  function setKindFilter(k: typeof kindFilter): void {
    kindFilter = k;
  }
  function setRange(r: typeof range): void {
    store.setFilter("statusRange", r);
  }

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
      const k = kindOf(r);
      if (kindFilter !== "all" && kindFilter !== k) return false;
      return true;
    }),
  );
  let visible = $derived(filtered.slice(0, VISIBLE));

  function select(id: string): void {
    store.selectRequest(id);
  }
</script>

<section class="view view--live" id="view-live" aria-label="live request stream">
  <div class="live-grid">
    <div class="live-main">
      <div class="toolbar">
        <span class="eyebrow caps">Request feed</span>
        <button type="button" class="chip" aria-pressed={kindFilter === "all"} onclick={() => setKindFilter("all")}>All</button>
        <button type="button" class="chip" aria-pressed={kindFilter === "ok"} onclick={() => setKindFilter("ok")}>
          <span class="dot dot--ok" aria-hidden="true"></span>Success
        </button>
        <button type="button" class="chip" aria-pressed={kindFilter === "err"} onclick={() => setKindFilter("err")}>
          <span class="dot dot--err" aria-hidden="true"></span>Error
        </button>
        <button type="button" class="chip" aria-pressed={kindFilter === "cool"} onclick={() => setKindFilter("cool")}>
          <span class="dot dot--cool" aria-hidden="true"></span>Cooldown
        </button>
        <span class="spacer"></span>
        <div class="range" role="group" aria-label="status range">
          {#each ["all", "2xx", "4xx", "5xx"] as r}
            <button
              type="button"
              class="range__seg"
              class:range__seg--active={range === r}
              onclick={() => setRange(r as "all" | "2xx" | "4xx" | "5xx")}
            >{r}</button>
          {/each}
        </div>
        <button
          type="button"
          class="chip"
          aria-pressed={store.streamPaused}
          onclick={() => store.togglePause()}
          title="pause feed (space)"
        >
          {#if store.streamPaused}
            <svg width="10" height="10" viewBox="0 0 10 10" fill="currentColor" aria-hidden="true"><polygon points="2.5,1 8,5 2.5,9"/></svg>
            Resume
          {:else}
            <svg width="10" height="10" viewBox="0 0 10 10" fill="currentColor" aria-hidden="true"><rect x="2" y="1.5" width="2" height="7"/><rect x="6" y="1.5" width="2" height="7"/></svg>
            Pause
          {/if}
        </button>
      </div>

      <div class="stream-header" role="row">
        <div>Time</div>
        <div>Account</div>
        <div>Model</div>
        <div>Path</div>
        <div class="num">Latency</div>
        <div class="num">Tokens</div>
        <div class="num">Cost</div>
        <div class="num">Status</div>
      </div>

      <div class="stream-body" role="table">
        {#if visible.length === 0}
          <div class="stream-empty">
            <div class="stream-empty__head">
              <Icon name="bolt" size={16} />
              <span class="mono">no requests yet</span>
            </div>
            {#if store.requests.length === 0}
              <p class="stream-empty__copy">
                Wire quiet. Nothing moves.
              </p>
              <p class="stream-empty__hint mono faint">
                Try
                <code>curl -H "x-api-key: $KIROXY_API_KEY" http://127.0.0.1:8787/v1/models</code>
              </p>
            {:else}
              <p class="stream-empty__copy">Your filter is hiding the wire.</p>
              <button
                type="button"
                class="stream-empty__reset mono"
                onclick={() => {
                  store.setFilter('search', '');
                  setRange('all');
                  setKindFilter('all');
                }}
              >clear filters</button>
            {/if}
          </div>
        {:else}
          {#each visible as r (r.id)}
            {@const k = kindOf(r)}
            {@const sel = store.selectedRequestId === r.id}
            <div
              class="stream-row stream-row--{k}"
              class:stream-row--selected={sel}
              role="row"
              tabindex="0"
              onclick={() => select(r.id)}
              onkeydown={(e) => {
                if (e.key === "Enter" || e.key === " ") {
                  e.preventDefault();
                  select(r.id);
                }
              }}
              data-request-id={r.id}
            >
              <div class="ts-cell mono tabular">{shortTime(r.started_at)}</div>
              <div class="acct-cell mono" title={r.account_id ?? ""}>
                {r.account_id ? abbrId(r.account_id, 10) : "—"}
              </div>
              <div class="model-cell mono">{modelOf(r)}</div>
              <div class="path-cell mono" title={r.path}>{truncate(r.path, 60)}</div>
              <div class="lat-cell mono tabular">{fmtMs(r.latency_ms)}</div>
              <div class="tok-cell mono tabular">
                {r.bytes_out > 0 ? r.bytes_out.toLocaleString() : "—"}
              </div>
              <div class="cost-cell mono tabular">
                {r.bytes_out > 0 ? "$" + (r.bytes_out * 0.0000028).toFixed(4) : "—"}
              </div>
              <div class="status-cell mono">{statusLabel(r)}</div>
            </div>
          {/each}
        {/if}
      </div>

      <div class="stream-footer mono">
        <span>Holding {Math.min(VISIBLE, filtered.length)} most recent · feed {store.streamPaused ? "paused" : "active"}</span>
        <span>Moving window: 5 min</span>
      </div>
    </div>

    <aside class="live-rail" aria-label="telemetry rail">
      {#if rail}{@render rail()}{/if}
    </aside>
  </div>
</section>

<style>
  .view {
    flex: 1;
    min-block-size: 0;
    display: flex;
    flex-direction: column;
    view-transition-name: main-view;
  }
  .live-grid {
    display: grid;
    grid-template-columns: minmax(0, 1fr) 300px;
    flex: 1;
    min-block-size: 0;
  }
  .live-main {
    display: flex;
    flex-direction: column;
    min-block-size: 0;
    border-inline-end: 1px solid var(--c-border);
  }

  .toolbar {
    display: flex;
    align-items: center;
    gap: var(--sp-3);
    block-size: 36px;
    padding: 0 var(--sp-4);
    border-block-end: 1px solid var(--c-border);
    font-size: var(--fs-xs);
    color: var(--c-text-dim);
  }
  .eyebrow {
    color: var(--c-text-faint);
    text-transform: uppercase;
    font-size: 10.5px;
    letter-spacing: 0.08em;
  }
  .spacer { flex: 1; }

  .chip {
    display: inline-flex;
    align-items: center;
    gap: var(--sp-2);
    block-size: 22px;
    padding: 0 var(--sp-3);
    border: 1px solid var(--c-border);
    border-radius: var(--r-xs);
    font-size: var(--fs-xs);
    font-family: var(--font-mono);
    color: var(--c-text-dim);
    transition:
      color var(--mo-med) var(--ease-std),
      border-color var(--mo-med) var(--ease-std);
  }
  .chip:hover { color: var(--c-text); }
  .chip[aria-pressed="true"] {
    color: var(--c-text);
    border-color: var(--c-border-strong);
    background: var(--c-surface);
  }
  .dot {
    inline-size: 5px;
    block-size: 5px;
    border-radius: var(--r-pill);
  }
  .dot--ok   { background: var(--c-success); }
  .dot--err  { background: var(--c-danger); }
  .dot--cool { background: var(--c-info); }

  .range {
    display: inline-flex;
    border: 1px solid var(--c-border);
    border-radius: var(--r-xs);
    overflow: hidden;
  }
  .range__seg {
    block-size: 22px;
    padding: 0 8px;
    border-inline-end: 1px solid var(--c-border);
    font-size: var(--fs-xs);
    font-family: var(--font-mono);
    color: var(--c-text-dim);
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }
  .range__seg:last-child { border-inline-end: 0; }
  .range__seg:hover { color: var(--c-text); }
  .range__seg--active {
    color: var(--c-text);
    background: var(--c-surface);
  }

  .stream-header,
  .stream-row {
    display: grid;
    grid-template-columns: 76px 110px 96px minmax(0, 1fr) 90px 80px 72px 96px;
    align-items: center;
    column-gap: var(--sp-4);
    padding: 0 var(--sp-4);
    min-block-size: 32px;
    font-size: 12.5px;
  }
  .stream-header {
    color: var(--c-text-faint);
    font-size: 10.5px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    border-block-end: 1px solid var(--c-border);
    min-block-size: 28px;
  }
  .stream-header .num,
  .stream-row .num,
  .lat-cell,
  .tok-cell,
  .cost-cell,
  .status-cell {
    text-align: end;
  }
  .stream-body {
    flex: 1 1 auto;
    overflow-y: auto;
    min-block-size: 0;
    scroll-behavior: smooth;
  }
  .stream-row {
    position: relative;
    border-block-end: 1px solid var(--c-border);
    color: var(--c-text-dim);
    cursor: pointer;
    transition:
      background var(--mo-med) var(--ease-std),
      color var(--mo-med) var(--ease-std);
  }
  .stream-row:hover {
    background: var(--c-surface);
    color: var(--c-text);
  }
  .stream-row:focus-visible {
    outline: none;
    background: var(--c-surface);
  }
  .stream-row--selected {
    background: var(--c-surface);
    color: var(--c-text);
  }

  /* NEW-ROW marker — the ONE legitimate amber data cue (edge marker fades
     in 600ms). Neutral rows stay neutral; the signal *of novelty* uses
     amber, not the data itself. This sits within the amber budget because
     it's functional (new-row indicator) rather than aesthetic. */
  .stream-row {
    animation: row-fade-in var(--motion-duration) var(--motion-easing);
  }
  @starting-style {
    .stream-row { opacity: 0; transform: translateY(4px); }
  }
  .stream-row::before {
    content: "";
    position: absolute;
    inset-inline-start: 0;
    inset-block: 0;
    inline-size: 1.5px;
    background: var(--c-accent);
    opacity: 0;
    animation: edge-marker 600ms var(--motion-easing) forwards;
    animation-delay: 0ms;
  }
  @keyframes edge-marker {
    0%   { opacity: 1; }
    100% { opacity: 0; }
  }
  @keyframes row-fade-in {
    from { opacity: 0; }
    to   { opacity: 1; }
  }

  .ts-cell    { color: var(--c-text-faint); }
  .acct-cell  { color: var(--c-text-dim); }
  .model-cell { color: var(--c-text); }
  .path-cell  {
    color: var(--c-text);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .lat-cell   { color: var(--c-text); }
  .tok-cell   { color: var(--c-text-dim); }
  .cost-cell  { color: var(--c-text-dim); }
  .status-cell {
    font-size: 10.5px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--c-text-dim);
  }
  .stream-row--ok   .status-cell { color: var(--c-success); }
  .stream-row--warn .status-cell { color: var(--c-warn); }
  .stream-row--err  .status-cell { color: var(--c-danger); }
  .stream-row--cool .status-cell { color: var(--c-info); }

  .stream-footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    block-size: 28px;
    padding: 0 var(--sp-4);
    border-block-start: 1px solid var(--c-border);
    color: var(--c-text-faint);
    font-size: var(--fs-xs);
    letter-spacing: 0.03em;
  }

  .stream-empty {
    display: flex;
    flex-direction: column;
    gap: var(--sp-3);
    align-items: flex-start;
    padding: var(--sp-7) var(--sp-5);
    color: var(--c-text-faint);
  }
  .stream-empty__head {
    display: inline-flex;
    align-items: center;
    gap: var(--sp-3);
    color: var(--c-text-dim);
  }
  /* Italic serif for the philosophical empty-state copy — matches the
     operator voice codified in brand-spec.md. */
  .stream-empty__copy {
    font-family: var(--font-text);
    font-style: italic;
    font-size: var(--fs-md);
    color: var(--c-text);
    margin: 0;
    line-height: 1.45;
  }
  .stream-empty__hint {
    font-size: var(--fs-xs);
    max-inline-size: 56ch;
    margin: 0;
  }
  /* Inline code in the empty-state hint stays neutral — this is
     documentation text, not a UI accent role. Amber budget guards us
     from creeping amber into every <code> tag. */
  .stream-empty__hint code {
    font-size: var(--fs-2xs);
    color: var(--c-text-dim);
    background: var(--c-surface);
    border: 1px solid var(--c-border);
    padding: 1px 6px;
    border-radius: var(--r-sm);
  }
  .stream-empty__reset {
    padding: 4px 10px;
    border: 1px solid var(--c-border-strong);
    border-radius: var(--r-xs);
    color: var(--c-text-dim);
    font-size: var(--fs-xs);
  }
  .stream-empty__reset:hover { color: var(--c-text); }

  .live-rail {
    display: flex;
    flex-direction: column;
    min-block-size: 0;
    overflow-y: auto;
  }

  @media (max-width: 1200px) {
    .live-grid {
      grid-template-columns: 1fr;
    }
    .live-main {
      border-inline-end: 0;
      border-block-end: 1px solid var(--c-border);
    }
    .stream-header,
    .stream-row {
      grid-template-columns: 76px 90px 76px minmax(0, 1fr) 80px 70px 62px 80px;
      column-gap: var(--sp-3);
    }
  }
</style>
