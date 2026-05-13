<!--
  PoolPulse — hero strip above the account table. Per-account compact
  card grid so the operator can take in pool health at a glance before
  drilling into the ledger.

  Each card:
    - Large status dot
    - Abbreviated id
    - Refresh countdown ring (big)
    - req / err counters (tabular)
    - sparkline across the footer
  Hovering or clicking jumps focus to the matching row in AccountBoard.
-->
<script lang="ts">
  import { store } from "../lib/store.svelte";
  import { accountStatus } from "../lib/types";
  import { abbrId, fmtPct } from "../lib/format";
  import CountdownRing from "./CountdownRing.svelte";
  import Sparkline from "./Sparkline.svelte";
  import StatusDot from "./StatusDot.svelte";

  let accounts = $derived(store.snapshot.accounts);
  function focus(id: string): void {
    store.selectAccount(id);
  }

  function totalReqs(): number {
    return accounts.reduce((s, a) => s + a.requests, 0);
  }
  let errorsPct = $derived(() => {
    const t = totalReqs();
    if (t === 0) return 0;
    return accounts.reduce((s, a) => s + a.errors, 0) / t;
  });
</script>

<section class="pulse" aria-label="pool pulse">
  <header class="pulse__head">
    <h2 class="pulse__title">pool pulse</h2>
    <div class="pulse__meta">
      <span class="mono faint" style="font-size: var(--fs-xs)">
        {accounts.length} {accounts.length === 1 ? "identity" : "identities"}
        · error rate {fmtPct(errorsPct())}
      </span>
    </div>
  </header>

  <div class="pulse__grid">
    {#each accounts as a (a.id)}
      {@const st = accountStatus(a)}
      {@const sel = store.selectedAccountId === a.id}
      <button
        type="button"
        class="card card--{st}"
        class:card--selected={sel}
        onclick={() => focus(a.id)}
        title={a.id}
      >
        <header class="card__top">
          <StatusDot status={st} size={10} />
          <code class="card__id mono">{abbrId(a.id, 10)}</code>
          <span class="card__provider faint mono">{a.provider ?? ""}</span>
        </header>
        <div class="card__mid">
          <CountdownRing
            expiresAt={a.expires_at}
            ttlSeconds={3600}
            size={44}
            stroke={3.5}
            showLabel={true}
          />
          <div class="card__stats">
            <div>
              <span class="caps">req</span>
              <span class="card__num mono tabular">{a.requests.toLocaleString()}</span>
            </div>
            <div>
              <span class="caps">err</span>
              <span class="card__num mono tabular card__num--{a.errors > 0 ? 'warn' : 'dim'}">
                {a.errors.toLocaleString()}
              </span>
            </div>
          </div>
        </div>
        <div class="card__bot">
          <span class="card__bot-label caps">last 5m</span>
          <Sparkline
            values={store.perAccountSpark[a.id] ?? []}
            width={180}
            height={22}
            accent="neutral"
          />
        </div>
      </button>
    {/each}

    {#if accounts.length === 0}
      <div class="pulse__empty">
        <p class="pulse__empty-copy">Seventy-six lamps. None lit yet.</p>
        <p class="pulse__empty-hint mono faint">Press <kbd>i</kbd> to paste a vault export.</p>
      </div>
    {/if}

    {#if accounts.length > 0 && accounts.length < 3}
      {#each Array(3 - accounts.length) as _ (_)}
        <div class="card card--ghost" aria-hidden="true">
          <div class="card__ghost-inner">
            <span class="faint mono" style="font-size: var(--fs-2xs); letter-spacing: var(--tr-wide); text-transform: uppercase;">
              capacity slot
            </span>
            <span class="faint" style="font-size: var(--fs-sm)">
              press <kbd>i</kbd> to onboard an identity
            </span>
          </div>
        </div>
      {/each}
    {/if}
  </div>
</section>

<style>
  .pulse {
    background: linear-gradient(
      180deg,
      color-mix(in oklch, var(--c-accent), var(--c-surface) 94%),
      var(--c-surface)
    );
    border: 1px solid var(--c-border);
    border-radius: var(--r-md);
    box-shadow: var(--sh-1);
    padding: var(--sp-5);
  }
  .pulse__head {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    margin-block-end: var(--sp-4);
  }
  .pulse__title {
    margin: 0;
    font-size: var(--fs-md);
    font-family: var(--font-display);
    font-weight: var(--fw-semibold);
    letter-spacing: var(--tr-tight);
  }
  .pulse__grid {
    display: grid;
    gap: var(--sp-3);
    grid-template-columns: repeat(auto-fill, minmax(230px, 1fr));
  }
  .card {
    display: grid;
    gap: var(--sp-3);
    padding: var(--sp-3) var(--sp-4);
    background: var(--c-surface);
    border: 1px solid var(--c-border);
    border-radius: var(--r-sm);
    text-align: start;
    transition:
      border-color var(--mo-fast) var(--ease-std),
      box-shadow var(--mo-fast) var(--ease-std),
      transform var(--mo-fast) var(--ease-std);
  }
  .card:hover {
    border-color: var(--c-border-strong);
    box-shadow: var(--sh-2);
    transform: translateY(-1px);
  }
  .card--selected {
    border-color: var(--c-accent);
    box-shadow: 0 0 0 1px var(--c-accent), var(--sh-2);
  }
  .card--error {
    border-color: color-mix(in oklch, var(--c-danger), transparent 60%);
    background:
      radial-gradient(
        circle at 100% 0%,
        color-mix(in oklch, var(--c-danger), transparent 90%) 0%,
        transparent 60%
      ),
      var(--c-surface);
  }
  .card--cooldown {
    background:
      radial-gradient(
        circle at 100% 0%,
        color-mix(in oklch, var(--c-warn), transparent 90%) 0%,
        transparent 60%
      ),
      var(--c-surface);
  }
  .card__top {
    display: inline-flex;
    align-items: center;
    gap: var(--sp-2);
  }
  .card__id {
    font-size: var(--fs-sm);
    color: var(--c-text);
    flex: 1 1 auto;
  }
  .card__provider {
    font-size: var(--fs-2xs);
    text-transform: uppercase;
    letter-spacing: var(--tr-wide);
  }
  .card__mid {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--sp-4);
  }
  .card__stats {
    display: flex;
    gap: var(--sp-5);
  }
  .card__stats > div {
    display: flex;
    flex-direction: column;
    gap: 1px;
    font-size: var(--fs-xs);
  }
  .card__num {
    font-size: var(--fs-lg);
    font-weight: var(--fw-semibold);
    color: var(--c-text);
    line-height: var(--lh-tight);
  }
  .card__num--warn {
    color: var(--c-danger);
  }
  .card__num--dim {
    color: var(--c-text-faint);
  }
  .card__bot {
    color: var(--c-accent);
    padding-inline-start: 0;
    margin-block-start: 2px;
    display: grid;
    grid-template-columns: auto 1fr;
    align-items: center;
    gap: var(--sp-3);
  }
  .card__bot-label {
    font-size: var(--fs-2xs);
    color: var(--c-text-faint);
  }
  .card__bot :global(svg) {
    inline-size: 100%;
  }
  .pulse__empty {
    grid-column: 1 / -1;
    padding: var(--sp-6);
    text-align: center;
    display: flex;
    flex-direction: column;
    gap: var(--sp-2);
    align-items: center;
  }
  .pulse__empty-copy {
    margin: 0;
    font-family: var(--font-text);
    font-style: italic;
    font-size: var(--fs-md);
    color: var(--c-text);
  }
  .pulse__empty-hint {
    margin: 0;
    font-size: var(--fs-xs);
    color: var(--c-text-faint);
  }
  .card--ghost {
    background: transparent;
    border: 1px dashed var(--c-rule);
    cursor: default;
    min-block-size: 118px;
    display: grid;
    place-items: center;
  }
  .card--ghost:hover {
    border-color: var(--c-border);
    transform: none;
    box-shadow: none;
  }
  .card__ghost-inner {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--sp-2);
    text-align: center;
  }
</style>
