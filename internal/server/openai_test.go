package server

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"local/kiroxy/internal/auth"
	"local/kiroxy/internal/kiroclient"
	"local/kiroxy/internal/kiroproto"
)

// stubKiroClientMulti lets tests control the response body per-call.
type stubKiroClientMulti struct {
	body []byte
}

func (c *stubKiroClientMulti) GenerateAssistantResponse(_ context.Context, _ string, _ *kiroproto.Payload, _ string) (*kiroclient.Response, error) {
	return &kiroclient.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(c.body)),
		Header:     http.Header{"Content-Type": []string{"application/vnd.amazon.eventstream"}},
	}, nil
}

func newOpenAIServer(t *testing.T, body []byte) *httptest.Server {
	t.Helper()
	srv := New(Options{
		Version:    "test",
		Auth:       &stubTokenGetter{creds: &auth.Credentials{AccessToken: "tok", Region: "us-east-1"}},
		KiroClient: &stubKiroClientMulti{body: body},
	})
	return httptest.NewServer(srv.Handler())
}

func TestHandleListModels_Works(t *testing.T) {
	srv := httptest.NewServer(New(Options{Version: "test"}).Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/v1/models")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var list struct {
		Object string `json:"object"`
		Data   []struct {
			ID      string `json:"id"`
			Object  string `json:"object"`
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
	}
	if err := json.UnmarshalRead(resp.Body, &list); err != nil {
		t.Fatal(err)
	}
	if list.Object != "list" {
		t.Errorf("object: %q", list.Object)
	}
	if len(list.Data) == 0 {
		t.Error("empty models")
	}
	gotGPT4o := false
	for _, m := range list.Data {
		if m.ID == "gpt-4o" {
			gotGPT4o = true
		}
	}
	if !gotGPT4o {
		t.Error("gpt-4o alias missing from /v1/models")
	}
}

func TestHandleChatCompletions_NoAuth503(t *testing.T) {
	srv := httptest.NewServer(New(Options{Version: "test"}).Handler()) // no Auth
	defer srv.Close()

	body := `{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`
	resp, err := http.Post(srv.URL+"/v1/chat/completions", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d", resp.StatusCode)
	}
	b, _ := io.ReadAll(resp.Body)
	if !bytes.Contains(b, []byte(`"error"`)) {
		t.Errorf("response should be OpenAI error shape: %s", b)
	}
	if !bytes.Contains(b, []byte(`"authentication_error"`)) {
		t.Errorf("response should mention authentication_error: %s", b)
	}
}

func TestHandleChatCompletions_BadJSON(t *testing.T) {
	srv := newOpenAIServer(t, nil)
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/v1/chat/completions", "application/json", strings.NewReader("{not json"))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("want 400, got %d", resp.StatusCode)
	}
	b, _ := io.ReadAll(resp.Body)
	if !bytes.Contains(b, []byte(`"invalid_request_error"`)) {
		t.Errorf("error envelope: %s", b)
	}
}

func TestHandleChatCompletions_EmptyMessages400(t *testing.T) {
	srv := newOpenAIServer(t, nil)
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/v1/chat/completions", "application/json", strings.NewReader(`{"model":"gpt-4o","messages":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("want 400, got %d", resp.StatusCode)
	}
}

