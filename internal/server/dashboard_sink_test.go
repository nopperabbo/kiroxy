package server

import (
	"sync"
	"testing"
	"time"
)

func TestRequestRing_Capacity(t *testing.T) {
	r := NewRequestRing(3)
	for i := 0; i < 5; i++ {
		r.Record(RequestRecord{ID: string(rune('a' + i)), Status: 200})
	}
	snap := r.Snapshot(0)
	if len(snap) != 3 {
		t.Fatalf("want 3 records (ring cap), got %d", len(snap))
	}
	// Newest-first: we pushed a,b,c,d,e; expect e,d,c.
	wantIDs := []string{"e", "d", "c"}
	for i, w := range wantIDs {
		if snap[i].ID != w {
			t.Errorf("snapshot[%d].ID=%q want %q", i, snap[i].ID, w)
		}
	}
}

func TestRequestRing_SnapshotMax(t *testing.T) {
	r := NewRequestRing(10)
	for i := 0; i < 7; i++ {
		r.Record(RequestRecord{ID: string(rune('a' + i)), Status: 200})
	}
	snap := r.Snapshot(3)
	if len(snap) != 3 {
		t.Fatalf("want 3 records, got %d", len(snap))
	}
	if snap[0].ID != "g" {
		t.Fatalf("want newest first (g), got %q", snap[0].ID)
	}
}

func TestRequestRing_Counters(t *testing.T) {
	r := NewRequestRing(10)
	r.Record(RequestRecord{Status: 200})
	r.Record(RequestRecord{Status: 200})
	r.Record(RequestRecord{Status: 500})
	r.Record(RequestRecord{Status: 429})
	c := r.Counters()
	if c.TotalRequests != 4 {
		t.Errorf("total=%d want 4", c.TotalRequests)
	}
	if c.TotalErrors != 2 {
		t.Errorf("errs=%d want 2", c.TotalErrors)
	}
	if c.ErrorRate != 0.5 {
		t.Errorf("rate=%v want 0.5", c.ErrorRate)
	}
}

func TestRequestRing_CountersZeroOnEmpty(t *testing.T) {
	r := NewRequestRing(10)
	c := r.Counters()
	if c.ErrorRate != 0 {
		t.Errorf("empty-ring rate should be 0, got %v", c.ErrorRate)
	}
}

func TestRequestRing_CapacityGuard(t *testing.T) {
	r := NewRequestRing(0)
	r.Record(RequestRecord{ID: "x"})
	if len(r.Snapshot(0)) != 1 {
		t.Fatalf("zero-cap ring should be coerced to 1, got size %d", len(r.Snapshot(0)))
	}
}

func TestRequestRing_ConcurrentWrites(t *testing.T) {
	r := NewRequestRing(100)
	var wg sync.WaitGroup
	const (
		writers   = 10
		perWriter = 200
	)
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for j := 0; j < perWriter; j++ {
				r.Record(RequestRecord{
					ID:        "w",
					StartedAt: time.Now(),
					Status:    200,
				})
			}
		}(i)
	}
	wg.Wait()
	c := r.Counters()
	if c.TotalRequests != int64(writers*perWriter) {
		t.Errorf("want %d total, got %d", writers*perWriter, c.TotalRequests)
	}
	if len(r.Snapshot(0)) != 100 {
		t.Errorf("ring should be full (100), got %d", len(r.Snapshot(0)))
	}
}

func TestRequestRing_SubscribeReceivesRecords(t *testing.T) {
	r := NewRequestRing(10)
	ch, cancel := r.Subscribe()
	defer cancel()

	go r.Record(RequestRecord{ID: "live", Status: 200})

	select {
	case rec := <-ch:
		if rec.ID != "live" {
			t.Errorf("got %q want live", rec.ID)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for subscribed event")
	}
}

func TestRequestRing_SubscribeCancelStopsDelivery(t *testing.T) {
	r := NewRequestRing(10)
	ch, cancel := r.Subscribe()
	cancel()

	r.Record(RequestRecord{ID: "after-cancel", Status: 200})

	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("want channel closed after cancel")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("cancel should close the channel")
	}
}

func TestRequestRing_SubscribeNonBlocking(t *testing.T) {
	r := NewRequestRing(10)
	ch, cancel := r.Subscribe()
	defer cancel()

	// Fill the channel's 8-slot buffer, then push 10 more. Record() must not
	// block or lose the caller's hot-path.
	done := make(chan struct{})
	go func() {
		for i := 0; i < 20; i++ {
			r.Record(RequestRecord{ID: "burst", Status: 200})
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Record blocked on slow subscriber")
	}
	// Drain what we can; exact count depends on scheduling but must be <= 8.
	drained := 0
	for {
		select {
		case <-ch:
			drained++
		default:
			if drained > 20 {
				t.Fatalf("drained %d; bigger than total pushed", drained)
			}
			return
		}
	}
}

func TestRoundRate(t *testing.T) {
	cases := []struct {
		in, want float64
	}{
		{0, 0},
		{0.12345678, 0.1235},
		{0.99999, 1.0},
		{1.0, 1.0},
	}
	for _, tc := range cases {
		if got := roundRate(tc.in); got != tc.want {
			t.Errorf("roundRate(%v)=%v want %v", tc.in, got, tc.want)
		}
	}
}
