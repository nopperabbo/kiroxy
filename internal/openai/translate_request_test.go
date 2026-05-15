package openai

import (
	"encoding/json/v2"
	"strings"
	"testing"

	"github.com/nopperabbo/kiroxy/internal/anthropic"
)

// unmarshalRequest is a small helper for the tests: parse an OpenAI request
// from a JSON literal and return the value. Lets tests stay close to the
// wire format rather than constructing structs by hand.
func unmarshalRequest(t *testing.T, body string) *ChatCompletionRequest {
	t.Helper()
	var req ChatCompletionRequest
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}
	return &req
}

func TestTranslateRequest_SimpleUserMessage(t *testing.T) {
	in := unmarshalRequest(t, `{
		"model": "gpt-4o",
		"messages": [
			{"role": "user", "content": "hello"}
		]
	}`)
	out, err := TranslateRequest(in)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	if out.Model != "claude-sonnet-4-6" {
		t.Errorf("alias gpt-4o should map to claude-sonnet-4-6, got %q", out.Model)
	}
	if len(out.Messages) != 1 || out.Messages[0].Role != "user" {
		t.Fatalf("wrong messages: %+v", out.Messages)
	}
	if out.Messages[0].Content.Text != "hello" {
		t.Errorf("content mismatch: got %q", out.Messages[0].Content.Text)
	}
	if out.MaxTokens != defaultMaxTokens {
		t.Errorf("max_tokens should default to %d, got %d", defaultMaxTokens, out.MaxTokens)
	}
}

func TestTranslateRequest_SystemMessagesJoined(t *testing.T) {
	in := unmarshalRequest(t, `{
		"model": "claude-sonnet-4-6",
		"max_tokens": 100,
		"messages": [
			{"role": "system", "content": "You are helpful."},
			{"role": "system", "content": "Be concise."},
			{"role": "user", "content": "hi"}
		]
	}`)
	out, err := TranslateRequest(in)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	if out.System.Text != "You are helpful.\n\nBe concise." {
		t.Errorf("system join mismatch: %q", out.System.Text)
	}
	if out.MaxTokens != 100 {
		t.Errorf("max_tokens not honored: got %d", out.MaxTokens)
	}
	if len(out.Messages) != 1 {
		t.Errorf("system should be stripped from messages, got %d messages", len(out.Messages))
	}
}

func TestTranslateRequest_SystemAfterUserRejected(t *testing.T) {
	in := unmarshalRequest(t, `{
		"model": "gpt-4o",
		"messages": [
			{"role": "user", "content": "hi"},
			{"role": "system", "content": "oops"}
		]
	}`)
	_, err := TranslateRequest(in)
	if err == nil {
		t.Fatal("expected error for mid-list system message")
	}
	if ve, ok := AsValidationError(err); !ok || !strings.Contains(ve.Message, "system messages must appear at the start") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTranslateRequest_ToolChoiceNoneStripsTools(t *testing.T) {
	in := unmarshalRequest(t, `{
		"model": "gpt-4o",
		"messages": [{"role": "user", "content": "hi"}],
		"tools": [
			{"type": "function", "function": {"name": "get_weather", "description": "get the weather"}}
		],
		"tool_choice": "none"
	}`)
	out, err := TranslateRequest(in)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	if len(out.Tools) != 0 {
		t.Errorf("tool_choice=none should strip tools, got %d tools", len(out.Tools))
	}
}

func TestTranslateRequest_ToolChoiceSpecificFunctionRejected(t *testing.T) {
	in := unmarshalRequest(t, `{
		"model": "gpt-4o",
		"messages": [{"role": "user", "content": "hi"}],
		"tools": [{"type":"function","function":{"name":"f"}}],
		"tool_choice": {"type": "function", "function": {"name": "f"}}
	}`)
	_, err := TranslateRequest(in)
	if err == nil {
		t.Fatal("expected error for specific function tool_choice")
	}
	ve, ok := AsValidationError(err)
	if !ok || ve.Param != "tool_choice" {
		t.Errorf("expected validation error on tool_choice: %v", err)
	}
}

func TestTranslateRequest_ToolsTranslated(t *testing.T) {
	in := unmarshalRequest(t, `{
		"model": "claude-sonnet-4-6",
		"messages": [{"role": "user", "content": "weather?"}],
		"tools": [
			{"type": "function", "function": {
				"name": "get_weather",
				"description": "Get weather for city",
				"parameters": {"type": "object", "properties": {"city": {"type": "string"}}}
			}}
		]
	}`)
	out, err := TranslateRequest(in)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	if len(out.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(out.Tools))
	}
	tool := out.Tools[0]
	if tool.Name != "get_weather" {
		t.Errorf("tool name: got %q", tool.Name)
	}
	if tool.Description != "Get weather for city" {
		t.Errorf("description: got %q", tool.Description)
	}
	if tool.InputSchema["type"] != "object" {
		t.Errorf("input_schema not translated: %+v", tool.InputSchema)
	}
}

