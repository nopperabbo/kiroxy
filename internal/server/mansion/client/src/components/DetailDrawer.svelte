<!--
  DetailDrawer — the "open a request / account to read it" split-pane
  drawer.

  Selected request: shows the lifecycle timeline. Each phase is a thick
  horizontal bar proportional to the synthesized ms breakdown (see
  ../lib/phases.ts). Hover explains the phase.

  Selected account: shows counters, cooldown state, vault expiry ring,
  last error in a readable panel, and a small per-account spark.

  Esc closes (handled in App). Clicking the scrim also closes by clearing
  the selection.
-->
<script lang="ts">
  import { store } from "../lib/store.svelte";
  import { synthesizePhases } from "../lib/phases";
  import { abbrId, fmtBytes, fmtMs, relTime, shortTime } from "../lib/format";
  import { accountStatus } from "../lib/types";
  import Icon from "./Icon.svelte";
  import CountdownRing from "./CountdownRing.svelte";
  import Sparkline from "./Sparkline.svelte";

  let request = $derived(
    store.selectedRequestId ? store.requests.find((r) => r.id === store.selectedRequestId) : null,
  );
  let account = $derived(
    store.selectedAccountId
      ? store.snapshot.accounts.find((a) => a.id === store.selectedAccountId)
      : null,
  );
  let open = $derived(!!(request || account));

  function closeAll(): void {
    store.selectRequest(null);
    store.selectAccount(null);
  }

  let phases = $derived.by(() => {
    if (!request) return [];
    const hasRefresh =
      request.latency_ms > 200 && Math.random() > 0.75; /* synth hint */
    return synthesizePhases(request, hasRefresh);
  });
  let phaseTotal = $derived(phases.reduce((s, p) => s + p.ms, 0));
</script>

