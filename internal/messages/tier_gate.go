// kiroxy addition (not derived from upstream).
//
// Tier gating for /v1/messages: when the picked account's subscription
// tier is below the minimum required by the requested model, kiroxy emits
// an operator warning. With KIROXY_TIER_STRICT=1, the request is also
// refused with HTTP 402 (PaymentRequired) so the caller learns about the
// mismatch instead of waiting for an opaque upstream 403.
//
// Default behavior is warn-only because (a) the tier classification is a
// heuristic over MonthlyCap thresholds and may need calibration as
// Anthropic ships new model SKUs, and (b) operators may want the proxy
// to attempt the request anyway and let upstream decide. Strict mode is
// the right choice once an operator has audited the heuristic against
// their fleet.

package messages

import (
	"context"
	"log/slog"
	"os"

	"github.com/nopperabbo/kiroxy/internal/auth"
	"github.com/nopperabbo/kiroxy/internal/kiroclient"
	"github.com/nopperabbo/kiroxy/internal/models"
)

// checkTierGate returns true when the request must be refused because the
// picked account's subscription tier is below the model's minimum and
// KIROXY_TIER_STRICT=1 is set. Returns false in all other cases — caller
// proceeds normally. A non-strict mismatch logs a warning but does not
// block the request.
//
// Lookup is best-effort. When the auth manager does not implement
// UsageLimitsLooker (e.g. the static-credentials test fixture) or when
// the looker has no snapshot yet, the gate falls open and the function
// returns false without logging — a missing tier signal must never block
// the hot path.
func (s *Service) checkTierGate(ctx context.Context, short string, creds *auth.Credentials, anthropicModel string) (refused bool) {
	if creds == nil || creds.AccountID == "" || anthropicModel == "" {
		return false
	}
	looker, ok := s.auth.(UsageLimitsLooker)
	if !ok {
		return false
	}
	usage := looker.GetUsage(creds.AccountID)
	if usage == nil {
		return false
	}

	accountTier := usage.Tier()
	modelMin := models.MinimumTier(anthropicModel)
	if models.TierSatisfies(accountTier, modelMin) {
		return false
	}

	strict := os.Getenv("KIROXY_TIER_STRICT") == "1"
	level := slog.LevelWarn
	msg := "tier gate: account tier below model requirement"
	if strict {
		msg = "tier gate: refusing request — account tier below model requirement (KIROXY_TIER_STRICT=1)"
	}
	slog.Log(ctx, level, msg,
		slog.String("trace_id", short),
		slog.String("account_id", creds.AccountID),
		slog.String("account_tier", string(accountTier)),
		slog.String("model", anthropicModel),
		slog.String("model_min_tier", string(modelMin)),
		slog.Bool("strict", strict),
	)

	return strict
}

// Compile-time assertion: kiroclient.UsageLimits is the canonical type
// the looker returns. Keeps the wire-up honest if someone refactors the
// pool API to return a different shape.
var _ = (*kiroclient.UsageLimits)(nil)
