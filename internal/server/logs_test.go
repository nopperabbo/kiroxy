package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestLogsHandler_Disabled404(t *testing.T) {
	s := New(Options{Version: "test"})
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	for _, path := range []string{"/dashboard/api/logs", "/dashboard/api/logs/stream"} {
		res, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		if res.StatusCode != http.StatusNotFound {
			t.Fatalf("GET %s: want 404 when LogSink is nil, got %d", path, res.StatusCode)
		}
		res.Body.Close()
	}
}

func TestLogsHandler_Snapshot(t *testing.T) {
	sink := NewLogSink(8)
	sink.Record(LogRecord{Time: time.Now(), Level: "INFO", Message: "http request", Source: "server/logging.go", Fields: map[string]string{"path": "/v1/messages"}})
	sink.Record(LogRecord{Time: time.Now(), Level: "ERROR", Message: "upstream failed", Source: "messages/service.go"})

	s := New(Options{Version: "test", LogSink: sink})
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	res, err := http.Get(ts.URL + "/dashboard/api/logs")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", res.StatusCode)
	}
	var payload struct {
		Records  []LogRecord `json:"records"`
		Counters LogCounters `json:"counters"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(payload.Records) != 2 {
		t.Fatalf("want 2 records, got %d", len(payload.Records))
	}
	if payload.Counters.Total != 2 {
		t.Fatalf("want counters.Total=2, got %d", payload.Counters.Total)
	}
}

func TestLogsHandler_LevelFilter(t *testing.T) {
	sink := NewLogSink(8)
	sink.Record(LogRecord{Level: "INFO", Message: "a"})
	sink.Record(LogRecord{Level: "ERROR", Message: "b"})
	sink.Record(LogRecord{Level: "WARN", Message: "c"})

	s := New(Options{Version: "test", LogSink: sink})
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	res, err := http.Get(ts.URL + "/dashboard/api/logs?level=ERROR")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer res.Body.Close()
	var payload struct {
		Records []LogRecord `json:"records"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(payload.Records) != 1 || payload.Records[0].Level != "ERROR" {
		t.Fatalf("want 1 ERROR record, got %+v", payload.Records)
	}
}

func TestLogsHandler_StreamReplay(t *testing.T) {
	sink := NewLogSink(8)
	sink.Record(LogRecord{Level: "INFO", Message: "first"})
	sink.Record(LogRecord{Level: "INFO", Message: "second"})

	s := New(Options{Version: "test", LogSink: sink})
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	req, err := http.NewRequest("GET", ts.URL+"/dashboard/api/logs/stream", nil)
	if err != nil {
		t.Fatalf("new req: %v", err)
	}

	client := &http.Client{Timeout: 3 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer res.Body.Close()

	if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("want text/event-stream, got %q", ct)
	}

	buf := make([]byte, 4096)
	deadline := time.Now().Add(2 * time.Second)
	var collected strings.Builder
	for time.Now().Before(deadline) {
		n, _ := res.Body.Read(buf)
		if n > 0 {
			collected.Write(buf[:n])
			if strings.Contains(collected.String(), `"first"`) && strings.Contains(collected.String(), `"second"`) {
				break
			}
		} else {
			time.Sleep(50 * time.Millisecond)
		}
	}
	got := collected.String()
	if !strings.Contains(got, `"first"`) || !strings.Contains(got, `"second"`) {
		t.Fatalf("want both replayed records in stream, got:\n%s", got)
	}
	if !strings.Contains(got, "event: log") {
		t.Fatalf("want SSE 'event: log' framing, got:\n%s", got)
	}
}
