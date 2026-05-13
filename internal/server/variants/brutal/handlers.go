// Package brutal serves the "Terminal" dashboard variant under
// /dashboard-brutal — one of six taste-exploration variants committed
// in Phase V. See README.md for the philosophy statement.
//
// Philosophy: information is the product, chrome is the enemy. Pure
// black canvas, monospace everywhere, phosphor-green signal color, zero
// rounded corners, ASCII box-drawing for tables. Reference: htop,
// plan9 acme, AWS cli --output table.
//
// Routing:
//
//	GET /dashboard-brutal                    -> HTML shell (index.html)
//	GET /dashboard-brutal/assets/{path...}   -> embedded dist assets
//
// Data surface: consumes the existing /dashboard/api/state endpoint.
// No new backend endpoints are introduced by this variant. Where the
// UI would benefit from data the backend doesn't yet expose (e.g. live
// request stream SSE, per-account history), the variant degrades
// gracefully and flags the gap in README.md as a v1.1 backend TODO.
//
// Register is called once from server.go alongside the other variants.
package brutal

import (
	_ "embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// assetPrefix is the URL prefix under which bundled assets are served.
// Kept as a const so it can't drift from the <link>/<script> paths in
// dist/index.html.
const assetPrefix = "/dashboard-brutal/assets/"

// indexHTML is the embedded HTML shell. The brutal variant ships
// hand-authored HTML (no build step) — see dist/index.html for the
// source of truth.
//
//go:embed dist/index.html
var indexHTML []byte

// Register mounts the brutal variant's routes on mux. Caller is
// responsible for putting the result behind the existing auth +
// logging middleware (server.New's Handler does this for everything
// on the same mux).
func Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /dashboard-brutal", handleIndex)
	mux.HandleFunc("GET "+assetPrefix+"{path...}", handleAsset)
}

// handleIndex serves the HTML shell. Cache-Control:no-cache matches
// the other variants' convention so the operator can iterate on UI
// without restarting the server.
func handleIndex(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(indexHTML)
}

// handleAsset serves bundled assets from the embedded dist.
//
// Security notes (identical to next/mansion — intentional parity so
// all variants follow the same hardening):
//   - path.Clean + rejection of "..": guards against directory
//     traversal.
//   - fs.Sub("dist"): anchors read-scope to the embedded subtree.
//   - Explicit Content-Type whitelist: protects against browsers
//     sniffing a .svg as text/html.
func handleAsset(w http.ResponseWriter, r *http.Request) {
	raw := r.PathValue("path")
	clean := path.Clean("/" + raw)
	if clean == "/" || strings.Contains(clean, "..") {
		http.NotFound(w, r)
		return
	}
	name := strings.TrimPrefix(clean, "/")
	sub, err := fs.Sub(assetsFS, "dist")
	if err != nil {
		http.Error(w, "asset registry unavailable", http.StatusInternalServerError)
		return
	}
	data, err := fs.ReadFile(sub, name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", contentTypeFor(name))
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(data)
}

// contentTypeFor mirrors next.contentTypeFor. Duplicated intentionally
// per-variant so each package is self-contained and the helper can be
// customised per variant without coupling.
func contentTypeFor(name string) string {
	switch {
	case strings.HasSuffix(name, ".js"), strings.HasSuffix(name, ".mjs"):
		return "application/javascript; charset=utf-8"
	case strings.HasSuffix(name, ".css"):
		return "text/css; charset=utf-8"
	case strings.HasSuffix(name, ".html"):
		return "text/html; charset=utf-8"
	case strings.HasSuffix(name, ".svg"):
		return "image/svg+xml"
	case strings.HasSuffix(name, ".json"):
		return "application/json; charset=utf-8"
	case strings.HasSuffix(name, ".woff2"):
		return "font/woff2"
	case strings.HasSuffix(name, ".woff"):
		return "font/woff"
	case strings.HasSuffix(name, ".ico"):
		return "image/x-icon"
	default:
		return "application/octet-stream"
	}
}
