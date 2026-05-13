package pool

import (
	"math"
	"testing"
	"time"
)

func TestHealth_FreshAccountHasCeilingWeight(t *testing.T) {
	h := newAccountHealth()
	if got := h.SuccessRate(); got != 1.0 {
		t.Errorf("fresh SuccessRate want 1.0, got %f", got)
	}
	if got := h.Weight(time.Now()); got < 0.9 {
		t.Errorf("fresh Weight should be near ceiling, got %f", got)
	}
}

func TestHealth_SuccessRateFromRing(t *testing.T) {
	h := newAccountHealth()
	now := time.Now()

	// 7 successes, 3 failures -> rate = 0.7
	for i := 0; i < 7; i++ {
		h.recordSuccess(now, 0)
	}
	for i := 0; i < 3; i++ {
		h.recordFailure(now, FailureTransient)
	}
	got := h.SuccessRate()
	if math.Abs(got-0.7) > 1e-9 {
		t.Errorf("want 0.7, got %f", got)
	}
}

func TestHealth_RingWrapsAtCapacity(t *testing.T) {
	h := newAccountHealth()
	now := time.Now()

	// Fill with successes
	for i := 0; i < defaultRingSize; i++ {
		h.recordSuccess(now, 0)
	}
	if got := h.SuccessRate(); got != 1.0 {
		t.Fatalf("100 successes want rate 1.0, got %f", got)
	}

	// Overwrite half with failures; ring size stays at defaultRingSize
	for i := 0; i < defaultRingSize/2; i++ {
		h.recordFailure(now, FailureTransient)
	}
	// Ring should now hold (defaultRingSize/2) successes at older slots
	// + (defaultRingSize/2) failures at newer slots = 0.5
	got := h.SuccessRate()
	want := 0.5
	if math.Abs(got-want) > 0.01 {
		t.Errorf("after wrap want ~%f, got %f", want, got)
	}
	if h.ringFilled != defaultRingSize {
		t.Errorf("ringFilled should cap at %d, got %d", defaultRingSize, h.ringFilled)
	}
}

func TestHealth_WeightDecreasesOnFailures(t *testing.T) {
	h := newAccountHealth()
	now := time.Now()

	for i := 0; i < 10; i++ {
		h.recordFailure(now, FailureTransient)
	}
	got := h.Weight(now)
	if got >= 0.5 {
		t.Errorf("after 10 failures weight should drop below 0.5, got %f", got)
	}
	if got < weightFloor {
		t.Errorf("weight must clamp at floor %f, got %f", weightFloor, got)
	}
}

func TestHealth_QuotaFailureRecordsRateLimit(t *testing.T) {
	h := newAccountHealth()
	now := time.Now()

	h.recordFailure(now, FailureQuota)
	if h.LastRateLimit.IsZero() {
		t.Fatalf("Quota failure should stamp LastRateLimit")
	}
	wFresh := h.Weight(now)

	// Within cooldown window: 0.1x penalty multiplies onto success rate
	// (which is 0 after the one failure) so weight sits at the floor.
	if wFresh > 0.1 {
		t.Errorf("quota weight during cooldown should be heavily penalized, got %f", wFresh)
	}

	// Outside cooldown window: penalty lifts; success rate is still 0,
	// so weight stays low but no longer has the 0.1x multiplier layered on.
	later := now.Add(rateLimitCooldownWindow + time.Minute)
	wAfter := h.Weight(later)
	if wAfter < wFresh {
		t.Errorf("weight after cooldown (%f) should not be lower than during (%f)", wAfter, wFresh)
	}
}

func TestHealth_RateLimitPenaltyLifts(t *testing.T) {
	h := newAccountHealth()
	now := time.Now()

	// Seed high success + a single rate-limit stamp to isolate the penalty.
	// Use pushOutcome to avoid touching recentReqs (which would perturb the
	// load factor).
	for i := 0; i < 100; i++ {
		h.pushOutcome(true)
	}
	h.LastRateLimit = now

	wWith := h.Weight(now)
	wAfter := h.Weight(now.Add(rateLimitCooldownWindow + time.Minute))
	if wWith >= wAfter {
		t.Errorf("within-window (%f) should be LESS than outside-window (%f)", wWith, wAfter)
	}
	if wAfter < 0.9 {
		t.Errorf("after cooldown + full success rate, weight should be near ceiling, got %f", wAfter)
	}
}

