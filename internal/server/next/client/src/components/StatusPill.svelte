<script lang="ts">
  import type { AccountStatus } from "../lib/types";

  interface Props {
    status: AccountStatus;
    label?: string;
  }
  let { status, label }: Props = $props();

  const text = $derived(label ?? status);
</script>

<span class="pill pill--{status}">
  <span class="pill__dot" aria-hidden="true"></span>
  <span class="pill__text">{text}</span>
</span>

<style>
  .pill {
    display: inline-flex;
    align-items: center;
    gap: var(--sp-3);
    font-size: var(--fs-sm);
    line-height: 1;
    color: var(--c-text-dim);
  }
  .pill__dot {
    inline-size: 6px;
    block-size: 6px;
    border-radius: 50%;
    background: var(--c-text-faint);
    flex: 0 0 auto;
  }
  .pill__text {
    font-variant-numeric: tabular-nums;
  }
  .pill--healthy .pill__dot {
    background: var(--c-success);
    box-shadow: 0 0 6px color-mix(in oklch, var(--c-success), transparent 50%);
  }
  .pill--cooldown .pill__dot {
    background: var(--c-warn);
  }
  .pill--disabled .pill__dot {
    background: var(--c-text-faint);
  }
  .pill--error .pill__dot {
    background: var(--c-danger);
    animation: pulse 1.6s ease-in-out infinite;
  }
  .pill--healthy .pill__text {
    color: var(--c-text);
  }
  .pill--cooldown .pill__text {
    color: var(--c-warn);
  }
  .pill--error .pill__text {
    color: var(--c-danger);
  }

  @keyframes pulse {
    0%,
    100% {
      opacity: 1;
    }
    50% {
      opacity: 0.4;
    }
  }
</style>
