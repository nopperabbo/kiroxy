<!-- src/components/Guide.svelte -->
<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { store } from "../lib/store.svelte";

  const WIZARD_KEY = "kiroxy:wizard-dismissed";
  const GUIDE_KEY = "kiroxy:guide-dismissed";

  let visible = $state(false);

  // Derived state: guide auto-dismisses permanently the moment end-to-end traffic works.
  let hasTraffic = $derived(store.requests.length > 0);

  $effect(() => {
    if (visible && hasTraffic) {
      dismiss();
    }
  });

  onMount(() => {
    try {
      const wizardDone = localStorage.getItem(WIZARD_KEY) === "1";
      const guideDone = localStorage.getItem(GUIDE_KEY) === "1";
      if (wizardDone && !guideDone && !hasTraffic) {
        visible = true;
      }
    } catch {
      // Silent on localStorage denial
    }
    window.addEventListener("keydown", onKey);
  });

  onDestroy(() => {
    window.removeEventListener("keydown", onKey);
  });

  function onKey(e: KeyboardEvent): void {
    if (!visible) return;
    if (e.key === "Escape") {
      e.preventDefault();
      dismiss();
    }
  }

  function dismiss(): void {
    try {
      localStorage.setItem(GUIDE_KEY, "1");
    } catch {}
    visible = false;
  }

  function importAccount(): void {
    window.dispatchEvent(new CustomEvent("kiroxy:open-import"));
  }

  function copyText(text: string, msg: string): void {
    void navigator.clipboard.writeText(text).then(() => {
      store.pushToast("ok", msg);
    }).catch(() => {
      store.pushToast("err", "clipboard denied");
    });
  }

  const CURL_TEXT = 'curl -H "x-api-key: $KIROXY_API_KEY" http://127.0.0.1:8787/v1/models';
</script>

{#if visible}
  <aside class="guide" role="complementary" aria-label="initial setup guide">
    <header class="guide__head">
      <h2 class="guide__title caps">Awaiting initial telemetry.</h2>
      <button type="button" class="close-btn mono" onclick={dismiss} title="close (ESC)">
        <span>ESC</span><span class="faint">· dismiss</span>
      </button>
    </header>
    
    <ol class="guide__steps">
      <li class="guide__step">
        <span class="guide__text">1. Mount an identity</span>
        <button type="button" class="btn btn--accent" onclick={importAccount}>Import account</button>
      </li>
      <li class="guide__step">
        <span class="guide__text">2. Point your client to localhost:8787</span>
        <button type="button" class="btn btn--ghost" onclick={() => copyText("http://localhost:8787", "URL copied")}>Copy URL</button>
      </li>
      <li class="guide__step">
        <span class="guide__text">3. Send a test request</span>
        <button type="button" class="btn btn--ghost" onclick={() => copyText(CURL_TEXT, "curl command copied")}>Copy curl</button>
      </li>
      <li class="guide__step guide__step--philo">
        <span class="guide__text">4. The stream wakes when the wire catches traffic.</span>
      </li>
    </ol>
  </aside>
{/if}

<style>
  .guide {
    position: fixed;
    inset-block-end: var(--sp-5);
    inset-inline-end: var(--sp-5);
    inline-size: 360px;
    max-inline-size: calc(100vw - var(--sp-6));
    background: var(--c-surface);
    border: 1px solid var(--c-border);
    border-radius: var(--r-md);
    z-index: var(--z-guide, 70); /* Below drawer (80), above sticky headers (50) */
    display: flex;
    flex-direction: column;
    animation: fade-in var(--mo-med) var(--ease-out);
  }

  .guide__head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: var(--sp-3) var(--sp-4);
    border-block-end: 1px solid var(--c-rule);
  }

  .guide__title {
    margin: 0;
    color: var(--c-text-dim);
    font-size: 10.5px;
    letter-spacing: var(--tr-caps);
  }

  .close-btn {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    color: var(--c-text-faint);
    font-size: 10.5px;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    padding: 2px 4px;
    border-radius: var(--r-xs);
    transition: color var(--mo-fast) var(--ease-std);
  }
  .close-btn:hover { color: var(--c-text); }

  .guide__steps {
    list-style: none;
    margin: 0;
    padding: var(--sp-3) var(--sp-4) var(--sp-4);
    display: flex;
    flex-direction: column;
    gap: var(--sp-3);
  }

  .guide__step {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--sp-3);
    font-size: var(--fs-sm);
    color: var(--c-text);
  }

  .guide__step--philo {
    color: var(--c-text-faint);
    margin-block-start: var(--sp-2);
  }

  .guide__text {
    flex: 1;
    line-height: var(--lh-snug);
  }

  .btn {
    display: inline-flex;
    align-items: center;
    padding: 4px 8px;
    font-size: var(--fs-xs);
    font-family: var(--font-mono);
    border-radius: var(--r-sm);
    border: 1px solid transparent;
    transition:
      color var(--mo-fast) var(--ease-std),
      background var(--mo-fast) var(--ease-std),
      border-color var(--mo-fast) var(--ease-std);
    cursor: pointer;
  }

  .btn--ghost {
    color: var(--c-text-dim);
    background: var(--c-surface-sunken);
    border-color: var(--c-rule);
  }
  .btn--ghost:hover {
    color: var(--c-text);
    background: var(--c-surface-hover);
    border-color: var(--c-border);
  }

  .btn--accent {
    color: var(--c-accent);
    background: var(--c-accent-wash);
    border-color: color-mix(in oklch, var(--c-accent), transparent 50%);
  }
  .btn--accent:hover {
    color: var(--c-accent-strong);
    background: color-mix(in oklch, var(--c-accent-wash), transparent 20%);
    border-color: var(--c-accent-strong);
  }

  @media (prefers-reduced-motion: reduce) {
    .guide {
      animation: none;
    }
  }
</style>
