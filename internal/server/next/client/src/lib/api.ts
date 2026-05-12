/**
 * Typed fetch wrappers. All responses resolve to a discriminated union so
 * callers must handle both success and failure explicitly — no thrown
 * exceptions to forget to catch.
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
      // Try to extract a structured error payload if server returned JSON.
      let msg = `HTTP ${res.status}`;
      try {
        const body = await res.clone().json();
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
    // 204 No Content is a valid success with no body.
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
