<!--
  ImportDrawer — paste a JSON array of account entries (same schema as
  kiroxy's CLI import-json). We validate client-side and POST to
  /dashboard/api/import. Phase H backend may not expose this endpoint yet
  (404) — we surface the error honestly rather than pretending it worked.

  Side drawer (not modal) so the pool board remains visible while
  importing. Esc or the X closes it. Cmd-Enter submits.
-->
<script lang="ts">
  import { api } from "../lib/api";
  import { store } from "../lib/store.svelte";
  import type { ImportEntry, ImportResult } from "../lib/types";
  import Icon from "./Icon.svelte";

  interface Props {
    open: boolean;
    onClose: () => void;
  }
  let { open, onClose }: Props = $props();

  let raw = $state("");
  let submitting = $state(false);
  let parseErr: string | null = $state(null);
  let results: ImportResult[] | null = $state(null);
  let textarea: HTMLTextAreaElement | null = $state(null);

  $effect(() => {
    if (open) setTimeout(() => textarea?.focus(), 50);
  });

  function parse(): ImportEntry[] | null {
    parseErr = null;
    if (!raw.trim()) {
      parseErr = "paste a JSON array of account entries";
      return null;
    }
    try {
      const parsed: unknown = JSON.parse(raw);
      if (!Array.isArray(parsed)) {
        parseErr = "expected a JSON array";
        return null;
      }
      const entries: ImportEntry[] = [];
      for (const [i, item] of parsed.entries()) {
        if (!item || typeof item !== "object") {
          parseErr = `entry ${i}: not an object`;
          return null;
        }
        const e = item as Record<string, unknown>;
        if (typeof e.provider !== "string" || typeof e.authMethod !== "string") {
          parseErr = `entry ${i}: provider and authMethod required`;
          return null;
        }
        entries.push({
          provider: e.provider,
          authMethod: e.authMethod,
          accessToken: String(e.accessToken ?? ""),
          refreshToken: String(e.refreshToken ?? ""),
          profileArn: typeof e.profileArn === "string" ? e.profileArn : undefined,
          expiresIn: Number(e.expiresIn ?? 0),
          addedAt: typeof e.addedAt === "string" ? e.addedAt : undefined,
        });
      }
      return entries;
    } catch (err) {
      parseErr = err instanceof Error ? err.message : "parse error";
      return null;
    }
  }

  async function submit(): Promise<void> {
    const entries = parse();
    if (!entries) return;
    submitting = true;
    results = null;
    const res = await api.importAccounts(entries);
    submitting = false;
    if (!res.ok) {
      store.pushToast("err", `import failed: ${res.error}`);
      return;
    }
    results = res.data.results;
    const added = results.filter((r) => r.status === "added").length;
    const skipped = results.length - added;
    store.pushToast("ok", `imported ${added} new${skipped ? `, skipped ${skipped}` : ""}`);
  }

  function onKey(e: KeyboardEvent): void {
    if ((e.metaKey || e.ctrlKey) && e.key === "Enter") {
      e.preventDefault();
      void submit();
    }
  }
</script>

{#if open}
  <div class="drawer-scrim" onclick={onClose} role="presentation"></div>
  <aside class="drawer drawer-panel" role="dialog" aria-modal="true" aria-label="import accounts">
    <header class="drawer__head">
      <div>
        <h2 class="drawer__title">import accounts</h2>
        <p class="drawer__sub faint">
          paste a JSON array — same schema as the <code class="mono">kiroxy add-account --json</code> CLI.
        </p>
      </div>
      <button type="button" class="iconbtn" onclick={onClose} aria-label="close">
        <Icon name="x" size={14} />
      </button>
    </header>
    <div class="drawer__body">
      <label class="drawer__label caps" for="import-json">paste vault export</label>
      <textarea
        id="import-json"
        bind:this={textarea}
        bind:value={raw}
        onkeydown={onKey}
        class="drawer__textarea mono"
        rows="12"
        placeholder={`[\n  {\n    "provider": "IDC",\n    "authMethod": "oauth",\n    "accessToken": "…",\n    "refreshToken": "…",\n    "profileArn": "arn:aws:codewhisperer:…",\n    "expiresIn": 3600\n  }\n]`}
        spellcheck="false"
        autocomplete="off"
      ></textarea>

      {#if parseErr}
        <div class="drawer__err">
          <Icon name="alert" size={14} />
          <span>{parseErr}</span>
        </div>
      {/if}

      {#if results}
        <div class="drawer__results">
          <h3 class="caps">results</h3>
          <ul class="drawer__reslist">
            {#each results as r}
              <li class="drawer__resitem drawer__resitem--{r.status}">
                <span class="mono">{r.id ?? `#${r.index}`}</span>
                <span class="mono faint">{r.status}</span>
                {#if r.reason}<span class="drawer__reasoning faint">{r.reason}</span>{/if}
              </li>
            {/each}
          </ul>
        </div>
      {/if}
    </div>
    <footer class="drawer__foot">
      <button type="button" class="btn btn--ghost" onclick={onClose}>cancel</button>
      <button type="button" class="btn btn--primary" onclick={submit} disabled={submitting}>
        {submitting ? "importing…" : "import"}
        <kbd class="kbd-lite">⌘↩</kbd>
      </button>
    </footer>
  </aside>
{/if}

<style>
  .drawer-scrim {
    position: fixed;
    inset: 0;
    background: color-mix(in oklch, var(--c-bg), transparent 20%);
    z-index: var(--z-drawer);
    animation: fade-in var(--mo-fast);
  }
  .drawer {
    position: fixed;
    inset-block: 0;
    inset-inline-end: 0;
    inline-size: min(520px, 96vw);
    background: var(--c-surface);
    border-inline-start: 1px solid var(--c-border-strong);
    box-shadow: var(--sh-3);
    display: flex;
    flex-direction: column;
    z-index: calc(var(--z-drawer) + 1);
    animation: slide-in var(--mo-med) var(--ease-out);
  }
  @keyframes slide-in {
    from {
      transform: translateX(8%);
      opacity: 0.6;
    }
    to {
      transform: translateX(0);
      opacity: 1;
    }
  }
  .drawer__head {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: var(--sp-5);
    padding: var(--sp-5);
    border-block-end: 1px solid var(--c-rule);
  }
  .drawer__title {
    margin: 0;
    font-size: var(--fs-lg);
    font-family: var(--font-display);
    font-weight: var(--fw-semibold);
  }
  .drawer__sub {
    margin: 2px 0 0;
    font-size: var(--fs-sm);
  }
  .iconbtn {
    display: inline-grid;
    place-items: center;
    inline-size: 28px;
    block-size: 28px;
    border-radius: var(--r-sm);
    color: var(--c-text-dim);
    border: 1px solid transparent;
  }
  .iconbtn:hover {
    color: var(--c-text);
    background: var(--c-surface-hover);
    border-color: var(--c-rule);
  }

  .drawer__body {
    flex: 1 1 auto;
    padding: var(--sp-5);
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: var(--sp-4);
  }
  .drawer__label {
    display: block;
  }
  .drawer__textarea {
    inline-size: 100%;
    resize: vertical;
    min-block-size: 260px;
    font-size: var(--fs-xs);
    line-height: var(--lh-snug);
    padding: var(--sp-3);
    background: var(--c-surface-sunken);
    border: 1px solid var(--c-border);
    border-radius: var(--r-sm);
    color: var(--c-text);
  }
  .drawer__textarea:focus {
    outline: none;
    border-color: var(--c-accent);
  }

  .drawer__err {
    display: flex;
    align-items: center;
    gap: var(--sp-3);
    padding: var(--sp-3) var(--sp-4);
    color: var(--c-danger);
    background: var(--c-danger-bg);
    border: 1px solid color-mix(in oklch, var(--c-danger), transparent 70%);
    border-radius: var(--r-sm);
    font-size: var(--fs-sm);
  }

  .drawer__results h3 {
    margin: 0 0 var(--sp-2);
  }
  .drawer__reslist {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: var(--sp-2);
  }
  .drawer__resitem {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto 1fr;
    align-items: center;
    gap: var(--sp-3);
    padding: var(--sp-2) var(--sp-3);
    border-radius: var(--r-sm);
    border: 1px solid var(--c-rule);
    font-size: var(--fs-xs);
  }
  .drawer__resitem--added {
    color: var(--c-success);
    border-color: color-mix(in oklch, var(--c-success), transparent 70%);
    background: var(--c-success-bg);
  }
  .drawer__resitem--skipped {
    color: var(--c-warn);
  }
  .drawer__reasoning {
    text-align: end;
    font-family: var(--font-mono);
  }

  .drawer__foot {
    display: flex;
    justify-content: flex-end;
    gap: var(--sp-3);
    padding: var(--sp-4) var(--sp-5);
    border-block-start: 1px solid var(--c-rule);
    background: var(--c-surface-sunken);
  }
  .btn {
    display: inline-flex;
    align-items: center;
    gap: var(--sp-2);
    padding: 6px var(--sp-4);
    border: 1px solid var(--c-border);
    border-radius: var(--r-md);
    font-size: var(--fs-sm);
  }
  .btn--ghost {
    color: var(--c-text-dim);
    background: var(--c-surface);
  }
  .btn--ghost:hover {
    color: var(--c-text);
    background: var(--c-surface-hover);
  }
  .btn--primary {
    color: var(--c-bg);
    background: var(--c-accent);
    border-color: var(--c-accent);
    font-weight: var(--fw-semibold);
  }
  .btn--primary:hover {
    background: var(--c-accent-strong);
    border-color: var(--c-accent-strong);
  }
  .btn--primary:disabled {
    opacity: 0.55;
    cursor: not-allowed;
  }
  .kbd-lite {
    padding: 1px var(--sp-2);
    font-size: var(--fs-2xs);
    color: color-mix(in oklch, var(--c-bg), transparent 30%);
    background: transparent;
    border: 1px solid color-mix(in oklch, var(--c-bg), transparent 50%);
    border-radius: var(--r-sm);
  }
</style>
