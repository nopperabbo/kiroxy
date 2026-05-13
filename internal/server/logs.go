package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

// registerLogsHandlers mounts /dashboard/api/logs (snapshot) and
// /dashboard/api/logs/stream (SSE). Both are no-ops when s.opts.LogSink is
// nil — the dashboard tolerates a 404 here gracefully.
func (s *Server) registerLogsHandlers(mux *http.ServeMux) {
	if s.opts.LogSink == nil {
		return
	}
	mux.HandleFunc("GET /dashboard/api/logs", s.handleLogsSnapshot)
	mux.HandleFunc("GET /dashboard/api/logs/stream", s.handleLogsStream)
}

// handleLogsSnapshot returns up to ?max=N (default 200, cap 1000) records
// matching the optional filters: level, source, search, since_id.
//
// Returns a JSON object (not a bare array) so we can grow the schema with
// counters / next_cursor without a breaking change.
func (s *Server) handleLogsSnapshot(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	max := parseIntDefault(q.Get("max"), 200)
	if max < 1 {
		max = 1
	}
	if max > 1000 {
		max = 1000
	}
	sinceID := uint64(parseIntDefault(q.Get("since_id"), 0))
	opts := LogSnapshotOpts{
		Max:     max,
		SinceID: sinceID,
		Level:   q.Get("level"),
		Source:  q.Get("source"),
		Search:  q.Get("search"),
	}
	rows := s.opts.LogSink.Snapshot(opts)
	resp := struct {
		Records  []LogRecord `json:"records"`
		Counters LogCounters `json:"counters"`
	}{
		Records:  rows,
		Counters: s.opts.LogSink.Counters(),
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(resp)
}

// handleLogsStream is an SSE endpoint that emits every captured log record as
// it lands, plus periodic 'heartbeat' comments to keep proxies from killing
// the connection.
//
// Optional Last-Event-ID: the standard SSE reconnection header. We honour it
// by replaying records with ID > last_event_id from the in-memory ring before
// switching to live fan-out. That keeps the UI's tail intact across brief
// network blips without a separate "since" handshake.
//
// Filter params (level, source, search, since_id) are honoured for both the
// initial replay AND the live stream, so a user with `level=ERROR` selected
// won't see INFO frames pushed into their tab.
func (s *Server) handleLogsStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	q := r.URL.Query()
	sinceID := uint64(parseIntDefault(q.Get("since_id"), 0))
	if hdr := r.Header.Get("Last-Event-ID"); hdr != "" {
		if v, err := strconv.ParseUint(hdr, 10, 64); err == nil && v > sinceID {
			sinceID = v
		}
	}
	level := q.Get("level")
	source := q.Get("source")
	search := q.Get("search")

	flusher.Flush()

	replay := s.opts.LogSink.Snapshot(LogSnapshotOpts{
		SinceID: sinceID,
		Level:   level,
		Source:  source,
		Search:  search,
		Max:     500,
	})
	for i := len(replay) - 1; i >= 0; i-- {
		writeLogSSE(w, replay[i])
	}
	flusher.Flush()

	ch, cancel := s.opts.LogSink.Subscribe()
	defer cancel()

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case rec, open := <-ch:
			if !open {
				return
			}
			if !logPassesFilter(rec, level, source, search) {
				continue
			}
			writeLogSSE(w, rec)
			flusher.Flush()
		case <-heartbeat.C:
			_, _ = w.Write([]byte(": ping\n\n"))
			flusher.Flush()
		}
	}
}

// logPassesFilter mirrors LogSink.Snapshot's filtering for live-stream frames
// so SSE doesn't push records the snapshot would have hidden. Cheap because
// the filter strings are tiny and the comparison is per-record.
func logPassesFilter(r LogRecord, level, source, search string) bool {
	if level != "" {
		if !strEqualFold(r.Level, level) {
			return false
		}
	}
	if source != "" {
		if !strContainsFold(r.Source, source) {
			return false
		}
	}
	if search != "" {
		needle := toLower(search)
		if !strContainsFold(r.Message, needle) {
			for _, v := range r.Fields {
				if strContainsFold(v, needle) {
					return true
				}
			}
			return false
		}
	}
	return true
}

func writeLogSSE(w http.ResponseWriter, r LogRecord) {
	b, err := json.Marshal(r)
	if err != nil {
		return
	}
	_, _ = w.Write([]byte("id: "))
	_, _ = w.Write([]byte(strconv.FormatUint(r.ID, 10)))
	_, _ = w.Write([]byte("\nevent: log\ndata: "))
	_, _ = w.Write(b)
	_, _ = w.Write([]byte("\n\n"))
}

func parseIntDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

// Pre-existing helper avoidance: bytes/strings already pull these in elsewhere
// but keeping them tiny + local lets logsink_handlers stand alone.
func strEqualFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		if asciiLower(a[i]) != asciiLower(b[i]) {
			return false
		}
	}
	return true
}

func strContainsFold(haystack, needle string) bool {
	if needle == "" {
		return true
	}
	if len(needle) > len(haystack) {
		return false
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		match := true
		for j := 0; j < len(needle); j++ {
			if asciiLower(haystack[i+j]) != asciiLower(needle[j]) {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func asciiLower(b byte) byte {
	if b >= 'A' && b <= 'Z' {
		return b + ('a' - 'A')
	}
	return b
}

func toLower(s string) string {
	out := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		out[i] = asciiLower(s[i])
	}
	return string(out)
}

// Silence: ensure context import is used even if we ever drop the heartbeat select.
var _ = context.Background
