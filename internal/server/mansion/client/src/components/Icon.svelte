<!--
  Icon — inline SVG sprite. Hand-drawn from Lucide geometry but redrawn
  at 16px optical grid with warmer stroke terminators for the "operator
  desk" feel. Keeping it a switch so dead-code-elimination drops unused
  paths at build time.
-->
<script lang="ts">
  interface Props {
    name: string;
    size?: number;
    strokeWidth?: number;
  }
  let { name, size = 16, strokeWidth = 1.5 }: Props = $props();
</script>

<svg
  xmlns="http://www.w3.org/2000/svg"
  width={size}
  height={size}
  viewBox="0 0 16 16"
  fill="none"
  stroke="currentColor"
  stroke-width={strokeWidth}
  stroke-linecap="round"
  stroke-linejoin="round"
  aria-hidden="true"
  focusable="false"
>
  {#if name === "x"}
    <path d="M3 3 L13 13 M13 3 L3 13" />
  {:else if name === "check"}
    <path d="M3 8.5 L6.5 12 L13 4.5" />
  {:else if name === "alert"}
    <path d="M8 2 L14 13 L2 13 Z" />
    <path d="M8 6 L8 9" />
    <circle cx="8" cy="11.25" r="0.35" fill="currentColor" stroke="none" />
  {:else if name === "search"}
    <circle cx="7" cy="7" r="4" />
    <path d="M10.25 10.25 L13.5 13.5" />
  {:else if name === "bolt"}
    <path d="M9 1 L3 9 L7 9 L7 15 L13 7 L9 7 Z" />
  {:else if name === "download"}
    <path d="M8 1 L8 10" />
    <path d="M4 6.5 L8 10.5 L12 6.5" />
    <path d="M2.5 13.5 L13.5 13.5" />
  {:else if name === "plus"}
    <path d="M8 3 L8 13 M3 8 L13 8" />
  {:else if name === "sun"}
    <circle cx="8" cy="8" r="3" />
    <path
      d="M8 1 L8 3 M8 13 L8 15 M1 8 L3 8 M13 8 L15 8 M3.05 3.05 L4.5 4.5 M11.5 11.5 L12.95 12.95 M3.05 12.95 L4.5 11.5 M11.5 4.5 L12.95 3.05"
    />
  {:else if name === "moon"}
    <path d="M13 10.5 A5.5 5.5 0 1 1 5.5 3 A4 4 0 0 0 13 10.5 Z" />
  {:else if name === "monitor"}
    <rect x="2" y="3" width="12" height="9" rx="1" />
    <path d="M5.5 14.5 L10.5 14.5 M8 12 L8 14.5" />
  {:else if name === "arrow-right"}
    <path d="M3 8 L13 8 M9.5 4.5 L13 8 L9.5 11.5" />
  {:else if name === "arrow-down"}
    <path d="M8 3 L8 13 M4.5 9.5 L8 13 L11.5 9.5" />
  {:else if name === "arrow-up"}
    <path d="M8 13 L8 3 M4.5 6.5 L8 3 L11.5 6.5" />
  {:else if name === "layers"}
    <path d="M8 1.5 L14.5 5 L8 8.5 L1.5 5 Z" />
    <path d="M1.5 8 L8 11.5 L14.5 8" />
    <path d="M1.5 11 L8 14.5 L14.5 11" />
  {:else if name === "dot"}
    <circle cx="8" cy="8" r="2" fill="currentColor" stroke="none" />
  {:else if name === "refresh"}
    <path d="M13 5 A5 5 0 1 0 13.5 11.5" />
    <path d="M13 2 L13 5 L10 5" />
  {:else if name === "chevron-right"}
    <path d="M6 3.5 L10.5 8 L6 12.5" />
  {:else if name === "eye"}
    <path d="M1 8 C3 4 5 3 8 3 S13 4 15 8 C13 12 11 13 8 13 S3 12 1 8 Z" />
    <circle cx="8" cy="8" r="2" />
  {:else if name === "copy"}
    <rect x="5" y="5" width="8" height="8" rx="1" />
    <path d="M11 5 L11 3 A1 1 0 0 0 10 2 L3 2 A1 1 0 0 0 2 3 L2 10 A1 1 0 0 0 3 11 L5 11" />
  {:else if name === "command"}
    <path
      d="M5 4 A2 2 0 1 1 7 6 L7 10 A2 2 0 1 1 5 12 L9 12 A2 2 0 1 1 11 10 L11 6 A2 2 0 1 1 9 4 Z"
    />
  {:else if name === "link"}
    <path d="M7 5 L5 5 A3 3 0 0 0 5 11 L7 11" />
    <path d="M9 5 L11 5 A3 3 0 0 1 11 11 L9 11" />
    <path d="M6 8 L10 8" />
  {:else if name === "play"}
    <path d="M4 3 L12 8 L4 13 Z" fill="currentColor" />
  {:else if name === "pause"}
    <rect x="4" y="3" width="3" height="10" fill="currentColor" />
    <rect x="9" y="3" width="3" height="10" fill="currentColor" />
  {:else if name === "trash"}
    <path d="M3 5 L13 5" />
    <path d="M5 5 L5 3 A1 1 0 0 1 6 2 L10 2 A1 1 0 0 1 11 3 L11 5" />
    <path d="M5 5 L5 13 A1 1 0 0 0 6 14 L10 14 A1 1 0 0 0 11 13 L11 5" />
  {:else if name === "settings"}
    <circle cx="8" cy="8" r="2" />
    <path d="M8 1 L8 3 M8 13 L8 15 M1 8 L3 8 M13 8 L15 8 M3 3 L4.5 4.5 M11.5 11.5 L13 13 M3 13 L4.5 11.5 M11.5 4.5 L13 3" />
  {:else if name === "tool"}
    <path d="M11 3 A3 3 0 1 1 9 9 L3 15 L1 13 L7 7 A3 3 0 0 1 11 3 Z" />
  {:else if name === "terminal"}
    <path d="M2 4 L6 8 L2 12" />
    <path d="M7 12 L12 12" />
  {:else if name === "server"}
    <rect x="2" y="3" width="12" height="4" rx="1" />
    <rect x="2" y="9" width="12" height="4" rx="1" />
    <circle cx="5" cy="5" r="0.6" fill="currentColor" />
    <circle cx="5" cy="11" r="0.6" fill="currentColor" />
  {:else if name === "key"}
    <circle cx="5" cy="8" r="3" />
    <path d="M8 8 L14 8 L14 10 M12 8 L12 10" />
  {:else if name === "shield"}
    <path d="M8 2 L13 4 L13 8 A6 6 0 0 1 8 14 A6 6 0 0 1 3 8 L3 4 Z" />
  {/if}
</svg>
