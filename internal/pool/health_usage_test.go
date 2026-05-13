package pool

import (
	"context"
	"testing"
	"time"

	"local/kiroxy/internal/kiroclient"
)

// TestWeight_NoUsageDataMeansNoPenalty verifies the nil-safety
// contract: an account that has never been polled competes at full
// strength, so an introspection failure never drops an account out of
// the rotation.
func TestWeight_NoUsageDataMeansNoPenalty(t *testing.T) {
	h := newAccountHealth()
	got := h.Weight(time.Now())
	if got < 0.99 {
		t.Errorf("nil UsageLimits should not penalize, got %f", got)
	}
}

// TestWeight_ZeroCapMeansNoPenalty confirms that a malformed poll
// response (cap=0, used=0) is treated identically to nil — we don't
// know the actual ledger, so we don't penalize.
func TestWeight_ZeroCapMeansNoPenalty(t *testing.T) {
	h := newAccountHealth()
	h.UsageLimits = &kiroclient.UsageLimits{
		MonthlyCap:              0,
		MonthlyCreditsUsed:      0,
		MonthlyCreditsRemaining: 0,
	}
	got := h.Weight(time.Now())
	if got < 0.99 {
		t.Errorf("zero cap should not penalize, got %f", got)
	}
}

// TestWeight_DrainedAccountCollapsesToFloor: an account at < 10%
// remaining should drop to the floor fraction so weighted selection
// effectively skips it. The hard cooldown gate stays the operator's
// only true skip; this is just a soft routing nudge.
func TestWeight_DrainedAccountCollapsesToFloor(t *testing.T) {
	h := newAccountHealth()
	h.UsageLimits = &kiroclient.UsageLimits{
		MonthlyCap:              1000,
		MonthlyCreditsUsed:      950,
		MonthlyCreditsRemaining: 50,
	}
	got := h.Weight(time.Now())
	if got > 0.05 {
		t.Errorf("drained account (5%% remaining) should collapse near floor, got %f", got)
	}
}

// TestWeight_HalfDrainedScalesLinearly: a 50%-remaining account
// should weigh roughly half a fresh one.
func TestWeight_HalfDrainedScalesLinearly(t *testing.T) {
	hHealthy := newAccountHealth()
	hHalf := newAccountHealth()
	hHalf.UsageLimits = &kiroclient.UsageLimits{
		MonthlyCap:              1000,
		MonthlyCreditsUsed:      500,
		MonthlyCreditsRemaining: 500,
	}
	now := time.Now()
	full := hHealthy.Weight(now)
	half := hHalf.Weight(now)
	ratio := half / full
	if ratio < 0.45 || ratio > 0.55 {
		t.Errorf("50%% remaining should yield ~half weight; got full=%f half=%f ratio=%f", full, half, ratio)
	}
}

// TestPick_FavorsAccountWithMoreCreditsRemaining: with everything
// else equal, the pool should bias toward the account that has more
// runway. Validates the fleet-spread behavior end-to-end.
func TestPick_FavorsAccountWithMoreCreditsRemaining(t *testing.T) {
	pool, vault := newPoolWithVault(t)
	seed(t, pool, vault, "fresh", "burnt")

	pool.SetUsage("fresh", &kiroclient.UsageLimits{
		MonthlyCap:              1000,
		MonthlyCreditsUsed:      50,
		MonthlyCreditsRemaining: 950,
	})
	pool.SetUsage("burnt", &kiroclient.UsageLimits{
		MonthlyCap:              1000,
		MonthlyCreditsUsed:      850,
		MonthlyCreditsRemaining: 150,
	})

	counts := map[string]int{}
	for i := 0; i < 200; i++ {
		r, err := pool.Pick(context.Background(), vault)
		if err != nil {
			t.Fatalf("pick %d: %v", i, err)
		}
		counts[r.ID]++
	}
	if counts["fresh"] <= counts["burnt"] {
		t.Errorf("fresh account should win majority share; counts=%v", counts)
	}
	if float64(counts["fresh"])/float64(counts["burnt"]+1) < 2.0 {
		t.Errorf("expected fresh:burnt ratio > 2x; counts=%v", counts)
	}
}

// TestPick_DrainedAccountEffectivelySkipped: an account in the
// drained band (<10% remaining) should get effectively no traffic
// when a healthy alternative exists.
func TestPick_DrainedAccountEffectivelySkipped(t *testing.T) {
	pool, vault := newPoolWithVault(t)
	seed(t, pool, vault, "alive", "drained")

	pool.SetUsage("alive", &kiroclient.UsageLimits{
		MonthlyCap:              1000,
		MonthlyCreditsUsed:      100,
		MonthlyCreditsRemaining: 900,
	})
	pool.SetUsage("drained", &kiroclient.UsageLimits{
		MonthlyCap:              1000,
		MonthlyCreditsUsed:      995,
		MonthlyCreditsRemaining: 5,
	})

	counts := map[string]int{}
	for i := 0; i < 200; i++ {
		r, err := pool.Pick(context.Background(), vault)
		if err != nil {
			t.Fatal(err)
		}
		counts[r.ID]++
	}
	if counts["alive"] < 180 {
		t.Errorf("alive account should sweep; counts=%v", counts)
	}
}

// TestHealthSnapshots_IncludesUsageWhenKnown verifies the dashboard
// shape returned by HealthSnapshots includes the usage block.
func TestHealthSnapshots_IncludesUsageWhenKnown(t *testing.T) {
	pool, vault := newPoolWithVault(t)
	seed(t, pool, vault, "instrumented", "blank")

	pool.SetUsage("instrumented", &kiroclient.UsageLimits{
		MonthlyCap:              1000,
		MonthlyCreditsUsed:      300,
		MonthlyCreditsRemaining: 700,
		PercentUsed:             0.30,
		LastQueryTime:           time.Now().Add(-30 * time.Second),
		DaysUntilReset:          12,
	})

	snaps := pool.HealthSnapshots()
	if len(snaps) != 2 {
		t.Fatalf("want 2 snapshots, got %d", len(snaps))
	}

	var instrumented, blank *HealthSnapshot
	for i := range snaps {
		switch snaps[i].AccountID {
		case "instrumented":
			instrumented = &snaps[i]
		case "blank":
			blank = &snaps[i]
		}
	}
	if instrumented == nil || blank == nil {
		t.Fatalf("missing snapshot row: %+v", snaps)
	}
	if !instrumented.UsageKnown {
		t.Error("instrumented account should report UsageKnown=true")
	}
	if instrumented.UsageRemaining != 700 || instrumented.UsageCap != 1000 {
		t.Errorf("usage drift: cap=%d remaining=%d", instrumented.UsageCap, instrumented.UsageRemaining)
	}
	if instrumented.UsageDaysUntilRst != 12 {
		t.Errorf("days until reset: got %d", instrumented.UsageDaysUntilRst)
	}
	if blank.UsageKnown {
		t.Error("never-polled account must report UsageKnown=false")
	}
}

// TestUsageFactor_ExhaustedNeverNegative defends against numeric
// drift. An "overage" account (used > cap) should clamp to the floor
// fraction, never flip negative.
func TestUsageFactor_ExhaustedNeverNegative(t *testing.T) {
	got := usageFactor(&kiroclient.UsageLimits{
		MonthlyCap:              100,
		MonthlyCreditsUsed:      150,
		MonthlyCreditsRemaining: 0,
	})
	if got <= 0 {
		t.Errorf("exhausted account factor must stay positive (>=floor), got %f", got)
	}
	if got > 0.05 {
		t.Errorf("exhausted account factor must be near floor, got %f", got)
	}
}
