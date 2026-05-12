/**
 * Shared types mirroring Go's server types.
 *
 * Keep keys aligned with:
 *   internal/server/dashboard.go         (DashboardState, DashboardAccount)
 *   internal/server/dashboard_sink.go    (RequestRecord)
 *   internal/server/dashboard_v2.go      (DashboardImportEntry, DashboardImportResult)
 *
 * These are snake_case as received from Go's default JSON tags.
 */

export interface Account {
  id: string;
  enabled: boolean;
  requests: number;
  errors: number;
  cooldown_until?: string;
  last_error?: string;
  provider?: string;
  region?: string;
  auth_method?: string;
  last_used?: string;
}

export interface Snapshot {
  version: string;
  uptime_s: number;
  ready: boolean;
  ready_detail?: string;
  vault_ok: boolean;
  vault_path?: string;
  accounts: Account[];
  total_requests: number;
  total_errors: number;
  error_rate: number;
}

export interface RequestRecord {
  id: string;
  started_at: string;
  latency_ms: number;
  method: string;
  path: string;
  status: number;
  bytes_out: number;
  remote_ip?: string;
  user_agent?: string;
}

export interface ImportEntry {
  provider: string;
  authMethod: string;
  accessToken: string;
  refreshToken: string;
  profileArn?: string;
  expiresIn: number;
  addedAt?: string;
}

export interface ImportResult {
  index: number;
  id?: string;
  status: "added" | "updated" | "skipped" | string;
  reason?: string;
}

export type Theme = "system" | "dark" | "light";

export type AccountStatus = "healthy" | "cooldown" | "disabled" | "error";

export function accountStatus(a: Account): AccountStatus {
  if (!a.enabled) return "disabled";
  if (a.cooldown_until) {
    const t = Date.parse(a.cooldown_until);
    if (!Number.isNaN(t) && t > Date.now()) return "cooldown";
  }
  if (a.errors > 0 && a.requests > 0 && a.errors / a.requests > 0.25) {
    return "error";
  }
  return "healthy";
}

/** Empty snapshot used before the first SSE message arrives. */
export const emptySnapshot: Snapshot = {
  version: "",
  uptime_s: 0,
  ready: false,
  vault_ok: false,
  accounts: [],
  total_requests: 0,
  total_errors: 0,
  error_rate: 0,
};
