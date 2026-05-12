<!--
  CountdownRing — signature feature 1. A hand-rolled SVG ring that
  visualizes time-to-expiry or time-to-refresh. Color interpolates from
  accent→warn→danger as expiry approaches.

  Input: `expiresAt` ISO string, optional `ttlSeconds` for the full
  window. If ttlSeconds is omitted we assume a 1-hour token and show the
  last-hour fraction.
-->
<script lang="ts">
  import { onMount, onDestroy } from "svelte";

  interface Props {
    expiresAt?: string;
    /** Full TTL of the token in seconds. Defaults to 3600. */
    ttlSeconds?: number;
    /** Diameter in CSS px. */
    size?: number;
    /** Stroke width. */
    stroke?: number;
    /** Show the numeric label inside the ring. */
    showLabel?: boolean;
  }
  let {
    expiresAt,
    ttlSeconds = 3600,
    size = 42,
    stroke = 3,
    showLabel = true,
  }: Props = $props();

  let now = $state(Date.now());
  let raf: number | null = null;

  function tick(): void {
    now = Date.now();
    raf = requestAnimationFrame(tick);
  }

  onMount(() => {
    raf = requestAnimationFrame(tick);
  });
  onDestroy(() => {
    if (raf !== null) cancelAnimationFrame(raf);
  });

  let remainingMs = $derived(() => {
    if (!expiresAt) return 0;
    const t = Date.parse(expiresAt);
    if (Number.isNaN(t)) return 0;
    return Math.max(0, t - now);
  });
  let fraction = $derived(Math.max(0, Math.min(1, remainingMs() / (ttlSeconds * 1000))));
  let tone = $derived(fraction < 0.08 ? "danger" : fraction < 0.25 ? "warn" : "accent");

  let r = $derived((size - stroke) / 2);
  let cx = $derived(size / 2);
  let cy = $derived(size / 2);
  let circ = $derived(2 * Math.PI * r);
  let offset = $derived(circ * (1 - fraction));
  let label = $derived(formatRemaining(remainingMs()));

  function formatRemaining(ms: number): string {
    if (ms <= 0) return "--:--";
    const s = Math.floor(ms / 1000);
    const mm = Math.floor(s / 60);
    const ss = s % 60;
    if (mm >= 60) {
      const h = Math.floor(mm / 60);
      const m = mm % 60;
      return `${h}h${m.toString().padStart(2, "0")}`;
    }
    return `${mm.toString().padStart(2, "0")}:${ss.toString().padStart(2, "0")}`;
  }
</script>

<svg class="ring ring--{tone}" width={size} height={size} viewBox="0 0 {size} {size}" aria-hidden={!showLabel ? "true" : undefined} role={showLabel ? "img" : undefined} aria-label={showLabel ? `time to refresh: ${label}` : undefined}>
  <circle {cx} {cy} {r} class="ring__track" stroke-width={stroke} fill="none" />
  <circle
    {cx}
    {cy}
    {r}
    class="ring__fill"
    stroke-width={stroke}
    fill="none"
    stroke-dasharray={circ}
    stroke-dashoffset={offset}
    transform="rotate(-90 {cx} {cy})"
    stroke-linecap="round"
  />
  {#if showLabel}
    <text x={cx} y={cy + 3} class="ring__label" text-anchor="middle">
      {label}
    </text>
  {/if}
</svg>

<style>
  .ring {
    display: inline-block;
  }
  .ring__track {
    stroke: var(--c-rule);
  }
  .ring__fill {
    stroke: currentColor;
    transition: stroke-dashoffset var(--mo-fast) linear;
  }
  .ring--accent {
    color: var(--c-accent);
  }
  .ring--warn {
    color: var(--c-warn);
  }
  .ring--danger {
    color: var(--c-danger);
  }
  .ring__label {
    font-family: var(--font-mono);
    font-size: 9.5px;
    font-weight: 500;
    fill: var(--c-text-dim);
    letter-spacing: 0.02em;
  }
</style>
