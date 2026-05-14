<!--
  LiveRail — the 300px telemetry rail next to the LiveStream feed.

  Four blocks, stacked vertically:
    1) 5-min window mini-tiles: RPS, P95, Err %, Pool
    2) Throughput · 60s spark-strip (neutral stroke)
    3) Model mix (derived from recent request paths)
    4) Upstream status k/v

  The rail stays neutral — no amber in charts or tables (amber budget).
  Data is derived from the existing store (requests + snapshot + totalSpark).
-->
<script lang="ts">
  import { store } from "../lib/store.svelte";

  let rps = $derived.by(() => {
    const sum = store.totalSpark.reduce((a, b) => a + b, 0);
    if (sum <= 0) return 0;
    return sum / 300;
  });

  let p95 = $derived.by(() => {
    const lats = store.requests.slice(0, 200).map((r) => r.latency_ms).filter((n) => n > 0);
    if (lats.length < 3) return 0;
    lats.sort((a, b) => a - b);
    const idx = Math.floor(lats.length * 0.95);
    return lats[Math.min(idx, lats.length - 1)];
  });

  let errPct = $derived.by(() => {
    const recent = store.requests.slice(0, 500);
    if (recent.length === 0) return 0;
    const errs = recent.filter((r) => r.status >= 400 && r.status !== 0).length;
    return (errs / recent.length) * 100;
  });

  let activeCount = $derived(
    store.snapshot.accounts.filter((a) => a.enabled && !a.cooldown_until).length,
  );
  let refreshCount = $derived(
    store.snapshot.accounts.filter((a) => a.cooldown_until).length,
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
    const total = store.requests.slice(0, 500).length || 1;
    const items = [...buckets.entries()]
      .sort((a, b) => b[1] - a[1])
      .slice(0, 4)
      .map(([name, n]) => ({ name, pct: n / total }));
    return items;
  });

  // Build the throughput sparkline path. Neutral stroke — NOT amber.
  // Points sampled from totalSpark (30 buckets = 5 min, every 10s).
  let sparkPath = $derived.by(() => buildPath(store.totalSpark, 300, 40));
  let sparkArea = $derived.by(() => buildArea(store.totalSpark, 300, 40));

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
  function buildArea(v: number[], w: number, h: number): string {
    if (v.length === 0) return "";
    const line = buildPath(v, w, h);
    return `${line} L ${w} ${h} L 0 ${h} Z`;
  }

  function fmtDelta(): string {
    const n = store.requests.length;
    if (n === 0) return "awaiting first signal";
    return `${n} in ring`;
  }
</script>

