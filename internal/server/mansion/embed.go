// This file owns the embedded filesystem for the Mansion dashboard's built
// assets.
//
// Directory layout:
//
//	internal/server/mansion/
//	├── handlers.go
//	├── embed.go          (this file)
//	└── dist/             (populated by `pnpm build` in client/; committed)
//	    ├── index.html
//	    ├── app.js
//	    └── app.css
//
// Committing the built dist means `go build` works on a fresh clone without
// requiring Node/pnpm. The Makefile target rebuilds when source changes, and
// the tree stays reviewable because Vite emits deterministic filenames (see
// client/vite.config.ts).

package mansion

import "embed"

// assetsFS is the embedded filesystem rooted at ./dist. Every file under
// dist/ — including nested chunks and fonts — is embedded.
//
// The `all:` prefix includes files whose names start with `.` or `_` (Vite
// chunk-files generally don't, but this keeps us robust to future tool
// choices).
//
//go:embed all:dist
var assetsFS embed.FS
