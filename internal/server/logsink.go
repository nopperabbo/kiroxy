package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// LogRecord is one captured slog record surfaced on the dashboard Logs view.
//
// The shape is deliberately minimal: a timestamp, a severity level, a source
// (the caller's package when slog's AddSource is on, else empty), a message,
// and the structured attributes as a pre-flattened map. Callers that want to
// render a row should not have to re-flatten groups.
type LogRecord struct {
	// ID is a monotonically increasing sequence number assigned at capture
	// time. Lets SSE clients filter out duplicates after reconnect (send
	// Last-Event-ID: <seq> and the server fast-forwards past it).
	ID uint64 `json:"id"`
	// Time is UTC wall-clock at log emission, ISO 8601 for JSON.
	Time time.Time `json:"time"`
	// Level is the short uppercase slog level name (DEBUG/INFO/WARN/ERROR).
	Level string `json:"level"`
	// Source is the caller's short file:line when AddSource is enabled.
	// Empty string is tolerated — the UI shows "—" in that case.
	Source string `json:"source,omitempty"`
	// Message is the log message (slog's msg arg).
	Message string `json:"message"`
	// Fields holds the structured attrs flattened to string values. Groups
	// become dotted keys ("group.key"). The values are JSON-encoded so the
	// UI can render them verbatim inside a <code> block when expanded.
	Fields map[string]string `json:"fields,omitempty"`
}

// LogSink is a process-wide capture of recent log records + a fan-out channel
// for SSE subscribers. It is populated by wrapping slog's handler with
// NewCapturingHandler. Sink methods are safe for concurrent use.
//
// Design choices mirror RequestRing in dashboard_sink.go:
//   - Ring buffer with overwrite on full. This is operator telemetry, not an
//     audit log; dropping the oldest 5% under sustained load is acceptable.
//   - Fan-out to SSE via buffered channels with non-blocking send. A slow
//     consumer misses events; the server never stalls on slog.
//   - Counters via atomic.Int64 so the status panel can read without taking
//     the ring mutex.
type LogSink struct {
	mu       sync.Mutex
	buf      []LogRecord
	capacity int
	head     int
	size     int
	seq      atomic.Uint64

	subsMu sync.Mutex
	subs   map[chan LogRecord]struct{}
}

// NewLogSink returns a sink holding up to capacity records. A capacity of <1
// is coerced to 1 (never panic at boot). Typical production value: 2048.
func NewLogSink(capacity int) *LogSink {
	if capacity < 1 {
		capacity = 1
	}
	return &LogSink{
		buf:      make([]LogRecord, capacity),
		capacity: capacity,
		subs:     map[chan LogRecord]struct{}{},
	}
}

// Record appends a LogRecord, evicting the oldest when at capacity. The ID is
// assigned here so the sequence is monotonic across concurrent handlers.
func (s *LogSink) Record(r LogRecord) {
	if r.ID == 0 {
		r.ID = s.seq.Add(1)
	}

	s.mu.Lock()
	s.buf[s.head] = r
	s.head = (s.head + 1) % s.capacity
	if s.size < s.capacity {
		s.size++
	}
	s.mu.Unlock()

	s.subsMu.Lock()
	for ch := range s.subs {
		select {
		case ch <- r:
		default:
			// Slow subscriber — drop this record, they will catch up
			// on next snapshot poll.
		}
	}
	s.subsMu.Unlock()
}

// LogSnapshotOpts filters a snapshot read. Zero values mean "no filter".
type LogSnapshotOpts struct {
	// Max caps the number of records returned (newest-first). 0 = all.
	Max int
	// SinceID returns only records with ID > SinceID.
	SinceID uint64
	// Level, when non-empty, keeps only records whose level equals (case
	// insensitive) the given value. Values: DEBUG, INFO, WARN, ERROR.
	Level string
	// Source, when non-empty, is a substring match against the record's
	// Source field.
	Source string
	// Search, when non-empty, is a substring match against the record's
	// Message OR any field value. Case-insensitive.
	Search string
}

