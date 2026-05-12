<!--
  StatusDot — a small, readable status indicator. Not just a colored
  circle: each status has a distinct fill pattern so it's also
  differentiable in high-contrast mode and for colorblind users.

  healthy   — solid filled dot
  warm      — filled dot with inner ring (active)
  cooldown  — hollow ring with diagonal bar (pending)
  error     — solid dot with X cap
  disabled  — hollow ring
-->
<script lang="ts">
  import type { AccountStatus } from "../lib/types";

  interface Props {
    status: AccountStatus;
    size?: number;
  }
  let { status, size = 12 }: Props = $props();
  let tone = $derived(
    status === "error"
      ? "danger"
      : status === "cooldown"
        ? "warn"
        : status === "disabled"
          ? "ghost"
          : status === "warm"
            ? "accent"
            : "good",
  );
  let label = $derived(
    status === "error"
      ? "error"
      : status === "cooldown"
        ? "cooldown"
        : status === "disabled"
          ? "disabled"
          : status === "warm"
            ? "warm (recently used)"
            : "healthy",
  );
</script>

<span class="dot dot--{tone}" role="img" aria-label={label} style="--s:{size}px">
  {#if status === "cooldown"}
    <svg viewBox="0 0 10 10" width={size} height={size} aria-hidden="true">
      <circle cx="5" cy="5" r="3.5" class="dot__ring" />
      <path d="M2.8 7.2 L7.2 2.8" class="dot__slash" />
    </svg>
  {:else if status === "error"}
    <svg viewBox="0 0 10 10" width={size} height={size} aria-hidden="true">
      <circle cx="5" cy="5" r="3.2" class="dot__fill" />
      <path d="M3.5 3.5 L6.5 6.5 M6.5 3.5 L3.5 6.5" class="dot__cap" />
    </svg>
  {:else if status === "disabled"}
    <svg viewBox="0 0 10 10" width={size} height={size} aria-hidden="true">
      <circle cx="5" cy="5" r="3.2" class="dot__ring" />
    </svg>
  {:else if status === "warm"}
    <svg viewBox="0 0 10 10" width={size} height={size} aria-hidden="true">
      <circle cx="5" cy="5" r="3.2" class="dot__fill" />
      <circle cx="5" cy="5" r="1.5" class="dot__eye" />
    </svg>
  {:else}
    <svg viewBox="0 0 10 10" width={size} height={size} aria-hidden="true">
      <circle cx="5" cy="5" r="3" class="dot__fill" />
    </svg>
  {/if}
</span>

<style>
  .dot {
    display: inline-grid;
    place-items: center;
    inline-size: var(--s);
    block-size: var(--s);
  }
  .dot__fill {
    fill: currentColor;
  }
  .dot__ring {
    fill: none;
    stroke: currentColor;
    stroke-width: 1.2;
  }
  .dot__slash {
    stroke: currentColor;
    stroke-width: 1.2;
    stroke-linecap: round;
  }
  .dot__cap {
    stroke: var(--c-surface);
    stroke-width: 1;
    stroke-linecap: round;
  }
  .dot__eye {
    fill: var(--c-surface);
  }
  .dot--good {
    color: var(--c-success);
  }
  .dot--warn {
    color: var(--c-warn);
  }
  .dot--danger {
    color: var(--c-danger);
  }
  .dot--accent {
    color: var(--c-accent);
  }
  .dot--ghost {
    color: var(--c-text-faint);
  }
</style>
