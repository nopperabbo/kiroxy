// This file is NOT derived from kirocc — it is a kiroxy addition.

package messages

import (
	"net/http"
	"time"

	"local/kiroxy/internal/metrics"
)

// WithMetrics attaches a metrics Sink to the service. Passing nil (or
// omitting the option) disables metric emission entirely; every sink call
// then becomes a no-op via the Sink type's nil-safety contract.
func WithMetrics(s *metrics.Sink) Option {
	return func(svc *Service) { svc.metrics = s }
}

// requestMetrics tracks the per-invocation state the Service uses to emit
// metrics once the handler has finished. The struct is cheap (small and
// short-lived), created inline at the top of HandleMessages and updated
// along the code path.
//
// A zero-value *requestMetrics is a valid no-op when metrics aren't wired,
// but in practice Service only allocates one when s.metrics != nil so that
// even the time.Now call is skipped on the default disabled path.
type requestMetrics struct {
	sink       *metrics.Sink
	start      time.Time
	model      string // canonical Anthropic ID returned to client
	stream     bool
	inputToks  int
	outputToks int
	ttfbSeen   bool // guard: observe TTFB only once per request
}

// newRequestMetrics returns a tracker iff the service has a Sink wired.
// Nil return is the caller's signal to skip all instrumentation.
func (s *Service) newRequestMetrics() *requestMetrics {
	if s.metrics == nil {
		return nil
	}
	return &requestMetrics{
		sink:  s.metrics,
		start: time.Now(),
	}
}

// setModel is called once the model has been resolved.
func (rm *requestMetrics) setModel(model string, stream bool) {
	if rm == nil {
		return
	}
	rm.model = model
	rm.stream = stream
}

// observeTTFB records upstream time-to-first-byte, once per request.
// Subsequent calls after the first are ignored so retries don't double-count.
func (rm *requestMetrics) observeTTFB(startedAt time.Time) {
	if rm == nil || rm.ttfbSeen {
		return
	}
	rm.ttfbSeen = true
	rm.sink.ObserveUpstreamTTFB(rm.model, time.Since(startedAt))
}

// setTokens updates the usage counts observed at end-of-stream.
func (rm *requestMetrics) setTokens(input, output int) {
	if rm == nil {
		return
	}
	rm.inputToks, rm.outputToks = input, output
}

// errKind records a typed request error and lets the caller decide whether
// to also emit a status-coded request observation in finalize. Multiple
// error kinds per request are allowed (rare, but possible in a retry path).
func (rm *requestMetrics) errKind(k metrics.RequestKind) {
	if rm == nil {
		return
	}
	rm.sink.RequestError(k)
}

// finalize is invoked exactly once after the request lifecycle completes.
// status is the HTTP status code written to the client.
func (rm *requestMetrics) finalize(status int) {
	if rm == nil {
		return
	}
	rm.sink.ObserveRequest(rm.model, status, rm.stream, time.Since(rm.start))
	if rm.inputToks > 0 || rm.outputToks > 0 {
		rm.sink.ObserveTokens(rm.model, rm.inputToks, rm.outputToks)
	}
}

// statusCapturingWriter wraps an http.ResponseWriter so we can recover the
// final status code in finalize without plumbing it through every return
// path. Defaults to 200 (net/http's implicit default when no WriteHeader
// was called before the first Write).
type statusCapturingWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusCapturingWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusCapturingWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(p)
}

func (w *statusCapturingWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// currentStatus returns 200 as the sensible default when no handler wrote
// a status; the net/http runtime implicitly uses 200 on first Write.
func (w *statusCapturingWriter) currentStatus() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}
