package messages

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nopperabbo/kiroxy/internal/kiroclient"
	"github.com/nopperabbo/kiroxy/internal/testutil"
)

func TestResolveContextPercent(t *testing.T) {
	tests := []struct {
		name            string
		reported        float64
		hasContextUsage bool
		inputTokens     int
		windowSize      int
		want            float64
	}{
		{"has context usage returns reported", 85.5, true, 1000, 200000, 85.5},
		{"zero window returns reported", 42.0, false, 1000, 0, 42.0},
		{"fallback calculation", 0, false, 100000, 200000, 50.0},
		{"fallback 100%", 0, false, 200000, 200000, 100.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveContextPercent(tt.reported, tt.hasContextUsage, tt.inputTokens, tt.windowSize)
			if got != tt.want {
				t.Errorf("resolveContextPercent() = %f, want %f", got, tt.want)
			}
		})
	}
}

func TestLogResponseStats_NoPanic(t *testing.T) {
	ctx := context.Background()
	logResponseStats(ctx, "abc12345", 1000, 500, true, 50.0, 200000)
	logResponseStats(ctx, "abc12345", 0, 0, false, 0, 200000)
	logResponseStats(ctx, "abc12345", 200000, 1000, true, 100.5, 200000)
}

func buildHeader(name, value string) []byte {
	nameBytes := []byte(name)
	valueBytes := []byte(value)
	h := []byte{byte(len(nameBytes))}
	h = append(h, nameBytes...)
	h = append(h, 7) // string type
	h = append(h, byte(len(valueBytes)>>8), byte(len(valueBytes)))
	h = append(h, valueBytes...)
	return h
}

func TestHandleNonStreamingResponse_InvalidStream(t *testing.T) {
	svc := New(&mockTokenGetter{}, &mockClient{})
	w := httptest.NewRecorder()
	apiResp := &kiroclient.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader([]byte("garbage"))),
		Header:     http.Header{"Content-Type": {"application/vnd.amazon.eventstream"}},
	}
	capture := &upstreamAttemptCapture{toolUseNames: make(map[string]int)}
	rm := &requestMetrics{}
	reason := svc.handleNonStreamingResponse(context.Background(), w, apiResp, "claude-sonnet-4-20250514", 200000, nil, 4096, 0, capture, nil, rm)
	if reason != "" {
		t.Errorf("expected empty reason for parse error, got %q", reason)
	}
	if w.Code != http.StatusBadGateway {
		t.Errorf("expected 502, got %d", w.Code)
	}
}

func TestHandleNonStreamingResponse_Exception(t *testing.T) {
	svc := New(&mockTokenGetter{}, &mockClient{})
	w := httptest.NewRecorder()

	// Exception is detected via :message-type header, not :event-type
	msgTypeHeader := buildHeader(":message-type", "exception")
	exTypeHeader := buildHeader(":exception-type", "InternalServerException")
	headers := append(msgTypeHeader, exTypeHeader...)
	payload := []byte(`{"message":"upstream broke"}`)
	frame := testutil.AssembleFrame(headers, payload)

	apiResp := &kiroclient.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(frame)),
		Header:     http.Header{"Content-Type": {"application/vnd.amazon.eventstream"}},
	}
	capture := &upstreamAttemptCapture{toolUseNames: make(map[string]int)}
	rm := &requestMetrics{}
	reason := svc.handleNonStreamingResponse(context.Background(), w, apiResp, "claude-sonnet-4-20250514", 200000, nil, 4096, 0, capture, nil, rm)
	if reason != "" {
		t.Errorf("expected empty reason for exception (non-retryable), got %q", reason)
	}
	if w.Code != http.StatusBadGateway {
		t.Errorf("expected 502, got %d", w.Code)
	}
}

func TestHandleNonStreamingResponse_InvalidState_Retryable(t *testing.T) {
	svc := New(&mockTokenGetter{}, &mockClient{})
	w := httptest.NewRecorder()

	payload := []byte(`{"reason":"STALE_CONVERSATION","message":"stale"}`)
	frame := testutil.BuildFrame("invalidStateEvent", payload)

	apiResp := &kiroclient.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(frame)),
		Header:     http.Header{"Content-Type": {"application/vnd.amazon.eventstream"}},
	}
	capture := &upstreamAttemptCapture{toolUseNames: make(map[string]int)}
	rm := &requestMetrics{}
	reason := svc.handleNonStreamingResponse(context.Background(), w, apiResp, "claude-sonnet-4-20250514", 200000, nil, 4096, 0, capture, nil, rm)
	if reason != "STALE_CONVERSATION" {
		t.Errorf("expected STALE_CONVERSATION retry reason, got %q", reason)
	}
}

func TestHandleStreamingResponse_InvalidStream(t *testing.T) {
	svc := New(&mockTokenGetter{}, &mockClient{})
	w := httptest.NewRecorder()
	apiResp := &kiroclient.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader([]byte("garbage"))),
		Header:     http.Header{"Content-Type": {"application/vnd.amazon.eventstream"}},
	}
	capture := &upstreamAttemptCapture{toolUseNames: make(map[string]int)}
	rm := &requestMetrics{}
	reason := svc.handleStreamingResponse(context.Background(), w, apiResp, "claude-sonnet-4-20250514", 200000, nil, 4096, 0, capture, nil, rm)
	if reason != "" {
		t.Errorf("expected empty reason for stream error, got %q", reason)
	}
	if w.Code != http.StatusBadGateway {
		t.Errorf("expected 502, got %d", w.Code)
	}
}

