<!--
  ImportModal — drag-drop OR paste JSON array of kiroTokenEntry.
  Validates client-side before POST /dashboard/api/import.
-->
<script lang="ts">
  import Icon from "./icons/Icon.svelte";
  import { api } from "../lib/api";
  import { store } from "../lib/stores.svelte";
  import type { ImportEntry, ImportResult } from "../lib/types";

  interface Props {
    open: boolean;
    onClose: () => void;
  }
  let { open, onClose }: Props = $props();

  let dialogEl: HTMLDialogElement | undefined = $state();
  let raw = $state("");
  let dragActive = $state(false);
  let submitting = $state(false);
  let results = $state<ImportResult[] | null>(null);

  interface ParsedRow {
    ok: boolean;
    index: number;
    provider?: string;
    id?: string;
    authMethod?: string;
    error?: string;
  }

  const parsed = $derived.by<{
    entries: ImportEntry[];
    rows: ParsedRow[];
    globalError: string | null;
  }>(() => {
    if (!raw.trim()) return { entries: [], rows: [], globalError: null };
    let data: unknown;
    try {
      data = JSON.parse(raw);
    } catch (e) {
      return {
        entries: [],
        rows: [],
        globalError: `JSON parse error: ${e instanceof Error ? e.message : String(e)}`,
      };
    }
    if (!Array.isArray(data)) {
      return { entries: [], rows: [], globalError: "expected a JSON array" };
    }
    const entries: ImportEntry[] = [];
    const rows: ParsedRow[] = [];
    for (let i = 0; i < data.length; i++) {
      const r = validateEntry(data[i], i);
      if (r.ok && r.entry) {
        entries.push(r.entry);
        rows.push({
          ok: true,
          index: i,
          provider: r.entry.provider,
          ...(r.id !== undefined ? { id: r.id } : {}),
          authMethod: r.entry.authMethod,
        });
      } else {
        rows.push({
          ok: false,
          index: i,
          error: r.error || "invalid entry",
        });
      }
    }
    return { entries, rows, globalError: null };
  });

  const hasErrors = $derived(parsed.rows.some((r) => !r.ok));
  const canSubmit = $derived(
    !submitting && parsed.entries.length > 0 && parsed.globalError === null,
  );

  interface ValidatedEntry {
    ok: boolean;
    entry?: ImportEntry;
    id?: string;
    error?: string;
  }

  function validateEntry(raw: unknown, _idx: number): ValidatedEntry {
    if (!raw || typeof raw !== "object") return { ok: false, error: "not an object" };
    const v = raw as Record<string, unknown>;
    const provider = asStr(v.provider);
    const authMethod = asStr(v.authMethod);
    const accessToken = asStr(v.accessToken);
    const refreshToken = asStr(v.refreshToken);
    const expiresIn = typeof v.expiresIn === "number" ? v.expiresIn : 0;

    if (!provider) return { ok: false, error: "provider required" };
    if (!authMethod) return { ok: false, error: "authMethod required" };
    if (!accessToken) return { ok: false, error: "accessToken required" };
    if (!refreshToken) return { ok: false, error: "refreshToken required" };

    const profileArn = asStr(v.profileArn);
    const addedAt = asStr(v.addedAt);

    const entry: ImportEntry = {
      provider,
      authMethod,
      accessToken,
      refreshToken,
      expiresIn,
    };
    if (profileArn) entry.profileArn = profileArn;
    if (addedAt) entry.addedAt = addedAt;

    const idHint = profileArn ? profileArn.split("/").pop() || "" : "";
    return { ok: true, entry, id: idHint };
  }

  function asStr(v: unknown): string {
    return typeof v === "string" ? v : "";
  }

  $effect(() => {
    if (open) queueMicrotask(() => dialogEl?.showModal());
    else dialogEl?.close();
  });

  async function handleSubmit(): Promise<void> {
    if (!canSubmit) return;
    submitting = true;
    results = null;
    const res = await api.importAccounts(parsed.entries);
    submitting = false;
    if (res.ok) {
      results = res.data.results;
      const added = res.data.results.filter((r) => r.status === "added").length;
      const updated = res.data.results.filter((r) => r.status === "updated").length;
      const skipped = res.data.results.filter((r) => r.status === "skipped").length;
      store.pushToast(
        "ok",
        `imported — ${added} added · ${updated} updated · ${skipped} skipped`,
      );
    } else {
      store.pushToast("err", `import failed: ${res.error}`);
    }
  }

  function handleDrop(e: DragEvent): void {
    e.preventDefault();
    dragActive = false;
    const file = e.dataTransfer?.files?.[0];
    if (!file) return;
    const reader = new FileReader();
    reader.onload = () => {
      if (typeof reader.result === "string") raw = reader.result;
    };
    reader.readAsText(file);
  }

  function handleDragOver(e: DragEvent): void {
    e.preventDefault();
    dragActive = true;
  }

  function handleDragLeave(): void {
    dragActive = false;
  }

  function reset(): void {
    raw = "";
    results = null;
  }
