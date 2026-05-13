// This file owns the embedded filesystem for the brutal variant's
// assets. See handlers.go for the routing and contentTypeFor rules.
//
// Directory layout:
//
//	internal/server/variants/brutal/
//	├── handlers.go
//	├── handlers_test.go
//	├── embed.go         (this file)
//	├── README.md
//	└── dist/
//	    ├── index.html
//	    ├── app.css
//	    └── app.js
//
// There is no build step — dist/ is hand-authored. This is
// intentional: a "terminal aesthetic" dashboard run through Vite would
// betray its own philosophy.

package brutal

import "embed"

// assetsFS is the embedded filesystem rooted at ./dist. Every file
// under dist/ is embedded. The `all:` prefix includes files whose
// names start with `.` or `_` (none today, kept for robustness).
//
//go:embed all:dist
var assetsFS embed.FS