func TestHandleStreamingResponse_Exception(t *testing.T) {
	svc := New(&mockTokenGetter{}, &mockClient{})
	w := httptest.NewRecorder()

	msgTypeHeader := buildHeader(":message-type", "exception")
	exTypeHeader := buildHeader(":exception-type", "InternalServerException")
	headers := append(msgTypeHeader, exTypeHeader...)
	payload := []byte(`{"message":"upstream broke"}`)
	frame := testutil.AssembleFrame(headers, payload)

	apiResp := &kiroclient.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(frame)),
		Header:     http.Header{"Content-Type": {"application/vnd.amazon.eventstream"}},
	}
	capture := &upstreamAttemptCapture{toolUseNames: make(map[string]int)}
	rm := &requestMetrics{}
	reason := svc.handleStreamingResponse(context.Background(), w, apiResp, "claude-sonnet-4-20250514", 200000, nil, 4096, 0, capture, nil, rm)
	if reason != "" {
		t.Errorf("expected empty reason for exception, got %q", reason)
	}
	if w.Code != http.StatusBadGateway {
		t.Errorf("expected 502, got %d", w.Code)
	}
}

func TestHandleStreamingResponse_Success(t *testing.T) {
	svc := New(&mockTokenGetter{}, &mockClient{})
	w := httptest.NewRecorder()

	var buf bytes.Buffer
	buf.Write(testutil.BuildFrame("assistantResponseEvent", []byte(`{"content":"Hello"}`)))
	buf.Write(testutil.BuildFrame("meteringEvent", []byte(`{"usage":0.1,"inputTokens":10,"outputTokens":5}`)))

	apiResp := &kiroclient.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(&buf),
		Header:     http.Header{"Content-Type": {"application/vnd.amazon.eventstream"}},
	}
	capture := &upstreamAttemptCapture{toolUseNames: make(map[string]int)}
	rm := &requestMetrics{}
	reason := svc.handleStreamingResponse(context.Background(), w, apiResp, "claude-sonnet-4-20250514", 200000, nil, 4096, 0, capture, nil, rm)
	if reason != "" {
		t.Errorf("expected empty reason for success, got %q", reason)
	}
}

func TestHandleNonStreamingResponse_Success(t *testing.T) {
	svc := New(&mockTokenGetter{}, &mockClient{})
	w := httptest.NewRecorder()

	var buf bytes.Buffer
	buf.Write(testutil.BuildFrame("assistantResponseEvent", []byte(`{"content":"Hello world"}`)))
	buf.Write(testutil.BuildFrame("meteringEvent", []byte(`{"usage":0.1,"inputTokens":50,"outputTokens":20}`)))

	apiResp := &kiroclient.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(&buf),
		Header:     http.Header{"Content-Type": {"application/vnd.amazon.eventstream"}},
	}
	capture := &upstreamAttemptCapture{toolUseNames: make(map[string]int)}
	rm := &requestMetrics{}
	reason := svc.handleNonStreamingResponse(context.Background(), w, apiResp, "claude-sonnet-4-20250514", 200000, nil, 4096, 0, capture, nil, rm)
	if reason != "" {
		t.Errorf("expected empty reason for success, got %q", reason)
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("expected application/json content-type, got %q", w.Header().Get("Content-Type"))
	}
}

func TestWithCapture(t *testing.T) {
	svc := New(&mockTokenGetter{}, &mockClient{}, WithCapture(true))
	if !svc.captureEnabled {
		t.Error("expected captureEnabled = true")
	}
	svc2 := New(&mockTokenGetter{}, &mockClient{}, WithCapture(false))
	if svc2.captureEnabled {
		t.Error("expected captureEnabled = false")
	}
}

func TestHandleNonStreamingResponse_InvalidState_NonRetryable(t *testing.T) {
	svc := New(&mockTokenGetter{}, &mockClient{})
	w := httptest.NewRecorder()

	payload := []byte(`{"reason":"UNKNOWN_REASON","message":"bad"}`)
	frame := testutil.BuildFrame("invalidStateEvent", payload)

	apiResp := &kiroclient.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(frame)),
		Header:     http.Header{"Content-Type": {"application/vnd.amazon.eventstream"}},
	}
	capture := &upstreamAttemptCapture{toolUseNames: make(map[string]int)}
	rm := &requestMetrics{}
	reason := svc.handleNonStreamingResponse(context.Background(), w, apiResp, "claude-sonnet-4-20250514", 200000, nil, 4096, 0, capture, nil, rm)
	if reason != "" {
		t.Errorf("expected empty reason for non-retryable invalidState, got %q", reason)
	}
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleStreamingResponse_InvalidState_Retryable(t *testing.T) {
	svc := New(&mockTokenGetter{}, &mockClient{})
	w := httptest.NewRecorder()

	payload := []byte(`{"reason":"INVALID_CONVERSATION_STATE","message":"invalid"}`)
	frame := testutil.BuildFrame("invalidStateEvent", payload)

	apiResp := &kiroclient.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(frame)),
		Header:     http.Header{"Content-Type": {"application/vnd.amazon.eventstream"}},
	}
	capture := &upstreamAttemptCapture{toolUseNames: make(map[string]int)}
	rm := &requestMetrics{}
	reason := svc.handleStreamingResponse(context.Background(), w, apiResp, "claude-sonnet-4-20250514", 200000, nil, 4096, 0, capture, nil, rm)
	if reason != "INVALID_CONVERSATION_STATE" {
		t.Errorf("expected INVALID_CONVERSATION_STATE, got %q", reason)
	}
}
