// Request translation: OpenAI Chat Completions → Anthropic Messages.
//
// Rules of thumb (full details in .sisyphus/plans/phase-J-design.md):
//   - system messages (always at the top in well-formed requests) are
//     concatenated into anthropic.Request.System.
//   - user/assistant messages pass through role-preserving.
//   - OpenAI "tool" role messages carry tool results → translated into an
//     Anthropic user message with a tool_result block.
//   - assistant messages with tool_calls → translated into an Anthropic
//     assistant message with tool_use blocks.
//   - image_url parts: data-URI base64 only; https URLs are rejected
//     because the Anthropic upstream only accepts inline base64.
//   - tools: OpenAI function-tool array → anthropic.Tool[] using the
//     function.parameters schema as input_schema verbatim.
//   - tool_choice: "auto" default, "none" strips tools, "required" and
//     specific function-name choices are documented as unsupported (v1.1).
//   - Dropped silently: temperature, top_p, presence_penalty,
//     frequency_penalty, user, logit_bias, logprobs, response_format.
//
// Anthropic requires max_tokens; when an OpenAI request omits it we default
// to 4096 (a conservative OpenAI-compatible default).

package openai

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/nopperabbo/kiroxy/internal/anthropic"
)

const defaultMaxTokens = 4096

// TranslateRequest converts an OpenAI Chat Completions request into an
// Anthropic Messages request. It returns the translated request, the
// canonical model string to advertise back on the response (the caller will
// prefer echoing back the original OpenAI alias), and any validation error.
//
// The returned *anthropic.Request can be marshaled to JSON and fed into
// the existing /v1/messages handler.
func TranslateRequest(in *ChatCompletionRequest) (*anthropic.Request, error) {
	if in == nil {
		return nil, fmt.Errorf("request is nil")
	}
	if strings.TrimSpace(in.Model) == "" {
		return nil, newValidationError("model is required", "model")
	}
	if len(in.Messages) == 0 {
		return nil, newValidationError("messages must not be empty", "messages")
	}
	if in.N != nil && *in.N != 1 {
		return nil, newValidationError("only n=1 is supported", "n")
	}

	// Resolve alias BEFORE translation so the request hitting messages.Service
	// uses a canonical model name messages/Resolve knows about.
	modelOut := ResolveModel(in.Model)

	out := &anthropic.Request{
		Model:  modelOut,
		Stream: in.Stream,
	}

	// max_tokens: accept either "max_tokens" or the newer
	// "max_completion_tokens" alias. Fall back to a safe default.
	switch {
	case in.MaxTokens != nil:
		out.MaxTokens = *in.MaxTokens
	case in.MaxOutputTokens != nil:
		out.MaxTokens = *in.MaxOutputTokens
	default:
		out.MaxTokens = defaultMaxTokens
	}
	if out.MaxTokens <= 0 {
		out.MaxTokens = defaultMaxTokens
	}

	// Stop sequences.
	if !in.Stop.IsEmpty() {
		out.StopSequences = append([]string(nil), in.Stop.Values...)
	}

	// Tools.
	if tc := in.ToolChoice; tc != nil {
		if tc.Function != "" {
			return nil, newValidationError(
				"specific function tool_choice is not supported yet (v1.1 follow-up); use \"auto\" or \"none\"",
				"tool_choice",
			)
		}
	}
	if in.ToolChoice == nil || in.ToolChoice.String != "none" {
		// tool_choice=none effectively disables tools; otherwise pass them through.
		for i := range in.Tools {
			t := &in.Tools[i]
			if t.Type != "" && t.Type != "function" {
				return nil, newValidationError(
					fmt.Sprintf("unsupported tool type %q (only \"function\" is supported)", t.Type),
					"tools",
				)
			}
			if strings.TrimSpace(t.Function.Name) == "" {
				return nil, newValidationError("tool function name is required", "tools")
			}
			out.Tools = append(out.Tools, anthropic.Tool{
				Name:        t.Function.Name,
				Description: t.Function.Description,
				InputSchema: t.Function.Parameters,
			})
		}
	}

	// Messages: extract system prompt, then translate user/assistant/tool roles.
	sys, rest, err := splitSystem(in.Messages)
	if err != nil {
		return nil, err
	}
	if sys != "" {
		out.System = anthropic.SystemPrompt{Text: sys}
	}

	// Tool messages in OpenAI carry tool results that must be attached to the
	// preceding assistant message as a user-role message with a tool_result
	// block. Coalesce runs of consecutive tool messages into one user message
	// (Anthropic allows multiple tool_result blocks in a single user message).
	msgs, err := translateMessages(rest)
	if err != nil {
		return nil, err
	}
	out.Messages = msgs

	return out, nil
}

