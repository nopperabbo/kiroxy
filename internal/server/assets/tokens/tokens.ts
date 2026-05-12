/**
 * kiroxy design tokens — typed TypeScript exports
 * ----------------------------------------------------------------------------
 * Source of truth: docs/DESIGN_SYSTEM.md
 * Runtime counterpart: tokens.css (CSS custom properties)
 * Machine-readable: tokens.json (W3C DTCG format)
 *
 * This file exists so:
 *   - Components can reference tokens by typed name (no stringly-typed
 *     className soup).
 *   - Lint rules can detect out-of-system values at compile time.
 *   - Future Style-Dictionary / Figma-sync tools have a typed entry point.
 *
 * Usage:
 *   import { tokens } from "./tokens";
 *   element.style.setProperty("color", tokens.color.textDefault.cssVar);
 *
 * Every `cssVar` value maps 1:1 to a CSS custom property declared in
 * tokens.css. When a component wants the resolved value at runtime:
 *   getComputedStyle(element).getPropertyValue(tokens.color.bg.cssVar);
 *
 * Do NOT edit tokens here without mirroring the change to tokens.css and
 * tokens.json. The three files are authoritative in lockstep.
 */

/* ============================================================================
 * TYPES
 * ========================================================================== */

export interface Token<TValue = string> {
  /** CSS custom property name, including leading `--`. Use with `var(...)`. */
  readonly cssVar: string;
  /** Canonical value at theme=dark (default). Light/dark-dimmed/HC override via CSS. */
  readonly value: TValue;
  /** Short description for tooling (Figma sync, tokens.json). */
  readonly description: string;
}

type Theme = "dark" | "dark-dimmed" | "light" | "dark-highcontrast" | "light-highcontrast";
type Density = "comfortable" | "compact";

/* ============================================================================
 * COLOR TOKENS
 * ==========================================================================
 * All values are canonical dark-theme. Light and high-contrast are declared
 * in tokens.css; this file gives the *semantic name* components should use.
 * See DESIGN_SYSTEM.md §2.
 */

export const color = {
  /** Canvas — the deepest background. */
  bg:             { cssVar: "--color-bg",             value: "oklch(0.145 0.005 285)", description: "Canvas background (deepest layer)" },
  /** Cards, table rows, sidebar surfaces. */
  surface:        { cssVar: "--color-surface",        value: "oklch(0.205 0.006 285)", description: "Surface layer — cards, table rows, sidebar" },
  /** Hovered row, popover, modal surface. */
  elevated:       { cssVar: "--color-elevated",       value: "oklch(0.265 0.007 285)", description: "Elevated layer — hover, popover, modal" },
  /** 1px default border. */
  border:         { cssVar: "--color-border",         value: "oklch(0.340 0.008 285)", description: "Default 1px border" },
  /** Faint border for subtle dividers. */
  borderSubtle:   { cssVar: "--color-border-subtle",  value: "color-mix(in oklch, var(--color-text-default) 8%, transparent)", description: "Subtle divider, ~8% opacity" },

  textDim:        { cssVar: "--color-text-dim",       value: "oklch(0.660 0.015 285)", description: "Secondary text, timestamps, metadata" },
  textDefault:    { cssVar: "--color-text-default",   value: "oklch(0.830 0.012 285)", description: "Body text" },
  textBright:     { cssVar: "--color-text-bright",    value: "oklch(0.970 0.003 285)", description: "Headings, emphasis" },
  textMuted:      { cssVar: "--color-text-muted",     value: "color-mix(in oklch, var(--color-text-default) 62%, transparent)", description: "Muted text derived from default at 62%" },

  accent:         { cssVar: "--color-accent",         value: "oklch(0.720 0.130 200)", description: "Cyan-teal primary accent" },
  accentHover:    { cssVar: "--color-accent-hover",   value: "oklch(0.780 0.130 200)", description: "Accent hover state" },
  accentPressed:  { cssVar: "--color-accent-pressed", value: "oklch(0.660 0.130 200)", description: "Accent pressed state" },
  accentSubtle:   { cssVar: "--color-accent-subtle",  value: "color-mix(in oklch, var(--color-accent) 14%, transparent)", description: "Accent subtle background (selected rows, ring)" },
  accentBorder:   { cssVar: "--color-accent-border",  value: "color-mix(in oklch, var(--color-accent) 30%, transparent)", description: "Accent border (active state, focused card)" },

  success:        { cssVar: "--color-success",        value: "oklch(0.720 0.180 145)", description: "Healthy/running state" },
  warning:        { cssVar: "--color-warning",        value: "oklch(0.800 0.165 85)",  description: "Cooldown/caution state" },
  danger:         { cssVar: "--color-danger",         value: "oklch(0.680 0.220 25)",  description: "Failed/destructive state" },
  info:           { cssVar: "--color-info",           value: "oklch(0.720 0.130 240)", description: "Informational state" },

  successSubtle:  { cssVar: "--color-success-subtle", value: "color-mix(in oklch, var(--color-success) 14%, transparent)", description: "Success subtle fill" },
  warningSubtle:  { cssVar: "--color-warning-subtle", value: "color-mix(in oklch, var(--color-warning) 14%, transparent)", description: "Warning subtle fill" },
  dangerSubtle:   { cssVar: "--color-danger-subtle",  value: "color-mix(in oklch, var(--color-danger) 14%, transparent)",  description: "Danger subtle fill" },
  infoSubtle:     { cssVar: "--color-info-subtle",    value: "color-mix(in oklch, var(--color-info) 14%, transparent)",    description: "Info subtle fill" },

  focusRing:      { cssVar: "--color-focus-ring",     value: "var(--color-accent)",    description: "Focus-visible ring color" },
  selectionBg:    { cssVar: "--color-selection-bg",   value: "color-mix(in oklch, var(--color-accent) 28%, transparent)", description: "Selected-row / text-selection background" },
} as const satisfies Record<string, Token>;

