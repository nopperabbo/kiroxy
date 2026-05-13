// Package server wires the HTTP mux, middleware, and route handlers.
package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"local/kiroxy/internal/kiroclient"
	"local/kiroxy/internal/messages"
	"local/kiroxy/internal/metrics"
	"local/kiroxy/internal/server/mansion"
	"local/kiroxy/internal/server/next"
	"local/kiroxy/internal/server/variants/brutal"
	"local/kiroxy/internal/server/variants/linearpremium"
	"local/kiroxy/internal/server/variants/muji"
	"local/kiroxy/internal/server/variants/neon"
	"local/kiroxy/internal/server/variants/nord"
	"local/kiroxy/internal/server/variants/paper"
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

	// APIKey is the inbound proxy key. M6 enforces it via constant-time compare.
	APIKey string

	// ReadinessChecks are registered as subchecks of /readyz. Missing checks
	// just aren't probed.
	ReadinessChecks map[string]ReadinessChecker

	// Logger is used by the logging middleware for structured request logs.
	// If nil, slog.Default() is used.
	Logger *slog.Logger

	// DashboardStateProvider, when set, powers the /dashboard/api/state endpoint.
	// When nil, /dashboard/api/state returns an empty state.
	DashboardStateProvider DashboardStateProvider

	// DashboardControlProvider, when set, powers the write-paths of the
	// dashboard v2 API (import + remove account). May be nil in tests or
	// when the server is running in kiro-cli SQLite mode where the vault is
	// owned by an external process.
	DashboardControlProvider DashboardControlProvider

	// RequestRing captures the last N completed HTTP requests for the
	// dashboard "recent requests" feed. When nil, request recording is
	// disabled and the feed shows no history. A process-wide ring is
	// typical; tests construct smaller rings as needed.
	RequestRing *RequestRing

	// Metrics is the Prometheus registry exposed at /metrics. When nil,
	// the endpoint returns 503 and no request instrumentation is emitted.
	Metrics *metrics.Registry
}

// Server bundles the process-wide handler tree.
type Server struct {
	opts      Options
	startedAt time.Time
	msgSvc    *messages.Service
	ready     *readiness
	logger    *slog.Logger
}

// New returns a Server with /healthz, /readyz, /v1/messages, /v1/messages/count_tokens
// routes registered.
func New(opts Options) *Server {
	if opts.KiroClient == nil {
		opts.KiroClient = kiroclient.NewHTTPClient()
	}
	var svc *messages.Service
	if opts.Auth != nil {
		svcOpts := []messages.Option{}
		if opts.Metrics != nil {
			svcOpts = append(svcOpts, messages.WithMetrics(opts.Metrics.Sink()))
		}
		svc = messages.New(opts.Auth, opts.KiroClient, svcOpts...)
	}
	ready := newReadiness()
	for name, c := range opts.ReadinessChecks {
		ready.register(name, c)
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Server{
		opts:      opts,
		startedAt: time.Now().UTC(),
		msgSvc:    svc,
		ready:     ready,
		logger:    logger,
	}
}

// Handler returns the full http.Handler tree.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /readyz", s.ready.handle)
	s.registerDashboard(mux)
	next.Register(mux)    // /_variants/dashboard-next (legacy /dashboard-next 302s)
	mansion.Register(mux) // /dashboard-mansion: canonical operator dashboard (post-v1.0.0)
	// Phase V taste-exploration variants, archived under /_variants/<slug>
	// after Mansion was chosen as canonical. Each legacy /dashboard-<slug>
	// URL 302s to its /_variants/<slug> equivalent. Kept fully functional
	// for historical reference and future design-language comparisons.
	brutal.Register(mux)           // /_variants/brutal:        terminal / htop aesthetic
	paper.Register(mux)            // /_variants/paper:         ink on cream / document aesthetic
	nord.Register(mux)             // /_variants/nord:          arctic calm palette
	neon.Register(mux)             // /_variants/neon:          cyberpunk grafana aesthetic
	muji.Register(mux, s.mujiSnap) // /_variants/muji:          zero-JS server-rendered
	linearpremium.Register(mux)    // /_variants/linear-premium: refined SaaS dark + indigo

	if s.msgSvc != nil {
		mux.HandleFunc("POST /v1/messages", s.msgSvc.HandleMessages)
		mux.HandleFunc("POST /v1/messages/count_tokens", s.msgSvc.HandleCountTokens)
	} else {
		mux.HandleFunc("POST /v1/messages", s.handleNoAuth)
		mux.HandleFunc("POST /v1/messages/count_tokens", s.handleNoAuth)
	}

	// OpenAI-compatible surface. /v1/models is always safe to serve (static
	// listing); /v1/chat/completions shares msgSvc state with /v1/messages
	// and returns a 503 via the OpenAI error shape when msgSvc is nil.
	mux.HandleFunc("POST /v1/chat/completions", s.handleChatCompletions)
	mux.HandleFunc("GET /v1/models", s.handleListModels)

	s.registerMetrics(mux)

	authMW := newAuthMiddleware(s.opts.APIKey)
	var rec RequestRecorder
	if s.opts.RequestRing != nil {
		rec = s.opts.RequestRing
	}
	logMW := newLoggingMiddleware(s.logger, rec)
	return logMW.wrap(authMW.wrap(mux))
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
