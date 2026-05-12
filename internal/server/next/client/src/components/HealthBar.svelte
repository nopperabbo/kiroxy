<!--
  HealthBar — top strip. Version, uptime (ticking), total req, error rate.
  Uses an inline sparkline SVG that tracks error rate over the last N
  snapshots in-memory (not persisted).

  Design:
  - Monospace tabular-nums so digits don't jitter.
  - Uptime ticks client-side: we take snapshot.uptime_s as baseline when a
    new snapshot lands, then rAF-increment locally. Prevents stutter between
    2s server snapshots.
-->
<script lang="ts">
  import { onMount } from "svelte";
  import { store } from "../lib/stores.svelte";
  import { formatUptime, formatCount, formatPercent } from "../lib/format";

  const MAX_SAMPLES = 40;

  let samples = $state<number[]>([]);
  let uptimeDisplay = $state<number>(0);

  // Snapshot-change effect: seed baseline and push a sample.
  let lastSnapshotAt = 0;
  let baselineUptime = 0;

  $effect(() => {
    const s = store.snapshot;
    if (store.lastUpdated !== lastSnapshotAt) {
      lastSnapshotAt = store.lastUpdated;
      baselineUptime = s.uptime_s;
      uptimeDisplay = s.uptime_s;
      samples = [...samples.slice(-MAX_SAMPLES + 1), s.error_rate];
    }
  });

  // Local uptime tick — rAF for smoothness, but only once per second.
  onMount(() => {
    let lastSec = 0;
    let raf = 0;
    const loop = (t: number): void => {
      if (lastSnapshotAt > 0) {
        const sec = Math.floor((t - lastSec) / 1000);
        if (sec >= 1) {
          const elapsed = Math.floor((Date.now() - lastSnapshotAt) / 1000);
          uptimeDisplay = baselineUptime + elapsed;
          lastSec = t;
        }
      }
      raf = requestAnimationFrame(loop);
    };
    raf = requestAnimationFrame(loop);
    return () => cancelAnimationFrame(raf);
  });

  const sparkPath = $derived(buildSparkPath(samples));

  function buildSparkPath(data: number[]): string {
    if (data.length < 2) return "";
    const W = 80;
    const H = 16;
    const max = Math.max(0.001, ...data);
    const step = W / (data.length - 1);
    return data
      .map((v, i) => {
        const x = i * step;
        const y = H - (v / max) * H;
        return `${i === 0 ? "M" : "L"}${x.toFixed(1)},${y.toFixed(1)}`;
      })
      .join(" ");
  }

  const statusText = $derived(
    store.sseStatus === "open"
      ? "live"
      : store.sseStatus === "polling"
        ? "polling"
        : store.sseStatus === "reconnecting"
          ? "reconnecting"
          : store.sseStatus === "closed"
            ? "offline"
            : "connecting",
  );
</script>

