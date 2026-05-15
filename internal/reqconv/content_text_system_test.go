// This file is derived from github.com/d-kuro/kirocc
// Original commit: 5633c47f0d65aaef748728bae1c68160b0ea538d
// Copyright (c) 2026 d-kuro. Licensed under Apache License, Version 2.0.
// Modifications (c) 2026 kiroxy contributors.

package reqconv

import (
	"testing"

	"github.com/nopperabbo/kiroxy/internal/anthropic"
)

func TestExtractSystemPrompt(t *testing.T) {
	tests := []struct {
		name   string
		prompt anthropic.SystemPrompt
		want   string
	}{
		{
			name:   "string",
			prompt: anthropic.SystemPrompt{Text: "You are helpful."},
			want:   "You are helpful.",
		},
		{
			name: "array",
			prompt: anthropic.SystemPrompt{
				Blocks: []anthropic.SystemBlock{
					{Type: "text", Text: "Part 1"},
					{Type: "text", Text: "Part 2"},
				},
			},
			want: "Part 1\nPart 2",
		},
		{
			name:   "empty",
			prompt: anthropic.SystemPrompt{},
			want:   "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractSystemPrompt(tt.prompt)
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}
