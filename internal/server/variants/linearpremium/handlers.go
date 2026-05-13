// Package linearpremium serves the "Signature SaaS done right" dashboard
// variant under /dashboard-linear-premium — sixth and final of the
// Phase V taste-exploration variants.
//
// Philosophy: the best-in-class admin genre, executed with 2026-web
// platform primitives — proving that "Linear-like" can be a discipline
// rather than a template. Near-black canvas + indigo-purple accent +
// View Transitions API + @starting-style for enter animations.
//
// See README.md and .sisyphus/plans/variant-linear-premium-manifesto.md.
package linearpremium

import (
	_ "embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

const assetPrefix = "/dashboard-linear-premium/assets/"

//go:embed dist/index.html
var indexHTML []byte

func Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /dashboard-linear-premium", handleIndex)
	mux.HandleFunc("GET "+assetPrefix+"{path...}", handleAsset)
}

func handleIndex(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(indexHTML)
}

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
