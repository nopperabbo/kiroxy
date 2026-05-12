// kiroxy addition — not derived from upstream.

package server

import (
	"net/http"
	"os"
)

// registerMetrics wires the /metrics endpoint onto mux. The auth decision
// lives in authMiddleware alongside /dashboard's loopback bypass so that
// all "personal-use UX" concessions are in one place.
//
// The handler itself is zero-configuration when no Registry is plugged in:
// it returns 503 so Prometheus surfaces a clear scrape failure rather than
// silently returning an empty document.
func (s *Server) registerMetrics(mux *http.ServeMux) {
	mux.Handle("GET /metrics", s.opts.Metrics.Handler())
}

// metricsIsPublic reports whether the operator has opted into unauthenticated
// /metrics scrapes (KIROXY_METRICS_PUBLIC=1).
func metricsIsPublic() bool {
	return os.Getenv("KIROXY_METRICS_PUBLIC") == "1"
}
