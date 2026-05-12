import { defineConfig } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";
import { fileURLToPath } from "node:url";
import { resolve } from "node:path";

// Build output lands in ../dist so Go's go:embed (see ../embed.go) picks it up.
// Filenames stay deterministic (no content hash): the Go server serves from
// memory with Cache-Control:no-cache so hash-busted URLs aren't needed, and
// committed dist diffs stay readable.
const ROOT = fileURLToPath(new URL(".", import.meta.url));
const OUT_DIR = resolve(ROOT, "../dist");

export default defineConfig({
  plugins: [
    svelte({
      compilerOptions: {
        runes: true,
      },
    }),
  ],
  base: "/dashboard-mansion/assets/",
  build: {
    outDir: OUT_DIR,
    emptyOutDir: true,
    minify: "esbuild",
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
    port: 5187,
    strictPort: true,
  },
});
