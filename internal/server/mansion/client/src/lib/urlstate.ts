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

import type { Filters, MansionView } from "./store.svelte";

const KNOWN_VIEWS: MansionView[] = ["live", "pool", "metrics", "logs", "settings", "tools", "models"];

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

/**
 * Read the "view=..." parameter from the location hash. Unknown values
 * are ignored so an injected or typo'd hash can't break the UI.
 */
export function viewFromHash(hash: string): MansionView | null {
  const raw = hash.startsWith("#") ? hash.slice(1) : hash;
  for (const pair of raw.split("&")) {
    const [k, v = ""] = pair.split("=");
    if (k === "view" && (KNOWN_VIEWS as string[]).includes(v)) {
      return v as MansionView;
    }
  }
  return null;
}

export function writeHash(f: Filters, view?: MansionView): void {
  const parts: string[] = [];
  if (view && view !== "live") parts.push(`view=${view}`);
  const filters = filtersToHash(f);
  if (filters) parts.push(filters);
  const next = parts.join("&");
  const target = next ? `#${next}` : "";
  if (window.location.hash !== target) {
    // replaceState avoids spamming history on every keystroke.
    window.history.replaceState(null, "", window.location.pathname + window.location.search + target);
  }
}

export function readHash(): Partial<Filters> {
  return filtersFromHash(window.location.hash);
}
