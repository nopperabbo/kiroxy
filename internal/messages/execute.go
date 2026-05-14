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
	"net"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/http2"

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

// capacityShortageBaseDelay is used when the upstream signals a server-side
// capacity shortage (INSUFFICIENT_MODEL_CAPACITY). Rotating to a different
// account does NOT help because every account hits the same Kiro model
// fleet; we slow down to give the upstream a chance to recover. ~2s,
// ~4s, ~8s with ±25% jitter, capping total worst-case rotation latency
// under ~14s for capacity events.
const capacityShortageBaseDelay = 2 * time.Second

// poolRotationBackoff returns an exponential backoff delay with ±25% jitter
// for the given rotation attempt number (0-indexed). attempt=0 returns 0
// because the first rotation is the response to the original failure and
// should fire immediately.
func poolRotationBackoff(attempt int) time.Duration {
	return rotationBackoffFor(attempt, poolRotationBaseDelay)
}

// rotationBackoffFor returns an exponential backoff delay with ±25% jitter,
// parameterized on a base delay so callers can pick a longer floor for
// upstream signals where rotation alone is insufficient (capacity shortage).
func rotationBackoffFor(attempt int, base time.Duration) time.Duration {
	if attempt <= 0 {
		return 0
	}
	d := base << (attempt - 1)
	jitter := time.Duration(rand.Int64N(int64(d)/2)) - d/4
	return d + jitter
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
//   - HTTP/2 / transport timeouts ("http2: timeout awaiting response headers",
//     "context deadline exceeded", "i/o timeout"). Without rotation here,
//     the caller would burn the entire request budget on the same account
//     reusing a half-broken HTTP/2 connection. A synthetic UpstreamError is
//     returned so callers can log uniform fields.
func isRotatableUpstreamError(err error) (*kiroclient.UpstreamError, bool) {
	var ue *kiroclient.UpstreamError
	if errors.As(err, &ue) {
		if kiroclient.IsRetryableAWSException(ue.Exception) {
			return ue, true
		}
		switch ue.Status {
		case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			return ue, true
		case http.StatusUnauthorized, http.StatusForbidden:
			// kiroclient already attempted token refresh and surrendered.
			// The refresh_token on this account is likely dead or the upstream
			// is rejecting it permanently. Rotate to a different account whose
			// credentials may still be valid.
			return ue, true
		}
		return ue, false
	}
	if classified, ok := classifyTransportError(err); ok {
		return &kiroclient.UpstreamError{Exception: classified}, true
	}
	return nil, false
}

// classifyTransportError detects HTTP/2 stream errors and transport timeouts
// using typed error matching (errors.As / errors.Is) rather than substring
// inspection. Returns a stable Exception tag for metric labeling.
//
// HTTP/2 categories handled per RFC 7540 §8.1.4 + GOAWAY semantics:
//   - REFUSED_STREAM: server explicitly didn't process the request, safe to retry.
//   - ENHANCE_YOUR_CALM: soft throttle — rotate to a different account/IP.
//   - GOAWAY: server is shutting down this connection. Streams with id >
//     LastStreamID are guaranteed unprocessed and safe to retry on a new conn.
//   - CANCEL / INTERNAL_ERROR / PROTOCOL_ERROR: stream-level abort, retry.
//
// Transport-layer fallbacks (net.Error.Timeout, context.DeadlineExceeded,
// http2.ErrCodeNo) cover stalled streams that don't surface a coded error.
func classifyTransportError(err error) (string, bool) {
	if err == nil {
		return "", false
	}
	var goAway http2.GoAwayError
	if errors.As(err, &goAway) {
		return "Http2:GOAWAY", true
	}
	var streamErr http2.StreamError
	if errors.As(err, &streamErr) {
		switch streamErr.Code {
		case http2.ErrCodeRefusedStream,
			http2.ErrCodeEnhanceYourCalm,
			http2.ErrCodeCancel,
			http2.ErrCodeInternal,
			http2.ErrCodeProtocol:
			return "Http2:" + streamErr.Code.String(), true
		}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "TransportTimeout", true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "TransportTimeout", true
	}
	return "", false
}

// isCapacityShortage reports whether the upstream error is a server-side
// capacity shortage that affects ALL accounts equally — not a per-account
// rate limit. Kiro decorates ThrottlingException with `reason` field:
//
//   - "INSUFFICIENT_MODEL_CAPACITY" — Kiro's model fleet is overloaded.
//     Hitting this on account A means account B will also hit it. Cooling
//     down accounts on this signal causes a stampede where the entire
//     pool is locked out for an hour while the upstream recovers in
//     minutes. Treat as TRANSIENT (rotate, but don't cooldown).
//
// Without this distinction, a 30-minute Kiro-side overload window
// converts every account in the pool to "quota-cooldown for 1h" — even
// though no account is actually rate-limited. Symptom: "akun yang error
// dan ga bisa di pake" reported by users.
func isCapacityShortage(ue *kiroclient.UpstreamError) bool {
	if ue == nil {
		return false
	}
	// Explicit reason field (Kiro-specific extension).
	if ue.Reason == "INSUFFICIENT_MODEL_CAPACITY" {
		return true
	}
	// Heuristic fallback: ThrottlingException with the canonical capacity
	// message but no `reason` field (older response shape).
	if ue.Exception == "ThrottlingException" &&
		strings.Contains(ue.Body, "experiencing high traffic") {
		return true
	}
	return false
}

// isQuotaFailure classifies an upstream error for the FailureRecorder. Quota
// failures (throttling / rate-limit / too-many-requests) should cool down the
// account immediately; everything else (5xx gateway, internal errors) is
// treated as transient and only cools down after repeated consecutive hits.
//
// Server-side capacity shortages (INSUFFICIENT_MODEL_CAPACITY) are explicitly
// EXCLUDED — they affect every account equally, so cooling down one account
// just shrinks the pool while leaving the actual problem unchanged.
func isQuotaFailure(ue *kiroclient.UpstreamError) bool {
	if isCapacityShortage(ue) {
		return false
	}
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
		if ue.Reason != "" {
			return "upstream_exception:" + ue.Exception + "/" + ue.Reason
		}
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
	callCtx := logging.WithAccountID(ctx, inv.creds.AccountID)
	apiResp, err := s.client.GenerateAssistantResponse(callCtx, inv.creds.AccessToken, inv.payload, inv.creds.Region)
	for poolAttempt := 0; err != nil && poolAttempt < upstreamPoolRetries; poolAttempt++ {
		ue, ok := isRotatableUpstreamError(err)
		if !ok {
			break
		}
		// Before rotating, tell the pool which account just failed so
		// subsequent Pick calls bias away from it. This is a hint — the
		// FailureRecorder interface is optional and a stale or unknown
		// account ID is tolerated by the implementation.
		//
		// Capacity shortages (INSUFFICIENT_MODEL_CAPACITY) are an exception:
		// the upstream model fleet is overloaded across all accounts. The
		// account did nothing wrong — bumping its consecutive-error counter
		// would falsely penalize healthy accounts during a regional Kiro
		// hiccup. Skip recording entirely; just rotate.
		if rec, ok := s.auth.(FailureRecorder); ok && inv.creds != nil && inv.creds.AccountID != "" && !isCapacityShortage(ue) {
			reason := rotationFailureReason(ue)
			rec.RecordFailure(inv.creds.AccountID, isQuotaFailure(ue), reason)
		}
		// Capacity shortages affect every account in the pool equally, so
		// rotation alone won't help. Slow the loop down to give the upstream
		// a chance to recover instead of burning the rotation budget at
		// network-RTT speed.
		base := poolRotationBaseDelay
		if isCapacityShortage(ue) {
			base = capacityShortageBaseDelay
		}
		if delay := rotationBackoffFor(poolAttempt, base); delay > 0 {
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
		callCtx = logging.WithAccountID(ctx, newCreds.AccountID)
		apiResp, err = s.client.GenerateAssistantResponse(callCtx, newCreds.AccessToken, inv.payload, newCreds.Region)
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
