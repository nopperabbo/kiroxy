package metrics

import (
	"net/http"
	"strconv"
	"time"
)

// Sink is the call-site-facing handle on a Registry. Unlike Registry itself,
// a nil *Sink is valid and every method on it is a no-op — this is the
// central mechanism that keeps instrumentation calls in hot paths free of
// "if sink != nil" wrappers.
//
// Safe for concurrent use.
type Sink struct {
	reg *Registry
}

// NopSink returns a no-op Sink. Use in tests or when metrics are disabled.
// Equivalent to (*Sink)(nil) but clearer at call sites.
func NopSink() *Sink { return nil }

// RequestKind classifies a /v1/messages outcome for the errors counter.
// Values are stable for Prometheus labels.
type RequestKind string

const (
	// RequestKindUpstream: Kiro/AWS returned a non-2xx or malformed stream.
	RequestKindUpstream RequestKind = "upstream"
	// RequestKindAuth: inbound auth failed (bad API key) OR token getter
	// could not produce credentials.
	RequestKindAuth RequestKind = "auth"
	// RequestKindProxy: internal kiroxy error (payload build, marshal, I/O).
	RequestKindProxy RequestKind = "proxy"
	// RequestKindInvalidRequest: client sent a malformed request body.
	RequestKindInvalidRequest RequestKind = "invalid_request"
)

// RefreshKind classifies when a refresh attempt was triggered.
type RefreshKind string

const (
	// RefreshKindProactive: a pre-expiry refresh triggered by expires_at skew.
	RefreshKindProactive RefreshKind = "proactive"
	// RefreshKindReactive: a refresh triggered by an upstream 401/403.
	RefreshKindReactive RefreshKind = "reactive"
)

// RefreshResult is the terminal outcome of a refresh attempt.
type RefreshResult string

const (
	// RefreshResultSuccess: new access_token committed to vault.
	RefreshResultSuccess RefreshResult = "success"
	// RefreshResultFail401: upstream returned 401/403 and refresh_token is dead.
	RefreshResultFail401 RefreshResult = "fail_401"
	// RefreshResultFailTransient: transient failure after all retries exhausted.
	RefreshResultFailTransient RefreshResult = "fail_transient"
	// RefreshResultFailOther: unclassified error (vault write, malformed response).
	RefreshResultFailOther RefreshResult = "fail_other"
)

// CooldownReason classifies why an account entered a cooldown.
type CooldownReason string

const (
	// CooldownReasonQuota: upstream returned 429 / quota-exhausted.
	CooldownReasonQuota CooldownReason = "quota"
	// CooldownReasonConsecutiveErrors: the per-account error threshold was
	// crossed (transient errors accumulated).
	CooldownReasonConsecutiveErrors CooldownReason = "consecutive_errors"
	// CooldownReasonUnauthorized: refresh_token is dead; account cannot
	// self-heal and was disabled until manual rotation.
	CooldownReasonUnauthorized CooldownReason = "unauthorized"
	// CooldownReasonManual: operator-initiated disable via dashboard / CLI.
	CooldownReasonManual CooldownReason = "manual"
	// CooldownReasonStructural: upstream returned a structural API error
	// (e.g. UnknownOperationException, ValidationException) indicating the
	// request shape or account credentials are incompatible with the
	// upstream contract. These errors do NOT recover via retry/rotate —
	// the account needs operator attention (re-onboard, fix metadata, or
	// remove from pool). 24h cooldown lets the operator see the dashboard
	// signal without the account being silently broken forever.
	CooldownReasonStructural CooldownReason = "structural"
)

// ObserveRequest records one completed /v1/messages invocation.
// Model is the CANONICAL API id (the one returned to the client, e.g.
// "claude-sonnet-4-6"), NOT the upstream Kiro SKU. Status is the HTTP
// status code written to the client.
//
// Safe to call on nil Sink.
func (s *Sink) ObserveRequest(model string, status int, stream bool, duration time.Duration) {
	if s == nil || s.reg == nil {
		return
	}
	modelLabel := normaliseModel(model)
	streamLabel := strconv.FormatBool(stream)
	statusClass := classifyStatus(status)
	s.reg.requestsTotal.WithLabelValues(modelLabel, statusClass, streamLabel).Inc()
	s.reg.requestDuration.WithLabelValues(modelLabel, streamLabel).Observe(duration.Seconds())
}

// RequestError increments the error counter for the given error kind.
// Always paired with an ObserveRequest (with the actual HTTP status); this
// counter gives operators a way to distinguish error TYPE across the same
// status-class bucket.
//
// Safe to call on nil Sink.
func (s *Sink) RequestError(kind RequestKind) {
	if s == nil || s.reg == nil {
		return
	}
	s.reg.requestErrorsTotal.WithLabelValues(string(kind)).Inc()
}

// ObserveUpstreamTTFB records time-to-first-byte from the Kiro upstream.
//
// Safe to call on nil Sink.
func (s *Sink) ObserveUpstreamTTFB(model string, d time.Duration) {
	if s == nil || s.reg == nil {
		return
	}
	s.reg.upstreamTTFB.WithLabelValues(normaliseModel(model)).Observe(d.Seconds())
}

// ObserveTokens records per-request token usage. Zero values are allowed
// and still observed (they're meaningful — zero output means a failed or
// empty generation).
//
// Safe to call on nil Sink.
func (s *Sink) ObserveTokens(model string, input, output int) {
	if s == nil || s.reg == nil {
		return
	}
	m := normaliseModel(model)
	if input > 0 {
		s.reg.tokensInput.WithLabelValues(m).Observe(float64(input))
	}
	if output > 0 {
		s.reg.tokensOutput.WithLabelValues(m).Observe(float64(output))
	}
}

// RefreshAttempt records the terminal result of a token refresh flow.
//
// Safe to call on nil Sink.
func (s *Sink) RefreshAttempt(kind RefreshKind, result RefreshResult) {
	if s == nil || s.reg == nil {
		return
	}
	s.reg.refreshAttemptsTotal.WithLabelValues(string(kind), string(result)).Inc()
}

// Cooldown records that an account entered a cooldown. Callers should emit
// this exactly once per transition (not once per failing request).
//
// Safe to call on nil Sink.
func (s *Sink) Cooldown(reason CooldownReason) {
	if s == nil || s.reg == nil {
		return
	}
	s.reg.cooldownsTotal.WithLabelValues(string(reason)).Inc()
}

// Handler is a pass-through to Registry.Handler that's safe on a nil Sink
// (returns a 503 handler).
func (s *Sink) Handler() http.Handler {
	if s == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "metrics registry not configured", http.StatusServiceUnavailable)
		})
	}
	return s.reg.Handler()
}

// classifyStatus maps an HTTP status code to a coarse class label used on
// the requests_total counter. Exact codes are intentionally NOT used — that
// would create a cardinality hazard as new codes appear.
func classifyStatus(status int) string {
	switch {
	case status >= 200 && status < 300:
		return "2xx"
	case status >= 300 && status < 400:
		return "3xx"
	case status >= 400 && status < 500:
		return "4xx"
	case status >= 500 && status < 600:
		return "5xx"
	default:
		return "other"
	}
}

// normaliseModel collapses empty/unknown model strings to a fixed "unknown"
// label so cardinality stays bounded when a request bypasses normal model
// resolution (e.g. auth-failure path writes without resolving the model).
func normaliseModel(m string) string {
	if m == "" {
		return "unknown"
	}
	return m
}