func TestHandleChatCompletions_NonStreamingEndToEnd(t *testing.T) {
	srv := newOpenAIServer(t, buildSingleShotEventStream(t, "hello from kiro"))
	defer srv.Close()

	body := `{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`
	req, _ := http.NewRequest("POST", srv.URL+"/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("want 200, got %d: %s", resp.StatusCode, b)
	}
	var out struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Model   string `json:"model"`
		Choices []struct {
			Index   int `json:"index"`
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.UnmarshalRead(resp.Body, &out); err != nil {
		t.Fatal(err)
	}
	if out.Object != "chat.completion" {
		t.Errorf("object: %q", out.Object)
	}
	if !strings.HasPrefix(out.ID, "chatcmpl-") {
		t.Errorf("id: %q", out.ID)
	}
	if out.Model != "gpt-4o" {
		t.Errorf("response should echo gpt-4o alias, got %q", out.Model)
	}
	if len(out.Choices) != 1 {
		t.Fatalf("choices: %d", len(out.Choices))
	}
	if out.Choices[0].Message.Role != "assistant" {
		t.Errorf("role: %q", out.Choices[0].Message.Role)
	}
	if out.Choices[0].Message.Content != "hello from kiro" {
		t.Errorf("content: %q", out.Choices[0].Message.Content)
	}
	if out.Choices[0].FinishReason != "stop" {
		t.Errorf("finish_reason: %q", out.Choices[0].FinishReason)
	}
}

func TestHandleChatCompletions_StreamingEndToEnd(t *testing.T) {
	srv := newOpenAIServer(t, buildSingleShotEventStream(t, "stream hi"))
	defer srv.Close()

	body := `{"model":"gpt-4o","stream":true,"messages":[{"role":"user","content":"hi"}]}`
	req, _ := http.NewRequest("POST", srv.URL+"/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("want 200, got %d: %s", resp.StatusCode, b)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/event-stream") {
		t.Errorf("expected SSE content type, got %q", ct)
	}

	all, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	s := string(all)
	if !strings.HasSuffix(strings.TrimSpace(s), "data: [DONE]") {
		t.Errorf("stream should end with [DONE]: %q", s)
	}
	// Ensure at least one content delta with our fixture text made it through.
	if !strings.Contains(s, "stream hi") {
		t.Errorf("text delta missing: %s", s)
	}
	// Every non-DONE chunk must be an OpenAI chat.completion.chunk.
	for _, frame := range bytes.Split(all, []byte("\n\n")) {
		line := bytes.TrimSpace(frame)
		if len(line) == 0 {
			continue
		}
		if bytes.Equal(line, []byte("data: [DONE]")) {
			continue
		}
		if !bytes.HasPrefix(line, []byte("data: ")) {
			t.Errorf("malformed SSE frame: %q", line)
			continue
		}
		var chunk struct {
			Object string `json:"object"`
			Model  string `json:"model"`
		}
		if err := json.Unmarshal(line[len("data: "):], &chunk); err != nil {
			t.Errorf("chunk json: %v — %q", err, line)
			continue
		}
		if chunk.Object != "chat.completion.chunk" {
			t.Errorf("wrong object: %q (line: %q)", chunk.Object, line)
		}
		if chunk.Model != "gpt-4o" {
			t.Errorf("wrong model echo: %q", chunk.Model)
		}
	}
}

func TestHandleChatCompletions_ModelAliasRoutes(t *testing.T) {
	// gpt-4o should resolve to claude-sonnet-4-6 inside the pipeline. We can't
	// observe the resolved ID directly but we can assert the response comes
	// back OK and the translation doesn't error out.
	srv := newOpenAIServer(t, buildSingleShotEventStream(t, "ok"))
	defer srv.Close()

	for _, alias := range []string{"gpt-4o", "gpt-4-turbo", "gpt-3.5-turbo", "openai/gpt-4o"} {
		body := `{"model":"` + alias + `","messages":[{"role":"user","content":"hi"}]}`
		resp, err := http.Post(srv.URL+"/v1/chat/completions", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatalf("%s: %v", alias, err)
		}
		if resp.StatusCode != 200 {
			b, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			t.Errorf("%s: got %d: %s", alias, resp.StatusCode, b)
			continue
		}
		var out struct {
			Model string `json:"model"`
		}
		if err := json.UnmarshalRead(resp.Body, &out); err != nil {
			resp.Body.Close()
			t.Errorf("%s decode: %v", alias, err)
			continue
		}
		resp.Body.Close()
		if out.Model != alias {
			t.Errorf("%s: response should echo same alias, got %q", alias, out.Model)
		}
	}
}

func TestHandleChatCompletions_MethodNotAllowed(t *testing.T) {
	srv := newOpenAIServer(t, nil)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/v1/chat/completions")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("want 405, got %d", resp.StatusCode)
	}
}

func TestHandleListModels_MethodNotAllowed(t *testing.T) {
	srv := newOpenAIServer(t, nil)
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/v1/models", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("want 405, got %d", resp.StatusCode)
	}
}

func TestDeriveAnthropicErrorMessage(t *testing.T) {
	cases := []struct {
		body string
		want string
	}{
		{`{"type":"error","error":{"type":"invalid_request_error","message":"oops"}}`, "oops"},
		{``, "upstream error"},
		{`unstructured`, "unstructured"},
		{`{}`, "{}"},
	}
	for _, tc := range cases {
		got := deriveAnthropicErrorMessage([]byte(tc.body))
		if got != tc.want {
			t.Errorf("deriveAnthropicErrorMessage(%q) = %q, want %q", tc.body, got, tc.want)
		}
	}
}