func TestTranslateRequest_ToolCallAndToolResult(t *testing.T) {
	in := unmarshalRequest(t, `{
		"model": "gpt-4o",
		"messages": [
			{"role": "user", "content": "weather in NYC?"},
			{"role": "assistant", "content": null, "tool_calls": [
				{"id": "call_abc", "type": "function", "function": {"name": "get_weather", "arguments": "{\"city\":\"NYC\"}"}}
			]},
			{"role": "tool", "tool_call_id": "call_abc", "content": "72F sunny"},
			{"role": "user", "content": "thanks"}
		]
	}`)
	out, err := TranslateRequest(in)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	if len(out.Messages) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(out.Messages))
	}
	// Assistant message with tool_use block.
	asst := out.Messages[1]
	if asst.Role != "assistant" || asst.Content.IsString() {
		t.Fatalf("assistant message shape: %+v", asst)
	}
	if len(asst.Content.Blocks) != 1 || asst.Content.Blocks[0].Type != anthropic.BlockTypeToolUse {
		t.Fatalf("expected tool_use block: %+v", asst.Content.Blocks)
	}
	tu := asst.Content.Blocks[0]
	if tu.ID != "call_abc" || tu.Name != "get_weather" {
		t.Errorf("tool_use fields: %+v", tu)
	}
	if tu.Input["city"] != "NYC" {
		t.Errorf("tool_use input not parsed: %+v", tu.Input)
	}
	// Tool role → user with tool_result block.
	tr := out.Messages[2]
	if tr.Role != "user" {
		t.Errorf("tool role should become user, got %q", tr.Role)
	}
	if len(tr.Content.Blocks) != 1 || tr.Content.Blocks[0].Type != anthropic.BlockTypeToolResult {
		t.Fatalf("expected tool_result block: %+v", tr.Content.Blocks)
	}
	if tr.Content.Blocks[0].ToolUseID != "call_abc" {
		t.Errorf("tool_use_id mismatch: %q", tr.Content.Blocks[0].ToolUseID)
	}
	if tr.Content.Blocks[0].Content.Text != "72F sunny" {
		t.Errorf("tool_result text: %q", tr.Content.Blocks[0].Content.Text)
	}
}

func TestTranslateRequest_ConsecutiveToolResultsCoalesced(t *testing.T) {
	in := unmarshalRequest(t, `{
		"model": "gpt-4o",
		"messages": [
			{"role": "user", "content": "?"},
			{"role": "assistant", "content": null, "tool_calls": [
				{"id":"a","type":"function","function":{"name":"f","arguments":"{}"}},
				{"id":"b","type":"function","function":{"name":"f","arguments":"{}"}}
			]},
			{"role": "tool", "tool_call_id": "a", "content": "r1"},
			{"role": "tool", "tool_call_id": "b", "content": "r2"}
		]
	}`)
	out, err := TranslateRequest(in)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	// Should be user, assistant, single user (coalesced tool results).
	if len(out.Messages) != 3 {
		t.Fatalf("expected tool results coalesced into 1 user msg, got %d messages", len(out.Messages))
	}
	last := out.Messages[2]
	if last.Role != "user" || len(last.Content.Blocks) != 2 {
		t.Fatalf("coalesced tool_result blocks: %+v", last)
	}
}

func TestTranslateRequest_ImageDataURI(t *testing.T) {
	// 1x1 transparent PNG base64 ("iVBORw0...").
	in := unmarshalRequest(t, `{
		"model": "gpt-4o",
		"messages": [{
			"role": "user",
			"content": [
				{"type": "text", "text": "describe this"},
				{"type": "image_url", "image_url": {"url": "data:image/png;base64,iVBORw0KGgo="}}
			]
		}]
	}`)
	out, err := TranslateRequest(in)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	blocks := out.Messages[0].Content.Blocks
	if len(blocks) != 2 {
		t.Fatalf("want 2 blocks, got %d", len(blocks))
	}
	img := blocks[1]
	if img.Type != anthropic.BlockTypeImage || img.Source == nil {
		t.Fatalf("image block: %+v", img)
	}
	if img.Source.MediaType != "image/png" {
		t.Errorf("media type: %q", img.Source.MediaType)
	}
	if img.Source.Data != "iVBORw0KGgo=" {
		t.Errorf("data: %q", img.Source.Data)
	}
}

func TestTranslateRequest_HTTPSImageRejected(t *testing.T) {
	in := unmarshalRequest(t, `{
		"model": "gpt-4o",
		"messages": [{
			"role": "user",
			"content": [
				{"type": "image_url", "image_url": {"url": "https://example.com/cat.png"}}
			]
		}]
	}`)
	_, err := TranslateRequest(in)
	if err == nil {
		t.Fatal("expected error for https image URL")
	}
	ve, ok := AsValidationError(err)
	if !ok || !strings.Contains(ve.Message, "data: URIs") {
		t.Errorf("wrong error: %v", err)
	}
}