// Snapshot returns records newest-first after applying the filter. The
// returned slice is a copy; callers may mutate it freely.
func (s *LogSink) Snapshot(opts LogSnapshotOpts) []LogRecord {
	s.mu.Lock()
	// Materialize newest-first view under the lock, then filter outside so
	// the lock window stays tight.
	view := make([]LogRecord, 0, s.size)
	for i := 0; i < s.size; i++ {
		idx := (s.head - 1 - i + s.capacity) % s.capacity
		view = append(view, s.buf[idx])
	}
	s.mu.Unlock()

	wantLevel := strings.ToUpper(strings.TrimSpace(opts.Level))
	wantSource := strings.ToLower(strings.TrimSpace(opts.Source))
	wantSearch := strings.ToLower(strings.TrimSpace(opts.Search))

	out := make([]LogRecord, 0, len(view))
	for _, r := range view {
		if opts.SinceID > 0 && r.ID <= opts.SinceID {
			continue
		}
		if wantLevel != "" && !strings.EqualFold(r.Level, wantLevel) {
			continue
		}
		if wantSource != "" && !strings.Contains(strings.ToLower(r.Source), wantSource) {
			continue
		}
		if wantSearch != "" && !logMatches(r, wantSearch) {
			continue
		}
		out = append(out, r)
		if opts.Max > 0 && len(out) >= opts.Max {
			break
		}
	}
	return out
}

// logMatches reports whether the search string appears in the message or any
// attribute value. Case-insensitive.
func logMatches(r LogRecord, needle string) bool {
	if strings.Contains(strings.ToLower(r.Message), needle) {
		return true
	}
	for _, v := range r.Fields {
		if strings.Contains(strings.ToLower(v), needle) {
			return true
		}
	}
	return false
}

// LogCounters returns lifetime counts + current buffer size for the Logs view
// status header. Total is the sequence number (strictly monotonic, never
// resets); Buffered is the number of records currently retained.
type LogCounters struct {
	Total    uint64 `json:"total"`
	Buffered int    `json:"buffered"`
	Capacity int    `json:"capacity"`
}

// Counters returns a consistent snapshot. The read of s.size takes the ring
// lock; s.seq is a plain atomic.
func (s *LogSink) Counters() LogCounters {
	s.mu.Lock()
	n := s.size
	s.mu.Unlock()
	return LogCounters{
		Total:    s.seq.Load(),
		Buffered: n,
		Capacity: s.capacity,
	}
}

// Subscribe registers an SSE fan-out channel. The channel buffer of 32 is
// large enough for bursty startup logs without blocking the hot path. The
// returned cancel closes the channel and removes the subscription.
func (s *LogSink) Subscribe() (<-chan LogRecord, func()) {
	ch := make(chan LogRecord, 32)
	s.subsMu.Lock()
	s.subs[ch] = struct{}{}
	s.subsMu.Unlock()
	cancel := func() {
		s.subsMu.Lock()
		delete(s.subs, ch)
		s.subsMu.Unlock()
		close(ch)
	}
	return ch, cancel
}

// capturingHandler wraps an inner slog.Handler (typically a JSONHandler that
// writes to stderr) and, for every record, also emits a normalized LogRecord
// to the LogSink. The inner handler sees the record first so stderr and the
// sink stay consistent even if the sink blocks (which it does not — Record
// is non-blocking).
type capturingHandler struct {
	inner slog.Handler
	sink  *LogSink
	// attrs + group are the slog "prefixes" added by WithAttrs/WithGroup on
	// this handler instance. They need to be applied before capturing so
	// the sink sees the same shape stderr does.
	attrs []slog.Attr
	group string
}

// NewCapturingHandler returns a slog.Handler that writes to both inner AND
// sink. Pass stderr-JSONHandler as inner to preserve normal log output.
func NewCapturingHandler(inner slog.Handler, sink *LogSink) slog.Handler {
	return &capturingHandler{inner: inner, sink: sink}
}

// Enabled is delegated to the inner handler so -level plumbing (KIROXY_LOG_LEVEL)
// continues to gate output consistently. The sink never sees records the
// inner handler would drop.
func (h *capturingHandler) Enabled(ctx context.Context, lvl slog.Level) bool {
	return h.inner.Enabled(ctx, lvl)
}

