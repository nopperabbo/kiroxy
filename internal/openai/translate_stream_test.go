package openai

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"net/http/httptest"
	"strings"
	"testing"
)

// recordingWriter captures written bytes in order while still behaving as an
// http.ResponseWriter + http.Flusher. Use httptest.NewRecorder instead where
// flush semantics matter for the test — it implements Flusher too.

// runStream feeds an Anthropic SSE body through the translator and returns
// the captured OpenAI output bytes.
func runStream(t *testing.T, anthropic string, echoModel string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	tr := NewStreamTranslator(rec, echoModel)
	if err := tr.Translate(context.Background(), strings.NewReader(anthropic)); err != nil {
		t.Fatalf("translate: %v", err)
	}
	return rec.Body.String()
}

// parseOpenAIChunks scans text/event-stream produced by the translator,
// returns the data: payloads in order. The "[DONE]" sentinel is represented
// by the literal string "[DONE]" in the slice.
func parseOpenAIChunks(t *testing.T, body string) []string {
	t.Helper()
	var out []string
	for _, frame := range bytes.Split([]byte(body), []byte("\n\n")) {
		line := bytes.TrimSpace(frame)
		if len(line) == 0 {
			continue
		}
		if !bytes.HasPrefix(line, []byte("data:")) {
			t.Fatalf("unexpected frame: %q", line)
		}
		data := strings.TrimSpace(string(line[len("data:"):]))
		out = append(out, data)
	}
	return out
}

func TestStreamTranslator_SimpleText(t *testing.T) {
	body := `event: message_start
data: {"type":"message_start","message":{"id":"msg_abc","role":"assistant","content":[],"model":"claude-sonnet-4-6","usage":{"input_tokens":10,"output_tokens":0}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":", world"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":3}}

event: message_stop
data: {"type":"message_stop"}

`
	out := runStream(t, body, "gpt-4o")
	chunks := parseOpenAIChunks(t, out)
	if len(chunks) < 4 {
		t.Fatalf("expected at least 4 chunks (role, delta, delta, final, [DONE]), got %d:\n%s", len(chunks), out)
	}
	// Last chunk must be [DONE].
	if chunks[len(chunks)-1] != "[DONE]" {
		t.Errorf("last chunk should be [DONE], got %q", chunks[len(chunks)-1])
	}

	// First real chunk has role:"assistant".
	var first ChatCompletionChunk
	if err := json.Unmarshal([]byte(chunks[0]), &first); err != nil {
		t.Fatalf("parse first chunk: %v", err)
	}
	if first.Object != ObjectChatCompletionChunk {
		t.Errorf("object: %q", first.Object)
	}
	if first.ID != "chatcmpl-abc" {
		t.Errorf("chatID: %q", first.ID)
	}
	if first.Model != "gpt-4o" {
		t.Errorf("echo model: %q", first.Model)
	}
	if first.Choices[0].Delta.Role != "assistant" {
		t.Errorf("first chunk should carry role:assistant, got %+v", first.Choices[0].Delta)
	}

	// Collect text content across chunks.
	var text strings.Builder
	var finish string
	for _, c := range chunks[:len(chunks)-1] {
		var chunk ChatCompletionChunk
		if err := json.Unmarshal([]byte(c), &chunk); err != nil {
			t.Fatalf("parse chunk: %v", err)
		}
		text.WriteString(chunk.Choices[0].Delta.Content)
		if chunk.Choices[0].FinishReason != nil {
			finish = *chunk.Choices[0].FinishReason
		}
	}
	if text.String() != "Hello, world" {
		t.Errorf("concat text: %q", text.String())
	}
	if finish != FinishReasonStop {
		t.Errorf("finish_reason: %q", finish)
	}
}

