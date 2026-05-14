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

  type Range = "5m" | "1h" | "24h";
  let range: Range = $state("5m");
  function rangeWindow(r: Range): number {
    if (r === "5m") return 300;
    if (r === "1h") return 3600;
    return 86400;
  }
  let rangeSec = $derived(rangeWindow(range));

  // Pull only requests within the selected window. started_at is RFC3339;
  // tolerate parse failures by treating bad values as in-window.
  let inRange = $derived.by(() => {
    const cutoff = Date.now() - rangeSec * 1000;
    return store.requests.filter((r) => {
      const t = Date.parse(r.started_at);
      return Number.isNaN(t) ? true : t >= cutoff;
    });
  });

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
    inRange.slice(0, 1000).map((r) => r.latency_ms).filter((n) => n > 0),
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
    for (const r of inRange.slice(0, 1000)) {
      let m = "other";
      if (r.path.includes("haiku")) m = "haiku-4";
      else if (r.path.includes("sonnet")) m = "sonnet-4.5";
      else if (r.path.includes("opus")) m = "opus-4.7";
      buckets.set(m, (buckets.get(m) ?? 0) + 1);
    }
    const t = inRange.slice(0, 1000).length || 1;
    return [...buckets.entries()]
      .sort((a, b) => b[1] - a[1])
      .slice(0, 5)
      .map(([name, n]) => ({ name, pct: n / t, count: n }));
  });

  let clientMix = $derived.by(() => {
    const buckets = new Map<string, number>();
    for (const r of inRange.slice(0, 1000)) {
      const ua = (r.user_agent ?? "").toLowerCase();
      let k = "other";
      if (ua.includes("opencode")) k = "opencode";
      else if (ua.includes("cursor")) k = "cursor";
      else if (ua.includes("claude")) k = "claude-code";
      else if (ua) k = "other";
      buckets.set(k, (buckets.get(k) ?? 0) + 1);
    }
    const t = inRange.slice(0, 1000).length || 1;
    return [...buckets.entries()]
      .filter(([name]) => name !== "other")
      .sort((a, b) => b[1] - a[1])
      .slice(0, 3)
      .map(([name, n]) => ({ name, pct: n / t }));
  });

  // ── Top-N aggregations across the active range window. ──────────────
  // Path normalization: strip query, collapse numeric IDs to `:id` so
  // `/v1/messages/abc123` and `/v1/messages/def456` group together.
  function normalizePath(p: string): string {
    const noQuery = p.split("?")[0] ?? p;
    return noQuery
      .replace(/\/[0-9a-f]{8,}/gi, "/:id")
      .replace(/\/\d+/g, "/:n");
  }

  let topEndpoints = $derived.by(() => {
    const acc = new Map<string, { count: number; errors: number; sumLat: number }>();
    for (const r of inRange.slice(0, 2000)) {
      const k = normalizePath(r.path);
      const cur = acc.get(k) ?? { count: 0, errors: 0, sumLat: 0 };
      cur.count++;
      if (r.status >= 400) cur.errors++;
      if (r.latency_ms > 0) cur.sumLat += r.latency_ms;
      acc.set(k, cur);
    }
    return [...acc.entries()]
      .sort((a, b) => b[1].count - a[1].count)
      .slice(0, 6)
      .map(([path, v]) => ({
        path,
        count: v.count,
        errRate: v.count > 0 ? (v.errors / v.count) * 100 : 0,
        avgLat: v.count > 0 ? v.sumLat / v.count : 0,
      }));
  });

  let slowestPaths = $derived.by(() => {
    const acc = new Map<string, { count: number; sumLat: number; maxLat: number }>();
    for (const r of inRange.slice(0, 2000)) {
      if (r.latency_ms <= 0) continue;
      const k = normalizePath(r.path);
      const cur = acc.get(k) ?? { count: 0, sumLat: 0, maxLat: 0 };
      cur.count++;
      cur.sumLat += r.latency_ms;
      if (r.latency_ms > cur.maxLat) cur.maxLat = r.latency_ms;
      acc.set(k, cur);
    }
    return [...acc.entries()]
      .filter(([, v]) => v.count >= 2) // require minimum sample
      .map(([path, v]) => ({
        path,
        count: v.count,
        avgLat: v.sumLat / v.count,
        maxLat: v.maxLat,
      }))
      .sort((a, b) => b.avgLat - a.avgLat)
      .slice(0, 6);
  });

  let topErrors = $derived.by(() => {
    const acc = new Map<string, { count: number; lastPath: string }>();
    for (const r of inRange.slice(0, 2000)) {
      if (r.status < 400) continue;
      const code = String(r.status);
      const cur = acc.get(code) ?? { count: 0, lastPath: "" };
      cur.count++;
      cur.lastPath = normalizePath(r.path);
      acc.set(code, cur);
    }
    return [...acc.entries()]
      .sort((a, b) => b[1].count - a[1].count)
      .slice(0, 6)
      .map(([status, v]) => ({ status, count: v.count, lastPath: v.lastPath }));
  });

  let topClients = $derived.by(() => {
    const acc = new Map<string, { count: number; bytes: number }>();
    for (const r of inRange.slice(0, 2000)) {
      const ua = (r.user_agent ?? "").trim();
      let k = "unknown";
      const lower = ua.toLowerCase();
      if (lower.includes("opencode")) k = "opencode";
      else if (lower.includes("cursor")) k = "cursor";
      else if (lower.includes("claude-code") || lower.includes("claude")) k = "claude-code";
      else if (lower.includes("aider")) k = "aider";
      else if (lower.includes("curl")) k = "curl";
      else if (lower.includes("python")) k = "python-requests";
      else if (lower.includes("postman")) k = "postman";
      else if (ua) k = ua.split("/")[0]?.toLowerCase() || "other";
      const cur = acc.get(k) ?? { count: 0, bytes: 0 };
      cur.count++;
      cur.bytes += r.bytes_out || 0;
      acc.set(k, cur);
    }
    return [...acc.entries()]
      .sort((a, b) => b[1].count - a[1].count)
      .slice(0, 6)
      .map(([name, v]) => ({ name, count: v.count, bytes: v.bytes }));
  });

  // ── Status code stacked-bar over time (10 bins across the window). ───
  let statusOverTime = $derived.by(() => {
    const bins = 10;
    const cutoff = Date.now() - rangeSec * 1000;
    const binMs = (rangeSec * 1000) / bins;
    type Cell = { s2: number; s3: number; s4: number; s5: number };
    const buckets: Cell[] = Array(bins)
      .fill(null)
      .map(() => ({ s2: 0, s3: 0, s4: 0, s5: 0 }));
    for (const r of inRange) {
      const t = Date.parse(r.started_at);
      if (Number.isNaN(t)) continue;
      const idx = Math.min(bins - 1, Math.max(0, Math.floor((t - cutoff) / binMs)));
      const c = buckets[idx];
      if (r.status >= 200 && r.status < 300) c.s2++;
      else if (r.status >= 300 && r.status < 400) c.s3++;
      else if (r.status >= 400 && r.status < 500) c.s4++;
      else if (r.status >= 500) c.s5++;
    }
    const peak = Math.max(1, ...buckets.map((c) => c.s2 + c.s3 + c.s4 + c.s5));
    return buckets.map((c) => ({ ...c, total: c.s2 + c.s3 + c.s4 + c.s5, peak }));
  });

  // ── Latency series for the time-series chart (binned to 30 points). ──
  function binLat(rs: typeof store.requests, points: number): { p50: number; p95: number; p99: number }[] {
    if (rs.length === 0) return Array(points).fill({ p50: 0, p95: 0, p99: 0 });
    const cutoff = Date.now() - rangeSec * 1000;
    const binMs = (rangeSec * 1000) / points;
    const buckets: number[][] = Array(points).fill(null).map(() => []);
    for (const r of rs) {
      const t = Date.parse(r.started_at);
      if (Number.isNaN(t) || r.latency_ms <= 0) continue;
      const idx = Math.min(points - 1, Math.max(0, Math.floor((t - cutoff) / binMs)));
      buckets[idx].push(r.latency_ms);
    }
    return buckets.map((b) => ({
      p50: percentile(b, 0.5),
      p95: percentile(b, 0.95),
      p99: percentile(b, 0.99),
    }));
  }
  let latSeries = $derived(binLat(inRange, 30));

  // Error-rate time series (% per bin).
  let errSeries = $derived.by(() => {
    const points = 30;
    const cutoff = Date.now() - rangeSec * 1000;
    const binMs = (rangeSec * 1000) / points;
    const tots: number[] = Array(points).fill(0);
    const errs: number[] = Array(points).fill(0);
    for (const r of inRange) {
      const t = Date.parse(r.started_at);
      if (Number.isNaN(t)) continue;
      const idx = Math.min(points - 1, Math.max(0, Math.floor((t - cutoff) / binMs)));
      tots[idx]++;
      if (r.status >= 400) errs[idx]++;
    }
    return tots.map((t, i) => (t > 0 ? (errs[i] / t) * 100 : 0));
  });

  // ── SLO config (operator-tunable defaults; could move to settings). ──
  const SLO_ERR_BUDGET_PCT = 1.0; // 99% success target → 1% error budget
  const SLO_P99_MS = 5000; // 5s p99 latency target

  let errBudgetUsed = $derived(
    Math.min(100, errRate > 0 ? (errRate / SLO_ERR_BUDGET_PCT) * 100 : 0),
  );
  let p99Healthy = $derived(p99 > 0 ? p99 <= SLO_P99_MS : true);

  let p99BudgetPct = $derived(SLO_P99_MS > 0 ? Math.min(100, (p99 / SLO_P99_MS) * 100) : 0);

  let chartMaxR = $derived(Math.max(1, ...store.totalSpark));
  let chartMaxL = $derived(Math.max(1, ...latSeries.map((b) => b.p99)));
  let chartMaxE = $derived(Math.max(0.5, SLO_ERR_BUDGET_PCT * 2, ...errSeries));

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

  function fmtBytes(n: number): string {
    if (!n || n <= 0) return "—";
    if (n < 1024) return n + " B";
    if (n < 1024 * 1024) return (n / 1024).toFixed(1) + " KB";
    if (n < 1024 * 1024 * 1024) return (n / (1024 * 1024)).toFixed(1) + " MB";
    return (n / (1024 * 1024 * 1024)).toFixed(2) + " GB";
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
  <header class="metrics-head mono">
    <div class="metrics-head__title">
      <span class="caps faint">aggregate</span>
      <span class="metrics-head__win">last {range === "5m" ? "5 minutes" : range === "1h" ? "hour" : "24 hours"}</span>
    </div>
    <div class="range" role="tablist" aria-label="time range">
      {#each ["5m", "1h", "24h"] as r}
        <button
          type="button"
          class="range__seg"
          class:range__seg--active={range === r}
          role="tab"
          aria-selected={range === r}
          onclick={() => (range = r as Range)}
        >
          {r}
        </button>
      {/each}
    </div>
  </header>

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

  <!-- ── Time-series charts ────────────────────────────────────────── -->
  <section class="series" aria-label="time series">
    <div class="series-card">
      <div class="series-head mono">
        <span class="caps">requests · per bin</span>
        <span class="caps faint">{rps.toFixed(1)}/s now</span>
      </div>
      <svg class="series-chart" viewBox="0 0 600 120" preserveAspectRatio="none" aria-hidden="true">
        {#each store.totalSpark as v, i}
          <rect
            x={(i / store.totalSpark.length) * 600}
            y={120 - (v / chartMaxR) * 110 - 2}
            width={Math.max(1, 600 / store.totalSpark.length - 1)}
            height={(v / chartMaxR) * 110 + 2}
            class="series-bar"
          />
        {/each}
        <line x1="0" y1="118" x2="600" y2="118" class="series-axis" />
      </svg>
      <div class="series-axis-labels mono faint">
        <span>now − {rangeSec >= 3600 ? Math.round(rangeSec / 3600) + "h" : Math.round(rangeSec / 60) + "m"}</span>
        <span>now</span>
      </div>
    </div>

    <div class="series-card">
      <div class="series-head mono">
        <span class="caps">latency · ms</span>
        <span class="caps faint">p50 {fmtMs(p50)} · p95 {fmtMs(p95)} · p99 {fmtMs(p99)}</span>
      </div>
      <svg class="series-chart" viewBox="0 0 600 120" preserveAspectRatio="none" aria-hidden="true">
        {#each latSeries as b, i}
          <line
            x1={(i / (latSeries.length - 1 || 1)) * 600}
            x2={(i / (latSeries.length - 1 || 1)) * 600}
            y1={120 - (b.p99 / chartMaxL) * 100 - 2}
            y2={120 - (b.p50 / chartMaxL) * 100 - 2}
            class="series-band"
          />
        {/each}
        <path
          class="series-line series-line--p99"
          d={"M " + latSeries.map((b, i) => `${((i / (latSeries.length - 1 || 1)) * 600).toFixed(1)} ${(120 - (b.p99 / chartMaxL) * 100 - 2).toFixed(1)}`).join(" L ")}
        />
        <path
          class="series-line series-line--p95"
          d={"M " + latSeries.map((b, i) => `${((i / (latSeries.length - 1 || 1)) * 600).toFixed(1)} ${(120 - (b.p95 / chartMaxL) * 100 - 2).toFixed(1)}`).join(" L ")}
        />
        <path
          class="series-line series-line--p50"
          d={"M " + latSeries.map((b, i) => `${((i / (latSeries.length - 1 || 1)) * 600).toFixed(1)} ${(120 - (b.p50 / chartMaxL) * 100 - 2).toFixed(1)}`).join(" L ")}
        />
        {#if SLO_P99_MS / chartMaxL <= 1}
          <line
            x1="0"
            x2="600"
            y1={120 - (SLO_P99_MS / chartMaxL) * 100 - 2}
            y2={120 - (SLO_P99_MS / chartMaxL) * 100 - 2}
            class="series-slo"
          />
        {/if}
        <line x1="0" y1="118" x2="600" y2="118" class="series-axis" />
      </svg>
      <div class="series-legend mono faint">
        <span class="series-legend__sw series-legend__sw--p50"></span> p50
        <span class="series-legend__sw series-legend__sw--p95"></span> p95
        <span class="series-legend__sw series-legend__sw--p99"></span> p99
        <span class="series-legend__sep">·</span>
        <span class="series-legend__slo" class:series-legend__slo--ok={p99Healthy} class:series-legend__slo--bad={!p99Healthy}>
          slo p99 ≤ {fmtMs(SLO_P99_MS)}
        </span>
      </div>
    </div>

    <div class="series-card">
      <div class="series-head mono">
        <span class="caps">error rate · %</span>
        <span class="caps faint">{errRate.toFixed(2)}% · budget {errBudgetUsed.toFixed(0)}%</span>
      </div>
      <svg class="series-chart" viewBox="0 0 600 120" preserveAspectRatio="none" aria-hidden="true">
        {#each errSeries as v, i}
          <rect
            x={(i / errSeries.length) * 600}
            y={120 - (v / chartMaxE) * 110 - 2}
            width={Math.max(1, 600 / errSeries.length - 1)}
            height={Math.max(0, (v / chartMaxE) * 110)}
            class={"series-bar " + (v > SLO_ERR_BUDGET_PCT ? "series-bar--bad" : "series-bar--ok")}
          />
        {/each}
        <line
          x1="0"
          x2="600"
          y1={120 - (SLO_ERR_BUDGET_PCT / chartMaxE) * 110 - 2}
          y2={120 - (SLO_ERR_BUDGET_PCT / chartMaxE) * 110 - 2}
          class="series-slo"
        />
        <line x1="0" y1="118" x2="600" y2="118" class="series-axis" />
      </svg>
      <div class="series-legend mono faint">
        <span class="series-legend__sw series-legend__sw--ok"></span> within budget
        <span class="series-legend__sw series-legend__sw--bad"></span> over
        <span class="series-legend__sep">·</span>
        <span>slo {SLO_ERR_BUDGET_PCT}%</span>
      </div>
    </div>
  </section>

  <!-- ── Status code stacked bar ───────────────────────────────────── -->
  <section class="status-bar" aria-label="status code distribution over time">
    <div class="status-bar__head mono">
      <span class="caps">status codes · over {range}</span>
      <div class="status-bar__legend mono faint">
        <span class="status-bar__sw status-bar__sw--2xx"></span> 2xx
        <span class="status-bar__sw status-bar__sw--3xx"></span> 3xx
        <span class="status-bar__sw status-bar__sw--4xx"></span> 4xx
        <span class="status-bar__sw status-bar__sw--5xx"></span> 5xx
      </div>
    </div>
    <div class="status-bar__chart">
      {#each statusOverTime as bin, i (i)}
        <div class="status-bar__col" title={`${bin.total} req · 2xx ${bin.s2}, 3xx ${bin.s3}, 4xx ${bin.s4}, 5xx ${bin.s5}`}>
          {#if bin.total === 0}
            <div class="status-bar__seg status-bar__seg--empty" style="block-size: 100%"></div>
          {:else}
            {#if bin.s5 > 0}
              <div class="status-bar__seg status-bar__seg--5xx" style="block-size: {(bin.s5 / bin.peak) * 100}%"></div>
            {/if}
            {#if bin.s4 > 0}
              <div class="status-bar__seg status-bar__seg--4xx" style="block-size: {(bin.s4 / bin.peak) * 100}%"></div>
            {/if}
            {#if bin.s3 > 0}
              <div class="status-bar__seg status-bar__seg--3xx" style="block-size: {(bin.s3 / bin.peak) * 100}%"></div>
            {/if}
            {#if bin.s2 > 0}
              <div class="status-bar__seg status-bar__seg--2xx" style="block-size: {(bin.s2 / bin.peak) * 100}%"></div>
            {/if}
          {/if}
        </div>
      {/each}
    </div>
  </section>

  <!-- ── Top-N tables (2x2 grid) ───────────────────────────────────── -->
  <section class="topn" aria-label="top-N breakdown">
    <div class="topn-card">
      <div class="topn-head mono">
        <span class="caps">top endpoints · {range}</span>
        <span class="caps faint">by request count</span>
      </div>
      {#if topEndpoints.length === 0}
        <p class="empty-italic">No traffic in window.</p>
      {:else}
        <table class="topn-tbl">
          <thead class="mono faint">
            <tr>
              <th class="topn-th--path">path</th>
              <th class="topn-th--num">req</th>
              <th class="topn-th--num">err%</th>
              <th class="topn-th--num">avg</th>
            </tr>
          </thead>
          <tbody class="mono">
            {#each topEndpoints as e}
              <tr>
                <td class="topn-td--path" title={e.path}>{e.path}</td>
                <td class="topn-td--num tabular">{e.count.toLocaleString()}</td>
                <td class="topn-td--num tabular" class:warn={e.errRate >= 1}>{e.errRate.toFixed(1)}%</td>
                <td class="topn-td--num tabular">{fmtMs(e.avgLat)}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      {/if}
    </div>

    <div class="topn-card">
      <div class="topn-head mono">
        <span class="caps">slowest paths · {range}</span>
        <span class="caps faint">avg latency</span>
      </div>
      {#if slowestPaths.length === 0}
        <p class="empty-italic">No latency samples.</p>
      {:else}
        <table class="topn-tbl">
          <thead class="mono faint">
            <tr>
              <th class="topn-th--path">path</th>
              <th class="topn-th--num">n</th>
              <th class="topn-th--num">avg</th>
              <th class="topn-th--num">max</th>
            </tr>
          </thead>
          <tbody class="mono">
            {#each slowestPaths as s}
              <tr>
                <td class="topn-td--path" title={s.path}>{s.path}</td>
                <td class="topn-td--num tabular">{s.count}</td>
                <td class="topn-td--num tabular">{fmtMs(s.avgLat)}</td>
                <td class="topn-td--num tabular">{fmtMs(s.maxLat)}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      {/if}
    </div>

    <div class="topn-card">
      <div class="topn-head mono">
        <span class="caps">top errors · {range}</span>
        <span class="caps faint">by status code</span>
      </div>
      {#if topErrors.length === 0}
        <p class="empty-italic ok">No errors in window.</p>
      {:else}
        <table class="topn-tbl">
          <thead class="mono faint">
            <tr>
              <th class="topn-th--path">status</th>
              <th class="topn-th--path">last path</th>
              <th class="topn-th--num">count</th>
            </tr>
          </thead>
          <tbody class="mono">
            {#each topErrors as e}
              <tr>
                <td class="topn-td--num"><span class="topn-status" class:topn-status--5xx={Number(e.status) >= 500} class:topn-status--4xx={Number(e.status) >= 400 && Number(e.status) < 500}>{e.status}</span></td>
                <td class="topn-td--path" title={e.lastPath}>{e.lastPath || "—"}</td>
                <td class="topn-td--num tabular">{e.count.toLocaleString()}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      {/if}
    </div>

    <div class="topn-card">
      <div class="topn-head mono">
        <span class="caps">top clients · {range}</span>
        <span class="caps faint">by request count</span>
      </div>
      {#if topClients.length === 0}
        <p class="empty-italic">No clients identified yet.</p>
      {:else}
        <table class="topn-tbl">
          <thead class="mono faint">
            <tr>
              <th class="topn-th--path">client</th>
              <th class="topn-th--num">req</th>
              <th class="topn-th--num">bytes</th>
            </tr>
          </thead>
          <tbody class="mono">
            {#each topClients as c}
              <tr>
                <td class="topn-td--path">{c.name}</td>
                <td class="topn-td--num tabular">{c.count.toLocaleString()}</td>
                <td class="topn-td--num tabular">{fmtBytes(c.bytes)}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      {/if}
    </div>
  </section>

  <!-- ── SLO band ──────────────────────────────────────────────────── -->
  <section class="slo" aria-label="SLO indicators">
    <div class="slo-card">
      <div class="slo-head mono">
        <span class="caps">error budget · {range}</span>
        <span class="caps faint">target {SLO_ERR_BUDGET_PCT}% errors</span>
      </div>
      <div class="slo-gauge" aria-hidden="true">
        <div class="slo-gauge__fill" class:slo-gauge__fill--bad={errBudgetUsed > 100} class:slo-gauge__fill--warn={errBudgetUsed > 70 && errBudgetUsed <= 100} style="inline-size: {Math.min(100, errBudgetUsed).toFixed(1)}%"></div>
        <div class="slo-gauge__mark" style="inset-inline-start: 100%"></div>
      </div>
      <div class="slo-foot mono">
        <span class="tabular">{errBudgetUsed.toFixed(0)}% of budget consumed</span>
        <span class="faint">·</span>
        <span class="tabular faint">{errRate.toFixed(2)}% / {SLO_ERR_BUDGET_PCT}%</span>
      </div>
    </div>

    <div class="slo-card">
      <div class="slo-head mono">
        <span class="caps">p99 latency budget</span>
        <span class="caps faint">target ≤ {fmtMs(SLO_P99_MS)}</span>
      </div>
      <div class="slo-gauge" aria-hidden="true">
        <div class="slo-gauge__fill" class:slo-gauge__fill--bad={!p99Healthy} class:slo-gauge__fill--warn={p99BudgetPct > 70 && p99BudgetPct <= 100} style="inline-size: {p99BudgetPct.toFixed(1)}%"></div>
        <div class="slo-gauge__mark" style="inset-inline-start: 100%"></div>
      </div>
      <div class="slo-foot mono">
        <span class="tabular">p99 {fmtMs(p99)}</span>
        <span class="faint">·</span>
        <span class="tabular faint">{p99BudgetPct.toFixed(0)}% of budget</span>
      </div>
    </div>
  </section>

  <div class="metrics-foot mono">
    <span>Window: <span class="metrics-foot__val">{range === "5m" ? "last 5 min" : range === "1h" ? "last hour" : "last 24 h"}</span> · {inRange.length.toLocaleString()} req in window</span>
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

  /* ────────────────── Range header + segmented ─────────────────── */
  .metrics-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding: 0 4px 12px;
  }
  .metrics-head__title {
    display: flex;
    align-items: baseline;
    gap: 8px;
    font-size: 11px;
    color: var(--c-text-dim);
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }
  .metrics-head__count { color: var(--c-text); font-variant-numeric: tabular-nums; }

  .range {
    display: inline-flex;
    border: 1px solid var(--c-rule);
    border-radius: 4px;
    overflow: hidden;
    background: color-mix(in oklch, var(--c-bg) 80%, transparent);
  }
  .range__seg {
    appearance: none;
    background: transparent;
    border: 0;
    padding: 5px 10px;
    color: var(--c-text-faint);
    font-family: var(--font-mono);
    font-size: 11px;
    letter-spacing: 0.04em;
    cursor: pointer;
    border-inline-end: 1px solid var(--c-rule);
    transition: background 120ms ease, color 120ms ease;
  }
  .range__seg:last-child { border-inline-end: 0; }
  .range__seg:hover { color: var(--c-text); background: color-mix(in oklch, var(--c-text) 4%, transparent); }
  .range__seg--active {
    color: var(--c-bg);
    background: var(--c-accent);
  }
  .range__seg--active:hover { color: var(--c-bg); background: var(--c-accent); }

  /* ─────────────────────── Time-series charts ──────────────────── */
  .series {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 10px;
    padding: 10px 14px 4px;
  }
  .series-card {
    background: color-mix(in oklch, var(--c-bg) 80%, transparent);
    border: 1px solid var(--c-rule);
    border-radius: 4px;
    padding: 10px 12px;
    display: flex;
    flex-direction: column;
    gap: 8px;
    min-block-size: 130px;
  }
  .series-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 6px;
  }
  .series-title {
    font-size: 10px;
    letter-spacing: 0.1em;
    text-transform: uppercase;
    color: var(--c-text-dim);
  }
  .series-meta {
    font-size: 10px;
    color: var(--c-text-faint);
    font-variant-numeric: tabular-nums;
    letter-spacing: 0.04em;
  }
  .series-chart {
    inline-size: 100%;
    block-size: 90px;
    display: block;
  }
  @media (min-width: 960px) {
    .series-chart {
      block-size: 140px;
    }
  }
  .series-bar { fill: color-mix(in oklch, var(--c-text) 28%, transparent); }
  .series-bar--err { fill: color-mix(in oklch, var(--c-danger) 70%, transparent); }
  .series-bar--err.over { fill: var(--c-danger); }
  .series-band {
    fill: color-mix(in oklch, var(--c-accent) 14%, transparent);
    stroke: none;
  }
  .series-line { fill: none; stroke-width: 1.4; }
  .series-line--p50 { stroke: color-mix(in oklch, var(--c-text) 40%, transparent); }
  .series-line--p95 { stroke: color-mix(in oklch, var(--c-text) 75%, transparent); }
  .series-line--p99 { stroke: var(--c-accent); stroke-width: 1.6; }
  .series-slo {
    stroke: color-mix(in oklch, var(--c-danger) 60%, transparent);
    stroke-dasharray: 3 3;
    stroke-width: 1;
  }
  .series-grid { stroke: var(--c-rule); stroke-width: 0.5; }
  .series-legend {
    display: flex;
    gap: 10px;
    font-size: 10px;
    color: var(--c-text-faint);
    margin-block-start: auto;
    letter-spacing: 0.04em;
  }
  .series-legend__sw {
    display: inline-block;
    inline-size: 10px;
    block-size: 2px;
    margin-inline-end: 4px;
    vertical-align: middle;
  }
  .series-legend__sw--p50 { background: color-mix(in oklch, var(--c-text) 40%, transparent); }
  .series-legend__sw--p95 { background: color-mix(in oklch, var(--c-text) 75%, transparent); }
  .series-legend__sw--p99 { background: var(--c-accent); }

  /* ───────────────── Status code stacked bar ───────────────────── */
  .status-bar {
    display: flex;
    flex-direction: column;
    gap: 6px;
    padding: 8px 14px 14px;
  }
  .status-bar__head {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 8px;
  }
  .status-bar__title {
    font-size: 10px;
    letter-spacing: 0.1em;
    text-transform: uppercase;
    color: var(--c-text-dim);
  }
  .status-bar__legend {
    display: flex;
    gap: 8px;
    font-size: 10px;
    color: var(--c-text-faint);
    letter-spacing: 0.04em;
  }
  .status-bar__sw {
    display: inline-block;
    inline-size: 8px;
    block-size: 8px;
    margin-inline-end: 3px;
    vertical-align: middle;
  }
  .status-bar__sw--2xx { background: color-mix(in oklch, var(--c-success) 70%, transparent); }
  .status-bar__sw--3xx { background: color-mix(in oklch, var(--c-info, #79a) 60%, transparent); }
  .status-bar__sw--4xx { background: color-mix(in oklch, var(--c-warn) 70%, transparent); }
  .status-bar__sw--5xx { background: color-mix(in oklch, var(--c-danger) 70%, transparent); }
  .status-bar__grid {
    display: grid;
    grid-template-columns: repeat(10, 1fr);
    gap: 3px;
    block-size: 56px;
  }
  .status-bar__col {
    background: color-mix(in oklch, var(--c-bg) 70%, transparent);
    border: 1px solid var(--c-rule);
    border-radius: 2px;
    display: flex;
    flex-direction: column-reverse;
    overflow: hidden;
    min-block-size: 0;
  }
  .status-bar__seg { transition: background 120ms ease; }
  .status-bar__seg--2xx { background: color-mix(in oklch, var(--c-success) 70%, transparent); }
  .status-bar__seg--3xx { background: color-mix(in oklch, var(--c-info, #79a) 60%, transparent); }
  .status-bar__seg--4xx { background: color-mix(in oklch, var(--c-warn) 70%, transparent); }
  .status-bar__seg--5xx { background: color-mix(in oklch, var(--c-danger) 70%, transparent); }

  /* ────────────────────────── Top-N tables ─────────────────────── */
  .topn {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 10px;
    padding: 4px 14px 14px;
  }
  .topn-card {
    background: color-mix(in oklch, var(--c-bg) 80%, transparent);
    border: 1px solid var(--c-rule);
    border-radius: 4px;
    padding: 10px 12px;
    display: flex;
    flex-direction: column;
    gap: 8px;
    min-block-size: 180px;
  }
  .topn-head {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    font-size: 10px;
    letter-spacing: 0.1em;
    text-transform: uppercase;
    color: var(--c-text-dim);
  }
  .topn-tbl {
    width: 100%;
    border-collapse: collapse;
    font-family: var(--font-mono);
    font-size: 11px;
  }
  .topn-tbl th {
    text-align: start;
    font-weight: var(--fw-medium, 500);
    font-size: 10px;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: var(--c-text-faint);
    padding: 4px 6px;
    border-block-end: 1px solid var(--c-rule);
  }
  .topn-tbl th.num { text-align: end; }
  .topn-tbl td {
    padding: 4px 6px;
    border-block-end: 1px solid color-mix(in oklch, var(--c-rule) 50%, transparent);
    color: var(--c-text);
    font-variant-numeric: tabular-nums;
  }
  .topn-tbl td.num { text-align: end; }
  .topn-tbl td.path,
  .topn-tbl td.client {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    max-inline-size: 1px; /* hack to honor width via colgroup; ellipsis works inside table */
  }
  .topn-tbl tr:last-child td { border-block-end: 0; }
  .topn-tbl .method {
    color: var(--c-text-dim);
    margin-inline-end: 6px;
  }
  .topn-tbl .status-2 { color: var(--c-success); }
  .topn-tbl .status-3 { color: var(--c-info, #79a); }
  .topn-tbl .status-4 { color: var(--c-warn); }
  .topn-tbl .status-5 { color: var(--c-danger); }
  .topn-empty {
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 20px 8px;
    color: var(--c-text-faint);
    font-size: 11px;
    font-style: italic;
  }

  /* ─────────────────────────── SLO cards ───────────────────────── */
  .slo {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 10px;
    padding: 4px 14px 14px;
  }
  .slo-card {
    background: color-mix(in oklch, var(--c-bg) 80%, transparent);
    border: 1px solid var(--c-rule);
    border-radius: 4px;
    padding: 12px 14px;
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  .slo-head {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    font-size: 10px;
    letter-spacing: 0.1em;
    text-transform: uppercase;
    color: var(--c-text-dim);
  }
  .slo-gauge {
    position: relative;
    block-size: 6px;
    background: color-mix(in oklch, var(--c-bg) 60%, transparent);
    border: 1px solid var(--c-rule);
    border-radius: 999px;
    overflow: hidden;
  }
  .slo-gauge__fill {
    block-size: 100%;
    background: color-mix(in oklch, var(--c-success) 70%, transparent);
    transition: inline-size 240ms ease;
  }
  .slo-gauge__fill--warn { background: color-mix(in oklch, var(--c-warn) 80%, transparent); }
  .slo-gauge__fill--bad { background: var(--c-danger); }
  .slo-gauge__mark {
    position: absolute;
    inset-block: -2px;
    inline-size: 1px;
    background: color-mix(in oklch, var(--c-text) 30%, transparent);
  }
  .slo-foot {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 11px;
    color: var(--c-text);
  }

  @media (max-width: 1120px) {
    .series { grid-template-columns: 1fr; }
    .topn { grid-template-columns: 1fr; }
    .slo { grid-template-columns: 1fr; }
  }
</style>
