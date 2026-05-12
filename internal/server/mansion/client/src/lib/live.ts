/**
 * LiveSource — unified data feed.
 *
 * Responsibilities:
 *   1. Poll /dashboard/api/state on an interval (default 2s).
 *   2. Opportunistically try SSE at /dashboard/api/stream; if it works,
 *      downgrade polling to slow background (15s) as a safety net.
 *   3. Best-effort fetch /dashboard/api/requests once at boot for history.
 *   4. Synthesize synthetic RequestRecord entries when the account
 *      counters in a new snapshot show deltas. This keeps the request
 *      feed alive even when Phase H's recent-requests endpoint is 404.
 *
 * All event emission is push-based (onSnapshot / onRequest / onStatus)
 * so consumers don't poll internal state.
 */

import { api } from "./api";
import type { Account, RequestRecord, Snapshot } from "./types";
import { ulidLite } from "./format";

export type LiveStatus =
  | "connecting"
  | "stream" // SSE is delivering events
  | "polling" // falling back to fetch /state on an interval
  | "reconnecting"
  | "offline"; // /state failed N times in a row

export interface LiveHandlers {
  onSnapshot: (s: Snapshot) => void;
  onRequest: (r: RequestRecord) => void;
  onStatus: (s: LiveStatus) => void;
}

const POLL_FAST_MS = 2_000;
const POLL_SLOW_MS = 15_000;
const SSE_WATCHDOG_MS = 4_000;
const OFFLINE_AFTER_CONSEC_FAIL = 3;

export class LiveSource {
  private closed = false;
  private es: EventSource | null = null;
  private timer: ReturnType<typeof setTimeout> | null = null;
  private watchdog: ReturnType<typeof setTimeout> | null = null;
  private consecutiveFail = 0;
  private hadStream = false;

  /** Previous snapshot's per-account counters keyed by id. Used to
   *  synthesize RequestRecord deltas when /requests is unavailable. */
  private prevCounters = new Map<string, { req: number; err: number }>();

  constructor(private readonly handlers: LiveHandlers) {}

  start(): void {
    if (this.closed) return;
    this.handlers.onStatus("connecting");
    // Kick both channels in parallel. Whichever succeeds first wins.
    void this.bootstrap();
  }

  close(): void {
    this.closed = true;
    this.clearWatchdog();
    this.clearTimer();
    if (this.es) {
      this.es.close();
      this.es = null;
    }
  }

  /** One-shot manual refresh — used by the palette "reload" action. */
  async refreshNow(): Promise<void> {
    const [s, r] = await Promise.all([api.state(), api.requests()]);
    if (s.ok) this.ingestSnapshot(s.data, "poll-manual");
    if (r.ok && Array.isArray(r.data)) {
      for (const rec of r.data) this.handlers.onRequest(rec);
    }
  }

  // ─── Boot ───────────────────────────────────────────────────────

  private async bootstrap(): Promise<void> {
    // Fire the first poll immediately so the UI has data in <200ms even
    // if SSE is available but takes longer to open. Also primes delta
    // tracking so synth records start after the first snapshot.
    const first = await api.state();
    if (first.ok) {
      this.consecutiveFail = 0;
      this.ingestSnapshot(first.data, "boot");
    } else {
      this.handleFail();
    }

    // Best-effort history fetch; failures are silent.
    void (async () => {
      const h = await api.requests();
      if (h.ok && Array.isArray(h.data)) {
        for (const rec of h.data) this.handlers.onRequest(rec);
      }
    })();

    // Try to upgrade to SSE.
    this.openStream();

    // Always arm polling — it's the floor, SSE is a bonus.
    this.schedulePoll(POLL_FAST_MS);
  }

  // ─── SSE ────────────────────────────────────────────────────────

