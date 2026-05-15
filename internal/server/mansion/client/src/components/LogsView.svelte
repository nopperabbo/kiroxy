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
  import { store } from "../lib/store.svelte";
  import { api, type LogRecord, type LogsQuery } from "../lib/api";
  import { shortTime } from "../lib/format";
  import Icon from "./Icon.svelte";
  import EmptyState from "./EmptyState.svelte";

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

  // P3.2 multi-level chips: when one or more levels selected, drives both
  // the request to /logs (lowest of the set, server filters >= that) and
  // the local mask. Empty set = "all" (no client-side mask).
  const ALL_LEVELS = ["DEBUG", "INFO", "WARN", "ERROR"] as const;
  type Level = (typeof ALL_LEVELS)[number];
  let levelMask = $state<Set<Level>>(new Set());
  function toggleLevel(l: Level): void {
    const next = new Set(levelMask);
    if (next.has(l)) next.delete(l);
    else next.add(l);
    levelMask = next;
    // Map mask back to legacy levelFilter so server-side filtering still
    // applies for the lowest level selected. Falsy means "all".
    if (next.size === 0 || next.size === ALL_LEVELS.length) {
      levelFilter = "";
    } else {
      // Lowest level = most permissive on server (slog filters >= level).
      const order: Record<Level, number> = { DEBUG: 0, INFO: 1, WARN: 2, ERROR: 3 };
      let lowest: Level = "ERROR";
      let lowestOrd = 99;
      for (const lv of next) {
        if (order[lv] < lowestOrd) {
          lowestOrd = order[lv];
          lowest = lv;
        }
      }
      levelFilter = lowest;
    }
  }

  // P3.3 wrap toggle for long messages.
  let wrap = $state<boolean>(false);

  // P3.4 facets sidebar visibility.
  let showFacets = $state<boolean>(false);
  function toggleFacets(): void {
    showFacets = !showFacets;
  }

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

  // P3 derived: client-side display mask (when more than one level selected
  // but not all, server returns >= lowest, so we mask the rest locally).
  const displayed = $derived.by(() => {
    if (levelMask.size === 0 || levelMask.size === ALL_LEVELS.length) {
      return records;
    }
    return records.filter((r) => {
      const lv = (r.level || "").toUpperCase() as Level;
      return levelMask.has(lv);
    });
  });

  // P3.1 volume histogram — 60 1-min buckets covering the displayed range.
  // Each bucket carries per-level counts so the rendered bars can stack.
  type VolumeBin = {
    t: number;
    debug: number;
    info: number;
    warn: number;
    error: number;
    total: number;
  };
  const volume = $derived.by(() => {
    const bins = 60;
    const now = Date.now();
    const span = 60 * 60 * 1000; // last hour
    const start = now - span;
    const step = span / bins;
    const out: VolumeBin[] = [];
    for (let i = 0; i < bins; i++) {
      out.push({ t: start + i * step, debug: 0, info: 0, warn: 0, error: 0, total: 0 });
    }
    for (const r of displayed) {
      const ts = Date.parse(r.time);
      if (!Number.isFinite(ts)) continue;
      const idx = Math.floor((ts - start) / step);
      if (idx < 0 || idx >= bins) continue;
      const slot = out[idx];
      slot.total++;
      const lv = (r.level || "").toUpperCase();
      if (lv === "ERROR") slot.error++;
      else if (lv === "WARN") slot.warn++;
      else if (lv === "DEBUG") slot.debug++;
      else slot.info++;
    }
    return out;
  });
  const volumeMax = $derived(Math.max(1, ...volume.map((b) => b.total)));

  // P3.4 field facets — top values for level/source over the displayed window.
  type Facet = { name: string; value: string; count: number };
  const facets = $derived.by(() => {
    const lvCount = new Map<string, number>();
    const srcCount = new Map<string, number>();
    for (const r of displayed) {
      const lv = (r.level || "").toUpperCase();
      lvCount.set(lv, (lvCount.get(lv) ?? 0) + 1);
      const src = r.source || "";
      if (src) srcCount.set(src, (srcCount.get(src) ?? 0) + 1);
    }
    const lvList: Facet[] = [...lvCount.entries()]
      .map(([value, count]) => ({ name: "level", value, count }))
      .sort((a, b) => b.count - a.count);
    const srcList: Facet[] = [...srcCount.entries()]
      .map(([value, count]) => ({ name: "source", value, count }))
      .sort((a, b) => b.count - a.count)
      .slice(0, 8);
    return { lvList, srcList };
  });

  // P3.4 export filtered (current displayed) as JSONL — one record per line.
  function exportLogs(): void {
    if (displayed.length === 0) return;
    const lines = displayed.map((r) => {
      const obj: Record<string, unknown> = {
        time: r.time,
        level: r.level,
        message: r.message,
      };
      if (r.source) obj.source = r.source;
      if (r.fields) obj.fields = r.fields;
      return JSON.stringify(obj);
    });
    const blob = new Blob([lines.join("\n") + "\n"], { type: "application/x-ndjson" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    const stamp = new Date().toISOString().replace(/[:.]/g, "-");
    a.download = `kiroxy-logs-${stamp}.jsonl`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
  }

  // P3.4 facet click applies a filter chip.
  function applyFacet(name: string, value: string): void {
    if (name === "level") {
      const next = new Set<Level>([value as Level]);
      levelMask = next;
      const order: Record<Level, number> = { DEBUG: 0, INFO: 1, WARN: 2, ERROR: 3 };
      // Single-level: server can match exactly via legacy filter.
      levelFilter = value;
      void order; // referenced for future expansions
    } else if (name === "source") {
      sourceFilter = value;
    }
  }

  function focusBin(bin: VolumeBin, e: MouseEvent): void {
    if (bin.total === 0) return;

    if (e.shiftKey) {
      let dominant: Level | null = null;
      if (bin.error / bin.total >= 0.7) dominant = "ERROR";
      else if (bin.warn / bin.total >= 0.7) dominant = "WARN";
      else if (bin.info / bin.total >= 0.7) dominant = "INFO";
      else if (bin.debug / bin.total >= 0.7) dominant = "DEBUG";

      if (dominant) {
        levelMask = new Set([dominant]);
        levelFilter = dominant;
      }
    }

    const step = 60 * 1000;
    const target = displayed.find(r => {
      const ts = Date.parse(r.time);
      return ts >= bin.t && ts < bin.t + step;
    });

    if (target) {
      const el = document.getElementById(`log-row-${target.id}`);
      if (el) {
        el.scrollIntoView({ behavior: "smooth", block: "center" });
        el.classList.add("row--flash");
        setTimeout(() => el.classList.remove("row--flash"), 1600);
      }
    } else {
      store.pushToast("info", "No matching logs in this window — adjust filter");
    }
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
      <div class="lvl-chips" role="group" aria-label="filter by level">
        <span class="caps faint">level</span>
        {#each ALL_LEVELS as lv (lv)}
          <button
            type="button"
            class="lvl-chip lvl-chip--{lv.toLowerCase()}"
            class:lvl-chip--active={levelMask.has(lv)}
            onclick={() => toggleLevel(lv)}
            aria-pressed={levelMask.has(lv)}
            title="toggle {lv.toLowerCase()}"
          >
            <span class="lvl-chip__dot" aria-hidden="true"></span>
            {lv.toLowerCase()}
          </button>
        {/each}
      </div>
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
      <button
        type="button"
        class="seg"
        class:seg--active={wrap}
        onclick={() => (wrap = !wrap)}
        aria-pressed={wrap}
        title="wrap long messages"
      >
        <Icon name="text" size={11} />
        wrap
      </button>
      <button
        type="button"
        class="seg"
        class:seg--active={showFacets}
        onclick={toggleFacets}
        aria-pressed={showFacets}
        title="toggle facets sidebar"
      >
        <Icon name="filter" size={11} />
        facets
      </button>
      <button
        type="button"
        class="seg"
        onclick={exportLogs}
        disabled={displayed.length === 0}
        title="export filtered logs as JSONL"
      >
        <Icon name="download" size={11} />
        export
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

  <!-- ── P3.1 Volume histogram — 60 1-min stacked bars over the last hour ─ -->
  <div class="logs__hist" aria-hidden="true">
    {#each volume as bin, i (i)}
      {@const errH = (bin.error / volumeMax) * 100}
      {@const warnH = (bin.warn / volumeMax) * 100}
      {@const infoH = (bin.info / volumeMax) * 100}
      {@const dbgH = (bin.debug / volumeMax) * 100}
      <button
        type="button"
        class="hist-col"
        title="{new Date(bin.t).toLocaleTimeString()} · {bin.total} records"
        onclick={(e) => focusBin(bin, e)}
      >
        {#if errH > 0}<span class="hist-seg hist-seg--error" style="block-size: {errH}%"></span>{/if}
        {#if warnH > 0}<span class="hist-seg hist-seg--warn" style="block-size: {warnH}%"></span>{/if}
        {#if infoH > 0}<span class="hist-seg hist-seg--info" style="block-size: {infoH}%"></span>{/if}
        {#if dbgH > 0}<span class="hist-seg hist-seg--debug" style="block-size: {dbgH}%"></span>{/if}
      </button>
    {/each}
    <div class="hist-foot mono caps faint">
      <span>−60m</span>
      <span>volume · {displayed.length.toLocaleString()} in window</span>
      <span>now</span>
    </div>
  </div>

  <div class="logs__layout" class:logs__layout--facets={showFacets}>
    <div class="logs__body" class:logs__body--wrap={wrap}>
      {#if displayed.length === 0}
        {#if records.length === 0}
          <EmptyState
            glyph="~"
            title="The record is blank."
            hint="Trigger a request to generate telemetry."
          >
            <button type="button" class="btn btn--accent" onclick={async () => {
              const cmd = 'curl -H "x-api-key: $KIROXY_API_KEY" http://127.0.0.1:8787/v1/models';
              await navigator.clipboard.writeText(cmd);
              store.pushToast("ok", "curl copied — paste in shell to see logs");
            }}>Copy test request</button>
          </EmptyState>
        {:else}
          <EmptyState
            glyph="⌐"
            title="The wire stays hidden under your filter."
            hint="Loosen the filter criteria to reveal the wire."
          />
        {/if}
      {:else}
        {#each displayed as r (r.id)}
          <div
            id="log-row-{r.id}"
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

    {#if showFacets}
      <aside class="logs__facets" aria-label="field facets">
        <header class="facets__head caps faint">facets</header>
        <section class="facets__group">
          <h4 class="facets__title caps">level · {facets.lvList.length}</h4>
          <ul class="facets__list mono">
            {#each facets.lvList as f (f.value)}
              <li>
                <button
                  type="button"
                  class="facets__item"
                  onclick={() => applyFacet("level", f.value)}
                  title="filter by level: {f.value}"
                >
                  <span class="facets__name">{f.value.toLowerCase()}</span>
                  <span class="facets__count tabular">{f.count}</span>
                </button>
              </li>
            {/each}
          </ul>
        </section>
        <section class="facets__group">
          <h4 class="facets__title caps">source · top 8</h4>
          <ul class="facets__list mono">
            {#if facets.srcList.length === 0}
              <li class="facets__empty faint">none</li>
            {/if}
            {#each facets.srcList as f (f.value)}
              <li>
                <button
                  type="button"
                  class="facets__item"
                  onclick={() => applyFacet("source", f.value)}
                  title="filter by source: {f.value}"
                >
                  <span class="facets__name" title={f.value}>{f.value}</span>
                  <span class="facets__count tabular">{f.count}</span>
                </button>
              </li>
            {/each}
          </ul>
        </section>
      </aside>
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
  .row.row--flash {
    animation: row-flash 1.5s ease-out;
  }
  @keyframes row-flash {
    0% { background: color-mix(in oklch, var(--c-accent) 20%, transparent); }
    100% { background: transparent; }
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

  .lvl-chips {
    display: inline-flex;
    align-items: center;
    gap: var(--sp-1);
    padding-inline-end: var(--sp-2);
    border-inline-end: 1px solid var(--c-rule);
    margin-inline-end: var(--sp-2);
  }
  .lvl-chips > .caps {
    margin-inline-end: var(--sp-2);
    font-size: var(--fs-2xs);
  }
  .lvl-chip {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    padding: 2px 7px;
    font-size: var(--fs-2xs);
    font-family: var(--font-mono);
    letter-spacing: var(--tr-wide);
    text-transform: uppercase;
    color: var(--c-text-faint);
    background: var(--c-surface-sunken);
    border: 1px solid var(--c-rule);
    border-radius: var(--r-pill);
    transition: color var(--mo-fast) var(--ease-std);
  }
  .lvl-chip:hover {
    color: var(--c-text);
  }
  .lvl-chip__dot {
    inline-size: 5px;
    block-size: 5px;
    border-radius: 50%;
    background: currentcolor;
  }
  .lvl-chip--debug { color: var(--c-text-dim); }
  .lvl-chip--info { color: var(--c-info); }
  .lvl-chip--warn { color: var(--c-warn); }
  .lvl-chip--error { color: var(--c-danger); }
  .lvl-chip--active {
    background: color-mix(in oklch, currentcolor, transparent 80%);
    box-shadow: inset 0 0 0 1px currentcolor;
  }

  .logs__hist {
    display: grid;
    grid-template-columns: repeat(60, minmax(0, 1fr));
    align-items: end;
    gap: 1px;
    padding: var(--sp-3) var(--sp-5) 0;
    block-size: 56px;
    border-block-end: 1px solid var(--c-rule);
    position: relative;
  }
  .hist-col {
    block-size: 100%;
    display: flex;
    flex-direction: column-reverse;
    background: transparent;
    cursor: pointer;
    border: 0;
    padding: 0;
    transition: opacity var(--mo-fast) var(--ease-std);
  }
  .hist-col:hover {
    opacity: 0.8;
  }
  .hist-col:focus-visible {
    outline: 1px solid var(--c-accent);
    outline-offset: 1px;
    border-radius: 1px;
  }
  .hist-seg {
    inline-size: 100%;
    display: block;
  }
  .hist-seg--debug { background: var(--c-text-dim); opacity: 0.5; }
  .hist-seg--info { background: var(--c-info); }
  .hist-seg--warn { background: var(--c-warn); }
  .hist-seg--error { background: var(--c-danger); }
  .hist-foot {
    grid-column: 1 / -1;
    display: flex;
    justify-content: space-between;
    font-size: var(--fs-2xs);
    margin-block-start: var(--sp-1);
    padding-block-end: var(--sp-1);
  }

  .logs__layout {
    flex: 1 1 auto;
    display: flex;
    min-block-size: 0;
  }
  .logs__layout--facets .logs__body {
    border-inline-end: 1px solid var(--c-rule);
  }
  .logs__body--wrap .row__msg {
    white-space: normal;
    word-break: break-word;
  }
  .logs__body:not(.logs__body--wrap) .row__msg {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .logs__layout > .logs__body {
    flex: 1 1 auto;
  }

  .logs__facets {
    flex: 0 0 220px;
    overflow-y: auto;
    padding: var(--sp-3) var(--sp-4);
    background: var(--c-surface-sunken);
    border-inline-start: 1px solid var(--c-rule);
  }
  .facets__head {
    font-size: var(--fs-2xs);
    margin-block-end: var(--sp-3);
    padding-block-end: var(--sp-2);
    border-block-end: 1px solid var(--c-rule);
  }
  .facets__group {
    margin-block-end: var(--sp-4);
  }
  .facets__title {
    font-size: var(--fs-2xs);
    color: var(--c-text-dim);
    margin-block-end: var(--sp-2);
  }
  .facets__list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .facets__item {
    inline-size: 100%;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--sp-2);
    padding: 3px 6px;
    font-size: var(--fs-xs);
    color: var(--c-text);
    background: transparent;
    border: 0;
    border-radius: var(--r-sm);
    cursor: pointer;
    text-align: start;
    transition: background var(--mo-fast) var(--ease-std);
  }
  .facets__item:hover {
    background: var(--c-surface-hover);
  }
  .facets__name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .facets__count {
    color: var(--c-text-faint);
    font-size: var(--fs-2xs);
    flex-shrink: 0;
  }
  .facets__empty {
    padding: 3px 6px;
    font-size: var(--fs-xs);
  }

  @media (max-width: 480px) {
    .fld, .seg, .btn, .lvl-chip, .facets__item {
      padding-block: 14px;
    }
  }

  @media (max-width: 720px) {
    .logs__hist { display: none; }
    .logs__facets { flex: 0 0 180px; padding: var(--sp-2) var(--sp-3); }
  }
</style>
