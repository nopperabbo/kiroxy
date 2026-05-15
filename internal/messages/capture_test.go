// This file is derived from github.com/d-kuro/kirocc
// Original commit: 5633c47f0d65aaef748728bae1c68160b0ea538d
// Copyright (c) 2026 d-kuro. Licensed under Apache License, Version 2.0.
// Modifications (c) 2026 kiroxy contributors.

package messages

import (
	"context"
	"testing"

	"github.com/nopperabbo/kiroxy/internal/kiroproto"
)

func TestNewUpstreamAttemptCapture_DisabledReturnsNil(t *testing.T) {
	payload := &kiroproto.Payload{}
	got := newUpstreamAttemptCapture(context.Background(), false, payload, "model", false, true, 1)
	if got != nil {
		t.Errorf("expected nil when disabled, got %+v", got)
	}
}

func TestNewUpstreamAttemptCapture_EnabledReturnsNonNil(t *testing.T) {
	payload := &kiroproto.Payload{}
	got := newUpstreamAttemptCapture(context.Background(), true, payload, "model", false, true, 1)
	if got == nil {
		t.Fatal("expected non-nil capture when enabled")
	}
}
