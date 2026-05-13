<!--
  SettingsView — bundled view of runtime info, env vars, inbound keys, and
  vault stats. One fetch (/dashboard/api/settings) hydrates 3 of 4 tabs;
  the Inbound Keys tab loads its own list because it has CRUD.

  Tabs:
    1. General — version, uptime, paths, log level
    2. Env Vars — KIROXY_* env exposure with redaction for secrets
    3. Inbound Keys — list / generate / revoke
    4. Vault — pool breakdown + DB file path/size
-->
<script lang="ts">
  import { onMount } from "svelte";
  import {
    api,
    type SettingsSnapshot,
    type InboundKeyView,
    type CreatedInboundKey,
  } from "../lib/api";
  import { fmtBytes, fmtUptime, relTime } from "../lib/format";
  import Icon from "./Icon.svelte";

  type Tab = "general" | "env" | "keys" | "vault";

  let activeTab: Tab = $state("general");
  let snap: SettingsSnapshot | null = $state(null);
  let snapErr: string | null = $state(null);

  let keys: InboundKeyView[] = $state([]);
  let keysErr: string | null = $state(null);
  let keysLoading = $state(false);
  let createPending = $state(false);
  let revealedKey: CreatedInboundKey | null = $state(null);
  let newLabel = $state("");
  let toRevokeID: string | null = $state(null);
  let copyOK = $state(false);

  onMount(() => {
    void loadSnap();
    void loadKeys();
  });

  async function loadSnap(): Promise<void> {
    const r = await api.settings();
    if (r.ok) {
      snap = r.data;
      snapErr = null;
    } else if (r.status === 404) {
      snapErr = "settings endpoint disabled (no SettingsProvider)";
    } else {
      snapErr = `load failed: ${r.error}`;
    }
  }

  async function loadKeys(): Promise<void> {
    keysLoading = true;
    const r = await api.listInboundKeys();
    keysLoading = false;
    if (r.ok) {
      keys = r.data.keys ?? [];
      keysErr = null;
    } else if (r.status === 404) {
      keysErr = "inbound keys disabled (no provider)";
    } else {
      keysErr = `load failed: ${r.error}`;
    }
  }

  async function createKey(): Promise<void> {
    createPending = true;
    const r = await api.createInboundKey(newLabel.trim());
    createPending = false;
    if (r.ok) {
      revealedKey = r.data;
      newLabel = "";
      void loadKeys();
      void loadSnap();
    } else {
      keysErr = `create failed: ${r.error}`;
    }
  }

  async function copyPlaintext(): Promise<void> {
    if (!revealedKey) return;
    try {
      await navigator.clipboard.writeText(revealedKey.plaintext);
      copyOK = true;
      setTimeout(() => (copyOK = false), 1800);
    } catch {
      keysErr = "clipboard denied — copy manually";
    }
  }

  function dismissReveal(): void {
    revealedKey = null;
  }

  async function confirmRevoke(): Promise<void> {
    if (!toRevokeID) return;
    const id = toRevokeID;
    toRevokeID = null;
    const r = await api.revokeInboundKey(id);
    if (r.ok) {
      void loadKeys();
      void loadSnap();
    } else {
      keysErr = `revoke failed: ${r.error}`;
    }
  }
</script>

