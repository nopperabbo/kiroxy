package pool

import (
	"context"
	"testing"
	"time"
)

// TestPool_WeightedPickFavorsHealthyAccount injects failure history into
// one account, seeds the other as pristine, and verifies weighted random
// selection strongly prefers the healthy one over 200 trials.
func TestPool_WeightedPickFavorsHealthyAccount(t *testing.T) {
	p, v := newPoolWithVault(t)
	seed(t, p, v, "healthy", "flaky")

	// Degrade "flaky" via 80 simulated failures out of 100 outcomes.
	flaky := p.ext["flaky"]
	for i := 0; i < 80; i++ {
		flaky.pushOutcome(false)
	}
	for i := 0; i < 20; i++ {
		flaky.pushOutcome(true)
	}
	// Leave "healthy" pristine (empty ring -> SuccessRate 1.0).

	counts := map[string]int{}
	for i := 0; i < 200; i++ {
		r, err := p.Pick(context.Background(), v)
		if err != nil {
			t.Fatalf("pick %d: %v", i, err)
		}
		counts[r.ID]++
	}

	if counts["healthy"] <= counts["flaky"] {
		t.Fatalf("weighted random should favor healthy: counts=%v", counts)
	}
	// Expected ratio: weight(healthy)/weight(flaky) = 1.0/0.2 = 5:1.
	// Allow wide tolerance: healthy should win by at least 2x, and
	// flaky should still be picked some (non-zero).
	if counts["flaky"] == 0 {
		t.Errorf("flaky account should still be picked occasionally (soft weighting, not hard skip)")
	}
	if float64(counts["healthy"])/float64(counts["flaky"]+1) < 2.0 {
		t.Errorf("expected healthy:flaky ratio > 2x; counts=%v", counts)
	}
}

// TestPool_WeightedPickFallsBackToLRUWhenAllDegraded verifies that when
// every candidate has collapsed to the weight floor, the fallback LRU
// order kicks in (so selection is deterministic-ish rather than RNG
// noise among 0.01 weights).
func TestPool_WeightedPickFallsBackToLRUWhenAllDegraded(t *testing.T) {
	p, v := newPoolWithVault(t)
	seed(t, p, v, "a", "b", "c")

	// Crush every account to the weight floor: full failure history +
	// recent rate-limit + saturated load.
	for id := range p.accounts {
		ah := p.ext[id]
		for i := 0; i < defaultRingSize; i++ {
			ah.pushOutcome(false)
		}
		ah.LastRateLimit = time.Now()
		for i := 0; i < 200; i++ {
			ah.recentReqs.add(time.Now(), 1)
		}
	}

	// None are on hard cooldown (we didn't call RecordFailure), so
	// candidates survive the gate but all hit the weight floor.
	// Degenerate path should return LRU-oldest, which with fresh
	// LastUsed=zero breaks ties by map iteration order. We only verify
	// that SOME account is picked and no error is returned.
	for i := 0; i < 10; i++ {
		r, err := p.Pick(context.Background(), v)
		if err != nil {
			t.Fatalf("pick %d: %v under degraded weights should fall back to LRU", i, err)
		}
		if r == nil {
			t.Fatalf("pick %d returned nil result", i)
		}
	}
}

// TestPool_HealthSnapshotsReturnsPerAccountRow checks the dashboard-
// facing projection.
func TestPool_HealthSnapshotsReturnsPerAccountRow(t *testing.T) {
	p, v := newPoolWithVault(t)
	seed(t, p, v, "a", "b")

	// One success to give AvgLatency a non-zero value.
	p.RecordSuccessWithLatency("a", 200*time.Millisecond)
	p.RecordFailure("b", FailureQuota, "429")

	snaps := p.HealthSnapshots()
	if len(snaps) != 2 {
		t.Fatalf("want 2 snapshots, got %d", len(snaps))
	}
	// Sorted by AccountID so snaps[0] == "a".
	if snaps[0].AccountID != "a" {
		t.Errorf("snapshots must sort by ID; got %v first", snaps[0].AccountID)
	}
	if snaps[0].AvgLatency != 200*time.Millisecond {
		t.Errorf("AvgLatency for 'a' should reflect 200ms sample, got %v", snaps[0].AvgLatency)
	}
	if snaps[1].LastRateLimit.IsZero() {
		t.Errorf("LastRateLimit for 'b' should be stamped by the quota failure")
	}
	if snaps[0].Weight <= snaps[1].Weight {
		t.Errorf("healthy 'a' weight (%f) should exceed rate-limited 'b' weight (%f)", snaps[0].Weight, snaps[1].Weight)
	}
}

// TestPool_WeightedRecoveryAfterRateLimitCooldown: after the 30min
// rate-limit penalty window lifts, a previously-flagged account's
// weight should recover and it should compete normally again.
func TestPool_WeightedRecoveryAfterRateLimitCooldown(t *testing.T) {
	p, v := newPoolWithVault(t)
	seed(t, p, v, "recovered", "healthy")

	// Stamp rate-limit > 30min ago so the penalty has lifted.
	p.ext["recovered"].LastRateLimit = time.Now().Add(-45 * time.Minute)
	// Seed success history on the recovered account to confirm weight
	// rebuilds.
	for i := 0; i < 90; i++ {
		p.ext["recovered"].pushOutcome(true)
	}

	counts := map[string]int{}
	for i := 0; i < 100; i++ {
		r, err := p.Pick(context.Background(), v)
		if err != nil {
			t.Fatal(err)
		}
		counts[r.ID]++
	}

	// Roughly balanced now; neither should dominate beyond ~70/30.
	if counts["recovered"] == 0 {
		t.Errorf("recovered account should participate; counts=%v", counts)
	}
	if counts["healthy"] == 0 {
		t.Errorf("healthy account should participate; counts=%v", counts)
	}
	if counts["recovered"] < 20 || counts["recovered"] > 80 {
		t.Errorf("recovered account share %d/100 should be roughly balanced [20,80]; counts=%v", counts["recovered"], counts)
	}
}
