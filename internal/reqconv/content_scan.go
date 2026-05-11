// This file is derived from github.com/d-kuro/kirocc
// Original commit: 5633c47f0d65aaef748728bae1c68160b0ea538d
// Copyright (c) 2026 d-kuro. Licensed under Apache License, Version 2.0.
// Modifications (c) 2026 kiroxy contributors.

package reqconv

import (
	"log/slog"
	"strings"

	"local/kiroxy/internal/anthropic"
	"local/kiroxy/internal/kiroproto"
)

// scanCurrentMessage walks message content once and extracts tool_results and
// images. Replaces the former pattern of calling ExtractToolResults and
// ExtractImages separately, which scanned the block list twice.
func scanCurrentMessage(content anthropic.MessageContent) (toolResults []kiroproto.ToolResult, images []kiroproto.Image) {
	if content.IsString() {
		return nil, nil
	}
	for _, b := range content.Blocks {
		switch {
		case b.IsToolResult():
			status := kiroproto.ToolResultStatusSuccess
			exitStatus := "0"
			if b.IsError {
				status = kiroproto.ToolResultStatusError
				exitStatus = "1"
			}
			text := extractToolResultContentText(b)
			if text == "" {
				text = "(empty result)"
			}
			toolResults = append(toolResults, kiroproto.ToolResult{
				ToolUseID: b.ToolUseID,
				Status:    status,
				Content: []kiroproto.ToolResultContent{{JSON: map[string]any{
					"exit_status": exitStatus,
					"stdout":      text,
					"stderr":      "",
				}}},
			})
		case b.Type == anthropic.BlockTypeImage && b.Source != nil:
			if b.Source.Type != "base64" {
				slog.Warn("skipping non-base64 image source type", "type", b.Source.Type)
				continue
			}
			format := b.Source.MediaType
			if idx := strings.LastIndex(format, "/"); idx >= 0 {
				format = format[idx+1:]
			}
			images = append(images, kiroproto.Image{
				Format: format,
				Source: kiroproto.ImageSource{Bytes: b.Source.Data},
			})
		}
	}
	return toolResults, images
}
