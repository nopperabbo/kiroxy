<!--
  ModelsView — canonical Claude model table with per-model usage stats.

  Rows are sortable by click on column headers. Grouping by tier toggles
  between "all models" and "free then pro" (visual grouping only; the
  table is still a single sortable list).

  Usage stats (requests / avg latency) are OPTIONAL — live numbers come
  from /metrics which is Prometheus text format. For v1.1.0 we ship the
  static table and show "—" in usage columns; v1.2.0 will parse /metrics
  client-side and fill them in.
-->
<script lang="ts">
  import { onMount } from "svelte";
  import { api, type ModelEntry } from "../lib/api";
  import Icon from "./Icon.svelte";

  type SortKey = "anthropic" | "kiro" | "context" | "family";
  const DISABLE_KEY = "mansion.models.disabled";

  let models: ModelEntry[] = $state([]);
  let defaultModel = $state("");
  let loadErr: string | null = $state(null);
  let sortKey: SortKey = $state("anthropic");
  let sortDesc = $state(false);
  let groupByTier = $state(true);
  let disabled: Set<string> = $state(loadDisabled());
  let copiedId: string | null = $state(null);
  let probing: string | null = $state(null);
  let probeResult: Record<string, { ok: boolean; latency_ms: number; status: number; note?: string }> = $state({});

  onMount(() => void load());

  async function load(): Promise<void> {
    const r = await api.modelsList();
    if (r.ok) {
      models = r.data.models;
      defaultModel = r.data.default_model;
      loadErr = null;
    } else {
      loadErr = `load failed: ${r.error}`;
    }
  }

  function loadDisabled(): Set<string> {
    try {
      const raw = localStorage.getItem(DISABLE_KEY);
      if (!raw) return new Set();
      const arr = JSON.parse(raw) as string[];
      return new Set(Array.isArray(arr) ? arr : []);
    } catch {
      return new Set();
    }
  }

  function persistDisabled(): void {
    try {
      localStorage.setItem(DISABLE_KEY, JSON.stringify([...disabled]));
    } catch {
      /* storage disabled — non-fatal */
    }
  }

  function toggleDisabled(id: string): void {
    if (disabled.has(id)) {
      disabled.delete(id);
    } else {
      disabled.add(id);
    }
    disabled = new Set(disabled);
    persistDisabled();
  }

  async function copyCurl(m: ModelEntry): Promise<void> {
    const curl = `curl http://localhost:8787/v1/messages \\
  -H "anthropic-version: 2023-06-01" \\
  -H "x-api-key: $KIROXY_API_KEY" \\
  -H "content-type: application/json" \\
  -d '{"model":"${m.anthropic}","max_tokens":64,"messages":[{"role":"user","content":"ping"}]}'`;
    try {
      await navigator.clipboard.writeText(curl);
      copiedId = m.anthropic;
      setTimeout(() => {
        if (copiedId === m.anthropic) copiedId = null;
      }, 1600);
    } catch {
      copiedId = null;
    }
  }

  async function probeModel(m: ModelEntry): Promise<void> {
    if (probing) return;
    probing = m.anthropic;
    const t0 = performance.now();
    try {
      const res = await fetch("/v1/messages", {
        method: "POST",
        headers: {
          "anthropic-version": "2023-06-01",
          "content-type": "application/json",
        },
        body: JSON.stringify({
          model: m.anthropic,
          max_tokens: 16,
          messages: [{ role: "user", content: "ping" }],
        }),
      });
      const dt = Math.round(performance.now() - t0);
      probeResult[m.anthropic] = {
        ok: res.ok,
        latency_ms: dt,
        status: res.status,
        note: res.status === 401 ? "auth required (set inbound key)" : res.status >= 500 ? "upstream error" : undefined,
      };
    } catch (e) {
      probeResult[m.anthropic] = {
        ok: false,
        latency_ms: Math.round(performance.now() - t0),
        status: 0,
        note: e instanceof Error ? e.message : "network error",
      };
    } finally {
      probing = null;
    }
  }

  function setSort(k: SortKey): void {
    if (sortKey === k) {
      sortDesc = !sortDesc;
    } else {
      sortKey = k;
      sortDesc = false;
    }
  }

  function cmpStr(a: string, b: string): number {
    return a.localeCompare(b);
  }

  let sorted = $derived(
    [...models].sort((a, b) => {
      const dir = sortDesc ? -1 : 1;
      switch (sortKey) {
        case "anthropic":
          return cmpStr(a.anthropic, b.anthropic) * dir;
        case "kiro":
          return cmpStr(a.kiro, b.kiro) * dir;
        case "context":
          return (a.context_window_size - b.context_window_size) * dir;
        case "family":
          return cmpStr(a.family, b.family) * dir;
        default:
          return 0;
      }
    }),
  );

  let grouped = $derived(() => {
    if (!groupByTier) return [{ tier: "all", rows: sorted }];
    const pro = sorted.filter((m) => m.tier === "pro");
    const free = sorted.filter((m) => m.tier !== "pro");
    return [
      { tier: "pro", rows: pro },
      { tier: "free", rows: free },
    ].filter((g) => g.rows.length > 0);
  });

  function fmtContext(n: number): string {
    if (n >= 1_000_000) return "1M";
    if (n >= 1000) return `${Math.round(n / 1000)}K`;
    return String(n);
  }
