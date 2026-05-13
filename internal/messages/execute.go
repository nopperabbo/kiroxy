// This file is derived from github.com/d-kuro/kirocc
// Original commit: 5633c47f0d65aaef748728bae1c68160b0ea538d
// Copyright (c) 2026 d-kuro. Licensed under Apache License, Version 2.0.
// Modifications (c) 2026 kiroxy contributors.

package messages

import (
	"context"
	"errors"
	"log/slog"
	"math/rand/v2"
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
// the upstream returns a retryable error before the first response byte has
// been written. Kept small so a genuine outage still fails fast.
const upstreamPoolRetries = 3

// poolRotationBaseDelay is the base inter-rotation backoff. We back off
// between rotations to avoid hammering the upstream when several accounts
// share the same regional capacity squeeze: the second rotation sleeps
// ~500ms, the third ~1s (with ±25% jitter). Total worst-case added latency
// stays under 2s for the common case where rotation #1 succeeds.
const poolRotationBaseDelay = 500 * time.Millisecond

// poolRotationBackoff returns an exponential backoff delay with ±25% jitter
// for the given rotation attempt number (0-indexed). attempt=0 returns 0
// because the first rotation is the response to the original failure and
// should fire immediately.
func poolRotationBackoff(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}
	base := poolRotationBaseDelay << (attempt - 1)
	jitter := time.Duration(rand.Int64N(int64(base)/2)) - base/4
	return base + jitter
}

// rotationSleep waits for delay, respecting ctx cancellation. Returns the
// ctx error if cancelled mid-wait.
func rotationSleep(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	t := time.NewTimer(delay)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// isRotatableUpstreamError returns true when an error from the kiroclient is
// worth rotating to a different pool account. Covers:
//
//   - Retryable AWS exception types (ThrottlingException, InternalServer*,
//     ServiceUnavailable*, TooManyRequests) regardless of HTTP status.
//   - Raw HTTP 502 / 503 / 504 from the Bedrock gateway. These can arrive
//     with an empty or unparseable body (no AWS exception type), which
//     IsRetryableAWSException alone would miss. Treating them as rotatable
//     captures transient upstream gateway hiccups where a different account
//     routed through a different edge is likely to succeed.
func isRotatableUpstreamError(err error) (*kiroclient.UpstreamError, bool) {
	var ue *kiroclient.UpstreamError
	if !errors.As(err, &ue) {
		return nil, false
	}
	if kiroclient.IsRetryableAWSException(ue.Exception) {
		return ue, true
	}
	switch ue.Status {
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return ue, true
	}
	return ue, false
}

// isQuotaFailure classifies an upstream error for the FailureRecorder. Quota
// failures (throttling / rate-limit / too-many-requests) should cool down the
// account immediately; everything else (5xx gateway, internal errors) is
// treated as transient and only cools down after repeated consecutive hits.
func isQuotaFailure(ue *kiroclient.UpstreamError) bool {
	switch ue.Exception {
	case "ThrottlingException", "TooManyRequestsException":
		return true
	}
	return ue.Status == http.StatusTooManyRequests
}

// rotationFailureReason builds a short, structured reason string suitable for
// the pool's cooldown logs and metrics. Kept concise so the pool's bounded
// reason field doesn't truncate useful info.
func rotationFailureReason(ue *kiroclient.UpstreamError) string {
	if ue.Exception != "" {
		return "upstream_exception:" + ue.Exception
	}
	if ue.Status != 0 {
		return "upstream_status:" + http.StatusText(ue.Status)
	}
	return "upstream_error"
}

// callAndHandle performs one upstream call for the invocation and streams or
// buffers the response to w. Returns a non-empty reason if the request failed
// with a retryable invalidStateEvent before any bytes were written to w.
//
// When the upstream returns a retryable AWS exception (ThrottlingException,
// ServiceUnavailableException, ...) or a raw HTTP 502/503/504 gateway error
// and nothing has been written to w yet, we rotate to a fresh account from
// the pool and retry up to upstreamPoolRetries times. This makes
// INSUFFICIENT_MODEL_CAPACITY on one account transparent to the caller as
// long as another account in the pool has capacity.
func (s *Service) callAndHandle(ctx context.Context, w http.ResponseWriter, inv *invocation, attempt int) string {
	_, short := logging.TraceIDs(ctx)
	capture := newUpstreamAttemptCapture(ctx, s.captureEnabled, inv.payload, inv.model, inv.thinking, inv.req.Stream, attempt)

	upstreamStart := time.Now()
	apiResp, err := s.client.GenerateAssistantResponse(ctx, inv.creds.AccessToken, inv.payload, inv.creds.Region)
	for poolAttempt := 0; err != nil && poolAttempt < upstreamPoolRetries; poolAttempt++ {
		ue, ok := isRotatableUpstreamError(err)
		if !ok {
			break
		}
		// Before rotating, tell the pool which account just failed so
		// subsequent Pick calls bias away from it. This is a hint — the
		// FailureRecorder interface is optional and a stale or unknown
		// account ID is tolerated by the implementation.
		if rec, ok := s.auth.(FailureRecorder); ok && inv.creds != nil && inv.creds.AccountID != "" {
			reason := rotationFailureReason(ue)
			rec.RecordFailure(inv.creds.AccountID, isQuotaFailure(ue), reason)
		}
		if delay := poolRotationBackoff(poolAttempt); delay > 0 {
			if waitErr := rotationSleep(ctx, delay); waitErr != nil {
				slog.WarnContext(ctx, "kiro: pool rotation aborted by ctx",
					"trace_id", short, "err", waitErr,
					"pool_attempt", poolAttempt+1, "max", upstreamPoolRetries)
				break
			}
		}
		newCreds, tokErr := s.auth.GetToken(ctx)
		if tokErr != nil {
			slog.WarnContext(ctx, "kiro: pool rotation token fetch failed",
				"trace_id", short, "err", tokErr,
				"pool_attempt", poolAttempt+1, "max", upstreamPoolRetries)
			break
		}
		slog.WarnContext(ctx, "kiro: rotating account after retryable upstream error",
			"trace_id", short, "exception", ue.Exception, "status", ue.Status,
			"pool_attempt", poolAttempt+1, "max", upstreamPoolRetries,
			"prev_account_id", inv.creds.AccountID,
			"new_account_id", newCreds.AccountID,
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
