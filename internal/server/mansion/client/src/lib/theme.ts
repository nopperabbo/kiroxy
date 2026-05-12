/**
 * Theme bootstrap + toggle. The token layer handles both schemes via
 * light-dark(), so all we do here is:
 *   1. Read persisted preference (localStorage key "mansion.scheme").
 *   2. Apply `data-scheme` to <html> at boot (before first paint) so there
 *      is no flash of wrong scheme.
 *   3. Expose setScheme() so a user toggle can flip at runtime.
 *
 * Values: "dark" | "light" | "system" (absent attribute = system).
 */

export type Scheme = "dark" | "light" | "system";

const KEY = "mansion.scheme";

export function readScheme(): Scheme {
  try {
    const v = localStorage.getItem(KEY);
    if (v === "dark" || v === "light" || v === "system") return v;
  } catch {
    /* storage may be disabled; fall through to operator default */
  }
  // Operator Desk is dark-first: first-visit defaults to dark, not OS.
  return "dark";
}

export function setScheme(s: Scheme): void {
  const root = document.documentElement;
  if (s === "system") {
    root.removeAttribute("data-scheme");
  } else {
    root.setAttribute("data-scheme", s);
  }
  try {
    localStorage.setItem(KEY, s);
  } catch {
    /* no-op: storage disabled */
  }
}

export function cycleScheme(current: Scheme): Scheme {
  if (current === "system") return "dark";
  if (current === "dark") return "light";
  return "system";
}

export function initTheme(): void {
  const s = readScheme();
  setScheme(s);
}
