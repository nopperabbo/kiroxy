// Package server wires the HTTP mux, middleware, and route handlers.
package server

import (
	"encoding/json"
	"net/http"
	"time"
)

// Options is how main constructs a Server.
type Options struct {
	// Version is the build version string surfaced via /healthz.
	Version string
}

// Server bundles the process-wide handler tree.
type Server struct {
	opts      Options
	startedAt time.Time
}

// New returns a Server ready to serve /healthz and, in later milestones,
// /v1/messages + /dashboard.
func New(opts Options) *Server {
	return &Server{
		opts:      opts,
		startedAt: time.Now().UTC(),
	}
}

// Handler returns the full http.Handler tree.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Liveness: exists to say "process is up".
	// Readiness will land in M7 with real DB + upstream checks.
	mux.HandleFunc("GET /healthz", s.handleHealthz)

	return mux
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	resp := map[string]any{
		"status":     "ok",
		"version":    s.opts.Version,
		"started_at": s.startedAt.Format(time.RFC3339),
		"uptime_s":   int(time.Since(s.startedAt).Seconds()),
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}