/* ============================================================================
 * TYPOGRAPHY
 * ========================================================================== */

export const font = {
  sans:        { cssVar: "--font-sans", value: '"InterVariable", Inter, -apple-system, BlinkMacSystemFont, "Segoe UI", system-ui, sans-serif', description: "UI sans family" },
  mono:        { cssVar: "--font-mono", value: '"JetBrains Mono Variable", "JetBrains Mono", ui-monospace, "SF Mono", Menlo, Consolas, monospace', description: "Mono data family" },
} as const satisfies Record<string, Token>;

export const type = {
  size11: { cssVar: "--type-11", value: "0.6875rem", description: "11px — micro meta, keycap badges" },
  size12: { cssVar: "--type-12", value: "0.75rem",   description: "12px — metadata, timestamps" },
  size13: { cssVar: "--type-13", value: "0.8125rem", description: "13px — compact body (Linear 510 weight pattern)" },
  size14: { cssVar: "--type-14", value: "0.875rem",  description: "14px — default body" },
  size16: { cssVar: "--type-16", value: "1rem",      description: "16px — h3" },
  size20: { cssVar: "--type-20", value: "1.25rem",   description: "20px — h2" },
  size24: { cssVar: "--type-24", value: "1.5rem",    description: "24px — h1" },
  size32: { cssVar: "--type-32", value: "2rem",      description: "32px — welcome hero only" },
} as const satisfies Record<string, Token>;

export const weight = {
  regular:   { cssVar: "--weight-regular",   value: "400", description: "Body weight" },
  solid:     { cssVar: "--weight-solid",     value: "510", description: "Linear's body-text trick — slightly heavier than regular" },
  medium:    { cssVar: "--weight-medium",    value: "560", description: "UI labels, buttons" },
  semibold:  { cssVar: "--weight-semibold",  value: "620", description: "Headings" },
} as const satisfies Record<string, Token>;

export const lineHeight = {
  tight:   { cssVar: "--lh-tight",   value: "1.20", description: "Headings" },
  snug:    { cssVar: "--lh-snug",    value: "1.40", description: "UI default" },
  normal:  { cssVar: "--lh-normal",  value: "1.55", description: "Body prose" },
  relaxed: { cssVar: "--lh-relaxed", value: "1.70", description: "Empty-state prose" },
} as const satisfies Record<string, Token>;

/* ============================================================================
 * SPACING — 4px grid
 * ========================================================================== */

export const space = {
  s0:  { cssVar: "--space-0",  value: "0",        description: "Zero" },
  s1:  { cssVar: "--space-1",  value: "0.25rem",  description: "4px" },
  s2:  { cssVar: "--space-2",  value: "0.5rem",   description: "8px" },
  s3:  { cssVar: "--space-3",  value: "0.75rem",  description: "12px" },
  s4:  { cssVar: "--space-4",  value: "1rem",     description: "16px" },
  s6:  { cssVar: "--space-6",  value: "1.5rem",   description: "24px" },
  s8:  { cssVar: "--space-8",  value: "2rem",     description: "32px" },
  s12: { cssVar: "--space-12", value: "3rem",     description: "48px" },
  s16: { cssVar: "--space-16", value: "4rem",     description: "64px" },
} as const satisfies Record<string, Token>;

/* ============================================================================
 * RADII
 * ========================================================================== */

