<!--
  RequestFeed — rolling 50 most-recent requests. New rows animate in via
  @starting-style. Click/keyboard opens the native <dialog> with details.
-->
<script lang="ts">
  import { store } from "../lib/stores.svelte";
  import { formatLatency, formatTimeOfDay } from "../lib/format";
  import type { RequestRecord } from "../lib/types";
  import Icon from "./icons/Icon.svelte";

  let detail = $state<RequestRecord | null>(null);
  let dialogEl: HTMLDialogElement | undefined = $state();

  function statusKind(s: number): "ok" | "warn" | "err" {
    if (s >= 500) return "err";
    if (s >= 400) return "warn";
    return "ok";
  }

  function openDetail(r: RequestRecord): void {
    detail = r;
    queueMicrotask(() => dialogEl?.showModal());
  }

  function closeDetail(): void {
    dialogEl?.close();
  }

  function onRowKey(e: KeyboardEvent, r: RequestRecord): void {
    if (e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      openDetail(r);
    }
  }

  function shortUA(ua?: string): string {
    if (!ua) return "";
    // Heuristic: surface known clients.
    if (ua.includes("opencode")) return "opencode";
    if (ua.includes("claude-code")) return "claude-code";
    if (ua.includes("curl")) return "curl";
    return ua.split(/[\s/]/)[0] ?? ua.slice(0, 20);
  }
</script>

