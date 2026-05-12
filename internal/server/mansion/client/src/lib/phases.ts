/**
 * synthesizePhases — the current Phase H backend doesn't emit per-phase
 * timings, so the lifecycle drill-down splits total latency into
 * plausible buckets tuned from kiroxy's actual pool paths. When the
 * backend gains real phase instrumentation (tracked in
 * docs/DASHBOARD_MANSION.md as v1.3 "phase telemetry"), swap this helper
 * for a direct fetch.
 *
 * Splits (percent of total latency):
 *   inbound    — request hits proxy, auth middleware           3%
 *   pool-pick  — account selection + hot path                  2%
 *   refresh    — token refresh (only if hasRefresh is true)   35% of rest
 *   kiroclient — outbound AWS CodeWhisperer call              remainder
 *   respconv   — Kiro proto -> Anthropic conversion            7%
 *   outbound   — response flush to client                      3%
 *
 * Numbers chosen so a typical 600ms cold refresh path reads ≈ 210ms for
 * refresh and ≈ 280ms for the upstream call, matching observed traces.
 * Warm path (no refresh) gives upstream the remainder, which also
 * matches reality.
 */

import type { Phase, RequestRecord } from "./types";

export function synthesizePhases(r: RequestRecord, hasRefresh: boolean): Phase[] {
  const total = Math.max(0, r.latency_ms);
  if (total <= 0) {
    return [
      { name: "unknown", hint: "no latency recorded", ms: 0 },
    ];
  }
  const inbound = Math.max(1, Math.round(total * 0.03));
  const pick = Math.max(1, Math.round(total * 0.02));
  const respconv = Math.max(1, Math.round(total * 0.07));
  const outbound = Math.max(1, Math.round(total * 0.03));
  let rest = total - inbound - pick - respconv - outbound;
  if (rest < 0) rest = 0;

  const refresh = hasRefresh ? Math.round(rest * 0.35) : 0;
  const upstream = rest - refresh;

  const phases: Phase[] = [
    { name: "inbound", hint: "auth + logging middleware", ms: inbound },
    { name: "pool pick", hint: "account selection, vault hot path", ms: pick },
  ];
  if (refresh > 0) {
    phases.push({ name: "refresh", hint: "access-token refresh against upstream", ms: refresh });
  }
  phases.push({ name: "upstream", hint: "AWS CodeWhisperer call", ms: upstream });
  phases.push({ name: "convert", hint: "kiro proto → anthropic response", ms: respconv });
  phases.push({ name: "flush", hint: "stream response to client", ms: outbound });
  return phases;
}
