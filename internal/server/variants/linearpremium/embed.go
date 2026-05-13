// embed.go — linear-premium variant assets. See handlers.go for routing.

package linearpremium

import "embed"

//go:embed all:dist
var assetsFS embed.FS
