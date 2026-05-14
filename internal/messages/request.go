// This file is derived from github.com/d-kuro/kirocc
// Original commit: 5633c47f0d65aaef748728bae1c68160b0ea538d
// Copyright (c) 2026 d-kuro. Licensed under Apache License, Version 2.0.
// Modifications (c) 2026 kiroxy contributors.

package messages

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"local/kiroxy/internal/anthropic"
	"local/kiroxy/internal/httpx"
	"local/kiroxy/internal/tokencount"
)

// HandleCountTokens serves POST /v1/messages/count_tokens.
func (s *Service) HandleCountTokens(w http.ResponseWriter, r *http.Request) {
	req, err := parseAndValidateRequest(r.Context(), w, r)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, errTypeInvalidRequest, err.Error())
		return
	}

	// Build a token-counter input from the Anthropic request shape rather
	// than the Kiro wire payload. The wire payload bakes in 1–3 KB of
	// protocol overhead per request (synthetic ack message, ConversationState
	// envelope, x-amz-target tags, ARN string, tool_specification wrapper)
	// that the model NEVER sees. Counting that overhead inflates the
	// returned input_tokens, causing clients (e.g. Claude Code, Cline) that
	// budget against this number to over-trim conversation history.
	//
	// We approximate what the model actually consumes: system prompt +
	// concatenated message text + each tool's name/description/schema.
	// Tool input_schema is JSON-marshaled as-is (the model does see the
	// schema as text), but the tool itself is NOT wrapped in toolSpecification.
	data := buildCountTokensInput(req)

	n, err := tokencount.CountBytes(data)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, errTypeAPI, "token counting unavailable")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.MarshalWrite(w, map[string]int{"input_tokens": n}); err != nil {
		slog.ErrorContext(r.Context(), "write count_tokens response failed", "err", err)
		return
	}
	_, _ = w.Write([]byte("\n"))
}

// buildCountTokensInput synthesizes a byte slice representing what the
// model would actually see for token-counting purposes. Order: system,
// each message in turn (role + content), then each tool definition
// (name + description + JSON schema). Newline-separated for tokenizer
// boundary stability.
func buildCountTokensInput(req *anthropic.Request) []byte {
	var buf bytes.Buffer
	if !req.System.IsEmpty() {
		if req.System.Text != "" {
			buf.WriteString(req.System.Text)
			buf.WriteByte('\n')
		}
		for _, b := range req.System.Blocks {
			buf.WriteString(b.Text)
			buf.WriteByte('\n')
		}
	}
	for _, m := range req.Messages {
		buf.WriteString(m.Role)
		buf.WriteString(": ")
		buf.WriteString(m.Content.String())
		buf.WriteByte('\n')
	}
	for _, t := range req.Tools {
		buf.WriteString(t.Name)
		buf.WriteByte('\n')
		buf.WriteString(t.Description)
		buf.WriteByte('\n')
		if len(t.InputSchema) > 0 {
			if schemaJSON, err := json.Marshal(t.InputSchema); err == nil {
				buf.Write(schemaJSON)
				buf.WriteByte('\n')
			}
		}
	}
	return buf.Bytes()
}

// parseAndValidateRequest decodes and validates an Anthropic request from the HTTP body.
func parseAndValidateRequest(ctx context.Context, w http.ResponseWriter, r *http.Request) (*anthropic.Request, error) {
	r.Body = http.MaxBytesReader(w, r.Body, 4<<20)
	var req anthropic.Request
	if slog.Default().Enabled(ctx, slog.LevelDebug) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, fmt.Errorf("invalid request: %w", err)
		}
		slog.DebugContext(ctx, "client request body", "request_body", jsontext.Value(raw))
		if err := json.UnmarshalDecode(jsontext.NewDecoder(bytes.NewReader(raw)), &req); err != nil {
			return nil, fmt.Errorf("invalid request: %w", err)
		}
	} else {
		if err := json.UnmarshalRead(r.Body, &req); err != nil {
			return nil, fmt.Errorf("invalid request: %w", err)
		}
	}
	if len(req.Messages) == 0 {
		return nil, fmt.Errorf("messages must not be empty")
	}
	return &req, nil
}
