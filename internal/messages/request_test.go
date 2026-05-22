package messages

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nopperabbo/kiroxy/internal/anthropic"
)

func TestBuildCountTokensInput(t *testing.T) {
	t.Run("system text", func(t *testing.T) {
		req := &anthropic.Request{
			System:   anthropic.SystemPrompt{Text: "You are helpful"},
			Messages: []anthropic.Message{{Role: "user", Content: anthropic.MessageContent{Text: "hi"}}},
		}
		got := string(buildCountTokensInput(req))
		if !strings.Contains(got, "You are helpful") {
			t.Error("expected system text in output")
		}
		if !strings.Contains(got, "user: hi") {
			t.Error("expected message in output")
		}
	})
	t.Run("tools included", func(t *testing.T) {
		req := &anthropic.Request{
			Messages: []anthropic.Message{{Role: "user", Content: anthropic.MessageContent{Text: "hi"}}},
			Tools:    []anthropic.Tool{{Name: "bash", Description: "run commands"}},
		}
		got := string(buildCountTokensInput(req))
		if !strings.Contains(got, "bash") || !strings.Contains(got, "run commands") {
			t.Error("expected tool name and description in output")
		}
	})
	t.Run("empty request", func(t *testing.T) {
		req := &anthropic.Request{}
		got := buildCountTokensInput(req)
		if len(got) != 0 {
			t.Errorf("expected empty output for empty request, got %d bytes", len(got))
		}
	})
}

func TestHandleCountTokens_InvalidBody(t *testing.T) {
	svc := New(&mockTokenGetter{}, &mockClient{})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/messages/count_tokens", strings.NewReader("not json"))
	r.Header.Set("Content-Type", "application/json")
	svc.HandleCountTokens(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleCountTokens_EmptyMessages(t *testing.T) {
	svc := New(&mockTokenGetter{}, &mockClient{})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/messages/count_tokens", strings.NewReader(`{"model":"claude-sonnet-4-20250514","max_tokens":100,"messages":[]}`))
	r.Header.Set("Content-Type", "application/json")
	svc.HandleCountTokens(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleCountTokens_Success(t *testing.T) {
	svc := New(&mockTokenGetter{}, &mockClient{})
	w := httptest.NewRecorder()
	body := `{"model":"claude-sonnet-4-20250514","max_tokens":100,"messages":[{"role":"user","content":"hello world"}]}`
	r := httptest.NewRequest(http.MethodPost, "/v1/messages/count_tokens", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	svc.HandleCountTokens(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "input_tokens") {
		t.Errorf("expected input_tokens in response, got %s", w.Body.String())
	}
}

func TestParseAndValidateRequest_EmptyMessages(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"model":"x","max_tokens":1,"messages":[]}`))
	w := httptest.NewRecorder()
	_, err := parseAndValidateRequest(r.Context(), w, r)
	if err == nil {
		t.Fatal("expected error for empty messages")
	}
	if !strings.Contains(err.Error(), "messages must not be empty") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestParseAndValidateRequest_ValidBody(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"model":"claude-sonnet-4-20250514","max_tokens":100,"messages":[{"role":"user","content":"hi"}]}`))
	w := httptest.NewRecorder()
	req, err := parseAndValidateRequest(r.Context(), w, r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Model != "claude-sonnet-4-20250514" {
		t.Errorf("model = %q, want claude-sonnet-4-20250514", req.Model)
	}
}
