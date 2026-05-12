<!--
  Icon — inline SVG glyphs. 12 hand-picked, no external icon bundle.
  Each lives as a case in a switch. Keeping them inline avoids runtime
  fetch + lets the compiler tree-shake unused paths.
-->
<script lang="ts">
  interface Props {
    name:
      | "plus"
      | "reload"
      | "trash"
      | "x"
      | "search"
      | "theme"
      | "cog"
      | "arrow"
      | "check"
      | "alert"
      | "clock"
      | "dot";
    size?: number;
    "aria-label"?: string;
  }
  let { name, size = 14, "aria-label": ariaLabel = "" }: Props = $props();
</script>

<svg
  width={size}
  height={size}
  viewBox="0 0 16 16"
  fill="none"
  stroke="currentColor"
  stroke-width="1.5"
  stroke-linecap="round"
  stroke-linejoin="round"
  aria-label={ariaLabel || undefined}
  role={ariaLabel ? "img" : undefined}
  aria-hidden={ariaLabel ? undefined : "true"}
>
  {#if name === "plus"}
    <path d="M8 3v10M3 8h10" />
  {:else if name === "reload"}
    <path d="M13 3v4h-4M3 13v-4h4" />
    <path d="M4 8a4 4 0 017-2.7l2-2M12 8a4 4 0 01-7 2.7l-2 2" />
  {:else if name === "trash"}
    <path
      d="M3 4h10M6 4V3a1 1 0 011-1h2a1 1 0 011 1v1M5 4l1 9a1 1 0 001 1h2a1 1 0 001-1l1-9"
    />
  {:else if name === "x"}
    <path d="M4 4l8 8M12 4l-8 8" />
  {:else if name === "search"}
    <circle cx="7" cy="7" r="4.5" />
    <path d="M10.5 10.5L14 14" />
  {:else if name === "theme"}
    <circle cx="8" cy="8" r="5.5" />
    <path d="M8 2.5v11" />
  {:else if name === "cog"}
    <circle cx="8" cy="8" r="2.2" />
    <path
      d="M8 2v2M8 12v2M2 8h2M12 8h2M3.8 3.8l1.4 1.4M10.8 10.8l1.4 1.4M3.8 12.2l1.4-1.4M10.8 5.2l1.4-1.4"
    />
  {:else if name === "arrow"}
    <path d="M5 3l5 5-5 5" />
  {:else if name === "check"}
    <path d="M3 8.5L7 12l6-8" />
  {:else if name === "alert"}
    <path d="M8 2l6 11H2L8 2zM8 7v3M8 11.5v.5" />
  {:else if name === "clock"}
    <circle cx="8" cy="8" r="5.5" />
    <path d="M8 5v3l2 1.5" />
  {:else if name === "dot"}
    <circle cx="8" cy="8" r="3" fill="currentColor" />
  {/if}
</svg>