// splitSystem pulls leading system-role messages off the list and joins
// their text content with blank lines into a single system prompt. Anthropic
// only allows system as a top-level field, not inline. A system message
// appearing mid-list is rejected to keep the semantics predictable.
func splitSystem(msgs []ChatMessage) (string, []ChatMessage, error) {
	var sys []string
	cut := 0
	for i, m := range msgs {
		if m.Role != RoleSystem {
			cut = i
			break
		}
		text, err := contentAsText(m.Content, "system")
		if err != nil {
			return "", nil, err
		}
		sys = append(sys, text)
		cut = i + 1
	}
	// Reject system messages appearing after non-system messages.
	for _, m := range msgs[cut:] {
		if m.Role == RoleSystem {
			return "", nil, newValidationError(
				"system messages must appear at the start of the messages array",
				"messages",
			)
		}
	}
	return strings.Join(sys, "\n\n"), msgs[cut:], nil
}

// contentAsText extracts text from a MessageContent, rejecting images for
// roles that do not support them (system, tool).
func contentAsText(c MessageContent, role string) (string, error) {
	if c.IsNull {
		return "", nil
	}
	if c.IsString() {
		return c.Text, nil
	}
	var out strings.Builder
	for _, p := range c.Parts {
		switch p.Type {
		case PartTypeText:
			if out.Len() > 0 {
				out.WriteString("\n")
			}
			out.WriteString(p.Text)
		case PartTypeImageURL:
			return "", newValidationError(
				fmt.Sprintf("%s messages must not contain images", role),
				"messages",
			)
		default:
			return "", newValidationError(
				fmt.Sprintf("unsupported content part type %q", p.Type),
				"messages",
			)
		}
	}
	return out.String(), nil
}

// translateMessages converts the non-system tail of the OpenAI message list
// into Anthropic messages, handling tool-role coalescing and assistant
// tool_calls conversion.
func translateMessages(in []ChatMessage) ([]anthropic.Message, error) {
	var out []anthropic.Message
	for i := 0; i < len(in); i++ {
		m := in[i]
		switch m.Role {
		case RoleUser:
			am, err := translateUserMessage(m)
			if err != nil {
				return nil, err
			}
			out = append(out, am)
		case RoleAssistant:
			am, err := translateAssistantMessage(m)
			if err != nil {
				return nil, err
			}
			out = append(out, am)
		case RoleTool:
			// Coalesce consecutive tool messages into one user message.
			j := i
			var blocks []anthropic.ContentBlock
			for j < len(in) && in[j].Role == RoleTool {
				block, err := translateToolResult(in[j])
				if err != nil {
					return nil, err
				}
				blocks = append(blocks, block)
				j++
			}
			out = append(out, anthropic.Message{
				Role:    RoleUser,
				Content: anthropic.MessageContent{Blocks: blocks},
			})
			i = j - 1
		case RoleSystem:
			return nil, newValidationError(
				"system messages must appear at the start of the messages array",
				"messages",
			)
		default:
			return nil, newValidationError(
				fmt.Sprintf("unknown role %q", m.Role),
				"messages",
			)
		}
	}
	return out, nil
}

func translateUserMessage(m ChatMessage) (anthropic.Message, error) {
	if m.IsString() {
		return anthropic.Message{
			Role:    RoleUser,
			Content: anthropic.MessageContent{Text: m.Content.Text},
		}, nil
	}
	var blocks []anthropic.ContentBlock
	if m.Content.IsString() {
		blocks = append(blocks, anthropic.ContentBlock{
			Type: anthropic.BlockTypeText,
			Text: m.Content.Text,
		})
	} else {
		for _, p := range m.Content.Parts {
			switch p.Type {
			case PartTypeText:
				blocks = append(blocks, anthropic.ContentBlock{
					Type: anthropic.BlockTypeText,
					Text: p.Text,
				})
			case PartTypeImageURL:
				if p.ImageURL == nil {
					return anthropic.Message{}, newValidationError(
						"image_url part missing url",
						"messages",
					)
				}
				src, err := parseImageURL(p.ImageURL.URL)
				if err != nil {
					return anthropic.Message{}, err
				}
				blocks = append(blocks, anthropic.ContentBlock{
					Type:   anthropic.BlockTypeImage,
					Source: src,
				})
			default:
				return anthropic.Message{}, newValidationError(
					fmt.Sprintf("unsupported content part type %q", p.Type),
					"messages",
				)
			}
		}
	}
	return anthropic.Message{
		Role:    RoleUser,
		Content: anthropic.MessageContent{Blocks: blocks},
	}, nil
}