<header class="health" role="banner">
  <div class="health__brand" aria-label="kiroxy dashboard next">
    <span class="health__brand-k">K</span>
    <span class="health__brand-name">kiroxy</span>
    <span class="health__brand-variant">next</span>
  </div>

  <div class="health__stats" aria-live="polite">
    <span class="stat">
      <span class="stat__label">v</span>
      <span class="stat__val mono">{store.snapshot.version || "—"}</span>
    </span>
    <span class="stat">
      <span class="stat__label">uptime</span>
      <span class="stat__val mono tnum">{formatUptime(uptimeDisplay)}</span>
    </span>
    <span class="stat">
      <span class="stat__label">req</span>
      <span class="stat__val mono tnum">
        {formatCount(store.snapshot.total_requests)}
      </span>
    </span>
    <span class="stat">
      <span class="stat__label">err</span>
      <span
        class="stat__val mono tnum"
        class:stat__val--warn={store.snapshot.error_rate >= 0.02 &&
          store.snapshot.error_rate < 0.1}
        class:stat__val--danger={store.snapshot.error_rate >= 0.1}
      >
        {formatPercent(store.snapshot.error_rate)}
      </span>
      <svg
        class="spark"
        viewBox="0 0 80 16"
        width="80"
        height="16"
        aria-hidden="true"
      >
        {#if sparkPath}
          <path d={sparkPath} stroke="currentColor" stroke-width="1" fill="none" />
        {/if}
      </svg>
    </span>
    <span class="stat">
      <span class="stat__label">vault</span>
      <span
        class="stat__val mono"
        class:stat__val--danger={!store.snapshot.vault_ok &&
          store.snapshot.version !== ""}
      >
        {store.snapshot.vault_ok ? "ok" : "—"}
      </span>
    </span>
    <span class="stat stat--status">
      <span class="stat__dot stat__dot--{store.sseStatus}" aria-hidden="true"
      ></span>
      <span class="stat__val">{statusText}</span>
    </span>
  </div>
</header>

<style>
  .health {
    display: flex;
    align-items: center;
    gap: var(--sp-6);
    padding: var(--sp-4) var(--sp-6);
    border-block-end: 1px solid var(--c-border);
    background: var(--c-surface);
    position: sticky;
    inset-block-start: 0;
    z-index: var(--z-sticky);
  }

  .health__brand {
    display: inline-flex;
    align-items: center;
    gap: var(--sp-3);
    font-weight: var(--fw-semibold);
    letter-spacing: 0.02em;
  }
  .health__brand-k {
    display: inline-grid;
    place-items: center;
    inline-size: 22px;
    block-size: 22px;
    border-radius: var(--r-sm);
    background: var(--c-accent);
    color: light-dark(oklch(98% 0 0), oklch(10% 0 0));
    font-family: var(--font-mono);
    font-weight: var(--fw-bold);
    font-size: var(--fs-sm);
  }
  .health__brand-name {
    color: var(--c-text);
  }
  .health__brand-variant {
    color: var(--c-text-faint);
    font-family: var(--font-mono);
    font-size: var(--fs-xs);
    padding: 1px 5px;
    border-radius: var(--r-sm);
    border: 1px solid var(--c-border);
  }

  .health__stats {
    display: flex;
    align-items: center;
    gap: var(--sp-6);
    margin-inline-start: auto;
    flex-wrap: wrap;
  }

  .stat {
    display: inline-flex;
    align-items: center;
    gap: var(--sp-3);
    font-size: var(--fs-sm);
  }
  .stat__label {
    color: var(--c-text-faint);
    text-transform: uppercase;
    font-size: var(--fs-xs);
    letter-spacing: 0.08em;
  }
  .stat__val {
    color: var(--c-text);
    font-weight: var(--fw-medium);
  }
  .stat__val--warn {
    color: var(--c-warn);
  }
  .stat__val--danger {
    color: var(--c-danger);
  }

  .spark {
    color: var(--c-accent);
    opacity: 0.6;
  }

  .stat--status {
    padding: 2px var(--sp-3);
    border-radius: var(--r-sm);
    border: 1px solid var(--c-border);
    background: var(--c-surface-2);
  }
  .stat__dot {
    inline-size: 6px;
    block-size: 6px;
    border-radius: 50%;
    background: var(--c-text-faint);
  }
  .stat__dot--open {
    background: var(--c-success);
  }
  .stat__dot--reconnecting,
  .stat__dot--connecting {
    background: var(--c-warn);
    animation: pulse 1.6s ease-in-out infinite;
  }
  .stat__dot--polling {
    background: var(--c-accent);
  }
  .stat__dot--closed {
    background: var(--c-danger);
  }

  @keyframes pulse {
    0%,
    100% {
      opacity: 1;
    }
    50% {
      opacity: 0.45;
    }
  }

  @container (inline-size < 620px) {
    .health {
      flex-wrap: wrap;
    }
    .health__stats {
      margin-inline-start: 0;
      inline-size: 100%;
    }
  }
</style>
