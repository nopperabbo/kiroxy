<!--
  App — root. Orchestrates:
    - Mounting LiveSource + wiring callbacks to the store
    - Global hotkeys (cmd-k, i, /, ?, Esc)
    - URL-hash sync for filters
    - Layout scaffolding: Topbar, split-pane main, status ribbon
-->
<script lang="ts">
  import { onMount } from "svelte";
  import { LiveSource } from "./lib/live";
  import { store } from "./lib/store.svelte";
  import { readHash, writeHash } from "./lib/urlstate";
  import Topbar from "./components/Topbar.svelte";
  import AccountBoard from "./components/AccountBoard.svelte";
  import RequestStream from "./components/RequestStream.svelte";
  import StatusRibbon from "./components/StatusRibbon.svelte";
  import CommandPalette from "./components/CommandPalette.svelte";
  import ImportDrawer from "./components/ImportDrawer.svelte";
  import DetailDrawer from "./components/DetailDrawer.svelte";
  import ShortcutSheet from "./components/ShortcutSheet.svelte";
  import Toasts from "./components/Toasts.svelte";
  import PoolPulse from "./components/PoolPulse.svelte";
  import ActivityLedger from "./components/ActivityLedger.svelte";

  let paletteOpen = $state(false);
  let importOpen = $state(false);
  let sheetOpen = $state(false);
  let live: LiveSource | null = null;

  onMount(() => {
    // Hydrate filters from URL hash so shareable-view links work.
    const fromHash = readHash();
    if (fromHash.search !== undefined) store.setFilter("search", fromHash.search);
    if (fromHash.onlyErrors !== undefined) store.setFilter("onlyErrors", fromHash.onlyErrors);
    if (fromHash.onlyCooldown !== undefined) store.setFilter("onlyCooldown", fromHash.onlyCooldown);
    if (fromHash.statusRange !== undefined) store.setFilter("statusRange", fromHash.statusRange);

    live = new LiveSource({
      onSnapshot: (s) => store.applySnapshot(s),
      onRequest: (r) => store.appendRequest(r),
      onStatus: (s) => (store.liveStatus = s),
    });
    live.start();

    const onKey = (e: KeyboardEvent): void => {
      const t = e.target as HTMLElement | null;
      const inEdit =
        t && (t.tagName === "INPUT" || t.tagName === "TEXTAREA" || t.isContentEditable);

      if ((e.metaKey || e.ctrlKey) && e.key === "k") {
        e.preventDefault();
        paletteOpen = !paletteOpen;
        return;
      }
      // Esc always works, even while typing in the palette or search.
      if (e.key === "Escape") {
        if (paletteOpen) {
          paletteOpen = false;
          return;
        }
        if (importOpen) {
          importOpen = false;
          return;
        }
        if (sheetOpen) {
          sheetOpen = false;
          return;
        }
        if (store.selectedRequestId || store.selectedAccountId) {
          store.selectRequest(null);
          store.selectAccount(null);
          return;
        }
        if (inEdit) {
          (t as HTMLElement).blur();
        }
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
      }
    };
    window.addEventListener("keydown", onKey);

    return () => {
      window.removeEventListener("keydown", onKey);
      live?.close();
    };
  });

  // URL hash sync — write whenever filters change.
  $effect(() => {
    writeHash(store.filters);
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
    <section class="shell__pulse">
      <PoolPulse />
    </section>
    <section class="shell__board">
      <AccountBoard />
      <ActivityLedger />
    </section>
    <section class="shell__stream">
      <RequestStream />
    </section>
  </main>

  <StatusRibbon />

  <CommandPalette open={paletteOpen} onClose={() => (paletteOpen = false)} onAction={onPaletteAction} />
  <ImportDrawer open={importOpen} onClose={() => (importOpen = false)} />
  <DetailDrawer />
  <ShortcutSheet open={sheetOpen} onClose={() => (sheetOpen = false)} />
  <Toasts />
</div>

<style>
  .shell {
    min-block-size: 100dvh;
    display: grid;
    grid-template-rows: auto 1fr auto;
  }
  .shell__main {
    max-inline-size: var(--app-max);
    inline-size: 100%;
    margin-inline: auto;
    padding: var(--sp-5) var(--app-pad) var(--sp-8);
    display: grid;
    gap: var(--sp-5);
    grid-template-columns: minmax(0, 1fr);
  }
  .shell__pulse {
    grid-column: 1 / -1;
  }
  @media (min-width: 1120px) {
    .shell__main {
      grid-template-columns: minmax(0, 1.35fr) minmax(0, 1fr);
      gap: var(--sp-5) var(--sp-6);
    }
    .shell__pulse {
      grid-column: 1 / -1;
    }
  }
  .shell__board,
  .shell__stream {
    min-inline-size: 0;
    display: flex;
    flex-direction: column;
    gap: var(--sp-5);
  }
</style>
