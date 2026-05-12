/**
 * Shared types mirroring Go's server shapes. Keys stay snake_case because
 * Go's default JSON tags emit snake_case. Aligned with:
 *   internal/server/dashboard.go         — DashboardState, DashboardAccount
 *   internal/server/dashboard_sink.go    — RequestRecord
 *
 * The backend schema lives in those files; if you touch them, touch this
 * file too. Missing optional keys are tolerated at runtime; new required
 * keys must land here or the typecheck breaks.
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
  /** Optional upstream-token expiry, if the backend exposes it. */
  expires_at?: string;
}

export interface Snapshot {
  version: string;
  uptime_s: number;
  ready: boolean;
  ready_detail?: string;
  vault_ok: boolean;
  vault_path?: string;
  accounts: Account[];
  total_requests?: number;
  total_errors?: number;
  error_rate?: number;
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
  /** Client-derived: account id that served this request. Not populated
   *  by the current backend — we enrich client-side when we learn it. */
  account_id?: string;
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

export type AccountStatus = "healthy" | "cooldown" | "disabled" | "error" | "warm";

export function accountStatus(a: Account): AccountStatus {
  if (!a.enabled) return "disabled";
  if (a.cooldown_until) {
    const t = Date.parse(a.cooldown_until);
    if (!Number.isNaN(t) && t > Date.now()) return "cooldown";
  }
  if (a.errors > 0 && a.requests > 0 && a.errors / a.requests > 0.25) {
    return "error";
  }
  // "warm" — has served traffic recently but no errors. Purely a visual
  // cue; fall through to healthy if not warm enough.
  if (a.last_used) {
    const ago = Date.now() - Date.parse(a.last_used);
    if (!Number.isNaN(ago) && ago < 60_000) return "warm";
  }
  return "healthy";
}

export const emptySnapshot: Snapshot = {
  version: "",
  uptime_s: 0,
  ready: false,
  vault_ok: false,
  accounts: [],
};

/**
 * Lifecycle phase breakdown for the signature drill-down view. Since the
 * backend doesn't emit per-phase timings yet, we SYNTHESIZE them from the
 * total latency using plausible proportions tuned to what the pool
 * typically does. When the backend gains phase instrumentation (tracked
 * as v1.3 in DASHBOARD_MANSION.md), swap synthesizePhases() for a direct
 * fetch.
 */
export interface Phase {
  name: string;
  hint: string;
  ms: number;
}
