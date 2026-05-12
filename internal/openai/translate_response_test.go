package openai

import (
	"encoding/json/v2"
	"strings"
	"testing"
)

// buildAnthropicJSON serializes a raw response shape for the translator
// tests. Kept here so tests document the contract both ways (input shape,
// output shape).
func buildAnthropicJSON(t *testing.T, body string) []byte {
	t.Helper()
	// Parse and re-marshal to normalize (and to make sure the literal parses
	// — guards against hand-authored JSON typos in tests).
	var v any
	if err := json.Unmarshal([]byte(body), &v); err != nil {
		t.Fatalf("bad test fixture JSON: %v", err)
	}
	out, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("remarshal fixture: %v", err)
	}
	return out
}

func TestTranslateResponse_SimpleText(t *testing.T) {
	body := buildAnthropicJSON(t, `{
		"id": "msg_abc123xyz",
		"type": "message",
		"role": "assistant",
		"model": "claude-sonnet-4-6",
		"content": [
			{"type": "text", "text": "Hello, world!"}
		],
		"stop_reason": "end_turn",
		"usage": {"input_tokens": 12, "output_tokens": 5}
	}`)
	resp, err := TranslateResponse(body, "gpt-4o")
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	if resp.Object != ObjectChatCompletion {
		t.Errorf("object: %q", resp.Object)
	}
	if resp.ID != "chatcmpl-abc123xyz" {
		t.Errorf("id: %q", resp.ID)
	}
	if resp.Model != "gpt-4o" {
		t.Errorf("model should echo caller's alias, got %q", resp.Model)
	}
	if len(resp.Choices) != 1 {
		t.Fatalf("choices: %d", len(resp.Choices))
	}
	c := resp.Choices[0]
	if c.Message.Role != "assistant" {
		t.Errorf("role: %q", c.Message.Role)
	}
	if c.Message.Content.Text != "Hello, world!" {
		t.Errorf("text: %q", c.Message.Content.Text)
	}
	if c.FinishReason != FinishReasonStop {
		t.Errorf("finish_reason: %q", c.FinishReason)
	}
	if resp.Usage.PromptTokens != 12 || resp.Usage.CompletionTokens != 5 || resp.Usage.TotalTokens != 17 {
		t.Errorf("usage: %+v", resp.Usage)
	}
}

func TestTranslateResponse_MaxTokensFinish(t *testing.T) {
	body := buildAnthropicJSON(t, `{
		"id": "msg_x",
		"role": "assistant",
		"model": "claude-sonnet-4-6",
		"content": [{"type": "text", "text": "truncated"}],
		"stop_reason": "max_tokens",
		"usage": {"input_tokens": 1, "output_tokens": 1}
	}`)
	resp, err := TranslateResponse(body, "gpt-4o")
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	if resp.Choices[0].FinishReason != FinishReasonLength {
		t.Errorf("max_tokens should map to length, got %q", resp.Choices[0].FinishReason)
	}
}

func TestTranslateResponse_StopSequenceFinish(t *testing.T) {
	body := buildAnthropicJSON(t, `{
		"id": "msg_x",
		"role": "assistant",
		"model": "claude-sonnet-4-6",
		"content": [{"type": "text", "text": "halted"}],
		"stop_reason": "stop_sequence",
		"stop_sequence": "END",
		"usage": {"input_tokens": 1, "output_tokens": 1}
	}`)
	resp, err := TranslateResponse(body, "gpt-4o")
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	if resp.Choices[0].FinishReason != FinishReasonStop {
		t.Errorf("stop_sequence should map to stop, got %q", resp.Choices[0].FinishReason)
	}
}

func TestTranslateResponse_ToolUse(t *testing.T) {
	body := buildAnthropicJSON(t, `{
		"id": "msg_tool_1",
		"role": "assistant",
		"model": "claude-sonnet-4-6",
		"content": [
			{"type": "tool_use", "id": "toolu_01", "name": "get_weather", "input": {"city": "NYC"}}
		],
		"stop_reason": "tool_use",
		"usage": {"input_tokens": 20, "output_tokens": 10}
	}`)
	resp, err := TranslateResponse(body, "gpt-4o")
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	c := resp.Choices[0]
	if c.FinishReason != FinishReasonToolCalls {
		t.Errorf("finish_reason: %q", c.FinishReason)
	}
	if len(c.Message.ToolCalls) != 1 {
		t.Fatalf("tool_calls: %+v", c.Message.ToolCalls)
	}
	tc := c.Message.ToolCalls[0]
	if tc.ID != "toolu_01" || tc.Function.Name != "get_weather" || tc.Type != "function" {
		t.Errorf("tool_call shape: %+v", tc)
	}
	if !strings.Contains(tc.Function.Arguments, `"city"`) || !strings.Contains(tc.Function.Arguments, `"NYC"`) {
		t.Errorf("arguments: %q", tc.Function.Arguments)
	}
	// Content should be null when only tool_calls present.
	if !c.Message.Content.IsNull {
		t.Errorf("content should be null for tool-only response, got %+v", c.Message.Content)
	}
}

