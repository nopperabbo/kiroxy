package main

import (
	"testing"
	"time"

	"github.com/nopperabbo/kiroxy/internal/tokenvault"
)

func TestToInboundKeyView_Basic(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	k := tokenvault.InboundKey{
		ID:        "key-123",
		Label:     "test-key",
		Tail:      "abcd",
		Revoked:   false,
		CreatedAt: now,
	}
	v := toInboundKeyView(k)
	if v.ID != "key-123" {
		t.Errorf("ID = %q", v.ID)
	}
	if v.Label != "test-key" {
		t.Errorf("Label = %q", v.Label)
	}
	if v.Tail != "abcd" {
		t.Errorf("Tail = %q", v.Tail)
	}
	if v.Revoked {
		t.Error("Revoked should be false")
	}
	if v.CreatedAt != "2025-01-15T10:30:00Z" {
		t.Errorf("CreatedAt = %q", v.CreatedAt)
	}
	if v.LastUsedAt != "" {
		t.Errorf("LastUsedAt should be empty, got %q", v.LastUsedAt)
	}
}

func TestToInboundKeyView_WithLastUsed(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	used := time.Date(2025, 1, 16, 12, 0, 0, 0, time.UTC)
	k := tokenvault.InboundKey{
		ID:         "key-456",
		Label:      "used-key",
		Tail:       "wxyz",
		Revoked:    true,
		CreatedAt:  now,
		LastUsedAt: used,
	}
	v := toInboundKeyView(k)
	if !v.Revoked {
		t.Error("Revoked should be true")
	}
	if v.LastUsedAt != "2025-01-16T12:00:00Z" {
		t.Errorf("LastUsedAt = %q", v.LastUsedAt)
	}
}

func TestNewInboundKeyProvider_Nil(t *testing.T) {
	p := newInboundKeyProvider(nil)
	if p != nil {
		t.Error("expected nil for nil vault")
	}
}

func TestRandomID_Format(t *testing.T) {
	id := randomID()
	if len(id) != 8 {
		t.Errorf("randomID length = %d, want 8", len(id))
	}
	// Should be valid hex
	for _, c := range id {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("non-hex char %c in randomID %q", c, id)
		}
	}
}

func TestRandomID_Unique(t *testing.T) {
	a := randomID()
	b := randomID()
	if a == b {
		t.Error("two randomID calls should produce different values")
	}
}
