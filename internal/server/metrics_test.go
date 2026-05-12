// kiroxy addition — not derived from upstream.

package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"local/kiroxy/internal/metrics"
)

// TestMetricsEndpoint_NoRegistry_Returns503 verifies the zero-config path:
// when no metrics.Registry is wired, the endpoint surfaces a clear 503
// rather than an empty 200 that would look healthy to Prometheus.
func TestMetricsEndpoint_NoRegistry_Returns503(t *testing.T) {
	srv := New(Options{Version: "test"})
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("no-registry /metrics: got %d, want 503 body=%s", rr.Code, rr.Body.String())
	}
}

// TestMetricsEndpoint_LoopbackBypass verifies localhost can scrape without
// an API key, matching /dashboard's personal-use UX decision.
func TestMetricsEndpoint_LoopbackBypass(t *testing.T) {
	reg := metrics.New(time.Now())
	srv := New(Options{Version: "test", APIKey: "the-secret", Metrics: reg})
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("loopback /metrics: got %d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "kiroxy_uptime_seconds") {
		t.Errorf("expected kiroxy_uptime_seconds, body:\n%s", rr.Body.String())
	}
}

// TestMetricsEndpoint_NonLoopback_RequiresKey verifies a remote scraper
// without credentials is rejected.
func TestMetricsEndpoint_NonLoopback_RequiresKey(t *testing.T) {
	reg := metrics.New(time.Now())
	srv := New(Options{Version: "test", APIKey: "the-secret", Metrics: reg})
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.RemoteAddr = "192.168.1.42:12345"
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("non-loopback /metrics without key: got %d body=%s", rr.Code, rr.Body.String())
	}
}

// TestMetricsEndpoint_NonLoopback_WithKey accepts a valid API key for
// remote scrapes.
func TestMetricsEndpoint_NonLoopback_WithKey(t *testing.T) {
	reg := metrics.New(time.Now())
	srv := New(Options{Version: "test", APIKey: "the-secret", Metrics: reg})
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.RemoteAddr = "192.168.1.42:12345"
	req.Header.Set("X-Api-Key", "the-secret")
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("non-loopback /metrics with key: got %d body=%s", rr.Code, rr.Body.String())
	}
}

// TestMetricsEndpoint_PublicEnv accepts remote scrapes without a key when
// KIROXY_METRICS_PUBLIC=1.
func TestMetricsEndpoint_PublicEnv(t *testing.T) {
	t.Setenv("KIROXY_METRICS_PUBLIC", "1")
	reg := metrics.New(time.Now())
	srv := New(Options{Version: "test", APIKey: "the-secret", Metrics: reg})
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.RemoteAddr = "192.168.1.42:12345"
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("public /metrics: got %d body=%s", rr.Code, rr.Body.String())
	}
}

// TestMetricsEndpoint_ContentType verifies the response is the Prometheus
// text exposition format (text/plain). Catches accidental middleware that
// would rewrite the header.
func TestMetricsEndpoint_ContentType(t *testing.T) {
	reg := metrics.New(time.Now())
	srv := New(Options{Version: "test", Metrics: reg})
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/plain") {
		t.Errorf("Content-Type = %q, expected text/plain exposition format", ct)
	}
}
