// @ts-check
import { defineConfig } from 'astro/config';

// Static-first. Zero runtime JS by default.
// Deploy target: GitHub Pages by default; override via SITE env for custom
// domain (e.g., SITE=https://kiroxy.dev npm run build).
const site = process.env.SITE || 'https://nopperabbo.github.io';
const base = process.env.BASE || '/kiroxy';

export default defineConfig({
  site,
  base,
  output: 'static',
  compressHTML: true,
  build: {
    inlineStylesheets: 'auto',
  },
  devToolbar: { enabled: false },
  // Vite: keep asset inlining small so favicon/svg get inlined but fonts don't.
  vite: {
    build: {
      assetsInlineLimit: 4096,
    },
  },
});
