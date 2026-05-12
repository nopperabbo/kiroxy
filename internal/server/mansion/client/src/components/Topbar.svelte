<!--
  Topbar — the "brass plaque" for the operator desk.

  Three bands:
    left   — wordmark + vault status sigil + ready state
    center — total-requests sparkline + headline counters (uptime, errors)
    right  — search, import button, theme, palette hint

  Responsive: below 780px we collapse center into right so nothing wraps.
-->
<script lang="ts">
  import { store } from "../lib/store.svelte";
  import { readScheme, setScheme, cycleScheme, type Scheme } from "../lib/theme";
  import { fmtUptime } from "../lib/format";
  import Sparkline from "./Sparkline.svelte";
  import Icon from "./Icon.svelte";

  interface Props {
    onOpenPalette: () => void;
    onOpenImport: () => void;
  }
  let { onOpenPalette, onOpenImport }: Props = $props();

  let scheme: Scheme = $state(readScheme());
  function toggleScheme(): void {
    scheme = cycleScheme(scheme);
    setScheme(scheme);
  }
  function schemeIcon(s: Scheme): string {
    if (s === "dark") return "moon";
    if (s === "light") return "sun";
    return "monitor";
  }

  let total = $derived(
    store.snapshot.total_requests ??
      store.snapshot.accounts.reduce((n, a) => n + a.requests, 0),
  );
  let errs = $derived(
    store.snapshot.total_errors ?? store.snapshot.accounts.reduce((n, a) => n + a.errors, 0),
  );
  let errRate = $derived(total > 0 ? errs / total : 0);
  let errClass = $derived(errRate > 0.1 ? "danger" : errRate > 0.02 ? "warn" : "good");
</script>

<header class="topbar" aria-label="kiroxy control plane">
  <div class="topbar__inner">
    <!-- Brand -->
    <div class="brand">
      <span class="brand__glyph" aria-hidden="true">
        <svg width="22" height="22" viewBox="0 0 22 22" fill="none">
          <!-- Brass keyhole geometry — a hand-drawn sigil, not a brand icon. -->
          <rect x="1" y="1" width="20" height="20" rx="3" stroke="currentColor" stroke-width="1.25" />
          <circle cx="11" cy="8.5" r="2.5" stroke="currentColor" stroke-width="1.25" />
          <path d="M11 11 L11 15.5 L9 15.5 M11 13.5 L12.5 13.5" stroke="currentColor" stroke-width="1.25" stroke-linecap="round" />
        </svg>
      </span>
      <div class="brand__stack">
        <span class="brand__word">kiroxy</span>
        <span class="brand__sub faint">mansion · operator desk</span>
      </div>
    </div>

    <!-- Center: state + spark -->
    <div class="state">
      <div class="state__sigil state__sigil--{store.snapshot.ready ? 'good' : 'warn'}" aria-hidden="true">
        <span class="state__pulse"></span>
      </div>
      <div class="state__stack">
        <span class="state__label caps">status</span>
        <span class="state__value">{store.snapshot.ready ? "ready" : "not ready"}</span>
      </div>
      <span class="state__sep" aria-hidden="true"></span>
      <div class="state__stack" title="total requests observed by this proxy">
        <span class="state__label caps">requests</span>
        <span class="state__value mono tabular">{total.toLocaleString()}</span>
      </div>
      <div class="state__spark" aria-hidden="true">
        <Sparkline values={store.totalSpark} width={120} height={28} accent="accent" />
      </div>
      <div class="state__stack state__stack--{errClass}" title="error rate over total requests">
        <span class="state__label caps">errors</span>
        <span class="state__value mono tabular">{errs.toLocaleString()} · {(errRate * 100).toFixed(1)}%</span>
      </div>
      <span class="state__sep" aria-hidden="true"></span>
      <div class="state__stack" title="uptime since server start">
        <span class="state__label caps">uptime</span>
        <span class="state__value mono tabular">{fmtUptime(store.snapshot.uptime_s)}</span>
      </div>
    </div>

    <!-- Actions -->
    <div class="actions">
      <label class="search" aria-label="search accounts and requests">
        <Icon name="search" size={13} />
        <input
          class="search__input"
          placeholder="search pool / requests"
          data-search-input
          bind:value={() => store.filters.search, (v) => store.setFilter("search", v)}
        />
        <kbd>/</kbd>
      </label>
      <button class="btn btn--ghost" type="button" onclick={onOpenImport} title="import accounts (i)">
        <Icon name="download" size={13} /><span>import</span>
      </button>
      <button class="btn btn--icon" type="button" onclick={toggleScheme} title="cycle theme" aria-label="cycle theme">
        <Icon name={schemeIcon(scheme)} size={13} />
      </button>
      <button class="btn btn--ghost btn--palette" type="button" onclick={onOpenPalette} title="command palette (⌘K)">
        <Icon name="command" size={13} /><span>palette</span><kbd class="kbd-lite">⌘K</kbd>
      </button>
    </div>
  </div>

  <!-- Hairline brass rule — single ledger divider under the plaque. -->
  <div class="topbar__rule" aria-hidden="true"></div>
</header>

