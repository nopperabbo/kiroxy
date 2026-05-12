// Streaming translation: parse an Anthropic SSE byte stream and emit OpenAI
// chat.completion.chunk frames plus a terminating data: [DONE].
//
// Anthropic's SSE shape (from respconv/streaming.go):
//   event: message_start           data: {message: {id, role, model, usage}}
//   event: content_block_start     data: {index, content_block: {type, ...}}
//   event: content_block_delta     data: {index, delta: {type: text_delta | thinking_delta | input_json_delta | ..., ...}}
//   event: content_block_stop      data: {index}
//   event: message_delta           data: {delta: {stop_reason, stop_sequence}, usage: {...}}
//   event: message_stop            data: {}
//   event: error                   data: {error: {type, message}}
//
// OpenAI's SSE shape:
//   data: {"id":"chatcmpl-...","object":"chat.completion.chunk",...,"choices":[{"index":0,"delta":{...},"finish_reason":null|"stop"|...}]}
//
// We maintain per-stream state: the current chat completion ID, the
// assistant-message ID, the echo model, and a mapping from Anthropic content
// block indexes to our own tool_call index counter (OpenAI tool_calls are
// ordered by increasing index starting at 0, independent of Anthropic block
// index which may include text blocks in between).

package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json/v2"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// StreamTranslator converts an Anthropic SSE byte stream into OpenAI SSE
// chunks written to its http.ResponseWriter. A single translator instance
// is used for exactly one request.
type StreamTranslator struct {
	w         http.ResponseWriter
	flusher   http.Flusher
	echoModel string
	created   int64

	chatID   string // chatcmpl-<suffix>; lazily populated when we see message_start
	sentRole bool   // whether we've emitted the role:"assistant" chunk

	// Per-block state. blockToolIdx[anthropicBlockIdx] -> OpenAI tool_calls index.
	// -1 means "not a tool block" or "not yet seen".
	blockToolIdx map[int]int
	// blockToolMeta[anthropicBlockIdx] -> the content_block start metadata so
	// we can emit the OpenAI tool_call head chunk (id, name) before any
	// argument deltas.
	blockToolMeta map[int]struct {
		ID   string
		Name string
	}
	nextToolIdx int

	// Final usage, captured from message_start and updated from message_delta.
	inputTokens  int
	outputTokens int

	// Final finish_reason, populated from message_delta.stop_reason and
	// promoted to the terminal chunk on message_stop.
	finishReason string
	// Whether any tool_use block was seen (forces finish_reason="tool_calls").
	hasToolUse bool

	// sentFinal guards the final chunk: message_stop, stream EOF, and error all race to emit it.
	sentFinal bool

	// writeErr is sticky — once the underlying writer errors we stop writing.
	writeErr bool
}

// NewStreamTranslator builds a translator bound to w. It configures the
// SSE response headers. echoModel is the OpenAI-flavor model name echoed
// back in every chunk.
func NewStreamTranslator(w http.ResponseWriter, echoModel string) *StreamTranslator {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	// Nginx / proxies: disable buffering so chunks reach the client promptly.
	w.Header().Set("X-Accel-Buffering", "no")

	f, _ := w.(http.Flusher)
	return &StreamTranslator{
		w:             w,
		flusher:       f,
		echoModel:     echoModel,
		created:       time.Now().Unix(),
		blockToolIdx:  map[int]int{},
		blockToolMeta: map[int]struct{ ID, Name string }{},
	}
}

