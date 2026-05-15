package pool

import (
	"context"
	"testing"
	"time"

	"github.com/nopperabbo/kiroxy/internal/logging"
)

func TestPool_StickinessPinsSessionToSameAccount(t *testing.T) {
	p, v := newPoolWithVault(t)
	seed(t, p, v, "a", "b", "c")
	stick := NewStickiness(time.Minute)
	t.Cleanup(stick.Stop)
	p.SetStickiness(stick)

	ctx := logging.WithSessionID(context.Background(), "sess-1")
	first, err := p.Pick(ctx, v)
	if err != nil {
		t.Fatalf("first pick: %v", err)
	}
	for i := 0; i < 10; i++ {
		r, err := p.Pick(ctx, v)
		if err != nil {
			t.Fatalf("pick %d: %v", i, err)
		}
		if r.ID != first.ID {
			t.Fatalf("iteration %d: session sess-1 migrated from %s to %s", i, first.ID, r.ID)
		}
	}
}

func TestPool_DifferentSessionsCanHitDifferentAccounts(t *testing.T) {
	// With 3 accounts and distinct session IDs, LRU-assigned pins should
	// eventually cover more than one account across independent sessions.
	p, v := newPoolWithVault(t)
	seed(t, p, v, "a", "b", "c")
	stick := NewStickiness(time.Minute)
	t.Cleanup(stick.Stop)
	p.SetStickiness(stick)

	seen := map[string]bool{}
	for i := 0; i < 10; i++ {
		ctx := logging.WithSessionID(context.Background(), "sess-"+string(rune('0'+i)))
		r, err := p.Pick(ctx, v)
		if err != nil {
			t.Fatalf("pick %d: %v", i, err)
		}
		seen[r.ID] = true
	}
	if len(seen) < 2 {
		t.Fatalf("expected distinct sessions to distribute across >=2 accounts; seen=%v", seen)
	}
}

func TestPool_EmptySessionIDUsesWeighted(t *testing.T) {
	p, v := newPoolWithVault(t)
	seed(t, p, v, "a", "b", "c")
	stick := NewStickiness(time.Minute)
	t.Cleanup(stick.Stop)
	p.SetStickiness(stick)

	// Context without session ID: behavior must exercise the weighted-
	// selection path. With equal fresh weights all 3 accounts must be
	// picked at least once across 60 calls (P(missing one) ≈ (2/3)^60 ≈ 1e-11).
	counts := map[string]int{}
	for i := 0; i < 60; i++ {
		r, err := p.Pick(context.Background(), v)
		if err != nil {
			t.Fatalf("pick %d: %v", i, err)
		}
		counts[r.ID]++
	}
	for _, id := range []string{"a", "b", "c"} {
		if counts[id] == 0 {
			t.Fatalf("empty session must exercise full account set; account %s never picked; counts=%v", id, counts)
		}
	}
}

func TestPool_StickinessReleasedOnFailure(t *testing.T) {
	p, v := newPoolWithVault(t)
	seed(t, p, v, "a", "b")
	stick := NewStickiness(time.Minute)
	t.Cleanup(stick.Stop)
	p.SetStickiness(stick)

	ctx := logging.WithSessionID(context.Background(), "sess-failover")
	first, err := p.Pick(ctx, v)
	if err != nil {
		t.Fatalf("first pick: %v", err)
	}

	// Quota-fail the pinned account. Stickiness.Release should drop the
	// pin so the next Pick re-evaluates.
	p.RecordFailure(first.ID, FailureQuota, "429")

	second, err := p.Pick(ctx, v)
	if err != nil {
		t.Fatalf("second pick: %v", err)
	}
	if second.ID == first.ID {
		t.Fatalf("session should have migrated off failed account %s, but got %s again", first.ID, second.ID)
	}
}

func TestPool_StickinessMigratesOffCooldownedAccount(t *testing.T) {
	// Even without going through RecordFailure's Release cascade, a pin
	// pointing at a cooldown-locked account must trigger a re-pick at
	// Pick time and migrate the session.
	p, v := newPoolWithVault(t)
	seed(t, p, v, "a", "b")
	stick := NewStickiness(time.Minute)
	t.Cleanup(stick.Stop)
	p.SetStickiness(stick)

	ctx := logging.WithSessionID(context.Background(), "sess-cooldown")
	first, err := p.Pick(ctx, v)
	if err != nil {
		t.Fatalf("first pick: %v", err)
	}

	// Manually put the pinned account on cooldown AND forcibly re-pin
	// (bypassing the RecordFailure release cascade) to exercise the
	// Pick-time stale-pin handling.
	h := p.health[first.ID]
	h.CooldownUntil = time.Now().Add(10 * time.Second)

	// Re-install the pin that Pool.Pick would have released.
	stick.sessions["sess-cooldown"] = stickySession{
		accountID: first.ID,
		expires:   time.Now().Add(time.Minute),
	}

	second, err := p.Pick(ctx, v)
	if err != nil {
		t.Fatalf("second pick: %v", err)
	}
	if second.ID == first.ID {
		t.Fatalf("session should have migrated off cooldown account %s, got %s", first.ID, second.ID)
	}
}

func TestPool_RemoveReleasesStickyPins(t *testing.T) {
	p, v := newPoolWithVault(t)
	seed(t, p, v, "a", "b")
	stick := NewStickiness(time.Minute)
	t.Cleanup(stick.Stop)
	p.SetStickiness(stick)

	ctx := logging.WithSessionID(context.Background(), "sess-remove")
	first, err := p.Pick(ctx, v)
	if err != nil {
		t.Fatalf("first pick: %v", err)
	}
	p.Remove(first.ID)
	if v, ok := stick.Snapshot()["sess-remove"]; ok {
		t.Fatalf("Remove should release pin; snapshot still shows sess-remove -> %s", v)
	}

	// Next pick in the same session must route to the surviving account.
	second, err := p.Pick(ctx, v)
	if err != nil {
		t.Fatalf("post-remove pick: %v", err)
	}
	if second.ID == first.ID {
		t.Fatalf("surviving account expected, got removed %s", first.ID)
	}
}
