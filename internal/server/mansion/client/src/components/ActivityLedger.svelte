<!--
  ActivityLedger — ambient log strip. Shows the last 20 interesting
  events synthesized from the snapshot + request stream: account
  additions/removals (detected from id-set diffs), cooldown triggers,
  error bursts, refresh churn. Every row is stamped with a time and a
  subtle color-coded glyph so the operator can see what happened at a
  glance without opening DetailDrawer.

  When the activity is quiet, the strip shows a watermark rather than
  a skeleton — it communicates "alert system armed, all quiet."
-->
<script lang="ts">
  import { store } from "../lib/store.svelte";
  import { shortTime } from "../lib/format";

  interface Event {
    id: number;
    at: string;
    kind: "start" | "info" | "warn" | "error" | "quiet";
    text: string;
  }

  let events = $state<Event[]>([]);
  let counter = 0;

  function add(ev: Omit<Event, "id" | "at">): void {
    const e: Event = {
      id: ++counter,
      at: new Date().toISOString(),
      ...ev,
    };
    events = [e, ...events].slice(0, 20);
  }

  // Seed a single "armed" event on first snapshot.
  let armed = false;
  let prevIds = new Set<string>();
  let prevCool = new Map<string, string>();
  let prevErrs = new Map<string, number>();

  $effect(() => {
    const s = store.snapshot;
    if (!armed && s.accounts.length >= 0 && s.version) {
      armed = true;
      add({ kind: "start", text: `operator desk armed · ${s.accounts.length} ${s.accounts.length === 1 ? "identity" : "identities"} in vault` });
    }

    const currentIds = new Set(s.accounts.map((a) => a.id));

    // Detect additions (skip first cycle since prevIds is empty).
    if (prevIds.size > 0) {
      for (const id of currentIds) {
        if (!prevIds.has(id)) add({ kind: "info", text: `account added · ${id}` });
      }
      for (const id of prevIds) {
        if (!currentIds.has(id)) add({ kind: "info", text: `account removed · ${id}` });
      }
    }

    // Cooldown transitions.
    for (const a of s.accounts) {
      const prev = prevCool.get(a.id) ?? "";
      const now = a.cooldown_until ?? "";
      if (now && now !== prev) {
        add({ kind: "warn", text: `cooldown armed · ${a.id} → ${new Date(now).toLocaleTimeString()}` });
      }
    }

    // Error bumps.
    for (const a of s.accounts) {
      const prev = prevErrs.get(a.id) ?? a.errors;
      if (a.errors > prev) {
        add({ kind: "error", text: `error count up · ${a.id} (+${a.errors - prev})` });
      }
    }

    prevIds = currentIds;
    prevCool = new Map(s.accounts.map((a) => [a.id, a.cooldown_until ?? ""]));
    prevErrs = new Map(s.accounts.map((a) => [a.id, a.errors]));
  });

  // Request-stream observer: log notable responses.
  let seenReq = new Set<string>();
  $effect(() => {
    for (const r of store.requests.slice(0, 10)) {
      if (seenReq.has(r.id)) continue;
      seenReq.add(r.id);
      if (r.status >= 500) {
        add({ kind: "error", text: `${r.method} ${r.path} → ${r.status}` });
      } else if (r.status >= 400) {
        add({ kind: "warn", text: `${r.method} ${r.path} → ${r.status}` });
      }
    }
    // Don't let seenReq grow forever.
    if (seenReq.size > 500) seenReq = new Set([...seenReq].slice(-200));
  });

  function glyph(k: Event["kind"]): string {
    if (k === "error") return "◇";
    if (k === "warn") return "△";
    if (k === "start") return "◉";
    if (k === "quiet") return "·";
    return "○";
  }
</script>

