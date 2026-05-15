<!--
  Topbar — the "brass plaque" for the operator desk.

  Three-band layout:
    left   — wordmark with trailing amber cursor (amber budget role 1)
             + live dot (amber budget role 2)
    center — three-tab nav: Live Stream / Pool / Metrics.
             The active tab gets the amber underline (amber budget role 3).
    right  — search, import button, theme, palette hint (⌘K kbd = role 5).

  Amber budget (5 roles, enforced across the app):
    1. Wordmark trailing cursor (this file)
    2. Live dot pulse           (this file)
    3. Active tab underline     (this file + DetailDrawer tab bar)
    4. Primary CTA              (DetailDrawer 'Refresh token')
    5. Keyboard shortcut pills  (this file ⌘K + CommandPalette ↵)

  Nothing else carries amber — not data, tables, charts, dividers,
  timestamps, or IDs. Errors use --c-danger, success uses --c-success.

  The old status-pill mid-band moved out to the StatusRibbon at page-
  bottom, which is where the mockup puts ambient telemetry. That kept
  the topbar visually quieter and made room for the nav.
-->
<script lang="ts">
  import { store, type MansionView } from "../lib/store.svelte";
  import { readScheme, setScheme, cycleScheme, type Scheme } from "../lib/theme";
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

  let activeCount = $derived(
    store.snapshot.accounts.filter((a) => a.enabled && !a.cooldown_until).length,
  );
  let poolCount = $derived(store.snapshot.accounts.length);

  // Rolling rate for Live-tab counter hint: total request delta over the
  // 5-min spark window, then /300 for per-second. Cheap, adequate.
  let rate = $derived.by(() => {
    const sum = store.totalSpark.reduce((a, b) => a + b, 0);
    if (sum <= 0) return 0;
    return sum / 300;
  });

  function switchTo(v: MansionView): void {
    if (store.view === v) return;
    // View Transitions API: if supported, wrap the state mutation so the
    // browser cross-fades between views for free (no JS animation runner).
    // The fallback is a plain synchronous swap — acceptable in older browsers.
    type VtDoc = Document & { startViewTransition?: (cb: () => void) => void };
    const d = document as VtDoc;
    if (d.startViewTransition) {
      d.startViewTransition(() => {
        store.setView(v);
      });
    } else {
      store.setView(v);
    }
  }
</script>

