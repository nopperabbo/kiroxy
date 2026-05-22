// This file is derived from github.com/d-kuro/kirocc
// Original commit: 5633c47f0d65aaef748728bae1c68160b0ea538d
// Copyright (c) 2026 d-kuro. Licensed under Apache License, Version 2.0.
// Modifications (c) 2026 kiroxy contributors.

package messages

import (
	"context"
	"time"

	"github.com/nopperabbo/kiroxy/internal/auth"
	"github.com/nopperabbo/kiroxy/internal/kiroclient"
	"github.com/nopperabbo/kiroxy/internal/metrics"
)

// TokenGetter loads valid upstream credentials for a request.
type TokenGetter interface {
	GetToken(ctx context.Context) (*auth.Credentials, error)
}

// FailureRecorder is an optional interface that a TokenGetter may implement
// to receive notifications when an upstream call fails for a specific
// account. Implementations should mark the account as cooled down (when
// quota=true) or transient-failed (quota=false) so subsequent token fetches
// bias away from it. A no-op implementation is acceptable; the rotation
// logic only treats RecordFailure as a hint, not a guarantee.
type FailureRecorder interface {
	RecordFailure(accountID string, quota bool, reason string)
}

// StructuralRecorder is an optional interface for marking an account as
// broken at the API contract level. Distinct from FailureRecorder: a
// structural error signals the account's request shape is fundamentally
// incompatible with the upstream (UnknownOperationException, AccessDenied,
// migrated/deprecated account, missing metadata) and will NOT recover via
// retry or rotation. Implementations should apply the maximum cooldown so
// the account stops pulling traffic until an operator inspects it.
type StructuralRecorder interface {
	RecordStructuralError(accountID string, reason string)
}

// UsageLimitsLooker is an optional interface that lets the messages handler
// read the most recent UsageLimits snapshot for a picked account without
// importing the pool package directly. The pool's UsagePoller writes
// snapshots and this read is best-effort: callers must accept nil (no
// snapshot yet, or polling disabled) and never block the request hot path
// on a missing tier signal.
type UsageLimitsLooker interface {
	GetUsage(accountID string) *kiroclient.UsageLimits
}

// LatencyRecorder is an optional interface for feeding per-request latency
// into the pool's EWMA health tracker. Implementations should be cheap
// (no I/O); the call happens on the request goroutine after the full
// response has been streamed.
type LatencyRecorder interface {
	RecordLatency(accountID string, latency time.Duration)
}

// Service owns message execution and token counting flows.
type Service struct {
	auth           TokenGetter
	client         kiroclient.Client
	captureEnabled bool
	requestTimeout time.Duration

	// metrics is the nil-safe metric sink. A nil value disables emission
	// on every instrumentation call; see internal/metrics.Sink.
	metrics *metrics.Sink
}

// Option configures a Service.
type Option func(*Service)

// WithCapture enables recording of full upstream request/response bodies on
// failure for debugging. Defaults to disabled; callers should enable it only
// when debug logging is on.
func WithCapture(enabled bool) Option {
	return func(s *Service) { s.captureEnabled = enabled }
}

// WithRequestTimeout sets the per-request context deadline.
func WithRequestTimeout(d time.Duration) Option {
	return func(s *Service) { s.requestTimeout = d }
}

// New constructs a message service.
func New(authMgr TokenGetter, client kiroclient.Client, opts ...Option) *Service {
	s := &Service{
		auth:   authMgr,
		client: client,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}
