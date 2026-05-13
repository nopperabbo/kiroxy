package server

import (
	"context"
	_ "embed"
	"encoding/json"
	"net/http"
	"time"
)

//go:embed dashboard.html
var dashboardHTML []byte

// DashboardState is what /dashboard/api/state returns.
type DashboardState struct {
	Version     string             `json:"version"`
	UptimeS     int64              `json:"uptime_s"`
	Ready       bool               `json:"ready"`
	ReadyDetail string             `json:"ready_detail,omitempty"`
	VaultOK     bool               `json:"vault_ok"`
	VaultPath   string             `json:"vault_path,omitempty"`
	Accounts    []DashboardAccount `json:"accounts"`
}

// DashboardAccount is the per-account row shape.
type DashboardAccount struct {
	ID            string `json:"id"`
	Enabled       bool   `json:"enabled"`
	Requests      int64  `json:"requests"`
	Errors        int64  `json:"errors"`
	CooldownUntil string `json:"cooldown_until,omitempty"`
	LastError     string `json:"last_error,omitempty"`

	// v1.1+ pool health fields. Zero values when the provider has no
	// health tracker (e.g. legacy auth-manager path). Dashboards must
	// treat these as optional.
	SuccessRate    float64 `json:"success_rate,omitempty"`
	Weight         float64 `json:"weight,omitempty"`
	RequestsLast5m int     `json:"requests_last_5m,omitempty"`
	AvgLatencyMs   int64   `json:"avg_latency_ms,omitempty"`
	LastRateLimit  string  `json:"last_rate_limit,omitempty"`
}

// DashboardStateProvider is implemented by whatever owns the pool+vault.
type DashboardStateProvider interface {
	DashboardSnapshot(ctx context.Context) DashboardState
}

// DashboardControlProvider exposes the mutating dashboard actions (import,
// remove, config-emission). Separated from DashboardStateProvider so reads
// and writes can be wired from different owners in tests.
type DashboardControlProvider interface {
	ImportAccounts(ctx context.Context, entries []DashboardImportEntry) ([]DashboardImportResult, error)
	RemoveAccount(ctx context.Context, provider, id string) error
	OpencodeConfig(ctx context.Context, baseURL string) ([]byte, error)
}

// DashboardImportEntry matches the schema of cmd/kiroxy/import_json.go so
// operators can paste the same JSON the CLI accepts.
type DashboardImportEntry struct {
	Provider     string `json:"provider"`
	AuthMethod   string `json:"authMethod"`
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ProfileArn   string `json:"profileArn"`
	ExpiresIn    int64  `json:"expiresIn"`
	AddedAt      string `json:"addedAt"`
}

// DashboardImportResult is one row of the import-endpoint response.
// Status is one of: "added", "updated", "skipped".
type DashboardImportResult struct {
	Index  int    `json:"index"`
	ID     string `json:"id,omitempty"`
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

// ErrAccountNotFound indicates the target account is absent from the vault.
var ErrAccountNotFound = dashboardError("account not found")

type dashboardError string

func (e dashboardError) Error() string { return string(e) }

func (s *Server) registerDashboard(mux *http.ServeMux) {
	mux.HandleFunc("GET /dashboard", s.handleDashboardHTML)
	mux.HandleFunc("GET /dashboard/api/state", s.handleDashboardState)
	// Embedded operator documentation catalog consumed by the Mansion
	// command palette. See docs.go for the curated source set and the
	// one-shot build strategy.
	mux.HandleFunc("GET /dashboard/api/docs/index", s.handleDocsIndex)
	// The legacy hand-authored shell is archived under /_variants/dashboard-legacy
	// for historical reference. It no longer receives new features, and its
	// /dashboard/api/state consumers keep working because the data endpoint above
	// stays at its original URL.
	mux.HandleFunc("GET /_variants/dashboard-legacy", s.handleLegacyDashboard)
}

func (s *Server) handleLegacyDashboard(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(dashboardHTML)
}

func (s *Server) handleDashboardHTML(w http.ResponseWriter, r *http.Request) {
	// Post-v1.0.0: Mansion is the canonical dashboard. The legacy handwritten
	// shell is archived under /_variants/dashboard-legacy; direct visitors of
	// the old /dashboard URL are redirected via 302 so bookmarks keep working.
	// The /dashboard/api/state data endpoint below is NOT redirected — several
	// consumers (scripts, the legacy variant itself) still fetch from it, and
	// Mansion's API shim reads it unchanged.
	http.Redirect(w, r, "/dashboard-mansion", http.StatusFound)
}

func (s *Server) handleDashboardState(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var state DashboardState
	if s.opts.DashboardStateProvider != nil {
		state = s.opts.DashboardStateProvider.DashboardSnapshot(ctx)
	}
	if state.Version == "" {
		state.Version = s.opts.Version
	}
	if state.UptimeS == 0 {
		state.UptimeS = int64(time.Since(s.startedAt).Seconds())
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(state)
}
