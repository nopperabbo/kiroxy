/**
 * Global reactive state. Svelte 5 runes in a module-level class give us
 * shared-singleton state that's fully reactive and type-safe. No external
 * state library.
 *
 * Usage from components:
 *   import { store } from "../lib/stores.svelte";
 *   // in a .svelte: { #each store.accounts as a }
 *   // writes: store.applySnapshot(snap)
 *
 * The .svelte.ts extension opts this file into the runes syntax.
 */

import type { RequestRecord, Snapshot, Account } from "./types";
import { emptySnapshot } from "./types";
import type { SseStatus } from "./sse";

const MAX_REQUESTS = 50;

class Store {
  snapshot = $state<Snapshot>(emptySnapshot);
  requests = $state<RequestRecord[]>([]);
  sseStatus = $state<SseStatus>("connecting");
  lastUpdated = $state<number>(0);

  // Toast-style transient notifications. Each has auto-dismiss handled by the
  // component that renders them.
  toasts = $state<Array<{ id: number; kind: "ok" | "err"; msg: string }>>([]);
  private toastSeq = 0;

  applySnapshot(s: Snapshot): void {
    // Sort accounts by last_used desc, falling back to id for stable order.
    const accts = [...s.accounts].sort(compareAccounts);
    this.snapshot = { ...s, accounts: accts };
    this.lastUpdated = Date.now();
  }

  appendRequest(r: RequestRecord): void {
    // Rolling window, newest-first.
    const next = [r, ...this.requests];
    if (next.length > MAX_REQUESTS) next.length = MAX_REQUESTS;
    this.requests = next;
  }

  replaceRequests(list: RequestRecord[]): void {
    this.requests = list.slice(0, MAX_REQUESTS);
  }

  pushToast(kind: "ok" | "err", msg: string): void {
    const id = ++this.toastSeq;
    this.toasts = [...this.toasts, { id, kind, msg }];
    setTimeout(() => {
      this.toasts = this.toasts.filter((t) => t.id !== id);
    }, 4000);
  }
}

function compareAccounts(a: Account, b: Account): number {
  // Most recently used first.
  const ta = a.last_used ? Date.parse(a.last_used) : 0;
  const tb = b.last_used ? Date.parse(b.last_used) : 0;
  if (ta !== tb) return tb - ta;
  return a.id.localeCompare(b.id);
}

export const store = new Store();
