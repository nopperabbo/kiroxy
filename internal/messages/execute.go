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
	"time"

	"local/kiroxy/internal/anthropic"
	"local/kiroxy/internal/auth"
	"local/kiroxy/internal/httpx"
	"local/kiroxy/internal/kiroclient"
	"local/kiroxy/internal/kiroproto"
	"local/kiroxy/internal/logging"
	"local/kiroxy/internal/metrics"
)

// invocation bundles everything callAndHandle needs for one upstream attempt.
// Replaces the former 11-argument callAndHandle signature.
type invocation struct {
	req               *anthropic.Request
	payload           *kiroproto.Payload
	creds             *auth.Credentials
	model             string
	responseModel     string
	contextWindowSize int
	thinking          bool
	toolNameMap       map[string]string
	metrics           *requestMetrics
}

// upstreamPoolRetries caps how many different accounts we rotate through when
// the upstream returns a retryable AWS exception before the first response
// byte has been written. Kept small so a genuine outage still fails fast.
const upstreamPoolRetries = 3

// callAndHandle performs one upstream call for the invocation and streams or
// buffers the response to w. Returns a non-empty reason if the request failed
// with a retryable invalidStateEvent before any bytes were written to w.
//
// When the upstream returns a retryable AWS exception (ThrottlingException,
// ServiceUnavailableException, ...) and nothing has been written to w yet, we
// rotate to a fresh account from the pool and retry up to upstreamPoolRetries
// times. This makes INSUFFICIENT_MODEL_CAPACITY on one account transparent to
// the caller as long as another account in the pool has capacity.
func (s *Service) callAndHandle(ctx context.Context, w http.ResponseWriter, inv *invocation, attempt int) string {
	_, short := logging.TraceIDs(ctx)
	capture := newUpstreamAttemptCapture(ctx, s.captureEnabled, inv.payload, inv.model, inv.thinking, inv.req.Stream, attempt)

	upstreamStart := time.Now()
	apiResp, err := s.client.GenerateAssistantResponse(ctx, inv.creds.AccessToken, inv.payload, inv.creds.Region)
	for poolAttempt := 0; err != nil && poolAttempt < upstreamPoolRetries; poolAttempt++ {
		var ue *kiroclient.UpstreamError
		if !errors.As(err, &ue) || !kiroclient.IsRetryableAWSException(ue.Exception) {
			break
		}
		newCreds, tokErr := s.auth.GetToken(ctx)
		if tokErr != nil {
			slog.WarnContext(ctx, "kiro: pool rotation token fetch failed",
				"trace_id", short, "err", tokErr,
				"pool_attempt", poolAttempt+1, "max", upstreamPoolRetries)
			break
		}
		slog.WarnContext(ctx, "kiro: rotating account after retryable upstream exception",
			"trace_id", short, "exception", ue.Exception,
			"pool_attempt", poolAttempt+1, "max", upstreamPoolRetries,
			"new_client_id", newCreds.ClientID)
		inv.creds = newCreds
		inv.payload.ProfileARN = newCreds.ProfileARN
		apiResp, err = s.client.GenerateAssistantResponse(ctx, newCreds.AccessToken, inv.payload, newCreds.Region)
	}
	if err != nil {
		logUpstreamError(ctx, short, err)
		inv.metrics.errKind(metrics.RequestKindUpstream)
		httpx.WriteError(w, http.StatusBadGateway, errTypeAPI, "upstream API error")
		return ""
	}
	// Observe time-to-first-byte (header + TCP round-trip only, not body);
	// apiResp returns as soon as status+headers are in, so this captures
	// exactly what we want. Only emitted on the first successful attempt.
	inv.metrics.observeTTFB(upstreamStart)
	body := apiResp.Body
	defer func() { _ = body.Close() }()
	if capture != nil {
		capture.setResponseHeaders(apiResp.Header)
	}

	var reason string
	if inv.req.Stream {
		reason = s.handleStreamingResponse(ctx, w, apiResp, inv.responseModel, inv.contextWindowSize, inv.req.StopSequences, inv.req.MaxTokens, apiResp.PromptTokens, capture, inv.toolNameMap, inv.metrics)
	} else {
		reason = s.handleNonStreamingResponse(ctx, w, apiResp, inv.responseModel, inv.contextWindowSize, inv.req.StopSequences, inv.req.MaxTokens, apiResp.PromptTokens, capture, inv.toolNameMap, inv.metrics)
	}
	if reason == retryReasonEmptyVisibleEndTurn {
		capture.logCapture(ctx, reason)
	}
	return reason
}

// executeWithRetry runs the invocation and handles retryable invalidStateEvent
// responses by clearing ConversationID and attempting once more. Terminal error
// responses are written to w and the function returns.
func (s *Service) executeWithRetry(ctx context.Context, w http.ResponseWriter, inv *invocation) {
	_, short := logging.TraceIDs(ctx)

	reason := s.callAndHandle(ctx, w, inv, 1)
	if reason == "" {
		return
	}

	slog.WarnContext(ctx, "retrying upstream request",
		"trace_id", short,
		"reason", reason,
	)
	// Clear conversation ID to break out of stuck state (empty_visible_end_turn
	// or retryable invalidStateEvent like CONTENT_LENGTH_EXCEEDS_THRESHOLD).
	inv.payload.ConversationState.ConversationID = ""

	reason2 := s.callAndHandle(ctx, w, inv, 2)
	if reason2 == "" {
		return
	}
	if reason2 == retryReasonEmptyVisibleEndTurn {
		slog.ErrorContext(ctx, "retry also returned empty visible end_turn",
			"trace_id", short, "reason", reason2)
		inv.metrics.errKind(metrics.RequestKindUpstream)
		httpx.WriteError(w, http.StatusBadGateway, errTypeAPI, "upstream returned empty response")
		return
	}
	// Retry ended with a different (final) error — report it as invalid state.
	slog.ErrorContext(ctx, "retry failed",
		"trace_id", short, "first_reason", reason, "second_reason", reason2)
	inv.metrics.errKind(metrics.RequestKindUpstream)
	httpx.WriteError(w, http.StatusBadRequest, errTypeInvalidRequest, "invalid state: "+reason2)
}
