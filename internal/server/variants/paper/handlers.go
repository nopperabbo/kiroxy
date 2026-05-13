// Package paper serves the "Ink on Cream" dashboard variant under
// /dashboard-paper — second of six Phase V taste-exploration variants.
// See README.md and .sisyphus/plans/variant-paper-manifesto.md.
//
// Philosophy: this isn't a tool, it's a document — quiet dignity,
// long-read focus, a printed report rendered in HTML. Cream paper
// background, ink text, serif headings, narrow column.
//
// Routing:
//
//	GET /dashboard-paper                    -> HTML shell (index.html)
//	GET /dashboard-paper/assets/{path...}   -> embedded dist assets
//
// Data surface: consumes the existing /dashboard/api/state endpoint.
// No new backend endpoints. Import is documented as a v1.1 backend TODO.
package paper

import (
	_ "embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

const assetPrefix = "/dashboard-paper/assets/"

//go:embed dist/index.html
var indexHTML []byte

// Register mounts the paper variant's routes on mux.
//
// Post-v1.0.0 archive layout: canonical URL is /_variants/paper; the
// legacy /dashboard-paper 302s there. Asset prefix is unchanged because
// the built HTML shell hard-references it.
func Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /_variants/paper", handleIndex)
	mux.HandleFunc("GET /dashboard-paper", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/_variants/paper", http.StatusFound)
	})
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
