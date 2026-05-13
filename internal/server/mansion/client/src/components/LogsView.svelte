<!--
  LogsView — live tail of captured slog records from /dashboard/api/logs.

  Architecture (SSE-first, polling-fallback matches LiveSource):
    1. Boot: fetch /dashboard/api/logs?max=200 for initial rows.
    2. Open EventSource to /dashboard/api/logs/stream with current filters.
    3. On 'log' event, prepend row (respecting pause).
    4. Filter changes close and reopen the EventSource so server-side
       filtering stays authoritative (client doesn't have records we
       didn't ask for).

  Row anatomy:
    timestamp · LEVEL badge · source · message · [expand fields]

  Filters:
    - Level select (all/DEBUG/INFO/WARN/ERROR)
    - Source text (substring)
    - Search text (message + field values)
    - Pause toggle (retains buffered records but doesn't auto-scroll)
    - Clear (empties local ring)

  Max local rows: 1000 — keeps scroll perf snappy; the server ring may
  hold up to its capacity but we don't need everything in the DOM.
-->
<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { api, type LogRecord, type LogsQuery } from "../lib/api";
  import { shortTime } from "../lib/format";
  import Icon from "./Icon.svelte";

  const MAX_LOCAL = 1000;

  let records: LogRecord[] = $state([]);
  let paused = $state(false);
  let expandedIds = $state(new Set<number>());
  let counters = $state<{ total: number; buffered: number; capacity: number } | null>(null);

  let levelFilter = $state<string>("");
  let sourceFilter = $state<string>("");
  let searchFilter = $state<string>("");
  let connectionState = $state<"connecting" | "live" | "polling" | "offline">("connecting");
  let errorBanner = $state<string | null>(null);

  let es: EventSource | null = null;
  let pollTimer: ReturnType<typeof setInterval> | null = null;
  let lastSeenId = 0;
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  let filterDebounce: ReturnType<typeof setTimeout> | null = null;

  onMount(() => {
    void boot();
    return () => teardown();
  });
  onDestroy(() => teardown());

  function currentQuery(): LogsQuery {
    return {
      level: levelFilter || undefined,
      source: sourceFilter.trim() || undefined,
      search: searchFilter.trim() || undefined,
    };
  }

  async function boot(): Promise<void> {
    connectionState = "connecting";
    errorBanner = null;
    const initial = await api.logs({ ...currentQuery(), max: 200 });
    if (initial.ok) {
      records = initial.data.records;
      counters = initial.data.counters;
      if (initial.data.records.length > 0) {
        lastSeenId = Math.max(...initial.data.records.map((r) => r.id));
      }
    } else {
      if (initial.status === 404) {
        errorBanner = "logs endpoint disabled (Options.LogSink is nil)";
        connectionState = "offline";
        return;
      }
      errorBanner = `load failed: ${initial.error}`;
    }
    openStream();
  }

  function openStream(): void {
    closeStream();
    const url = api.logsStreamURL({ ...currentQuery(), since_id: lastSeenId });
    try {
      es = new EventSource(url);
    } catch {
      startPollingFallback();
      return;
    }
    es.addEventListener("open", () => {
      connectionState = "live";
      errorBanner = null;
    });
    es.addEventListener("log", (ev) => {
      try {
        const rec = JSON.parse((ev as MessageEvent).data) as LogRecord;
        ingest(rec);
      } catch {
        // Malformed frame — skip, next one will arrive.
      }
    });
    es.addEventListener("error", () => {
      connectionState = "offline";
      closeStream();
      // Browsers auto-reconnect EventSource but we want a visible backoff.
      if (!reconnectTimer) {
        reconnectTimer = setTimeout(() => {
          reconnectTimer = null;
          openStream();
        }, 2_500);
      }
    });
  }

  function closeStream(): void {
    if (es) {
      es.close();
      es = null;
    }
    if (reconnectTimer) {
      clearTimeout(reconnectTimer);
      reconnectTimer = null;
    }
  }

  function startPollingFallback(): void {
    connectionState = "polling";
    pollTimer = setInterval(async () => {
      const res = await api.logs({ ...currentQuery(), since_id: lastSeenId });
      if (res.ok) {
        for (const r of res.data.records.slice().reverse()) {
          ingest(r);
        }
        counters = res.data.counters;
      }
    }, 2_000);
  }

  function ingest(r: LogRecord): void {
    if (r.id <= lastSeenId) return;
    lastSeenId = r.id;
    if (paused) return;
    records = [r, ...records].slice(0, MAX_LOCAL);
  }

  function teardown(): void {
    closeStream();
    if (pollTimer) clearInterval(pollTimer);
    pollTimer = null;
    if (filterDebounce) clearTimeout(filterDebounce);
    filterDebounce = null;
  }

  // Filter changes re-open the stream so server-side filtering is
  // authoritative. Debounce typing so we don't reconnect on every keystroke.
  $effect(() => {
    const _sig = levelFilter + "\u0000" + sourceFilter + "\u0000" + searchFilter;
    void _sig;
    if (filterDebounce) clearTimeout(filterDebounce);
    filterDebounce = setTimeout(() => {
      lastSeenId = 0;
      records = [];
      void boot();
    }, 220);
  });

  function toggleExpand(id: number): void {
    const next = new Set(expandedIds);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    expandedIds = next;
  }

  function levelClass(l: string): string {
    switch (l.toUpperCase()) {
      case "ERROR":
        return "lv--error";
      case "WARN":
        return "lv--warn";
      case "INFO":
        return "lv--info";
      case "DEBUG":
        return "lv--debug";
      default:
        return "lv--info";
    }
  }

  function fmtRowText(r: LogRecord): string {
    const ts = new Date(r.time).toISOString();
    const lv = r.level.padEnd(5);
    const src = r.source ?? "";
    const fields = r.fields
      ? Object.entries(r.fields)
          .map(([k, v]) => `${k}=${v}`)
          .join(" ")
      : "";
    return `${ts} ${lv} ${src} ${r.message} ${fields}`.trim();
  }

  async function copyRow(r: LogRecord): Promise<void> {
    try {
      await navigator.clipboard.writeText(fmtRowText(r));
    } catch {
      /* ignore — no toast store here, silent fail */
    }
  }

  function clearLocal(): void {
    records = [];
    expandedIds = new Set();
  }

  function togglePause(): void {
    paused = !paused;
  }
</script>

<section class="logs" aria-label="live logs">
  <header class="logs__head">
    <div class="logs__title">
      <span class="caps">logs</span>
      <span class="logs__count mono tabular">{records.length}</span>
      <span class="logs__status mono" style="font-size: var(--fs-2xs)">
        {#if connectionState === "live"}
          <span class="pip pip--good" aria-hidden="true"></span>live
        {:else if connectionState === "polling"}
          <span class="pip pip--accent" aria-hidden="true"></span>polling
        {:else if connectionState === "connecting"}
          <span class="pip pip--warn" aria-hidden="true"></span>connecting
        {:else}
          <span class="pip pip--bad" aria-hidden="true"></span>offline
        {/if}
        {#if counters}
          <span class="faint">
            · {counters.buffered}/{counters.capacity}
          </span>
        {/if}
      </span>
    </div>
    <div class="logs__filters">
      <label class="fld" aria-label="level">
        <span class="caps">level</span>
        <select bind:value={levelFilter} class="fld__input mono">
          <option value="">all</option>
          <option value="DEBUG">debug</option>
          <option value="INFO">info</option>
          <option value="WARN">warn</option>
          <option value="ERROR">error</option>
        </select>
      </label>
      <label class="fld" aria-label="source substring">
        <span class="caps">source</span>
        <input
          type="text"
          bind:value={sourceFilter}
          class="fld__input mono"
          placeholder="server/logging"
          spellcheck="false"
          autocomplete="off"
        />
      </label>
      <label class="fld fld--wide" aria-label="message + fields search">
        <span class="caps">search</span>
        <input
          type="text"
          bind:value={searchFilter}
          class="fld__input mono"
          placeholder="text, path, account id…"
          spellcheck="false"
          autocomplete="off"
        />
      </label>
      <button
        type="button"
        class="seg"
        class:seg--active={paused}
        onclick={togglePause}
        title="pause live append"
        aria-pressed={paused}
      >
        <Icon name={paused ? "play" : "pause"} size={11} />
        {paused ? "paused" : "live"}
      </button>
      <button type="button" class="seg" onclick={clearLocal} title="clear local view">
        <Icon name="trash" size={11} />
        clear
      </button>
    </div>
  </header>

  {#if errorBanner}
    <div class="logs__banner" role="status">{errorBanner}</div>
  {/if}

  <div class="logs__body">
    {#if records.length === 0}
      <div class="logs__empty">
        <span class="faint mono">no log records match the current filter</span>
      </div>
    {:else}
      {#each records as r (r.id)}
        <div
          class="row"
          class:row--expanded={expandedIds.has(r.id)}
          role="button"
          tabindex="0"
          onclick={() => toggleExpand(r.id)}
          onkeydown={(e) => {
            if (e.key === "Enter" || e.key === " ") {
              e.preventDefault();
              toggleExpand(r.id);
            }
          }}
        >
          <span class="row__time mono tabular">{shortTime(r.time)}</span>
          <span class="row__level lv {levelClass(r.level)} mono caps">{r.level}</span>
          <span class="row__source mono faint">{r.source ?? "—"}</span>
          <span class="row__msg">{r.message}</span>
          <button
            type="button"
            class="row__copy"
            title="copy row"
            onclick={(e) => {
              e.stopPropagation();
              void copyRow(r);
            }}
            aria-label="copy log row"
          >
            <Icon name="copy" size={11} />
          </button>

          {#if expandedIds.has(r.id) && r.fields && Object.keys(r.fields).length > 0}
            <dl class="row__fields mono">
              {#each Object.entries(r.fields) as [k, v]}
                <dt>{k}</dt>
                <dd>{v}</dd>
              {/each}
            </dl>
          {/if}
        </div>
      {/each}
    {/if}
  </div>
</section>

<style>
  .logs {
    display: flex;
    flex-direction: column;
    min-block-size: 0;
    background: var(--c-surface);
    border: 1px solid var(--c-border);
    border-radius: var(--r-md);
    box-shadow: var(--sh-1);
    max-block-size: calc(100dvh - 160px);
  }

  .logs__head {
    display: flex;
    align-items: center;
    gap: var(--sp-5);
    padding: var(--sp-3) var(--sp-5);
    border-block-end: 1px solid var(--c-rule);
    flex-wrap: wrap;
  }
  .logs__title {
    display: inline-flex;
    align-items: baseline;
    gap: var(--sp-3);
  }
  .logs__count {
    font-size: var(--fs-xs);
    color: var(--c-accent);
    padding: 1px 6px;
    border: 1px solid color-mix(in oklch, var(--c-accent), transparent 60%);
    border-radius: var(--r-pill);
    background: var(--c-accent-wash);
  }
  .logs__status {
    color: var(--c-text-dim);
  }

  .logs__filters {
    display: inline-flex;
    align-items: center;
    gap: var(--sp-3);
    margin-inline-start: auto;
    flex-wrap: wrap;
  }
  .fld {
    display: inline-flex;
    align-items: center;
    gap: var(--sp-2);
    padding: 2px var(--sp-3);
    background: var(--c-surface-sunken);
    border: 1px solid var(--c-rule);
    border-radius: var(--r-sm);
  }
  .fld__input {
    font-size: var(--fs-xs);
    color: var(--c-text);
    background: transparent;
    border: 0;
    outline: none;
    min-inline-size: 120px;
  }
  .fld--wide .fld__input {
    min-inline-size: 200px;
  }
  .fld__input::placeholder {
    color: var(--c-text-faint);
  }

  .seg {
    display: inline-flex;
    align-items: center;
    gap: var(--sp-2);
    padding: 3px 8px;
    font-size: var(--fs-xs);
    font-family: var(--font-mono);
    letter-spacing: var(--tr-wide);
    text-transform: uppercase;
    color: var(--c-text-dim);
    background: var(--c-surface-sunken);
    border: 1px solid var(--c-rule);
    border-radius: var(--r-sm);
    transition: color var(--mo-fast) var(--ease-std);
  }
  .seg:hover {
    color: var(--c-text);
  }
  .seg--active {
    color: var(--c-accent);
    box-shadow: var(--sh-1), inset 0 0 0 1px color-mix(in oklch, var(--c-accent), transparent 60%);
  }

  .logs__banner {
    padding: var(--sp-3) var(--sp-5);
    font-size: var(--fs-xs);
    font-family: var(--font-mono);
    color: var(--c-warn);
    background: var(--c-warn-bg);
    border-block-end: 1px solid var(--c-rule);
  }

  .logs__body {
    flex: 1 1 auto;
    overflow-y: auto;
    min-block-size: 200px;
  }
  .logs__empty {
    padding: var(--sp-6);
    text-align: center;
  }

  .row {
    display: grid;
    grid-template-columns: 72px 58px minmax(120px, 220px) minmax(0, 1fr) 26px;
    gap: var(--sp-3);
    align-items: start;
    padding: var(--sp-2) var(--sp-5);
    border-block-end: 1px solid var(--c-rule);
    font-size: var(--fs-sm);
    cursor: pointer;
    transition: background var(--mo-fast) var(--ease-std);
  }
  .row:hover {
    background: var(--c-surface-hover);
  }
  .row:focus-visible {
    outline: none;
    box-shadow: inset 0 0 0 2px var(--c-accent);
  }

  .row__time {
    color: var(--c-text-faint);
    font-size: var(--fs-xs);
    padding-block-start: 2px;
  }
  .row__level {
    padding: 1px 6px;
    border-radius: var(--r-sm);
    font-size: var(--fs-2xs);
    text-align: center;
    justify-self: start;
  }
  .lv--error {
    color: var(--c-danger);
    background: var(--c-danger-bg);
  }
  .lv--warn {
    color: var(--c-warn);
    background: var(--c-warn-bg);
  }
  .lv--info {
    color: var(--c-info);
    background: var(--c-info-bg);
  }
  .lv--debug {
    color: var(--c-text-dim);
    background: var(--c-surface-sunken);
  }

  .row__source {
    font-size: var(--fs-xs);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    padding-block-start: 2px;
  }
  .row__msg {
    color: var(--c-text);
    word-break: break-word;
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
  }
  .row__copy {
    opacity: 0;
    color: var(--c-text-faint);
    background: transparent;
    border: 0;
    padding: 2px;
    transition: opacity var(--mo-fast) var(--ease-std);
  }
  .row:hover .row__copy,
  .row:focus-within .row__copy {
    opacity: 1;
  }
  .row__copy:hover {
    color: var(--c-accent);
  }

  .row__fields {
    grid-column: 3 / -1;
    display: grid;
    grid-template-columns: auto minmax(0, 1fr);
    gap: var(--sp-1) var(--sp-4);
    margin-block-start: var(--sp-2);
    padding: var(--sp-3);
    background: var(--c-surface-sunken);
    border: 1px solid var(--c-rule);
    border-radius: var(--r-sm);
    font-size: var(--fs-xs);
  }
  .row__fields dt {
    color: var(--c-accent);
  }
  .row__fields dd {
    color: var(--c-text);
    overflow-wrap: anywhere;
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
</style>
