// This file is derived from github.com/d-kuro/kirocc
// Original commit: 5633c47f0d65aaef748728bae1c68160b0ea538d
// Copyright (c) 2026 d-kuro. Licensed under Apache License, Version 2.0.
// Modifications (c) 2026 kiroxy contributors.

package respconv

import (
	"testing"
)

func TestComputeDelta(t *testing.T) {
	tests := []struct {
		name     string
		chunk    string
		previous string
		want     string
	}{
		{"empty previous", "Hello", "", "Hello"},
		{"same text", "Hello", "Hello", ""},
		{"prefix extension", "Hello world", "Hello", " world"},
		{"shrunk text", "Hel", "Hello", ""},
		{"overlap", "world!", "Hello world", "!"},
		{"no overlap", "Goodbye", "Hello", "Goodbye"},
		{"empty chunk", "", "Hello", ""},
		// Multi-byte UTF-8 overlap: previous ends with "日本", chunk starts with
		// "日本". Overlap detection must match all 6 bytes of "日本" (not only
		// the rune-start bytes) or the resulting delta begins mid-rune and
		// emits invalid UTF-8 into the SSE stream.
		{"multibyte overlap", "日本語のテスト", "こんにちは日本", "語のテスト"},
		{"multibyte overlap kanji only", "日本語", "日本", "語"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputeDelta(tt.chunk, tt.previous)
			if got != tt.want {
				t.Errorf("ComputeDelta(%q, %q) = %q, want %q", tt.chunk, tt.previous, got, tt.want)
			}
		})
	}
}
