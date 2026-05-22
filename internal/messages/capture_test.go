// This file is derived from github.com/d-kuro/kirocc
// Original commit: 5633c47f0d65aaef748728bae1c68160b0ea538d
// Copyright (c) 2026 d-kuro. Licensed under Apache License, Version 2.0.
// Modifications (c) 2026 kiroxy contributors.

package messages

import (
	"context"
	"net/http"
	"testing"

	"github.com/nopperabbo/kiroxy/internal/kiroproto"
)

func TestNewUpstreamAttemptCapture_DisabledReturnsNil(t *testing.T) {
	payload := &kiroproto.Payload{}
	got := newUpstreamAttemptCapture(context.Background(), false, payload, "model", false, true, 1)
	if got != nil {
		t.Errorf("expected nil when disabled, got %+v", got)
	}
}

func TestNewUpstreamAttemptCapture_EnabledReturnsNonNil(t *testing.T) {
	payload := &kiroproto.Payload{}
	got := newUpstreamAttemptCapture(context.Background(), true, payload, "model", false, true, 1)
	if got == nil {
		t.Fatal("expected non-nil capture when enabled")
	}
}

func TestCapture_ShortTraceID(t *testing.T) {
	c := &upstreamAttemptCapture{traceID: "abcdefgh-1234-5678-9012-abcdefghijkl"}
	got := c.shortTraceID()
	if got != "abcdefgh" {
		t.Errorf("shortTraceID() = %q, want %q", got, "abcdefgh")
	}
}

func TestCapture_SetResponseHeaders(t *testing.T) {
	t.Run("nil receiver", func(t *testing.T) {
		var c *upstreamAttemptCapture
		c.setResponseHeaders(http.Header{"X-Test": {"val"}}) // must not panic
	})
	t.Run("nil header", func(t *testing.T) {
		c := &upstreamAttemptCapture{}
		c.setResponseHeaders(nil)
		if c.responseHeader == nil {
			t.Fatal("expected non-nil empty header")
		}
	})
	t.Run("clones header", func(t *testing.T) {
		c := &upstreamAttemptCapture{}
		h := http.Header{"X-Foo": {"bar"}}
		c.setResponseHeaders(h)
		h.Set("X-Foo", "changed")
		if c.responseHeader.Get("X-Foo") != "bar" {
			t.Error("header was not cloned")
		}
	})
}

func TestCapture_RecordEvent(t *testing.T) {
	t.Run("nil receiver", func(t *testing.T) {
		var c *upstreamAttemptCapture
		c.recordEvent(kiroproto.Event{Type: "test"}) // must not panic
	})
	t.Run("assistant response with content", func(t *testing.T) {
		c := &upstreamAttemptCapture{toolUseNames: make(map[string]int)}
		c.recordEvent(kiroproto.Event{Type: kiroproto.EventAssistantResponse, Content: "hello"})
		if c.assistantResponseEvents != 1 {
			t.Errorf("assistantResponseEvents = %d, want 1", c.assistantResponseEvents)
		}
		if !c.hasVisibleText {
			t.Error("expected hasVisibleText = true")
		}
	})
	t.Run("assistant response without content", func(t *testing.T) {
		c := &upstreamAttemptCapture{toolUseNames: make(map[string]int)}
		c.recordEvent(kiroproto.Event{Type: kiroproto.EventAssistantResponse})
		if c.hasVisibleText {
			t.Error("expected hasVisibleText = false for empty content")
		}
	})
	t.Run("reasoning content", func(t *testing.T) {
		c := &upstreamAttemptCapture{toolUseNames: make(map[string]int)}
		c.recordEvent(kiroproto.Event{Type: kiroproto.EventReasoningContent})
		if c.reasoningEvents != 1 {
			t.Errorf("reasoningEvents = %d, want 1", c.reasoningEvents)
		}
	})
	t.Run("tool use real", func(t *testing.T) {
		c := &upstreamAttemptCapture{toolUseNames: make(map[string]int)}
		c.recordEvent(kiroproto.Event{Type: kiroproto.EventToolUse, ToolName: "bash"})
		if c.toolUseEventCount != 1 {
			t.Errorf("toolUseEventCount = %d, want 1", c.toolUseEventCount)
		}
		if !c.hasRealToolUse {
			t.Error("expected hasRealToolUse = true")
		}
		if c.toolUseNames["bash"] != 1 {
			t.Error("expected toolUseNames[bash] = 1")
		}
	})
	t.Run("tool use thinking tool", func(t *testing.T) {
		c := &upstreamAttemptCapture{toolUseNames: make(map[string]int)}
		c.recordEvent(kiroproto.Event{Type: kiroproto.EventToolUse, ToolName: kiroproto.ThinkingToolName})
		if c.hasRealToolUse {
			t.Error("thinking tool should not set hasRealToolUse")
		}
	})
	t.Run("event seq increments", func(t *testing.T) {
		c := &upstreamAttemptCapture{toolUseNames: make(map[string]int)}
		c.recordEvent(kiroproto.Event{Type: "a"})
		c.recordEvent(kiroproto.Event{Type: "b"})
		if len(c.events) != 2 {
			t.Fatalf("expected 2 events, got %d", len(c.events))
		}
		if c.events[0].Seq != 1 || c.events[1].Seq != 2 {
			t.Errorf("seq = %d, %d; want 1, 2", c.events[0].Seq, c.events[1].Seq)
		}
	})
}

