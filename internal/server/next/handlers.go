// Package next serves the experimental "Dashboard Next" frontend under
// /dashboard-next. It is a parallel alternative to the Phase H dashboard v2
// served at /dashboard — same proxy data, different stack (Svelte 5 + TS +
// Vite-bundled assets rather than vanilla JS + hand-authored CSS).
//
// Routing:
//
//	GET /dashboard-next                 -> HTML shell (index.html)
//	GET /dashboard-next/assets/{path...} -> Vite build output (JS/CSS/fonts)
//
// All data-API endpoints (/dashboard/api/*) are consumed from Phase H's
// existing handlers. No duplicate endpoints added.
//
// The Register function takes a *http.ServeMux and attaches the two routes
// above; it's called once from server.go next to registerDashboard.
package next

import (
	_ "embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// assetPrefix is the URL prefix under which the bundled assets are served.
// Kept as a const so it can't drift from vite.config.ts.
const assetPrefix = "/dashboard-next/assets/"

// indexHTML is the Vite-compiled HTML shell for the Dashboard Next root.
// vite writes this into ../assets/next/index.html at build time. Even when
// the dist is empty (fresh clone before pnpm build), this embed still
// compiles because go:embed of a missing file would fail — embed.go
// verifies the minimum fileset exists.
//
// See embed.go for the directory embed used by asset serving.
//
//go:embed dist/index.html
var indexHTML []byte

// Register mounts the Dashboard Next routes on mux. Caller is responsible
// for putting the result behind the existing auth + logging middleware
// (server.New's Handler does this for everything on the same mux).
//
// Post-v1.0.0 archive layout: canonical URL is /_variants/dashboard-next;
// the legacy /dashboard-next 302s there.
func Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /_variants/dashboard-next", handleIndex)
	mux.HandleFunc("GET /dashboard-next", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/_variants/dashboard-next", http.StatusFound)
	})
	mux.HandleFunc("GET "+assetPrefix+"{path...}", handleAsset)
}

// handleIndex serves the SPA shell. Cache-Control:no-cache is important:
// the shell references asset filenames (app.js, app.css) that are stable,
// but the bundle content is cache-busted via Cache-Control on the assets
// themselves. Serving the shell fresh every time keeps asset references
// current when the operator rebuilds the UI without restarting the server.
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
//   - fs.Sub("assets"): anchors the read-scope to the embedded subtree so
//     even a cleverly crafted path can't escape to another package's files.
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
	// Vite does not hash our filenames (see vite.config.ts): rely on
	// Cache-Control:no-cache + ETag on large assets; short-circuit for
	// operator-dev cycles where changes need to be visible immediately.
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(data)
}

// contentTypeFor returns a sensible Content-Type for the file extension.
// Explicit switch over a MIME library: we control exactly what's served,
// so there's no need for net/http.DetectContentType and its sniff behavior.
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
