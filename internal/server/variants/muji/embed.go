// embed.go — muji variant assets. The HTML shell is a template
// (index.html.tmpl) compiled once at package init (see handlers.go
// var block). Other assets under dist/ are served verbatim.

package muji

import "embed"

//go:embed all:dist
var assetsFS embed.FS
