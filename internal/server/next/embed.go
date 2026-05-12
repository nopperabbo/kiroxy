// This file owns the embedded filesystem for Dashboard Next's built assets.
//
// Directory layout:
//
//	internal/server/next/
//	├── handlers.go
//	├── embed.go          (this file)
//	└── dist/             (populated by `pnpm build` in client/; committed)
//	    ├── index.html
//	    ├── app.js
//	    ├── app.css
//	    └── (chunks + fonts)
//
// Committing the built dist directory means `go build` works on a fresh
// clone without requiring Node/pnpm to be installed. The Makefile build
// target rebuilds the frontend when source changes, and the tree stays
// reviewable because Vite emits deterministic filenames (see vite.config.ts).
//
// Committing the built assets directory means `go build` works on a fresh
// clone without requiring Node/pnpm to be installed. The Makefile build
// target rebuilds the frontend when source changes, and the tree stays
// reviewable because Vite emits deterministic filenames (see vite.config.ts).

package next

import "embed"

// assetsFS is the embedded filesystem rooted at ./assets. Every file under
// assets/ — including nested chunks and fonts — is embedded.
//
// The `all:` prefix includes files whose names start with `.` or `_` (Vite
// chunk-files generally don't, but this keeps us robust to future tool
// choices).
//
//go:embed all:dist
var assetsFS embed.FS
