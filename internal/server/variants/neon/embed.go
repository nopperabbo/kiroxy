// embed.go — neon variant assets. See handlers.go for routing.

package neon

import "embed"

//go:embed all:dist
var assetsFS embed.FS
