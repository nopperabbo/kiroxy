package server

import (
	"sync"
	"sync/atomic"
	"time"
)

// RequestRecord is one completed HTTP request as surfaced on the dashboard
// recent-requests feed. Only facets safe to show in a local developer UI
// are captured; request/response bodies are intentionally NOT kept.
type RequestRecord struct {
	// ID is the per-request ULID assigned by the logging middleware.
	ID string `json:"id"`
	// StartedAt is UTC wall-clock at request begin, ISO 8601 for JSON display.
	StartedAt time.Time `json:"started_at"`
	// LatencyMS is the full handler duration in milliseconds.
	LatencyMS int64 `json:"latency_ms"`
	// Method is the HTTP method (GET / POST / ...).
	Method string `json:"method"`
	// Path is the request URL path (no query string for dashboard tidiness).
	Path string `json:"path"`
	// Status is the HTTP status code the handler wrote.
	Status int `json:"status"`
	// BytesOut is the total body bytes written to the client.
	BytesOut int64 `json:"bytes_out"`
	// RemoteIP is the client's IP address (first X-Forwarded-For or RemoteAddr).
	RemoteIP string `json:"remote_ip,omitempty"`
	// UserAgent is the raw User-Agent header. Useful for spotting opencode vs
	// Claude Code vs curl during dev.
	UserAgent string `json:"user_agent,omitempty"`
}

// RequestRecorder receives notifications as requests complete. Implementations
// must be safe for concurrent use; the logging middleware calls Record from
// arbitrary goroutines.
type RequestRecorder interface {
	Record(RequestRecord)
}

// requestRecorderFunc adapts a plain function into the interface; unexported
// because no current caller needs it outside this package.
type requestRecorderFunc func(RequestRecord)

func (f requestRecorderFunc) Record(r RequestRecord) { f(r) }

// RequestRing is a fixed-capacity, FIFO in-memory buffer of RequestRecords
// plus a rolling counters snapshot. It is the data source for the dashboard
// v2 recent-requests feed.
//
// Design choices:
//   - Mutex over ringbuffer pointers instead of a channel: the snapshot method
//     needs a consistent read across all fields, and a channel gives only
//     push/pop semantics.
//   - Zero backpressure: if the consumer is slow we overwrite the oldest entry.
//     This is a dashboard, not an audit log.
//   - No background goroutine: counters decay lazily on read.
type RequestRing struct {
	mu       sync.Mutex
	buf      []RequestRecord
	capacity int
	head     int // index where the next Record will be written
	size     int // number of valid entries in the ring (<= capacity)

	// Counters maintained for the top-of-dashboard health panel. Both are
	// 64-bit so Load/Store are naturally atomic on every supported arch; we
	// only use atomic for counters that live outside the mutex.
	totalReqs atomic.Int64
	totalErrs atomic.Int64

	// subs are fan-out channels for SSE clients.
	subsMu sync.Mutex
	subs   map[chan RequestRecord]struct{}
}

// NewRequestRing returns a ring with capacity for N records. Capacities < 1
// are coerced to 1 (never panic at boot).
func NewRequestRing(capacity int) *RequestRing {
	if capacity < 1 {
		capacity = 1
	}
	return &RequestRing{
		buf:      make([]RequestRecord, capacity),
		capacity: capacity,
		subs:     map[chan RequestRecord]struct{}{},
	}
}

// Record appends a record, evicting the oldest if at capacity. Safe for
// concurrent use. Always returns immediately; slow subscribers get their
// newest event dropped rather than blocking the hot path.
func (r *RequestRing) Record(rec RequestRecord) {
	r.mu.Lock()
	r.buf[r.head] = rec
	r.head = (r.head + 1) % r.capacity
	if r.size < r.capacity {
		r.size++
	}
	r.mu.Unlock()

	r.totalReqs.Add(1)
	if rec.Status >= 400 {
		r.totalErrs.Add(1)
	}

	// Fan-out to SSE subs. Non-blocking: if a subscriber's buffer is full it
	// just misses this one record; SSE clients reconcile via the next state
	// snapshot tick.
	r.subsMu.Lock()
	for ch := range r.subs {
		select {
		case ch <- rec:
		default:
		}
	}
	r.subsMu.Unlock()
}

// Snapshot returns a newest-first copy of the current buffer up to max entries.
// max <= 0 means "all entries currently in the ring".
func (r *RequestRing) Snapshot(max int) []RequestRecord {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := r.size
	if max > 0 && max < n {
		n = max
	}
	out := make([]RequestRecord, 0, n)
	// Walk backward from the most recent write (head-1) wrapping around.
	for i := 0; i < n; i++ {
		idx := (r.head - 1 - i + r.capacity) % r.capacity
		out = append(out, r.buf[idx])
	}
	return out
}

// Counters is the lightweight status block surfaced in the top bar.
type Counters struct {
	TotalRequests int64 `json:"total_requests"`
	TotalErrors   int64 `json:"total_errors"`
	// ErrorRate is totalErrors / totalRequests, rounded to 4 places. 0 when
	// totalRequests is 0 so JSON stays a number (never NaN).
	ErrorRate float64 `json:"error_rate"`
}

// Counters returns a consistent snapshot of lifetime counters.
func (r *RequestRing) Counters() Counters {
	req := r.totalReqs.Load()
	errs := r.totalErrs.Load()
	c := Counters{TotalRequests: req, TotalErrors: errs}
	if req > 0 {
		c.ErrorRate = roundRate(float64(errs) / float64(req))
	}
	return c
}

// Subscribe registers a channel for per-record fan-out. Buffer size 8 is
// generous for a local SSE consumer; bursty traffic will cause oldest-first
// drop on the push side which is fine for a dashboard.
func (r *RequestRing) Subscribe() (<-chan RequestRecord, func()) {
	ch := make(chan RequestRecord, 8)
	r.subsMu.Lock()
	r.subs[ch] = struct{}{}
	r.subsMu.Unlock()
	cancel := func() {
		r.subsMu.Lock()
		delete(r.subs, ch)
		r.subsMu.Unlock()
		close(ch)
	}
	return ch, cancel
}

func roundRate(v float64) float64 {
	// 4 decimal places is plenty for an error-rate display.
	const scale = 10000
	return float64(int64(v*scale+0.5)) / scale
}