<section class="settings" aria-label="settings">
  <header class="settings__head">
    <span class="caps">settings</span>
    <nav class="tabs" role="tablist">
      {#each ["general", "env", "keys", "vault"] as t}
        <button
          type="button"
          role="tab"
          class="tab"
          class:tab--active={activeTab === t}
          aria-selected={activeTab === t}
          onclick={() => (activeTab = t as Tab)}
        >
          {t === "env" ? "env vars" : t === "keys" ? "inbound keys" : t}
        </button>
      {/each}
    </nav>
  </header>

  {#if snapErr}
    <div class="banner banner--err" role="status">{snapErr}</div>
  {/if}

  <div class="settings__body">
    {#if activeTab === "general"}
      {#if snap}
        <dl class="kv mono">
          <dt>version</dt>
          <dd>{snap.general.version}</dd>
          <dt>uptime</dt>
          <dd>{fmtUptime(snap.general.uptime_s)}</dd>
          <dt>started</dt>
          <dd>{snap.general.started_at}</dd>
          <dt>vault path</dt>
          <dd class="kv__path">{snap.general.vault_path ?? "—"}</dd>
          <dt>log level</dt>
          <dd>{snap.general.log_level ?? "—"}</dd>
        </dl>
      {:else}
        <div class="empty mono faint">loading…</div>
      {/if}
    {:else if activeTab === "env"}
      {#if snap}
        <table class="env">
          <thead>
            <tr>
              <th>key</th>
              <th>value</th>
              <th>state</th>
            </tr>
          </thead>
          <tbody>
            {#each snap.env_vars as v}
              <tr class:env--unset={!v.present}>
                <td class="env__k mono">{v.key}</td>
                <td class="env__v mono">
                  {#if !v.present}
                    <span class="faint">(unset)</span>
                  {:else if v.redacted}
                    <span title="redacted — secret env var">{v.value}</span>
                    <span class="env__redacted-tag caps faint">redacted</span>
                  {:else}
                    {v.value}
                  {/if}
                </td>
                <td class="env__state caps">
                  {#if !v.present}
                    <span class="faint">unset</span>
                  {:else if v.redacted}
                    <span class="env__sec">secret</span>
                  {:else}
                    <span class="env__public">public</span>
                  {/if}
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      {:else}
        <div class="empty mono faint">loading…</div>
      {/if}
    {:else if activeTab === "keys"}
      <div class="keys">
        {#if keysErr}
          <div class="banner banner--err">{keysErr}</div>
        {/if}

        {#if revealedKey}
          <div class="reveal" role="alert">
            <div class="reveal__head caps">save this now — won't show again</div>
            <code class="reveal__plain mono">{revealedKey.plaintext}</code>
            <div class="reveal__actions">
              <button type="button" class="btn btn--accent" onclick={() => void copyPlaintext()}>
                <Icon name="copy" size={11} /><span>{copyOK ? "copied" : "copy"}</span>
              </button>
              <button type="button" class="btn" onclick={dismissReveal}>dismiss</button>
            </div>
            {#if revealedKey.label}
              <div class="reveal__meta mono faint">label: {revealedKey.label}</div>
            {/if}
          </div>
        {/if}

        <form
          class="create"
          onsubmit={(e) => {
            e.preventDefault();
            void createKey();
          }}
        >
          <label class="create__label">
            <span class="caps">label</span>
            <input
              type="text"
              class="create__input mono"
              placeholder="ci, cron, dev-laptop…"
              maxlength={128}
              bind:value={newLabel}
              spellcheck="false"
              autocomplete="off"
            />
          </label>
          <button
            type="submit"
            class="btn btn--accent"
            disabled={createPending}
          >
            <Icon name="key" size={12} />
            <span>{createPending ? "minting…" : "generate key"}</span>
          </button>
        </form>

        <div class="keys__list">
          {#if keysLoading}
            <div class="empty mono faint">loading…</div>
          {:else if keys.length === 0}
            <div class="empty mono faint">no inbound keys yet</div>
          {:else}
            <table class="keytbl">
              <thead>
                <tr>
                  <th>label</th>
                  <th>tail</th>
                  <th>created</th>
                  <th>last used</th>
                  <th>state</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                {#each keys as k}
                  <tr class:keytbl--rev={k.revoked}>
                    <td class="mono">{k.label || "—"}</td>
                    <td class="mono">****{k.tail}</td>
                    <td class="mono faint">{relTime(k.created_at)}</td>
                    <td class="mono faint">{k.last_used_at ? relTime(k.last_used_at) : "—"}</td>
                    <td class="caps">
                      {#if k.revoked}
                        <span class="badge badge--rev">revoked</span>
                      {:else}
                        <span class="badge badge--ok">active</span>
                      {/if}
                    </td>
                    <td>
                      {#if !k.revoked}
                        <button
                          type="button"
                          class="btn btn--small btn--danger"
                          onclick={() => (toRevokeID = k.id)}
                          title="revoke key"
                          aria-label="revoke key"
                        >
                          <Icon name="trash" size={11} />
                          <span>revoke</span>
                        </button>
                      {/if}
                    </td>
                  </tr>
                {/each}
              </tbody>
            </table>
          {/if}
        </div>
      </div>
    {:else if activeTab === "vault"}
      {#if snap}
        <dl class="kv mono">
          <dt>vault path</dt>
          <dd class="kv__path">{snap.vault.path ?? snap.general.vault_path ?? "—"}</dd>
          <dt>file size</dt>
          <dd>{snap.vault.size_bytes ? fmtBytes(snap.vault.size_bytes) : "—"}</dd>
          <dt>total accounts</dt>
          <dd>{snap.vault.total}</dd>
          <dt>healthy</dt>
          <dd class="kv--good">{snap.vault.healthy}</dd>
          <dt>cooldown</dt>
          <dd class="kv--warn">{snap.vault.cooldown}</dd>
          <dt>disabled</dt>
          <dd class="kv--dim">{snap.vault.disabled}</dd>
          <dt>inbound keys (active)</dt>
          <dd>{snap.inbound_keys.active}</dd>
          <dt>inbound keys (total)</dt>
          <dd>{snap.inbound_keys.total}</dd>
        </dl>
      {:else}
        <div class="empty mono faint">loading…</div>
      {/if}
    {/if}
  </div>
</section>

{#if toRevokeID}
  <div class="modal" role="dialog" aria-modal="true" aria-label="revoke confirm">
    <div class="modal__panel">
      <h3 class="modal__title">Revoke this key?</h3>
      <p class="modal__msg">
        Apps using this key will start getting 401. This cannot be undone.
      </p>
      <div class="modal__actions">
        <button type="button" class="btn" onclick={() => (toRevokeID = null)}>cancel</button>
        <button type="button" class="btn btn--danger" onclick={() => void confirmRevoke()}>revoke</button>
      </div>
    </div>
  </div>
{/if}

<style>
  .settings {
    display: flex;
    flex-direction: column;
    min-block-size: 0;
    background: var(--c-surface);
    border: 1px solid var(--c-border);
    border-radius: var(--r-md);
    box-shadow: var(--sh-1);
  }
  .settings__head {
    display: flex;
    align-items: center;
    gap: var(--sp-5);
    padding: var(--sp-3) var(--sp-5);
    border-block-end: 1px solid var(--c-rule);
  }

  .tabs {
    display: inline-flex;
    gap: 1px;
    padding: 1px;
    background: var(--c-surface-sunken);
    border: 1px solid var(--c-rule);
    border-radius: var(--r-sm);
    margin-inline-start: auto;
  }
  .tab {
    padding: 4px 12px;
    font-family: var(--font-mono);
    font-size: var(--fs-xs);
    text-transform: uppercase;
    letter-spacing: var(--tr-wide);
    color: var(--c-text-dim);
    background: transparent;
    border: 0;
    border-radius: var(--r-xs);
    cursor: pointer;
    transition: color var(--mo-fast) var(--ease-std);
  }
  .tab:hover {
    color: var(--c-text);
  }
  .tab--active {
    color: var(--c-accent);
    background: var(--c-surface);
    box-shadow: var(--sh-1), inset 0 0 0 1px color-mix(in oklch, var(--c-accent), transparent 60%);
  }

  .settings__body {
    padding: var(--sp-5);
    overflow-y: auto;
  }

  .empty {
    padding: var(--sp-6);
    text-align: center;
  }

  .banner {
    padding: var(--sp-3) var(--sp-5);
    font-size: var(--fs-xs);
    font-family: var(--font-mono);
    border-block-end: 1px solid var(--c-rule);
  }
  .banner--err {
    color: var(--c-warn);
    background: var(--c-warn-bg);
  }

  .kv {
    display: grid;
    grid-template-columns: max-content minmax(0, 1fr);
    gap: var(--sp-2) var(--sp-5);
    font-size: var(--fs-sm);
  }
  .kv dt {
    color: var(--c-text-dim);
    text-transform: uppercase;
    letter-spacing: var(--tr-wide);
    font-size: var(--fs-2xs);
    align-self: center;
  }
  .kv dd {
    color: var(--c-text);
  }
  .kv__path {
    overflow-wrap: anywhere;
  }
  .kv--good {
    color: var(--c-success);
  }
  .kv--warn {
    color: var(--c-warn);
  }
  .kv--dim {
    color: var(--c-text-faint);
  }

  .env {
    inline-size: 100%;
    border-collapse: collapse;
    font-size: var(--fs-sm);
  }
  .env th,
  .env td {
    text-align: start;
    padding: var(--sp-2) var(--sp-4);
    border-block-end: 1px solid var(--c-rule);
  }
  .env th {
    text-transform: uppercase;
    letter-spacing: var(--tr-wide);
    font-size: var(--fs-2xs);
    color: var(--c-text-dim);
    font-weight: var(--fw-normal);
  }
  .env__k {
    color: var(--c-accent);
    white-space: nowrap;
  }
  .env__v {
    overflow-wrap: anywhere;
    color: var(--c-text);
  }
  .env__redacted-tag {
    margin-inline-start: var(--sp-2);
    font-size: 9px;
  }
  .env__state {
    font-size: var(--fs-2xs);
    text-align: end;
  }
  .env__sec {
    color: var(--c-warn);
  }
  .env__public {
    color: var(--c-success);
  }
  .env--unset .env__v,
  .env--unset .env__k {
    color: var(--c-text-faint);
  }

  .reveal {
    margin-block-end: var(--sp-5);
    padding: var(--sp-4);
    background: var(--c-accent-wash);
    border: 1px solid color-mix(in oklch, var(--c-accent), transparent 50%);
    border-radius: var(--r-sm);
  }
  .reveal__head {
    color: var(--c-accent);
    font-size: var(--fs-2xs);
    margin-block-end: var(--sp-2);
  }
  .reveal__plain {
    display: block;
    padding: var(--sp-3);
    background: var(--c-surface-sunken);
    border-radius: var(--r-sm);
    word-break: break-all;
    font-size: var(--fs-sm);
    color: var(--c-text);
  }
  .reveal__actions {
    display: flex;
    gap: var(--sp-3);
    margin-block-start: var(--sp-3);
  }
  .reveal__meta {
    margin-block-start: var(--sp-2);
    font-size: var(--fs-2xs);
  }

  .create {
    display: flex;
    align-items: end;
    gap: var(--sp-3);
    padding: var(--sp-3);
    margin-block-end: var(--sp-4);
    background: var(--c-surface-sunken);
    border: 1px solid var(--c-rule);
    border-radius: var(--r-sm);
  }
  .create__label {
    display: flex;
    flex-direction: column;
    gap: var(--sp-1);
    flex: 1;
  }
  .create__input {
    padding: 5px var(--sp-3);
    font-size: var(--fs-sm);
    background: var(--c-surface);
    border: 1px solid var(--c-border);
    border-radius: var(--r-sm);
    color: var(--c-text);
  }

  .keytbl {
    inline-size: 100%;
    border-collapse: collapse;
    font-size: var(--fs-sm);
  }
  .keytbl th,
  .keytbl td {
    text-align: start;
    padding: var(--sp-2) var(--sp-4);
    border-block-end: 1px solid var(--c-rule);
  }
  .keytbl th {
    text-transform: uppercase;
    letter-spacing: var(--tr-wide);
    font-size: var(--fs-2xs);
    color: var(--c-text-dim);
    font-weight: var(--fw-normal);
  }
  .keytbl--rev td {
    color: var(--c-text-faint);
  }

  .badge {
    padding: 1px 6px;
    border-radius: var(--r-pill);
    font-size: var(--fs-2xs);
  }
  .badge--ok {
    color: var(--c-success);
    background: var(--c-success-bg);
  }
  .badge--rev {
    color: var(--c-text-faint);
    background: var(--c-surface-sunken);
  }

  .btn {
    display: inline-flex;
    align-items: center;
    gap: var(--sp-2);
    padding: 5px 10px;
    font-size: var(--fs-sm);
    font-family: var(--font-mono);
    background: var(--c-surface);
    border: 1px solid var(--c-border);
    border-radius: var(--r-sm);
    color: var(--c-text-dim);
    cursor: pointer;
    transition: all var(--mo-fast) var(--ease-std);
  }
  .btn:hover {
    color: var(--c-text);
    border-color: var(--c-border-strong);
  }
  .btn:disabled {
    cursor: not-allowed;
    opacity: 0.5;
  }
  .btn--accent {
    color: var(--c-accent);
    border-color: color-mix(in oklch, var(--c-accent), transparent 50%);
    background: var(--c-accent-wash);
  }
  .btn--accent:hover {
    color: var(--c-accent-strong);
  }
  .btn--danger {
    color: var(--c-danger);
    border-color: color-mix(in oklch, var(--c-danger), transparent 50%);
  }
  .btn--small {
    padding: 3px 8px;
    font-size: var(--fs-xs);
  }

  .modal {
    position: fixed;
    inset: 0;
    background: color-mix(in oklch, var(--c-bg), transparent 30%);
    display: grid;
    place-items: center;
    z-index: var(--z-dialog);
    padding: var(--sp-5);
  }
  .modal__panel {
    background: var(--c-surface);
    border: 1px solid var(--c-border-strong);
    border-radius: var(--r-md);
    box-shadow: var(--sh-3);
    padding: var(--sp-5);
    max-inline-size: 420px;
    inline-size: 100%;
  }
  .modal__title {
    font-family: var(--font-display);
    font-size: var(--fs-lg);
    margin-block-end: var(--sp-3);
    color: var(--c-text);
  }
  .modal__msg {
    color: var(--c-text-dim);
    font-size: var(--fs-sm);
    margin-block-end: var(--sp-5);
  }
  .modal__actions {
    display: flex;
    justify-content: flex-end;
    gap: var(--sp-3);
  }
</style>
