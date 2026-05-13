// embed.go — nord variant assets. See handlers.go for routing.

package nord

import "embed"

//go:embed all:dist
var assetsFS embed.FS