func (h *capturingHandler) Handle(ctx context.Context, r slog.Record) error {
	// inner first so stderr stays authoritative; if it errors we still emit
	// to the sink (operators may only see the sink during a disk-full
	// scenario where stderr fails).
	innerErr := h.inner.Handle(ctx, r)

	fields := make(map[string]string, r.NumAttrs()+len(h.attrs))
	// Apply WithAttrs-accumulated attrs first so later record attrs can
	// override. Prefix with h.group if present.
	for _, a := range h.attrs {
		flattenAttr(fields, h.group, a)
	}
	r.Attrs(func(a slog.Attr) bool {
		flattenAttr(fields, h.group, a)
		return true
	})

	source := ""
	if r.PC != 0 {
		// Cheap source formatting: filename:line from the PC. We avoid
		// slog's runtime.CallersFrames call allocation by reading the
		// record's Source attr when the outer JSONHandler has AddSource
		// enabled; fall back to empty when not. Keep it best-effort.
		if v, ok := fields["source"]; ok {
			source = v
		}
	}

	lvl := r.Level.String()
	switch {
	case r.Level < slog.LevelInfo:
		lvl = "DEBUG"
	case r.Level < slog.LevelWarn:
		lvl = "INFO"
	case r.Level < slog.LevelError:
		lvl = "WARN"
	default:
		lvl = "ERROR"
	}

	h.sink.Record(LogRecord{
		Time:    r.Time.UTC(),
		Level:   lvl,
		Source:  source,
		Message: r.Message,
		Fields:  fields,
	})
	return innerErr
}

func (h *capturingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, 0, len(h.attrs)+len(attrs))
	newAttrs = append(newAttrs, h.attrs...)
	newAttrs = append(newAttrs, attrs...)
	return &capturingHandler{
		inner: h.inner.WithAttrs(attrs),
		sink:  h.sink,
		attrs: newAttrs,
		group: h.group,
	}
}

func (h *capturingHandler) WithGroup(name string) slog.Handler {
	next := h.group
	if name != "" {
		if next == "" {
			next = name
		} else {
			next = next + "." + name
		}
	}
	return &capturingHandler{
		inner: h.inner.WithGroup(name),
		sink:  h.sink,
		attrs: h.attrs,
		group: next,
	}
}

// flattenAttr walks a slog.Attr (including nested groups) and writes every
// leaf value as a JSON-encoded string into out. Group names produce dotted
// keys to match what operators see in JSON logs.
func flattenAttr(out map[string]string, prefix string, a slog.Attr) {
	key := a.Key
	if prefix != "" {
		key = prefix + "." + key
	}
	v := a.Value.Resolve()
	if v.Kind() == slog.KindGroup {
		for _, ga := range v.Group() {
			flattenAttr(out, key, ga)
		}
		return
	}
	out[key] = valueToJSONString(v)
}

func valueToJSONString(v slog.Value) string {
	switch v.Kind() {
	case slog.KindString:
		return v.String()
	case slog.KindInt64:
		return strconv.FormatInt(v.Int64(), 10)
	case slog.KindUint64:
		return strconv.FormatUint(v.Uint64(), 10)
	case slog.KindFloat64:
		return strconv.FormatFloat(v.Float64(), 'g', -1, 64)
	case slog.KindBool:
		if v.Bool() {
			return "true"
		}
		return "false"
	case slog.KindDuration:
		return v.Duration().String()
	case slog.KindTime:
		return v.Time().UTC().Format(time.RFC3339Nano)
	default:
		// Any — best-effort JSON encode. Failures produce an empty string
		// rather than a Go %v fallback (less confusing in the UI).
		b, err := json.Marshal(v.Any())
		if err != nil {
			if s, ok := v.Any().(string); ok {
				return s
			}
			if e, ok := v.Any().(error); ok {
				return e.Error()
			}
			return ""
		}
		return string(b)
	}
}

// Silence unused-import warnings when the file is read in isolation.
var _ = strconv.Itoa
