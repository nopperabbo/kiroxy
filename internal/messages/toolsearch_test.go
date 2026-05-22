// This file is derived from github.com/d-kuro/kirocc
// Original commit: 5633c47f0d65aaef748728bae1c68160b0ea538d
// Copyright (c) 2026 d-kuro. Licensed under Apache License, Version 2.0.
// Modifications (c) 2026 kiroxy contributors.

package messages

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nopperabbo/kiroxy/internal/anthropic"
	"github.com/nopperabbo/kiroxy/internal/auth"
	"github.com/nopperabbo/kiroxy/internal/reqconv"
	"github.com/nopperabbo/kiroxy/internal/respconv"
	"github.com/nopperabbo/kiroxy/internal/testutil"
	"github.com/nopperabbo/kiroxy/internal/toolsearch"
)

func TestParseToolSearchInput(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantQuery   string
		wantMax     int
		wantErr     bool
		errContains string
	}{
		{
			name:      "valid with max_results",
			input:     `{"query":"foo","max_results":5}`,
			wantQuery: "foo",
			wantMax:   5,
		},
		{
			name:      "valid without max_results",
			input:     `{"query":"bar"}`,
			wantQuery: "bar",
			wantMax:   0,
		},
		{
			name:        "invalid JSON",
			input:       `{broken`,
			wantErr:     true,
			errContains: "parse",
		},
		{
			name:        "empty string",
			input:       ``,
			wantErr:     true,
			errContains: "parse",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, maxResults, err := parseToolSearchInput(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (query=%q, max=%d)", query, maxResults)
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if query != tt.wantQuery {
				t.Errorf("query: got %q, want %q", query, tt.wantQuery)
			}
			if maxResults != tt.wantMax {
				t.Errorf("maxResults: got %d, want %d", maxResults, tt.wantMax)
			}
		})
	}
}

func TestBuildSearchInput(t *testing.T) {
	t.Run("with max_results", func(t *testing.T) {
		got := buildSearchInput("find tools", 5)
		if got["query"] != "find tools" {
			t.Errorf("query = %v", got["query"])
		}
		if got["max_results"] != 5 {
			t.Errorf("max_results = %v", got["max_results"])
		}
	})
	t.Run("without max_results", func(t *testing.T) {
		got := buildSearchInput("find tools", 0)
		if _, ok := got["max_results"]; ok {
			t.Error("expected no max_results key when 0")
		}
	})
}

func TestAppendSearchMessages_Success(t *testing.T) {
	o := &toolSearchOrchestrator{req: &anthropic.Request{}}
	nameMap := reqconv.NewToolNameMap()
	msgs := o.appendSearchMessages(nil, "tool-use-123", map[string]any{"query": "test"}, []string{"bash", "read"}, nil, nameMap)
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Role != "assistant" {
		t.Errorf("first message role = %q", msgs[0].Role)
	}
	if msgs[1].Role != "user" {
		t.Errorf("second message role = %q", msgs[1].Role)
	}
}

func TestAppendSearchMessages_Error(t *testing.T) {
	o := &toolSearchOrchestrator{req: &anthropic.Request{}}
	nameMap := reqconv.NewToolNameMap()
	msgs := o.appendSearchMessages(nil, "tool-use-123", map[string]any{"query": "test"}, nil, errors.New("search failed"), nameMap)
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if !msgs[1].Content.Blocks[0].IsError {
		t.Error("expected IsError = true on tool result")
	}
}

func TestWriteStreamingOrJSONError_NotPromoted(t *testing.T) {
	w := httptest.NewRecorder()
	gw := NewGateWriter(w)
	_, _ = gw.Write([]byte("buffered"))

	sw := respconv.NewSSEWriter(context.Background(), gw, "model", 200000, nil, 4096, 0)
	writeStreamingOrJSONError(gw, sw, w, http.StatusBadGateway, "api_error", "upstream error")
	if w.Code != http.StatusBadGateway {
		t.Errorf("expected 502, got %d", w.Code)
	}
}

func TestToolSearchOrchestrator_Run_Streaming_ClientError(t *testing.T) {
	svc := New(&mockTokenGetter{}, &mockClient{err: errors.New("connection refused")})
	o := &toolSearchOrchestrator{
		service:           svc,
		tsCtx:             &toolsearch.Context{ActiveTools: []anthropic.Tool{{Name: "bash"}}},
		req:               &anthropic.Request{Stream: true, Model: "claude-sonnet-4-20250514", MaxTokens: 4096, Messages: []anthropic.Message{{Role: "user", Content: anthropic.MessageContent{Text: "hi"}}}},
		creds:             &auth.Credentials{AccessToken: "tok", Region: "us-east-1", ProfileARN: "arn:aws:iam::123:profile/test"},
		buildOpts:         reqconv.BuildOptions{ProfileARN: "arn:aws:iam::123:profile/test", ModelID: "claude-sonnet-4-20250514", ConversationID: "test"},
		contextWindowSize: 200000,
		responseModel:     "claude-sonnet-4-20250514",
	}
	w := httptest.NewRecorder()
	reason := o.run(context.Background(), w)
	if reason != "" {
		t.Errorf("expected empty reason, got %q", reason)
	}
	if w.Code != http.StatusBadGateway {
		t.Errorf("expected 502, got %d", w.Code)
	}
}