<section class="feed" aria-labelledby="feed-heading">
  <div class="feed__head">
    <h2 id="feed-heading" class="feed__title">
      requests
      <span class="feed__count dim">
        {store.requests.length === 0 ? "no activity yet" : `last ${store.requests.length}`}
      </span>
    </h2>
  </div>

  {#if store.requests.length === 0}
    <div class="feed__empty">
      <p class="faint">waiting for the first request…</p>
    </div>
  {:else}
    <ol class="feed__list" aria-live="polite" aria-relevant="additions">
      {#each store.requests as r (r.id)}
        {@const kind = statusKind(r.status)}
        <li
          class="item item--{kind}"
          tabindex="0"
          onclick={() => openDetail(r)}
          onkeydown={(e) => onRowKey(e, r)}
        >
          <span class="item__time mono tnum">{formatTimeOfDay(r.started_at)}</span>
          <span class="item__method mono">{r.method}</span>
          <span class="item__path mono">{r.path}</span>
          <span class="item__status mono item__status--{kind}">{r.status}</span>
          <span class="item__lat mono tnum">{formatLatency(r.latency_ms)}</span>
          <span class="item__ua mono faint">{shortUA(r.user_agent)}</span>
        </li>
      {/each}
    </ol>
  {/if}
</section>

<dialog
  bind:this={dialogEl}
  class="rdetail"
  aria-labelledby="rdetail-heading"
>
  {#if detail}
    <div class="rdetail__head">
      <h3 id="rdetail-heading" class="rdetail__title">
        <span class="mono">{detail.method}</span>
        <span class="mono">{detail.path}</span>
      </h3>
      <button
        type="button"
        class="iconbtn"
        aria-label="close"
        onclick={closeDetail}><Icon name="x" /></button
      >
    </div>
    <dl class="rdetail__meta">
      <dt>id</dt>
      <dd class="mono">{detail.id}</dd>
      <dt>started</dt>
      <dd class="mono">{new Date(detail.started_at).toLocaleString()}</dd>
      <dt>status</dt>
      <dd class="mono tnum">{detail.status}</dd>
      <dt>latency</dt>
      <dd class="mono tnum">{formatLatency(detail.latency_ms)}</dd>
      <dt>bytes</dt>
      <dd class="mono tnum">{detail.bytes_out}</dd>
      <dt>remote</dt>
      <dd class="mono">{detail.remote_ip || "—"}</dd>
      <dt>agent</dt>
      <dd class="mono agent">{detail.user_agent || "—"}</dd>
    </dl>
  {/if}
</dialog>

<style>
  .feed {
    display: flex;
    flex-direction: column;
    gap: var(--sp-4);
  }
  .feed__head {
    display: flex;
    align-items: baseline;
    gap: var(--sp-5);
    padding-inline: var(--sp-6);
  }
  .feed__title {
    font-size: var(--fs-lg);
    font-weight: var(--fw-semibold);
    margin: 0;
    color: var(--c-text);
    letter-spacing: -0.01em;
  }
  .feed__count {
    font-size: var(--fs-sm);
    font-weight: var(--fw-normal);
    margin-inline-start: var(--sp-3);
  }
  .feed__empty {
    padding: var(--sp-7) var(--sp-6);
    color: var(--c-text-faint);
  }

  .feed__list {
    border-block-start: 1px solid var(--c-border);
    overflow-y: auto;
    max-block-size: 60vh;
  }
  .item {
    display: grid;
    grid-template-columns: auto auto 1fr auto auto auto;
    gap: var(--sp-4);
    padding: var(--sp-3) var(--sp-6);
    font-size: var(--fs-sm);
    border-block-end: 1px solid var(--c-border);
    cursor: pointer;
    align-items: center;
    transition: background var(--mo-fast);
  }
  .item:hover,
  .item:focus-within {
    background: var(--c-surface-hover);
  }
  .item:last-child {
    border-block-end: none;
  }

  /* Enter animation for new rows */
  @starting-style {
    .item {
      background: var(--c-accent-bg);
    }
  }
  .item {
    transition:
      background var(--mo-slow) var(--ease-out),
      border-color var(--mo-fast);
  }

  .item__time {
    color: var(--c-text-faint);
    font-size: var(--fs-xs);
  }
  .item__method {
    color: var(--c-text-dim);
    font-size: var(--fs-xs);
    padding: 1px 5px;
    border-radius: var(--r-sm);
    border: 1px solid var(--c-border);
  }
  .item__path {
    color: var(--c-text);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .item__status {
    font-weight: var(--fw-semibold);
  }
  .item__status--ok {
    color: var(--c-success);
  }
  .item__status--warn {
    color: var(--c-warn);
  }
  .item__status--err {
    color: var(--c-danger);
  }
  .item--err {
    background: color-mix(in oklch, var(--c-danger), transparent 94%);
  }
  .item__lat {
    color: var(--c-text-dim);
    font-size: var(--fs-xs);
    min-inline-size: 3.5ch;
    text-align: end;
  }
  .item__ua {
    font-size: var(--fs-xs);
    justify-self: end;
  }

  .faint {
    color: var(--c-text-faint);
  }
  .dim {
    color: var(--c-text-dim);
  }

  .rdetail {
    inline-size: min(560px, 92vw);
    max-block-size: 80vh;
    border: 1px solid var(--c-border);
    border-radius: var(--r-lg);
    background: var(--c-surface);
    color: var(--c-text);
    padding: 0;
  }
  .rdetail::backdrop {
    background: color-mix(in oklch, var(--c-bg), transparent 30%);
    backdrop-filter: blur(2px);
  }
  .rdetail__head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: var(--sp-4) var(--sp-5);
    border-block-end: 1px solid var(--c-border);
  }
  .rdetail__title {
    margin: 0;
    display: flex;
    gap: var(--sp-3);
    align-items: baseline;
    font-weight: var(--fw-semibold);
    font-size: var(--fs-md);
  }
  .rdetail__meta {
    display: grid;
    grid-template-columns: max-content 1fr;
    gap: var(--sp-3) var(--sp-5);
    padding: var(--sp-5);
    margin: 0;
  }
  .rdetail__meta dt {
    color: var(--c-text-faint);
    font-size: var(--fs-xs);
    text-transform: uppercase;
    letter-spacing: 0.06em;
  }
  .rdetail__meta dd {
    margin: 0;
    font-size: var(--fs-sm);
    overflow-wrap: anywhere;
  }
  .agent {
    font-size: var(--fs-xs);
    color: var(--c-text-dim);
  }
  .iconbtn {
    display: inline-grid;
    place-items: center;
    inline-size: 24px;
    block-size: 24px;
    border-radius: var(--r-sm);
    color: var(--c-text-dim);
  }
  .iconbtn:hover {
    background: var(--c-surface-hover);
    color: var(--c-text);
  }

  @container (inline-size < 580px) {
    .item {
      grid-template-columns: auto 1fr auto auto;
    }
    .item__ua,
    .item__method {
      display: none;
    }
  }
</style>
