<!--
  Wizard — first-run onboarding sherpa.

  Appears once per browser when all three conditions hold:
    1. localStorage flag `kiroxy:wizard-dismissed` is not set.
    2. Initial /dashboard/api/state response reports zero accounts.
    3. The mount happens within ~2s of page load (prevents the wizard
       from ambushing an operator who imported accounts mid-session
       and then Ctrl-Reloaded).

  Three steps, operator-voice microcopy pulled from docs/DASHBOARD_MANSION.md
  and /tmp/kiroxy-mockup/brand-spec.md:

    1. Welcome — what kiroxy is in 30 seconds, three CTAs.
    2. Path   — "I have tokens" (opens ImportDrawer via DOM event) vs
                "I need tokens" (shows camoufox onboarder instructions).
    3. Done   — pointers to ⌘K, ?, and the Live Stream.

  Self-mounted from main.ts into its own root so this component does
  NOT depend on App.svelte modifications (concurrent session owns App).
  Communicates with the rest of the app via CustomEvents on window:
    - "kiroxy:open-import"  — requests the import drawer
    - "kiroxy:open-palette" — requests the command palette

  Skippable: "skip for now" and Esc both persist the dismissal flag.
-->
<script lang="ts">
  import { onMount, onDestroy } from "svelte";

  interface Props {
    /** When true, bypasses the localStorage + vault-empty check and shows
     *  the wizard immediately. Used by the "show onboarding" palette
     *  action once C4 wires that command through. */
    force?: boolean;
    /** Parent notifies when the overlay should close — either the user
     *  dismissed it, or it finished. Parent can unmount after. */
    onClose?: () => void;
  }
  let { force = false, onClose }: Props = $props();

  const DISMISS_KEY = "kiroxy:wizard-dismissed";
  const STATE_URL = "/dashboard/api/state";

  let visible = $state(false);
  let step = $state<1 | 2 | 3>(1);
  let showingInstall = $state(false);

  onMount(() => {
    if (force) {
      visible = true;
      return;
    }

    if (readDismissal()) return;

    void (async () => {
      try {
        const res = await fetch(STATE_URL, {
          headers: { Accept: "application/json" },
        });
        if (!res.ok) return;
        const data = (await res.json()) as { accounts?: unknown };
        const accounts = Array.isArray(data.accounts) ? data.accounts : [];
        if (accounts.length === 0) {
          visible = true;
        }
      } catch {
        // Silent: network unreachable or JSON malformed. The empty-state
        // components will still show the right message on the board.
      }
    })();

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

  function readDismissal(): boolean {
    try {
      return localStorage.getItem(DISMISS_KEY) === "1";
    } catch {
      return false;
    }
  }

  function writeDismissal(): void {
    try {
      localStorage.setItem(DISMISS_KEY, "1");
    } catch {
      // Ignore — incognito or quota-exhausted. The wizard will show
      // again next load but that's the right behaviour in that mode.
    }
  }

  function dismiss(): void {
    writeDismissal();
    visible = false;
    onClose?.();
  }

  function finish(): void {
    writeDismissal();
    visible = false;
    onClose?.();
  }

  function goImport(): void {
    window.dispatchEvent(new CustomEvent("kiroxy:open-import"));
    finish();
  }

  function copyCmd(cmd: string): void {
    void navigator.clipboard.writeText(cmd).catch(() => {
      // Clipboard denied; user can still select the command by hand.
    });
  }
</script>

{#if visible}
  <div class="wiz-scrim" role="presentation" onclick={dismiss}></div>
  <div class="wiz" role="dialog" aria-modal="true" aria-labelledby="wiz-title">
    <header class="wiz__head">
      <div class="wiz__brand mono">
        <span class="wiz__brand-mark">kiroxy_</span>
        <span class="wiz__brand-sub faint">first-run</span>
      </div>
      <button type="button" class="wiz__dismiss" onclick={dismiss} aria-label="skip for now">
        skip for now
      </button>
    </header>

    <ol class="wiz__rail" aria-label="wizard progress">
      {#each [1, 2, 3] as n (n)}
        <li
          class="wiz__rail-dot"
          class:wiz__rail-dot--active={step === n}
          class:wiz__rail-dot--done={step > n}
          aria-current={step === n ? "step" : undefined}
        >
          <span class="wiz__rail-num mono tabular">{n}</span>
        </li>
      {/each}
    </ol>

    <div class="wiz__body">
      {#if step === 1}
        <h1 id="wiz-title" class="wiz__title">Welcome to the operator desk.</h1>
        <p class="wiz__lede">
          kiroxy is an Anthropic-compatible proxy that rotates a pool of Kiro
          accounts, handles token refresh, and streams telemetry to this
          dashboard.
        </p>
        <p class="wiz__lede faint">
          Right now the pool is empty. Pick how you want to fill it.
        </p>
        <div class="wiz__actions">
          <button type="button" class="wiz__btn wiz__btn--primary" onclick={() => (step = 2)}>
            get started
            <span class="wiz__kbd">↵</span>
          </button>
          <button type="button" class="wiz__btn wiz__btn--ghost" onclick={dismiss}>
            skip for now
            <span class="wiz__kbd">esc</span>
          </button>
        </div>
      {:else if step === 2}
        <h1 id="wiz-title" class="wiz__title">Two paths to a working pool.</h1>
        <p class="wiz__lede faint">
          Pick whichever matches your situation. You can always change your
          mind and come back via <span class="mono">⌘K → open onboarding</span>.
        </p>

        <div class="wiz__paths">
          <article class="wiz__path">
            <h2 class="wiz__path-title">I already have tokens</h2>
            <p class="wiz__path-body faint">
              You exported an account bundle from kiro-cli or another kiroxy
              vault and have it as a JSON array. Paste it into the import
              drawer.
            </p>
            <button type="button" class="wiz__btn wiz__btn--primary" onclick={goImport}>
              open import drawer
              <span class="wiz__kbd">i</span>
            </button>
          </article>

          <article class="wiz__path">
            <h2 class="wiz__path-title">I need to generate tokens</h2>
            <p class="wiz__path-body faint">
              Use the Camoufox onboarder to walk through the Kiro auth flow in
              a managed browser and write the tokens back to the vault.
            </p>

            {#if !showingInstall}
              <button
                type="button"
                class="wiz__btn wiz__btn--ghost"
                onclick={() => (showingInstall = true)}
              >
                show onboarder instructions
              </button>
            {:else}
              <ol class="wiz__steps">
                <li>
                  <span class="faint">install the onboarder once:</span>
                  <pre class="wiz__code mono"><code>cd tools/onboard && python -m venv .venv && .venv/bin/pip install -r requirements.txt</code></pre>
                  <button
                    type="button"
                    class="wiz__copy"
                    onclick={() => copyCmd("cd tools/onboard && python -m venv .venv && .venv/bin/pip install -r requirements.txt")}
                  >copy</button>
                </li>
                <li>
                  <span class="faint">run it against a fresh identity:</span>
                  <pre class="wiz__code mono"><code>python tools/onboard/onboard.py</code></pre>
                  <button
                    type="button"
                    class="wiz__copy"
                    onclick={() => copyCmd("python tools/onboard/onboard.py")}
                  >copy</button>
                </li>
                <li class="faint">
                  Tokens land in your vault automatically. This dashboard will
                  pick them up on the next poll.
                </li>
              </ol>
            {/if}
          </article>
        </div>

        <div class="wiz__actions wiz__actions--footer">
          <button type="button" class="wiz__btn wiz__btn--ghost" onclick={() => (step = 1)}>
            ← back
          </button>
          <button type="button" class="wiz__btn wiz__btn--ghost" onclick={() => (step = 3)}>
            skip to tips →
          </button>
        </div>
      {:else}
        <h1 id="wiz-title" class="wiz__title">The desk is yours.</h1>
        <p class="wiz__lede">
          Three shortcuts worth knowing before you dive in.
        </p>

        <dl class="wiz__tips">
          <dt>
            <kbd>⌘</kbd><kbd>K</kbd>
          </dt>
          <dd>
            Command palette. Search accounts, jump to requests, run actions,
            and — new in this release — read the docs inline without leaving
            the dashboard.
          </dd>
          <dt>
            <kbd>?</kbd>
          </dt>
          <dd>
            Keyboard cheatsheet. Every shortcut in one place.
          </dd>
          <dt>
            <span class="mono">Live</span>
          </dt>
          <dd>
            The request stream on the right is quiet until the first request
            lands. That's normal. "Wire quiet. Nothing moves."
          </dd>
        </dl>

        <div class="wiz__actions wiz__actions--footer">
          <button type="button" class="wiz__btn wiz__btn--ghost" onclick={() => (step = 2)}>
            ← back
          </button>
          <button type="button" class="wiz__btn wiz__btn--primary" onclick={finish}>
            to the desk
            <span class="wiz__kbd">↵</span>
          </button>
        </div>
      {/if}
    </div>
  </div>
{/if}

<style>
  .wiz-scrim {
    position: fixed;
    inset: 0;
    background: color-mix(in oklch, var(--c-bg, #111), transparent 18%);
    backdrop-filter: blur(2px);
    z-index: 200;
    animation: wiz-fade 180ms ease-out;
    cursor: default;
  }
  @keyframes wiz-fade {
    from {
      opacity: 0;
    }
    to {
      opacity: 1;
    }
  }

  .wiz {
    position: fixed;
    inset-block-start: 12vh;
    inset-inline: 0;
    margin-inline: auto;
    inline-size: min(620px, 94vw);
    max-block-size: 78vh;
    z-index: 201;
    background: var(--c-surface, #1b1b1b);
    color: var(--c-text, #eaeaea);
    border: 1px solid var(--c-border-strong, #2f2f2f);
    border-radius: var(--r-lg, 8px);
    box-shadow: 0 18px 56px rgba(0, 0, 0, 0.38);
    display: flex;
    flex-direction: column;
    overflow: hidden;
    animation: wiz-rise 240ms cubic-bezier(0.22, 1, 0.36, 1);
  }
  @keyframes wiz-rise {
    from {
      opacity: 0;
      transform: translateY(12px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }

  .wiz__head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: var(--sp-4, 12px) var(--sp-5, 16px);
    border-block-end: 1px solid var(--c-rule, #262626);
  }
  .wiz__brand {
    display: inline-flex;
    align-items: baseline;
    gap: var(--sp-3, 10px);
    font-family: var(--font-mono, monospace);
    font-size: var(--fs-sm, 13px);
  }
  .wiz__brand-mark {
    color: var(--c-accent, #d5a048);
    font-weight: 600;
  }
  .wiz__brand-sub {
    letter-spacing: var(--tr-caps, 0.08em);
    text-transform: uppercase;
    font-size: var(--fs-2xs, 11px);
  }
  .wiz__dismiss {
    font-size: var(--fs-xs, 12px);
    color: var(--c-text-faint, #7a7a7a);
    padding: 4px 8px;
    border-radius: var(--r-sm, 4px);
  }
  .wiz__dismiss:hover {
    color: var(--c-text, #eaeaea);
    background: var(--c-surface-hover, #242424);
  }

  .wiz__rail {
    display: flex;
    justify-content: center;
    gap: var(--sp-4, 12px);
    list-style: none;
    margin: 0;
    padding: var(--sp-3, 10px) var(--sp-5, 16px) 0;
  }
  .wiz__rail-dot {
    display: inline-grid;
    place-items: center;
    inline-size: 22px;
    block-size: 22px;
    border-radius: 999px;
    border: 1px solid var(--c-rule, #2a2a2a);
    color: var(--c-text-faint, #7a7a7a);
    background: var(--c-surface-sunken, #151515);
    transition: border-color 200ms var(--ease-std, ease), color 200ms var(--ease-std, ease);
  }
  .wiz__rail-dot--done {
    border-color: color-mix(in oklch, var(--c-accent, #d5a048), transparent 50%);
    color: var(--c-accent, #d5a048);
    background: var(--c-accent-wash, #2a2113);
  }
  .wiz__rail-dot--active {
    border-color: var(--c-accent, #d5a048);
    color: var(--c-accent, #d5a048);
    background: var(--c-accent-wash, #2a2113);
  }
  .wiz__rail-num {
    font-size: var(--fs-2xs, 11px);
  }

  .wiz__body {
    padding: var(--sp-5, 16px) var(--sp-6, 22px) var(--sp-6, 22px);
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: var(--sp-4, 12px);
  }
  .wiz__title {
    margin: var(--sp-3, 10px) 0 0;
    font-family: var(--font-display, var(--font-mono, monospace));
    font-size: var(--fs-xl, 22px);
    font-weight: var(--fw-semibold, 600);
    letter-spacing: -0.005em;
  }
  .wiz__lede {
    margin: 0;
    font-size: var(--fs-md, 14px);
    line-height: var(--lh-relaxed, 1.55);
    color: var(--c-text, #eaeaea);
    max-inline-size: 58ch;
  }
  .wiz__lede.faint {
    color: var(--c-text-dim, #a0a0a0);
  }

  .wiz__actions {
    display: flex;
    gap: var(--sp-3, 10px);
    align-items: center;
    margin-block-start: var(--sp-3, 10px);
  }
  .wiz__actions--footer {
    margin-block-start: var(--sp-4, 16px);
    justify-content: space-between;
  }
  .wiz__btn {
    display: inline-flex;
    align-items: center;
    gap: var(--sp-2, 8px);
    padding: 8px var(--sp-4, 14px);
    border: 1px solid var(--c-border, #2a2a2a);
    border-radius: var(--r-md, 6px);
    font-size: var(--fs-sm, 13px);
    color: var(--c-text, #eaeaea);
    background: var(--c-surface, #1b1b1b);
    transition: background 160ms var(--ease-std, ease), border-color 160ms var(--ease-std, ease);
  }
  .wiz__btn:hover {
    background: var(--c-surface-hover, #242424);
  }
  .wiz__btn--primary {
    color: var(--c-bg, #111);
    background: var(--c-accent, #d5a048);
    border-color: var(--c-accent, #d5a048);
    font-weight: var(--fw-semibold, 600);
  }
  .wiz__btn--primary:hover {
    background: var(--c-accent-strong, var(--c-accent, #e0ad58));
    border-color: var(--c-accent-strong, var(--c-accent, #e0ad58));
  }
  .wiz__btn--ghost {
    color: var(--c-text-dim, #a0a0a0);
    background: transparent;
  }
  .wiz__btn--ghost:hover {
    color: var(--c-text, #eaeaea);
  }
  .wiz__kbd {
    font-family: var(--font-mono, monospace);
    font-size: var(--fs-2xs, 11px);
    padding: 1px 5px;
    border: 1px solid color-mix(in oklch, currentColor, transparent 60%);
    border-radius: var(--r-sm, 4px);
    opacity: 0.85;
  }

  .wiz__paths {
    display: grid;
    grid-template-columns: 1fr;
    gap: var(--sp-4, 14px);
    margin-block-start: var(--sp-2, 8px);
  }
  @media (min-width: 620px) {
    .wiz__paths {
      grid-template-columns: 1fr 1fr;
    }
  }
  .wiz__path {
    display: flex;
    flex-direction: column;
    gap: var(--sp-3, 10px);
    padding: var(--sp-4, 14px);
    border: 1px solid var(--c-rule, #262626);
    border-radius: var(--r-md, 6px);
    background: var(--c-surface-sunken, #151515);
  }
  .wiz__path-title {
    margin: 0;
    font-size: var(--fs-md, 14px);
    font-weight: var(--fw-semibold, 600);
    font-family: var(--font-display, var(--font-mono, monospace));
  }
  .wiz__path-body {
    margin: 0;
    font-size: var(--fs-sm, 13px);
    line-height: var(--lh-relaxed, 1.5);
  }
  .wiz__steps {
    list-style: decimal;
    margin: 0;
    padding-inline-start: var(--sp-5, 18px);
    display: flex;
    flex-direction: column;
    gap: var(--sp-3, 10px);
    font-size: var(--fs-sm, 13px);
  }
  .wiz__steps li {
    display: flex;
    flex-direction: column;
    gap: 4px;
    align-items: flex-start;
  }
  .wiz__code {
    margin: 0;
    padding: 6px 8px;
    background: var(--c-surface-2, #1f1f1f);
    border: 1px solid var(--c-rule, #262626);
    border-radius: var(--r-sm, 4px);
    font-size: var(--fs-2xs, 11px);
    line-height: var(--lh-snug, 1.35);
    inline-size: 100%;
    overflow-x: auto;
    white-space: pre;
  }
  .wiz__copy {
    font-size: var(--fs-2xs, 11px);
    color: var(--c-text-faint, #7a7a7a);
    letter-spacing: var(--tr-caps, 0.08em);
    text-transform: uppercase;
  }
  .wiz__copy:hover {
    color: var(--c-accent, #d5a048);
  }

  .wiz__tips {
    display: grid;
    grid-template-columns: max-content 1fr;
    gap: var(--sp-3, 10px) var(--sp-5, 18px);
    margin: 0;
    font-size: var(--fs-sm, 13px);
  }
  .wiz__tips dt {
    display: inline-flex;
    gap: 4px;
    align-items: center;
    color: var(--c-text, #eaeaea);
  }
  .wiz__tips dd {
    margin: 0;
    color: var(--c-text-dim, #a0a0a0);
    line-height: var(--lh-relaxed, 1.5);
  }
  .wiz__tips kbd {
    display: inline-grid;
    place-items: center;
    min-inline-size: 18px;
    padding: 1px 5px;
    font-family: var(--font-mono, monospace);
    font-size: var(--fs-2xs, 11px);
    color: var(--c-text-dim, #a0a0a0);
    background: var(--c-surface-2, #1f1f1f);
    border: 1px solid var(--c-rule, #262626);
    border-radius: var(--r-sm, 4px);
  }

  @media (prefers-reduced-motion: reduce) {
    .wiz,
    .wiz-scrim {
      animation: none;
    }
  }
</style>
