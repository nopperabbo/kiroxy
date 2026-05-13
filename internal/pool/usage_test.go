package pool

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"local/kiroxy/internal/kiroclient"
)

// TestUsagePoller_Disabled_NoOp confirms that an unwired poller (nil
// PollFn) shuts down cleanly without spawning a goroutine.
func TestUsagePoller_Disabled_NoOp(t *testing.T) {
	p := NewUsagePoller(UsagePollerConfig{Pool: nil, Vault: nil, PollFn: nil})
	p.Start(context.Background())
	p.Stop()
}

// TestUsagePoller_FirstPassPopulatesCache runs one full pass with a
// fake poll fn and verifies SetUsage propagated the result onto every
// account's AccountHealth.UsageLimits field.
func TestUsagePoller_FirstPassPopulatesCache(t *testing.T) {
	pool, vault := newPoolWithVault(t)
	seed(t, pool, vault, "a", "b", "c")

	calls := atomic.Int32{}
	fakePoll := func(_ context.Context, token, _, _ string) (*kiroclient.UsageLimits, error) {
		calls.Add(1)
		return &kiroclient.UsageLimits{
			MonthlyCap:              1000,
			MonthlyCreditsUsed:      250,
			MonthlyCreditsRemaining: 750,
			PercentUsed:             0.25,
			LastQueryTime:           time.Now(),
		}, nil
	}

	poller := NewUsagePoller(UsagePollerConfig{
		Pool:     pool,
		Vault:    vault,
		PollFn:   fakePoll,
		Interval: 10 * time.Second,
		Timeout:  2 * time.Second,
	})
	ctx, cancel := context.WithCancel(context.Background())
	poller.Start(ctx)
	defer poller.Stop()
	defer cancel()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if calls.Load() >= 3 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got := calls.Load(); got < 3 {
		t.Fatalf("expected at least 3 polls (one per account), got %d", got)
	}

	for _, id := range []string{"a", "b", "c"} {
		u := pool.GetUsage(id)
		if u == nil {
			t.Errorf("account %s: cache empty after first pass", id)
			continue
		}
		if u.MonthlyCreditsRemaining != 750 {
			t.Errorf("account %s: remaining = %d, want 750", id, u.MonthlyCreditsRemaining)
		}
	}
}

// TestUsagePoller_BannedAccountStampedAsDrained verifies the banned
// classification path stores a sentinel marking the account drained,
// so weighted selection deweights it (verified in Package 3 tests).
func TestUsagePoller_BannedAccountStampedAsDrained(t *testing.T) {
	pool, vault := newPoolWithVault(t)
	seed(t, pool, vault, "victim")

	fakePoll := func(_ context.Context, _, _, _ string) (*kiroclient.UsageLimits, error) {
		return nil, &kiroclient.UsageError{
			Status: 403,
			Kind:   kiroclient.UsageErrKindBanned,
			Reason: "TemporarilySuspended",
		}
	}

	poller := NewUsagePoller(UsagePollerConfig{
		Pool: pool, Vault: vault, PollFn: fakePoll,
		Interval: 10 * time.Second,
	})
	ctx, cancel := context.WithCancel(context.Background())
	poller.Start(ctx)
	defer poller.Stop()
	defer cancel()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if pool.GetUsage("victim") != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	u := pool.GetUsage("victim")
	if u == nil {
		t.Fatal("banned account must still get a sentinel stamp")
	}
	if !u.IsExhausted() {
		t.Errorf("banned account sentinel must report exhausted=true, got %+v", u)
	}
}

// TestUsagePoller_TransientErrorKeepsStaleCache validates that a
// one-off failure does not erase the previous good reading.
func TestUsagePoller_TransientErrorKeepsStaleCache(t *testing.T) {
	pool, vault := newPoolWithVault(t)
	seed(t, pool, vault, "intermittent")

	good := &kiroclient.UsageLimits{
		MonthlyCap:              1000,
		MonthlyCreditsUsed:      100,
		MonthlyCreditsRemaining: 900,
		LastQueryTime:           time.Now(),
	}
	pool.SetUsage("intermittent", good)

	failing := func(_ context.Context, _, _, _ string) (*kiroclient.UsageLimits, error) {
		return nil, &kiroclient.UsageError{Status: 500, Kind: kiroclient.UsageErrKindTransient}
	}

	poller := NewUsagePoller(UsagePollerConfig{
		Pool: pool, Vault: vault, PollFn: failing,
		Interval: 10 * time.Second,
	})
	ctx, cancel := context.WithCancel(context.Background())
	poller.Start(ctx)
	defer poller.Stop()
	defer cancel()

	time.Sleep(200 * time.Millisecond)
	if u := pool.GetUsage("intermittent"); u == nil || u.MonthlyCreditsRemaining != 900 {
		t.Fatalf("transient failure must preserve stale cache, got %+v", u)
	}
}

