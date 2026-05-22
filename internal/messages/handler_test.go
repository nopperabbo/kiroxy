package messages

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nopperabbo/kiroxy/internal/auth"
	"github.com/nopperabbo/kiroxy/internal/kiroclient"
	"github.com/nopperabbo/kiroxy/internal/kiroproto"
	"github.com/nopperabbo/kiroxy/internal/testutil"
)

type mockTokenGetter struct {
	creds *auth.Credentials
	err   error
}

func (m *mockTokenGetter) GetToken(_ context.Context) (*auth.Credentials, error) {
	return m.creds, m.err
}

type mockClient struct {
	resp *kiroclient.Response
	err  error
}

func (m *mockClient) GenerateAssistantResponse(_ context.Context, _ string, _ *kiroproto.Payload, _ string, _ string) (*kiroclient.Response, error) {
	return m.resp, m.err
}

func validAnthropicBody() string {
	return `{"model":"claude-sonnet-4-20250514","max_tokens":1024,"stream":true,"messages":[{"role":"user","content":"hello"}]}`
}

func TestHandleMessages_InvalidBody(t *testing.T) {
	svc := New(&mockTokenGetter{creds: &auth.Credentials{AccessToken: "tok", Region: "us-east-1"}}, &mockClient{})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader("not json"))
	r.Header.Set("Content-Type", "application/json")
	svc.HandleMessages(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleMessages_AuthError(t *testing.T) {
	svc := New(&mockTokenGetter{err: io.ErrUnexpectedEOF}, &mockClient{})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(validAnthropicBody()))
	r.Header.Set("Content-Type", "application/json")
	svc.HandleMessages(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestHandleMessages_UpstreamSuccess(t *testing.T) {
	client := &mockClient{
		resp: &kiroclient.Response{
			StatusCode:   200,
			Body:         io.NopCloser(bytes.NewReader([]byte("not-valid-eventstream"))),
			Header:       http.Header{"Content-Type": {"application/vnd.amazon.eventstream"}},
			PromptTokens: 10,
		},
	}
	svc := New(
		&mockTokenGetter{creds: &auth.Credentials{AccessToken: "tok", Region: "us-east-1", ProfileARN: "arn:aws:iam::123:profile/test"}},
		client,
	)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(validAnthropicBody()))
	r.Header.Set("Content-Type", "application/json")
	svc.HandleMessages(w, r)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 (stream error from malformed body), got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleMessages_UpstreamError(t *testing.T) {
	client := &mockClient{
		err: &kiroclient.UpstreamError{
			Status:    500,
			Exception: "InternalServerException",
			Reason:    "something broke",
			Body:      `{"message":"internal error"}`,
		},
	}
	svc := New(
		&mockTokenGetter{creds: &auth.Credentials{AccessToken: "tok", Region: "us-east-1", ProfileARN: "arn:aws:iam::123:profile/test"}},
		client,
	)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(validAnthropicBody()))
	r.Header.Set("Content-Type", "application/json")
	svc.HandleMessages(w, r)
	if w.Code == http.StatusOK {
		t.Fatalf("expected non-200, got 200")
	}
}

func TestHandleMessages_RequestTimeout(t *testing.T) {
	client := &mockClient{
		err: context.DeadlineExceeded,
	}
	svc := New(
		&mockTokenGetter{creds: &auth.Credentials{AccessToken: "tok", Region: "us-east-1", ProfileARN: "arn:aws:iam::123:profile/test"}},
		client,
		WithRequestTimeout(1), // 1ns — will expire immediately
	)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(validAnthropicBody()))
	r.Header.Set("Content-Type", "application/json")
	svc.HandleMessages(w, r)
	if w.Code == http.StatusOK {
		t.Fatalf("expected non-200 with expired timeout")
	}
}

func TestHandleMessages_ThinkingEnabled(t *testing.T) {
	client := &mockClient{
		resp: &kiroclient.Response{
			StatusCode:   200,
			Body:         io.NopCloser(bytes.NewReader([]byte("not-valid-eventstream"))),
			Header:       http.Header{"Content-Type": {"application/vnd.amazon.eventstream"}},
			PromptTokens: 10,
		},
	}
	svc := New(
		&mockTokenGetter{creds: &auth.Credentials{AccessToken: "tok", Region: "us-east-1", ProfileARN: "arn:aws:iam::123:profile/test"}},
		client,
	)
	w := httptest.NewRecorder()
	body := `{"model":"claude-sonnet-4-20250514","max_tokens":1024,"stream":true,"messages":[{"role":"user","content":"hello"}],"thinking":{"type":"enabled"},"output_config":{"effort":"high"}}`
	r := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	svc.HandleMessages(w, r)
	if w.Code == http.StatusOK {
		t.Fatalf("expected non-200 (stream error from malformed body)")
	}
}

func TestHandleMessages_NonStreaming(t *testing.T) {
	var buf bytes.Buffer
	buf.Write(testutil.BuildFrame("assistantResponseEvent", []byte(`{"content":"Hello"}`)))
	buf.Write(testutil.BuildFrame("meteringEvent", []byte(`{"usage":0.1,"inputTokens":50,"outputTokens":20}`)))

	client := &mockClient{
		resp: &kiroclient.Response{
			StatusCode:   200,
			Body:         io.NopCloser(&buf),
			Header:       http.Header{"Content-Type": {"application/vnd.amazon.eventstream"}},
			PromptTokens: 10,
		},
	}
	svc := New(
		&mockTokenGetter{creds: &auth.Credentials{AccessToken: "tok", Region: "us-east-1", ProfileARN: "arn:aws:iam::123:profile/test"}},
		client,
	)
	w := httptest.NewRecorder()
	body := `{"model":"claude-sonnet-4-20250514","max_tokens":1024,"stream":false,"messages":[{"role":"user","content":"hello"}]}`
	r := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	svc.HandleMessages(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleMessages_StreamingSuccess(t *testing.T) {
	var buf bytes.Buffer
	buf.Write(testutil.BuildFrame("assistantResponseEvent", []byte(`{"content":"Hello"}`)))
	buf.Write(testutil.BuildFrame("meteringEvent", []byte(`{"usage":0.1,"inputTokens":50,"outputTokens":20}`)))

	client := &mockClient{
		resp: &kiroclient.Response{
			StatusCode:   200,
			Body:         io.NopCloser(&buf),
			Header:       http.Header{"Content-Type": {"application/vnd.amazon.eventstream"}},
			PromptTokens: 10,
		},
	}
	svc := New(
		&mockTokenGetter{creds: &auth.Credentials{AccessToken: "tok", Region: "us-east-1", ProfileARN: "arn:aws:iam::123:profile/test"}},
		client,
	)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(validAnthropicBody()))
	r.Header.Set("Content-Type", "application/json")
	svc.HandleMessages(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}
