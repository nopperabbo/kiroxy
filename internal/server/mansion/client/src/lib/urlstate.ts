/**
 * URL-persisted filter state. Serializes the mansion `Filters` object
 * into the location hash so views are shareable:
 *
 *   /dashboard-mansion#q=foo&err=1&cool=1&status=5xx
 *
 * Why hash not query? Keeps the browser's back button natural without
 * server round-trips, and the Go handler doesn't care what comes after
 * the `#`. Encoding is intentionally flat (no JSON) so it's human-
 * inspectable in the URL bar.
 */

import type { Filters } from "./store.svelte";

export function filtersToHash(f: Filters): string {
  const parts: string[] = [];
  if (f.search) parts.push(`q=${encodeURIComponent(f.search)}`);
  if (f.onlyErrors) parts.push("err=1");
  if (f.onlyCooldown) parts.push("cool=1");
  if (f.statusRange && f.statusRange !== "all") parts.push(`status=${f.statusRange}`);
  return parts.join("&");
}

export function filtersFromHash(hash: string): Partial<Filters> {
  const raw = hash.startsWith("#") ? hash.slice(1) : hash;
  if (!raw) return {};
  const out: Partial<Filters> = {};
  for (const pair of raw.split("&")) {
    const [k, v = ""] = pair.split("=");
    if (k === "q") out.search = decodeURIComponent(v);
    else if (k === "err") out.onlyErrors = v === "1";
    else if (k === "cool") out.onlyCooldown = v === "1";
    else if (k === "status") {
      if (v === "2xx" || v === "4xx" || v === "5xx" || v === "all") {
        out.statusRange = v;
      }
    }
  }
  return out;
}

export function writeHash(f: Filters): void {
  const next = filtersToHash(f);
  const target = next ? `#${next}` : "";
  if (window.location.hash !== target) {
    // Use replaceState to avoid spamming history on every keystroke.
    window.history.replaceState(null, "", window.location.pathname + window.location.search + target);
  }
}

export function readHash(): Partial<Filters> {
  return filtersFromHash(window.location.hash);
}