<style>
  .topbar {
    position: sticky;
    inset-block-start: 0;
    z-index: var(--z-sticky);
    background: color-mix(in oklch, var(--c-bg), transparent 12%);
    backdrop-filter: blur(10px) saturate(1.15);
    -webkit-backdrop-filter: blur(10px) saturate(1.15);
  }
  .topbar__inner {
    max-inline-size: var(--app-max);
    margin-inline: auto;
    padding: var(--sp-4) var(--app-pad);
    display: grid;
    grid-template-columns: auto 1fr auto;
    gap: var(--sp-6);
    align-items: center;
  }
  .topbar__rule {
    height: 1px;
    background: linear-gradient(
      to right,
      transparent,
      color-mix(in oklch, var(--c-accent), transparent 60%) 15%,
      color-mix(in oklch, var(--c-accent), transparent 60%) 85%,
      transparent
    );
    opacity: 0.5;
  }

  .brand {
    display: inline-flex;
    align-items: center;
    gap: var(--sp-3);
    color: var(--c-accent);
  }
  .brand__glyph {
    display: inline-grid;
    place-items: center;
    inline-size: 30px;
    block-size: 30px;
    border: 1px solid color-mix(in oklch, var(--c-accent), transparent 55%);
    border-radius: var(--r-sm);
    box-shadow: var(--sh-1);
    background: color-mix(in oklch, var(--c-accent), transparent 92%);
  }
  .brand__stack {
    display: flex;
    flex-direction: column;
    line-height: var(--lh-tight);
  }
  .brand__word {
    font-family: var(--font-display);
    font-size: var(--fs-md);
    font-weight: var(--fw-semibold);
    letter-spacing: var(--tr-tight);
    color: var(--c-text);
  }
  .brand__sub {
    font-family: var(--font-mono);
    font-size: var(--fs-2xs);
    letter-spacing: var(--tr-wide);
    text-transform: uppercase;
    color: var(--c-text-faint);
  }

  .state {
    display: inline-flex;
    align-items: center;
    gap: var(--sp-5);
    justify-self: center;
    padding: var(--sp-3) var(--sp-5);
    border: 1px solid var(--c-border);
    border-radius: var(--r-md);
    background: var(--c-surface);
    box-shadow: var(--sh-1);
  }
  .state__sigil {
    position: relative;
    inline-size: 10px;
    block-size: 10px;
    border-radius: var(--r-pill);
  }
  .state__sigil--good {
    background: var(--c-success);
  }
  .state__sigil--warn {
    background: var(--c-warn);
  }
  .state__pulse {
    position: absolute;
    inset: 0;
    border-radius: var(--r-pill);
    background: currentColor;
    animation: pulse-ring 2.2s var(--ease-out) infinite;
  }
  .state__stack {
    display: flex;
    flex-direction: column;
    line-height: var(--lh-tight);
    min-inline-size: 0;
  }
  .state__stack--warn .state__value {
    color: var(--c-warn);
  }
  .state__stack--danger .state__value {
    color: var(--c-danger);
  }
  .state__label {
    font-size: var(--fs-2xs);
  }
  .state__value {
    font-size: var(--fs-sm);
    color: var(--c-text);
  }
  .state__sep {
    inline-size: 1px;
    block-size: 20px;
    background: var(--c-rule);
  }
  .state__spark {
    display: inline-flex;
    padding: 2px var(--sp-2);
    border-left: 1px solid var(--c-rule);
    border-right: 1px solid var(--c-rule);
    color: var(--c-accent);
  }

  .actions {
    display: inline-flex;
    align-items: center;
    gap: var(--sp-3);
  }

  .search {
    display: inline-flex;
    align-items: center;
    gap: var(--sp-3);
    padding: 6px var(--sp-3);
    min-inline-size: 200px;
    background: var(--c-surface);
    border: 1px solid var(--c-border);
    border-radius: var(--r-md);
    color: var(--c-text-dim);
    transition: border-color var(--mo-fast) var(--ease-std);
  }
  .search:focus-within {
    border-color: var(--c-accent);
  }
  .search__input {
    flex: 1 1 auto;
    min-inline-size: 0;
    font-size: var(--fs-sm);
    font-family: var(--font-mono);
    color: var(--c-text);
  }
  .search__input::placeholder {
    color: var(--c-text-faint);
  }

  .btn {
    display: inline-flex;
    align-items: center;
    gap: var(--sp-2);
    padding: 6px var(--sp-3);
    border: 1px solid var(--c-border);
    border-radius: var(--r-md);
    background: var(--c-surface);
    color: var(--c-text-dim);
    font-size: var(--fs-sm);
    transition:
      color var(--mo-fast) var(--ease-std),
      border-color var(--mo-fast) var(--ease-std),
      background var(--mo-fast) var(--ease-std);
  }
  .btn:hover {
    color: var(--c-text);
    border-color: var(--c-border-strong);
    background: var(--c-surface-hover);
  }
  .btn--icon {
    inline-size: 30px;
    block-size: 30px;
    padding: 0;
    justify-content: center;
  }
  .btn--palette {
    color: var(--c-accent);
    border-color: color-mix(in oklch, var(--c-accent), transparent 60%);
    background: var(--c-accent-wash);
  }
  .kbd-lite {
    padding: 1px var(--sp-2);
    font-size: var(--fs-2xs);
    color: var(--c-text-dim);
    background: transparent;
    border: 1px solid var(--c-border);
    border-radius: var(--r-sm);
  }

  @media (max-width: 900px) {
    .topbar__inner {
      grid-template-columns: auto 1fr;
    }
    .state {
      grid-column: 1 / -1;
      order: 3;
      justify-self: stretch;
      overflow-x: auto;
    }
    .actions {
      order: 2;
      justify-self: end;
    }
    .brand__sub {
      display: none;
    }
    .btn--palette span,
    .btn--ghost span {
      display: none;
    }
    .search {
      min-inline-size: 160px;
    }
  }
</style>
