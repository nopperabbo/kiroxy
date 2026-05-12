<!--
  AccountTable — dense, sortable, keyboard-navigable. Rows update in place
  via key-driven re-render. Click (or Enter) opens a drill-down dialog.

  Sort: last_used desc default. Column header click toggles direction.
-->
<script lang="ts">
  import { store } from "../lib/stores.svelte";
  import type { Account } from "../lib/types";
  import { accountStatus } from "../lib/types";
  import {
    formatTimeAgo,
    formatCooldown,
    formatCount,
  } from "../lib/format";
  import StatusPill from "./StatusPill.svelte";
  import Icon from "./icons/Icon.svelte";
  import { api } from "../lib/api";

  type SortKey = "id" | "status" | "requests" | "errors" | "cooldown" | "last_used";

  let sortKey = $state<SortKey>("last_used");
  let sortDir = $state<"asc" | "desc">("desc");

  let detail = $state<Account | null>(null);
  let detailOpen = $state(false);
  let dialogEl: HTMLDialogElement | undefined = $state();

  const rows = $derived(sortAccounts(store.snapshot.accounts, sortKey, sortDir));

  const summary = $derived.by(() => {
    const accts = store.snapshot.accounts;
    const healthy = accts.filter((a) => accountStatus(a) === "healthy").length;
    const cooldown = accts.filter((a) => accountStatus(a) === "cooldown").length;
    const disabled = accts.filter((a) => accountStatus(a) === "disabled").length;
    return { total: accts.length, healthy, cooldown, disabled };
  });

  function sortAccounts(list: Account[], key: SortKey, dir: "asc" | "desc"): Account[] {
    const mult = dir === "asc" ? 1 : -1;
    const copy = [...list];
    copy.sort((a, b) => {
      const va = extract(a, key);
      const vb = extract(b, key);
      if (va === vb) return a.id.localeCompare(b.id);
      if (va == null) return 1;
      if (vb == null) return -1;
      if (typeof va === "number" && typeof vb === "number") return (va - vb) * mult;
      return String(va).localeCompare(String(vb)) * mult;
    });
    return copy;
  }

  function extract(a: Account, key: SortKey): number | string | null {
    switch (key) {
      case "id":
        return a.id;
      case "status":
        return accountStatus(a);
      case "requests":
        return a.requests;
      case "errors":
        return a.errors;
      case "cooldown":
        return a.cooldown_until ? Date.parse(a.cooldown_until) : 0;
      case "last_used":
        return a.last_used ? Date.parse(a.last_used) : 0;
    }
  }

  function toggleSort(key: SortKey): void {
    if (sortKey === key) {
      sortDir = sortDir === "asc" ? "desc" : "asc";
    } else {
      sortKey = key;
      sortDir = key === "id" ? "asc" : "desc";
    }
  }

  function sortIndicator(key: SortKey): string {
    if (sortKey !== key) return "";
    return sortDir === "asc" ? "▲" : "▼";
  }

  function openDetail(a: Account): void {
    detail = a;
    detailOpen = true;
    queueMicrotask(() => dialogEl?.showModal());
  }

  function closeDetail(): void {
    detailOpen = false;
    dialogEl?.close();
  }

  async function removeAccount(a: Account): Promise<void> {
    const provider = a.provider || "kiro";
    const ok = confirm(`Remove account ${a.id} (${provider}) from the pool?`);
    if (!ok) return;
    const res = await api.removeAccount(provider, a.id);
    if (res.ok) {
      store.pushToast("ok", `removed ${a.id}`);
      closeDetail();
    } else {
      store.pushToast("err", `remove failed: ${res.error}`);
    }
  }

  function onRowKey(e: KeyboardEvent, a: Account): void {
    if (e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      openDetail(a);
    }
  }
</script>