func TestTranslateResponse_MixedTextAndToolUse(t *testing.T) {
	body := buildAnthropicJSON(t, `{
		"id": "msg_x",
		"role": "assistant",
		"model": "claude-sonnet-4-6",
		"content": [
			{"type": "text", "text": "Let me check. "},
			{"type": "tool_use", "id": "toolu_01", "name": "f", "input": {}}
		],
		"stop_reason": "tool_use",
		"usage": {"input_tokens": 1, "output_tokens": 1}
	}`)
	resp, err := TranslateResponse(body, "gpt-4o")
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	c := resp.Choices[0]
	if c.Message.Content.Text != "Let me check. " {
		t.Errorf("text content: %q", c.Message.Content.Text)
	}
	if len(c.Message.ToolCalls) != 1 {
		t.Errorf("tool_calls: %+v", c.Message.ToolCalls)
	}
	if c.FinishReason != FinishReasonToolCalls {
		t.Errorf("finish_reason: %q", c.FinishReason)
	}
}

func TestTranslateResponse_ThinkingDropped(t *testing.T) {
	body := buildAnthropicJSON(t, `{
		"id": "msg_x",
		"role": "assistant",
		"model": "claude-sonnet-4-6",
		"content": [
			{"type": "thinking", "thinking": "internal musings"},
			{"type": "text", "text": "answer"}
		],
		"stop_reason": "end_turn",
		"usage": {"input_tokens": 1, "output_tokens": 1}
	}`)
	resp, err := TranslateResponse(body, "gpt-4o")
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	if resp.Choices[0].Message.Content.Text != "answer" {
		t.Errorf("thinking leaked: %q", resp.Choices[0].Message.Content.Text)
	}
}

func TestTranslateResponse_MultipleTextBlocksConcatenated(t *testing.T) {
	body := buildAnthropicJSON(t, `{
		"id": "msg_x",
		"role": "assistant",
		"model": "claude-sonnet-4-6",
		"content": [
			{"type": "text", "text": "part 1"},
			{"type": "text", "text": " part 2"}
		],
		"stop_reason": "end_turn",
		"usage": {"input_tokens": 1, "output_tokens": 1}
	}`)
	resp, err := TranslateResponse(body, "gpt-4o")
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	if resp.Choices[0].Message.Content.Text != "part 1 part 2" {
		t.Errorf("concatenation: %q", resp.Choices[0].Message.Content.Text)
	}
}

func TestTranslateResponse_MissingMessageIDGetsFallback(t *testing.T) {
	body := buildAnthropicJSON(t, `{
		"role": "assistant",
		"model": "claude-sonnet-4-6",
		"content": [{"type":"text","text":"hi"}],
		"stop_reason": "end_turn",
		"usage": {"input_tokens": 1, "output_tokens": 1}
	}`)
	resp, err := TranslateResponse(body, "gpt-4o")
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	if resp.ID != "chatcmpl-unknown" {
		t.Errorf("fallback id: %q", resp.ID)
	}
}

func TestTranslateResponse_InvalidJSON(t *testing.T) {
	_, err := TranslateResponse([]byte("{not json"), "gpt-4o")
	if err == nil {
		t.Fatal("expected error on invalid JSON")
	}
}

func TestMarshalArguments_NilInput(t *testing.T) {
	args, err := marshalArguments(nil)
	if err != nil {
		t.Fatal(err)
	}
	if args != "{}" {
		t.Errorf("nil input should give {}, got %q", args)
	}
}

func TestMarshalArguments_RoundTrip(t *testing.T) {
	input := map[string]any{"a": "b", "n": float64(1)}
	args, err := marshalArguments(input)
	if err != nil {
		t.Fatal(err)
	}
	var round map[string]any
	if err := json.Unmarshal([]byte(args), &round); err != nil {
		t.Fatal(err)
	}
	if round["a"] != "b" || round["n"] != float64(1) {
		t.Errorf("round trip: %+v", round)
	}
}

func TestChatCompletionID_StripsMsgPrefix(t *testing.T) {
	cases := map[string]string{
		"msg_abc": "chatcmpl-abc",
		"abc":     "chatcmpl-abc",
		"":        "chatcmpl-unknown",
	}
	for in, want := range cases {
		if got := chatCompletionID(in); got != want {
			t.Errorf("chatCompletionID(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTranslateResponse_OutputJSONShape(t *testing.T) {
	// Regression: ensure the JSON we produce has all the fields OpenAI SDKs
	// expect, including the fixed-value object, and the choices array not
	// missing index/finish_reason.
	body := buildAnthropicJSON(t, `{
		"id": "msg_x",
		"role": "assistant",
		"model": "claude-sonnet-4-6",
		"content": [{"type":"text","text":"hi"}],
		"stop_reason": "end_turn",
		"usage": {"input_tokens": 1, "output_tokens": 1}
	}`)
	resp, err := TranslateResponse(body, "gpt-4o")
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	out, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, want := range []string{
		`"object":"chat.completion"`,
		`"model":"gpt-4o"`,
		`"role":"assistant"`,
		`"finish_reason":"stop"`,
		`"prompt_tokens":1`,
		`"completion_tokens":1`,
		`"total_tokens":2`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("output missing %q\nfull: %s", want, s)
		}
	}
}
