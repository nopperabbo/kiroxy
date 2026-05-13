// embed.go owns the embedded filesystem for the paper variant. See
// handlers.go for routing and the contentTypeFor whitelist. Layout
// matches brutal/next/mansion conventions:
//
//	internal/server/variants/paper/
//	├── handlers.go
//	├── embed.go (this file)
//	├── handlers_test.go
//	├── README.md
//	└── dist/
//	    ├── index.html
//	    ├── app.css
//	    └── app.js
//
// No build step — the variant is hand-authored vanilla HTML + CSS + JS.

package paper

import "embed"

//go:embed all:dist
var assetsFS embed.FS