// TestUsagePoller_ForcePollTriggersImmediateCall verifies that
// ForcePoll bypasses the regular tick.
func TestUsagePoller_ForcePollTriggersImmediateCall(t *testing.T) {
	pool, vault := newPoolWithVault(t)
	seed(t, pool, vault, "hot")

	var mu sync.Mutex
	var seen []string
	fakePoll := func(_ context.Context, token, _, _ string) (*kiroclient.UsageLimits, error) {
		mu.Lock()
		seen = append(seen, token)
		mu.Unlock()
		return &kiroclient.UsageLimits{MonthlyCap: 100, LastQueryTime: time.Now()}, nil
	}

	poller := NewUsagePoller(UsagePollerConfig{
		Pool:  pool,
		Vault: vault, PollFn: fakePoll,
		Interval:     1 * time.Hour,
		StartupDelay: 50 * time.Millisecond,
	})
	ctx, cancel := context.WithCancel(context.Background())
	poller.Start(ctx)
	defer poller.Stop()
	defer cancel()

	time.Sleep(150 * time.Millisecond)
	mu.Lock()
	first := len(seen)
	mu.Unlock()
	if first == 0 {
		t.Fatal("first pass should have polled at least once")
	}

	poller.ForcePoll("hot")
	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := len(seen)
		mu.Unlock()
		if n > first {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Errorf("ForcePoll did not trigger an additional call: seen=%d", first)
}

// TestUsagePoller_StopIdempotent ensures Stop can be called repeatedly
// without panicking on a closed channel.
func TestUsagePoller_StopIdempotent(t *testing.T) {
	pool, vault := newPoolWithVault(t)
	fakePoll := func(_ context.Context, _, _, _ string) (*kiroclient.UsageLimits, error) {
		return &kiroclient.UsageLimits{}, nil
	}
	poller := NewUsagePoller(UsagePollerConfig{
		Pool: pool, Vault: vault, PollFn: fakePoll,
	})
	poller.Start(context.Background())
	poller.Stop()
	poller.Stop()
}

// TestPool_GetUsage_NilSafeForUnknownAccount confirms the dashboard
// path doesn't panic on accounts that vanished mid-poll.
func TestPool_GetUsage_NilSafeForUnknownAccount(t *testing.T) {
	pool, _ := newPoolWithVault(t)
	if got := pool.GetUsage("ghost"); got != nil {
		t.Errorf("missing account should return nil, got %+v", got)
	}
}

// TestPool_SetUsage_OverwritesPrevious confirms repeated polls update
// in place, not append.
func TestPool_SetUsage_OverwritesPrevious(t *testing.T) {
	pool, vault := newPoolWithVault(t)
	seed(t, pool, vault, "x")

	pool.SetUsage("x", &kiroclient.UsageLimits{MonthlyCreditsRemaining: 100})
	pool.SetUsage("x", &kiroclient.UsageLimits{MonthlyCreditsRemaining: 50})

	u := pool.GetUsage("x")
	if u == nil || u.MonthlyCreditsRemaining != 50 {
		t.Errorf("expected latest sample (50), got %+v", u)
	}
}

// TestUsagePoller_PollFnNilTreatedAsDisabled is a smoke test for the
// guard inside Start.
func TestUsagePoller_PollFnNilTreatedAsDisabled(t *testing.T) {
	pool, vault := newPoolWithVault(t)
	seed(t, pool, vault, "a")
	poller := NewUsagePoller(UsagePollerConfig{Pool: pool, Vault: vault, PollFn: nil})
	poller.Start(context.Background())
	poller.Stop()
	if u := pool.GetUsage("a"); u != nil {
		t.Errorf("disabled poller must not populate cache, got %+v", u)
	}
}

// TestUsagePoller_VaultMissBundleIsSilent is a regression guard: when
// the vault returns ErrNotFound mid-poll (e.g. account just removed),
// the goroutine must continue rather than crash.
func TestUsagePoller_VaultMissBundleIsSilent(t *testing.T) {
	pool, vault := newPoolWithVault(t)
	pool.Add(Account{
		ID: "phantom", Provider: "kiro", Region: "us-east-1", Enabled: true,
	})
	defer pool.Remove("phantom")

	called := atomic.Int32{}
	fakePoll := func(_ context.Context, _, _, _ string) (*kiroclient.UsageLimits, error) {
		called.Add(1)
		return &kiroclient.UsageLimits{}, nil
	}
	poller := NewUsagePoller(UsagePollerConfig{
		Pool: pool, Vault: vault, PollFn: fakePoll,
		Interval: 10 * time.Second,
	})
	ctx, cancel := context.WithCancel(context.Background())
	poller.Start(ctx)
	defer poller.Stop()
	defer cancel()

	time.Sleep(200 * time.Millisecond)
	if called.Load() != 0 {
		t.Errorf("vault miss should skip PollFn, but called %d times", called.Load())
	}
}

// keep errors import alive for ad-hoc assertion needs in future tests
var _ = errors.New