<section class="ledger" aria-label="activity ledger">
  <header class="ledger__head">
    <span class="caps">activity ledger</span>
    <span class="faint mono" style="font-size: var(--fs-xs)">{events.length} / 20</span>
  </header>
  <ol class="ledger__list">
    {#if events.length === 0}
      <li class="ledger__quiet">
        <span class="ledger__dot" aria-hidden="true"></span>
        <span class="faint">awaiting first signal — operator desk lies in wait</span>
      </li>
    {/if}
    {#each events as e (e.id)}
      <li class="ledger__row ledger__row--{e.kind}">
        <span class="ledger__time mono tabular">{shortTime(e.at)}</span>
        <span class="ledger__glyph mono" aria-hidden="true">{glyph(e.kind)}</span>
        <span class="ledger__text">{e.text}</span>
      </li>
    {/each}

    <!-- Skeleton rows — fill the panel visually with ghosted "what could appear here" hints. Not decorative: they teach the operator what the ledger logs. -->
    {#if events.length < 8}
      <li class="ledger__ghost" aria-hidden="true">
        <span class="ledger__time mono tabular">--:--:--</span>
        <span class="ledger__glyph mono">·</span>
        <span class="ledger__text">waiting for event · cooldown arm</span>
      </li>
      <li class="ledger__ghost" aria-hidden="true">
        <span class="ledger__time mono tabular">--:--:--</span>
        <span class="ledger__glyph mono">·</span>
        <span class="ledger__text">waiting for event · request ≥ 400</span>
      </li>
      <li class="ledger__ghost" aria-hidden="true">
        <span class="ledger__time mono tabular">--:--:--</span>
        <span class="ledger__glyph mono">·</span>
        <span class="ledger__text">waiting for event · token refresh</span>
      </li>
      <li class="ledger__ghost" aria-hidden="true">
        <span class="ledger__time mono tabular">--:--:--</span>
        <span class="ledger__glyph mono">·</span>
        <span class="ledger__text">waiting for event · account added</span>
      </li>
      <li class="ledger__ghost" aria-hidden="true">
        <span class="ledger__time mono tabular">--:--:--</span>
        <span class="ledger__glyph mono">·</span>
        <span class="ledger__text">waiting for event · account removed</span>
      </li>
      <li class="ledger__ghost" aria-hidden="true">
        <span class="ledger__time mono tabular">--:--:--</span>
        <span class="ledger__glyph mono">·</span>
        <span class="ledger__text">waiting for event · request ≥ 500</span>
      </li>
      <li class="ledger__ghost" aria-hidden="true">
        <span class="ledger__time mono tabular">--:--:--</span>
        <span class="ledger__glyph mono">·</span>
        <span class="ledger__text">waiting for event · vault rotation</span>
      </li>
      <li class="ledger__ghost" aria-hidden="true">
        <span class="ledger__time mono tabular">--:--:--</span>
        <span class="ledger__glyph mono">·</span>
        <span class="ledger__text">&nbsp;</span>
      </li>
    {/if}
  </ol>
</section>

<style>
  .ledger {
    display: flex;
    flex-direction: column;
    padding: var(--sp-4) var(--sp-5) var(--sp-5);
    background: var(--c-surface);
    border: 1px solid var(--c-border);
    border-radius: var(--r-md);
    box-shadow: var(--sh-1);
  }
  .ledger__head {
    display: flex;
    justify-content: space-between;
    align-items: baseline;
    padding-block-end: var(--sp-3);
    border-block-end: 1px solid var(--c-rule);
    margin-block-end: var(--sp-3);
  }
  .ledger__list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: var(--sp-2);
    font-size: var(--fs-xs);
  }
  .ledger__row {
    display: grid;
    grid-template-columns: 72px 16px 1fr;
    align-items: baseline;
    gap: var(--sp-3);
    font-family: var(--font-mono);
    color: var(--c-text-dim);
    padding-block: 2px;
    border-block-end: 1px dashed color-mix(in oklch, var(--c-rule), transparent 40%);
    animation: slide-up var(--mo-med) var(--ease-out);
  }
  .ledger__row:last-child {
    border-block-end: none;
  }
  .ledger__time {
    color: var(--c-text-faint);
  }
  .ledger__glyph {
    color: var(--c-text-dim);
    text-align: center;
  }
  .ledger__text {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--c-text);
  }
  .ledger__row--start .ledger__glyph {
    color: var(--c-accent);
  }
  .ledger__row--warn .ledger__glyph {
    color: var(--c-warn);
  }
  .ledger__row--warn .ledger__text {
    color: var(--c-warn);
  }
  .ledger__row--error .ledger__glyph {
    color: var(--c-danger);
  }
  .ledger__row--error .ledger__text {
    color: var(--c-danger);
  }
  .ledger__quiet {
    display: flex;
    align-items: center;
    gap: var(--sp-3);
    padding: var(--sp-4) 0;
    font-family: var(--font-mono);
    font-size: var(--fs-xs);
  }
  .ledger__ghost {
    display: grid;
    grid-template-columns: 72px 16px 1fr;
    align-items: baseline;
    gap: var(--sp-3);
    font-family: var(--font-mono);
    font-size: var(--fs-xs);
    color: var(--c-text-ghost);
    opacity: 0.6;
    padding-block: 2px;
  }
  .ledger__ghost .ledger__time,
  .ledger__ghost .ledger__glyph,
  .ledger__ghost .ledger__text {
    color: inherit;
  }
  .ledger__dot {
    inline-size: 8px;
    block-size: 8px;
    border-radius: var(--r-pill);
    background: var(--c-accent);
    animation: pulse-ring 2.4s var(--ease-out) infinite;
  }
</style>
