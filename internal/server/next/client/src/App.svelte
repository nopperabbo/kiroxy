<!--
  App — root component. Owns:
    - Global SSE connection lifecycle
    - Global hotkeys (cmd-k, i, esc)
    - Modal open/close state (palette + import)
    - Layout container + footer
    - Toast surface
-->
<script lang="ts">
  import { onMount } from "svelte";
  import { Sse } from "./lib/sse";
  import { store } from "./lib/stores.svelte";
  import { api } from "./lib/api";
  import HealthBar from "./components/HealthBar.svelte";
  import AccountTable from "./components/AccountTable.svelte";
  import RequestFeed from "./components/RequestFeed.svelte";
  import CommandPalette from "./components/CommandPalette.svelte";
  import ImportModal from "./components/ImportModal.svelte";
  import ThemeToggle from "./components/ThemeToggle.svelte";
  import Icon from "./components/icons/Icon.svelte";

  let paletteOpen = $state(false);
  let importOpen = $state(false);
  let cheatOpen = $state(false);

  const paletteActions = [
    { id: "import", label: "import accounts", hint: "open modal" },
    { id: "reload", label: "reload snapshot", hint: "fetch /api/state" },
    { id: "toggle-theme", label: "toggle theme", hint: "system / dark / light" },
    { id: "open-dashboard-v1", label: "open dashboard v1", hint: "/dashboard" },
    { id: "copy-curl", label: "copy curl for /dashboard/api/state", hint: "clipboard" },
  ];

  function handlePaletteAction(id: string): void {
    if (id === "action:import") {
      paletteOpen = false;
      importOpen = true;
    } else if (id === "action:reload") {
      paletteOpen = false;
      void (async () => {
        const [s, r] = await Promise.all([api.state(), api.requests()]);
        if (s.ok) store.applySnapshot(s.data);
        if (r.ok) store.replaceRequests(r.data);
        store.pushToast("ok", "reloaded");
      })();
    } else if (id === "action:toggle-theme") {
      paletteOpen = false;
      // Let ThemeToggle handle it via a click simulation.
      document.querySelector<HTMLButtonElement>(".themebtn")?.click();
    } else if (id === "action:open-dashboard-v1") {
      paletteOpen = false;
      window.location.assign("/dashboard");
    } else if (id === "action:copy-curl") {
      paletteOpen = false;
      const origin = window.location.origin;
      const cmd = `curl -s ${origin}/dashboard/api/state | jq`;
      void navigator.clipboard.writeText(cmd).then(
        () => store.pushToast("ok", "curl copied"),
        () => store.pushToast("err", "clipboard denied"),
      );
    } else if (id.startsWith("account:")) {
      paletteOpen = false;
      // Scroll to the account row.
      const acctId = id.slice("account:".length);
      document
        .querySelector(`[data-account-id="${CSS.escape(acctId)}"]`)
        ?.scrollIntoView({ behavior: "smooth", block: "center" });
    }
    // request: rows just close the palette
    if (id.startsWith("request:")) {
      paletteOpen = false;
    }
  }

  onMount(() => {
    // Prime requests cache (SSE only sends NEW ones).
    void api.requests().then((r) => {
      if (r.ok) store.replaceRequests(r.data);
    });

    const sse = new Sse({
      onSnapshot: (s) => store.applySnapshot(s),
      onRequest: (r) => store.appendRequest(r),
      onStatus: (s) => (store.sseStatus = s),
    });
    sse.start();

    const onKey = (e: KeyboardEvent): void => {
      // Respect inputs — only react when not in an editable context.
      const target = e.target as HTMLElement | null;
      const inEditable =
        target &&
        (target.tagName === "INPUT" ||
          target.tagName === "TEXTAREA" ||
          target.isContentEditable);

      if ((e.metaKey || e.ctrlKey) && e.key === "k") {
        e.preventDefault();
        paletteOpen = !paletteOpen;
        return;
      }
      if (inEditable) return;

      if (e.key === "i" && !e.metaKey && !e.ctrlKey && !e.altKey) {
        e.preventDefault();
        importOpen = true;
      } else if (e.key === "?") {
        e.preventDefault();
        cheatOpen = !cheatOpen;
      } else if (e.key === "Escape") {
        if (paletteOpen) paletteOpen = false;
        else if (importOpen) importOpen = false;
        else if (cheatOpen) cheatOpen = false;
      }
    };
    window.addEventListener("keydown", onKey);

    return () => {
      window.removeEventListener("keydown", onKey);
      sse.close();
    };
  });
</script>

