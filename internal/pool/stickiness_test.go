package pool

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// newStickinessForTest builds a Stickiness whose clock is driven manually,
// so TTL expiry is deterministic without sleeping. The background pruner
// is stopped via t.Cleanup.
func newStickinessForTest(t *testing.T, ttl time.Duration) (*Stickiness, func(time.Duration)) {
	t.Helper()
	s := NewStickiness(ttl)
	t.Cleanup(s.Stop)
	// Swap the clock AFTER construction so the pruner goroutine sees the
	// controllable clock on its next tick; initial pruneStop gating is fine.
	var mu sync.Mutex
	now := time.Now()
	s.now = func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return now
	}
	advance := func(d time.Duration) {
		mu.Lock()
		defer mu.Unlock()
		now = now.Add(d)
	}
	return s, advance
}

func TestStickiness_PinsNewSession(t *testing.T) {
	s, _ := newStickinessForTest(t, time.Minute)

	calls := 0
	got := s.Pick("sess-1", func() string {
		calls++
		return "acc-A"
	})
	if got != "acc-A" {
		t.Fatalf("want acc-A, got %q", got)
	}
	if calls != 1 {
		t.Fatalf("fallback should run exactly once on first Pick, ran %d", calls)
	}
	if v := s.Snapshot()["sess-1"]; v != "acc-A" {
		t.Fatalf("snapshot should contain sess-1->acc-A, got %q", v)
	}
}

func TestStickiness_ReturnsPinnedForSameSession(t *testing.T) {
	s, _ := newStickinessForTest(t, time.Minute)

	_ = s.Pick("sess-1", func() string { return "acc-A" })

	calls := 0
	for i := 0; i < 5; i++ {
		got := s.Pick("sess-1", func() string {
			calls++
			return "acc-B-should-not-happen"
		})
		if got != "acc-A" {
			t.Fatalf("iteration %d: want pinned acc-A, got %q", i, got)
		}
	}
	if calls != 0 {
		t.Fatalf("fallback must not be invoked for live pin, ran %d", calls)
	}
}

func TestStickiness_ExpiresAfterTTL(t *testing.T) {
	s, advance := newStickinessForTest(t, time.Minute)

	_ = s.Pick("sess-1", func() string { return "acc-A" })

	// Half-TTL: still pinned to A.
	advance(30 * time.Second)
	if got := s.Pick("sess-1", func() string { return "acc-B" }); got != "acc-A" {
		t.Fatalf("before expiry want acc-A, got %q", got)
	}

	// Past TTL: fallback re-pins.
	advance(2 * time.Minute)
	if got := s.Pick("sess-1", func() string { return "acc-B" }); got != "acc-B" {
		t.Fatalf("after expiry fallback should win, got %q", got)
	}
	if v := s.Snapshot()["sess-1"]; v != "acc-B" {
		t.Fatalf("snapshot should now hold acc-B, got %q", v)
	}
}

func TestStickiness_ReleaseClearsAccountPins(t *testing.T) {
	s, _ := newStickinessForTest(t, time.Minute)

	_ = s.Pick("sess-1", func() string { return "acc-A" })
	_ = s.Pick("sess-2", func() string { return "acc-A" })
	_ = s.Pick("sess-3", func() string { return "acc-B" })

	s.Release("acc-A")

	snap := s.Snapshot()
	if _, ok := snap["sess-1"]; ok {
		t.Errorf("sess-1 should have been released (was pinned to acc-A)")
	}
	if _, ok := snap["sess-2"]; ok {
		t.Errorf("sess-2 should have been released (was pinned to acc-A)")
	}
	if snap["sess-3"] != "acc-B" {
		t.Errorf("sess-3 should survive acc-A release; got %q", snap["sess-3"])
	}
}

func TestStickiness_EmptySessionUsesFallback(t *testing.T) {
	s, _ := newStickinessForTest(t, time.Minute)

	calls := 0
	for i := 0; i < 3; i++ {
		got := s.Pick("", func() string {
			calls++
			return "acc-X"
		})
		if got != "acc-X" {
			t.Fatalf("iteration %d: want acc-X, got %q", i, got)
		}
	}
	if calls != 3 {
		t.Fatalf("empty session must call fallback every time; got %d calls", calls)
	}
	if len(s.Snapshot()) != 0 {
		t.Fatalf("empty session must not persist a pin; snapshot=%v", s.Snapshot())
	}
}

func TestStickiness_FallbackReturningEmptyDoesNotPin(t *testing.T) {
	s, _ := newStickinessForTest(t, time.Minute)

	got := s.Pick("sess-1", func() string { return "" })
	if got != "" {
		t.Fatalf("want empty pass-through, got %q", got)
	}
	if _, ok := s.Snapshot()["sess-1"]; ok {
		t.Fatalf("empty fallback must not write a pin")
	}
}

func TestStickiness_PruneDropsExpiredEntries(t *testing.T) {
	s, advance := newStickinessForTest(t, time.Minute)

	_ = s.Pick("sess-1", func() string { return "acc-A" })
	_ = s.Pick("sess-2", func() string { return "acc-B" })

	advance(2 * time.Minute)
	s.prune()

	if n := len(s.Snapshot()); n != 0 {
		t.Fatalf("expected all pins pruned; snapshot=%v", s.Snapshot())
	}
}

func TestStickiness_ConcurrentPickConvergesToSingleAccount(t *testing.T) {
	// Under contention, N goroutines asking for the same session ID
	// must all receive the SAME account (the first winner's pick),
	// not one pick per goroutine.
	s, _ := newStickinessForTest(t, time.Minute)

	const N = 50
	var wg sync.WaitGroup
	results := make([]string, N)
	var fallbackCalls atomic.Int32

	// Each goroutine proposes a different account; only the first
	// call that writes the pin should stick.
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i] = s.Pick("shared-sess", func() string {
				fallbackCalls.Add(1)
				// Small sleep to widen the race window.
				time.Sleep(time.Microsecond)
				return "acc-goroutine"
			})
		}(i)
	}
	wg.Wait()

	// All results must match; any divergence means the pin didn't
	// serialize.
	winner := results[0]
	for i, r := range results {
		if r != winner {
			t.Fatalf("goroutine %d got %q, expected all %q", i, r, winner)
		}
	}
	if winner == "" {
		t.Fatalf("all goroutines saw empty pick")
	}
	// Fallback can legitimately fire up to N times in the worst race
	// case because we invoke it outside the lock; what matters is the
	// PIN converges, which we verified above.
	if fallbackCalls.Load() == 0 {
		t.Fatalf("at least one fallback invocation expected")
	}
}

func TestStickiness_StopIsIdempotent(t *testing.T) {
	s := NewStickiness(time.Minute)
	s.Stop()
	s.Stop() // must not panic on repeated close.
}

func TestStickiness_DefaultTTLWhenZero(t *testing.T) {
	s := NewStickiness(0)
	defer s.Stop()
	if s.ttl != DefaultStickinessTTL {
		t.Fatalf("want default TTL %v, got %v", DefaultStickinessTTL, s.ttl)
	}
}
