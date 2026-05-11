// Package server wires the HTTP mux, middleware, and route handlers.
package server

import (
	"encoding/json"
	"net/http"
	"time"

	"local/kiroxy/internal/kiroclient"
	"local/kiroxy/internal/messages"
)

// Options is how main constructs a Server.
type Options struct {
	Version string

	// Auth loads Kiro upstream credentials. In M2 this is kirocc's
	// auth.AuthManager (kiro-cli SQLite reader); M4/M5 swap in our own
	// pool+tokenvault. When nil, /v1/messages returns 503.
	Auth messages.TokenGetter

	// KiroClient is the upstream HTTP client. Defaults to kiroclient.NewHTTPClient().
	KiroClient kiroclient.Client

	// APIKey is the inbound proxy key. M2 does not enforce it; M6 will.
	APIKey string
}

// Server bundles the process-wide handler tree.
type Server struct {
	opts      Options
	startedAt time.Time
	msgSvc    *messages.Service
}

// New returns a Server with /healthz, /v1/messages, /v1/messages/count_tokens
// routes registered.
func New(opts Options) *Server {
	if opts.KiroClient == nil {
		opts.KiroClient = kiroclient.NewHTTPClient()
	}
	var svc *messages.Service
	if opts.Auth != nil {
		svc = messages.New(opts.Auth, opts.KiroClient)
	}
	return &Server{
		opts:      opts,
		startedAt: time.Now().UTC(),
		msgSvc:    svc,
	}
}

// Handler returns the full http.Handler tree.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", s.handleHealthz)

	if s.msgSvc != nil {
		mux.HandleFunc("POST /v1/messages", s.msgSvc.HandleMessages)
		mux.HandleFunc("POST /v1/messages/count_tokens", s.msgSvc.HandleCountTokens)
	} else {
		mux.HandleFunc("POST /v1/messages", s.handleNoAuth)
		mux.HandleFunc("POST /v1/messages/count_tokens", s.handleNoAuth)
	}

	authMW := newAuthMiddleware(s.opts.APIKey)
	return authMW.wrap(mux)
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

func (s *Server) handleNoAuth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusServiceUnavailable)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"type":    "authentication_error",
		"message": "no Kiro account configured; set KIROXY_KIRO_DB_PATH (points to your kiro-cli data.sqlite3) or run 'kiroxy add-account' (M9)",
	})
}
