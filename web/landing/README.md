# kiroxy-landing

Landing page for [kiroxy](https://github.com/nopperabbo/kiroxy) — the
self-hosted Kiro proxy. Static site built with Astro, zero runtime JS
by default, deployable to any static host.

## Stack

- **Astro 6** — static output, auto-inlined CSS, zero JS shipped unless
  a component explicitly adds a `<script>` tag.
- **Hand-crafted CSS** — no Tailwind, no shadcn, no UI framework. Design
  tokens in `src/styles/tokens.css` descend directly from the kiroxy
  Mansion dashboard (`internal/server/mansion/client/src/styles/tokens.css`).
- **Self-hosted fonts** — `@fontsource-variable/jetbrains-mono` +
  `@fontsource-variable/inter`, WOFF2, subset by `unicode-range` so only
  the Latin file loads for English readers.
- **No tracking, no analytics, no telemetry.** The page stores nothing
  about the visitor.

## Development

```bash
npm install
npm run dev         # http://localhost:4321
npm run build       # static output in ./dist
npm run preview     # preview the built output locally
```

Requires Node 18+ (tested on Node 25).

## Bundle targets

Both gzipped, Latin-only initial load:

| asset | target | actual |
|---|---:|---:|
| HTML + CSS + SVG | ≤ 50 KB | ~16 KB |
| Latin Inter WOFF2 | — | ~48 KB (on-demand) |
| Latin JetBrains Mono WOFF2 | — | ~40 KB (on-demand) |

Non-Latin font subsets are downloaded only if the page renders those
glyphs. A first-paint English visitor sees CSS + one font file.

## Deployment

The build output in `dist/` is plain static HTML/CSS/SVG/WOFF2. Upload
it to any static host. Three common recipes:

### Vercel

```bash
npm i -g vercel
vercel --prod
```

Or drop the repo into vercel.com and let it auto-detect Astro. No
configuration required.

### Cloudflare Pages

```bash
# via wrangler
npx wrangler pages deploy dist --project-name kiroxy-landing
```

Or connect the git repo at dash.cloudflare.com → Pages and set:
- Build command: `npm run build`
- Output directory: `dist`

### Netlify

```bash
npm i -g netlify-cli
netlify deploy --prod --dir=dist
```

Or point Netlify at the repo with the same build command + output
directory.

### GitHub Pages

Add a GitHub Action:

```yaml
# .github/workflows/deploy.yml
name: Deploy
on:
  push:
    branches: [main]
permissions:
  contents: read
  pages: write
  id-token: write
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with: { node-version: 20, cache: npm }
      - run: npm ci
      - run: npm run build
      - uses: actions/upload-pages-artifact@v3
        with: { path: dist }
  deploy:
    needs: build
    runs-on: ubuntu-latest
    environment: github-pages
    steps:
      - uses: actions/deploy-pages@v4
```

### Any other static host

The `dist/` directory is self-contained. `scp -r dist/ host:/var/www/...`
works. Point the web server at the directory and serve `index.html`
at `/`.

## Project structure

```
src/
├── components/
│   ├── Layout.astro          # <html> shell + head meta + og tags
│   ├── Nav.astro             # Sticky top nav with wordmark
│   ├── Hero.astro            # Wordmark + tagline + CTAs + terminal demo
│   ├── WhatIs.astro          # 30-second explanation
│   ├── FeatureGrid.astro     # 3x3 feature grid
│   ├── Audience.astro        # For / not-for table
│   ├── Installation.astro    # Three install paths + first-request
│   ├── DashboardPreview.astro # LiveRequestStream mock (CSS only)
│   ├── Performance.astro     # Benchmark table
│   ├── Docs.astro            # Doc link grid
│   ├── FAQ.astro             # Native <details> accordion
│   └── Footer.astro          # Footer with column links
├── pages/
│   └── index.astro           # Single page, imports all components
├── styles/
│   ├── tokens.css            # Design tokens (OKLCH palette, type, motion)
│   └── base.css              # Reset + primitives + section rhythm
└── assets/
    └── fonts/                # (unused — fonts load from fontsource)
```

## Design principles

Anchored in `docs/DESIGN_SYSTEM.md` of the kiroxy repo. Short version:

- **OKLCH-first** palette. No HSL. Warm charcoal canvas + aged-brass amber.
- **JetBrains Mono** for display and data, **Inter** for prose.
- **Ledger hairline borders** instead of shadow chrome. No glassmorphism.
- **One accent color**, surgical use. No gradient hero, no pastel stats.
- **Dark-first.** A considered light port is possible via `light-dark()`
  tokens; we ship dark only for v1.
- **Copy register:** operator-tool-with-taste. Honest hedges, exact
  numbers, no "revolutionize your workflow."

## License

MIT. See [LICENSE](./LICENSE) if shipped; otherwise the repository root.

The kiroxy project itself is at
[github.com/nopperabbo/kiroxy](https://github.com/nopperabbo/kiroxy).
