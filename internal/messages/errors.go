// This file is derived from github.com/d-kuro/kirocc
// Original commit: 5633c47f0d65aaef748728bae1c68160b0ea538d
// Copyright (c) 2026 d-kuro. Licensed under Apache License, Version 2.0.
// Modifications (c) 2026 kiroxy contributors.

package messages

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"local/kiroxy/internal/httpx"
	"local/kiroxy/internal/kiroclient"
)

// Re-exports of httpx error type constants so in-package callers stay concise.
const (
	errTypeInvalidRequest = httpx.ErrTypeInvalidRequest
	errTypeAPI            = httpx.ErrTypeAPI
	ErrTypeAuthentication = httpx.ErrTypeAuthentication
	errTypeStreamError    = httpx.ErrTypeStream
)

// retryableInvalidStateReasons are invalidStateEvent reasons that can be resolved
// by clearing the conversation ID and retrying.
var retryableInvalidStateReasons = map[string]struct{}{
	"CONTENT_LENGTH_EXCEEDS_THRESHOLD": {},
	"INVALID_CONVERSATION_STATE":       {},
	"STALE_CONVERSATION":               {},
}

// handleUpstreamError writes the appropriate error response for upstream failures.
// Returns a non-empty reason string if the error is retryable, or "" if a final error was written.
func handleUpstreamError(w http.ResponseWriter, isException bool, invalidReason string) string {
	if isException {
		httpx.WriteError(w, http.StatusBadGateway, errTypeAPI, "upstream exception")
		return ""
	}
	if _, ok := retryableInvalidStateReasons[invalidReason]; ok {
		return invalidReason
	}
	httpx.WriteError(w, http.StatusBadRequest, errTypeInvalidRequest, "invalid state: request rejected by upstream")
	return ""
}

// logUpstreamError logs a "kiro api error" with structured attributes when the
// error is an *UpstreamError. Falls back to plain err logging otherwise.
//
// Severity is chosen by HTTP status: 4xx (client/upstream-classified) is
// logged at WARN since the failure is not actionable by the operator and
// gets returned to the client as-is; 5xx (and non-HTTP transport failures)
// is logged at ERROR because it indicates real upstream instability or a
// proxy bug. This avoids polluting ERROR streams with mundane upstream
// 400s like UnknownOperationException that are entirely upstream-side.
func logUpstreamError(ctx context.Context, short string, err error, extra ...any) {
	attrs := []any{"trace_id", short, "err", err}
	attrs = append(attrs, extra...)
	level := slog.LevelError
	var ue *kiroclient.UpstreamError
	if errors.As(err, &ue) {
		attrs = append(attrs,
			"status", ue.Status,
			"content_type", ue.ContentType,
			"exception", ue.Exception,
		)
		if ue.Reason != "" {
			attrs = append(attrs, "reason", ue.Reason)
		}
		attrs = append(attrs, "body", ue.Body)
		if ue.Status >= 400 && ue.Status < 500 {
			level = slog.LevelWarn
		}
	}
	slog.Log(ctx, level, "kiro api error", attrs...)
}
