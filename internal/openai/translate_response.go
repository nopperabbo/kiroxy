// Non-streaming response translation: Anthropic Messages JSON → OpenAI
// Chat Completion JSON. The Anthropic response comes from
// respconv.buildResponseFromAcc as a map[string]any with the shape
// documented at https://docs.anthropic.com/en/api/messages.
//
// We read it through a typed struct instead of map traversal for clarity.
// Thinking blocks are intentionally dropped (OpenAI has no equivalent).

package openai

import (
	"encoding/json/v2"
	"fmt"
	"strings"
	"time"

	"github.com/nopperabbo/kiroxy/internal/anthropic"
)

// anthropicResponse mirrors the fields we care about from the Anthropic
// /v1/messages JSON response. Extra fields are ignored.
type anthropicResponse struct {
	ID           string                   `json:"id"`
	Model        string                   `json:"model"`
	Role         string                   `json:"role"`
	Content      []anthropic.ContentBlock `json:"content"`
	StopReason   string                   `json:"stop_reason"`
	StopSequence string                   `json:"stop_sequence"`
	Usage        anthropicUsage           `json:"usage"`
}

type anthropicUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
}

// TranslateResponse parses Anthropic response bytes and returns an OpenAI
// ChatCompletionResponse. echoModel is the OpenAI-flavor model name we
// advertise back (the alias the caller sent), used in the response's Model
// field so clients see the same string they requested.
func TranslateResponse(body []byte, echoModel string) (*ChatCompletionResponse, error) {
	var ar anthropicResponse
	if err := json.Unmarshal(body, &ar); err != nil {
		return nil, fmt.Errorf("parse anthropic response: %w", err)
	}
	return buildChatCompletion(&ar, echoModel), nil
}

// buildChatCompletion assembles the OpenAI envelope from a parsed Anthropic
// response. Splits responsibility from TranslateResponse so callers that
// already have the parsed shape (streaming accumulator, tests) can reuse.
func buildChatCompletion(ar *anthropicResponse, echoModel string) *ChatCompletionResponse {
	msg, finishReason := convertAssistantContent(ar.Content, ar.StopReason)
	id := chatCompletionID(ar.ID)

	return &ChatCompletionResponse{
		ID:      id,
		Object:  ObjectChatCompletion,
		Created: time.Now().Unix(),
		Model:   echoModel,
		Choices: []Choice{{
			Index:        0,
			Message:      msg,
			FinishReason: finishReason,
		}},
		Usage: Usage{
			PromptTokens:     ar.Usage.InputTokens,
			CompletionTokens: ar.Usage.OutputTokens,
			TotalTokens:      ar.Usage.InputTokens + ar.Usage.OutputTokens,
		},
	}
}

// convertAssistantContent folds Anthropic content blocks into an OpenAI
// assistant ChatMessage and returns the finish_reason matching the stop
// reason / tool_use presence.
func convertAssistantContent(blocks []anthropic.ContentBlock, stopReason string) (ChatMessage, string) {
	var textParts []string
	var toolCalls []ToolCall
	hasToolUse := false

	for _, b := range blocks {
		switch b.Type {
		case anthropic.BlockTypeText:
			if b.Text != "" {
				textParts = append(textParts, b.Text)
			}
		case anthropic.BlockTypeToolUse, anthropic.BlockTypeServerToolUse:
			hasToolUse = true
			args, _ := marshalArguments(b.Input)
			toolCalls = append(toolCalls, ToolCall{
				ID:   b.ID,
				Type: "function",
				Function: ToolCallFunction{
					Name:      b.Name,
					Arguments: args,
				},
			})
		case anthropic.BlockTypeThinking, anthropic.BlockTypeRedactedThinking:
			// OpenAI has no equivalent. Drop silently.
		default:
			// Unknown blocks (tool_search_*, etc.) are ignored on the OpenAI
			// surface; they do not have a clean mapping.
		}
	}

	msg := ChatMessage{Role: RoleAssistant}
	if len(textParts) > 0 {
		msg.Content = MessageContent{Text: strings.Join(textParts, "")}
	} else if len(toolCalls) > 0 {
		// When only tool_calls are present, OpenAI convention is content: null.
		msg.Content = MessageContent{IsNull: true}
	} else {
		msg.Content = MessageContent{Text: ""}
	}
	msg.ToolCalls = toolCalls

	return msg, mapFinishReason(stopReason, hasToolUse)
}

// mapFinishReason translates Anthropic stop_reason to OpenAI finish_reason.
// When the upstream finished on tool_use, OpenAI expects "tool_calls" even
// if stop_reason was not explicitly tool_use (some models forget to set it
// — hasToolUse is the source of truth).
func mapFinishReason(stopReason string, hasToolUse bool) string {
	if hasToolUse {
		return FinishReasonToolCalls
	}
	switch stopReason {
	case "end_turn":
		return FinishReasonStop
	case "stop_sequence":
		return FinishReasonStop
	case "max_tokens":
		return FinishReasonLength
	case "tool_use":
		return FinishReasonToolCalls
	case "":
		return FinishReasonStop
	default:
		return FinishReasonStop
	}
}

// marshalArguments serializes a tool_use input map back to the JSON string
// OpenAI clients expect in function.arguments. Falls back to "{}" on error
// to avoid poisoning the response.
func marshalArguments(input map[string]any) (string, error) {
	if input == nil {
		return "{}", nil
	}
	b, err := json.Marshal(input)
	if err != nil {
		return "{}", err
	}
	return string(b), nil
}

// chatCompletionID normalizes an Anthropic message ID ("msg_...") into an
// OpenAI-style "chatcmpl-..." ID. The Anthropic suffix is reused so the
// IDs remain correlatable in logs.
func chatCompletionID(anthropicID string) string {
	suffix := anthropicID
	if strings.HasPrefix(suffix, "msg_") {
		suffix = strings.TrimPrefix(suffix, "msg_")
	}
	if suffix == "" {
		return "chatcmpl-unknown"
	}
	return "chatcmpl-" + suffix
}
