// This file is derived from github.com/d-kuro/kirocc
// Original commit: 5633c47f0d65aaef748728bae1c68160b0ea538d
// Copyright (c) 2026 d-kuro. Licensed under Apache License, Version 2.0.
// Modifications (c) 2026 kiroxy contributors.

package reqconv

import (
	"github.com/google/uuid"
	"github.com/nopperabbo/kiroxy/internal/anthropic"
	"github.com/nopperabbo/kiroxy/internal/kiroproto"
)

// ExtractThinkingToolUses extracts thinking content blocks from assistant messages
// and converts them to Kiro thinking tool_use entries for history.
// Unlike regular tool_use, thinking tool results are NOT sent back to the upstream.
func ExtractThinkingToolUses(content anthropic.MessageContent) []kiroproto.HistoryToolUse {
	if content.IsString() {
		return nil
	}
	var toolUses []kiroproto.HistoryToolUse
	for _, b := range content.Blocks {
		if b.Type != anthropic.BlockTypeThinking || b.Thinking == "" {
			continue
		}
		id := "thinking_" + uuid.New().String()[:8]
		toolUses = append(toolUses, kiroproto.HistoryToolUse{
			ToolUseID: id,
			Name:      ThinkingToolName,
			Input:     map[string]any{"thought": b.Thinking},
		})
	}
	return toolUses
}
