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
};
