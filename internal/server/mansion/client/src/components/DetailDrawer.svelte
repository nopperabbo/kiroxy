<!--
  DetailDrawer — right-slide 480px drawer for drill-down.

  Two modes:
    1) Request selected  — shows the lifecycle timeline (synthesized phases)
    2) Account selected  — 4-tab layout from the mockup:
         Overview | Requests | Token | Raw

  Both modes slide in from the right edge (480px panel). The pool list
  stays visible underneath — this is NOT a modal; there's no opaque scrim.
  We use a subtle dimming overlay so focus is clear without hiding the
  working context.

  Keyboard:
    - ESC closes (handled in App.svelte)
    - Prev/Next chevrons cycle selection (placeholder: jumps to
      neighboring account in the current pool order)
-->
<script lang="ts">
  import { store } from "../lib/store.svelte";
  import { synthesizePhases } from "../lib/phases";
  import { fmtBytes, fmtMs, relTime, shortTime } from "../lib/format";
  import { accountStatus, type Account } from "../lib/types";
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

  let tab = $derived(store.drawerTab);

  function closeAll(): void {
    store.selectRequest(null);
    store.selectAccount(null);
  }

  function setTab(t: "overview" | "requests" | "token" | "raw"): void {
    store.setDrawerTab(t);
  }

  // Cycle selection to prev/next account in the snapshot order.
  function cycle(dir: -1 | 1): void {
    if (!account) return;
    const list = store.snapshot.accounts;
    const idx = list.findIndex((a) => a.id === account?.id);
    if (idx === -1) return;
    const nextIdx = (idx + dir + list.length) % list.length;
    store.selectAccount(list[nextIdx].id);
  }

  let phases = $derived.by(() => {
    if (!request) return [];
    const hasRefresh = request.latency_ms > 200 && Math.random() > 0.75;
    return synthesizePhases(request, hasRefresh);
  });
  let phaseTotal = $derived(phases.reduce((s, p) => s + p.ms, 0));

  let recentForAccount = $derived.by(() => {
    if (!account) return [];
    return store.requests
      .filter((r) => r.account_id === account?.id)
      .slice(0, 20);
  });

  function weightOf(a: Account): string {
    if (!a.enabled) return "0.00";
    if (a.cooldown_until) {
      const t = Date.parse(a.cooldown_until);
      if (!Number.isNaN(t) && t > Date.now()) return "0.50";
    }
    return "1.00";
  }

  function ageLabel(a: Account): string {
    if (!a.last_used) return "unseen";
    const ms = Date.now() - Date.parse(a.last_used);
    if (Number.isNaN(ms) || ms < 0) return "—";
    const h = Math.floor(ms / 3_600_000);
    const d = Math.floor(h / 24);
    if (d > 0) return `${d}d ${h % 24}h`;
    if (h > 0) return `${h}h ${Math.floor((ms % 3_600_000) / 60_000)}m`;
    const m = Math.floor(ms / 60_000);
    return `${m}m`;
  }

  function statusKind(status: number): "ok" | "err" | "cool" {
    if (status === 0) return "cool";
    if (status >= 400) return "err";
    return "ok";
  }
  function statusLabel(status: number): string {
    if (status === 0) return "COOLDOWN";
    return String(status);
  }
</script>

