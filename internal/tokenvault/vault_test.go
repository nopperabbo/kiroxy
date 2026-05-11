package tokenvault

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func openVault(t *testing.T) *Vault {
	t.Helper()
	dir := t.TempDir()
	v, err := Open(context.Background(), filepath.Join(dir, "vault.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = v.Close() })
	return v
}

func TestSaveAndGet_RoundTrip(t *testing.T) {
	v := openVault(t)
	ctx := context.Background()
	b, err := v.Save(ctx, "kiro", "acct-1", Tokens{
		AccessToken:  "a1",
		RefreshToken: "r1",
		Source:       "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if b.Generation != 1 {
		t.Fatalf("want gen 1, got %d", b.Generation)
	}
	if b.AccessToken != "a1" || b.RefreshToken != "r1" {
		t.Fatalf("bad tokens round-trip: %+v", b)
	}

	got, err := v.Get(ctx, "kiro", "acct-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Generation != 1 || got.AccessToken != "a1" {
		t.Fatalf("get mismatch: %+v", got)
	}
}

func TestSave_IncrementsGenerationAndKeepsPrevious(t *testing.T) {
	v := openVault(t)
	ctx := context.Background()
	_, err := v.Save(ctx, "kiro", "acct-1", Tokens{AccessToken: "a1", RefreshToken: "r1"})
	if err != nil {
		t.Fatal(err)
	}
	b2, err := v.Save(ctx, "kiro", "acct-1", Tokens{AccessToken: "a2", RefreshToken: "r2"})
	if err != nil {
		t.Fatal(err)
	}
	if b2.Generation != 2 {
		t.Fatalf("want gen 2, got %d", b2.Generation)
	}
	if b2.PreviousRefreshToken != "r1" {
		t.Fatalf("previous refresh token not retained: %q", b2.PreviousRefreshToken)
	}
}

func TestReserve_LockRejectsSecondAttempt(t *testing.T) {
	v := openVault(t)
	ctx := context.Background()
	_, err := v.Save(ctx, "kiro", "acct-1", Tokens{AccessToken: "a1", RefreshToken: "r1"})
	if err != nil {
		t.Fatal(err)
	}

	_, gen1, err := v.Reserve(ctx, "kiro", "acct-1", 5*time.Second)
	if err != nil {
		t.Fatalf("first reserve failed: %v", err)
	}
	_, _, err = v.Reserve(ctx, "kiro", "acct-1", 5*time.Second)
	if !errors.Is(err, ErrLockHeld) {
		t.Fatalf("want ErrLockHeld on second reserve, got %v", err)
	}
	if err := v.Release(ctx, "kiro", "acct-1", gen1, false); err != nil {
		t.Fatalf("release: %v", err)
	}
}

func TestReserve_ExpiredLockIsReclaimable(t *testing.T) {
	v := openVault(t)
	ctx := context.Background()
	_, err := v.Save(ctx, "kiro", "acct-1", Tokens{AccessToken: "a1", RefreshToken: "r1"})
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = v.Reserve(ctx, "kiro", "acct-1", 10*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(30 * time.Millisecond)
	_, _, err = v.Reserve(ctx, "kiro", "acct-1", 5*time.Second)
	if err != nil {
		t.Fatalf("second reserve after TTL expiry should succeed, got %v", err)
	}
}

func TestCommit_RejectsStaleGeneration(t *testing.T) {
	v := openVault(t)
	ctx := context.Background()
	_, err := v.Save(ctx, "kiro", "acct-1", Tokens{AccessToken: "a1", RefreshToken: "r1"})
	if err != nil {
		t.Fatal(err)
	}
	_, gen, err := v.Reserve(ctx, "kiro", "acct-1", 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	_, err = v.Save(ctx, "kiro", "acct-1", Tokens{AccessToken: "ax", RefreshToken: "rx"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = v.Commit(ctx, "kiro", "acct-1", gen, Tokens{AccessToken: "a2", RefreshToken: "r2"})
	if !errors.Is(err, ErrGenerationStale) {
		t.Fatalf("want ErrGenerationStale, got %v", err)
	}
}

// TestRefresh_ConcurrentCallersProduceExactlyOneUpstreamCall is the M4 gate.
// N goroutines call Refresh simultaneously; exactly one wins Reserve and runs
// the RefreshFunc, the others return ErrLockHeld. After the winner commits,
// all subsequent Gets see the new access token.
func TestRefresh_ConcurrentCallersProduceExactlyOneUpstreamCall(t *testing.T) {
	v := openVault(t)
	ctx := context.Background()
	_, err := v.Save(ctx, "kiro", "acct-1", Tokens{AccessToken: "a1", RefreshToken: "r1"})
	if err != nil {
		t.Fatal(err)
	}

	const concurrency = 50
	var upstreamCalls atomic.Int32
	fn := func(ctx context.Context, _ string) (Tokens, error) {
		upstreamCalls.Add(1)
		time.Sleep(50 * time.Millisecond)
		return Tokens{AccessToken: "a2", RefreshToken: "r2", Source: "refresh"}, nil
	}

	var wg sync.WaitGroup
	results := make([]error, concurrency)
	start := make(chan struct{})
	for i := range concurrency {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start
			_, err := v.Refresh(ctx, "kiro", "acct-1", 5*time.Second, fn)
			results[idx] = err
		}(i)
	}
	close(start)
	wg.Wait()

	if n := upstreamCalls.Load(); n != 1 {
		t.Fatalf("want exactly 1 upstream refresh call, got %d", n)
	}

	var commits, held int
	for _, err := range results {
		switch {
		case err == nil:
			commits++
		case errors.Is(err, ErrLockHeld):
			held++
		default:
			t.Errorf("unexpected error: %v", err)
		}
	}
	if commits != 1 {
		t.Errorf("want 1 successful commit, got %d", commits)
	}
	if held != concurrency-1 {
		t.Errorf("want %d ErrLockHeld, got %d", concurrency-1, held)
	}

	b, err := v.Get(ctx, "kiro", "acct-1")
	if err != nil {
		t.Fatal(err)
	}
	if b.AccessToken != "a2" || b.RefreshToken != "r2" {
		t.Fatalf("post-refresh bundle missing new tokens: %+v", b)
	}
	if b.PreviousRefreshToken != "r1" {
		t.Fatalf("previous refresh token not retained: %q", b.PreviousRefreshToken)
	}
	if b.Generation != 2 {
		t.Fatalf("want gen 2 after refresh, got %d", b.Generation)
	}
}

func TestRefresh_UpstreamErrorReleasesLock(t *testing.T) {
	v := openVault(t)
	ctx := context.Background()
	_, err := v.Save(ctx, "kiro", "acct-1", Tokens{AccessToken: "a1", RefreshToken: "r1"})
	if err != nil {
		t.Fatal(err)
	}

	refreshErr := fmt.Errorf("simulated upstream 5xx")
	_, err = v.Refresh(ctx, "kiro", "acct-1", 5*time.Second, func(_ context.Context, _ string) (Tokens, error) {
		return Tokens{}, refreshErr
	})
	if !errors.Is(err, refreshErr) {
		t.Fatalf("want upstream error, got %v", err)
	}

	b, err := v.Get(ctx, "kiro", "acct-1")
	if err != nil {
		t.Fatal(err)
	}
	if b.RefreshInProgress {
		t.Fatalf("lock not released after upstream error: %+v", b)
	}
	if b.AccessToken != "a1" {
		t.Fatalf("access token was rotated despite upstream error: %+v", b)
	}
}

func TestListByProvider(t *testing.T) {
	v := openVault(t)
	ctx := context.Background()
	for _, id := range []string{"c", "a", "b"} {
		if _, err := v.Save(ctx, "kiro", id, Tokens{AccessToken: id + "-at", RefreshToken: id + "-rt"}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := v.Save(ctx, "codebuddy", "d", Tokens{AccessToken: "d-at", RefreshToken: "d-rt"}); err != nil {
		t.Fatal(err)
	}
	kiros, err := v.ListByProvider(ctx, "kiro")
	if err != nil {
		t.Fatal(err)
	}
	if len(kiros) != 3 {
		t.Fatalf("want 3 kiro bundles, got %d", len(kiros))
	}
	gotOrder := []string{kiros[0].ConnectionID, kiros[1].ConnectionID, kiros[2].ConnectionID}
	wantOrder := []string{"a", "b", "c"}
	for i := range wantOrder {
		if gotOrder[i] != wantOrder[i] {
			t.Errorf("ordering: got %v, want %v", gotOrder, wantOrder)
			break
		}
	}
}

func TestGet_NotFound(t *testing.T) {
	v := openVault(t)
	_, err := v.Get(context.Background(), "kiro", "nope")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}
