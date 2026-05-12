/**
 * Tiny formatters. Stay dependency-free — these ship in the bundle and
 * every KB counts. Shared across components so copy is consistent.
 */

const rtfShort = new Intl.RelativeTimeFormat(undefined, { style: "short" });

export function relTime(iso: string | undefined | null): string {
  if (!iso) return "—";
  const t = Date.parse(iso);
  if (Number.isNaN(t)) return "—";
  return relMillis(t - Date.now());
}

export function relMillis(deltaMs: number): string {
  const s = Math.round(deltaMs / 1000);
  if (Math.abs(s) < 1) return "now";
  if (Math.abs(s) < 60) return rtfShort.format(s, "second");
  const m = Math.round(s / 60);
  if (Math.abs(m) < 60) return rtfShort.format(m, "minute");
  const h = Math.round(m / 60);
  if (Math.abs(h) < 24) return rtfShort.format(h, "hour");
  const d = Math.round(h / 24);
  return rtfShort.format(d, "day");
}

export function shortTime(iso: string | undefined | null): string {
  if (!iso) return "—";
  const t = Date.parse(iso);
  if (Number.isNaN(t)) return "—";
  const d = new Date(t);
  return d.toLocaleTimeString(undefined, {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  });
}

export function fmtBytes(n: number | undefined | null): string {
  if (n == null || !Number.isFinite(n)) return "—";
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KiB`;
  if (n < 1024 * 1024 * 1024) return `${(n / (1024 * 1024)).toFixed(1)} MiB`;
  return `${(n / (1024 * 1024 * 1024)).toFixed(2)} GiB`;
}

export function fmtMs(n: number | undefined | null): string {
  if (n == null || !Number.isFinite(n)) return "—";
  if (n <= 0) return "—";
  if (n < 10) return `${n.toFixed(1)} ms`;
  if (n < 1000) return `${Math.round(n)} ms`;
  return `${(n / 1000).toFixed(2)} s`;
}

export function fmtPct(n: number | undefined | null, digits = 1): string {
  if (n == null || !Number.isFinite(n)) return "—";
  return `${(n * 100).toFixed(digits)}%`;
}

export function fmtUptime(s: number | undefined | null): string {
  if (s == null || !Number.isFinite(s) || s < 0) return "—";
  const days = Math.floor(s / 86400);
  const hrs = Math.floor((s % 86400) / 3600);
  const mins = Math.floor((s % 3600) / 60);
  const secs = s % 60;
  if (days > 0) return `${days}d ${hrs}h ${mins}m`;
  if (hrs > 0) return `${hrs}h ${mins}m`;
  if (mins > 0) return `${mins}m ${secs}s`;
  return `${secs}s`;
}

export function truncate(s: string, n: number): string {
  if (s.length <= n) return s;
  return s.slice(0, n - 1) + "…";
}

export function abbrId(id: string, keep = 8): string {
  if (id.length <= keep) return id;
  return id.slice(0, keep);
}

/** Millisecond diff between two ISO strings, NaN-safe. */
export function msBetween(a: string | undefined, b: string | undefined): number {
  if (!a || !b) return NaN;
  return Date.parse(b) - Date.parse(a);
}

/**
 * ULID-lite — 26 chars Crockford-base32, time-prefixed. Used for synthetic
 * client-generated request IDs when we can't fetch /requests. Not a real
 * ULID (no monotonic clock guarantee) but sortable and collision-free
 * enough for dashboard visuals.
 */
const CROCKFORD = "0123456789ABCDEFGHJKMNPQRSTVWXYZ";
export function ulidLite(): string {
  const t = Date.now();
  let time = "";
  let n = t;
  for (let i = 0; i < 10; i++) {
    time = CROCKFORD[n % 32] + time;
    n = Math.floor(n / 32);
  }
  let rand = "";
  const bytes = new Uint8Array(16);
  crypto.getRandomValues(bytes);
  for (let i = 0; i < 16; i++) rand += CROCKFORD[bytes[i] % 32];
  return time + rand;
}
