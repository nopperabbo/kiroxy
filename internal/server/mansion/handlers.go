// Package mansion serves the "Mansion" dashboard under /dashboard-mansion.
//
// This is the signature operator-tool UI for kiroxy: parallel to Phase H's
// hand-authored /dashboard and the experimental /dashboard-next, but built
// with a deliberate visual direction ("Operator Desk — Warm Dense") rather
// than minimum-viable scaffolding. Same proxy data surface, different stack
// + different aesthetic commitment.
//
// Routing:
//
//	GET /dashboard-mansion                 -> HTML shell (index.html)
//	GET /dashboard-mansion/assets/{path...} -> Vite build output (JS/CSS/fonts)
//
// All data-API endpoints (/dashboard/api/*) are consumed from Phase H's
// existing handlers. No duplicate endpoints added — if we need something
// the backend doesn't expose yet (e.g. per-account history), the frontend
// derives it client-side from polling snapshots and the gap is noted in
// docs/DASHBOARD_MANSION.md as a v1.3 backend TODO.
//
// Register is called once from server.go alongside registerDashboard +
// next.Register.
package mansion

import (
	_ "embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// assetPrefix is the URL prefix under which bundled assets are served. Kept
// as a const so it can't drift from client/vite.config.ts.
const assetPrefix = "/dashboard-mansion/assets/"

// indexHTML is the Vite-compiled HTML shell. vite writes this into
// ./dist/index.html at build time. The embed compiles even when dist is
// a placeholder (seeded by initial scaffold commit) — see embed.go.
//
//go:embed dist/index.html
var indexHTML []byte

// Register mounts the Mansion routes on mux. Caller is responsible for
// putting the result behind the existing auth + logging middleware
// (server.New's Handler does this for everything on the same mux).
func Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /dashboard-mansion", handleIndex)
	mux.HandleFunc("GET "+assetPrefix+"{path...}", handleAsset)
}

// handleIndex serves the SPA shell. Cache-Control:no-cache keeps asset
// references current when the operator rebuilds the frontend without
// restarting the server; the assets themselves have separate cache rules
// below.
func handleIndex(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(indexHTML)
}

// handleAsset serves bundled assets from the embedded dist.
//
// Security notes:
//   - path.Clean + rejection of "..": guards against directory traversal.
//   - fs.Sub("dist"): anchors read-scope to the embedded subtree so a
//     crafted path can't escape to another package's files.
//   - Explicit Content-Type whitelist: protects against browsers sniffing
//     a .svg as text/html or similar.
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
	ct := contentTypeFor(name)
	w.Header().Set("Content-Type", ct)
	// Vite doesn't hash our filenames (see client/vite.config.ts): rely on
	// Cache-Control:no-cache + short-circuit for operator-dev cycles where
	// changes need to be visible immediately.
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(data)
}

// contentTypeFor returns a sensible Content-Type for the file extension.
// Explicit switch over http.DetectContentType: we control exactly what's
// served, so there's no need for sniff behavior.
func contentTypeFor(name string) string {
	switch {
	case strings.HasSuffix(name, ".js"), strings.HasSuffix(name, ".mjs"):
		return "application/javascript; charset=utf-8"
	case strings.HasSuffix(name, ".css"):
		return "text/css; charset=utf-8"
	case strings.HasSuffix(name, ".svg"):
		return "image/svg+xml"
	case strings.HasSuffix(name, ".html"):
		return "text/html; charset=utf-8"
	case strings.HasSuffix(name, ".json"):
		return "application/json; charset=utf-8"
	case strings.HasSuffix(name, ".woff2"):
		return "font/woff2"
	case strings.HasSuffix(name, ".woff"):
		return "font/woff"
	case strings.HasSuffix(name, ".map"):
		return "application/json; charset=utf-8"
	case strings.HasSuffix(name, ".ico"):
		return "image/x-icon"
	case strings.HasSuffix(name, ".png"):
		return "image/png"
	default:
		return "application/octet-stream"
	}
}
