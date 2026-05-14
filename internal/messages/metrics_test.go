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

	"local/kiroxy/internal/auth"
	"local/kiroxy/internal/kiroclient"
	"local/kiroxy/internal/kiroproto"
	"local/kiroxy/internal/metrics"
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

func (c *stubKC) GenerateAssistantResponse(_ context.Context, _ string, _ *kiroproto.Payload, _ string) (*kiroclient.Response, error) {
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
