// This file is derived from github.com/d-kuro/kirocc
// Original commit: 5633c47f0d65aaef748728bae1c68160b0ea538d
// Copyright (c) 2026 d-kuro. Licensed under Apache License, Version 2.0.
// Modifications (c) 2026 kiroxy contributors.

package respconv

import (
	"github.com/nopperabbo/kiroxy/internal/anthropic"
	"github.com/nopperabbo/kiroxy/internal/toolsearch"
)

// WriteServerToolUse writes a server_tool_use content block start + input delta + stop.
func (s *SSEWriter) WriteServerToolUse(id, name, input string) {
	s.ensureStarted()
	s.fireVisibleOutput()
	s.writeBlock(
		map[string]any{
			"type":  anthropic.BlockTypeServerToolUse,
			"id":    id,
			"name":  name,
			"input": map[string]any{},
		},
		map[string]any{
			"type":         "input_json_delta",
			"partial_json": input,
		},
	)
}

// WriteToolSearchResult writes a tool_search_tool_result content block.
func (s *SSEWriter) WriteToolSearchResult(toolUseID string, toolRefs []string) {
	s.writeBlock(
		map[string]any{
			"type":        anthropic.BlockTypeToolSearchToolResult,
			"tool_use_id": toolUseID,
			"content": map[string]any{
				"type":            anthropic.BlockTypeToolSearchSearchResult,
				"tool_references": toolsearch.ToolRefMaps(toolRefs),
			},
		},
		nil,
	)
}

// WriteToolSearchError writes a tool_search_tool_result error content block.
func (s *SSEWriter) WriteToolSearchError(toolUseID string, errorCode string) {
	s.writeBlock(
		map[string]any{
			"type":        anthropic.BlockTypeToolSearchToolResult,
			"tool_use_id": toolUseID,
			"content": map[string]any{
				"type":       anthropic.BlockTypeToolSearchResultError,
				"error_code": errorCode,
			},
		},
		nil,
	)
}
