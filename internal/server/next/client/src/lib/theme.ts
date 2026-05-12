/**
 * Theme persistence and system-preference detection.
 *
 * The CSS layer handles all actual color choices via `light-dark()`. This
 * module only flips `<html data-theme="...">` and `color-scheme`.
 *
 * States:
 *   "system" — follow prefers-color-scheme
 *   "dark"   — force dark
 *   "light"  — force light
 */

import type { Theme } from "./types";

const STORAGE_KEY = "kiroxy-next-theme";

export function loadTheme(): Theme {
  try {
    const v = localStorage.getItem(STORAGE_KEY);
    if (v === "dark" || v === "light" || v === "system") return v;
  } catch {
    /* storage denied — use default */
  }
  return "system";
}

export function applyTheme(theme: Theme): void {
  const root = document.documentElement;
  if (theme === "system") {
    root.removeAttribute("data-theme");
  } else {
    root.setAttribute("data-theme", theme);
  }
  try {
    localStorage.setItem(STORAGE_KEY, theme);
  } catch {
    /* ignore */
  }
}

export function nextTheme(current: Theme): Theme {
  return current === "system" ? "dark" : current === "dark" ? "light" : "system";
}

/** Starts a view transition if available; otherwise applies synchronously. */
export function applyThemeWithTransition(theme: Theme): void {
  const doc = document as Document & {
    startViewTransition?: (cb: () => void) => { finished: Promise<void> };
  };
  if (typeof doc.startViewTransition === "function") {
    doc.startViewTransition(() => applyTheme(theme));
  } else {
    applyTheme(theme);
  }
}