export const radius = {
  r0:     { cssVar: "--radius-0",    value: "0",      description: "Hard edges" },
  r2:     { cssVar: "--radius-2",    value: "2px",    description: "Tight chips, keycap badges" },
  r4:     { cssVar: "--radius-4",    value: "4px",    description: "Compact affordances" },
  r6:     { cssVar: "--radius-6",    value: "6px",    description: "Inputs, buttons" },
  r8:     { cssVar: "--radius-8",    value: "8px",    description: "Cards, popovers" },
  r12:    { cssVar: "--radius-12",   value: "12px",   description: "Panels, drawer rail" },
  r16:    { cssVar: "--radius-16",   value: "16px",   description: "Dialogs" },
  rFull:  { cssVar: "--radius-full", value: "9999px", description: "Pills, status dots" },
} as const satisfies Record<string, Token>;

/* ============================================================================
 * SHADOWS
 * ========================================================================== */

export const shadow = {
  subtle:   { cssVar: "--shadow-subtle",   value: "0 1px 0 0 oklch(0 0 0 / 0.35)",     description: "Hairline bottom shadow for row separators" },
  elevated: { cssVar: "--shadow-elevated", value: "0 2px 8px oklch(0 0 0 / 0.45)",     description: "Card / popover elevation" },
  overlay:  { cssVar: "--shadow-overlay",  value: "0 8px 24px oklch(0.02 0 0 / 0.70)", description: "Modal / drawer overlay (Grafana dark-pop pattern)" },
  focus:    { cssVar: "--shadow-focus",    value: "0 0 0 2px color-mix(in oklch, var(--color-accent) 55%, transparent)", description: "Focus ring (alternative to outline)" },
} as const satisfies Record<string, Token>;

/* ============================================================================
 * MOTION
 * ========================================================================== */

export const ease = {
  default: { cssVar: "--ease-default", value: "cubic-bezier(0.16, 1, 0.3, 1)",     description: "Linear signature ease-out (2026 ops-tool default)" },
  snap:    { cssVar: "--ease-snap",    value: "cubic-bezier(0.3, 0, 0, 1)",        description: "Sharper in/out for state toggles" },
  spring:  { cssVar: "--ease-spring",  value: "cubic-bezier(0.34, 1.56, 0.64, 1)", description: "Subtle overshoot — reserved; use sparingly" },
} as const satisfies Record<string, Token>;

export const duration = {
  instant:  { cssVar: "--dur-instant",  value: "0ms",   description: "Selection highlight, focus ring" },
  quick:    { cssVar: "--dur-quick",    value: "120ms", description: "Hover, popover, tooltip, command palette" },
  moderate: { cssVar: "--dur-moderate", value: "200ms", description: "Dialog, drawer" },
  slow:     { cssVar: "--dur-slow",     value: "320ms", description: "Cross-page view transition" },
  flash:    { cssVar: "--dur-flash",    value: "600ms", description: "SSE row-update pulse (single-shot)" },
} as const satisfies Record<string, Token>;

/* ============================================================================
 * Z-INDEX
 * ========================================================================== */

export const z = {
  base:     { cssVar: "--z-base",     value: "0",   description: "Document baseline" },
  raised:   { cssVar: "--z-raised",   value: "10",  description: "Hover, row elevation" },
  sticky:   { cssVar: "--z-sticky",   value: "100", description: "Sticky sidebar / breadcrumb" },
  dropdown: { cssVar: "--z-dropdown", value: "200", description: "Select popout" },
  drawer:   { cssVar: "--z-drawer",   value: "300", description: "Right drawer" },
  modal:    { cssVar: "--z-modal",    value: "400", description: "Dialog" },
  popover:  { cssVar: "--z-popover",  value: "500", description: "Native [popover]" },
  toast:    { cssVar: "--z-toast",    value: "600", description: "Toast stack" },
  tooltip:  { cssVar: "--z-tooltip",  value: "700", description: "Hover tooltip" },
  palette:  { cssVar: "--z-palette",  value: "800", description: "Command palette (always top)" },
  max:      { cssVar: "--z-max",      value: "999", description: "Emergency escape hatch" },
} as const satisfies Record<string, Token>;

/* ============================================================================
 * LAYOUT
 * ========================================================================== */

