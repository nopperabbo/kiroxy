// Package openai provides OpenAI Chat Completions / Models API compatibility
// as a translation shim over the existing Anthropic Messages pipeline. The
// aim is to let OpenAI SDK clients (Cursor, Continue, Cline, aider, raw SDK)
// reach the same Kiro upstream with no kiroxy-specific changes on their side.
//
// The approach is an edge-only adapter: this package does NOT re-implement
// the Kiro pipeline. It translates OpenAI JSON shapes to Anthropic shapes on
// the way in and Anthropic JSON/SSE back to OpenAI JSON/SSE on the way out.
// All behavior (auth, retries, tool-search, token counting, streaming)
// continues to flow through internal/messages → internal/reqconv →
// internal/respconv → internal/kiroclient unchanged.
//
// See .sisyphus/plans/phase-J-design.md for the full design and alias table.
package openai

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
)

// Object type constants for OpenAI envelopes.
const (
	ObjectChatCompletion      = "chat.completion"
	ObjectChatCompletionChunk = "chat.completion.chunk"
	ObjectModel               = "model"
	ObjectList                = "list"
)

// Role constants as understood by OpenAI's Chat Completions API.
const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"
)

// Finish reason constants (OpenAI spec).
const (
	FinishReasonStop          = "stop"
	FinishReasonLength        = "length"
	FinishReasonToolCalls     = "tool_calls"
	FinishReasonContentFilter = "content_filter"
)

// Error type constants (OpenAI spec).
const (
	ErrTypeInvalidRequest = "invalid_request_error"
	ErrTypeAPI            = "api_error"
	ErrTypeAuthentication = "authentication_error"
)

// ChatCompletionRequest is the body of POST /v1/chat/completions. Fields we
// do not translate (temperature, top_p, presence_penalty, frequency_penalty,
// user, logit_bias, logprobs) are accepted and silently dropped so clients
// that include them unconditionally still work.
type ChatCompletionRequest struct {
	Model            string          `json:"model"`
	Messages         []ChatMessage   `json:"messages"`
	MaxTokens        *int            `json:"max_tokens,omitempty"`
	MaxOutputTokens  *int            `json:"max_completion_tokens,omitempty"` // newer OpenAI alias
	Temperature      *float64        `json:"temperature,omitempty"`
	TopP             *float64        `json:"top_p,omitempty"`
	Stream           bool            `json:"stream,omitempty"`
	StreamOptions    *StreamOptions  `json:"stream_options,omitempty"`
	Stop             StopField       `json:"stop,omitempty"`
	Tools            []Tool          `json:"tools,omitempty"`
	ToolChoice       *ToolChoice     `json:"tool_choice,omitempty"`
	N                *int            `json:"n,omitempty"`
	PresencePenalty  *float64        `json:"presence_penalty,omitempty"`
	FrequencyPenalty *float64        `json:"frequency_penalty,omitempty"`
	User             string          `json:"user,omitempty"`
	ResponseFormat   *ResponseFormat `json:"response_format,omitempty"`
}

// StreamOptions is the OpenAI stream_options object.
type StreamOptions struct {
	IncludeUsage bool `json:"include_usage,omitempty"`
}

// ResponseFormat maps to OpenAI's response_format. Currently dropped on
// translation; kept here so unmarshaling doesn't reject it.
type ResponseFormat struct {
	Type string `json:"type"`
}

// StopField is a union: a single string, an array of strings, or absent.
type StopField struct {
	Values []string
}

// IsEmpty reports whether there are no stop sequences set.
func (s StopField) IsEmpty() bool { return len(s.Values) == 0 }

func (s StopField) MarshalJSONTo(enc *jsontext.Encoder) error {
	if len(s.Values) == 0 {
		return enc.WriteToken(jsontext.Null)
	}
	if len(s.Values) == 1 {
		return json.MarshalEncode(enc, s.Values[0])
	}
	return json.MarshalEncode(enc, s.Values)
}

func (s *StopField) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	switch dec.PeekKind() {
	case 'n':
		_, err := dec.ReadToken()
		return err
	case '"':
		var v string
		if err := json.UnmarshalDecode(dec, &v); err != nil {
			return err
		}
		s.Values = []string{v}
		return nil
	case '[':
		return json.UnmarshalDecode(dec, &s.Values)
	default:
		return fmt.Errorf("stop: unexpected JSON kind %v", dec.PeekKind())
	}
}

