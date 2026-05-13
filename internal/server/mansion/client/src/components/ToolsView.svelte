<!--
  ToolsView — operator-facing utility cards.

  Subtabs:
    - Diagnostic — runs `kiroxy doctor` over the wire, shows colored
      results + remediation hints. Auto-runs on mount.
    - Backup     — instructions for the (not-yet-built) backup CLI; v1.2.0
      will turn this card into a real download button.
    - Restore    — same status: instructions for the future restore CLI.
    - Onboarder  — instructions for kiro_login.py / onboard.py; v1.2.0
      will offer a "Run from UI" button.

  We do NOT shell out from the dashboard — backup/restore can write
  encrypted files and accept arbitrary archives, both of which are
  privileged actions that need careful UX. v1.1.0 ships display-only
  cards so operators have a centralized place to learn the workflow.
-->
<script lang="ts">
  import { onMount } from "svelte";
  import { api, type DoctorReport, type DoctorResult } from "../lib/api";
  import Icon from "./Icon.svelte";

  type Tab = "diagnostic" | "backup" | "restore" | "onboarder";
  let tab: Tab = $state("diagnostic");

  let report: DoctorReport | null = $state(null);
  let reportErr: string | null = $state(null);
  let running = $state(false);

  onMount(() => {
    void runDoctor();
  });

  async function runDoctor(): Promise<void> {
    running = true;
    reportErr = null;
    const r = await api.doctor();
    running = false;
    if (r.ok) {
      report = r.data;
    } else if (r.status === 404) {
      reportErr = "doctor endpoint disabled (no ToolsProvider configured)";
    } else {
      reportErr = `doctor failed: ${r.error}`;
    }
  }

  function statusClass(s: DoctorResult["status"]): string {
    switch (s) {
      case "ok":
        return "rs--ok";
      case "warn":
        return "rs--warn";
      case "error":
        return "rs--error";
      default:
        return "rs--skip";
    }
  }
</script>