func TestToolSearchOrchestrator_Run_NonStreaming_ClientError(t *testing.T) {
	svc := New(&mockTokenGetter{}, &mockClient{err: errors.New("connection refused")})
	o := &toolSearchOrchestrator{
		service:           svc,
		tsCtx:             &toolsearch.Context{ActiveTools: []anthropic.Tool{{Name: "bash"}}},
		req:               &anthropic.Request{Stream: false, Model: "claude-sonnet-4-20250514", MaxTokens: 4096, Messages: []anthropic.Message{{Role: "user", Content: anthropic.MessageContent{Text: "hi"}}}},
		creds:             &auth.Credentials{AccessToken: "tok", Region: "us-east-1", ProfileARN: "arn:aws:iam::123:profile/test"},
		buildOpts:         reqconv.BuildOptions{ProfileARN: "arn:aws:iam::123:profile/test", ModelID: "claude-sonnet-4-20250514", ConversationID: "test"},
		contextWindowSize: 200000,
		responseModel:     "claude-sonnet-4-20250514",
	}
	w := httptest.NewRecorder()
	reason := o.run(context.Background(), w)
	if reason != "" {
		t.Errorf("expected empty reason, got %q", reason)
	}
	if w.Code != http.StatusBadGateway {
		t.Errorf("expected 502, got %d", w.Code)
	}
}

func TestToolSearchOrchestrator_Run_Streaming_Success(t *testing.T) {
	var buf bytes.Buffer
	buf.Write(testutil.BuildFrame("assistantResponseEvent", []byte(`{"content":"Hello"}`)))
	buf.Write(testutil.BuildFrame("meteringEvent", []byte(`{"usage":0.1,"inputTokens":50,"outputTokens":20}`)))

	svc := New(&mockTokenGetter{}, &stubKC{body: buf.Bytes()})
	o := &toolSearchOrchestrator{
		service:           svc,
		tsCtx:             &toolsearch.Context{ActiveTools: []anthropic.Tool{{Name: "bash"}}},
		req:               &anthropic.Request{Stream: true, Model: "claude-sonnet-4-20250514", MaxTokens: 4096, Messages: []anthropic.Message{{Role: "user", Content: anthropic.MessageContent{Text: "hi"}}}},
		creds:             &auth.Credentials{AccessToken: "tok", Region: "us-east-1", ProfileARN: "arn:aws:iam::123:profile/test"},
		buildOpts:         reqconv.BuildOptions{ProfileARN: "arn:aws:iam::123:profile/test", ModelID: "claude-sonnet-4-20250514", ConversationID: "test"},
		contextWindowSize: 200000,
		responseModel:     "claude-sonnet-4-20250514",
	}
	w := httptest.NewRecorder()
	reason := o.run(context.Background(), w)
	if reason != "" {
		t.Errorf("expected empty reason, got %q", reason)
	}
}

func TestToolSearchOrchestrator_Run_NonStreaming_Success(t *testing.T) {
	var buf bytes.Buffer
	buf.Write(testutil.BuildFrame("assistantResponseEvent", []byte(`{"content":"Hello"}`)))
	buf.Write(testutil.BuildFrame("meteringEvent", []byte(`{"usage":0.1,"inputTokens":50,"outputTokens":20}`)))

	svc := New(&mockTokenGetter{}, &stubKC{body: buf.Bytes()})
	o := &toolSearchOrchestrator{
		service:           svc,
		tsCtx:             &toolsearch.Context{ActiveTools: []anthropic.Tool{{Name: "bash"}}},
		req:               &anthropic.Request{Stream: false, Model: "claude-sonnet-4-20250514", MaxTokens: 4096, Messages: []anthropic.Message{{Role: "user", Content: anthropic.MessageContent{Text: "hi"}}}},
		creds:             &auth.Credentials{AccessToken: "tok", Region: "us-east-1", ProfileARN: "arn:aws:iam::123:profile/test"},
		buildOpts:         reqconv.BuildOptions{ProfileARN: "arn:aws:iam::123:profile/test", ModelID: "claude-sonnet-4-20250514", ConversationID: "test"},
		contextWindowSize: 200000,
		responseModel:     "claude-sonnet-4-20250514",
	}
	w := httptest.NewRecorder()
	reason := o.run(context.Background(), w)
	if reason != "" {
		t.Errorf("expected empty reason, got %q", reason)
	}
}
