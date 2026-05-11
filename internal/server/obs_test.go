package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestM7_RequestIDEchoedAndGenerated(t *testing.T) {
	srv := New(Options{})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	gen := resp.Header.Get("X-Request-Id")
	if gen == "" {
		t.Fatal("server did not generate X-Request-Id")
	}
	if len(gen) < 16 {
		t.Fatalf("generated request id too short: %q", gen)
	}

	req, _ := http.NewRequest("GET", ts.URL+"/healthz", nil)
	req.Header.Set("X-Request-Id", "custom-trace-123")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if got := resp.Header.Get("X-Request-Id"); got != "custom-trace-123" {
		t.Fatalf("server did not echo client request id; got %q", got)
	}
}

func TestM7_ReadyzReturns200WhenAllChecksPass(t *testing.T) {
	srv := New(Options{
		ReadinessChecks: map[string]ReadinessChecker{
			"always_ok": func(_ context.Context) error { return nil },
			"also_ok":   func(_ context.Context) error { return nil },
		},
	})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	resp, err := http.Get(ts.URL + "/readyz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("want 200, got %d body=%s", resp.StatusCode, body)
	}
	var payload struct {
		Status string            `json:"status"`
		Checks map[string]string `json:"checks"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&payload)
	if payload.Status != "ready" {
		t.Errorf("want status=ready, got %s", payload.Status)
	}
	if payload.Checks["always_ok"] != "ok" || payload.Checks["also_ok"] != "ok" {
		t.Errorf("want both checks ok, got %+v", payload.Checks)
	}
}

func TestM7_ReadyzReturns503WhenAnyCheckFails(t *testing.T) {
	srv := New(Options{
		ReadinessChecks: map[string]ReadinessChecker{
			"vault_offline": func(_ context.Context) error { return errors.New("connection refused") },
			"pool_ok":       func(_ context.Context) error { return nil },
		},
	})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	resp, err := http.Get(ts.URL + "/readyz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("want 503, got %d body=%s", resp.StatusCode, body)
	}
	var payload struct {
		Status string            `json:"status"`
		Checks map[string]string `json:"checks"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&payload)
	if payload.Status != "not_ready" {
		t.Errorf("want status=not_ready, got %s", payload.Status)
	}
	if payload.Checks["vault_offline"] != "connection refused" {
		t.Errorf("bad failure reason: %+v", payload.Checks)
	}
}

// TestM7_RequestLogContainsExpectedFields captures stderr log output to verify
// the structured json logger emits request_id, method, path, status, latency_ms
// for every non-healthz request.
func TestM7_RequestLogContainsExpectedFields(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	srv := New(Options{
		APIKey: "test-key",
		Logger: logger,
	})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	req, _ := http.NewRequest("POST", ts.URL+"/v1/messages", strings.NewReader("{}"))
	req.Header.Set("X-Api-Key", "test-key")
	req.Header.Set("X-Request-Id", "m7-req-id")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.ReadAll(resp.Body)
	resp.Body.Close()

	logLine := buf.String()
	for _, want := range []string{
		`"request_id":"m7-req-id"`,
		`"method":"POST"`,
		`"path":"/v1/messages"`,
		`"status":`,
		`"latency_ms":`,
	} {
		if !strings.Contains(logLine, want) {
			t.Errorf("missing %s in log line: %s", want, logLine)
		}
	}
}
