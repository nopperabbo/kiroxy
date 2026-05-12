/**
 * kiroxy icon manifest — typed export of the curated icon set.
 * ----------------------------------------------------------------------------
 * Source: tools/icons/*.svg
 * Spec:   docs/ICONOGRAPHY.md
 *
 * Usage (framework-agnostic):
 *   import { getIcon } from "../tools/icons/icons";
 *   el.innerHTML = getIcon("refresh", { size: 16 });
 *
 * Usage (JSX/TSX, when kiroxy dashboard bundles inline-SVG imports):
 *   import refresh from "./refresh.svg?raw";
 *   <span dangerouslySetInnerHTML={{ __html: refresh }} />
 *
 * DESIGN SYSTEM RULE: Every icon used in kiroxy MUST appear in this manifest.
 * If you need a new icon, go through docs/ICONOGRAPHY.md "How to add a new
 * icon" before adding it to the `icons` map here.
 */

/** Every icon kiroxy ships or plans to ship. */
export type IconName =
  // status
  | "status-healthy" | "status-cooldown" | "status-failed" | "status-refreshing" | "status-unknown"
  // actions
  | "refresh" | "remove" | "disable" | "enable" | "import" | "export" | "copy" | "search"
  // navigation
  | "chevron-left" | "chevron-right" | "chevron-up" | "chevron-down" | "close" | "menu"
  // semantic
  | "info" | "warning" | "error" | "question"
  // data
  | "stream" | "metric" | "log" | "token";

export interface IconManifestEntry {
  /** Whether the source SVG is present in tools/icons/ as of this commit. */
  readonly shipped: boolean;
  /** One-line description of the icon's intended use. */
  readonly description: string;
  /** kiroxy component files that reference this icon. */
  readonly referencedBy: readonly string[];
}

export const icons: Readonly<Record<IconName, IconManifestEntry>> = {
  // ---- status (5) -------------------------------------------------------
  "status-healthy":    { shipped: true,  description: "Account / request OK",        referencedBy: ["status-pill.md", "live-request-stream-block.md"] },
  "status-cooldown":   { shipped: false, description: "Account in backoff",          referencedBy: ["status-pill.md"] },
  "status-failed":     { shipped: false, description: "Auth error / upstream 5xx",   referencedBy: ["status-pill.md", "toast.md"] },
  "status-refreshing": { shipped: false, description: "Token refresh in-flight",     referencedBy: ["status-pill.md", "timeline.md"] },
  "status-unknown":    { shipped: false, description: "State indeterminate",         referencedBy: ["status-pill.md"] },

  // ---- actions (8) ------------------------------------------------------
  "refresh": { shipped: true,  description: "Refresh pool / token",            referencedBy: ["button.md", "command-palette.md", "empty-state.md"] },
  "remove":  { shipped: false, description: "Delete / drop account",           referencedBy: ["button.md", "command-palette.md"] },
  "disable": { shipped: false, description: "Pause account (reversible)",      referencedBy: ["button.md"] },
  "enable":  { shipped: false, description: "Resume account",                  referencedBy: ["button.md"] },
  "import":  { shipped: false, description: "Import accounts from JSON",       referencedBy: ["empty-state.md", "dialog.md"] },
  "export":  { shipped: false, description: "Export accounts to JSON",         referencedBy: ["button.md"] },
  "copy":    { shipped: false, description: "Copy to clipboard",               referencedBy: ["copyable-value.md", "empty-state.md", "button.md"] },
  "search":  { shipped: false, description: "Focus search input",              referencedBy: ["input.md", "table.md"] },

  // ---- navigation (6) ---------------------------------------------------
  "chevron-left":  { shipped: false, description: "Back, previous",                   referencedBy: ["timeline.md", "dialog.md"] },
  "chevron-right": { shipped: true,  description: "Drill affordance, forward",        referencedBy: ["live-request-stream-block.md", "table.md", "timeline.md"] },
  "chevron-up":    { shipped: false, description: "Collapse, sort-asc glyph",         referencedBy: ["table.md"] },
  "chevron-down":  { shipped: false, description: "Expand, sort-desc glyph",          referencedBy: ["select.md", "table.md"] },
  "close":         { shipped: true,  description: "Dismiss dialog / clear filter",    referencedBy: ["dialog.md", "toast.md", "input.md"] },
  "menu":          { shipped: false, description: "More actions trigger",             referencedBy: ["popover.md"] },

  // ---- semantic (4) -----------------------------------------------------
  "info":     { shipped: true,  description: "Tooltip trigger, info toast",      referencedBy: ["tooltip.md", "toast.md", "popover.md"] },
  "warning":  { shipped: false, description: "Warning toast / cooldown",         referencedBy: ["toast.md", "empty-state.md", "status-pill.md"] },
  "error":    { shipped: false, description: "Error toast / validation",         referencedBy: ["toast.md", "input.md"] },
  "question": { shipped: false, description: "Help trigger / `?` cheatsheet",    referencedBy: ["tooltip.md", "command-palette.md"] },

  // ---- data (4) ---------------------------------------------------------
  "stream": { shipped: true,  description: "LiveRequestStream nav, activity",  referencedBy: ["command-palette.md", "empty-state.md"] },
  "metric": { shipped: false, description: "Metrics route nav",                referencedBy: ["command-palette.md"] },
  "log":    { shipped: false, description: "Logs route nav, view-logs action", referencedBy: ["command-palette.md", "live-request-stream-block.md"] },
  "token":  { shipped: false, description: "Token / key affordance",           referencedBy: ["copyable-value.md"] },
};

/** Convenience: list of icons currently on disk. */
export const shippedIcons: readonly IconName[] =
  (Object.entries(icons) as [IconName, IconManifestEntry][])
    .filter(([, entry]) => entry.shipped)
    .map(([name]) => name);

/** Convenience: list of icons specified but not yet sourced. */
export const deferredIcons: readonly IconName[] =
  (Object.entries(icons) as [IconName, IconManifestEntry][])
    .filter(([, entry]) => !entry.shipped)
    .map(([name]) => name);

/**
 * Bundler-helper signature. The actual implementation is up to the bundler
 * (Vite `?raw` suffix, Webpack `raw-loader`, esbuild `loader:text`). This
 * function documents the contract consumers should expose.
 */
export interface GetIconOptions {
  /** Pixel size; sets width + height. Default 16. */
  size?: 14 | 16 | 20 | 24 | 32;
  /** Stroke width override; default 1.5. */
  strokeWidth?: number;
  /** `aria-hidden="true"` (decorative) vs aria-label (meaningful). */
  title?: string;
  /** Additional class names applied to the <svg> element. */
  className?: string;
}
