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
}

// DashboardStateProvider is implemented by whatever owns the pool+vault.
type DashboardStateProvider interface {
	DashboardSnapshot(ctx context.Context) DashboardState
}

func (s *Server) registerDashboard(mux *http.ServeMux) {
	mux.HandleFunc("GET /dashboard", s.handleDashboardHTML)
	mux.HandleFunc("GET /dashboard/api/state", s.handleDashboardState)
}

func (s *Server) handleDashboardHTML(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(dashboardHTML)
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
