// This file is derived from github.com/d-kuro/kirocc
// Original commit: 5633c47f0d65aaef748728bae1c68160b0ea538d
// Copyright (c) 2026 d-kuro. Licensed under Apache License, Version 2.0.
// Modifications (c) 2026 kiroxy contributors.

package reqconv

import (
	"local/kiroxy/internal/anthropic"
	"local/kiroxy/internal/kiroproto"
)

// ApplyToolCachePoints inserts cachePoint entries into the tools array
// after tools that have cache_control set.
func ApplyToolCachePoints(tools []anthropic.Tool, entries []kiroproto.ToolEntry) []kiroproto.ToolEntry {
	if len(tools) == 0 {
		return entries
	}
	var result []kiroproto.ToolEntry
	entryIdx := 0
	for _, t := range tools {
		if entryIdx < len(entries) {
			result = append(result, entries[entryIdx])
			entryIdx++
		}
		if t.CacheControl != nil {
			result = append(result, kiroproto.ToolEntry{
				CachePoint: &kiroproto.CachePoint{Type: "default"},
			})
		}
	}
	// Append any remaining entries (shouldn't happen normally).
	for ; entryIdx < len(entries); entryIdx++ {
		result = append(result, entries[entryIdx])
	}
	return result
}
