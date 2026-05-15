<!--
  AccountBoard — the account-pool view. Ledger-table aesthetic.

  Each row shows:
    - status glyph (healthy / warm / cooldown / error / disabled)
    - abbreviated id (hover to reveal full)
    - refresh countdown ring (signature)
    - inline sparkline (signature, rolling 5min window)
    - counters (req / err, tabular numerals)
    - cooldown end (countdown if active)
    - last error (dimmed, truncated)
    - row action: view details drawer

  Filter integration: we respect store.filters.search, .onlyErrors, .onlyCooldown.
  Empty state: explicit zero-account message with CTA to open import.
-->
<script lang="ts">
  import { store } from "../lib/store.svelte";
  import { accountStatus, type Account } from "../lib/types";
  import { abbrId, relTime, truncate } from "../lib/format";
  import CountdownRing from "./CountdownRing.svelte";
  import Sparkline from "./Sparkline.svelte";
  import StatusDot from "./StatusDot.svelte";
  import EmptyState from "./EmptyState.svelte";
  import Icon from "./Icon.svelte";

  let sortKey: "status" | "requests" | "errors" | "cooldown" | "id" = $state("status");
  let sortDir: 1 | -1 = $state(1);

  function toggleSort(k: typeof sortKey): void {
    if (sortKey === k) {
      sortDir = sortDir === 1 ? -1 : 1;
    } else {
      sortKey = k;
      sortDir = 1;
    }
  }

  let filtered = $derived(filterAccounts(store.snapshot.accounts));
  let sorted = $derived(sortAccounts(filtered));

  function filterAccounts(list: Account[]): Account[] {
    const q = store.filters.search.trim().toLowerCase();
    const { onlyErrors, onlyCooldown } = store.filters;
    return list.filter((a) => {
      if (q && !a.id.toLowerCase().includes(q)) return false;
      const st = accountStatus(a);
      if (onlyErrors && st !== "error") return false;
      if (onlyCooldown && st !== "cooldown") return false;
      return true;
    });
  }

  function sortAccounts(list: Account[]): Account[] {
    const factor = sortDir;
    const arr = [...list];
    arr.sort((a, b) => {
      if (sortKey === "requests") return (a.requests - b.requests) * factor;
      if (sortKey === "errors") return (a.errors - b.errors) * factor;
      if (sortKey === "cooldown") {
        const ta = a.cooldown_until ? Date.parse(a.cooldown_until) : 0;
        const tb = b.cooldown_until ? Date.parse(b.cooldown_until) : 0;
        return (ta - tb) * factor;
      }
      if (sortKey === "id") return a.id.localeCompare(b.id) * factor;
      // status: error > cooldown > warn > healthy > disabled
      const order: Record<string, number> = {
        error: 0,
        cooldown: 1,
        warm: 2,
        healthy: 3,
        disabled: 4,
      };
      return (order[accountStatus(a)] - order[accountStatus(b)]) * factor;
    });
    return arr;
  }

  function select(id: string): void {
    store.selectAccount(id);
  }
  function copyId(id: string, e: Event): void {
    e.stopPropagation();
    void navigator.clipboard.writeText(id).then(
      () => store.pushToast("ok", `copied ${abbrId(id, 10)}…`),
      () => store.pushToast("err", "clipboard denied"),
    );
  }
  let arrow = $derived(sortDir === 1 ? "▼" : "▲");
</script>

