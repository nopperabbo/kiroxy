// This file is derived from github.com/d-kuro/kirocc
// Original commit: 5633c47f0d65aaef748728bae1c68160b0ea538d
// Copyright (c) 2026 d-kuro. Licensed under Apache License, Version 2.0.
// Modifications (c) 2026 kiroxy contributors.

package tracing

import (
	"context"

	"go.opentelemetry.io/otel/trace"
)

// ExtractTraceID returns the OTel trace ID from the context as a 32-char hex string.
// Returns an empty string if no valid OTel span is present (e.g., when OTel is disabled).
func ExtractTraceID(ctx context.Context) string {
	sc := trace.SpanFromContext(ctx).SpanContext()
	if !sc.TraceID().IsValid() {
		return ""
	}
	return sc.TraceID().String()
}
