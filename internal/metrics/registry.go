// Package metrics exposes kiroxy operational telemetry in the Prometheus
// text-exposition format.
//
// Design
//
//  1. Exactly one process-wide *Registry is constructed at boot and injected
//     into the components that need to emit or advertise metrics
//     (messages.Service, pool.Pool, tokenvault.Vault, and the HTTP server).
//     All metrics are registered on the Registry's underlying *prometheus.Registry;
//     no package-level globals are used, so tests can build isolated registries.
//  2. Instrumentation hot paths use the *Sink interface, which is nil-safe:
//     a nil *Sink is a valid zero-value no-op. This keeps instrumentation
//     calls free of repetitive nil checks in caller code while allowing
//     tests to run without a registry.
//  3. Pool and vault snapshots are exposed via prometheus.GaugeFunc closures
//     so we never run a polling goroutine; Prometheus scrapes pull a live
//     snapshot. Cardinality is bounded by the registered collectors.
//  4. Labels are restricted to bounded sets (see doc.go for catalog).
//     Account IDs, tokens, and user identifiers never appear as labels.
package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Registry is the single process-wide home for kiroxy metrics. It owns a
// prometheus.Registry and the set of Collectors registered on it.
//
// Zero value is NOT usable; construct via New.
type Registry struct {
	reg *prometheus.Registry

	// Counters
	requestsTotal        *prometheus.CounterVec
	requestErrorsTotal   *prometheus.CounterVec
	refreshAttemptsTotal *prometheus.CounterVec
	cooldownsTotal       *prometheus.CounterVec

	// Histograms
	requestDuration *prometheus.HistogramVec
	upstreamTTFB    *prometheus.HistogramVec
	tokensInput     *prometheus.HistogramVec
	tokensOutput    *prometheus.HistogramVec

	// Static gauges
	uptime prometheus.GaugeFunc

	// Start time for uptime derivation.
	startedAt time.Time
}

// DefaultRequestBuckets are the histogram buckets for request duration and
// upstream TTFB. Optimised for the kiroxy use case where most requests are
// 1–30s streaming generations; shorter buckets below 0.1s are redundant
// because Prometheus le=+Inf captures everything above.
var (
	// DefaultRequestDurationBuckets: 100ms .. 60s, log-ish spacing.
	DefaultRequestDurationBuckets = []float64{0.1, 0.25, 0.5, 1, 2, 5, 10, 20, 30, 60}

	// DefaultTTFBBuckets: 100ms .. 5s; first-byte is usually well under 1s.
	DefaultTTFBBuckets = []float64{0.1, 0.25, 0.5, 0.75, 1, 1.5, 2, 3, 5}

	// DefaultTokenBuckets: exponential up to ~200k (default context window).
	// Input/output tokens span orders of magnitude; exponential buckets
	// give reasonable resolution across that range.
	DefaultTokenBuckets = prometheus.ExponentialBuckets(100, 2, 12) // 100 .. 204800
)

// New returns a Registry with all kiroxy metrics registered on a fresh
// underlying *prometheus.Registry.
//
// startedAt is captured for the kiroxy_uptime_seconds gauge; pass the same
// value the server uses for its internal uptime so scrapes agree.
func New(startedAt time.Time) *Registry {
	reg := prometheus.NewRegistry()

	// Go + process collectors are standard inclusions for any Prometheus
	// target. They give heap, goroutines, and FD counts for free.
	reg.MustRegister(collectors.NewGoCollector())
	reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	r := &Registry{
		reg:       reg,
		startedAt: startedAt,
	}

	r.requestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "kiroxy",
			Name:      "requests_total",
			Help:      "Count of /v1/messages requests partitioned by model, status class, and streaming flag.",
		},
		[]string{"model", "status", "stream"},
	)
	reg.MustRegister(r.requestsTotal)

	r.requestErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "kiroxy",
			Name:      "request_errors_total",
			Help:      "Count of /v1/messages errors partitioned by error kind (upstream|auth|proxy|invalid_request).",
		},
		[]string{"kind"},
	)
	reg.MustRegister(r.requestErrorsTotal)

	r.refreshAttemptsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "kiroxy",
			Name:      "refresh_attempts_total",
			Help:      "Count of token refresh outcomes. kind=(proactive|reactive), result=(success|fail_401|fail_transient|fail_other).",
		},
		[]string{"kind", "result"},
	)
	reg.MustRegister(r.refreshAttemptsTotal)

	r.cooldownsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "kiroxy",
			Name:      "account_cooldowns_total",
			Help:      "Count of account cooldowns applied, by reason. reason=(quota|consecutive_errors|unauthorized|manual).",
		},
		[]string{"reason"},
	)
	reg.MustRegister(r.cooldownsTotal)

	r.requestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "kiroxy",
			Name:      "request_duration_seconds",
			Help:      "End-to-end handler latency for /v1/messages in seconds.",
			Buckets:   DefaultRequestDurationBuckets,
		},
		[]string{"model", "stream"},
	)
	reg.MustRegister(r.requestDuration)

	r.upstreamTTFB = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "kiroxy",
			Name:      "upstream_ttfb_seconds",
			Help:      "Time from upstream call start to first byte received, in seconds.",
			Buckets:   DefaultTTFBBuckets,
		},
		[]string{"model"},
	)
	reg.MustRegister(r.upstreamTTFB)

	r.tokensInput = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "kiroxy",
			Name:      "tokens_input",
			Help:      "Input token count per completed request.",
			Buckets:   DefaultTokenBuckets,
		},
		[]string{"model"},
	)
	reg.MustRegister(r.tokensInput)

	r.tokensOutput = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "kiroxy",
			Name:      "tokens_output",
			Help:      "Output token count per completed request.",
			Buckets:   DefaultTokenBuckets,
		},
		[]string{"model"},
	)
	reg.MustRegister(r.tokensOutput)

	r.uptime = prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Namespace: "kiroxy",
			Name:      "uptime_seconds",
			Help:      "Process uptime in seconds.",
		},
		func() float64 { return time.Since(r.startedAt).Seconds() },
	)
	reg.MustRegister(r.uptime)

	return r
}

// Registerer exposes the underlying Prometheus registerer so callers can
// attach custom collectors (e.g. pool/vault GaugeFunc snapshots) without
// needing to reach through.
func (r *Registry) Registerer() prometheus.Registerer {
	if r == nil {
		return nil
	}
	return r.reg
}

// Gatherer returns the underlying Prometheus gatherer used by the HTTP
// scrape handler.
func (r *Registry) Gatherer() prometheus.Gatherer {
	if r == nil {
		return nil
	}
	return r.reg
}

// Handler returns an http.Handler that serves the text-exposition format on
// every request.
//
// Compression (gzip) is enabled when the client Accept-Encoding header
// requests it; promhttp handles this transparently.
//
// Errors during scrape are surfaced as HTTP 500 with a short textual body
// so operators can diagnose (e.g. a registered GaugeFunc panicked).
func (r *Registry) Handler() http.Handler {
	if r == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "metrics registry not configured", http.StatusServiceUnavailable)
		})
	}
	return promhttp.HandlerFor(r.reg, promhttp.HandlerOpts{
		ErrorHandling:     promhttp.HTTPErrorOnError,
		DisableCompression: false,
		Timeout:           5 * time.Second,
	})
}

// Sink returns a Sink backed by this Registry. Instrumentation code should
// hold a *Sink (nil-safe) rather than a *Registry.
func (r *Registry) Sink() *Sink {
	if r == nil {
		return nil
	}
	return &Sink{reg: r}
}