export const layout = {
  sidebarWidth:       { cssVar: "--layout-sidebar-width",       value: "240px", description: "Sidebar default width" },
  sidebarCollapsed:   { cssVar: "--layout-sidebar-collapsed",   value: "56px",  description: "Sidebar collapsed icon-rail width" },
  breadcrumbHeight:   { cssVar: "--layout-breadcrumb-height",   value: "40px",  description: "Top breadcrumb bar height" },
  drawerWidth:        { cssVar: "--layout-drawer-width",        value: "400px", description: "Drawer default width" },
  drawerWidthWide:    { cssVar: "--layout-drawer-width-wide",   value: "560px", description: "Drawer wide variant (request inspector)" },
  contentMaxProse:    { cssVar: "--layout-content-max-prose",   value: "72ch",  description: "Prose max width" },
  modalSimple:        { cssVar: "--layout-modal-width-simple",  value: "480px", description: "Simple modal width" },
  modalWizard:        { cssVar: "--layout-modal-width-wizard",  value: "640px", description: "Multi-step wizard modal width" },
  modalFull:          { cssVar: "--layout-modal-width-full",    value: "800px", description: "Full-form modal width" },
  paletteWidth:       { cssVar: "--layout-palette-width",       value: "640px", description: "Command palette width" },
  paletteTopOffset:   { cssVar: "--layout-palette-top-offset",  value: "20vh",  description: "Palette top anchor" },
  bpTablet:           { cssVar: "--bp-tablet",                  value: "768px", description: "Viewport breakpoint — sidebar collapses to icons" },
  bpDesktop:          { cssVar: "--bp-desktop",                 value: "1280px",description: "Primary design target" },
  cqNarrow:           { cssVar: "--cq-narrow",                  value: "600px", description: "Component container-query narrow breakpoint" },
  cqMedium:           { cssVar: "--cq-medium",                  value: "800px", description: "Component container-query medium breakpoint" },
} as const satisfies Record<string, Token>;

/* ============================================================================
 * DENSITY
 * ========================================================================== */

export const density = {
  rowHeight:     { cssVar: "--density-row-height",     value: "36px", description: "Table row height (comfortable=36, compact=28)" },
  cellPaddingX:  { cssVar: "--density-cell-padding-x", value: "12px", description: "Table cell horizontal padding" },
  buttonHeight:  { cssVar: "--density-button-height",  value: "32px", description: "Button height" },
  inputHeight:   { cssVar: "--density-input-height",   value: "32px", description: "Input height" },
  cardPadding:   { cssVar: "--density-card-padding",   value: "16px", description: "Card padding" },
  sidebarItem:   { cssVar: "--density-sidebar-item",   value: "32px", description: "Sidebar nav item height" },
} as const satisfies Record<string, Token>;

/* ============================================================================
 * COMPONENT ALIASES — the tokens components should reach for first
 * ========================================================================== */

export const ui = {
  ringWidth:    { cssVar: "--ring-width",    value: "2px",                description: "Focus ring width" },
  ringOffset:   { cssVar: "--ring-offset",   value: "2px",                description: "Focus ring offset" },
  ringColor:    { cssVar: "--ring-color",    value: "var(--color-accent)", description: "Focus ring color" },
  dotSize:      { cssVar: "--dot-size",      value: "8px",                description: "Status dot canonical size" },
  rowFlashColor:{ cssVar: "--row-flash-color", value: "var(--color-success)", description: "Row SSE flash border color" },
} as const satisfies Record<string, Token>;

/* ============================================================================
 * PUBLIC API
 * ========================================================================== */

export const tokens = {
  color,
  font,
  type,
  weight,
  lineHeight,
  space,
  radius,
  shadow,
  ease,
  duration,
  z,
  layout,
  density,
  ui,
} as const;

export type Tokens = typeof tokens;

/* ----------------------------------------------------------------------------
 * Helpers
 * ----------------------------------------------------------------------------
 */

/** Produces `var(--foo)` from a token. Prefer this over raw template strings. */
export function v<T extends Token>(token: T): string {
  return `var(${token.cssVar})`;
}

/** Produces `var(--foo, fallback)` with explicit fallback. */
export function vf<T extends Token>(token: T, fallback: string): string {
  return `var(${token.cssVar}, ${fallback})`;
}

/** Apply a theme at runtime. Persists to localStorage under `kiroxy:theme`. */
export function setTheme(theme: Theme): void {
  document.documentElement.setAttribute("data-theme", theme);
  try { localStorage.setItem("kiroxy:theme", theme); } catch { /* private-mode ok */ }
}

/** Apply a density mode. Persists to localStorage under `kiroxy:density`. */
export function setDensity(mode: Density): void {
  document.documentElement.setAttribute("data-density", mode);
  try { localStorage.setItem("kiroxy:density", mode); } catch { /* ok */ }
}

/** Resolve a token to its computed CSS value on the document root. */
export function resolve<T extends Token>(token: T): string {
  return getComputedStyle(document.documentElement).getPropertyValue(token.cssVar).trim();
}
