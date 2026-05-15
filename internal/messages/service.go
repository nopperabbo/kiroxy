// This file is derived from github.com/d-kuro/kirocc
// Original commit: 5633c47f0d65aaef748728bae1c68160b0ea538d
// Copyright (c) 2026 d-kuro. Licensed under Apache License, Version 2.0.
// Modifications (c) 2026 kiroxy contributors.

package messages

import (
	"context"

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

// Service owns message execution and token counting flows.
type Service struct {
	auth           TokenGetter
	client         kiroclient.Client
	captureEnabled bool

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
