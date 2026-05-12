/**
 * SSE manager. Wraps native EventSource with:
 *   - Automatic reconnect (EventSource does this; we surface the state).
 *   - Typed event handlers.
 *   - Fallback to polling /dashboard/api/state if the stream fails to
 *     produce a first event within a watchdog timeout.
 *   - Clean shutdown via close().
 *
 * Events consumed:
 *   snapshot  — full Snapshot JSON
 *   request   — single RequestRecord
 */

import { api } from "./api";
import type { RequestRecord, Snapshot } from "./types";

export type SseStatus = "connecting" | "open" | "reconnecting" | "polling" | "closed";

export interface SseHandlers {
  onSnapshot: (s: Snapshot) => void;
  onRequest: (r: RequestRecord) => void;
  onStatus: (s: SseStatus) => void;
}

const WATCHDOG_MS = 3500;
const POLL_INTERVAL_MS = 2500;

export class Sse {
  private es: EventSource | null = null;
  private watchdog: ReturnType<typeof setTimeout> | null = null;
  private pollTimer: ReturnType<typeof setInterval> | null = null;
  private gotFirstEvent = false;
  private closed = false;

  constructor(private readonly handlers: SseHandlers) {}

  start(): void {
    if (this.closed) return;
    this.handlers.onStatus("connecting");
    this.openStream();
    this.armWatchdog();
  }

  close(): void {
    this.closed = true;
    this.clearWatchdog();
    this.stopPolling();
    if (this.es) {
      this.es.close();
      this.es = null;
    }
    this.handlers.onStatus("closed");
  }

  private openStream(): void {
    const es = new EventSource("/dashboard/api/stream");
    this.es = es;

    es.addEventListener("snapshot", (ev) => {
      this.gotFirstEvent = true;
      this.clearWatchdog();
      this.stopPolling();
      try {
        const data = JSON.parse((ev as MessageEvent).data) as Snapshot;
        this.handlers.onSnapshot(data);
      } catch {
        /* malformed payload — ignore one event */
      }
    });

    es.addEventListener("request", (ev) => {
      try {
        const data = JSON.parse((ev as MessageEvent).data) as RequestRecord;
        this.handlers.onRequest(data);
      } catch {
        /* ignore */
      }
    });

    es.addEventListener("open", () => {
      this.handlers.onStatus("open");
    });

    es.addEventListener("error", () => {
      if (this.closed) return;
      // EventSource auto-reconnects; signal the UI.
      this.handlers.onStatus("reconnecting");
      if (!this.gotFirstEvent) {
        // Still never reached the server; lean on the watchdog to kick polling.
      }
    });
  }

  private armWatchdog(): void {
    this.clearWatchdog();
    this.watchdog = setTimeout(() => {
      if (this.gotFirstEvent || this.closed) return;
      // SSE never produced a first event — fall back to polling.
      this.handlers.onStatus("polling");
      if (this.es) {
        this.es.close();
        this.es = null;
      }
      this.startPolling();
    }, WATCHDOG_MS);
  }

  private clearWatchdog(): void {
    if (this.watchdog) {
      clearTimeout(this.watchdog);
      this.watchdog = null;
    }
  }

  private startPolling(): void {
    if (this.pollTimer) return;
    const tick = async (): Promise<void> => {
      if (this.closed) return;
      const res = await api.state();
      if (res.ok) this.handlers.onSnapshot(res.data);
    };
    void tick();
    this.pollTimer = setInterval(() => void tick(), POLL_INTERVAL_MS);
  }

  private stopPolling(): void {
    if (this.pollTimer) {
      clearInterval(this.pollTimer);
      this.pollTimer = null;
    }
  }
}