func TestHealth_RecentRequestsCount(t *testing.T) {
	h := newAccountHealth()
	now := time.Now()

	for i := 0; i < 30; i++ {
		h.recentReqs.add(now.Add(time.Duration(i)*time.Second), 1)
	}
	got := h.RequestsInWindow(now.Add(29 * time.Second))
	if got != 30 {
		t.Errorf("want 30, got %d", got)
	}
}

func TestHealth_RecentRequestsExpire(t *testing.T) {
	h := newAccountHealth()
	base := time.Now()

	// 10 requests 10 minutes ago (outside the 5-minute window)
	old := base.Add(-10 * time.Minute)
	for i := 0; i < 10; i++ {
		h.recentReqs.add(old, 1)
	}
	// 5 requests now
	for i := 0; i < 5; i++ {
		h.recentReqs.add(base, 1)
	}

	got := h.RequestsInWindow(base)
	if got != 5 {
		t.Errorf("old buckets should have aged out; want 5, got %d", got)
	}
}

func TestHealth_HighRecentLoadReducesWeight(t *testing.T) {
	h := newAccountHealth()
	now := time.Now()

	// Seed perfect success rate WITHOUT touching the recent-load counter.
	for i := 0; i < 100; i++ {
		h.pushOutcome(true)
	}
	w0 := h.Weight(now) // load factor 1.0, rate 1.0 -> weight ceiling

	// Now push the recent-load counter into saturation; success rate
	// unchanged.
	for i := 0; i < 200; i++ {
		h.recentReqs.add(now, 1)
	}
	wLoaded := h.Weight(now)
	if wLoaded >= w0 {
		t.Errorf("high load should reduce weight; w0=%f wLoaded=%f", w0, wLoaded)
	}
	if wLoaded < 0.29 || wLoaded > 0.31 {
		t.Errorf("load factor should clamp near 0.3, weight=%f", wLoaded)
	}
}

func TestHealth_LatencyEWMA(t *testing.T) {
	h := newAccountHealth()

	h.recordLatency(100 * time.Millisecond)
	if h.AvgLatency != 100*time.Millisecond {
		t.Errorf("first sample should set EWMA directly, got %v", h.AvgLatency)
	}

	// Blend in a 500ms sample with alpha=0.2 -> 0.2*500 + 0.8*100 = 180ms
	h.recordLatency(500 * time.Millisecond)
	want := 180 * time.Millisecond
	// Allow 1ms slop for float-to-duration rounding.
	diff := h.AvgLatency - want
	if diff < 0 {
		diff = -diff
	}
	if diff > time.Millisecond {
		t.Errorf("EWMA want ~%v, got %v", want, h.AvgLatency)
	}
}

func TestHealth_WeightClampsToFloor(t *testing.T) {
	h := newAccountHealth()
	now := time.Now()

	// All failures + quota stamp -> hits every penalty simultaneously
	for i := 0; i < defaultRingSize; i++ {
		h.recordFailure(now, FailureTransient)
	}
	h.LastRateLimit = now
	for i := 0; i < 500; i++ {
		h.recentReqs.add(now, 1)
	}

	got := h.Weight(now)
	if got != weightFloor {
		t.Errorf("weight should clamp to floor %f, got %f", weightFloor, got)
	}
}

func TestHealth_EmptyRingYieldsCeiling(t *testing.T) {
	h := newAccountHealth()
	if h.SuccessRate() != 1.0 {
		t.Fatalf("empty ring should report SuccessRate=1.0, got %f", h.SuccessRate())
	}
	w := h.Weight(time.Now())
	if w < 0.95 {
		t.Errorf("empty history should give near-ceiling weight, got %f", w)
	}
}
