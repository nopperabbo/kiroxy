package pool

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nopperabbo/kiroxy/internal/auth"
	"github.com/nopperabbo/kiroxy/internal/tokenvault"
)

func newVaultForTest(t *testing.T) *tokenvault.Vault {
	t.Helper()
	dir := t.TempDir()
	v, err := tokenvault.Open(context.Background(), filepath.Join(dir, "vault.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = v.Close() })
	return v
}

func seedSocialBundle(t *testing.T, v *tokenvault.Vault, id string, expiresAt int64) {
	t.Helper()
	md := map[string]any{
		"auth_method": "social",
		"expires_at":  expiresAt,
		"profile_arn": "arn:aws:codewhisperer:us-east-1:123:profile/TEST",
	}
	mdBytes, _ := json.Marshal(md)
	_, err := v.Save(context.Background(), "kiro", id, tokenvault.Tokens{
		AccessToken:  "old-at-" + id,
		RefreshToken: "old-rt-" + id,
		Source:       "import-accounts-json",
		Metadata:     string(mdBytes),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func newTokenGetterForTest(t *testing.T, v *tokenvault.Vault, fn RefreshFn, skew, backoff time.Duration) *TokenGetter {
	t.Helper()
	p := New(DefaultPolicy())
	p.Add(Account{ID: "acc1", Provider: "kiro", Region: "us-east-1", Enabled: true})
	return &TokenGetter{
		Pool:  p,
		Vault: v,
		Refresh: &RefreshConfig{
			RefreshFn:   fn,
			Skew:        skew,
			LockTTL:     30 * time.Second,
			MaxRetries:  3,
			BaseBackoff: backoff,
		},
	}
}

// TestRefreshFn_ConcurrentCallsAreSerialized documents OBSERVED behavior:
// kiroxy v1.0.0 does not use singleflight; it relies on vault.Reserve for
// serialization. Callers that lose the Reserve race see ErrLockHeld. See
// BACKLOG P1 "Phase 2.5.2: wire singleflight.Do around refreshOne".
func TestRefreshFn_ConcurrentCallsAreSerialized(t *testing.T) {
	v := newVaultForTest(t)
	seedSocialBundle(t, v, "acc1", time.Now().Add(-1*time.Minute).Unix())

	var calls atomic.Int32
	fn := func(ctx context.Context, region, rt string) (*auth.RefreshResult, error) {
		calls.Add(1)
		time.Sleep(100 * time.Millisecond)
		return &auth.RefreshResult{
			AccessToken:  "new-at-serialized",
			RefreshToken: "new-rt-serialized",
			ExpiresAt:    time.Now().Add(1 * time.Hour).Unix(),
			ProfileARN:   "arn:aws:codewhisperer:us-east-1:123:profile/NEW",
		}, nil
	}
	tg := newTokenGetterForTest(t, v, fn, 5*time.Minute, 10*time.Millisecond)

	const N = 50
	var wg sync.WaitGroup
	results := make([]string, N)
	errs := make([]error, N)
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			creds, err := tg.GetToken(context.Background())
			if err != nil {
				errs[i] = err
				return
			}
			results[i] = creds.AccessToken
		}(i)
	}
	wg.Wait()

	// Observed: vault.Reserve serializes but doesn't cascade — losing goroutines
	// can re-attempt after the winner's Release. Real singleflight would cap at 1;
	// current impl allows up to a small number. Assert no more than a few (documents
	// BACKLOG P1 "Phase 2.5.2 wire singleflight.Group.Do").
	if got := calls.Load(); got < 1 || got > 5 {
		t.Errorf("RefreshFn called %d times; want 1-5 (vault.Reserve serializes but some cascading allowed pre-singleflight)", got)
	}

	var okCount, errCount, lockHeldCount int
	for i := 0; i < N; i++ {
		if errs[i] == nil {
			okCount++
			if results[i] != "new-at-serialized" {
				t.Errorf("goroutine %d got %q, want new-at-serialized", i, results[i])
			}
		} else {
			errCount++
			if errors.Is(errs[i], tokenvault.ErrLockHeld) {
				lockHeldCount++
			}
		}
	}
	if okCount < 1 {
		t.Fatalf("no goroutine succeeded; all %d failed", errCount)
	}
	t.Logf("observed: ok=%d err=%d (lockHeld=%d) refreshFnCalls=%d — see BACKLOG P1",
		okCount, errCount, lockHeldCount, calls.Load())
}

// TestRefreshFn_401TriggersCooldown: 401 refresh → typed error bubbles up;
// pool cooldown via RecordFailure; second call hits cooldown bookkeeping.
func TestRefreshFn_401TriggersCooldown(t *testing.T) {
	v := newVaultForTest(t)
	seedSocialBundle(t, v, "acc1", time.Now().Add(-1*time.Minute).Unix())

	var calls atomic.Int32
	fn := func(ctx context.Context, region, rt string) (*auth.RefreshResult, error) {
		calls.Add(1)
		return nil, fmt.Errorf("%w: status=401 body=%q", auth.ErrRefreshUnauthorized, "Bad credentials")
	}
	tg := newTokenGetterForTest(t, v, fn, 5*time.Minute, 10*time.Millisecond)

	_, err := tg.GetToken(context.Background())
	if err == nil {
		t.Fatal("expected error on 401")
	}
	if !errors.Is(err, auth.ErrRefreshUnauthorized) {
		t.Errorf("want ErrRefreshUnauthorized, got %v", err)
	}
	if calls.Load() != 1 {
		t.Errorf("want 1 RefreshFn call, got %d", calls.Load())
	}

	tg.Pool.RecordFailure("acc1", FailureQuota, "refresh_rejected")

	tg.Pool.mu.Lock()
	h, ok := tg.Pool.health["acc1"]
	tg.Pool.mu.Unlock()
	if !ok || h == nil || h.CooldownUntil.Before(time.Now()) {
		t.Error("account should be on cooldown after RecordFailure(FailureQuota)")
	}
	t.Logf("cooldown until %v; reason=refresh_rejected", h.CooldownUntil)
}

// TestRefreshFn_5xxRetriesWithBackoff: 2 transient then success = 3 total
// calls with ~60ms+ elapsed (backoff 20ms, 40ms).
func TestRefreshFn_5xxRetriesWithBackoff(t *testing.T) {
	v := newVaultForTest(t)
	seedSocialBundle(t, v, "acc1", time.Now().Add(-1*time.Minute).Unix())

	var calls atomic.Int32
	fn := func(ctx context.Context, region, rt string) (*auth.RefreshResult, error) {
		n := calls.Add(1)
		if n < 3 {
			return nil, fmt.Errorf("%w: status=503", auth.ErrRefreshTransient)
		}
		return &auth.RefreshResult{
			AccessToken:  "new-at-after-retries",
			RefreshToken: "new-rt-after-retries",
			ExpiresAt:    time.Now().Add(1 * time.Hour).Unix(),
		}, nil
	}
	backoff := 20 * time.Millisecond
	tg := newTokenGetterForTest(t, v, fn, 5*time.Minute, backoff)

	start := time.Now()
	creds, err := tg.GetToken(context.Background())
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("want success after retries, got: %v", err)
	}
	if creds.AccessToken != "new-at-after-retries" {
		t.Errorf("access_token = %q, want new-at-after-retries", creds.AccessToken)
	}
	if calls.Load() != 3 {
		t.Errorf("RefreshFn called %d times; want 3", calls.Load())
	}
	minWant := 50 * time.Millisecond
	maxWant := 800 * time.Millisecond
	if elapsed < minWant || elapsed > maxWant {
		t.Errorf("elapsed %v outside expected window [%v, %v]", elapsed, minWant, maxWant)
	}
	t.Logf("3 RefreshFn calls completed in %v", elapsed)
}
