<!--
  App — root. Orchestrates:
    - Mounting LiveSource + wiring callbacks to the store
    - Global hotkeys (⌘K, i, /, ?, Space, Esc)
    - URL-hash sync for filters
    - Three top-level views as tabs: Live Stream / Pool / Metrics
      (Topbar owns the tab nav; App just renders whichever view is active.)
    - View Transitions API cross-fade between tab switches
-->
<script lang="ts">
  import { onMount } from "svelte";
  import { LiveSource } from "./lib/live";
  import { store } from "./lib/store.svelte";
  import { readHash, writeHash, viewFromHash } from "./lib/urlstate";
  import Topbar from "./components/Topbar.svelte";
  import AccountBoard from "./components/AccountBoard.svelte";
  import StatusRibbon from "./components/StatusRibbon.svelte";
  import CommandPalette from "./components/CommandPalette.svelte";
  import ImportDrawer from "./components/ImportDrawer.svelte";
  import DetailDrawer from "./components/DetailDrawer.svelte";
  import ShortcutSheet from "./components/ShortcutSheet.svelte";
  import Toasts from "./components/Toasts.svelte";
  import PoolPulse from "./components/PoolPulse.svelte";
  import ActivityLedger from "./components/ActivityLedger.svelte";
  import LiveStream from "./components/LiveStream.svelte";
  import LiveRail from "./components/LiveRail.svelte";
  import MetricsView from "./components/MetricsView.svelte";
  import LogsView from "./components/LogsView.svelte";
  import SettingsView from "./components/SettingsView.svelte";
  import ToolsView from "./components/ToolsView.svelte";
  import ModelsView from "./components/ModelsView.svelte";
  import Guide from "./components/Guide.svelte";

  let paletteOpen = $state(false);
  let importOpen = $state(false);
  let sheetOpen = $state(false);
  let live: LiveSource | null = null;

  onMount(() => {
    const fromHash = readHash();
    if (fromHash.search !== undefined) store.setFilter("search", fromHash.search);
    if (fromHash.onlyErrors !== undefined) store.setFilter("onlyErrors", fromHash.onlyErrors);
    if (fromHash.onlyCooldown !== undefined) store.setFilter("onlyCooldown", fromHash.onlyCooldown);
    if (fromHash.statusRange !== undefined) store.setFilter("statusRange", fromHash.statusRange);
    const initialView = viewFromHash(window.location.hash);
    if (initialView) store.setView(initialView);

    live = new LiveSource({
      onSnapshot: (s) => store.applySnapshot(s),
      onRequest: (r) => {
        // Respect stream pause: snapshot data still arrives (so pool stats
        // stay current), but the request ring ignores new rows while paused.
        if (!store.streamPaused) store.appendRequest(r);
      },
      onStatus: (s) => (store.liveStatus = s),
    });
    live.start();
    store.reconnectLive = () => {
      live?.close();
      live = new LiveSource({
        onSnapshot: (s) => store.applySnapshot(s),
        onRequest: (r) => {
          if (!store.streamPaused) store.appendRequest(r);
        },
        onStatus: (s) => (store.liveStatus = s),
      });
      live.start();
    };

    const onKey = (e: KeyboardEvent): void => {
      const t = e.target as HTMLElement | null;
      const inEdit =
        t && (t.tagName === "INPUT" || t.tagName === "TEXTAREA" || t.isContentEditable);

      if ((e.metaKey || e.ctrlKey) && e.key === "k") {
        e.preventDefault();
        paletteOpen = !paletteOpen;
        return;
      }
      if (e.key === "Escape") {
        if (paletteOpen) { paletteOpen = false; return; }
        if (importOpen) { importOpen = false; return; }
        if (sheetOpen) { sheetOpen = false; return; }
        if (store.selectedRequestId || store.selectedAccountId) {
          store.selectRequest(null);
          store.selectAccount(null);
          return;
        }
        if (inEdit) (t as HTMLElement).blur();
        return;
      }
      if (inEdit) return;
      if (e.key === "/") {
        e.preventDefault();
        document.querySelector<HTMLInputElement>('[data-search-input]')?.focus();
      } else if (e.key === "i") {
        e.preventDefault();
        importOpen = true;
      } else if (e.key === "?") {
        e.preventDefault();
        sheetOpen = !sheetOpen;
      } else if (e.key === " ") {
        // Space pauses the request feed when we're on Live.
        if (store.view === "live") {
          e.preventDefault();
          store.togglePause();
        }
      }
    };
    window.addEventListener("keydown", onKey);

    return () => {
      window.removeEventListener("keydown", onKey);
      live?.close();
    };
  });

  $effect(() => {
    writeHash(store.filters, store.view);
  });

  function onPaletteAction(id: string): void {
    if (id === "action:reload") {
      paletteOpen = false;
      void live?.refreshNow().then(() => store.pushToast("ok", "reloaded"));
    } else if (id === "action:import") {
      paletteOpen = false;
      importOpen = true;
    } else if (id === "action:dashboard-next") {
      paletteOpen = false;
      window.location.assign("/dashboard-next");
    } else if (id === "action:dashboard-v1") {
      paletteOpen = false;
      window.location.assign("/dashboard");
    } else if (id === "action:copy-curl") {
      paletteOpen = false;
      const cmd = `curl -s ${window.location.origin}/dashboard/api/state | jq`;
      void navigator.clipboard.writeText(cmd).then(
        () => store.pushToast("ok", "curl copied"),
        () => store.pushToast("err", "clipboard denied"),
      );
    } else if (id === "action:copy-link") {
      paletteOpen = false;
      const url = window.location.href;
      void navigator.clipboard.writeText(url).then(
        () => store.pushToast("ok", "shareable link copied"),
        () => store.pushToast("err", "clipboard denied"),
      );
    } else if (id === "action:clear-filters") {
      paletteOpen = false;
      store.setFilter("search", "");
      store.setFilter("onlyErrors", false);
      store.setFilter("onlyCooldown", false);
      store.setFilter("statusRange", "all");
      store.pushToast("ok", "filters cleared");
    } else if (
      id === "view:live" ||
      id === "view:pool" ||
      id === "view:metrics" ||
      id === "view:logs" ||
      id === "view:settings" ||
      id === "view:tools" ||
      id === "view:models"
    ) {
      paletteOpen = false;
      const v = id.slice("view:".length) as
        | "live"
        | "pool"
        | "metrics"
        | "logs"
        | "settings"
        | "tools"
        | "models";
      type VtDoc = Document & { startViewTransition?: (cb: () => void) => void };
      const d = document as VtDoc;
      if (d.startViewTransition) d.startViewTransition(() => store.setView(v));
      else store.setView(v);
    } else if (id === "action:pause-feed") {
      paletteOpen = false;
      store.togglePause();
    } else if (id.startsWith("account:")) {
      paletteOpen = false;
      store.selectAccount(id.slice("account:".length));
    } else if (id.startsWith("request:")) {
      paletteOpen = false;
      store.selectRequest(id.slice("request:".length));
    }
  }
