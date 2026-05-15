// kiroxy addition (not derived from upstream).
//
// Tier requirements for Anthropic models routed through Kiro upstream.
// AWS Q Developer subscription tiers (Free / Pro / Pro+ / Power) gate which
// model SKUs an account is allowed to invoke. Free accounts can call Haiku
// but are blocked from Sonnet/Opus; Pro accounts can call all current SKUs
// but with a 1,000 invocation/month cap; Pro+/Power add overage capacity
// but no new model access.
//
// kiroxy uses these requirements for two purposes:
//
//  1. Pre-flight check at the request hot path. When the picked account's
//     subscription tier is below the model's requirement, kiroxy emits an
//     operator warning (and optionally refuses with HTTP 402 when
//     KIROXY_TIER_STRICT=1) instead of letting Kiro return an opaque 403.
//
//  2. Pool weighting. Future enhancement: bias Pick away from Free accounts
//     for Pro-only models so the per-request hot-path warning rarely fires.
//
// Source: live probe of /getUsageLimits across 77 social accounts confirmed
// the tier vocabulary (Q_DEVELOPER_STANDALONE_FREE / PRO / PRO_PLUS / POWER)
// and the cap thresholds (≤100 / ≤1500 / ≤5000 / else). Per-model gating
// is derived from the AWS Q Developer pricing page (verified 2026-05-15)
// plus empirical observation: Haiku invocations from Free accounts succeed,
// Opus invocations from Free accounts return 403 with INSUFFICIENT_SUBSCRIPTION.

package models

import (
	"strings"

	"github.com/nopperabbo/kiroxy/internal/kiroclient"
)

// MinimumTier reports the minimum subscription tier required to invoke the
// given Anthropic model alias. Unknown or empty model strings return
// SubscriptionTierUnknown so the caller falls through to upstream-side
// validation rather than emitting a spurious warning.
//
// Classification rules (based on AWS Q Developer model gating):
//   - Haiku (any version): Free tier allowed.
//   - Sonnet (any version): Pro tier required.
//   - Opus (any version): Pro tier required.
//   - Anything else: Unknown (upstream decides).
//
// We deliberately do NOT distinguish Pro vs Pro+/Power here because those
// tiers differ only in overage allowance, not model access — so the gate
// is "Pro or higher", and any Free→non-Haiku request is the only failure
// mode worth warning about today. If Anthropic introduces a Power-only
// model in the future, extend this function (do not gate elsewhere).
func MinimumTier(anthropicModel string) kiroclient.SubscriptionTier {
	if anthropicModel == "" {
		return kiroclient.SubscriptionTierUnknown
	}
	lower := strings.ToLower(anthropicModel)
	switch {
	case strings.Contains(lower, "haiku"):
		return kiroclient.SubscriptionTierFree
	case strings.Contains(lower, "sonnet"), strings.Contains(lower, "opus"):
		return kiroclient.SubscriptionTierPro
	default:
		return kiroclient.SubscriptionTierUnknown
	}
}

// TierSatisfies reports whether an account at the given subscription tier
// can invoke a model with the given minimum tier requirement.
//
// Tier ordering (highest → lowest): Power > Pro+ > Pro > Free > Unknown.
// Unknown account-tier is treated as "fail open" (returns true) because we
// do not want to block traffic when the management plane has not yet
// returned subscription metadata for an account.
//
// Unknown model-tier is also "fail open" because we do not want to gate
// requests on classification heuristics that have not been audited.
func TierSatisfies(account, modelMin kiroclient.SubscriptionTier) bool {
	if account == kiroclient.SubscriptionTierUnknown || modelMin == kiroclient.SubscriptionTierUnknown {
		return true
	}
	return tierRank(account) >= tierRank(modelMin)
}

// tierRank returns the integer ordering for a subscription tier. Higher
// is more permissive. Unknown is the lowest non-zero rank so that any
// known tier outranks unknown when both are present.
func tierRank(t kiroclient.SubscriptionTier) int {
	switch t {
	case kiroclient.SubscriptionTierPower:
		return 4
	case kiroclient.SubscriptionTierProPlus:
		return 3
	case kiroclient.SubscriptionTierPro:
		return 2
	case kiroclient.SubscriptionTierFree:
		return 1
	default:
		return 0
	}
}