{#if open}
  <button class="det-scrim" type="button" aria-label="close details" onclick={closeAll}></button>
  <aside class="det drawer-panel" aria-label="details">
    <header class="det__head">
      {#if request}
        <div>
          <div class="caps">request</div>
          <div class="det__title mono">{request.method} {request.path}</div>
          <div class="faint mono" style="font-size: var(--fs-xs)">
            {shortTime(request.started_at)} · {fmtMs(request.latency_ms)} · {fmtBytes(request.bytes_out)} out
          </div>
        </div>
      {:else if account}
        <div>
          <div class="caps">account</div>
          <div class="det__title mono">{abbrId(account.id, 14)}</div>
          <div class="faint mono" style="font-size: var(--fs-xs)">
            {account.provider ?? "provider"} · {accountStatus(account)}
          </div>
        </div>
      {/if}
      <button type="button" class="iconbtn" aria-label="close" onclick={closeAll}>
        <Icon name="x" size={14} />
      </button>
    </header>

    <div class="det__body">
      {#if request}
        <section class="panel">
          <h3 class="caps">lifecycle</h3>
          <p class="panel__hint faint">
            timings are synthesized from the total request duration because the backend does not
            yet emit per-phase telemetry. swap for real numbers when v1.3 ships.
          </p>
          <ol class="timeline">
            {#each phases as p}
              {@const pct = phaseTotal > 0 ? Math.max(3, (p.ms / phaseTotal) * 100) : 0}
              <li class="timeline__row" title={p.hint}>
                <span class="timeline__label">{p.name}</span>
                <span class="timeline__bar">
                  <span class="timeline__fill" style="inline-size: {pct}%"></span>
                </span>
                <span class="timeline__ms mono tabular">{fmtMs(p.ms)}</span>
              </li>
            {/each}
          </ol>
        </section>
        <section class="panel">
          <h3 class="caps">request detail</h3>
          <dl class="detail-grid">
            <dt>id</dt><dd class="mono">{request.id}</dd>
            <dt>status</dt><dd class="mono">{request.status}</dd>
            <dt>latency</dt><dd class="mono">{fmtMs(request.latency_ms)}</dd>
            <dt>bytes</dt><dd class="mono">{fmtBytes(request.bytes_out)}</dd>
            <dt>remote</dt><dd class="mono">{request.remote_ip ?? "—"}</dd>
            <dt>ua</dt><dd class="mono faint">{request.user_agent ?? "—"}</dd>
            {#if request.account_id}
              <dt>account</dt><dd class="mono">{request.account_id}</dd>
            {/if}
          </dl>
        </section>
      {:else if account}
        <section class="panel panel--flex">
          <div class="aruler">
            <CountdownRing expiresAt={account.expires_at} size={68} stroke={5} />
            <div class="aruler__stack">
              <span class="caps">refresh</span>
              <span class="mono faint">expires_at = {account.expires_at ?? "unknown"}</span>
            </div>
          </div>
        </section>
        <section class="panel">
          <h3 class="caps">last 5 minutes</h3>
          <Sparkline
            values={store.perAccountSpark[account.id] ?? []}
            width={380}
            height={64}
            accent="accent"
            ariaLabel="per-account request volume, last 5 minutes"
          />
        </section>
        <section class="panel">
          <h3 class="caps">counters</h3>
          <dl class="detail-grid">
            <dt>id</dt><dd class="mono">{account.id}</dd>
            <dt>status</dt><dd class="mono">{accountStatus(account)}</dd>
            <dt>enabled</dt><dd class="mono">{account.enabled ? "yes" : "no"}</dd>
            <dt>requests</dt><dd class="mono">{account.requests.toLocaleString()}</dd>
            <dt>errors</dt><dd class="mono">{account.errors.toLocaleString()}</dd>
            {#if account.cooldown_until}
              <dt>cooldown</dt><dd class="mono">{relTime(account.cooldown_until)}</dd>
            {/if}
            {#if account.last_error}
              <dt>last error</dt>
              <dd class="mono det__err">{account.last_error}</dd>
            {/if}
          </dl>
        </section>
      {/if}
    </div>
  </aside>
{/if}

<style>
  .det-scrim {
    position: fixed;
    inset: 0;
    background: color-mix(in oklch, var(--c-bg), transparent 30%);
    z-index: var(--z-drawer);
    cursor: default;
    border: 0;
  }
  .det {
    position: fixed;
    inset-block: 0;
    inset-inline-end: 0;
    inline-size: min(480px, 94vw);
    background: var(--c-surface);
    border-inline-start: 1px solid var(--c-border-strong);
    box-shadow: var(--sh-3);
    z-index: calc(var(--z-drawer) + 1);
    display: flex;
    flex-direction: column;
    animation: slide-in var(--mo-med) var(--ease-out);
  }
  @keyframes slide-in {
    from { transform: translateX(8%); opacity: 0.6; }
    to   { transform: translateX(0);  opacity: 1;   }
  }
  .det__head {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    padding: var(--sp-5);
    border-block-end: 1px solid var(--c-rule);
    gap: var(--sp-5);
  }
  .det__title {
    font-size: var(--fs-md);
    font-weight: var(--fw-semibold);
    color: var(--c-text);
    margin-block-start: 2px;
  }
  .iconbtn {
    display: inline-grid;
    place-items: center;
    inline-size: 28px;
    block-size: 28px;
    border-radius: var(--r-sm);
    color: var(--c-text-dim);
  }
  .iconbtn:hover {
    color: var(--c-text);
    background: var(--c-surface-hover);
  }
  .det__body {
    flex: 1 1 auto;
    overflow-y: auto;
    padding: var(--sp-5);
    display: flex;
    flex-direction: column;
    gap: var(--sp-5);
  }
  .panel {
    padding: var(--sp-4);
    border: 1px solid var(--c-rule);
    border-radius: var(--r-sm);
    background: var(--c-surface-sunken);
    display: flex;
    flex-direction: column;
    gap: var(--sp-3);
  }
  .panel--flex {
    background: var(--c-surface);
  }
  .panel h3 {
    margin: 0;
    font-size: var(--fs-2xs);
  }
  .panel__hint {
    margin: 0;
    font-size: var(--fs-xs);
  }

  .timeline {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: var(--sp-2);
  }
  .timeline__row {
    display: grid;
    grid-template-columns: 80px 1fr 60px;
    align-items: center;
    gap: var(--sp-3);
    font-size: var(--fs-xs);
  }
  .timeline__label {
    color: var(--c-text-dim);
    text-transform: uppercase;
    letter-spacing: var(--tr-wide);
    font-size: var(--fs-2xs);
    font-weight: var(--fw-semibold);
  }
  .timeline__bar {
    position: relative;
    height: 8px;
    background: var(--c-surface-2);
    border-radius: var(--r-pill);
    overflow: hidden;
  }
  .timeline__fill {
    position: absolute;
    inset: 0;
    inline-size: 0;
    background: linear-gradient(to right, var(--c-accent), var(--c-ember));
    animation: bar-grow var(--mo-slow) var(--ease-out);
  }
  @keyframes bar-grow {
    from { transform: scaleX(0); transform-origin: left; }
    to   { transform: scaleX(1); transform-origin: left; }
  }
  .timeline__ms {
    text-align: end;
    color: var(--c-text-dim);
  }

  .detail-grid {
    display: grid;
    grid-template-columns: 100px 1fr;
    gap: var(--sp-2) var(--sp-4);
    font-size: var(--fs-sm);
    margin: 0;
  }
  .detail-grid dt {
    color: var(--c-text-faint);
    text-transform: uppercase;
    letter-spacing: var(--tr-wide);
    font-size: var(--fs-2xs);
  }
  .detail-grid dd {
    margin: 0;
    color: var(--c-text);
    overflow-wrap: anywhere;
  }

  .det__err {
    color: var(--c-danger);
  }

  .aruler {
    display: flex;
    align-items: center;
    gap: var(--sp-4);
  }
  .aruler__stack {
    display: flex;
    flex-direction: column;
    gap: 2px;
    font-size: var(--fs-xs);
  }
</style>