{#if open}
  <button class="det-dim" type="button" aria-label="close drawer" onclick={closeAll}></button>
  <aside class="det drawer-panel" aria-label="details" role="complementary">
    {#if request}
      <!-- Request mode (legacy — lifecycle timeline for a specific request) -->
      <header class="det__head">
        <div>
          <div class="caps">request</div>
          <div class="det__title mono">{request.method} {request.path}</div>
          <div class="det__meta mono">
            <span><span class="k">time</span> {shortTime(request.started_at)}</span>
            <span><span class="k">lat</span> {fmtMs(request.latency_ms)}</span>
            <span><span class="k">bytes</span> {fmtBytes(request.bytes_out)}</span>
            <span><span class="k">status</span> <span class="st st--{statusKind(request.status)}">{statusLabel(request.status)}</span></span>
          </div>
        </div>
        <button type="button" class="close-btn mono" onclick={closeAll} title="close (ESC)">
          <span>ESC</span><span class="faint">· close</span>
        </button>
      </header>
      <div class="det__body">
        <section class="panel">
          <div class="eyebrow caps">lifecycle</div>
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
          <p class="panel__hint faint">
            Phase timings synthesized from total latency — backend phase
            telemetry ships in v1.3.
          </p>
        </section>
        <section class="panel">
          <div class="eyebrow caps">request detail</div>
          <dl class="kv">
            <dt>id</dt><dd class="mono">{request.id}</dd>
            <dt>remote</dt><dd class="mono">{request.remote_ip ?? "—"}</dd>
            <dt>user-agent</dt><dd class="mono faint">{request.user_agent ?? "—"}</dd>
            {#if request.account_id}
              <dt>account</dt><dd class="mono">{request.account_id}</dd>
            {/if}
          </dl>
        </section>
      </div>
    {:else if account}
      <!-- Account mode — the signature 4-tab layout from the mockup. -->
      <header class="det__head">
        <div class="det__id-row">
          <div class="acct-id-lg mono">{account.id}</div>
          <div class="det__nav">
            <button type="button" class="close-btn" onclick={() => cycle(-1)} title="previous account">‹</button>
            <button type="button" class="close-btn" onclick={() => cycle(1)} title="next account">›</button>
          </div>
          <button type="button" class="close-btn mono" onclick={closeAll} title="close (ESC)">
            <span>ESC</span><span class="faint">· close</span>
          </button>
        </div>
        <div class="det__meta mono">
          <span><span class="k">region</span> {account.region ?? "—"}</span>
          <span><span class="k">state</span> <span class="st--{accountStatus(account) === 'healthy' || accountStatus(account) === 'warm' ? 'ok' : accountStatus(account) === 'cooldown' ? 'cool' : 'err'}">{accountStatus(account)}</span></span>
          <span><span class="k">weight</span> {weightOf(account)}</span>
          <span><span class="k">age</span> {ageLabel(account)}</span>
        </div>
      </header>

      <div class="det__tabs" role="tablist" aria-label="account detail">
        <button type="button" role="tab" class="det-tab" class:det-tab--active={tab === "overview"} aria-selected={tab === "overview"} onclick={() => setTab("overview")}>Overview</button>
        <button type="button" role="tab" class="det-tab" class:det-tab--active={tab === "requests"} aria-selected={tab === "requests"} onclick={() => setTab("requests")}>Requests</button>
        <button type="button" role="tab" class="det-tab" class:det-tab--active={tab === "token"} aria-selected={tab === "token"} onclick={() => setTab("token")}>Token</button>
        <button type="button" role="tab" class="det-tab" class:det-tab--active={tab === "raw"} aria-selected={tab === "raw"} onclick={() => setTab("raw")}>Raw</button>
      </div>

      <div class="det__body">
        {#if tab === "overview"}
          <section class="panel">
            <div class="eyebrow caps">Heartbeat · last 5m</div>
            <div class="det-spark" aria-hidden="true">
              <Sparkline
                values={store.perAccountSpark[account.id] ?? []}
                width={400}
                height={80}
                accent="neutral"
                ariaLabel="per-account request volume, last 5 minutes"
              />
              <div class="det-spark__axis mono">
                <span>-5m</span><span>-4m</span><span>-3m</span><span>-2m</span><span>-1m</span><span>now</span>
              </div>
            </div>
          </section>
          <section class="panel">
            <div class="eyebrow caps">Health · 5m window</div>
            <dl class="kv">
              <dt>requests</dt><dd class="mono tabular">{account.requests.toLocaleString()}</dd>
              <dt>errors</dt>
              <dd class="mono tabular">
                {#if account.errors > 0}
                  <span class="st--err">{account.errors.toLocaleString()} · {((account.errors / Math.max(1, account.requests)) * 100).toFixed(2)}%</span>
                {:else}
                  <span class="faint">0 · 0.00%</span>
                {/if}
              </dd>
              {#if account.last_error}
                <dt>last error</dt>
                <dd class="mono st--err">{account.last_error}</dd>
              {/if}
              {#if account.cooldown_until}
                <dt>cooldown</dt><dd class="mono">{relTime(account.cooldown_until)}</dd>
              {/if}
              <dt>provider</dt><dd class="mono">{account.provider ?? "—"}</dd>
              <dt>auth</dt><dd class="mono">{account.auth_method ?? "—"}</dd>
            </dl>
          </section>
        {:else if tab === "requests"}
          <section class="panel">
            <div class="eyebrow caps">recent · account-scoped</div>
            {#if recentForAccount.length === 0}
              <p class="empty-italic">Nothing recent. Account is idle.</p>
            {:else}
              <div class="mini-reqs">
                {#each recentForAccount as r (r.id)}
                  <button
                    type="button"
                    class="mini-req"
                    onclick={() => store.selectRequest(r.id)}
                  >
                    <span class="t mono tabular">{shortTime(r.started_at)}</span>
                    <span class="p mono">{r.method} {r.path}</span>
                    <span class="l mono tabular">{fmtMs(r.latency_ms)}</span>
                    <span class="s s--{statusKind(r.status)} mono">{statusLabel(r.status)}</span>
                  </button>
                {/each}
              </div>
            {/if}
          </section>
        {:else if tab === "token"}
          <section class="panel">
            <div class="eyebrow caps">token · refresh cycle</div>
            <dl class="kv">
              <dt>provider</dt><dd class="mono">{account.provider ?? "—"}</dd>
              <dt>auth method</dt><dd class="mono">{account.auth_method ?? "—"}</dd>
              {#if account.expires_at}
                <dt>expires</dt>
                <dd class="mono">{account.expires_at} <span class="faint">({relTime(account.expires_at)})</span></dd>
              {:else}
                <dt>expires</dt><dd class="mono faint">unknown</dd>
              {/if}
              <dt>auto-refresh</dt>
              <dd class="mono">
                {#if account.enabled}
                  <span class="st--ok">armed</span> <span class="faint">at T-30m</span>
                {:else}
                  <span class="faint">disabled</span>
                {/if}
              </dd>
            </dl>
          </section>
        {:else}
          <section class="panel">
            <div class="eyebrow caps">raw · vault metadata</div>
            <pre class="raw mono">{JSON.stringify(account, null, 2)}</pre>
          </section>
        {/if}
      </div>

      <div class="det__actions">
        <button
          type="button"
          class="action action--primary"
          onclick={() => store.pushToast("info", "Refresh token requires backend endpoint — stub")}
        >
          Refresh token
        </button>
        <button
          type="button"
          class="action"
          onclick={() => store.pushToast("info", "Pause cooldown requires backend endpoint — stub")}
        >
          Pause cooldown
        </button>
        <button
          type="button"
          class="action action--danger"
          onclick={() => store.pushToast("info", "Hold requires backend endpoint — stub")}
        >
          Hold
        </button>
      </div>
    {/if}
  </aside>
{/if}

<style>
  /* Subtle dim — the pool list stays visible underneath; this is NOT a
     modal scrim. Just enough to shift focus rightward. */
  .det-dim {
    position: fixed;
    inset: 0;
    background: color-mix(in oklch, var(--c-bg), transparent 40%);
    z-index: var(--z-drawer);
    border: 0;
    cursor: default;
  }
  .det {
    position: fixed;
    inset-block: 0;
    inset-inline-end: 0;
    inline-size: min(480px, 100vw);
    background: var(--c-surface);
    border-inline-start: 1px solid var(--c-border-strong);
    z-index: calc(var(--z-drawer) + 1);
    display: flex;
    flex-direction: column;
    min-block-size: 0;
  }

  .det__head {
    display: flex;
    flex-direction: column;
    gap: var(--sp-2);
    padding: var(--sp-4) var(--sp-5);
    border-block-end: 1px solid var(--c-border);
  }
  .det__id-row {
    display: grid;
    grid-template-columns: 1fr auto auto;
    align-items: center;
    gap: var(--sp-3);
  }
  .acct-id-lg {
    font-size: var(--fs-lg);
    letter-spacing: 0.01em;
    color: var(--c-text);
  }
  .det__title {
    font-size: var(--fs-md);
    font-weight: var(--fw-semibold);
    color: var(--c-text);
    margin-block-start: 2px;
  }
  .det__nav {
    display: inline-flex;
    gap: 2px;
  }
  .close-btn {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    color: var(--c-text-faint);
    font-size: 11px;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    padding: 4px 8px;
    border-radius: var(--r-xs);
    transition: color var(--mo-med) var(--ease-std);
  }
  .close-btn:hover { color: var(--c-text); }

  .det__meta {
    display: flex;
    flex-wrap: wrap;
    gap: var(--sp-4);
    color: var(--c-text-dim);
    font-size: 11.5px;
    letter-spacing: 0.02em;
  }
  .det__meta .k { color: var(--c-text-faint); margin-inline-end: 4px; }
  .st { font-size: 10.5px; letter-spacing: 0.08em; text-transform: uppercase; }
  .st--ok   { color: var(--c-success); }
  .st--err  { color: var(--c-danger); }
  .st--cool { color: var(--c-info); }

  .det__tabs {
    display: flex;
    border-block-end: 1px solid var(--c-border);
    padding: 0 var(--sp-4);
  }
  .det-tab {
    block-size: 34px;
    padding: 0 var(--sp-4);
    color: var(--c-text-dim);
    font-size: var(--fs-sm);
    letter-spacing: 0.02em;
    border-block-end: 1.5px solid transparent;
    transition: color var(--mo-med) var(--ease-std);
  }
  .det-tab:hover { color: var(--c-text); }
  /* Drawer tabs use the same amber underline role as the main nav — still
     within amber budget role 3 (active tab underline). */
  .det-tab--active {
    color: var(--c-text);
    border-block-end-color: var(--c-accent);
  }

  .det__body {
    flex: 1;
    overflow-y: auto;
    min-block-size: 0;
  }
  .panel {
    padding: var(--sp-5);
    border-block-end: 1px solid var(--c-border);
    display: flex;
    flex-direction: column;
    gap: var(--sp-3);
  }
  .eyebrow {
    color: var(--c-text-faint);
    font-size: 10.5px;
    letter-spacing: 0.08em;
  }
  .panel__hint {
    margin: 0;
    font-size: var(--fs-xs);
  }

  .det-spark {
    display: flex;
    flex-direction: column;
    gap: var(--sp-2);
  }
  .det-spark__axis {
    display: flex;
    justify-content: space-between;
    color: var(--c-text-faint);
    font-size: 10.5px;
    letter-spacing: 0.03em;
  }

  .kv {
    display: grid;
    grid-template-columns: 120px 1fr;
    gap: 4px var(--sp-4);
    font-size: 12.5px;
    margin: 0;
  }
  .kv dt { color: var(--c-text-dim); }
  .kv dd { margin: 0; color: var(--c-text); }
  .kv .faint { color: var(--c-text-faint); }

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
    letter-spacing: 0.04em;
    font-size: 10.5px;
  }
  .timeline__bar {
    position: relative;
    block-size: 6px;
    background: var(--c-surface-2);
    overflow: hidden;
    border-radius: 1px;
  }
  .timeline__fill {
    position: absolute;
    inset: 0;
    inline-size: 0;
    background: var(--c-text-faint);
  }
  .timeline__ms {
    text-align: end;
    color: var(--c-text-dim);
    font-size: 10.5px;
  }

  .mini-reqs {
    display: flex;
    flex-direction: column;
  }
  .mini-req {
    display: grid;
    grid-template-columns: 64px 1fr 60px 72px;
    gap: var(--sp-3);
    padding: 6px 0;
    border-block-end: 1px solid var(--c-border);
    font-size: 12px;
    align-items: center;
    text-align: start;
    color: var(--c-text-dim);
    transition: color var(--mo-med) var(--ease-std);
  }
  .mini-req:hover { color: var(--c-text); }
  .mini-req:last-child { border-block-end: 0; }
  .mini-req .t { color: var(--c-text-faint); }
  .mini-req .p { color: var(--c-text); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .mini-req .l { text-align: end; color: var(--c-text-dim); }
  .mini-req .s { text-align: end; font-size: 10.5px; letter-spacing: 0.08em; text-transform: uppercase; }
  .s--ok   { color: var(--c-success); }
  .s--err  { color: var(--c-danger); }
  .s--cool { color: var(--c-info); }

  .empty-italic {
    margin: 0;
    font-family: var(--font-text);
    font-style: italic;
    color: var(--c-text-faint);
    font-size: var(--fs-sm);
  }

  .raw {
    margin: 0;
    padding: var(--sp-3);
    background: var(--c-bg);
    border: 1px solid var(--c-border);
    color: var(--c-text-dim);
    font-size: 11px;
    line-height: 1.5;
    border-radius: var(--r-xs);
    overflow-x: auto;
    white-space: pre-wrap;
    word-break: break-all;
  }

  .det__actions {
    display: grid;
    grid-auto-flow: column;
    grid-auto-columns: 1fr;
    gap: 1px;
    background: var(--c-border);
    border-block-start: 1px solid var(--c-border);
  }
  .action {
    block-size: 40px;
    background: var(--c-surface);
    color: var(--c-text-dim);
    font-size: var(--fs-sm);
    letter-spacing: 0.03em;
    transition:
      color var(--mo-med) var(--ease-std),
      background var(--mo-med) var(--ease-std);
  }
  .action:hover { color: var(--c-text); background: var(--c-surface-hover); }
  /* amber budget: role 4 of 5 — primary CTA (Refresh token). */
  .action--primary { color: var(--c-accent); }
  .action--primary:hover {
    background: var(--c-accent-wash);
    color: var(--c-accent);
  }
  .action--danger:hover { color: var(--c-danger); }
</style>
