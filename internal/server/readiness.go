package server

import (
	"context"
	"encoding/json"
	"net/http"
)

// ReadinessChecker answers whether a dependency is currently usable.
type ReadinessChecker func(ctx context.Context) error

type readiness struct {
	checks map[string]ReadinessChecker
}

func newReadiness() *readiness {
	return &readiness{checks: make(map[string]ReadinessChecker)}
}

func (r *readiness) register(name string, c ReadinessChecker) {
	if c != nil {
		r.checks[name] = c
	}
}

func (r *readiness) handle(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	results := make(map[string]string, len(r.checks))
	allOK := true
	for name, c := range r.checks {
		if err := c(ctx); err != nil {
			results[name] = err.Error()
			allOK = false
		} else {
			results[name] = "ok"
		}
	}
	w.Header().Set("Content-Type", "application/json")
	status := http.StatusOK
	if !allOK {
		status = http.StatusServiceUnavailable
	}
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status": boolToReadiness(allOK),
		"checks": results,
	})
}

func boolToReadiness(ok bool) string {
	if ok {
		return "ready"
	}
	return "not_ready"
}
