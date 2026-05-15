// This file is derived from github.com/d-kuro/kirocc
// Original commit: 5633c47f0d65aaef748728bae1c68160b0ea538d
// Copyright (c) 2026 d-kuro. Licensed under Apache License, Version 2.0.
// Modifications (c) 2026 kiroxy contributors.

package reqconv

import (
	"testing"

	"github.com/nopperabbo/kiroxy/internal/anthropic"
	"github.com/nopperabbo/kiroxy/internal/kiroproto"
)

func TestApplyToolCachePoints_WithCacheControl(t *testing.T) {
	tools := []anthropic.Tool{
		{Name: "a", CacheControl: &anthropic.CacheControl{Type: "ephemeral"}},
		{Name: "b"},
	}
	entries := []kiroproto.ToolEntry{
		{ToolSpecification: &kiroproto.ToolSpecification{Name: "a"}},
		{ToolSpecification: &kiroproto.ToolSpecification{Name: "b"}},
	}
	got := ApplyToolCachePoints(tools, entries)
	if len(got) != 3 {
		t.Fatalf("got %d entries, want 3", len(got))
	}
	if got[0].ToolSpecification == nil || got[0].ToolSpecification.Name != "a" {
		t.Fatal("first should be tool a")
	}
	if got[1].CachePoint == nil || got[1].CachePoint.Type != "default" {
		t.Fatal("second should be cachePoint")
	}
	if got[2].ToolSpecification == nil || got[2].ToolSpecification.Name != "b" {
		t.Fatal("third should be tool b")
	}
}

func TestApplyToolCachePoints_NoCacheControl(t *testing.T) {
	tools := []anthropic.Tool{{Name: "a"}, {Name: "b"}}
	entries := []kiroproto.ToolEntry{
		{ToolSpecification: &kiroproto.ToolSpecification{Name: "a"}},
		{ToolSpecification: &kiroproto.ToolSpecification{Name: "b"}},
	}
	got := ApplyToolCachePoints(tools, entries)
	if len(got) != 2 {
		t.Fatalf("got %d entries, want 2", len(got))
	}
}