<section class="tools" aria-label="tools">
  <header class="tools__head">
    <span class="caps">tools</span>
    <nav class="tabs" role="tablist">
      {#each ["diagnostic", "backup", "restore", "onboarder"] as t}
        <button
          type="button"
          role="tab"
          class="tab"
          class:tab--active={tab === t}
          aria-selected={tab === t}
          onclick={() => (tab = t as Tab)}
        >
          {t}
        </button>
      {/each}
    </nav>
  </header>

  <div class="tools__body">
    {#if tab === "diagnostic"}
      <div class="card">
        <div class="card__head">
          <div>
            <h3 class="card__title">kiroxy doctor</h3>
            <p class="card__sub mono faint">
              checks runtime, vault file, upstream reachability, and pool size
            </p>
          </div>
          <button
            type="button"
            class="btn btn--accent"
            onclick={() => void runDoctor()}
            disabled={running}
          >
            <Icon name="refresh" size={12} />
            <span>{running ? "running…" : "run again"}</span>
          </button>
        </div>

        {#if reportErr}
          <div class="banner banner--err">{reportErr}</div>
        {:else if report}
          <div class="report" class:report--bad={!report.ok}>
            <header class="report__head">
              <span class="report__verdict caps" class:report__verdict--ok={report.ok}>
                {report.ok ? "all checks passed" : "issues found"}
              </span>
              <span class="report__meta mono faint">
                {report.elapsed} · {report.go_version ?? "—"}
              </span>
            </header>
            <ol class="report__list">
              {#each report.results as r}
                <li class="row {statusClass(r.status)}">
                  <span class="row__marker mono caps">
                    {#if r.status === "ok"}OK
                    {:else if r.status === "warn"}WARN
                    {:else if r.status === "error"}FAIL
                    {:else}SKIP{/if}
                  </span>
                  <span class="row__name mono">{r.name}</span>
                  <span class="row__detail">{r.detail}</span>
                  {#if r.hint}
                    <span class="row__hint mono faint">→ {r.hint}</span>
                  {/if}
                </li>
              {/each}
            </ol>
          </div>
        {:else}
          <div class="empty mono faint">running diagnostic…</div>
        {/if}
      </div>

    {:else if tab === "backup"}
      <div class="card">
        <h3 class="card__title">Backup vault to encrypted .age</h3>
        <p class="card__sub">
          Encrypted backups of your token vault. v1.2.0 will turn this card
          into a real download button.
        </p>
        <pre class="cmd mono">
{`# coming in v1.2.0:
kiroxy backup --out kiroxy-vault-$(date +%F).age --recipient $YOUR_AGE_PUB`}
        </pre>
        <p class="card__sub faint">
          For now, copy the SQLite file directly — but ensure mode 0600 and
          treat it like any other secret material.
        </p>
        <pre class="cmd mono">cp ~/.local/share/kiroxy/tokens.db ./vault-backup.db && chmod 600 ./vault-backup.db</pre>
      </div>

    {:else if tab === "restore"}
      <div class="card">
        <h3 class="card__title">Restore vault from backup</h3>
        <p class="card__sub">
          v1.2.0 will accept a .age upload here. v1.1.0 ships instructions
          only because restoring touches secrets directly.
        </p>
        <pre class="cmd mono">
{`# coming in v1.2.0:
kiroxy restore ./vault-backup.age`}
        </pre>
        <p class="card__sub faint">
          Until then, stop kiroxy first, then replace the SQLite file:
        </p>
        <pre class="cmd mono">
{`pkill -INT kiroxy
cp ./vault-backup.db ~/.local/share/kiroxy/tokens.db
chmod 600 ~/.local/share/kiroxy/tokens.db
kiroxy serve`}
        </pre>
      </div>

    {:else if tab === "onboarder"}
      <div class="card">
        <h3 class="card__title">Account onboarder</h3>
        <p class="card__sub">
          Helpers from research/<code class="mono">v4</code> that automate the Kiro IDE OAuth flow.
          v1.2.0 will offer a "Run from UI" button; today they ship as
          standalone Python scripts.
        </p>
        <pre class="cmd mono">
{`# Headless camoufox flow (Linux/macOS):
python3 onboard/kiro_login.py

# Once you have the session, import via:
kiroxy import-accounts-json ./kiro-accounts.json`}
        </pre>
        <p class="card__sub faint">
          Or use the import drawer: press <kbd>i</kbd> from anywhere on the
          dashboard, paste the JSON, and the accounts land in the vault.
        </p>
      </div>
    {/if}
  </div>
</section>

<style>
  .tools {
    display: flex;
    flex-direction: column;
    min-block-size: 0;
    background: var(--c-surface);
    border: 1px solid var(--c-border);
    border-radius: var(--r-md);
    box-shadow: var(--sh-1);
  }
  .tools__head {
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
  }
  .tab:hover {
    color: var(--c-text);
  }
  .tab--active {
    color: var(--c-accent);
    background: var(--c-surface);
    box-shadow: var(--sh-1), inset 0 0 0 1px color-mix(in oklch, var(--c-accent), transparent 60%);
  }

  .tools__body {
    padding: var(--sp-5);
    overflow-y: auto;
  }

  .empty {
    padding: var(--sp-6);
    text-align: center;
  }
  .banner {
    padding: var(--sp-3) var(--sp-4);
    font-size: var(--fs-sm);
    font-family: var(--font-mono);
    border-radius: var(--r-sm);
  }
  .banner--err {
    color: var(--c-warn);
    background: var(--c-warn-bg);
  }

  .card {
    padding: var(--sp-5);
    background: var(--c-surface-sunken);
    border: 1px solid var(--c-rule);
    border-radius: var(--r-sm);
  }
  .card__head {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: var(--sp-5);
    margin-block-end: var(--sp-4);
  }
  .card__title {
    font-family: var(--font-display);
    font-size: var(--fs-md);
    color: var(--c-text);
    margin: 0;
  }
  .card__sub {
    color: var(--c-text-dim);
    font-size: var(--fs-sm);
    margin: var(--sp-2) 0;
    max-inline-size: 60ch;
  }

  .cmd {
    display: block;
    padding: var(--sp-3) var(--sp-4);
    background: var(--c-bg);
    border: 1px solid var(--c-rule);
    border-radius: var(--r-sm);
    color: var(--c-accent);
    font-size: var(--fs-sm);
    overflow-x: auto;
    margin: var(--sp-3) 0;
    white-space: pre-wrap;
  }

  .report {
    margin-block-start: var(--sp-3);
  }
  .report__head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding-block-end: var(--sp-3);
    border-block-end: 1px solid var(--c-rule);
    margin-block-end: var(--sp-3);
  }
  .report__verdict {
    color: var(--c-warn);
    font-size: var(--fs-sm);
  }
  .report__verdict--ok {
    color: var(--c-success);
  }
  .report__meta {
    font-size: var(--fs-xs);
  }

  .report__list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: var(--sp-2);
  }
  .row {
    display: grid;
    grid-template-columns: 56px 100px minmax(0, 1fr);
    gap: var(--sp-3);
    align-items: start;
    padding: var(--sp-2) var(--sp-3);
    border-radius: var(--r-sm);
    font-size: var(--fs-sm);
  }
  .row__marker {
    font-size: var(--fs-2xs);
    padding: 1px 6px;
    border-radius: var(--r-sm);
    text-align: center;
  }
  .row__name {
    color: var(--c-text);
  }
  .row__detail {
    color: var(--c-text-dim);
    overflow-wrap: anywhere;
  }
  .row__hint {
    grid-column: 3 / -1;
    margin-block-start: var(--sp-1);
    font-size: var(--fs-xs);
  }

  .rs--ok .row__marker {
    color: var(--c-success);
    background: var(--c-success-bg);
  }
  .rs--warn .row__marker {
    color: var(--c-warn);
    background: var(--c-warn-bg);
  }
  .rs--error .row__marker {
    color: var(--c-danger);
    background: var(--c-danger-bg);
  }
  .rs--skip .row__marker {
    color: var(--c-text-faint);
    background: var(--c-surface-sunken);
  }
  .rs--error {
    background: color-mix(in oklch, var(--c-danger-bg), transparent 50%);
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
</style>