<header class="topbar" aria-label="kiroxy control plane">
  <div class="topbar__inner">
    <!-- Brand: wordmark + blinking amber cursor (amber role 1). -->
    <div class="brand">
      <span class="brand__word mono">kiroxy</span>
      <span class="brand__cursor mono" aria-hidden="true">_</span>
      <span class="brand__version mono faint">
        {store.snapshot.version || "dev"}
      </span>
    </div>

    <!-- Center: three-tab nav with amber underline (amber role 3). -->
    <nav class="nav" role="tablist" aria-label="view">
      <button
        type="button"
        class="nav__tab"
        class:nav__tab--active={store.view === "live"}
        role="tab"
        aria-selected={store.view === "live"}
        aria-controls="view-live"
        onclick={() => switchTo("live")}
        title="Live Stream (Warp-style request feed)"
      >
        <span class="nav__glyph" aria-hidden="true">
          <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
            <path d="M1 6h2.5l1.5-4 2 8 1.5-4H11" stroke="currentColor" stroke-width="1" stroke-linecap="round" stroke-linejoin="round"/>
          </svg>
        </span>
        <span>Live Stream</span>
        <span class="nav__count mono tabular" aria-hidden="true">
          {rate > 0 ? `· ${rate.toFixed(1)}/s` : "· idle"}
        </span>
      </button>
      <button
        type="button"
        class="nav__tab"
        class:nav__tab--active={store.view === "pool"}
        role="tab"
        aria-selected={store.view === "pool"}
        aria-controls="view-pool"
        onclick={() => switchTo("pool")}
        title="Account pool"
      >
        <span class="nav__glyph" aria-hidden="true">
          <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
            <rect x="1.5" y="1.5" width="3" height="3" stroke="currentColor" stroke-width="1"/>
            <rect x="7.5" y="1.5" width="3" height="3" stroke="currentColor" stroke-width="1"/>
            <rect x="1.5" y="7.5" width="3" height="3" stroke="currentColor" stroke-width="1"/>
            <rect x="7.5" y="7.5" width="3" height="3" stroke="currentColor" stroke-width="1"/>
          </svg>
        </span>
        <span>Pool</span>
        <span class="nav__count mono tabular" aria-hidden="true">
          · {activeCount}/{poolCount}
        </span>
      </button>
      <button
        type="button"
        class="nav__tab"
        class:nav__tab--active={store.view === "metrics"}
        role="tab"
        aria-selected={store.view === "metrics"}
        aria-controls="view-metrics"
        onclick={() => switchTo("metrics")}
        title="Aggregate metrics"
      >
        <span class="nav__glyph" aria-hidden="true">
          <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
            <path d="M2 10V5m3 5V2m3 8v-4m3 4V7" stroke="currentColor" stroke-width="1" stroke-linecap="round"/>
          </svg>
        </span>
        <span>Metrics</span>
      </button>

      <span class="nav__rule" aria-hidden="true"></span>

      <button
        type="button"
        class="nav__tab nav__tab--ops"
        class:nav__tab--active={store.view === "logs"}
        role="tab"
        aria-selected={store.view === "logs"}
        aria-controls="view-logs"
        onclick={() => switchTo("logs")}
        title="Server logs (cmd+K › view:logs)"
      >
        <span class="nav__glyph" aria-hidden="true">
          <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
            <path d="M2 2.5h8M2 6h8M2 9.5h5" stroke="currentColor" stroke-width="1" stroke-linecap="round"/>
          </svg>
        </span>
        <span>Logs</span>
      </button>
      <button
        type="button"
        class="nav__tab nav__tab--ops"
        class:nav__tab--active={store.view === "models"}
        role="tab"
        aria-selected={store.view === "models"}
        aria-controls="view-models"
        onclick={() => switchTo("models")}
        title="Models routing"
      >
        <span class="nav__glyph" aria-hidden="true">
          <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
            <circle cx="6" cy="6" r="2" stroke="currentColor" stroke-width="1"/>
            <path d="M6 1v2M6 9v2M1 6h2M9 6h2M2.5 2.5l1.4 1.4M8.1 8.1l1.4 1.4M2.5 9.5l1.4-1.4M8.1 3.9l1.4-1.4" stroke="currentColor" stroke-width="1" stroke-linecap="round"/>
          </svg>
        </span>
        <span>Models</span>
      </button>
      <button
        type="button"
        class="nav__tab nav__tab--ops"
        class:nav__tab--active={store.view === "tools"}
        role="tab"
        aria-selected={store.view === "tools"}
        aria-controls="view-tools"
        onclick={() => switchTo("tools")}
        title="Diagnostic, backup, onboarder"
      >
        <span class="nav__glyph" aria-hidden="true">
          <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
            <path d="M2 10l3-3M5 7l1.5-1.5a2 2 0 1 0-2-2L3 5l-1 1 4 4z" stroke="currentColor" stroke-width="1" stroke-linecap="round" stroke-linejoin="round"/>
          </svg>
        </span>
        <span>Tools</span>
      </button>
      <button
        type="button"
        class="nav__tab nav__tab--ops"
        class:nav__tab--active={store.view === "settings"}
        role="tab"
        aria-selected={store.view === "settings"}
        aria-controls="view-settings"
        onclick={() => switchTo("settings")}
        title="Server settings"
      >
        <span class="nav__glyph" aria-hidden="true">
          <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
            <circle cx="6" cy="6" r="1.5" stroke="currentColor" stroke-width="1"/>
            <path d="M6 1.5v1.5M6 9v1.5M1.5 6h1.5M9 6h1.5M2.5 2.5l1.1 1.1M8.4 8.4l1.1 1.1M2.5 9.5l1.1-1.1M8.4 3.6l1.1-1.1" stroke="currentColor" stroke-width="1" stroke-linecap="round"/>
          </svg>
        </span>
        <span>Settings</span>
      </button>
    </nav>

    <!-- Actions: search, import, theme, palette (⌘K kbd = amber role 5). -->
    <div class="actions">
      <span class="live-dot mono" aria-label="live stream connection">
        <span class="live-dot__pulse" aria-hidden="true"></span>LIVE
      </span>
      <label class="search" aria-label="search accounts and requests">
        <Icon name="search" size={12} />
        <input
          class="search__input mono"
          placeholder="filter acct / path / id"
          data-search-input
          bind:value={() => store.filters.search, (v) => store.setFilter("search", v)}
        />
        <kbd>/</kbd>
      </label>
      <button class="btn btn--ghost" type="button" onclick={onOpenImport} title="import accounts (i)">
        <Icon name="download" size={12} /><span>import</span>
      </button>
      <button class="btn btn--icon" type="button" onclick={toggleScheme} title="cycle theme" aria-label="cycle theme">
        <Icon name={schemeIcon(scheme)} size={12} />
      </button>
      <button class="btn btn--palette" type="button" onclick={onOpenPalette} title="command palette (⌘K)">
        <Icon name="command" size={12} /><kbd class="kbd-amber">⌘K</kbd>
      </button>
    </div>
  </div>
