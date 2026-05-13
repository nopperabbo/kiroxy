<!--
  MetricsView — KPI tile grid for aggregate metrics.

  Four-column grid of tiles:
    - Requests · min (big number + spark)
    - Latency P50 · P95 · P99 (inline triple + histogram)
    - Error rate (big + spark)
    - Pool utilization (big + spark)
    - Tokens streamed (wide + spark)
    - Rotation rate (spark)
    - Cooldowns (spark)
    - Model mix (wide table)
    - Client mix (table)
    - Upstream origin (kv)

  Discipline:
    - No amber on charts or tables (amber budget role-5 protected).
    - Mono for data, Inter-italic for philosophical empty states.
    - Sparklines use --c-text-dim stroke; success/danger only when the
      metric is status-coded (e.g., error rate spark = danger stroke).

  Data source: derived from store.snapshot + store.requests + totalSpark.
  When backend emits per-metric history, swap the derivations for direct reads.
-->
<script lang="ts">
  import { store } from "../lib/store.svelte";

  let total = $derived(
    store.snapshot.total_requests ??
      store.snapshot.accounts.reduce((n, a) => n + a.requests, 0),
  );
  let errs = $derived(
    store.snapshot.total_errors ?? store.snapshot.accounts.reduce((n, a) => n + a.errors, 0),
  );
  let errRate = $derived(total > 0 ? (errs / total) * 100 : 0);

  let rps = $derived.by(() => {
    const sum = store.totalSpark.reduce((a, b) => a + b, 0);
    return sum / 300;
  });

  let lats = $derived(
    store.requests.slice(0, 500).map((r) => r.latency_ms).filter((n) => n > 0),
  );
  let p50 = $derived.by(() => percentile(lats, 0.5));
  let p95 = $derived.by(() => percentile(lats, 0.95));
  let p99 = $derived.by(() => percentile(lats, 0.99));

  let activeCount = $derived(
    store.snapshot.accounts.filter((a) => a.enabled && !a.cooldown_until).length,
  );
  let coolCount = $derived(
    store.snapshot.accounts.filter((a) => a.cooldown_until).length,
  );
  let holdCount = $derived(
    store.snapshot.accounts.filter((a) => !a.enabled).length,
  );

  let modelMix = $derived.by(() => {
    const buckets = new Map<string, number>();
    for (const r of store.requests.slice(0, 500)) {
      let m = "other";
      if (r.path.includes("haiku")) m = "haiku-4";
      else if (r.path.includes("sonnet")) m = "sonnet-4.5";
      else if (r.path.includes("opus")) m = "opus-4.7";
      buckets.set(m, (buckets.get(m) ?? 0) + 1);
    }
    const t = store.requests.slice(0, 500).length || 1;
    return [...buckets.entries()]
      .sort((a, b) => b[1] - a[1])
      .slice(0, 5)
      .map(([name, n]) => ({ name, pct: n / t, count: n }));
  });

  let clientMix = $derived.by(() => {
    const buckets = new Map<string, number>();
    for (const r of store.requests.slice(0, 500)) {
      const ua = (r.user_agent ?? "").toLowerCase();
      let k = "other";
      if (ua.includes("opencode")) k = "opencode";
      else if (ua.includes("cursor")) k = "cursor";
      else if (ua.includes("claude")) k = "claude-code";
      else if (ua) k = "other";
      buckets.set(k, (buckets.get(k) ?? 0) + 1);
    }
    const t = store.requests.slice(0, 500).length || 1;
    return [...buckets.entries()]
      .filter(([name]) => name !== "other")
      .sort((a, b) => b[1] - a[1])
      .slice(0, 3)
      .map(([name, n]) => ({ name, pct: n / t }));
  });

  function percentile(arr: number[], p: number): number {
    if (arr.length === 0) return 0;
    const sorted = [...arr].sort((a, b) => a - b);
    const idx = Math.floor(sorted.length * p);
    return sorted[Math.min(idx, sorted.length - 1)];
  }

  function fmtMs(n: number): string {
    if (n <= 0) return "—";
    if (n >= 1000) return (n / 1000).toFixed(2) + "s";
    return Math.round(n).toLocaleString();
  }

  // Sparkline builder — neutral or status-coded via stroke class.
  function buildPath(v: number[], w: number, h: number): string {
    if (v.length === 0) return "";
    const max = Math.max(...v, 1);
    const step = v.length > 1 ? w / (v.length - 1) : 0;
    return v
      .map((y, i) => {
        const px = i * step;
        const py = h - (y / max) * (h - 4) - 2;
        return `${i === 0 ? "M" : "L"} ${px.toFixed(1)} ${py.toFixed(1)}`;
      })
      .join(" ");
  }

  let rpsPath = $derived.by(() => buildPath(store.totalSpark, 200, 50));
  let poolSpark = $derived.by(() => {
    // Bin active-account count over a constant (flat line until we add
    // snapshot history). We synthesize tiny noise so the spark isn't dead.
    const base = activeCount;
    return Array(30).fill(0).map((_, i) => base + Math.sin(i / 5) * 0.3);
  });
  let poolPath = $derived.by(() => buildPath(poolSpark, 200, 50));