<div class="rail">
  <div class="rail-block">
    <div class="rail-eyebrow caps">5-min window</div>
    <div class="tile-grid">
      <div class="tile">
        <div class="tile__lbl caps">RPS</div>
        <div class="tile__val mono tabular">{rps.toFixed(1)}</div>
        <div class="tile__delta faint mono">{fmtDelta()}</div>
      </div>
      <div class="tile">
        <div class="tile__lbl caps">P95</div>
        <div class="tile__val mono tabular">
          {p95 > 0 ? (p95 >= 1000 ? (p95 / 1000).toFixed(2) : Math.round(p95)) : "—"}
          <span class="tile__unit faint">{p95 >= 1000 ? "s" : p95 > 0 ? "ms" : ""}</span>
        </div>
        <div class="tile__delta faint mono">over last 200</div>
      </div>
      <div class="tile">
        <div class="tile__lbl caps">Err %</div>
        <div class="tile__val mono tabular">
          {errPct.toFixed(2)}<span class="tile__unit faint">%</span>
        </div>
        <div class="tile__delta faint mono">last 500 req</div>
      </div>
      <div class="tile">
        <div class="tile__lbl caps">Pool</div>
        <div class="tile__val mono tabular">{activeCount}/{store.snapshot.accounts.length}</div>
        <div class="tile__delta faint mono">
          {refreshCount > 0 ? `${refreshCount} cooling` : "all warm"}
        </div>
      </div>
    </div>
  </div>

  <div class="rail-block">
    <div class="rail-eyebrow caps">Throughput · 5m</div>
    <div class="spark-strip">
      <svg viewBox="0 0 300 40" preserveAspectRatio="none" aria-hidden="true">
        {#if !sparkPath}
          <line x1="0" y1="38" x2="300" y2="38" class="spark__baseline" />
          {#each Array(6) as _, i}
            <line x1={i * 60} y1="38" x2={i * 60} y2="34" class="spark__tick" />
          {/each}
        {/if}
        {#if sparkArea}<path d={sparkArea} class="spark__area" />{/if}
        {#if sparkPath}<path d={sparkPath} class="spark__line" />{/if}
      </svg>
    </div>
  </div>

  <div class="rail-block">
    <div class="rail-eyebrow caps">Model Mix · recent</div>
    {#if modelMix.length === 0 || (modelMix.length === 1 && modelMix[0].name === "other")}
      <p class="empty-italic">No traffic tagged yet. Wire is still quiet.</p>
    {:else}
      <div class="mix">
        {#each modelMix as m}
          <div>
            <div class="mix-row mono">
              <span class="mix-name">{m.name}</span>
              <span class="mix-pct tabular">{(m.pct * 100).toFixed(1)}%</span>
            </div>
            <div class="mix-bar" aria-hidden="true">
              <span style="transform: scaleX({m.pct.toFixed(3)});"></span>
            </div>
          </div>
        {/each}
      </div>
    {/if}
  </div>

  <div class="rail-block rail-block--grow">
    <div class="rail-eyebrow caps">Upstream</div>
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
      <dd class="mono">
        {store.snapshot.vault_ok ? "ok" : "missing"}
      </dd>
      <dt>accounts</dt>
      <dd class="mono tabular">{store.snapshot.accounts.length}</dd>
      <dt>errors total</dt>
      <dd class="mono tabular">{store.snapshot.total_errors ?? 0}</dd>
    </dl>

    {#if store.snapshot.accounts.length > 0}
      <div class="rail-eyebrow caps rail-eyebrow--spaced">Pool health</div>
      <div class="heat" aria-label="account health heatmap">
        {#each store.snapshot.accounts.slice(0, 64) as a}
          <span
            class="heat__cell"
            class:heat__cell--ok={a.enabled && !a.cooldown_until}
            class:heat__cell--cool={!!a.cooldown_until}
            class:heat__cell--off={!a.enabled}
            title="{a.id} · {a.cooldown_until ? 'cooling' : a.enabled ? 'ok' : 'disabled'}"
          ></span>
        {/each}
      </div>
      <p class="heat__legend mono faint">
        <span class="heat__swatch heat__swatch--ok"></span> {activeCount} active
        <span class="heat__swatch heat__swatch--cool"></span> {refreshCount} cool
      </p>
    {/if}
  </div>
</div>

<style>
  .rail {
    display: flex;
    flex-direction: column;
    min-block-size: 0;
  }
  .rail-block {
    padding: var(--sp-4);
    border-block-end: 1px solid var(--c-border);
  }
  .rail-block--grow { flex: 1; }
  .rail-eyebrow {
    color: var(--c-text-faint);
    font-size: 10.5px;
    letter-spacing: 0.08em;
    margin-block-end: var(--sp-3);
  }

  .tile-grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 1px;
    background: var(--c-border);
    border: 1px solid var(--c-border);
  }
  .tile {
    padding: 10px 12px;
    background: var(--c-bg);
    display: flex;
    flex-direction: column;
    gap: 3px;
  }
  .tile__lbl {
    font-size: 10.5px;
    color: var(--c-text-faint);
  }
  .tile__val {
    font-size: 20px;
    letter-spacing: -0.02em;
    color: var(--c-text);
    line-height: 1.0;
  }
  .tile__unit {
    font-size: 13px;
    color: var(--c-text-faint);
    letter-spacing: 0;
  }
  .tile__delta {
    font-size: 11px;
    color: var(--c-text-dim);
  }

  .spark-strip svg {
    inline-size: 100%;
    block-size: 40px;
    display: block;
  }
  /* Neutral stroke — data stays neutral. Amber budget respects this. */
  .spark__line {
    fill: none;
    stroke: var(--c-text-dim);
    stroke-width: 1;
    stroke-linejoin: round;
    stroke-linecap: round;
  }
  .spark__area {
    fill: color-mix(in oklch, var(--c-text-dim), transparent 85%);
    stroke: none;
  }
  .spark__baseline {
    stroke: var(--c-text-faint);
    stroke-width: 1;
    stroke-dasharray: 2 3;
    opacity: 0.5;
  }
  .spark__tick {
    stroke: var(--c-text-faint);
    stroke-width: 1;
    opacity: 0.4;
  }

  .mix {
    display: flex;
    flex-direction: column;
    gap: var(--sp-3);
  }
  .mix-row {
    display: grid;
    grid-template-columns: 1fr auto;
    gap: var(--sp-2);
    align-items: center;
    font-size: var(--fs-sm);
    color: var(--c-text-dim);
  }
  .mix-name { color: var(--c-text); }
  .mix-pct  { color: var(--c-text-dim); }
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
    row-gap: 6px;
    column-gap: var(--sp-4);
    margin: 0;
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

  .rail-eyebrow--spaced { margin-block-start: var(--sp-5); }
  .heat {
    display: grid;
    grid-template-columns: repeat(8, 1fr);
    gap: 3px;
    padding: 2px 0;
  }
  .heat__cell {
    block-size: 14px;
    border-radius: 2px;
    background: color-mix(in oklch, var(--c-text-faint), transparent 70%);
    transition: transform 120ms ease;
  }
  .heat__cell--ok   { background: color-mix(in oklch, var(--c-success), transparent 35%); }
  .heat__cell--cool { background: color-mix(in oklch, var(--c-info), transparent 40%); }
  .heat__cell--off  { background: color-mix(in oklch, var(--c-text-faint), transparent 70%); }
  .heat__cell:hover { transform: scale(1.2); }
  .heat__legend {
    margin: var(--sp-3) 0 0;
    font-size: var(--fs-2xs);
    display: flex;
    align-items: center;
    gap: 4px;
  }
  .heat__swatch {
    display: inline-block;
    inline-size: 8px;
    block-size: 8px;
    border-radius: 1px;
    margin-inline-start: 6px;
  }
  .heat__swatch--ok   { background: color-mix(in oklch, var(--c-success), transparent 35%); }
  .heat__swatch--cool { background: color-mix(in oklch, var(--c-info), transparent 40%); }
</style>
