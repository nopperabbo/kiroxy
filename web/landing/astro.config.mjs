// @ts-check
import { defineConfig } from 'astro/config';

// Static-first. Zero runtime JS by default.
// Deploy target: any static host (Vercel, Cloudflare Pages, Netlify, GH Pages).
export default defineConfig({
  site: 'https://kiroxy.dev',
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
