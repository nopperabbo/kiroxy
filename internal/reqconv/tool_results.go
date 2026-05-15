// This file is derived from github.com/d-kuro/kirocc
// Original commit: 5633c47f0d65aaef748728bae1c68160b0ea538d
// Copyright (c) 2026 d-kuro. Licensed under Apache License, Version 2.0.
// Modifications (c) 2026 kiroxy contributors.

package reqconv

import (
	"github.com/nopperabbo/kiroxy/internal/anthropic"
	"github.com/nopperabbo/kiroxy/internal/kiroproto"
)

// ExtractToolResults extracts tool_result blocks from message content and converts to Kiro format.
func ExtractToolResults(content anthropic.MessageContent) []kiroproto.ToolResult {
	if content.IsString() {
		return nil
	}
	var results []kiroproto.ToolResult
	for _, b := range content.Blocks {
		if !b.IsToolResult() {
			continue
		}
		status := kiroproto.ToolResultStatusSuccess
		if b.IsError {
			status = kiroproto.ToolResultStatusError
		}
		text := extractToolResultContentText(b)
		if text == "" {
			text = "(empty result)"
		}
		// v3 captures show kiro-cli uses exit_status/stdout/stderr format.
		exitStatus := "0"
		if b.IsError {
			exitStatus = "1"
		}
		results = append(results, kiroproto.ToolResult{
			ToolUseID: b.ToolUseID,
			Status:    status,
			Content: []kiroproto.ToolResultContent{{JSON: map[string]any{
				"exit_status": exitStatus,
				"stdout":      text,
				"stderr":      "",
			}}},
		})
	}
	return results
}

// ExtractToolUses extracts tool_use blocks from assistant message content and converts to Kiro format.
func ExtractToolUses(content anthropic.MessageContent) []kiroproto.HistoryToolUse {
	if content.IsString() {
		return nil
	}
	var toolUses []kiroproto.HistoryToolUse
	for _, b := range content.Blocks {
		if !b.IsToolUse() {
			continue
		}
		toolUses = append(toolUses, kiroproto.HistoryToolUse{
			ToolUseID: b.ID,
			Name:      b.Name,
			Input:     b.Input,
		})
	}
	return toolUses
}

// ReorderToolResults reorders tool results to match the order of tool_use IDs
// from the preceding assistant message. Results not found in toolUseIDs are appended at the end.
func ReorderToolResults(results []kiroproto.ToolResult, toolUseIDs []string) []kiroproto.ToolResult {
	if len(results) <= 1 || len(toolUseIDs) == 0 {
		return results
	}
	index := make(map[string]kiroproto.ToolResult, len(results))
	for _, r := range results {
		index[r.ToolUseID] = r
	}
	ordered := make([]kiroproto.ToolResult, 0, len(results))
	used := make(map[string]struct{}, len(results))
	for _, id := range toolUseIDs {
		if r, ok := index[id]; ok {
			ordered = append(ordered, r)
			used[id] = struct{}{}
		}
	}
	for _, r := range results {
		if _, ok := used[r.ToolUseID]; !ok {
			ordered = append(ordered, r)
		}
	}
	return ordered
}
