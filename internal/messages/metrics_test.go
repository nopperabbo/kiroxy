// This file is NOT derived from kirocc — kiroxy addition.

package messages

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nopperabbo/kiroxy/internal/auth"
	"github.com/nopperabbo/kiroxy/internal/kiroclient"
	"github.com/nopperabbo/kiroxy/internal/kiroproto"
	"github.com/nopperabbo/kiroxy/internal/metrics"
)

type stubTG struct {
	creds *auth.Credentials
	err   error
}

func (s *stubTG) GetToken(_ context.Context) (*auth.Credentials, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.creds, nil
}

type stubKC struct {
	body []byte
	err  error
}

func (c *stubKC) GenerateAssistantResponse(_ context.Context, _ string, _ *kiroproto.Payload, _ string, _ string) (*kiroclient.Response, error) {
	if c.err != nil {
		return nil, c.err
	}
	return &kiroclient.Response{
		StatusCode:   http.StatusOK,
		Body:         io.NopCloser(bytes.NewReader(c.body)),
		Header:       http.Header{"Content-Type": []string{"application/vnd.amazon.eventstream"}},
		PromptTokens: 0,
	}, nil
}

// TestMetrics_AuthError_IncrementsAuthKind exercises the path where
// GetToken fails so we can verify the metrics sink receives the typed
// auth error kind and the request is observed with a 4xx status.
func TestMetrics_AuthError_IncrementsAuthKind(t *testing.T) {
	reg := metrics.New(time.Now())
	s := New(
		&stubTG{err: errors.New("boom")},
		&stubKC{},
		WithMetrics(reg.Sink()),
	)

	body := `{"model":"claude-sonnet-4-6","messages":[{"role":"user","content":"hi"}],"max_tokens":100}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Claude-Code-Session-Id", "test-session")
	rr := httptest.NewRecorder()

	s.HandleMessages(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", rr.Code, rr.Body.String())
	}

	scrape := scrapeRegistry(t, reg)
	if !strings.Contains(scrape, `kiroxy_request_errors_total{kind="auth"} 1`) {
		t.Errorf("expected auth error counter = 1, body:\n%s", scrape)
	}
	if !strings.Contains(scrape, `status="4xx"`) {
		t.Errorf("expected 4xx class label in requests_total, body:\n%s", scrape)
	}
}

// TestMetrics_InvalidRequest_IncrementsCounter covers an invalid-request
// path that exits before model resolution, so the model label should be
// the unknown-fallback. Malformed JSON is a stable pre-resolution failure.
func TestMetrics_InvalidRequest_IncrementsCounter(t *testing.T) {
	reg := metrics.New(time.Now())
	s := New(
		&stubTG{creds: &auth.Credentials{AccessToken: "t"}},
		&stubKC{},
		WithMetrics(reg.Sink()),
	)

	body := `{not valid json`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Claude-Code-Session-Id", "test-session")
	rr := httptest.NewRecorder()

	s.HandleMessages(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rr.Code, rr.Body.String())
	}

	scrape := scrapeRegistry(t, reg)
	if !strings.Contains(scrape, `kiroxy_request_errors_total{kind="invalid_request"} 1`) {
		t.Errorf("expected invalid_request error counter = 1, body:\n%s", scrape)
	}
	if !strings.Contains(scrape, `model="unknown"`) {
		t.Errorf("expected unknown model label for pre-resolution failure, body:\n%s", scrape)
	}
}

// TestMetrics_NilSink_HandlerStillRuns proves the handler is safe without a
// sink wired (previous behaviour preserved).
func TestMetrics_NilSink_HandlerStillRuns(t *testing.T) {
	s := New(
		&stubTG{creds: &auth.Credentials{AccessToken: "t"}},
		&stubKC{},
	)

	body := `{not valid json`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Claude-Code-Session-Id", "test-session")
	// Malformed JSON → 400, but no panic.
	rr := httptest.NewRecorder()

	s.HandleMessages(rr, req)
	if rr.Code == 0 {
		t.Fatal("handler did not write a status")
	}
}

// scrapeRegistry hits the registry's /metrics handler and returns the body.
func scrapeRegistry(t *testing.T, reg *metrics.Registry) string {
	t.Helper()
	rr := httptest.NewRecorder()
	reg.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("scrape code=%d body=%s", rr.Code, rr.Body.String())
	}
	return rr.Body.String()
}

func TestStatusCapturingWriter_WriteHeader(t *testing.T) {
	rr := httptest.NewRecorder()
	w := &statusCapturingWriter{ResponseWriter: rr}
	w.WriteHeader(http.StatusNotFound)
	if w.status != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.status, http.StatusNotFound)
	}
	w.WriteHeader(http.StatusOK)
	if w.status != http.StatusNotFound {
		t.Error("second WriteHeader should not overwrite")
	}
}

func TestStatusCapturingWriter_Write(t *testing.T) {
	rr := httptest.NewRecorder()
	w := &statusCapturingWriter{ResponseWriter: rr}
	n, err := w.Write([]byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if n != 5 {
		t.Errorf("n = %d, want 5", n)
	}
	if w.status != http.StatusOK {
		t.Errorf("status after Write = %d, want 200", w.status)
	}
	if rr.Body.String() != "hello" {
		t.Errorf("body = %q, want hello", rr.Body.String())
	}
}

func TestStatusCapturingWriter_Flush(t *testing.T) {
	rr := httptest.NewRecorder()
	w := &statusCapturingWriter{ResponseWriter: rr}
	w.Flush()
	if rr.Flushed != true {
		t.Error("expected Flush to delegate to underlying writer")
	}
}

func TestStatusCapturingWriter_CurrentStatus(t *testing.T) {
	rr := httptest.NewRecorder()
	w := &statusCapturingWriter{ResponseWriter: rr}
	if w.currentStatus() != http.StatusOK {
		t.Errorf("default currentStatus = %d, want 200", w.currentStatus())
	}
	w.WriteHeader(http.StatusCreated)
	if w.currentStatus() != http.StatusCreated {
		t.Errorf("currentStatus after WriteHeader = %d, want 201", w.currentStatus())
	}
}

func TestRequestMetrics_NilSafety(t *testing.T) {
	var rm *requestMetrics
	rm.setModel("model", true)
	rm.observeTTFB(time.Now())
	rm.setTokens(100, 50)
	rm.errKind(metrics.RequestKindAuth)
	rm.finalize(200)
}

func TestRequestMetrics_ObserveTTFB_OnlyOnce(t *testing.T) {
	reg := metrics.New(time.Now())
	rm := &requestMetrics{sink: reg.Sink(), start: time.Now(), model: "test"}
	rm.observeTTFB(time.Now().Add(-10 * time.Millisecond))
	rm.observeTTFB(time.Now().Add(-20 * time.Millisecond))
	if !rm.ttfbSeen {
		t.Error("expected ttfbSeen = true")
	}
}

func TestRequestMetrics_SetTokens(t *testing.T) {
	reg := metrics.New(time.Now())
	rm := &requestMetrics{sink: reg.Sink(), start: time.Now(), model: "test"}
	rm.setTokens(100, 50)
	if rm.inputToks != 100 || rm.outputToks != 50 {
		t.Errorf("tokens = (%d, %d), want (100, 50)", rm.inputToks, rm.outputToks)
	}
}

func TestRequestMetrics_SetModel(t *testing.T) {
	reg := metrics.New(time.Now())
	rm := &requestMetrics{sink: reg.Sink(), start: time.Now()}
	rm.setModel("claude-sonnet-4-20250514", true)
	if rm.model != "claude-sonnet-4-20250514" {
		t.Errorf("model = %q", rm.model)
	}
	if !rm.stream {
		t.Error("expected stream = true")
	}
}

func TestRequestMetrics_Finalize(t *testing.T) {
	reg := metrics.New(time.Now())
	rm := &requestMetrics{sink: reg.Sink(), start: time.Now(), model: "test", inputToks: 10, outputToks: 5}
	rm.finalize(200)
	scrape := scrapeRegistry(t, reg)
	if !strings.Contains(scrape, "kiroxy_requests_total") {
		t.Error("expected kiroxy_requests_total in metrics")
	}
}