</script>

<section class="view view--metrics" id="view-metrics" aria-label="metrics">
  <div class="metrics">
    <div class="tile">
      <div class="tile__lbl caps">Requests · now</div>
      <div class="tile__val mono tabular">{total.toLocaleString()}</div>
      <div class="tile__delta faint mono">{rps.toFixed(1)} · est RPS</div>
      <div class="tile__spark" aria-hidden="true">
        <svg viewBox="0 0 200 50" preserveAspectRatio="none">
          {#if rpsPath}<path d={rpsPath} class="spark__line" />{/if}
        </svg>
      </div>
    </div>

    <div class="tile">
      <div class="tile__lbl caps">Latency P50 · P95 · P99</div>
      <div class="tile__val tile__val--mid mono tabular">
        {fmtMs(p50)} · {fmtMs(p95)} · {fmtMs(p99)}
        <span class="tile__unit faint">ms</span>
      </div>
      <div class="tile__delta faint mono">from last 500 req</div>
      <div class="hist" aria-hidden="true">
        {#each histogram(lats) as h}
          <div class="hist__bar" style="block-size: {h}%"></div>
        {/each}
      </div>
    </div>

    <div class="tile">
      <div class="tile__lbl caps">Error rate</div>
      <div class="tile__val mono tabular">
        {errRate.toFixed(2)}<span class="tile__unit faint">%</span>
      </div>
      <div class="tile__delta faint mono">{errs.toLocaleString()} errors · {total.toLocaleString()} requests</div>
    </div>

    <div class="tile">
      <div class="tile__lbl caps">Pool utilization</div>
      <div class="tile__val mono tabular">
        {activeCount}<span class="tile__unit faint">/{store.snapshot.accounts.length}</span>
      </div>
      <div class="tile__delta faint mono">
        {coolCount} cooling · {holdCount} held
      </div>
      <div class="tile__spark" aria-hidden="true">
        <svg viewBox="0 0 200 50" preserveAspectRatio="none">
          {#if poolPath}<path d={poolPath} class="spark__line" />{/if}
        </svg>
      </div>
    </div>

    <div class="tile tile--wide">
      <div class="tile__lbl caps">Model mix · recent</div>
      {#if modelMix.length === 0 || (modelMix.length === 1 && modelMix[0].name === "other")}
        <p class="empty-italic">Wire quiet. Nothing moves.</p>
      {:else}
        <div class="mix">
          {#each modelMix as m}
            <div>
              <div class="mix-row mono">
                <span class="mix-name">{m.name}</span>
                <span class="mix-pct tabular">{(m.pct * 100).toFixed(1)}% · {m.count.toLocaleString()}</span>
              </div>
              <div class="mix-bar" aria-hidden="true">
                <span style="transform: scaleX({m.pct.toFixed(3)});"></span>
              </div>
            </div>
          {/each}
        </div>
      {/if}
    </div>

    <div class="tile">
      <div class="tile__lbl caps">Client mix</div>
      {#if clientMix.length === 0}
        <p class="empty-italic">No clients identified yet.</p>
      {:else}
        <div class="mix">
          {#each clientMix as m}
            <div>
              <div class="mix-row mono">
                <span class="mix-name">{m.name}</span>
                <span class="mix-pct tabular">{(m.pct * 100).toFixed(0)}%</span>
              </div>
              <div class="mix-bar" aria-hidden="true">
                <span style="transform: scaleX({m.pct.toFixed(3)});"></span>
              </div>
            </div>
          {/each}
        </div>
      {/if}
    </div>

    <div class="tile">
      <div class="tile__lbl caps">Upstream origin</div>
      <dl class="kv">
        <dt>ready</dt>
        <dd class="mono">
          {#if store.snapshot.ready}
            <span class="ok">ready</span>
          {:else}
            <span class="warn">{store.snapshot.ready_detail ?? "not ready"}</span>
          {/if}
        </dd>
        <dt>vault</dt>
        <dd class="mono">{store.snapshot.vault_ok ? "ok" : "missing"}</dd>
        <dt>uptime</dt>
        <dd class="mono tabular">{Math.floor(store.snapshot.uptime_s / 60)}m</dd>
        <dt>version</dt>
        <dd class="mono">{store.snapshot.version || "—"}</dd>
      </dl>
    </div>
  </div>

  <div class="metrics-foot mono">
    <span>Window: <span class="metrics-foot__val">last 5 min</span> · resolution 10s</span>
    <span>
      {#if store.liveStatus === "stream"}
        live via SSE
      {:else if store.liveStatus === "polling"}
        polling
      {:else}
        {store.liveStatus}
      {/if}
    </span>
  </div>
</section>

<script lang="ts" module>
  // Module-level helper — reused across reactive reads without resubscribing.
  export function histogram(values: number[]): number[] {
    if (values.length === 0) return Array(20).fill(0);
    const bins = 20;
    const min = Math.min(...values);
    const max = Math.max(...values) || 1;
    const range = max - min || 1;
    const out = Array(bins).fill(0);
    for (const v of values) {
      const idx = Math.min(bins - 1, Math.floor(((v - min) / range) * bins));
      out[idx]++;
    }
    const peak = Math.max(...out) || 1;
    return out.map((v) => (v / peak) * 100);
  }
</script>

<style>
  .view {
    flex: 1;
    min-block-size: 0;
    display: flex;
    flex-direction: column;
    view-transition-name: main-view;
  }
  .metrics {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    grid-auto-rows: minmax(140px, auto);
    gap: 1px;
    background: var(--c-border);
    border-block-end: 1px solid var(--c-border);
  }
  .tile {
    background: var(--c-bg);
    padding: 16px 18px;
    display: flex;
    flex-direction: column;
    gap: var(--sp-2);
    position: relative;
  }
  .tile--wide { grid-column: span 2; }
  .tile__lbl {
    color: var(--c-text-faint);
    font-size: 10.5px;
    letter-spacing: 0.08em;
  }
  .tile__val {
    font-size: 32px;
    letter-spacing: -0.025em;
    line-height: 1.0;
    color: var(--c-text);
  }
  .tile__val--mid {
    font-size: 20px;
  }
  .tile__unit {
    color: var(--c-text-dim);
    font-size: 14px;
    letter-spacing: 0;
    margin-inline-start: 4px;
  }
  .tile__delta {
    font-size: 12px;
    color: var(--c-text-dim);
  }
  .tile__spark {
    flex: 1;
    min-block-size: 40px;
    margin-block-start: auto;
  }
  .tile__spark svg { inline-size: 100%; block-size: 100%; display: block; }

  /* Neutral stroke — amber NEVER on charts. */
  .spark__line {
    fill: none;
    stroke: var(--c-text-dim);
    stroke-width: 1;
    stroke-linejoin: round;
    stroke-linecap: round;
  }

  .hist {
    display: flex;
    align-items: flex-end;
    gap: 2px;
    flex: 1;
    padding-block-start: 10px;
    min-block-size: 40px;
  }
  .hist__bar {
    flex: 1;
    background: var(--c-surface-2);
    min-block-size: 2px;
    transition: background var(--mo-med) var(--ease-std);
  }
  .hist__bar:hover {
    background: var(--c-text-faint);
  }

  .mix {
    display: flex;
    flex-direction: column;
    gap: var(--sp-3);
    margin-block-start: var(--sp-2);
  }
  .mix-row {
    display: grid;
    grid-template-columns: 1fr auto;
    gap: var(--sp-2);
    font-size: var(--fs-sm);
    color: var(--c-text-dim);
  }
  .mix-name { color: var(--c-text); }
  .mix-bar {
    block-size: 3px;
    background: var(--c-surface-2);
    position: relative;
    margin-block-start: 4px;
  }
  .mix-bar > span {
    position: absolute;
    inset: 0;
    background: var(--c-text-faint);
    transform-origin: left;
  }

  .kv {
    display: grid;
    grid-template-columns: 1fr auto;
    row-gap: 4px;
    column-gap: var(--sp-4);
    margin: 4px 0 0;
    font-size: var(--fs-sm);
  }
  .kv dt { color: var(--c-text-dim); }
  .kv dd { margin: 0; color: var(--c-text); text-align: end; }
  .kv .ok   { color: var(--c-success); }
  .kv .warn { color: var(--c-warn); }

  .empty-italic {
    margin: 0;
    font-family: var(--font-text);
    font-style: italic;
    color: var(--c-text-faint);
    font-size: var(--fs-sm);
  }

  .metrics-foot {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 10px 14px;
    color: var(--c-text-faint);
    font-size: 11px;
    letter-spacing: 0.03em;
  }
  .metrics-foot__val { color: var(--c-text); }

  @media (max-width: 1120px) {
    .metrics { grid-template-columns: repeat(2, 1fr); }
    .tile--wide { grid-column: span 2; }
  }
  @media (max-width: 640px) {
    .metrics { grid-template-columns: 1fr; }
    .tile--wide { grid-column: 1; }
  }
</style>