<div class="app">
  <HealthBar />

  <main class="app__main" id="main">
    <AccountTable />
    <RequestFeed />
  </main>

  <footer class="app__foot">
    <div class="app__hints">
      <span><kbd>⌘</kbd><kbd>K</kbd> palette</span>
      <span><kbd>i</kbd> import</span>
      <span><kbd>?</kbd> keys</span>
    </div>
    <div class="app__brand">
      <ThemeToggle />
      <span class="app__brand-text faint">kiroxy dashboard next · MIT</span>
    </div>
  </footer>

  <CommandPalette
    open={paletteOpen}
    onClose={() => (paletteOpen = false)}
    onAction={handlePaletteAction}
    actions={paletteActions}
  />
  <ImportModal open={importOpen} onClose={() => (importOpen = false)} />

  {#if cheatOpen}
    <dialog class="cheat" open>
      <div class="cheat__box">
        <div class="cheat__head">
          <h3>keyboard shortcuts</h3>
          <button
            type="button"
            class="iconbtn"
            aria-label="close"
            onclick={() => (cheatOpen = false)}><Icon name="x" /></button
          >
        </div>
        <dl class="cheat__list">
          <dt><kbd>⌘</kbd><kbd>K</kbd></dt>
          <dd>open / close command palette</dd>
          <dt><kbd>i</kbd></dt>
          <dd>open import accounts modal</dd>
          <dt><kbd>?</kbd></dt>
          <dd>show this cheat sheet</dd>
          <dt><kbd>↑</kbd><kbd>↓</kbd></dt>
          <dd>navigate palette results</dd>
          <dt><kbd>Enter</kbd></dt>
          <dd>execute selected palette item</dd>
          <dt><kbd>Esc</kbd></dt>
          <dd>close overlay</dd>
          <dt><kbd>Tab</kbd></dt>
          <dd>move focus between rows and controls</dd>
        </dl>
      </div>
    </dialog>
  {/if}

  {#if store.toasts.length > 0}
    <div class="toasts" aria-live="polite">
      {#each store.toasts as t (t.id)}
        <div class="toast toast--{t.kind}">
          <Icon name={t.kind === "ok" ? "check" : "alert"} size={12} />
          <span>{t.msg}</span>
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  .app {
    min-block-size: 100vh;
    display: flex;
    flex-direction: column;
    container-type: inline-size;
  }
  .app__main {
    flex: 1 1 auto;
    padding-block: var(--sp-6);
    display: flex;
    flex-direction: column;
    gap: var(--sp-7);
  }
  .app__foot {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--sp-5);
    padding: var(--sp-4) var(--sp-6);
    border-block-start: 1px solid var(--c-border);
    background: var(--c-surface);
    font-size: var(--fs-xs);
    flex-wrap: wrap;
  }
  .app__hints {
    display: flex;
    gap: var(--sp-5);
    color: var(--c-text-faint);
  }
  .app__brand {
    display: inline-flex;
    align-items: center;
    gap: var(--sp-4);
  }
  .app__brand-text {
    color: var(--c-text-faint);
  }
  .faint {
    color: var(--c-text-faint);
  }

  .cheat {
    inline-size: min(420px, 92vw);
    border: 1px solid var(--c-border-strong);
    border-radius: var(--r-lg);
    background: var(--c-surface);
    color: var(--c-text);
    padding: 0;
    margin: 20vh auto auto;
    box-shadow: 0 8px 32px color-mix(in oklch, var(--c-bg), transparent 20%);
  }
  .cheat__box {
    padding: 0;
  }
  .cheat__head {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: var(--sp-4) var(--sp-5);
    border-block-end: 1px solid var(--c-border);
  }
  .cheat__head h3 {
    margin: 0;
    font-size: var(--fs-md);
    font-weight: var(--fw-semibold);
  }
  .cheat__list {
    display: grid;
    grid-template-columns: max-content 1fr;
    gap: var(--sp-3) var(--sp-5);
    padding: var(--sp-5);
    margin: 0;
  }
  .cheat__list dt {
    display: inline-flex;
    gap: var(--sp-2);
  }
  .cheat__list dd {
    margin: 0;
    color: var(--c-text-dim);
    font-size: var(--fs-sm);
  }

  .iconbtn {
    display: inline-grid;
    place-items: center;
    inline-size: 24px;
    block-size: 24px;
    border-radius: var(--r-sm);
    color: var(--c-text-dim);
  }
  .iconbtn:hover {
    background: var(--c-surface-hover);
    color: var(--c-text);
  }

  .toasts {
    position: fixed;
    inset-block-end: var(--sp-6);
    inset-inline-end: var(--sp-6);
    display: flex;
    flex-direction: column;
    gap: var(--sp-3);
    z-index: var(--z-toast);
    pointer-events: none;
  }
  .toast {
    display: inline-flex;
    align-items: center;
    gap: var(--sp-3);
    padding: var(--sp-3) var(--sp-4);
    font-size: var(--fs-sm);
    border-radius: var(--r-sm);
    border: 1px solid var(--c-border);
    background: var(--c-surface);
    color: var(--c-text);
    box-shadow: 0 4px 16px color-mix(in oklch, var(--c-bg), transparent 60%);
    pointer-events: auto;
  }
  .toast--ok {
    color: var(--c-success);
    border-color: color-mix(in oklch, var(--c-success), transparent 70%);
  }
  .toast--err {
    color: var(--c-danger);
    border-color: color-mix(in oklch, var(--c-danger), transparent 60%);
  }

  @starting-style {
    .toast {
      opacity: 0;
      transform: translateY(6px);
    }
  }
  .toast {
    transition:
      opacity var(--mo-med) var(--ease-out),
      transform var(--mo-med) var(--ease-out);
  }
</style>