</script>

<div class="shell">
  <Topbar onOpenPalette={() => (paletteOpen = true)} onOpenImport={() => (importOpen = true)} />

  <main class="shell__main" id="main">
    {#if store.view === "live"}
      <LiveStream>
        {#snippet rail()}<LiveRail />{/snippet}
      </LiveStream>
    {:else if store.view === "pool"}
      <section class="view view--pool" id="view-pool" aria-label="account pool">
        <div class="view__pool-inner">
          <PoolPulse />
          <AccountBoard />
          <ActivityLedger />
        </div>
      </section>
    {:else if store.view === "metrics"}
      <MetricsView />
    {:else if store.view === "logs"}
      <section class="view view--utility" aria-label="logs">
        <div class="view__utility-inner">
          <LogsView />
        </div>
      </section>
    {:else if store.view === "settings"}
      <section class="view view--utility" aria-label="settings">
        <div class="view__utility-inner">
          <SettingsView />
        </div>
      </section>
    {:else if store.view === "tools"}
      <section class="view view--utility" aria-label="tools">
        <div class="view__utility-inner">
          <ToolsView />
        </div>
      </section>
    {:else if store.view === "models"}
      <section class="view view--utility" aria-label="models">
        <div class="view__utility-inner">
          <ModelsView />
        </div>
      </section>
    {/if}
  </main>

  <StatusRibbon />

  <CommandPalette open={paletteOpen} onClose={() => (paletteOpen = false)} onAction={onPaletteAction} />
  <ImportDrawer open={importOpen} onClose={() => (importOpen = false)} />
  <DetailDrawer />
  <ShortcutSheet open={sheetOpen} onClose={() => (sheetOpen = false)} />
  <Guide />
  <Toasts />
</div>

<style>
  .shell {
    min-block-size: 100dvh;
    max-inline-size: 100vw;
    overflow-x: clip;
    display: grid;
    grid-template-rows: auto 1fr auto;
  }
  .shell__main {
    min-block-size: 0;
    display: flex;
    flex-direction: column;
  }
  .view--pool {
    flex: 1;
    min-block-size: 0;
    overflow-y: auto;
    view-transition-name: main-view;
  }
  .view__pool-inner {
    max-inline-size: var(--app-max);
    inline-size: 100%;
    margin-inline: auto;
    padding: var(--sp-5) var(--app-pad) var(--sp-8);
    display: flex;
    flex-direction: column;
    gap: var(--sp-5);
  }
  .view--utility {
    flex: 1;
    min-block-size: 0;
    overflow-y: auto;
    view-transition-name: main-view;
  }
  .view__utility-inner {
    max-inline-size: var(--app-max);
    inline-size: 100%;
    margin-inline: auto;
    padding: var(--sp-5) var(--app-pad) var(--sp-8);
    display: flex;
    flex-direction: column;
    gap: var(--sp-5);
  }

  /* View Transitions — 240ms cross-fade the three top-level views. */
  :global(::view-transition-old(main-view)),
  :global(::view-transition-new(main-view)) {
    animation-duration: var(--motion-duration);
    animation-timing-function: var(--motion-easing);
  }
</style>
