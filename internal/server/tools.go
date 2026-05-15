package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/nopperabbo/kiroxy/internal/doctor"
)

// ToolsProvider is the data source for /dashboard/api/tools/*. The server
// calls Doctor() to run the diagnostic check; main.go's wiring reuses the
// existing vault + pool snapshots so the dashboard report matches what
// the operator would get from `kiroxy doctor` on the same process.
//
// Backup / restore are intentionally NOT exposed yet — the v1.2.0
// instructions are display-only cards. When backup tooling lands, this
// interface grows additional methods rather than overloading Doctor().
type ToolsProvider interface {
	Doctor(ctx context.Context) *doctor.Report
}

func (s *Server) registerToolsHandlers(mux *http.ServeMux) {
	if s.opts.ToolsProvider == nil {
		return
	}
	mux.HandleFunc("POST /dashboard/api/tools/doctor", s.handleToolsDoctor)
	mux.HandleFunc("GET /dashboard/api/tools/doctor", s.handleToolsDoctor)
}

func (s *Server) handleToolsDoctor(w http.ResponseWriter, r *http.Request) {
	rep := s.opts.ToolsProvider.Doctor(r.Context())
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if rep == nil {
		writeJSONError(w, http.StatusInternalServerError, "doctor_failed", "doctor returned nil report")
		return
	}
	_ = json.NewEncoder(w).Encode(rep)
}