// Translate reads Anthropic SSE bytes from r and emits OpenAI chunks until
// EOF or a terminal error. Returns the first non-nil write/parse error; EOF
// is normal and reported as nil.
func (s *StreamTranslator) Translate(ctx context.Context, r io.Reader) error {
	scanner := bufio.NewScanner(r)
	// Allow large frames (tool call argument blobs can be big).
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	scanner.Split(splitSSEFrames)

	for scanner.Scan() {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		event, data := parseSSEFrame(scanner.Bytes())
		if event == "" && data == "" {
			continue
		}
		if err := s.handleFrame(event, data); err != nil {
			return err
		}
		if s.writeErr {
			return fmt.Errorf("client disconnected")
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("sse scan: %w", err)
	}
	// Ensure [DONE] sentinel even if upstream omitted message_stop.
	s.writeDone()
	return nil
}

// handleFrame dispatches one Anthropic event to the appropriate OpenAI
// chunk emitter.
func (s *StreamTranslator) handleFrame(event, data string) error {
	switch event {
	case "message_start":
		return s.handleMessageStart(data)
	case "content_block_start":
		return s.handleContentBlockStart(data)
	case "content_block_delta":
		return s.handleContentBlockDelta(data)
	case "content_block_stop":
		// No OpenAI equivalent — end of a block is implicit.
		return nil
	case "message_delta":
		return s.handleMessageDelta(data)
	case "message_stop":
		return s.handleMessageStop()
	case "error":
		return s.handleError(data)
	case "ping":
		return nil
	default:
		// Unknown event types are benign; log and skip.
		slog.Debug("openai stream: unknown anthropic event", "event", event)
		return nil
	}
}

func (s *StreamTranslator) handleMessageStart(data string) error {
	var payload struct {
		Message struct {
			ID    string `json:"id"`
			Usage struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
		} `json:"message"`
	}
	if err := json.Unmarshal([]byte(data), &payload); err != nil {
		return fmt.Errorf("message_start json: %w", err)
	}
	s.chatID = chatCompletionID(payload.Message.ID)
	s.inputTokens = payload.Message.Usage.InputTokens
	s.outputTokens = payload.Message.Usage.OutputTokens

	// Emit the opening role chunk.
	s.sentRole = true
	return s.writeChunk(ChunkDelta{Role: RoleAssistant}, nil)
}

func (s *StreamTranslator) handleContentBlockStart(data string) error {
	var payload struct {
		Index        int `json:"index"`
		ContentBlock struct {
			Type  string         `json:"type"`
			ID    string         `json:"id"`
			Name  string         `json:"name"`
			Input map[string]any `json:"input"`
		} `json:"content_block"`
	}
	if err := json.Unmarshal([]byte(data), &payload); err != nil {
		return fmt.Errorf("content_block_start json: %w", err)
	}
	switch payload.ContentBlock.Type {
	case "tool_use", "server_tool_use":
		s.hasToolUse = true
		idx := s.nextToolIdx
		s.nextToolIdx++
		s.blockToolIdx[payload.Index] = idx
		s.blockToolMeta[payload.Index] = struct{ ID, Name string }{
			ID:   payload.ContentBlock.ID,
			Name: payload.ContentBlock.Name,
		}
		// Emit the tool_call "head" chunk: id + name + empty arguments.
		// Per OpenAI spec, only the first chunk for a given tool_call index
		// carries id+name; subsequent chunks for the same index carry just
		// argument deltas.
		i := idx
		return s.writeChunk(ChunkDelta{
			ToolCalls: []ToolCall{{
				Index: &i,
				ID:    payload.ContentBlock.ID,
				Type:  "function",
				Function: ToolCallFunction{
					Name:      payload.ContentBlock.Name,
					Arguments: "",
				},
			}},
		}, nil)
	default:
		s.blockToolIdx[payload.Index] = -1
	}
	return nil
}

func (s *StreamTranslator) handleContentBlockDelta(data string) error {
	var payload struct {
		Index int `json:"index"`
		Delta struct {
			Type        string `json:"type"`
			Text        string `json:"text"`
			PartialJSON string `json:"partial_json"`
		} `json:"delta"`
	}
	if err := json.Unmarshal([]byte(data), &payload); err != nil {
		return fmt.Errorf("content_block_delta json: %w", err)
	}
	switch payload.Delta.Type {
	case "text_delta":
		if payload.Delta.Text == "" {
			return nil
		}
		return s.writeChunk(ChunkDelta{Content: payload.Delta.Text}, nil)
	case "input_json_delta":
		toolIdx, ok := s.blockToolIdx[payload.Index]
		if !ok || toolIdx < 0 {
			// Unknown block or not a tool_use block — drop.
			return nil
		}
		i := toolIdx
		return s.writeChunk(ChunkDelta{
			ToolCalls: []ToolCall{{
				Index: &i,
				Function: ToolCallFunction{
					Arguments: payload.Delta.PartialJSON,
				},
			}},
		}, nil)
	case "thinking_delta", "signature_delta":
		// Drop — OpenAI has no equivalent.
		return nil
	default:
		return nil
	}
}

func (s *StreamTranslator) handleMessageDelta(data string) error {
	var payload struct {
		Delta struct {
			StopReason   string `json:"stop_reason"`
			StopSequence string `json:"stop_sequence"`
		} `json:"delta"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal([]byte(data), &payload); err != nil {
		return fmt.Errorf("message_delta json: %w", err)
	}
	if payload.Usage.InputTokens > 0 {
		s.inputTokens = payload.Usage.InputTokens
	}
	if payload.Usage.OutputTokens > 0 {
		s.outputTokens = payload.Usage.OutputTokens
	}
	if payload.Delta.StopReason != "" {
		s.finishReason = mapFinishReason(payload.Delta.StopReason, s.hasToolUse)
	}
	return nil
}

func (s *StreamTranslator) handleMessageStop() error {
	if s.sentFinal {
		return nil
	}
	s.sentFinal = true
	reason := s.finishReason
	if reason == "" {
		reason = mapFinishReason("", s.hasToolUse)
	}

	// Final chunk: empty delta + finish_reason + usage.
	usage := &Usage{
		PromptTokens:     s.inputTokens,
		CompletionTokens: s.outputTokens,
		TotalTokens:      s.inputTokens + s.outputTokens,
	}
	return s.writeChunk(ChunkDelta{}, &reason, withUsage(usage))
}

func (s *StreamTranslator) handleError(data string) error {
	// Anthropic error event → OpenAI error is unusual mid-stream (clients
	// typically expect DONE). We write an OpenAI-shaped error chunk
	// (using an error field is non-standard; we fall back to a final chunk
	// with finish_reason=content_filter as the closest OpenAI signal).
	var payload struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.Unmarshal([]byte(data), &payload)
	reason := FinishReasonContentFilter
	if err := s.writeChunk(ChunkDelta{}, &reason); err != nil {
		return err
	}
	s.writeDone()
	return fmt.Errorf("upstream error: %s: %s", payload.Error.Type, payload.Error.Message)
}

// chunkOption applies tweaks to a chunk before emission.
type chunkOption func(*ChatCompletionChunk)

func withUsage(u *Usage) chunkOption {
	return func(c *ChatCompletionChunk) { c.Usage = u }
}

// writeChunk emits one OpenAI SSE chunk. finishReason may be nil for non-
// terminal chunks. opts may set optional fields like Usage.
func (s *StreamTranslator) writeChunk(delta ChunkDelta, finishReason *string, opts ...chunkOption) error {
	if s.writeErr {
		return nil
	}
	if s.chatID == "" {
		// Synthesize a stable ID if message_start never arrived.
		s.chatID = "chatcmpl-" + fmt.Sprintf("%d", s.created)
	}
	chunk := &ChatCompletionChunk{
		ID:      s.chatID,
		Object:  ObjectChatCompletionChunk,
		Created: s.created,
		Model:   s.echoModel,
		Choices: []ChunkChoice{{
			Index:        0,
			Delta:        delta,
			FinishReason: finishReason,
		}},
	}
	for _, opt := range opts {
		opt(chunk)
	}
	b, err := json.Marshal(chunk)
	if err != nil {
		return fmt.Errorf("marshal chunk: %w", err)
	}
	if _, err := fmt.Fprintf(s.w, "data: %s\n\n", b); err != nil {
		s.writeErr = true
		return nil // caller checks writeErr
	}
	if s.flusher != nil {
		s.flusher.Flush()
	}
	return nil
}

// writeDone emits the OpenAI SSE terminator.
func (s *StreamTranslator) writeDone() {
	if s.writeErr {
		return
	}
	if _, err := fmt.Fprintf(s.w, "data: [DONE]\n\n"); err != nil {
		s.writeErr = true
		return
	}
	if s.flusher != nil {
		s.flusher.Flush()
	}
}

// -- SSE frame parsing helpers --------------------------------------------

// splitSSEFrames is a bufio.Scanner SplitFunc that yields one SSE frame at
// a time, terminated by a blank line (\n\n or \r\n\r\n).
func splitSSEFrames(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	// Look for double-newline frame boundary.
	for i := 0; i+1 < len(data); i++ {
		if data[i] == '\n' && data[i+1] == '\n' {
			return i + 2, data[:i], nil
		}
		if i+3 < len(data) && data[i] == '\r' && data[i+1] == '\n' && data[i+2] == '\r' && data[i+3] == '\n' {
			return i + 4, data[:i], nil
		}
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}

// parseSSEFrame extracts (event, data) from an SSE frame. Data lines are
// joined with newlines per SSE spec but Anthropic always emits a single
// data: line so we keep it simple.
func parseSSEFrame(frame []byte) (string, string) {
	var event, data string
	for _, line := range bytes.Split(frame, []byte("\n")) {
		line = bytes.TrimRight(line, "\r")
		if len(line) == 0 || bytes.HasPrefix(line, []byte(":")) {
			continue
		}
		if bytes.HasPrefix(line, []byte("event:")) {
			event = strings.TrimSpace(string(line[len("event:"):]))
			continue
		}
		if bytes.HasPrefix(line, []byte("data:")) {
			value := strings.TrimSpace(string(line[len("data:"):]))
			if data == "" {
				data = value
			} else {
				data = data + "\n" + value
			}
			continue
		}
	}
	return event, data
}