// IsString is a convenience for ChatMessage to test if its content is
// a plain string and it has no tool_calls — i.e. the simplest case.
func (m ChatMessage) IsString() bool {
	return m.Content.IsString() && len(m.ToolCalls) == 0
}

func translateAssistantMessage(m ChatMessage) (anthropic.Message, error) {
	var blocks []anthropic.ContentBlock

	// Text content (optional when tool_calls is present).
	text, err := contentAsText(m.Content, "assistant")
	if err != nil {
		return anthropic.Message{}, err
	}
	if text != "" {
		blocks = append(blocks, anthropic.ContentBlock{
			Type: anthropic.BlockTypeText,
			Text: text,
		})
	}

	// Tool calls → tool_use blocks.
	for _, tc := range m.ToolCalls {
		if tc.Type != "" && tc.Type != "function" {
			return anthropic.Message{}, newValidationError(
				fmt.Sprintf("unsupported tool_call type %q", tc.Type),
				"messages",
			)
		}
		if tc.ID == "" {
			return anthropic.Message{}, newValidationError(
				"assistant tool_call missing id",
				"messages",
			)
		}
		input, err := parseArgumentsJSON(tc.Function.Arguments)
		if err != nil {
			return anthropic.Message{}, err
		}
		blocks = append(blocks, anthropic.ContentBlock{
			Type:  anthropic.BlockTypeToolUse,
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: input,
		})
	}

	if len(blocks) == 0 {
		// Empty assistant message — Anthropic allows this as empty string content.
		return anthropic.Message{
			Role:    RoleAssistant,
			Content: anthropic.MessageContent{Text: ""},
		}, nil
	}
	return anthropic.Message{
		Role:    RoleAssistant,
		Content: anthropic.MessageContent{Blocks: blocks},
	}, nil
}

// translateToolResult converts one OpenAI tool-role message into a single
// Anthropic tool_result content block (to be aggregated into a user message).
func translateToolResult(m ChatMessage) (anthropic.ContentBlock, error) {
	if m.ToolCallID == "" {
		return anthropic.ContentBlock{}, newValidationError(
			"tool message missing tool_call_id",
			"messages",
		)
	}
	text, err := contentAsText(m.Content, "tool")
	if err != nil {
		return anthropic.ContentBlock{}, err
	}
	return anthropic.ContentBlock{
		Type:      anthropic.BlockTypeToolResult,
		ToolUseID: m.ToolCallID,
		Content:   anthropic.MessageContent{Text: text},
	}, nil
}

// parseImageURL accepts a data URI and returns an Anthropic ImageSource.
// https/http URLs are rejected because the Anthropic upstream only
// supports base64-encoded images inline.
func parseImageURL(rawURL string) (*anthropic.ImageSource, error) {
	if !strings.HasPrefix(rawURL, "data:") {
		return nil, newValidationError(
			"only data: URIs are supported for images (https URLs not accepted by upstream)",
			"messages",
		)
	}
	// Format: data:<media-type>[;charset=...];base64,<data>
	rest := strings.TrimPrefix(rawURL, "data:")
	comma := strings.Index(rest, ",")
	if comma < 0 {
		return nil, newValidationError("malformed data URI", "messages")
	}
	header, data := rest[:comma], rest[comma+1:]
	if !strings.Contains(header, ";base64") {
		return nil, newValidationError(
			"data URI must be base64-encoded",
			"messages",
		)
	}
	mediaType := strings.TrimSuffix(strings.Split(header, ";")[0], ";base64")
	if mediaType == "" {
		mediaType = "image/png"
	}
	// Validate base64 data (lenient — accept padded and unpadded).
	if _, err := base64.StdEncoding.DecodeString(data); err != nil {
		if _, err2 := base64.RawStdEncoding.DecodeString(data); err2 != nil {
			return nil, newValidationError(
				"image data is not valid base64",
				"messages",
			)
		}
	}
	return &anthropic.ImageSource{
		Type:      "base64",
		MediaType: mediaType,
		Data:      data,
	}, nil
}

// parseArgumentsJSON parses a tool_call.function.arguments string. OpenAI
// sends it as a JSON string; Anthropic expects a map. Empty arguments are
// treated as the empty object.
func parseArgumentsJSON(args string) (map[string]any, error) {
	args = strings.TrimSpace(args)
	if args == "" {
		return map[string]any{}, nil
	}
	// Local import to avoid placing encoding/json/v2 at file-scope: keep
	// imports tidy. We use the standard stdlib json v2 consistent with the rest.
	var out map[string]any
	if err := unmarshalJSON([]byte(args), &out); err != nil {
		return nil, newValidationError(
			fmt.Sprintf("tool_call arguments not valid JSON: %v", err),
			"messages",
		)
	}
	if out == nil {
		out = map[string]any{}
	}
	return out, nil
}