// ChatMessage is a single message in the OpenAI chat format.
type ChatMessage struct {
	Role       string         `json:"role"`
	Content    MessageContent `json:"content"`
	Name       string         `json:"name,omitempty"`
	ToolCalls  []ToolCall     `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
}

// MessageContent is a union: a plain string or an array of content parts.
// OpenAI accepts both; we normalize to the same representation.
type MessageContent struct {
	Text  string        // set when content is a plain string
	Parts []ContentPart // set when content is an array
	IsNull bool         // set when content is explicitly null (assistant tool_calls messages)
}

// IsString reports whether the content was a plain string.
func (m MessageContent) IsString() bool { return m.Parts == nil && !m.IsNull }

func (m MessageContent) MarshalJSONTo(enc *jsontext.Encoder) error {
	if m.IsNull {
		return enc.WriteToken(jsontext.Null)
	}
	if m.Parts != nil {
		return json.MarshalEncode(enc, m.Parts)
	}
	return json.MarshalEncode(enc, m.Text)
}

func (m *MessageContent) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	switch dec.PeekKind() {
	case 'n':
		_, err := dec.ReadToken()
		if err != nil {
			return err
		}
		m.IsNull = true
		return nil
	case '"':
		return json.UnmarshalDecode(dec, &m.Text)
	case '[':
		return json.UnmarshalDecode(dec, &m.Parts)
	default:
		return fmt.Errorf("content: unexpected JSON kind %v", dec.PeekKind())
	}
}

// ContentPart is a single entry of a multi-part message content.
// The OpenAI spec distinguishes text and image_url parts.
type ContentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}

// Content part types.
const (
	PartTypeText     = "text"
	PartTypeImageURL = "image_url"
)

// ImageURL is the image_url object of a content part. We support data-URI
// images (base64 inline); https URLs are rejected because Anthropic upstream
// only accepts base64.
type ImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

// Tool is an OpenAI tool (function) definition.
type Tool struct {
	Type     string       `json:"type"` // must be "function"
	Function FunctionDef  `json:"function"`
}

// FunctionDef is the function schema inside a tool definition.
type FunctionDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

// ToolChoice is either a string ("auto" | "none" | "required") or a
// {type:"function", function:{name:...}} object.
type ToolChoice struct {
	String   string // when set, one of auto / none / required
	Function string // when set, specific function name
}

func (t ToolChoice) MarshalJSONTo(enc *jsontext.Encoder) error {
	if t.Function != "" {
		return json.MarshalEncode(enc, map[string]any{
			"type":     "function",
			"function": map[string]any{"name": t.Function},
		})
	}
	if t.String != "" {
		return json.MarshalEncode(enc, t.String)
	}
	return enc.WriteToken(jsontext.Null)
}

func (t *ToolChoice) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	switch dec.PeekKind() {
	case 'n':
		_, err := dec.ReadToken()
		return err
	case '"':
		return json.UnmarshalDecode(dec, &t.String)
	case '{':
		var obj struct {
			Type     string `json:"type"`
			Function struct {
				Name string `json:"name"`
			} `json:"function"`
		}
		if err := json.UnmarshalDecode(dec, &obj); err != nil {
			return err
		}
		t.Function = obj.Function.Name
		return nil
	default:
		return fmt.Errorf("tool_choice: unexpected JSON kind %v", dec.PeekKind())
	}
}

// ToolCall is a single tool call in a message.
// In assistant messages (request or response) this carries function invocation
// details; in streaming chunks partial argument JSON is emitted via the
// Function.Arguments field.
type ToolCall struct {
	Index    *int             `json:"index,omitempty"` // streaming only
	ID       string           `json:"id,omitempty"`
	Type     string           `json:"type,omitempty"` // "function"
	Function ToolCallFunction `json:"function"`
}

// ToolCallFunction holds the invoked function name and argument JSON.
type ToolCallFunction struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments"` // JSON string, may be empty or partial during streaming
}

// ChatCompletionResponse is the non-streaming response body.
type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// Choice is a single element in a ChatCompletionResponse.Choices array.
type Choice struct {
	Index        int              `json:"index"`
	Message      ChatMessage      `json:"message"`
	FinishReason string           `json:"finish_reason"`
	Logprobs     any              `json:"logprobs,omitempty"` // always null for us
}

// Usage is the OpenAI token usage envelope.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ChatCompletionChunk is a single streaming event body.
type ChatCompletionChunk struct {
	ID      string        `json:"id"`
	Object  string        `json:"object"`
	Created int64         `json:"created"`
	Model   string        `json:"model"`
	Choices []ChunkChoice `json:"choices"`
	Usage   *Usage        `json:"usage,omitempty"`
}

// ChunkChoice is a single choice in a streaming chunk.
type ChunkChoice struct {
	Index        int         `json:"index"`
	Delta        ChunkDelta  `json:"delta"`
	FinishReason *string     `json:"finish_reason"` // null until final chunk
	Logprobs     any         `json:"logprobs,omitempty"`
}

// ChunkDelta is the delta payload inside a streaming chunk.
type ChunkDelta struct {
	Role      string     `json:"role,omitempty"`
	Content   string     `json:"content,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// Model is a single entry in GET /v1/models.
type Model struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// ModelList is the envelope for GET /v1/models.
type ModelList struct {
	Object string  `json:"object"`
	Data   []Model `json:"data"`
}