</script>

<section class="models" aria-label="canonical models">
  <header class="models__head">
    <div class="models__title">
      <span class="caps">models</span>
      <span class="models__count mono tabular">{models.length}</span>
    </div>
    <div class="models__tools">
      <label class="toggle">
        <input type="checkbox" bind:checked={groupByTier} />
        <span class="caps">group by tier</span>
      </label>
    </div>
  </header>

  {#if loadErr}
    <div class="banner banner--err">{loadErr}</div>
  {/if}

  {#if defaultModel}
    <div class="default-row mono">
      <span class="caps faint">default</span>
      <code class="default-row__id">{defaultModel}</code>
      <span class="faint">— used when a non-claude model is requested</span>
    </div>
  {/if}

  <div class="models__body">
    {#if models.length === 0 && !loadErr}
      <div class="empty mono faint">loading…</div>
    {:else}
      {#each grouped() as g}
        {#if groupByTier}
          <h4 class="group caps">{g.tier} tier · {g.rows.length} {g.rows.length === 1 ? "model" : "models"}</h4>
        {/if}
        <table class="tbl">
          <thead>
            <tr>
              <th class="th" onclick={() => setSort("anthropic")}>
                anthropic id
                <span class="th__arrow">{sortKey === "anthropic" ? (sortDesc ? "▼" : "▲") : ""}</span>
              </th>
              <th class="th" onclick={() => setSort("kiro")}>
                upstream kiro sku
                <span class="th__arrow">{sortKey === "kiro" ? (sortDesc ? "▼" : "▲") : ""}</span>
              </th>
              <th class="th th--num" onclick={() => setSort("context")}>
                context
                <span class="th__arrow">{sortKey === "context" ? (sortDesc ? "▼" : "▲") : ""}</span>
              </th>
              <th class="th" onclick={() => setSort("family")}>
                family
                <span class="th__arrow">{sortKey === "family" ? (sortDesc ? "▼" : "▲") : ""}</span>
              </th>
              <th>mode</th>
              <th class="th--actions">actions</th>
            </tr>
          </thead>
          <tbody>
            {#each g.rows as m}
              {@const probe = probeResult[m.anthropic]}
              <tr class="row row--{m.tier} row--{m.family}" class:row--disabled={disabled.has(m.anthropic)}>
                <td class="cell__id mono">
                  {m.anthropic}
                  {#if m.anthropic === defaultModel}
                    <span class="badge badge--default caps">default</span>
                  {/if}
                  {#if disabled.has(m.anthropic)}
                    <span class="badge badge--off caps">off</span>
                  {/if}
                </td>
                <td class="cell__kiro mono">
                  {m.kiro}
                  {#if m.kiro_1m}
                    <span class="cell__kiro1m faint mono"> / {m.kiro_1m}</span>
                  {/if}
                </td>
                <td class="cell__ctx mono tabular">{fmtContext(m.context_window_size)}</td>
                <td class="cell__fam mono">{m.family}</td>
                <td class="cell__mode">
                  {#if m.is_thinking}
                    <span class="badge badge--thinking caps">1M</span>
                  {:else}
                    <span class="badge badge--standard caps">std</span>
                  {/if}
                  {#if m.tier === "pro"}
                    <span class="badge badge--pro caps">pro</span>
                  {:else}
                    <span class="badge badge--free caps">free</span>
                  {/if}
                </td>
                <td class="cell__actions">
                  <div class="row-actions">
                    <button
                      type="button"
                      class="ic-btn"
                      class:ic-btn--off={disabled.has(m.anthropic)}
                      onclick={() => toggleDisabled(m.anthropic)}
                      title={disabled.has(m.anthropic) ? "enable" : "disable (UI hint, not enforced server-side)"}
                      aria-label="toggle model"
                    >
                      <Icon name={disabled.has(m.anthropic) ? "x" : "check"} size={11} />
                    </button>
                    <button
                      type="button"
                      class="ic-btn"
                      onclick={() => void copyCurl(m)}
                      title="copy curl example"
                      aria-label="copy curl"
                    >
                      <Icon name={copiedId === m.anthropic ? "check" : "copy"} size={11} />
                    </button>
                    <button
                      type="button"
                      class="ic-btn"
                      onclick={() => void probeModel(m)}
                      disabled={probing === m.anthropic}
                      title="probe model with a 16-token ping"
                      aria-label="probe"
                    >
                      <Icon name="zap" size={11} />
                    </button>
                    {#if probe}
                      <span
                        class="probe"
                        class:probe--ok={probe.ok}
                        class:probe--bad={!probe.ok}
                        title={probe.note ?? `${probe.status} · ${probe.latency_ms}ms`}
                      >
                        {probe.status} · {probe.latency_ms}ms
                      </span>
                    {/if}
                  </div>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      {/each}
      <p class="footnote mono faint">
        per-model usage stats arrive in v1.2.0 — for now scrape <code>/metrics</code>
        (promql on <code>kiroxy_requests_total{"{model=\"…\"}"}</code>) for live counts.
      </p>
    {/if}
  </div>
</section>

<style>
  .models {
    display: flex;
    flex-direction: column;
    min-block-size: 0;
    background: var(--c-surface);
    border: 1px solid var(--c-border);
    border-radius: var(--r-md);
    box-shadow: var(--sh-1);
  }
  .models__head {
    display: flex;
    align-items: center;
    gap: var(--sp-5);
    padding: var(--sp-3) var(--sp-5);
    border-block-end: 1px solid var(--c-rule);
  }
  .models__title {
    display: inline-flex;
    align-items: baseline;
    gap: var(--sp-3);
  }
  .models__count {
    font-size: var(--fs-xs);
    color: var(--c-accent);
    padding: 1px 6px;
    border: 1px solid color-mix(in oklch, var(--c-accent), transparent 60%);
    border-radius: var(--r-pill);
    background: var(--c-accent-wash);
  }
  .models__tools {
    margin-inline-start: auto;
  }

  .toggle {
    display: inline-flex;
    align-items: center;
    gap: var(--sp-2);
    font-family: var(--font-mono);
    font-size: var(--fs-xs);
    color: var(--c-text-dim);
    cursor: pointer;
  }
  .toggle input {
    margin: 0;
  }

  .banner {
    padding: var(--sp-3) var(--sp-5);
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
  }
  .banner--err {
    color: var(--c-warn);
    background: var(--c-warn-bg);
    border-block-end: 1px solid var(--c-rule);
  }

  .default-row {
    display: flex;
    align-items: center;
    gap: var(--sp-3);
    padding: var(--sp-3) var(--sp-5);
    border-block-end: 1px solid var(--c-rule);
    font-size: var(--fs-sm);
  }
  .default-row__id {
    color: var(--c-accent);
    background: var(--c-accent-wash);
    padding: 1px 6px;
    border-radius: var(--r-sm);
  }

  .models__body {
    padding: var(--sp-4) var(--sp-5);
    overflow-y: auto;
  }
  .empty {
    padding: var(--sp-6);
    text-align: center;
  }
  .group {
    font-size: var(--fs-2xs);
    letter-spacing: var(--tr-caps);
    color: var(--c-text-dim);
    margin: var(--sp-4) 0 var(--sp-2);
  }
  .group:first-child {
    margin-block-start: 0;
  }

  .tbl {
    inline-size: 100%;
    border-collapse: collapse;
    font-size: var(--fs-sm);
    margin-block-end: var(--sp-5);
  }
  .tbl th,
  .tbl td {
    text-align: start;
    padding: var(--sp-2) var(--sp-4);
    border-block-end: 1px solid var(--c-rule);
  }
  .th {
    text-transform: uppercase;
    letter-spacing: var(--tr-wide);
    font-size: var(--fs-2xs);
    color: var(--c-text-dim);
    font-weight: var(--fw-normal);
    cursor: pointer;
    user-select: none;
  }
  .th:hover {
    color: var(--c-text);
  }
  .th__arrow {
    display: inline-block;
    min-inline-size: 8px;
    color: var(--c-accent);
    margin-inline-start: var(--sp-2);
  }
  .th--num {
    text-align: end;
  }

  .cell__id {
    color: var(--c-text);
    white-space: nowrap;
  }
  .cell__kiro {
    color: var(--c-accent);
    white-space: nowrap;
  }
  .cell__kiro1m {
    font-size: var(--fs-xs);
  }
  .cell__ctx {
    text-align: end;
    color: var(--c-text);
  }
  .cell__fam {
    color: var(--c-text-dim);
    text-transform: lowercase;
  }
  .cell__mode {
    display: inline-flex;
    gap: var(--sp-2);
    align-items: center;
  }

  .row--disabled {
    opacity: 0.45;
  }
  .row--disabled .cell__id {
    text-decoration: line-through;
  }
  .badge--off {
    color: var(--c-danger);
    background: color-mix(in oklch, var(--c-danger), transparent 80%);
    border: 1px solid color-mix(in oklch, var(--c-danger), transparent 60%);
    margin-inline-start: var(--sp-2);
  }
  .th--actions {
    inline-size: 220px;
    text-align: end;
  }
  .cell__actions {
    text-align: end;
  }
  .row-actions {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    flex-wrap: wrap;
    justify-content: flex-end;
  }
  .ic-btn {
    inline-size: 22px;
    block-size: 22px;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    color: var(--c-text-dim);
    background: transparent;
    border: 1px solid var(--c-rule);
    border-radius: var(--r-sm);
    cursor: pointer;
    transition: color var(--mo-fast) var(--ease-std), border-color var(--mo-fast) var(--ease-std);
  }
  .ic-btn:hover {
    color: var(--c-text);
    border-color: var(--c-border-strong);
  }
  .ic-btn:disabled {
    opacity: 0.4;
    cursor: wait;
  }
  .ic-btn--off {
    color: var(--c-danger);
    border-color: color-mix(in oklch, var(--c-danger), transparent 60%);
  }
  .probe {
    font-family: var(--font-mono);
    font-size: var(--fs-2xs);
    padding: 2px 6px;
    border-radius: var(--r-sm);
    border: 1px solid var(--c-rule);
  }
  .probe--ok {
    color: var(--c-success);
    border-color: color-mix(in oklch, var(--c-success), transparent 60%);
  }
  .probe--bad {
    color: var(--c-danger);
    border-color: color-mix(in oklch, var(--c-danger), transparent 60%);
  }

  @media (max-width: 720px) {
    .th--actions { inline-size: auto; }
    .cell__actions { text-align: start; }
    .row-actions { justify-content: flex-start; }
  }

  .badge {
    padding: 1px 6px;
    border-radius: var(--r-pill);
    font-size: 9px;
    font-family: var(--font-mono);
    font-weight: var(--fw-semibold);
  }
  .badge--default {
    color: var(--c-accent);
    background: var(--c-accent-wash);
    border: 1px solid color-mix(in oklch, var(--c-accent), transparent 60%);
    margin-inline-start: var(--sp-2);
  }
  .badge--thinking {
    color: var(--c-info);
    background: var(--c-info-bg);
  }
  .badge--standard {
    color: var(--c-text-faint);
    background: var(--c-surface-sunken);
  }
  .badge--pro {
    color: var(--c-warn);
    background: var(--c-warn-bg);
  }
  .badge--free {
    color: var(--c-success);
    background: var(--c-success-bg);
  }

  .footnote {
    font-size: var(--fs-xs);
    color: var(--c-text-faint);
    padding-block-start: var(--sp-3);
    border-block-start: 1px dashed var(--c-rule);
  }
  .footnote code {
    padding: 1px 4px;
    background: var(--c-surface-sunken);
    border-radius: var(--r-xs);
    color: var(--c-accent);
  }
</style>