func TestStreamTranslator_ToolCall(t *testing.T) {
	body := `event: message_start
data: {"type":"message_start","message":{"id":"msg_tool","role":"assistant","content":[],"model":"claude-sonnet-4-6","usage":{"input_tokens":20,"output_tokens":0}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_01","name":"get_weather","input":{}}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"city\":"}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"\"NYC\"}"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"tool_use","stop_sequence":null},"usage":{"output_tokens":5}}

event: message_stop
data: {"type":"message_stop"}

`
	out := runStream(t, body, "gpt-4o")
	chunks := parseOpenAIChunks(t, out)
	if chunks[len(chunks)-1] != "[DONE]" {
		t.Fatalf("missing [DONE] terminator")
	}

	var headChunk ChatCompletionChunk
	// chunks: [role, tool_head, arg_delta1, arg_delta2, final]
	if err := json.Unmarshal([]byte(chunks[1]), &headChunk); err != nil {
		t.Fatalf("parse head: %v", err)
	}
	if len(headChunk.Choices[0].Delta.ToolCalls) != 1 {
		t.Fatalf("tool head chunk missing tool_calls: %+v", headChunk.Choices[0].Delta)
	}
	tc := headChunk.Choices[0].Delta.ToolCalls[0]
	if tc.ID != "toolu_01" || tc.Function.Name != "get_weather" || tc.Type != "function" {
		t.Errorf("head chunk shape: %+v", tc)
	}
	if tc.Index == nil || *tc.Index != 0 {
		t.Errorf("head chunk tool_call index: %v", tc.Index)
	}

	// Collect arguments across subsequent chunks.
	var args strings.Builder
	var finish string
	for _, c := range chunks[2 : len(chunks)-1] {
		var chunk ChatCompletionChunk
		if err := json.Unmarshal([]byte(c), &chunk); err != nil {
			t.Fatalf("parse chunk: %v", err)
		}
		for _, tc := range chunk.Choices[0].Delta.ToolCalls {
			args.WriteString(tc.Function.Arguments)
		}
		if chunk.Choices[0].FinishReason != nil {
			finish = *chunk.Choices[0].FinishReason
		}
	}
	if args.String() != `{"city":"NYC"}` {
		t.Errorf("reassembled arguments: %q", args.String())
	}
	if finish != FinishReasonToolCalls {
		t.Errorf("finish_reason: %q", finish)
	}
}

func TestStreamTranslator_ThinkingDropped(t *testing.T) {
	body := `event: message_start
data: {"type":"message_start","message":{"id":"msg_x","usage":{"input_tokens":1,"output_tokens":0}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"hidden"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: content_block_start
data: {"type":"content_block_start","index":1,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"answer"}}

event: content_block_stop
data: {"type":"content_block_stop","index":1}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":1}}

event: message_stop
data: {"type":"message_stop"}

`
	out := runStream(t, body, "gpt-4o")
	if strings.Contains(out, "hidden") {
		t.Error("thinking text leaked to output")
	}
	if !strings.Contains(out, "answer") {
		t.Errorf("visible text missing: %s", out)
	}
}

func TestStreamTranslator_UsageInFinalChunk(t *testing.T) {
	body := `event: message_start
data: {"type":"message_start","message":{"id":"msg_x","usage":{"input_tokens":42,"output_tokens":0}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hi"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":7}}

event: message_stop
data: {"type":"message_stop"}

`
	out := runStream(t, body, "gpt-4o")
	chunks := parseOpenAIChunks(t, out)
	// Final chunk is the one before [DONE]; it should carry usage.
	var final ChatCompletionChunk
	if err := json.Unmarshal([]byte(chunks[len(chunks)-2]), &final); err != nil {
		t.Fatal(err)
	}
	if final.Usage == nil {
		t.Fatal("final chunk missing usage")
	}
	if final.Usage.PromptTokens != 42 || final.Usage.CompletionTokens != 7 || final.Usage.TotalTokens != 49 {
		t.Errorf("usage: %+v", final.Usage)
	}
}

func TestStreamTranslator_DoneAppendedOnMissingMessageStop(t *testing.T) {
	// Stream cuts off without message_stop. We should still emit [DONE] on EOF.
	body := `event: message_start
data: {"type":"message_start","message":{"id":"msg_x"}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hi"}}

`
	out := runStream(t, body, "gpt-4o")
	if !strings.HasSuffix(strings.TrimSpace(out), "data: [DONE]") {
		t.Errorf("missing [DONE] terminator: %q", out)
	}
}

func TestParseSSEFrame_Basic(t *testing.T) {
	event, data := parseSSEFrame([]byte("event: foo\ndata: {\"x\":1}"))
	if event != "foo" {
		t.Errorf("event: %q", event)
	}
	if data != `{"x":1}` {
		t.Errorf("data: %q", data)
	}
}

func TestParseSSEFrame_IgnoresComments(t *testing.T) {
	event, data := parseSSEFrame([]byte(": this is a keep-alive\nevent: foo\ndata: x"))
	if event != "foo" || data != "x" {
		t.Errorf("event=%q data=%q", event, data)
	}
}

func TestSplitSSEFrames(t *testing.T) {
	input := "event: a\ndata: 1\n\nevent: b\ndata: 2\n\n"
	frames := collectFrames(t, input)
	if len(frames) != 2 {
		t.Fatalf("want 2 frames, got %d: %v", len(frames), frames)
	}
	if !strings.Contains(frames[0], "event: a") || !strings.Contains(frames[1], "event: b") {
		t.Errorf("frames: %v", frames)
	}
}

func collectFrames(t *testing.T, input string) []string {
	t.Helper()
	var frames []string
	data := []byte(input)
	offset := 0
	for offset < len(data) {
		advance, token, err := splitSSEFrames(data[offset:], false)
		if err != nil {
			t.Fatal(err)
		}
		if advance == 0 {
			// No more frames.
			break
		}
		frames = append(frames, string(token))
		offset += advance
	}
	return frames
}