</header>

<style>
  .topbar {
    position: sticky;
    inset-block-start: 0;
    z-index: var(--z-sticky);
    background: var(--c-bg);
    border-block-end: 1px solid var(--c-border);
  }
  .topbar__inner {
    max-inline-size: var(--app-max);
    margin-inline: auto;
    padding: 0 var(--app-pad);
    display: grid;
    grid-template-columns: auto 1fr auto;
    align-items: stretch;
    block-size: 44px;
    gap: var(--sp-5);
  }

  .brand {
    display: inline-flex;
    align-items: baseline;
    gap: var(--sp-3);
    align-self: center;
  }
  .brand__word {
    font-family: var(--font-mono);
    font-size: var(--fs-md);
    font-weight: var(--fw-medium);
    letter-spacing: 0.01em;
    color: var(--c-text);
  }
  /* amber budget: role 1 of 5 — the trailing brand cursor. */
  .brand__cursor {
    color: var(--c-accent);
    animation: cursor-blink 1.1s steps(1) infinite;
    font-size: var(--fs-md);
  }
  @keyframes cursor-blink {
    50% { opacity: 0; }
  }
  .brand__version {
    font-size: var(--fs-xs);
    letter-spacing: 0.03em;
  }

  .nav {
    display: inline-flex;
    justify-self: center;
    gap: 0;
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    letter-spacing: 0.02em;
  }
  .nav__tab {
    display: inline-flex;
    align-items: center;
    gap: var(--sp-3);
    padding: 0 var(--sp-5);
    block-size: 44px;
    color: var(--c-text-dim);
    border-block-end: 1.5px solid transparent;
    transition:
      color var(--mo-med) var(--ease-std),
      border-color var(--mo-med) var(--ease-std);
  }
  .nav__tab:hover {
    color: var(--c-text);
  }
  /* amber budget: role 3 of 5 — active tab underline. */
  .nav__tab--active {
    color: var(--c-text);
    border-block-end-color: var(--c-accent);
  }
  .nav__glyph {
    color: currentColor;
    display: inline-grid;
    place-items: center;
  }
  .nav__count {
    color: var(--c-text-faint);
    font-size: var(--fs-xs);
    letter-spacing: 0;
  }
  .nav__tab--active .nav__count {
    color: var(--c-accent-dim);
  }

  /* Subtle vertical rule separating primary tabs (Live/Pool/Metrics) from
     ops tabs (Logs/Models/Tools/Settings). Helps signal the IA divide. */
  .nav__rule {
    inline-size: 1px;
    background: var(--c-border);
    align-self: stretch;
    margin-block: 10px;
    margin-inline: var(--sp-3);
  }

  /* Ops tabs: tighter padding + no count slot, leaving primary tabs
     (Live/Pool) to retain their data-glances. */
  .nav__tab--ops {
    padding: 0 var(--sp-4);
  }
  .nav__tab--ops .nav__glyph {
    color: var(--c-text-faint);
  }
  .nav__tab--ops:hover .nav__glyph,
  .nav__tab--ops.nav__tab--active .nav__glyph {
    color: currentColor;
  }

  .actions {
    display: inline-flex;
    align-items: center;
    gap: var(--sp-3);
    align-self: center;
  }

  /* amber budget: role 2 of 5 — the live pulse in the header. */
  .live-dot {
    display: inline-flex;
    align-items: center;
    gap: 7px;
    letter-spacing: 0.04em;
    text-transform: uppercase;
    font-size: 10.5px;
    color: var(--c-text-dim);
  }
  .live-dot__pulse {
    inline-size: 7px;
    block-size: 7px;
    border-radius: var(--r-pill);
    background: var(--c-accent);
    box-shadow: 0 0 0 0 var(--c-accent);
    animation: live-pulse 1.8s var(--motion-easing) infinite;
  }
  @keyframes live-pulse {
    0%   { box-shadow: 0 0 0 0 color-mix(in oklch, var(--c-accent) 60%, transparent); }
    70%  { box-shadow: 0 0 0 6px transparent; }
    100% { box-shadow: 0 0 0 0 transparent; }
  }

  .search {
    display: inline-flex;
    align-items: center;
    gap: var(--sp-2);
    padding: 0 var(--sp-3);
    min-inline-size: 210px;
    block-size: 26px;
    background: var(--c-surface);
    border: 1px solid var(--c-border);
    border-radius: var(--r-xs);
    color: var(--c-text-dim);
    transition: border-color var(--mo-fast) var(--ease-std);
  }
  .search:focus-within {
    border-color: var(--c-border-strong);
  }
  .search__input {
    flex: 1 1 auto;
    min-inline-size: 0;
    font-size: var(--fs-sm);
    color: var(--c-text);
  }
  .search__input::placeholder {
    color: var(--c-text-faint);
  }

  .btn {
    display: inline-flex;
    align-items: center;
    gap: var(--sp-2);
    block-size: 26px;
    padding: 0 var(--sp-3);
    border: 1px solid var(--c-border-strong);
    border-radius: var(--r-xs);
    color: var(--c-text-dim);
    font-size: var(--fs-sm);
    transition:
      color var(--mo-fast) var(--ease-std),
      border-color var(--mo-fast) var(--ease-std);
  }
  .btn:hover {
    color: var(--c-text);
    border-color: var(--c-text-faint);
  }
  .btn--icon {
    inline-size: 26px;
    padding: 0;
    justify-content: center;
  }
  /* amber budget: role 5 of 5 — ⌘K kbd pill inside the palette button.
     The palette *button* itself stays neutral; only the kbd is amber. */
  .btn--palette {
    color: var(--c-text-dim);
  }
  .btn--palette:hover {
    color: var(--c-text);
  }
  .kbd-amber {
    color: var(--c-accent);
    border: 1px solid color-mix(in oklch, var(--c-accent), transparent 60%);
    background: transparent;
    padding: 0 4px;
    font-size: var(--fs-xs);
    border-radius: var(--r-xs);
    letter-spacing: 0.02em;
  }

  @media (max-width: 1180px) {
    .nav__rule { margin-inline: var(--sp-2); }
    .nav__tab--ops {
      padding: 0 var(--sp-3);
    }
  }
  @media (max-width: 960px) {
    .topbar__inner {
      grid-template-columns: auto 1fr;
      block-size: auto;
      padding-block: var(--sp-3);
      gap: var(--sp-3);
    }
    .nav {
      grid-column: 1 / -1;
      order: 3;
      justify-self: start;
      overflow-x: auto;
    }
    .actions {
      order: 2;
      justify-self: end;
    }
    .nav__tab {
      block-size: 34px;
    }
    .nav__tab--ops > span:nth-of-type(2) {
      /* Hide the text label for ops tabs at narrow widths;
         the glyph alone communicates the destination. */
      display: none;
    }
    .nav__rule { margin-block: 6px; }
    .btn--ghost span,
    .brand__version {
      display: none;
    }
    .search {
      min-inline-size: 140px;
    }
  }
  @media (max-width: 560px) {
    .search {
      min-inline-size: 0;
      max-inline-size: 120px;
    }
    .search__input::placeholder {
      font-size: 10px;
    }
    kbd { display: none; }
    .brand__cursor {
      display: none;
    }
    .nav__tab > span:nth-of-type(2),
    .nav__count {
      display: none;
    }
    .nav__tab--active > span:nth-of-type(2) {
      display: inline;
      font-size: var(--fs-2xs);
    }
    .topbar__inner {
      column-gap: var(--sp-2);
    }
  }

  @media (max-width: 480px) {
    .btn, .search, .nav__tab {
      block-size: auto;
      min-block-size: 44px;
    }
    .btn--icon {
      inline-size: auto;
      min-inline-size: 44px;
    }
  }
</style>
