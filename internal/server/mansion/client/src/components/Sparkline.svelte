<!--
  Sparkline — hand-rolled SVG line+area. No chart library.

  Values are an array of numbers. We fit them to the given width/height
  with a 2px inset, draw a smooth (catmull-like) curve, then fill the
  area beneath with a soft accent wash. A single dot marks the latest
  value for readability at small sizes.

  Auto-scaling: y range is always 0..max(1, max(values)) so an empty
  sparkline shows a calm zero line instead of noise at the top.
-->
<script lang="ts">
  interface Props {
    values: number[];
    width?: number;
    height?: number;
    /** CSS variable family: "accent" | "success" | "warn" | "danger" */
    accent?: "accent" | "success" | "warn" | "danger";
    ariaLabel?: string;
  }
  let { values, width = 120, height = 32, accent = "accent", ariaLabel }: Props = $props();

  let pathD = $derived(linePath(values, width, height));
  let areaD = $derived(areaPath(values, width, height));
  let dot = $derived(latestDot(values, width, height));

  function range(v: number[]): { min: number; max: number } {
    let max = 1;
    for (const x of v) if (x > max) max = x;
    return { min: 0, max };
  }

  function linePath(v: number[], w: number, h: number): string {
    if (v.length === 0) return "";
    const { min, max } = range(v);
    const span = max - min || 1;
    const step = v.length > 1 ? (w - 4) / (v.length - 1) : 0;
    const pts = v.map((y, i) => [2 + i * step, 2 + (h - 4) * (1 - (y - min) / span)] as [number, number]);
    return smoothPath(pts);
  }

  function areaPath(v: number[], w: number, h: number): string {
    if (v.length === 0) return "";
    const line = linePath(v, w, h);
    return `${line} L ${w - 2} ${h - 2} L 2 ${h - 2} Z`;
  }

  function latestDot(v: number[], w: number, h: number): { x: number; y: number } | null {
    if (v.length === 0) return null;
    const { min, max } = range(v);
    const span = max - min || 1;
    const last = v[v.length - 1];
    const x = w - 2;
    const y = 2 + (h - 4) * (1 - (last - min) / span);
    return { x, y };
  }

  // Catmull-Rom-to-Bezier smoothing. Enough curvature to feel hand-drawn
  // without overshooting at local maxima.
  function smoothPath(pts: Array<[number, number]>): string {
    if (pts.length === 0) return "";
    if (pts.length === 1) return `M ${pts[0][0]} ${pts[0][1]}`;
    let d = `M ${pts[0][0]} ${pts[0][1]}`;
    for (let i = 0; i < pts.length - 1; i++) {
      const p0 = pts[i - 1] ?? pts[i];
      const p1 = pts[i];
      const p2 = pts[i + 1];
      const p3 = pts[i + 2] ?? p2;
      const t = 0.2;
      const c1x = p1[0] + (p2[0] - p0[0]) * t;
      const c1y = p1[1] + (p2[1] - p0[1]) * t;
      const c2x = p2[0] - (p3[0] - p1[0]) * t;
      const c2y = p2[1] - (p3[1] - p1[1]) * t;
      d += ` C ${c1x.toFixed(2)} ${c1y.toFixed(2)}, ${c2x.toFixed(2)} ${c2y.toFixed(2)}, ${p2[0].toFixed(2)} ${p2[1].toFixed(2)}`;
    }
    return d;
  }
</script>

<svg
  class="spark spark--{accent}"
  {width}
  {height}
  viewBox="0 0 {width} {height}"
  preserveAspectRatio="none"
  role={ariaLabel ? "img" : "presentation"}
  aria-label={ariaLabel}
>
  {#if areaD}
    <path d={areaD} class="spark__area" />
  {/if}
  {#if pathD}
    <path d={pathD} class="spark__line" />
  {/if}
  {#if dot}
    <circle cx={dot.x} cy={dot.y} r="2" class="spark__dot" />
  {/if}
</svg>

<style>
  .spark {
    display: block;
  }
  .spark__line {
    fill: none;
    stroke: currentColor;
    stroke-width: 1.5;
    stroke-linecap: round;
    stroke-linejoin: round;
  }
  .spark__area {
    fill: color-mix(in oklch, currentColor, transparent 82%);
    stroke: none;
  }
  .spark__dot {
    fill: currentColor;
    stroke: none;
  }
  .spark--accent {
    color: var(--c-accent);
  }
  .spark--success {
    color: var(--c-success);
  }
  .spark--warn {
    color: var(--c-warn);
  }
  .spark--danger {
    color: var(--c-danger);
  }
</style>
