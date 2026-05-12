<!--
  StatusRibbon — the narrow footer band. Shows live connection state,
  vault path (shortened), version, and a "last update" timestamp.

  The ribbon is deliberately quiet. If everything is fine it reads as a
  single brass line; it only lights up when something's worth noticing
  (offline, vault error, ready_detail set).
-->
<script lang="ts">
  import { store } from "../lib/store.svelte";
  import { shortTime } from "../lib/format";

  let now = $state(Date.now());
  $effect(() => {
    const id = setInterval(() => (now = Date.now()), 1_000);
    return () => clearInterval(id);
  });

  let statusTone = $derived(
    store.liveStatus === "offline"
      ? "bad"
      : store.liveStatus === "stream"
        ? "good"
        : store.liveStatus === "polling"
          ? "accent"
          : "warn",
  );
  let statusText = $derived(
    store.liveStatus === "stream"
      ? "streaming"
      : store.liveStatus === "polling"
        ? "polling every 2s"
        : store.liveStatus === "offline"
          ? "offline — retrying"
          : store.liveStatus,
  );
  let vaultShort = $derived(
    store.snapshot.vault_path ? shortenPath(store.snapshot.vault_path) : "",
  );
  function shortenPath(p: string): string {
    const parts = p.split("/").filter(Boolean);
    if (parts.length <= 3) return p;
    return ".../" + parts.slice(-3).join("/");
  }
</script>

<footer class="ribbon">
  <div class="ribbon__inner">
    <div class="ribbon__item">
      <span class="ribbon__pip ribbon__pip--{statusTone}" aria-hidden="true"></span>
      <span class="ribbon__label caps">live</span>
      <span class="ribbon__value mono">{statusText}</span>
    </div>

    {#if store.snapshot.version}
      <div class="ribbon__item">
        <span class="ribbon__label caps">build</span>
        <code class="ribbon__value mono">{store.snapshot.version}</code>
      </div>
    {/if}

    {#if vaultShort}
      <div class="ribbon__item ribbon__item--path" title={store.snapshot.vault_path}>
        <span class="ribbon__label caps">vault</span>
        <code class="ribbon__value mono">{vaultShort}</code>
        <span class="ribbon__pip ribbon__pip--{store.snapshot.vault_ok ? 'good' : 'bad'}" aria-hidden="true"></span>
      </div>
    {/if}

    {#if store.snapshot.ready_detail && !store.snapshot.ready}
      <div class="ribbon__item ribbon__item--warn">
        <span class="ribbon__label caps">note</span>
        <span class="ribbon__value">{store.snapshot.ready_detail}</span>
      </div>
    {/if}

    <div class="ribbon__spacer"></div>

    <div class="ribbon__item ribbon__item--hints">
      <span><kbd>⌘K</kbd> palette</span>
      <span><kbd>/</kbd> search</span>
      <span><kbd>i</kbd> import</span>
      <span><kbd>?</kbd> keys</span>
    </div>

    <div class="ribbon__item">
      <span class="ribbon__label caps">now</span>
      <span class="ribbon__value mono tabular">{shortTime(new Date(now).toISOString())}</span>
    </div>
  </div>
</footer>

<style>
  .ribbon {
    border-block-start: 1px solid var(--c-border);
    background: var(--c-surface);
    font-family: var(--font-mono);
    font-size: var(--fs-xs);
  }
  .ribbon__inner {
    max-inline-size: var(--app-max);
    margin-inline: auto;
    padding: var(--sp-3) var(--app-pad);
    display: flex;
    align-items: center;
    gap: var(--sp-5);
    flex-wrap: wrap;
  }
  .ribbon__item {
    display: inline-flex;
    align-items: center;
    gap: var(--sp-2);
    min-block-size: 20px;
  }
  .ribbon__label {
    color: var(--c-text-faint);
  }
  .ribbon__value {
    color: var(--c-text-dim);
  }
  .ribbon__pip {
    display: inline-block;
    inline-size: 7px;
    block-size: 7px;
    border-radius: var(--r-pill);
  }
  .ribbon__pip--good {
    background: var(--c-success);
    box-shadow: 0 0 0 3px color-mix(in oklch, var(--c-success), transparent 85%);
  }
  .ribbon__pip--accent {
    background: var(--c-accent);
    box-shadow: 0 0 0 3px var(--c-accent-wash);
  }
  .ribbon__pip--warn {
    background: var(--c-warn);
  }
  .ribbon__pip--bad {
    background: var(--c-danger);
    box-shadow: 0 0 0 3px color-mix(in oklch, var(--c-danger), transparent 80%);
  }
  .ribbon__spacer {
    flex: 1 1 auto;
  }
  .ribbon__item--hints {
    color: var(--c-text-faint);
    gap: var(--sp-4);
  }
  .ribbon__item--warn .ribbon__value {
    color: var(--c-warn);
  }
  @media (max-width: 720px) {
    .ribbon__item--hints {
      display: none;
    }
  }
</style>
