import { defineConfig } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";
import { fileURLToPath } from "node:url";
import { resolve } from "node:path";

// Build output lands in ../../assets/next so Go's go:embed picks it up.
// Paths kept deterministic (no content-hash in filenames) because:
//   - The Go server embeds the directory and serves from memory; browser
//     cache busting is handled via Cache-Control: no-cache, not filenames.
//   - Committed dist diffs are readable when filenames are stable.
const ROOT = fileURLToPath(new URL(".", import.meta.url));
const OUT_DIR = resolve(ROOT, "../../assets/next");

export default defineConfig({
  plugins: [
    svelte({
      compilerOptions: {
        // Svelte 5 runes mode is explicit; we opt in.
        runes: true,
      },
    }),
  ],
  base: "/dashboard-next/assets/",
  build: {
    outDir: OUT_DIR,
    emptyOutDir: true,
    // Use esbuild minifier (bundled with vite) — no terser install needed.
    minify: "esbuild",
    // Inline anything under 2KB; above that, emit as separate file. Our
    // font woff2s are big so they stay separate.
    assetsInlineLimit: 2048,
    cssCodeSplit: false,
    target: "esnext",
    sourcemap: false,
    reportCompressedSize: true,
    rollupOptions: {
      output: {
        entryFileNames: "app.js",
        chunkFileNames: "chunk-[name].js",
        assetFileNames: (info) => {
          const name = info.name ?? "asset";
          if (name.endsWith(".css")) return "app.css";
          return "asset-[name][extname]";
        },
      },
    },
  },
  server: {
    // dev server is not used in production path; kept for local iteration.
    port: 5184,
    strictPort: true,
  },
});