func TestCapture_ToolUseNameList(t *testing.T) {
	t.Run("nil receiver", func(t *testing.T) {
		var c *upstreamAttemptCapture
		if got := c.toolUseNameList(); got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})
	t.Run("empty map", func(t *testing.T) {
		c := &upstreamAttemptCapture{toolUseNames: make(map[string]int)}
		if got := c.toolUseNameList(); got != nil {
			t.Errorf("expected nil for empty map, got %v", got)
		}
	})
	t.Run("sorted output", func(t *testing.T) {
		c := &upstreamAttemptCapture{toolUseNames: map[string]int{"zeta": 1, "alpha": 2, "beta": 1}}
		got := c.toolUseNameList()
		want := []string{"alpha", "beta", "zeta"}
		if len(got) != len(want) {
			t.Fatalf("len = %d, want %d", len(got), len(want))
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("got[%d] = %q, want %q", i, got[i], want[i])
			}
		}
	})
}

func TestCapture_LogAttrs(t *testing.T) {
	t.Run("nil receiver", func(t *testing.T) {
		var c *upstreamAttemptCapture
		if got := c.logAttrs(); got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})
	t.Run("returns expected keys", func(t *testing.T) {
		c := &upstreamAttemptCapture{
			assistantResponseEvents: 2,
			currentContentLen:       5,
			toolCount:               3,
			toolResultCount:         1,
			reasoningEvents:         1,
			toolUseEventCount:       2,
			toolUseNames:            map[string]int{"bash": 1},
			hasVisibleText:          true,
			hasRealToolUse:          true,
		}
		attrs := c.logAttrs()
		if len(attrs) == 0 {
			t.Fatal("expected non-empty attrs")
		}
		// Check key-value pairs (even indices are keys)
		keys := make(map[string]bool)
		for i := 0; i < len(attrs); i += 2 {
			keys[attrs[i].(string)] = true
		}
		for _, k := range []string{"assistant_response_events", "current_content_len", "tool_count", "tool_result_count", "reasoning_events", "tool_use_event_count", "tool_use_names", "has_visible_text", "has_real_tool_use"} {
			if !keys[k] {
				t.Errorf("missing key %q", k)
			}
		}
	})
}

func TestCapture_LogCapture(t *testing.T) {
	t.Run("nil receiver", func(t *testing.T) {
		var c *upstreamAttemptCapture
		c.logCapture(context.Background(), "test") // must not panic
	})
	t.Run("non-nil does not panic", func(t *testing.T) {
		c := &upstreamAttemptCapture{
			traceID:      "trace123",
			attempt:      1,
			model:        "model",
			toolUseNames: make(map[string]int),
		}
		c.logCapture(context.Background(), "test reason")
	})
}

func TestMarshalRaw(t *testing.T) {
	t.Run("valid value", func(t *testing.T) {
		got := marshalRaw(map[string]int{"a": 1})
		if string(got) != `{"a":1}` {
			t.Errorf("got %s, want {\"a\":1}", got)
		}
	})
	t.Run("unmarshalable value", func(t *testing.T) {
		got := marshalRaw(make(chan int))
		if len(got) == 0 {
			t.Error("expected non-empty fallback for unmarshalable value")
		}
	})
}
