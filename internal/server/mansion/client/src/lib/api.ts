/**
 * Typed fetch wrappers returning a Result discriminated union so call
 * sites must handle success AND failure. No thrown exceptions.
 *
 * Endpoint availability (current Phase H backend):
 *   GET    /dashboard/api/state                 ← present
 *   GET    /dashboard/api/requests              ← returns 404 today (v1.3 TODO)
 *   POST   /dashboard/api/import                ← returns 404 today (v1.3 TODO)
 *   DELETE /dashboard/api/accounts/{p}/{id}     ← returns 404 today (v1.3 TODO)
 *   GET    /dashboard/api/opencode-config       ← returns 404 today (v1.3 TODO)
 *   (SSE)  /dashboard/api/stream                ← returns 404 today (v1.3 TODO)
 *
 * Mansion polls /state and synthesizes request history client-side from
 * account-counter deltas. When the backend grows the missing endpoints,
 * the UI light up automatically.
 */

import type { ImportEntry, ImportResult, RequestRecord, Snapshot } from "./types";

export type Result<T> =
  | { ok: true; data: T }
  | { ok: false; status: number; error: string };

export interface DocEntry {
  path: string;
  title: string;
  content: string;
  bytes: number;
}

/**
 * One captured slog record from /dashboard/api/logs. Mirrors Go's
 * internal/server/logsink.go LogRecord. ID is monotonic across the
 * process so SSE reconnect can fast-forward via Last-Event-ID.
 */
export interface LogRecord {
  id: number;
  time: string;
  level: "DEBUG" | "INFO" | "WARN" | "ERROR" | string;
  source?: string;
  message: string;
  fields?: Record<string, string>;
}

export interface LogCounters {
  total: number;
  buffered: number;
  capacity: number;
}

export interface LogsSnapshot {
  records: LogRecord[];
  counters: LogCounters;
}

export interface LogsQuery {
  level?: string;
  source?: string;
  search?: string;
  since_id?: number;
  max?: number;
}

/**
 * Mirrors Go's server.InboundKeyView. Plaintext NEVER appears here —
 * it is returned only by createInboundKey() and shown to the operator
 * exactly once.
 */
export interface InboundKeyView {
  id: string;
  label?: string;
  tail: string;
  created_at?: string;
  last_used_at?: string;
  revoked: boolean;
}

export interface CreatedInboundKey extends InboundKeyView {
  plaintext: string;
}

/**
 * Mirrors Go's server.SettingsSnapshot. The bundled settings response
 * contains everything the SettingsView needs to render its 4 tabs in
 * one fetch.
 */
export interface SettingsSnapshot {
  general: {
    version: string;
    uptime_s: number;
    started_at: string;
    vault_path?: string;
    log_level?: string;
  };
  env_vars: Array<{
    key: string;
    value: string;
    redacted?: boolean;
    present: boolean;
  }>;
  vault: {
    path?: string;
    size_bytes?: number;
    healthy: number;
    cooldown: number;
    disabled: number;
    total: number;
  };
  inbound_keys: {
    active: number;
    total: number;
  };
}

/**
 * One doctor check result. Mirrors Go's internal/doctor.Result.
 */
export interface DoctorResult {
  name: string;
  status: "ok" | "warn" | "error" | "skip";
  detail: string;
  hint?: string;
  elapsed_ns?: number;
}

export interface DoctorReport {
  ok: boolean;
  started_at: string;
  elapsed: string;
  results: DoctorResult[];
  go_version?: string;
}

async function jsonFetch<T>(input: RequestInfo, init?: RequestInit): Promise<Result<T>> {
  try {
    const res = await fetch(input, {
      headers: { Accept: "application/json", ...(init?.headers ?? {}) },
      ...init,
    });
    if (!res.ok) {
      let msg = `HTTP ${res.status}`;
      try {
        const body = (await res.clone().json()) as { error?: unknown };
        if (body && typeof body === "object" && typeof body.error === "string") {
          msg = body.error;
        }
      } catch {
        try {
          const txt = await res.text();
          if (txt) msg = txt.slice(0, 240);
        } catch {
          /* give up, keep default msg */
        }
      }
      return { ok: false, status: res.status, error: msg };
    }
    if (res.status === 204) {
      return { ok: true, data: undefined as T };
    }
    const data = (await res.json()) as T;
    return { ok: true, data };
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    return { ok: false, status: 0, error: msg };
  }
}

export const api = {
  state(): Promise<Result<Snapshot>> {
    return jsonFetch<Snapshot>("/dashboard/api/state");
  },
  requests(): Promise<Result<RequestRecord[]>> {
    return jsonFetch<RequestRecord[]>("/dashboard/api/requests");
  },
  importAccounts(entries: ImportEntry[]): Promise<Result<{ results: ImportResult[] }>> {
    return jsonFetch<{ results: ImportResult[] }>("/dashboard/api/import", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(entries),
    });
  },
  removeAccount(provider: string, id: string): Promise<Result<void>> {
    const p = encodeURIComponent(provider);
    const i = encodeURIComponent(id);
    return jsonFetch<void>(`/dashboard/api/accounts/${p}/${i}`, {
      method: "DELETE",
    });
  },
  opencodeConfig(): Promise<Result<unknown>> {
    return jsonFetch<unknown>("/dashboard/api/opencode-config");
  },
  docsIndex(): Promise<Result<{ docs: DocEntry[] }>> {
    return jsonFetch<{ docs: DocEntry[] }>("/dashboard/api/docs/index");
  },
  logs(q: LogsQuery = {}): Promise<Result<LogsSnapshot>> {
    return jsonFetch<LogsSnapshot>("/dashboard/api/logs" + buildQuery(q));
  },
  /**
   * Returns the SSE URL (callers wrap in EventSource themselves so the
   * lifecycle stays explicit). Filter params are applied server-side.
   */
  logsStreamURL(q: LogsQuery = {}): string {
    return "/dashboard/api/logs/stream" + buildQuery(q);
  },
  settings(): Promise<Result<SettingsSnapshot>> {
    return jsonFetch<SettingsSnapshot>("/dashboard/api/settings");
  },
  listInboundKeys(): Promise<Result<{ keys: InboundKeyView[] }>> {
    return jsonFetch<{ keys: InboundKeyView[] }>("/dashboard/api/inbound-keys");
  },
  createInboundKey(label: string): Promise<Result<CreatedInboundKey>> {
    return jsonFetch<CreatedInboundKey>("/dashboard/api/inbound-keys", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ label }),
    });
  },
  revokeInboundKey(id: string): Promise<Result<void>> {
    return jsonFetch<void>(`/dashboard/api/inbound-keys/${encodeURIComponent(id)}`, {
      method: "DELETE",
    });
  },
  doctor(): Promise<Result<DoctorReport>> {
    return jsonFetch<DoctorReport>("/dashboard/api/tools/doctor", { method: "POST" });
  },
};

function buildQuery(q: LogsQuery): string {
  const params = new URLSearchParams();
  for (const [k, v] of Object.entries(q)) {
    if (v == null || v === "") continue;
    params.set(k, String(v));
  }
  const s = params.toString();
  return s ? "?" + s : "";
}