<section class="board" aria-label="account pool">
  <header class="board__head">
    <div class="board__title">
      <span class="caps">accounts</span>
      <span class="board__count mono tabular">{store.snapshot.accounts.length}</span>
    </div>
    <div class="board__filters">
      <label class="chip chip--toggle" class:active={store.filters.onlyErrors}>
        <input type="checkbox" bind:checked={() => store.filters.onlyErrors, (v) => store.setFilter("onlyErrors", v)} />
        errors only
      </label>
      <label class="chip chip--toggle" class:active={store.filters.onlyCooldown}>
        <input type="checkbox" bind:checked={() => store.filters.onlyCooldown, (v) => store.setFilter("onlyCooldown", v)} />
        cooldown only
      </label>
    </div>
  </header>

  <div class="board__table" role="table">
    <div class="row row--head" role="row">
      <span role="columnheader" class="col col--status">
        <button type="button" class="th" onclick={() => toggleSort("status")}>
          status
          {#if sortKey === "status"}<span class="th__arrow">{arrow}</span>{/if}
        </button>
      </span>
      <span role="columnheader" class="col col--id">
        <button type="button" class="th" onclick={() => toggleSort("id")}>
          account
          {#if sortKey === "id"}<span class="th__arrow">{arrow}</span>{/if}
        </button>
      </span>
      <span role="columnheader" class="col col--ring">refresh</span>
      <span role="columnheader" class="col col--spark">last 5m</span>
      <span role="columnheader" class="col col--req">
        <button type="button" class="th th--right" onclick={() => toggleSort("requests")}>
          req
          {#if sortKey === "requests"}<span class="th__arrow">{arrow}</span>{/if}
        </button>
      </span>
      <span role="columnheader" class="col col--err">
        <button type="button" class="th th--right" onclick={() => toggleSort("errors")}>
          err
          {#if sortKey === "errors"}<span class="th__arrow">{arrow}</span>{/if}
        </button>
      </span>
      <span role="columnheader" class="col col--usage">credits</span>
      <span role="columnheader" class="col col--cool">
        <button type="button" class="th" onclick={() => toggleSort("cooldown")}>
          cooldown
          {#if sortKey === "cooldown"}<span class="th__arrow">{arrow}</span>{/if}
        </button>
      </span>
      <span role="columnheader" class="col col--note">last error</span>
    </div>

    {#if sorted.length === 0}
      <div class="empty" role="row">
        {#if store.snapshot.accounts.length === 0}
          <EmptyState
            glyph="◇"
            title="The pool lies dormant."
            hint="Press i to paste a JSON export, or use kiroxy import-accounts-json from the CLI."
          >
            <button type="button" class="btn btn--accent" onclick={() => window.dispatchEvent(new KeyboardEvent('keydown', {key: 'i'}))}>
              Import account
            </button>
          </EmptyState>
        {:else}
          <EmptyState
            glyph="≠"
            density="tight"
            title="All accounts hide behind the filter."
            hint="Loosen the chips above to see the full pool."
          />
        {/if}
      </div>
    {/if}

    {#each sorted as a (a.id)}
      {@const st = accountStatus(a)}
      {@const sel = store.selectedAccountId === a.id}
      {@const errRate = a.requests > 0 ? a.errors / a.requests : 0}
      {@const isAnomaly = a.requests >= 10 && errRate > 0.01 && errRate <= 0.25}
      {@const ago = a.last_used ? Date.now() - Date.parse(a.last_used) : Infinity}
      {@const isIdle = ago > 300000 && st !== "disabled"}
      <div
        class="row row--account"
        class:row--selected={sel}
        class:row--error={st === "error"}
        class:row--cooldown={st === "cooldown"}
        class:row--disabled={st === "disabled"}
        class:row--anomaly={isAnomaly}
        class:row--idle={isIdle}
        data-account-id={a.id}
        role="row"
        tabindex="0"
        onclick={() => select(a.id)}
        onkeydown={(e) => {
          if (e.key === "Enter" || e.key === " ") {
            e.preventDefault();
            select(a.id);
          }
        }}
      >
        <span class="col col--status" role="cell">
          <StatusDot status={st} />
        </span>
        <span class="col col--id" role="cell">
          <span class="idline">
            <code class="idline__code mono" title={a.id}>{abbrId(a.id, 14)}</code>
            <button
              type="button"
              class="idline__copy"
              onclick={(e) => copyId(a.id, e)}
              aria-label="copy full id"
              title="copy full id"
            >
              <Icon name="copy" size={11} />
            </button>
          </span>
          <span class="idline__meta faint">
            {#if a.provider || a.region}
              {a.provider ?? "provider"} · {a.region ?? "—"}
            {/if}
            <span class="mobile-last-used">
              {#if a.errors > 0}
                 · err <span class="c-danger">{a.errors}</span>
              {/if}
               · idle {a.last_used ? relTime(a.last_used) : "—"}
            </span>
          </span>
        </span>
        <span class="col col--ring" role="cell">
          <CountdownRing expiresAt={a.expires_at} ttlSeconds={3600} size={36} stroke={3} />
        </span>
        <span class="col col--spark" role="cell">
          <Sparkline
            values={store.perAccountSpark[a.id] ?? []}
            width={140}
            height={18}
            accent="neutral"
            ariaLabel="requests per bucket last 5 minutes"
          />
        </span>
        <span class="col col--req mono tabular" role="cell">{a.requests.toLocaleString()}</span>
        <span class="col col--err mono tabular" class:dim={a.errors === 0} role="cell">
          {a.errors.toLocaleString()}
        </span>
        <span class="col col--usage" role="cell">
          {#if a.usage_known && a.usage_cap}
            {@const pct = (a.usage_used ?? 0) / a.usage_cap}
            {@const tone = pct >= 0.9 ? "danger" : pct >= 0.7 ? "warn" : "ok"}
            <span class="usage usage--{tone}" title="{a.usage_used} / {a.usage_cap} credits used · {a.subscription_title ?? a.subscription_tier ?? ""} · resets in {a.usage_days_until_reset ?? "?"}d">
              <span class="usage__bar" aria-hidden="true">
                <span class="usage__fill" style:width="{Math.min(100, pct * 100).toFixed(1)}%"></span>
              </span>
              <span class="usage__num mono tabular">{a.usage_used ?? 0}<span class="usage__sep">/</span>{a.usage_cap}</span>
              {#if a.subscription_tier && a.subscription_tier !== "unknown"}
                <span class="tier tier--{a.subscription_tier}">{a.subscription_tier === "pro_plus" ? "pro+" : a.subscription_tier}</span>
              {/if}
            </span>
          {:else}
            <span class="faint">—</span>
          {/if}
        </span>
        <span class="col col--cool" role="cell">
          {#if a.cooldown_until}
            <span class="cool cool--{st}">
              <span class="cool__dot" aria-hidden="true"></span>
              {relTime(a.cooldown_until)}
            </span>
          {:else}
            <span class="faint">—</span>
          {/if}
        </span>
        <span class="col col--note faint" role="cell" title={a.last_error}>
          {a.last_error ? truncate(a.last_error, 64) : "—"}
        </span>
      </div>
    {/each}
  </div>
</section>

<style>
  .board {
    background: var(--c-surface);
    border: 1px solid var(--c-border);
    border-radius: var(--r-md);
    box-shadow: var(--sh-1);
  }
  .board__head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: var(--sp-4) var(--sp-5);
    gap: var(--sp-5);
    border-block-end: 1px solid var(--c-rule);
    position: sticky;
    top: 0;
    z-index: 20;
    background: var(--c-surface);
    border-start-start-radius: var(--r-md);
    border-start-end-radius: var(--r-md);
  }
  .row--head {
    padding-block: var(--sp-2);
    background: var(--c-surface-sunken);
    color: var(--c-text-dim);
    position: sticky;
    top: 61px;
    z-index: 10;
  }
  .board__title {
    display: inline-flex;
    align-items: baseline;
    gap: var(--sp-3);
  }
  .board__count {
    font-size: var(--fs-xs);
    color: var(--c-accent);
    padding: 1px 6px;
    border: 1px solid color-mix(in oklch, var(--c-accent), transparent 60%);
    border-radius: var(--r-pill);
    background: var(--c-accent-wash);
  }
  .board__filters {
    display: inline-flex;
    gap: var(--sp-3);
  }
  .chip--toggle {
    cursor: pointer;
    user-select: none;
    opacity: 0.68;
    transition:
      color var(--mo-fast) var(--ease-std),
      background var(--mo-fast) var(--ease-std),
      border-color var(--mo-fast) var(--ease-std);
  }
  .chip--toggle:hover {
    opacity: 1;
  }
  .chip--toggle input {
    position: absolute;
    opacity: 0;
    pointer-events: none;
  }
  .chip--toggle.active {
    opacity: 1;
    color: var(--c-accent);
    border-color: color-mix(in oklch, var(--c-accent), transparent 40%);
    background: var(--c-accent-wash);
  }

  .board__table {
    display: grid;
  }
  .row {
    display: grid;
    grid-template-columns:
      28px minmax(140px, 1.2fr) 54px 110px 64px 52px minmax(140px, 1fr) 96px minmax(0, 1.5fr);
    align-items: center;
    gap: var(--sp-4);
    padding: var(--sp-3) var(--sp-5);
    border-block-end: 1px solid var(--c-rule);
    text-align: start;
    color: var(--c-text);
  }
  .row:last-child {
    border-block-end: none;
  }
  .row--head {
    padding-block: var(--sp-2);
    background: var(--c-surface-sunken);
    color: var(--c-text-dim);
  }
  .row--account {
    cursor: pointer;
    background: var(--c-surface);
    transition: background var(--mo-fast) var(--ease-std);
    font-family: var(--font-text);
  }
  .row--account:hover {
    background: var(--c-surface-hover);
  }
  .row--account:focus-visible {
    outline: none;
    background: var(--c-surface-hover);
    box-shadow: inset 0 0 0 2px var(--c-accent);
  }
  .row--selected {
    background: color-mix(in oklch, var(--c-accent-wash), var(--c-surface));
    box-shadow: inset 2px 0 0 0 var(--c-accent);
  }
  .row--anomaly {
    box-shadow: inset 0 0 0 2px var(--c-accent);
    position: relative;
  }
  .row--anomaly::after {
    content: "";
    position: absolute;
    inset-block-start: 0;
    inset-inline-end: 0;
    inline-size: 6px;
    block-size: 6px;
    background: var(--c-accent);
    border-end-start-radius: var(--r-sm);
  }
  .row--anomaly.row--selected {
    box-shadow: inset 0 0 0 2px var(--c-accent), inset 4px 0 0 0 var(--c-accent);
  }
  .row--error .col--err {
    color: var(--c-danger);
  }
  .row--cooldown {
    background: color-mix(in oklch, var(--c-accent-wash), var(--c-surface));
  }
  .row--disabled {
    opacity: 0.7;
  }
  .row--idle {
    opacity: 0.6;
  }

  .col {
    display: inline-flex;
    align-items: center;
    min-inline-size: 0;
  }
  .col--status {
    justify-content: center;
  }
  .col--req,
  .col--err {
    justify-content: flex-end;
    font-variant-numeric: tabular-nums;
  }
  .col--note {
    font-size: var(--fs-xs);
  }
  .col--usage {
    display: flex;
    align-items: center;
    gap: var(--sp-2);
    min-inline-size: 0;
  }
  .usage {
    display: inline-flex;
    align-items: center;
    gap: var(--sp-2);
    inline-size: 100%;
    min-inline-size: 0;
  }
  .usage__bar {
    inline-size: 56px;
    block-size: 6px;
    background: var(--c-surface-sunken);
    border-radius: 999px;
    overflow: hidden;
    flex: 0 0 auto;
  }
  .usage__fill {
    display: block;
    block-size: 100%;
    background: var(--c-accent);
    border-radius: 999px;
    transition: width var(--mo-fast) var(--ease-std);
  }
  .usage--warn .usage__fill {
    background: var(--c-warn, #d4a017);
  }
  .usage--danger .usage__fill {
    background: var(--c-danger, #d04646);
  }
  .usage__num {
    font-size: var(--fs-xs);
    color: var(--c-text);
    white-space: nowrap;
  }
  .usage__sep {
    color: var(--c-text-dim);
    margin-inline: 1px;
  }
  .tier {
    font-size: var(--fs-2xs);
    text-transform: uppercase;
    letter-spacing: var(--tr-caps);
    padding: 1px 5px;
    border-radius: 3px;
    background: var(--c-surface-sunken);
    color: var(--c-text-dim);
    font-weight: var(--fw-semibold);
    flex: 0 0 auto;
  }
  .tier--pro,
  .tier--pro_plus,
  .tier--power {
    background: color-mix(in oklch, var(--c-accent-wash), var(--c-surface));
    color: var(--c-accent);
  }
  .tier--free {
    color: var(--c-text-dim);
  }

  .th {
    font: inherit;
    color: inherit;
    letter-spacing: var(--tr-caps);
    text-transform: uppercase;
    font-size: var(--fs-2xs);
    font-weight: var(--fw-semibold);
    cursor: pointer;
    padding-inline: var(--sp-1);
    border-radius: var(--r-xs);
    transition: color var(--mo-fast) var(--ease-std);
  }
  .th:hover {
    color: var(--c-accent);
  }
  .th--right {
    justify-content: flex-end;
  }
  .th__arrow {
    display: inline-block;
    margin-inline-start: var(--sp-1);
    color: var(--c-text-faint);
    transition:
      color var(--mo-fast) var(--ease-std),
      transform var(--mo-fast) var(--ease-std);
  }
  .th:hover .th__arrow {
    color: var(--c-accent);
  }
  .th__arrow--desc {
    transform: rotate(180deg);
  }

  .idline {
    display: inline-flex;
    align-items: center;
    gap: var(--sp-2);
    min-inline-size: 0;
  }
  .idline__code {
    font-size: var(--fs-sm);
    color: var(--c-text);
    padding: 1px var(--sp-2);
    background: var(--c-surface-2);
    border: 1px solid var(--c-rule);
    border-radius: var(--r-sm);
  }
  .idline__copy {
    display: inline-grid;
    place-items: center;
    color: var(--c-text-faint);
    padding: 2px;
    border-radius: var(--r-xs);
  }
  .idline__copy:hover {
    color: var(--c-accent);
    background: var(--c-accent-wash);
  }
  .idline__meta {
    display: block;
    grid-column: 1 / -1;
    font-size: var(--fs-2xs);
    font-family: var(--font-mono);
    color: var(--c-text-faint);
    margin-block-start: 1px;
  }

  .cool {
    display: inline-flex;
    align-items: center;
    gap: var(--sp-2);
    font-size: var(--fs-xs);
    font-family: var(--font-mono);
    color: var(--c-warn);
  }
  .cool--error {
    color: var(--c-danger);
  }
  .cool__dot {
    inline-size: 6px;
    block-size: 6px;
    border-radius: var(--r-pill);
    background: currentColor;
    animation: pulse-ring 1.8s var(--ease-out) infinite;
    position: relative;
  }

  .empty {
    display: flex;
    justify-content: center;
    padding: var(--sp-3) var(--sp-5);
    border-block-end: 1px solid var(--c-rule);
  }
  .empty:last-child {
    border-block-end: none;
  }

  .btn {
    display: inline-flex;
    align-items: center;
    gap: var(--sp-2);
    padding: 5px 10px;
    font-size: var(--fs-sm);
    font-family: var(--font-mono);
    background: var(--c-surface);
    border: 1px solid var(--c-border);
    border-radius: var(--r-sm);
    color: var(--c-text-dim);
    cursor: pointer;
    transition:
      color var(--mo-fast) var(--ease-std),
      border-color var(--mo-fast) var(--ease-std),
      background var(--mo-fast) var(--ease-std);
  }
  .btn:hover {
    color: var(--c-text);
    border-color: var(--c-border-strong);
  }
  .btn--accent {
    color: var(--c-accent);
    border-color: color-mix(in oklch, var(--c-accent), transparent 50%);
    background: var(--c-accent-wash);
  }
  .btn--accent:hover {
    color: var(--c-accent-strong);
  }

  @media (max-width: 720px) {
    .row {
      grid-template-columns: 24px minmax(0, 1fr) 44px 56px 56px;
      grid-auto-rows: auto;
      gap: var(--sp-3);
    }
    .col--spark,
    .col--cool,
    .col--note {
      display: none;
    }
  }

  .c-danger { color: var(--c-danger); }
  .mobile-last-used { display: none; }

  @media (max-width: 480px) {
    .row {
      grid-template-columns: 24px 1fr auto;
      grid-template-areas: 
        "status id req"
        "status meta cool";
      gap: 4px var(--sp-2);
      padding: var(--sp-3) var(--sp-4);
    }
    .row--head {
      display: none;
    }
    .col--status { grid-area: status; align-self: start; padding-top: 4px; }
    .col--id { grid-area: id; }
    .col--req { grid-area: req; justify-content: flex-end; }
    .col--req::before { content: "req "; margin-right: 4px; opacity: 0.5; font-weight: normal; }
    .col--err { display: none; }
    .col--cool { display: inline-flex; grid-area: cool; justify-content: flex-end; }
    .col--ring, .col--spark, .col--note { display: none; }

    .idline__meta {
      grid-area: meta;
      white-space: normal;
    }
    .mobile-last-used {
      display: inline;
    }

    .btn, .chip--toggle, .th {
      min-block-size: 44px;
      block-size: auto;
    }
    .board__head {
      padding-block: var(--sp-2);
      flex-wrap: wrap;
    }
    .board__filters {
      width: 100%;
      justify-content: flex-start;
    }
  }
</style>
