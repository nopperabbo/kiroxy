/**
 * Global reactive store using Svelte 5 runes ($state).
 *
 * Owns:
 *   - snapshot: latest /dashboard/api/state response
 *   - requests: bounded-depth request feed (newest first)
 *   - perAccountHistory: rolling 5-min window of per-account request counts
 *     for sparklines (signature feature)
 *   - sseStatus: connection lifecycle label
 *   - toasts: user-visible transient messages
 *   - filter / selection state (URL-synced via stateUrl)
 *
 * Shape deliberately flat. No nested reactivity for depth — too easy to
 * break with $state defaults. Mutations go through the named methods so
 * components stay pure consumers.
 */

import type { LiveStatus } from "./live";
import type { Account, RequestRecord, Snapshot } from "./types";
import { emptySnapshot } from "./types";

/** Max requests retained in-memory. 500 is enough for "scrolling through
 *  a quiet afternoon" without turning into a memory footgun. */
const MAX_REQUESTS = 500;

/** Sparkline window: 5 minutes, 30 buckets (10s each). */
export const SPARK_WINDOW_MS = 5 * 60_000;
export const SPARK_BUCKETS = 30;
const SPARK_BUCKET_MS = SPARK_WINDOW_MS / SPARK_BUCKETS;

export interface Toast {
  id: number;
  kind: "ok" | "err" | "info";
  msg: string;
}

export interface Filters {
  search: string;
  onlyErrors: boolean;
  onlyCooldown: boolean;
  statusRange?: "2xx" | "4xx" | "5xx" | "all";
}

/**
 * The three top-level views in the Mansion operator desk. These map 1:1
 * to the tabs in Topbar. "live" is the signature landing view — Warp-
 * inspired request stream with a telemetry rail. "pool" is the ledger-
 * style account table. "metrics" is the analytical KPI tile grid.
 */
export type MansionView = "live" | "pool" | "metrics" | "logs" | "settings" | "tools" | "models";

class Store {
  snapshot: Snapshot = $state(emptySnapshot);
  requests: RequestRecord[] = $state([]);
  liveStatus: LiveStatus = $state("connecting");
  toasts: Toast[] = $state([]);
  selectedRequestId: string | null = $state(null);
  selectedAccountId: string | null = $state(null);
  filters: Filters = $state({ search: "", onlyErrors: false, onlyCooldown: false, statusRange: "all" });

  /** Current top-level view. "live" is the default landing (signature
   *  Warp-style request feed). Tabs in Topbar drive this via setView(). */
  view: MansionView = $state("live");
  /** Stream pause toggle (Space key or palette action). When paused, new
   *  SSE requests still arrive in the store but LiveStream does not
   *  auto-scroll or prepend new rows beyond the ring. */
  streamPaused: boolean = $state(false);
  /** Drawer tab when an account is selected. Independent from filters so
   *  operators can switch tabs without losing selection. */
  drawerTab: "overview" | "requests" | "token" | "raw" = $state("overview");

  /** Map<accountId, number[]> — rolling bucket counts for sparklines. */
  perAccountSpark: Record<string, number[]> = $state({});
  /** Last bucket start (Date.now() floored to SPARK_BUCKET_MS). */
  private lastBucketAt = 0;
  /** Last known per-account request total — diff determines spark bucket. */
  private lastAccountCounters = new Map<string, number>();

  /** Total requests delta for the headline chart. */
  totalSpark: number[] = $state(Array(SPARK_BUCKETS).fill(0));
  private lastTotalReq = -1;

  // ─── Setters ────────────────────────────────────────────────────

  applySnapshot(s: Snapshot): void {
    this.snapshot = s;
    this.advanceSpark(s.accounts);
  }

  appendRequest(r: RequestRecord): void {
    const next = [r, ...this.requests];
    if (next.length > MAX_REQUESTS) next.length = MAX_REQUESTS;
    this.requests = next;
  }