func TestTranslateRequest_StopString(t *testing.T) {
	in := unmarshalRequest(t, `{
		"model": "gpt-4o",
		"messages": [{"role": "user", "content": "hi"}],
		"stop": "END"
	}`)
	out, err := TranslateRequest(in)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	if len(out.StopSequences) != 1 || out.StopSequences[0] != "END" {
		t.Errorf("stop string: %+v", out.StopSequences)
	}
}

func TestTranslateRequest_StopArray(t *testing.T) {
	in := unmarshalRequest(t, `{
		"model": "gpt-4o",
		"messages": [{"role": "user", "content": "hi"}],
		"stop": ["A", "B"]
	}`)
	out, err := TranslateRequest(in)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	if len(out.StopSequences) != 2 || out.StopSequences[0] != "A" || out.StopSequences[1] != "B" {
		t.Errorf("stop array: %+v", out.StopSequences)
	}
}

func TestTranslateRequest_NGreaterThanOneRejected(t *testing.T) {
	n := 2
	in := &ChatCompletionRequest{
		Model:    "gpt-4o",
		Messages: []ChatMessage{{Role: "user", Content: MessageContent{Text: "hi"}}},
		N:        &n,
	}
	_, err := TranslateRequest(in)
	if err == nil {
		t.Fatal("expected error for n > 1")
	}
	if ve, _ := AsValidationError(err); ve == nil || ve.Param != "n" {
		t.Errorf("wrong error: %v", err)
	}
}

func TestTranslateRequest_EmptyMessagesRejected(t *testing.T) {
	in := &ChatCompletionRequest{Model: "gpt-4o"}
	_, err := TranslateRequest(in)
	if err == nil {
		t.Fatal("expected error for empty messages")
	}
}

func TestTranslateRequest_MissingModelRejected(t *testing.T) {
	in := &ChatCompletionRequest{Messages: []ChatMessage{{Role: "user", Content: MessageContent{Text: "hi"}}}}
	_, err := TranslateRequest(in)
	if err == nil {
		t.Fatal("expected error for missing model")
	}
}

func TestTranslateRequest_MaxCompletionTokensAlias(t *testing.T) {
	mt := 42
	in := &ChatCompletionRequest{
		Model:           "gpt-4o",
		Messages:        []ChatMessage{{Role: "user", Content: MessageContent{Text: "hi"}}},
		MaxOutputTokens: &mt,
	}
	out, err := TranslateRequest(in)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	if out.MaxTokens != 42 {
		t.Errorf("max_completion_tokens not honored: %d", out.MaxTokens)
	}
}

func TestTranslateRequest_StreamFlag(t *testing.T) {
	in := unmarshalRequest(t, `{
		"model": "gpt-4o",
		"messages": [{"role": "user", "content": "hi"}],
		"stream": true
	}`)
	out, err := TranslateRequest(in)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	if !out.Stream {
		t.Error("stream flag not propagated")
	}
}

func TestTranslateRequest_MultipartUserTextParts(t *testing.T) {
	in := unmarshalRequest(t, `{
		"model": "gpt-4o",
		"messages": [{
			"role": "user",
			"content": [
				{"type": "text", "text": "line 1"},
				{"type": "text", "text": "line 2"}
			]
		}]
	}`)
	out, err := TranslateRequest(in)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	blocks := out.Messages[0].Content.Blocks
	if len(blocks) != 2 {
		t.Fatalf("want 2 text blocks, got %d", len(blocks))
	}
	if blocks[0].Text != "line 1" || blocks[1].Text != "line 2" {
		t.Errorf("text blocks: %+v", blocks)
	}
}

func TestResolveModel_Aliases(t *testing.T) {
	cases := []struct{ in, want string }{
		{"gpt-4o", "claude-sonnet-4-6"},
		{"gpt-4o-mini", "claude-sonnet-4-6"},
		{"gpt-4-turbo", "claude-opus-4-7"},
		{"gpt-4", "claude-opus-4-7"},
		{"gpt-3.5-turbo", "claude-haiku-4.5"},
		{"openai/gpt-4o", "claude-sonnet-4-6"},
		{"openai/claude-sonnet-4-6", "claude-sonnet-4-6"},
		{"claude-sonnet-4-6", "claude-sonnet-4-6"},
		{"claude-opus-4-7", "claude-opus-4-7"},
		{"totally-unknown", "totally-unknown"}, // pass-through
		{"", ""},
	}
	for _, tc := range cases {
		got := ResolveModel(tc.in)
		if got != tc.want {
			t.Errorf("ResolveModel(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
