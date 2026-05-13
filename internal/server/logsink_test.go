package server

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestLogSink_RecordAndSnapshot(t *testing.T) {
	s := NewLogSink(4)
	for i := 0; i < 3; i++ {
		s.Record(LogRecord{Time: time.Now(), Level: "INFO", Message: "msg"})
	}
	got := s.Snapshot(LogSnapshotOpts{})
	if len(got) != 3 {
		t.Fatalf("want 3 records, got %d", len(got))
	}
	if got[0].ID <= got[1].ID {
		t.Fatalf("want newest-first order by ID; got %+v", got)
	}
}

func TestLogSink_RingOverwrites(t *testing.T) {
	s := NewLogSink(2)
	s.Record(LogRecord{Message: "a"})
	s.Record(LogRecord{Message: "b"})
	s.Record(LogRecord{Message: "c"})
	got := s.Snapshot(LogSnapshotOpts{})
	if len(got) != 2 {
		t.Fatalf("want 2 records (capacity), got %d", len(got))
	}
	if got[0].Message != "c" || got[1].Message != "b" {
		t.Fatalf("want newest-first [c, b], got [%s, %s]", got[0].Message, got[1].Message)
	}
	c := s.Counters()
	if c.Total != 3 || c.Buffered != 2 || c.Capacity != 2 {
		t.Fatalf("want total=3 buffered=2 cap=2, got %+v", c)
	}
}

func TestLogSink_Filters(t *testing.T) {
	s := NewLogSink(10)
	s.Record(LogRecord{Level: "INFO", Source: "server/logging.go", Message: "http request", Fields: map[string]string{"path": "/v1/messages"}})
	s.Record(LogRecord{Level: "WARN", Source: "pool/refresh.go", Message: "rate limited"})
	s.Record(LogRecord{Level: "ERROR", Source: "messages/service.go", Message: "upstream failed", Fields: map[string]string{"err": "context deadline exceeded"}})

	if got := s.Snapshot(LogSnapshotOpts{Level: "error"}); len(got) != 1 || got[0].Level != "ERROR" {
		t.Fatalf("Level=error: want 1 record, got %d", len(got))
	}
	if got := s.Snapshot(LogSnapshotOpts{Source: "pool"}); len(got) != 1 || !strings.Contains(got[0].Source, "pool") {
		t.Fatalf("Source=pool: want 1 record, got %d", len(got))
	}
	if got := s.Snapshot(LogSnapshotOpts{Search: "messages"}); len(got) == 0 {
		t.Fatalf("Search=messages: want >0 records")
	}
	if got := s.Snapshot(LogSnapshotOpts{Max: 1}); len(got) != 1 {
		t.Fatalf("Max=1: want 1 record, got %d", len(got))
	}
}

func TestLogSink_SinceIDFastForward(t *testing.T) {
	s := NewLogSink(10)
	s.Record(LogRecord{Message: "a"})
	s.Record(LogRecord{Message: "b"})
	first := s.Snapshot(LogSnapshotOpts{})
	lastID := first[0].ID
	s.Record(LogRecord{Message: "c"})
	got := s.Snapshot(LogSnapshotOpts{SinceID: lastID})
	if len(got) != 1 || got[0].Message != "c" {
		t.Fatalf("SinceID: want [c], got %+v", got)
	}
}

func TestLogSink_Subscribe(t *testing.T) {
	s := NewLogSink(4)
	ch, cancel := s.Subscribe()
	defer cancel()

	done := make(chan LogRecord, 1)
	go func() {
		select {
		case r := <-ch:
			done <- r
		case <-time.After(1 * time.Second):
			done <- LogRecord{Message: "TIMEOUT"}
		}
	}()

	s.Record(LogRecord{Message: "hello"})
	got := <-done
	if got.Message != "hello" {
		t.Fatalf("want hello, got %q", got.Message)
	}
}

func TestCapturingHandler_PreservesInnerOutput(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	sink := NewLogSink(8)
	h := NewCapturingHandler(inner, sink)
	logger := slog.New(h)

	logger.Info("hello", slog.String("who", "world"), slog.Int("n", 42))
	logger.WithGroup("grp").Warn("inside", slog.String("k", "v"))

	if !strings.Contains(buf.String(), `"hello"`) {
		t.Fatalf("inner handler did not receive log; stderr=%q", buf.String())
	}

	got := sink.Snapshot(LogSnapshotOpts{})
	if len(got) != 2 {
		t.Fatalf("want 2 sink records, got %d", len(got))
	}
	if got[0].Message != "inside" || got[1].Message != "hello" {
		t.Fatalf("want newest-first [inside, hello]; got [%s, %s]", got[0].Message, got[1].Message)
	}
	if got[1].Fields["who"] != "world" || got[1].Fields["n"] != "42" {
		t.Fatalf("want fields who=world n=42; got %+v", got[1].Fields)
	}
	if v := got[0].Fields["grp.k"]; v != "v" {
		t.Fatalf("want grp.k=v; got %q (fields: %+v)", v, got[0].Fields)
	}
}

func TestCapturingHandler_EnabledDelegates(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})
	sink := NewLogSink(4)
	h := NewCapturingHandler(inner, sink)

	if h.Enabled(context.Background(), slog.LevelInfo) {
		t.Fatalf("Info should be disabled when inner level is Warn")
	}
	if !h.Enabled(context.Background(), slog.LevelWarn) {
		t.Fatalf("Warn should be enabled")
	}
}