</script>

<dialog bind:this={dialogEl} class="imp" onclose={onClose}>
  <div class="imp__head">
    <h3 class="imp__title">import accounts</h3>
    <button type="button" class="iconbtn" aria-label="close" onclick={onClose}>
      <Icon name="x" />
    </button>
  </div>

  <div
    class="imp__drop"
    class:imp__drop--active={dragActive}
    ondragover={handleDragOver}
    ondragleave={handleDragLeave}
    ondrop={handleDrop}
    role="region"
    aria-label="drop zone"
  >
    <p class="imp__drop-text faint">drop a JSON file here, or paste below</p>
  </div>

  <textarea
    class="imp__textarea mono"
    bind:value={raw}
    placeholder={`[\n  {\n    "provider": "kiro",\n    "authMethod": "social",\n    "accessToken": "…",\n    "refreshToken": "…",\n    "profileArn": "arn:aws:…",\n    "expiresIn": 3600\n  }\n]`}
    aria-label="paste JSON token entries"
    rows={10}
  ></textarea>

  {#if parsed.globalError}
    <div class="imp__error" role="alert">{parsed.globalError}</div>
  {/if}

  {#if parsed.rows.length > 0}
    <div class="imp__preview">
      <h4 class="imp__preview-title">preview · {parsed.rows.length} entries</h4>
      <ul class="imp__rows">
        {#each parsed.rows as row (row.index)}
          <li class="imp__row" class:imp__row--err={!row.ok}>
            <span class="imp__idx mono">#{row.index + 1}</span>
            {#if row.ok}
              <span class="mono">{row.provider}</span>
              <span class="mono faint">{row.authMethod}</span>
              <span class="mono faint">{row.id || "—"}</span>
            {:else}
              <span class="imp__row-err mono">{row.error}</span>
            {/if}
          </li>
        {/each}
      </ul>
    </div>
  {/if}

  {#if results}
    <div class="imp__results">
      <h4 class="imp__preview-title">server response</h4>
      <ul class="imp__rows">
        {#each results as r (r.index)}
          <li class="imp__row imp__row--r-{r.status}">
            <span class="imp__idx mono">#{r.index + 1}</span>
            <span class="mono">{r.status}</span>
            <span class="mono faint">{r.id ?? ""}</span>
            {#if r.reason}
              <span class="mono imp__row-err">{r.reason}</span>
            {/if}
          </li>
        {/each}
      </ul>
    </div>
  {/if}

  <div class="imp__actions">
    {#if raw || results}
      <button type="button" class="btn" onclick={reset}>clear</button>
    {/if}
    <button
      type="button"
      class="btn btn--primary"
      disabled={!canSubmit}
      onclick={() => void handleSubmit()}
    >
      {submitting ? "importing…" : `import ${parsed.entries.length || ""}`}
    </button>
  </div>

  {#if hasErrors && !parsed.globalError}
    <p class="imp__hint faint">
      entries with errors will be skipped; server does authoritative validation.
    </p>
  {/if}
</dialog>

<style>
  .imp {
    inline-size: min(640px, 92vw);
    max-block-size: 86vh;
    border: 1px solid var(--c-border-strong);
    border-radius: var(--r-lg);
    background: var(--c-surface);
    color: var(--c-text);
    padding: 0;
    overflow-y: auto;
  }
  .imp::backdrop {
    background: color-mix(in oklch, var(--c-bg), transparent 20%);
    backdrop-filter: blur(3px);
  }

  .imp__head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: var(--sp-4) var(--sp-5);
    border-block-end: 1px solid var(--c-border);
  }
  .imp__title {
    margin: 0;
    font-size: var(--fs-md);
    font-weight: var(--fw-semibold);
  }

  .imp__drop {
    margin: var(--sp-5);
    padding: var(--sp-5);
    border: 1.5px dashed var(--c-border-strong);
    border-radius: var(--r-md);
    text-align: center;
    transition:
      background var(--mo-fast),
      border-color var(--mo-fast);
  }
  .imp__drop--active {
    background: var(--c-accent-bg);
    border-color: var(--c-accent);
  }
  .imp__drop-text {
    margin: 0;
    font-size: var(--fs-sm);
  }

  .imp__textarea {
    display: block;
    inline-size: calc(100% - var(--sp-5) * 2);
    margin: 0 var(--sp-5) var(--sp-4);
    padding: var(--sp-4);
    min-block-size: 140px;
    field-sizing: content;
    border: 1px solid var(--c-border);
    border-radius: var(--r-sm);
    background: var(--c-bg);
    color: var(--c-text);
    font-size: var(--fs-xs);
    line-height: var(--lh-normal);
    resize: vertical;
  }
  .imp__textarea:focus {
    outline: 2px solid var(--c-focus);
    outline-offset: 1px;
  }

  .imp__error {
    margin: 0 var(--sp-5) var(--sp-4);
    padding: var(--sp-3) var(--sp-4);
    background: var(--c-danger-bg);
    color: var(--c-danger);
    border: 1px solid color-mix(in oklch, var(--c-danger), transparent 60%);
    border-radius: var(--r-sm);
    font-size: var(--fs-sm);
    font-family: var(--font-mono);
  }

  .imp__preview,
  .imp__results {
    margin: 0 var(--sp-5) var(--sp-4);
  }
  .imp__preview-title {
    margin: 0 0 var(--sp-3);
    font-size: var(--fs-xs);
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--c-text-faint);
    font-weight: var(--fw-medium);
  }

  .imp__rows {
    display: flex;
    flex-direction: column;
    gap: 2px;
    max-block-size: 180px;
    overflow-y: auto;
    border: 1px solid var(--c-border);
    border-radius: var(--r-sm);
  }
  .imp__row {
    display: flex;
    gap: var(--sp-4);
    padding: var(--sp-2) var(--sp-3);
    font-size: var(--fs-sm);
    align-items: center;
    border-block-end: 1px solid var(--c-border);
  }
  .imp__row:last-child {
    border-block-end: none;
  }
  .imp__row--err {
    background: var(--c-danger-bg);
  }
  .imp__row--r-added {
    background: color-mix(in oklch, var(--c-success), transparent 90%);
  }
  .imp__row--r-updated {
    background: var(--c-accent-bg);
  }
  .imp__row--r-skipped {
    background: var(--c-warn-bg);
  }
  .imp__idx {
    color: var(--c-text-faint);
    font-size: var(--fs-xs);
    min-inline-size: 3ch;
  }
  .imp__row-err {
    color: var(--c-danger);
    font-size: var(--fs-xs);
  }

  .imp__actions {
    display: flex;
    justify-content: flex-end;
    gap: var(--sp-3);
    padding: var(--sp-4) var(--sp-5);
    border-block-start: 1px solid var(--c-border);
  }
  .imp__hint {
    padding: 0 var(--sp-5) var(--sp-4);
    font-size: var(--fs-xs);
    margin: 0;
  }

  .btn {
    display: inline-flex;
    align-items: center;
    gap: var(--sp-2);
    padding: var(--sp-2) var(--sp-5);
    border-radius: var(--r-sm);
    font-size: var(--fs-sm);
    background: var(--c-surface-2);
    color: var(--c-text);
    border: 1px solid var(--c-border);
    transition: background var(--mo-fast);
    font-weight: var(--fw-medium);
  }
  .btn:hover {
    background: var(--c-surface-hover);
  }
  .btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
  .btn--primary {
    background: var(--c-accent);
    color: light-dark(oklch(98% 0 0), oklch(10% 0 0));
    border-color: var(--c-accent);
  }
  .btn--primary:hover:not(:disabled) {
    background: color-mix(in oklch, var(--c-accent), white 8%);
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

  .faint {
    color: var(--c-text-faint);
  }
</style>