  replaceRequests(list: RequestRecord[]): void {
    const sorted = [...list].sort(
      (a, b) => Date.parse(b.started_at) - Date.parse(a.started_at),
    );
    this.requests = sorted.slice(0, MAX_REQUESTS);
  }

  selectRequest(id: string | null): void {
    this.selectedRequestId = id;
  }
  selectAccount(id: string | null): void {
    this.selectedAccountId = id;
  }

  setFilter<K extends keyof Filters>(k: K, v: Filters[K]): void {
    this.filters = { ...this.filters, [k]: v };
  }

  /** Tab switch. Wrapped so hotkeys / palette can route through one place;
   *  View Transitions API is triggered by App.svelte when it observes the
   *  change. Keep this synchronous — the VT contract wants a sync mutation
   *  inside startViewTransition(). */
  setView(v: MansionView): void {
    this.view = v;
  }
  togglePause(): void {
    this.streamPaused = !this.streamPaused;
  }
  setDrawerTab(t: Store["drawerTab"]): void {
    this.drawerTab = t;
  }

  pushToast(kind: Toast["kind"], msg: string): void {
    const id = Date.now() + Math.random();
    this.toasts = [...this.toasts, { id, kind, msg }];
    setTimeout(() => {
      this.toasts = this.toasts.filter((t) => t.id !== id);
    }, 2_800);
  }

  // ─── Sparkline rollover ─────────────────────────────────────────

  private advanceSpark(accounts: Account[]): void {
    const now = Date.now();
    const bucket = Math.floor(now / SPARK_BUCKET_MS) * SPARK_BUCKET_MS;
    if (this.lastBucketAt === 0) {
      this.lastBucketAt = bucket;
    }
    const steps = Math.min(
      SPARK_BUCKETS,
      Math.max(0, Math.floor((bucket - this.lastBucketAt) / SPARK_BUCKET_MS)),
    );
    if (steps > 0) {
      // Shift left by `steps`, filling new buckets with 0.
      for (const id of Object.keys(this.perAccountSpark)) {
        const arr = this.perAccountSpark[id];
        this.perAccountSpark[id] = shiftLeft(arr, steps);
      }
      this.totalSpark = shiftLeft(this.totalSpark, steps);
      this.lastBucketAt = bucket;
    }

    // Credit deltas against the now-current last bucket.
    let totalDelta = 0;
    for (const a of accounts) {
      const prev = this.lastAccountCounters.get(a.id) ?? a.requests;
      const d = Math.max(0, a.requests - prev);
      this.lastAccountCounters.set(a.id, a.requests);
      if (!this.perAccountSpark[a.id]) {
        this.perAccountSpark[a.id] = Array(SPARK_BUCKETS).fill(0);
      }
      const arr = this.perAccountSpark[a.id];
      arr[arr.length - 1] = (arr[arr.length - 1] ?? 0) + d;
      totalDelta += d;
    }
    if (this.lastTotalReq < 0) {
      // First snapshot — seed without emitting a spike.
      this.lastTotalReq = accounts.reduce((s, a) => s + a.requests, 0);
    } else {
      const spark = this.totalSpark.slice();
      spark[spark.length - 1] = (spark[spark.length - 1] ?? 0) + totalDelta;
      this.totalSpark = spark;
      this.lastTotalReq += totalDelta;
    }
    // Prune accounts that disappeared from the snapshot — don't hold
    // stale sparks forever.
    const live = new Set(accounts.map((a) => a.id));
    for (const id of Object.keys(this.perAccountSpark)) {
      if (!live.has(id)) delete this.perAccountSpark[id];
    }
  }
}

function shiftLeft(arr: number[], steps: number): number[] {
  if (steps >= arr.length) return Array(arr.length).fill(0);
  const tail = arr.slice(steps);
  const pad = Array(steps).fill(0);
  return [...tail, ...pad];
}

export const store = new Store();
