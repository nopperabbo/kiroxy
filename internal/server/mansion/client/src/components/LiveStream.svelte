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
  type TimeWin = "5m" | "15m" | "1h" | "today";
  let timeRange = $state<TimeWin>("5m");
  let sortMode = $state<"time" | "latency">("time");
  type SearchScope = "all" | "path" | "account" | "id";
  let searchScope = $state<SearchScope>("all");

  function setKindFilter(k: typeof kindFilter): void {
    kindFilter = k;
  }
  function setRange(r: typeof range): void {
    store.setFilter("statusRange", r);
  }
  function setTimeRange(t: TimeWin): void {
    timeRange = t;
  }
  function toggleSort(): void {
    sortMode = sortMode === "time" ? "latency" : "time";
  }
  function setSearchScope(s: SearchScope): void {
    searchScope = s;
  }
  function reconnect(): void {
    store.reconnectLive?.();
  }
  function exportCsv(): void {
    const rows = filtered.slice(0, 1000);
    const header = ["time", "account_id", "model", "method", "path", "status", "latency_ms", "bytes_out"];
    const lines = [header.join(",")];
    for (const r of rows) {
      const cells = [
        r.started_at,
        r.account_id ?? "",
        modelOf(r),
        r.method ?? "",
        r.path,
        String(r.status),
        String(r.latency_ms),
        String(r.bytes_out),
      ].map((v) => {
        const s = String(v);
        return s.includes(",") || s.includes('"') ? '"' + s.replace(/"/g, '""') + '"' : s;
      });
      lines.push(cells.join(","));
    }
    const blob = new Blob([lines.join("\n")], { type: "text/csv;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `kiroxy-livestream-${new Date().toISOString().replace(/[:.]/g, "-")}.csv`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
  }
  function timeCutoff(t: TimeWin): number {
    const now = Date.now();
    if (t === "5m") return now - 5 * 60 * 1000;
    if (t === "15m") return now - 15 * 60 * 1000;
    if (t === "1h") return now - 60 * 60 * 1000;
    // today: midnight local
    const d = new Date();
    d.setHours(0, 0, 0, 0);
    return d.getTime();
  }
  let cutoff = $derived(timeCutoff(timeRange));

  let filtered = $derived(
    store.requests.filter((r) => {
      const q = store.filters.search.trim().toLowerCase();
      if (q) {
        const path = r.path.toLowerCase();
        const id = r.id.toLowerCase();
        const acct = (r.account_id ?? "").toLowerCase();
        let matches = false;
        if (searchScope === "all") matches = path.includes(q) || id.includes(q) || acct.includes(q);
        else if (searchScope === "path") matches = path.includes(q);
        else if (searchScope === "account") matches = acct.includes(q);
        else if (searchScope === "id") matches = id.includes(q);
        if (!matches) return false;
      }
      if (range === "2xx" && !(r.status >= 200 && r.status < 300)) return false;
      if (range === "4xx" && !(r.status >= 400 && r.status < 500)) return false;
      if (range === "5xx" && r.status < 500) return false;
      if (store.filters.onlyErrors && r.status < 400) return false;
      const k = kindOf(r);
      if (kindFilter !== "all" && kindFilter !== k) return false;
      const ts = Date.parse(r.started_at);
      if (Number.isFinite(ts) && ts < cutoff) return false;
      return true;
    }),
  );
  let sorted = $derived(
    sortMode === "latency"
      ? [...filtered].sort((a, b) => b.latency_ms - a.latency_ms)
      : filtered,
  );
  let visible = $derived(sorted.slice(0, VISIBLE));

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
        {#if store.filters.search.trim()}
          <span class="range__label caps faint">scope</span>
          <div class="range" role="group" aria-label="search scope">
            {#each ["all", "path", "account", "id"] as s}
              <button
                type="button"
                class="range__seg"
                class:range__seg--active={searchScope === s}
                onclick={() => setSearchScope(s as SearchScope)}
              >{s}</button>
            {/each}
          </div>
        {/if}
        <span class="spacer"></span>
        <span class="range__label caps faint">window</span>
        <div class="range" role="group" aria-label="time window">
          {#each ["5m", "15m", "1h", "today"] as t}
            <button
              type="button"
              class="range__seg"
              class:range__seg--active={timeRange === t}
              onclick={() => setTimeRange(t as TimeWin)}
            >{t}</button>
          {/each}
        </div>
        <span class="range__label caps faint">status</span>
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
        <button
          type="button"
          class="chip"
          aria-pressed={sortMode === "latency"}
          onclick={toggleSort}
          title="sort by latency"
        >
          <svg width="10" height="10" viewBox="0 0 10 10" fill="currentColor" aria-hidden="true">
            <path d="M2 8 V2 M2 2 L0.5 3.5 M2 2 L3.5 3.5 M6 2 V8 M6 8 L4.5 6.5 M6 8 L7.5 6.5"/>
          </svg>
          {sortMode === "latency" ? "Slowest" : "Newest"}
        </button>
        <button
          type="button"
          class="chip"
          onclick={exportCsv}
          title="download visible rows as CSV"
          disabled={filtered.length === 0}
        >
          <svg width="10" height="10" viewBox="0 0 10 10" fill="none" stroke="currentColor" stroke-width="1" aria-hidden="true">
            <path d="M5 1 V7 M3 5 L5 7 L7 5 M1.5 8.5 H8.5"/>
          </svg>
          CSV
        </button>
        <button
          type="button"
          class="chip"
          onclick={reconnect}
          title="re-open live connection"
        >
          <svg width="10" height="10" viewBox="0 0 10 10" fill="none" stroke="currentColor" stroke-width="1" aria-hidden="true">
            <path d="M8 3 A3 3 0 1 0 8 6 M8 1 V3 H6"/>
          </svg>
          {store.liveStatus === "stream" ? "Live" : store.liveStatus === "polling" ? "Poll" : "Reconnect"}
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
          <div class="stream-empty" role="status">
            {#if store.requests.length === 0}
              <div class="stream-empty__head">
                <span class="stream-empty__glyph mono" aria-hidden="true">~</span>
              </div>
              <p class="stream-empty__copy">
                Wire quiet. Nothing moves.
              </p>
              <p class="stream-empty__hint mono faint">
                <span style="margin-inline-end: 8px;">↓ Run this to see traffic</span>
                <code>curl -H "x-api-key: $KIROXY_API_KEY" http://127.0.0.1:8787/v1/models</code>
              </p>
              <div style="margin-block-start: var(--sp-4);">
                <button type="button" class="btn btn--accent" onclick={async () => {
                  const cmd = 'curl -H "x-api-key: $KIROXY_API_KEY" http://127.0.0.1:8787/v1/models';
                  await navigator.clipboard.writeText(cmd);
                  store.pushToast('ok', 'curl copied — paste in shell to see traffic');
                }}>Copy test request</button>
              </div>
            {:else if store.filters.search.trim() || range !== "all" || store.filters.onlyErrors || kindFilter !== "all"}
              <div class="stream-empty__head">
                <span class="stream-empty__glyph mono" aria-hidden="true">⌐</span>
              </div>
              <p class="stream-empty__copy">Your filter is hiding the wire.</p>
              <div style="margin-block-start: var(--sp-3);">
                <button
                  type="button"
                  class="stream-empty__reset mono"
                  onclick={() => {
                    store.setFilter('search', '');
                    setRange('all');
                    setKindFilter('all');
                  }}
                >clear filters</button>
              </div>
            {:else}
              {@const lastReq = store.requests[0]}
              {@const idleMins = Math.floor((Date.now() - Date.parse(lastReq.started_at)) / 60000)}
              <div class="stream-empty__head">
                <span class="stream-empty__glyph mono" aria-hidden="true">~</span>
              </div>
              <p class="stream-empty__copy">
                Wire quiet for {idleMins} minute{idleMins === 1 ? '' : 's'}.
              </p>
              <p class="stream-empty__hint mono faint">
                Stream still listening. Send a request to wake it.
              </p>
            {/if}
          </div>
          <div class="stream-ghosts" aria-hidden="true">
            {#each [
              ['14:02:01', 'EHGA3GRVQM', 'claude-sonnet-4-6', '/v1/messages', '820ms', '4,210', '$0.0126', '200'],
              ['14:01:45', 'act-7f3a', 'claude-haiku-4-5', '/v1/messages', '240ms', '812', '$0.0008', '200'],
              ['14:00:12', 'EHGA3GRVQM', 'claude-opus-4-7', '/v1/messages', '4,100ms', '12,050', '$0.1807', '200'],
              ['13:58:05', 'erinjones@', 'claude-sonnet-4-6', '/v1/messages', '120ms', '—', '—', '429']
            ] as row, i}
              {@const opacities = [1.0, 0.6, 0.35, 0.18]}
              <div class="stream-row stream-row--ghost" style="opacity: {opacities[i]};">
                <div class="ts-cell mono tabular">{row[0]}</div>
                <div class="acct-cell mono">{row[1]}</div>
                <div class="model-cell mono">{row[2]}</div>
                <div class="path-cell mono">{row[3]}</div>
                <div class="lat-cell mono tabular">{row[4]}</div>
                <div class="tok-cell mono tabular">{row[5]}</div>
                <div class="cost-cell mono tabular">{row[6]}</div>
                <div class="status-cell mono">{row[7]}</div>
              </div>
            {/each}
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
        <span>Holding {Math.min(VISIBLE, filtered.length)} most recent · feed {store.streamPaused ? "paused" : "active"} · sort {sortMode === "time" ? "newest first" : "slowest first"}</span>
        <span>Window: {timeRange === "today" ? "today" : timeRange}</span>
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
  .stream-empty__glyph {
    font-size: 28px;
    line-height: 1;
    color: var(--c-accent);
    opacity: 0.55;
    margin-block-end: var(--sp-2);
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
    border: 1px dashed var(--c-border-strong);
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

  .btn {
    display: inline-flex;
    align-items: center;
    gap: var(--sp-2);
    padding: 5px 10px;
    font-size: var(--fs-sm);
    font-family: var(--font-mono);
    background: var(--c-surface);
    border: 1px solid var(--c-border);
    border-radius: var(--r-sm);
    color: var(--c-text-dim);
    cursor: pointer;
    transition: all var(--mo-fast) var(--ease-std);
  }
  .btn:hover {
    color: var(--c-text);
    border-color: var(--c-border-strong);
  }
  .btn--accent {
    color: var(--c-accent);
    border-color: color-mix(in oklch, var(--c-accent), transparent 50%);
    background: var(--c-accent-wash);
  }
  .btn--accent:hover {
    color: var(--c-accent-strong);
  }

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
  @media (max-width: 768px) {
    .toolbar {
      flex-wrap: wrap;
      block-size: auto;
      min-block-size: 36px;
      padding: var(--sp-2) var(--sp-3);
      gap: var(--sp-2);
    }
    .toolbar .spacer { display: none; }
    .stream-header,
    .stream-row {
      grid-template-columns: 60px 80px 70px minmax(140px, 1fr) 60px 50px 50px 60px;
      column-gap: var(--sp-2);
      font-size: var(--fs-2xs);
    }
    .live-main {
      overflow-x: auto;
    }
  }
  @media (max-width: 480px) {
    .stream-header {
      display: none;
    }
    .stream-row {
      grid-template-columns: 75px 1fr 60px;
      grid-auto-rows: auto;
      row-gap: 4px;
      padding: var(--sp-3) var(--sp-4);
    }
    .ts-cell { grid-column: 1; grid-row: 1; }
    .path-cell { grid-column: 2; grid-row: 1; }
    .status-cell { grid-column: 3; grid-row: 1; text-align: right; }

    .acct-cell { grid-column: 1; grid-row: 2; font-size: var(--fs-2xs); }
    .model-cell { grid-column: 2; grid-row: 2; font-size: var(--fs-2xs); color: var(--c-text-dim); }
    .lat-cell { grid-column: 3; grid-row: 2; font-size: var(--fs-2xs); text-align: right; }
    
    .tok-cell, .cost-cell { display: none; }

    .chip, .range__seg, .btn {
      padding-block: 14px;
    }
  }
  .range__label {
    font-size: var(--fs-2xs);
    color: var(--c-text-faint);
    letter-spacing: 0.1em;
    margin-inline-end: 4px;
    align-self: center;
  }
  .stream-ghosts {
    display: flex;
    flex-direction: column;
    pointer-events: none;
    opacity: 0.55;
  }
  .stream-row--ghost {
    block-size: 32px;
    border-block-end: 1px solid color-mix(in oklch, var(--c-border), transparent 50%);
    align-items: center;
  }
  .g-bar {
    display: inline-block;
    block-size: 6px;
    background: color-mix(in oklch, var(--c-text-faint), transparent 60%);
    border-radius: 1px;
    animation: ghost-pulse 2.4s ease-in-out infinite;
    animation-delay: calc(var(--ghost-i, 0) * 80ms);
  }
  @keyframes ghost-pulse {
    0%, 100% { opacity: 0.35; }
    50% { opacity: 0.75; }
  }
  @media (prefers-reduced-motion: reduce) {
    .g-bar { animation: none; opacity: 0.5; }
  }
</style>
