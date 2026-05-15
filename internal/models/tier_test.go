// kiroxy addition (not derived from upstream).
//
// Tests for the Anthropic-model → minimum-subscription-tier mapping plus
// the tier-comparison helper. These gate operator-warning emission in
// the messages hot path.

package models

import (
	"testing"

	"github.com/nopperabbo/kiroxy/internal/kiroclient"
)

func TestMinimumTier(t *testing.T) {
	cases := []struct {
		name  string
		model string
		want  kiroclient.SubscriptionTier
	}{
		{"haiku 4.5 dashed", "claude-haiku-4-5", kiroclient.SubscriptionTierFree},
		{"haiku 4.5 dotted", "claude-haiku-4.5", kiroclient.SubscriptionTierFree},
		{"sonnet 4.5", "claude-sonnet-4-5", kiroclient.SubscriptionTierPro},
		{"sonnet 4.6", "claude-sonnet-4.6", kiroclient.SubscriptionTierPro},
		{"opus 4.7", "claude-opus-4.7", kiroclient.SubscriptionTierPro},
		{"opus 1m suffix", "claude-opus-4-7[1m]", kiroclient.SubscriptionTierPro},
		{"empty", "", kiroclient.SubscriptionTierUnknown},
		{"unknown family", "claude-flash-9", kiroclient.SubscriptionTierUnknown},
		{"case insensitive", "CLAUDE-OPUS-4.7", kiroclient.SubscriptionTierPro},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := MinimumTier(tc.model); got != tc.want {
				t.Errorf("MinimumTier(%q) = %q, want %q", tc.model, got, tc.want)
			}
		})
	}
}

func TestTierSatisfies(t *testing.T) {
	cases := []struct {
		name     string
		account  kiroclient.SubscriptionTier
		modelMin kiroclient.SubscriptionTier
		want     bool
	}{
		// The two unknown short-circuits — both return true (fail open).
		{"unknown account, pro model", kiroclient.SubscriptionTierUnknown, kiroclient.SubscriptionTierPro, true},
		{"pro account, unknown model", kiroclient.SubscriptionTierPro, kiroclient.SubscriptionTierUnknown, true},
		{"both unknown", kiroclient.SubscriptionTierUnknown, kiroclient.SubscriptionTierUnknown, true},

		// Free account boundaries.
		{"free / haiku ok", kiroclient.SubscriptionTierFree, kiroclient.SubscriptionTierFree, true},
		{"free / sonnet blocked", kiroclient.SubscriptionTierFree, kiroclient.SubscriptionTierPro, false},

		// Pro account allows everything in current model lineup.
		{"pro / haiku ok", kiroclient.SubscriptionTierPro, kiroclient.SubscriptionTierFree, true},
		{"pro / sonnet ok", kiroclient.SubscriptionTierPro, kiroclient.SubscriptionTierPro, true},

		// Higher tiers always pass.
		{"pro+ / sonnet", kiroclient.SubscriptionTierProPlus, kiroclient.SubscriptionTierPro, true},
		{"power / sonnet", kiroclient.SubscriptionTierPower, kiroclient.SubscriptionTierPro, true},
		{"power / haiku", kiroclient.SubscriptionTierPower, kiroclient.SubscriptionTierFree, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := TierSatisfies(tc.account, tc.modelMin); got != tc.want {
				t.Errorf("TierSatisfies(%q, %q) = %v, want %v", tc.account, tc.modelMin, got, tc.want)
			}
		})
	}
}
