// Package nord serves the "Arctic Calm" dashboard variant under
// /dashboard-nord — third of six Phase V taste-exploration variants.
//
// Philosophy: cold, composed, predictable — a palette calibrated for
// eight-hour sessions in a dim room, every color contributing to calm
// rather than urgency. Uses the Nord palette (nordtheme.com, MIT)
// directly; every CSS var is named after its Nord original so the
// provenance is auditable in-file.
//
// See README.md and .sisyphus/plans/variant-nord-manifesto.md for the
// locked philosophy this package commits to.
package nord

import (
	_ "embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

const assetPrefix = "/dashboard-nord/assets/"

//go:embed dist/index.html
var indexHTML []byte

func Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /dashboard-nord", handleIndex)
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