<section class="pool" aria-labelledby="pool-heading">
  <div class="pool__head">
    <h2 id="pool-heading" class="pool__title">
      pool
      <span class="pool__summary dim">
        {summary.total} account{summary.total === 1 ? "" : "s"}
        {#if summary.healthy > 0}· <span class="ok">{summary.healthy} healthy</span>{/if}
        {#if summary.cooldown > 0}· <span class="warn">{summary.cooldown} cooldown</span>{/if}
        {#if summary.disabled > 0}· <span class="faint">{summary.disabled} disabled</span>{/if}
      </span>
    </h2>
  </div>

  {#if rows.length === 0}
    <div class="pool__empty">
      <p>no accounts in vault</p>
      <p class="faint">
        run <code>kiroxy add-account</code> or press
        <kbd>i</kbd> to open the import modal
      </p>
    </div>
  {:else}
    <div class="pool__scroll">
      <table class="tbl" role="grid">
        <thead>
          <tr>
            <th><button type="button" onclick={() => toggleSort("id")}>id {sortIndicator("id")}</button></th>
            <th><button type="button" onclick={() => toggleSort("status")}>status {sortIndicator("status")}</button></th>
            <th class="num"><button type="button" onclick={() => toggleSort("requests")}>req {sortIndicator("requests")}</button></th>
            <th class="num"><button type="button" onclick={() => toggleSort("errors")}>err {sortIndicator("errors")}</button></th>
            <th><button type="button" onclick={() => toggleSort("cooldown")}>cooldown {sortIndicator("cooldown")}</button></th>
            <th><button type="button" onclick={() => toggleSort("last_used")}>last {sortIndicator("last_used")}</button></th>
            <th><span class="sr-only">actions</span></th>
          </tr>
        </thead>
        <tbody>
          {#each rows as a (a.id)}
            {@const status = accountStatus(a)}
            <tr
              class="row"
              class:row--dim={status === "disabled"}
              tabindex="0"
              onclick={() => openDetail(a)}
              onkeydown={(e) => onRowKey(e, a)}
              data-account-id={a.id}
            >
              <td class="mono">{a.id}</td>
              <td><StatusPill {status} /></td>
              <td class="num mono tnum">{formatCount(a.requests)}</td>
              <td class="num mono tnum" class:err={a.errors > 0}>
                {formatCount(a.errors)}
              </td>
              <td class="mono tnum">{formatCooldown(a.cooldown_until)}</td>
              <td class="mono faint">{formatTimeAgo(a.last_used)}</td>
              <td class="row__actions">
                <button
                  type="button"
                  class="iconbtn"
                  title="remove account"
                  aria-label="remove account {a.id}"
                  onclick={(e) => {
                    e.stopPropagation();
                    void removeAccount(a);
                  }}
                >
                  <Icon name="trash" size={13} />
                </button>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</section>

<dialog
  bind:this={dialogEl}
  class="drawer"
  aria-labelledby="drawer-heading"
  onclose={() => (detailOpen = false)}
>
  {#if detail}
    <div class="drawer__head">
      <h3 id="drawer-heading" class="drawer__title mono">{detail.id}</h3>
      <button
        type="button"
        class="iconbtn"
        aria-label="close"
        onclick={closeDetail}><Icon name="x" /></button
      >
    </div>
    <dl class="drawer__meta">
      <dt>status</dt>
      <dd><StatusPill status={accountStatus(detail)} /></dd>
      <dt>provider</dt>
      <dd class="mono">{detail.provider || "—"}</dd>
      <dt>region</dt>
      <dd class="mono">{detail.region || "—"}</dd>
      <dt>auth</dt>
      <dd class="mono">{detail.auth_method || "—"}</dd>
      <dt>requests</dt>
      <dd class="mono tnum">{detail.requests}</dd>
      <dt>errors</dt>
      <dd class="mono tnum" class:err={detail.errors > 0}>{detail.errors}</dd>
      <dt>cooldown</dt>
      <dd class="mono tnum">{formatCooldown(detail.cooldown_until)}</dd>
      <dt>last used</dt>
      <dd class="mono">{formatTimeAgo(detail.last_used)}</dd>
      {#if detail.last_error}
        <dt>last error</dt>
        <dd class="mono err drawer__error">{detail.last_error}</dd>
      {/if}
    </dl>
    <div class="drawer__actions">
      <button
        type="button"
        class="btn btn--danger"
        onclick={() => void removeAccount(detail!)}
      >
        <Icon name="trash" size={12} />
        remove
      </button>
    </div>
  {/if}
</dialog>

<style>
  .pool {
    container-type: inline-size;
    display: flex;
    flex-direction: column;
    gap: var(--sp-4);
  }
  .pool__head {
    display: flex;
    align-items: baseline;
    gap: var(--sp-5);
    padding-inline: var(--sp-6);
  }
  .pool__title {
    font-size: var(--fs-lg);
    font-weight: var(--fw-semibold);
    margin: 0;
    color: var(--c-text);
    letter-spacing: -0.01em;
  }
  .pool__summary {
    font-size: var(--fs-sm);
    font-weight: var(--fw-normal);
    margin-inline-start: var(--sp-3);
  }
  .pool__empty {
    padding: var(--sp-7) var(--sp-6);
    text-align: center;
    color: var(--c-text-dim);
  }
  .pool__empty p {
    margin: var(--sp-2) 0;
    text-wrap: pretty;
  }
  .pool__empty code {
    background: var(--c-surface-2);
    padding: 2px 6px;
    border-radius: var(--r-sm);
    border: 1px solid var(--c-border);
  }

  .pool__scroll {
    overflow-x: auto;
    border-block-start: 1px solid var(--c-border);
  }

  .tbl {
    inline-size: 100%;
    font-size: var(--fs-sm);
  }
  .tbl thead th {
    font-weight: var(--fw-medium);
    color: var(--c-text-faint);
    text-transform: uppercase;
    font-size: var(--fs-xs);
    letter-spacing: 0.06em;
    text-align: start;
    padding: var(--sp-3) var(--sp-4);
    border-block-end: 1px solid var(--c-border);
    background: var(--c-surface);
    position: sticky;
    inset-block-start: 0;
  }
  .tbl thead th.num {
    text-align: end;
  }
  .tbl thead button {
    background: transparent;
    color: inherit;
    font: inherit;
    padding: 0;
    cursor: pointer;
    letter-spacing: inherit;
  }
  .tbl thead button:hover {
    color: var(--c-text);
  }
  .tbl td {
    padding: var(--sp-3) var(--sp-4);
    border-block-end: 1px solid var(--c-border);
    color: var(--c-text);
    font-variant-numeric: tabular-nums;
  }
  .tbl td.num {
    text-align: end;
  }
  .tbl tbody tr:last-child td {
    border-block-end: none;
  }

  .row {
    transition: background var(--mo-fast) var(--ease-std);
    cursor: pointer;
  }
  .row:hover,
  .row:focus-within {
    background: var(--c-surface-hover);
  }
  .row--dim td {
    color: var(--c-text-faint);
  }
  .row__actions {
    text-align: end;
    inline-size: 1%;
  }

  .err {
    color: var(--c-danger);
  }
  .ok {
    color: var(--c-success);
  }
  .warn {
    color: var(--c-warn);
  }
  .faint {
    color: var(--c-text-faint);
  }
  .dim {
    color: var(--c-text-dim);
  }

  .iconbtn {
    display: inline-grid;
    place-items: center;
    inline-size: 24px;
    block-size: 24px;
    border-radius: var(--r-sm);
    color: var(--c-text-dim);
    transition: background var(--mo-fast), color var(--mo-fast);
  }
  .iconbtn:hover {
    color: var(--c-danger);
    background: var(--c-danger-bg);
  }

  /* Drawer dialog */
  .drawer {
    inline-size: min(420px, 92vw);
    block-size: 100vh;
    max-block-size: 100vh;
    margin: 0 0 0 auto;
    border: none;
    border-inline-start: 1px solid var(--c-border);
    background: var(--c-surface);
    color: var(--c-text);
    padding: 0;
    overflow-y: auto;
  }
  .drawer::backdrop {
    background: color-mix(in oklch, var(--c-bg), transparent 40%);
    backdrop-filter: blur(2px);
  }
  @starting-style {
    .drawer[open] {
      transform: translateX(100%);
      opacity: 0;
    }
  }
  .drawer[open] {
    transform: translateX(0);
    opacity: 1;
    transition:
      transform var(--mo-med) var(--ease-std),
      opacity var(--mo-med) var(--ease-std);
  }
  .drawer__head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: var(--sp-5) var(--sp-5) var(--sp-3);
    border-block-end: 1px solid var(--c-border);
  }
  .drawer__title {
    margin: 0;
    font-size: var(--fs-md);
    font-weight: var(--fw-semibold);
  }
  .drawer__meta {
    display: grid;
    grid-template-columns: max-content 1fr;
    gap: var(--sp-3) var(--sp-5);
    padding: var(--sp-5);
    margin: 0;
  }
  .drawer__meta dt {
    color: var(--c-text-faint);
    font-size: var(--fs-xs);
    text-transform: uppercase;
    letter-spacing: 0.06em;
    align-self: center;
  }
  .drawer__meta dd {
    margin: 0;
    font-size: var(--fs-sm);
  }
  .drawer__error {
    grid-column: 1 / -1;
    font-size: var(--fs-xs);
    word-break: break-word;
    background: var(--c-danger-bg);
    padding: var(--sp-3);
    border-radius: var(--r-sm);
    border: 1px solid color-mix(in oklch, var(--c-danger), transparent 60%);
  }
  .drawer__actions {
    padding: var(--sp-5);
    border-block-start: 1px solid var(--c-border);
    display: flex;
    justify-content: flex-end;
  }

  .btn {
    display: inline-flex;
    align-items: center;
    gap: var(--sp-2);
    padding: var(--sp-2) var(--sp-4);
    border-radius: var(--r-sm);
    font-size: var(--fs-sm);
    background: var(--c-surface-2);
    color: var(--c-text);
    border: 1px solid var(--c-border);
    transition: background var(--mo-fast);
  }
  .btn:hover {
    background: var(--c-surface-hover);
  }
  .btn--danger {
    color: var(--c-danger);
    border-color: color-mix(in oklch, var(--c-danger), transparent 60%);
  }
  .btn--danger:hover {
    background: var(--c-danger-bg);
  }
</style>