  private openStream(): void {
    try {
      const es = new EventSource("/dashboard/api/stream");
      this.es = es;

      es.addEventListener("snapshot", (ev) => {
        this.markStreaming();
        try {
          const s = JSON.parse((ev as MessageEvent).data) as Snapshot;
          this.ingestSnapshot(s, "sse");
        } catch {
          /* malformed frame — skip */
        }
      });
      es.addEventListener("request", (ev) => {
        try {
          const rec = JSON.parse((ev as MessageEvent).data) as RequestRecord;
          this.handlers.onRequest(rec);
        } catch {
          /* ignore */
        }
      });
      es.addEventListener("open", () => this.markStreaming());
      es.addEventListener("error", () => {
        if (this.closed) return;
        if (this.hadStream) this.handlers.onStatus("reconnecting");
        // Let the watchdog downgrade to polling-only if we never streamed.
      });

      this.watchdog = setTimeout(() => {
        if (this.closed) return;
        if (!this.hadStream) {
          // No SSE from this backend. Close, stick with polling, and
          // avoid the reconnect chatter of leaving EventSource alive.
          es.close();
          this.es = null;
          this.handlers.onStatus("polling");
        }
      }, SSE_WATCHDOG_MS);
    } catch {
      // EventSource construction failed synchronously (ancient browser).
      this.handlers.onStatus("polling");
    }
  }

  private markStreaming(): void {
    this.hadStream = true;
    this.clearWatchdog();
    this.handlers.onStatus("stream");
    // SSE works — back off the poller to a safety net.
    this.clearTimer();
    this.schedulePoll(POLL_SLOW_MS);
  }

  private clearWatchdog(): void {
    if (this.watchdog) {
      clearTimeout(this.watchdog);
      this.watchdog = null;
    }
  }

  // ─── Polling ────────────────────────────────────────────────────

  private schedulePoll(ms: number): void {
    this.clearTimer();
    this.timer = setTimeout(() => void this.tickPoll(), ms);
  }

  private clearTimer(): void {
    if (this.timer) {
      clearTimeout(this.timer);
      this.timer = null;
    }
  }

  private async tickPoll(): Promise<void> {
    if (this.closed) return;
    const res = await api.state();
    if (res.ok) {
      this.consecutiveFail = 0;
      this.ingestSnapshot(res.data, "poll");
      if (!this.hadStream) this.handlers.onStatus("polling");
    } else {
      this.handleFail();
    }
    const delay = this.hadStream ? POLL_SLOW_MS : POLL_FAST_MS;
    this.schedulePoll(delay);
  }

  private handleFail(): void {
    this.consecutiveFail += 1;
    if (this.consecutiveFail >= OFFLINE_AFTER_CONSEC_FAIL) {
      this.handlers.onStatus("offline");
    } else {
      this.handlers.onStatus("reconnecting");
    }
  }

  // ─── Snapshot ingestion + synthetic request deltas ──────────────

  private ingestSnapshot(s: Snapshot, src: "boot" | "poll" | "poll-manual" | "sse"): void {
    if (src !== "boot") this.synthesizeFromDeltas(s.accounts);
    this.rememberCounters(s.accounts);
    this.handlers.onSnapshot(s);
  }

  private synthesizeFromDeltas(accounts: Account[]): void {
    const now = Date.now();
    for (const a of accounts) {
      const prev = this.prevCounters.get(a.id);
      if (!prev) continue;
      const dReq = a.requests - prev.req;
      const dErr = a.errors - prev.err;
      if (dReq <= 0) continue;
      // Emit at most a few synthetic records per tick — we don't want to
      // stampede the feed if a counter jumps by 50.
      const toEmit = Math.min(dReq, 3);
      for (let i = 0; i < toEmit; i++) {
        const errThisOne = i < Math.min(dErr, toEmit);
        const rec: RequestRecord = {
          id: ulidLite(),
          started_at: new Date(now - i * 120).toISOString(),
          latency_ms: 0, // unknown — renderer shows "—"
          method: "POST",
          path: "/v1/messages",
          status: errThisOne ? 500 : 200,
          bytes_out: 0,
          account_id: a.id,
        };
        this.handlers.onRequest(rec);
      }
    }
  }

  private rememberCounters(accounts: Account[]): void {
    this.prevCounters.clear();
    for (const a of accounts) {
      this.prevCounters.set(a.id, { req: a.requests, err: a.errors });
    }
  }
}
