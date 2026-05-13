// Package muji serves the "Japanese minimalism" dashboard variant under
// /dashboard-muji — fifth of six Phase V taste-exploration variants.
//
// Philosophy: nothing unnecessary. Whitespace is the content. The
// dashboard achieves authority through absence rather than decoration.
//
// This variant is the ONLY one that is fully server-rendered —
// zero client-side JavaScript. The browser reloads the page every 5s
// via `<meta http-equiv="refresh">`. Import is a plain HTML form that
// posts to the backend import endpoint (404 today, v1.1 backend TODO).
//
// See README.md and .sisyphus/plans/variant-muji-manifesto.md.
package muji

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"
)

const assetPrefix = "/dashboard-muji/assets/"

// Snapshot is the subset of DashboardState this variant renders. A
// decoupled struct keeps the variant from importing the parent server
// package (which would create a cycle).
type Snapshot struct {
	Version   string
	UptimeS   int64
	Ready     bool
	ReadyText string
	VaultOK   bool
	Accounts  []Account
}

// Account mirrors just what muji displays. The server.go registration
// maps server.DashboardAccount to this struct.
type Account struct {
	ID            string
	Enabled       bool
	Requests      int64
	Errors        int64
	CooldownUntil string
	LastError     string
}

// SnapFn is supplied by server.go and returns the current pool snapshot.
type SnapFn func(ctx context.Context) Snapshot

//go:embed dist/index.html.tmpl
var indexTmplSrc string

var indexTmpl = template.Must(template.New("muji").Funcs(template.FuncMap{
	"uptime": formatUptime,
	"state":  accountState,
}).Parse(indexTmplSrc))

// Register mounts the muji variant. snap may be nil — if so, every
// render shows an empty pool with the CLI import instructions. The
// typical wiring is `muji.Register(mux, makeSnapFn(server))`.
func Register(mux *http.ServeMux, snap SnapFn) {
	mux.HandleFunc("GET /dashboard-muji", func(w http.ResponseWriter, r *http.Request) {
		handleIndex(w, r, snap)
	})
	mux.HandleFunc("GET "+assetPrefix+"{path...}", handleAsset)
}

// handleIndex server-renders the dashboard. We build into a
// bytes.Buffer before writing so template errors produce a 500 rather
// than a half-rendered page.
func handleIndex(w http.ResponseWriter, r *http.Request, snap SnapFn) {
	var s Snapshot
	if snap != nil {
		s = snap(r.Context())
	}

	// Stamp rendered server-side so the page works with JS disabled.
	now := time.Now().UTC().Format("15:04:05 UTC · 2006-01-02")

	totReq, totErr := int64(0), int64(0)
	for _, a := range s.Accounts {
		totReq += a.Requests
		totErr += a.Errors
	}

	data := struct {
		Snap     Snapshot
		TotReq   int64
		TotErr   int64
		Stamp    string
		Variants []linkRow
	}{
		Snap:   s,
		TotReq: totReq,
		TotErr: totErr,
		Stamp:  now,
		Variants: []linkRow{
			{Path: "/dashboard", Label: "classic"},
			{Path: "/dashboard-next", Label: "next"},
			{Path: "/dashboard-mansion", Label: "mansion"},
			{Path: "/dashboard-brutal", Label: "brutal"},
			{Path: "/dashboard-paper", Label: "paper"},
			{Path: "/dashboard-nord", Label: "nord"},
			{Path: "/dashboard-neon", Label: "neon"},
		},
	}

	var buf bytes.Buffer
	if err := indexTmpl.Execute(&buf, data); err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(buf.Bytes())
}

type linkRow struct {
	Path, Label string
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
	case strings.HasSuffix(name, ".css"):
		return "text/css; charset=utf-8"
	case strings.HasSuffix(name, ".html"):
		return "text/html; charset=utf-8"
	case strings.HasSuffix(name, ".svg"):
		return "image/svg+xml"
	case strings.HasSuffix(name, ".ico"):
		return "image/x-icon"
	default:
		return "application/octet-stream"
	}
}

// formatUptime is exposed to the template as {{ uptime .Snap.UptimeS }}.
func formatUptime(s int64) string {
	if s < 0 {
		return "—"
	}
	d := s / 86400
	h := (s % 86400) / 3600
	m := (s % 3600) / 60
	switch {
	case d > 0:
		return fmt.Sprintf("%dd %dh", d, h)
	case h > 0:
		return fmt.Sprintf("%dh %dm", h, m)
	default:
		return fmt.Sprintf("%dm", m)
	}
}

// accountState returns a single-word human-readable status for an
// account. Exposed as {{ state . }} on a loop over accounts.
func accountState(a Account) string {
	switch {
	case a.CooldownUntil != "":
		return "cooling"
	case a.LastError != "":
		return "error"
	case !a.Enabled:
		return "paused"
	default:
		return "active"
	}
}
