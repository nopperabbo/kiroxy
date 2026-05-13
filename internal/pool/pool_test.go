package pool

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"local/kiroxy/internal/tokenvault"
)

func newPoolWithVault(t *testing.T) (*Pool, *tokenvault.Vault) {
	t.Helper()
	dir := t.TempDir()
	v, err := tokenvault.Open(context.Background(), filepath.Join(dir, "vault.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = v.Close() })
	return New(Policy{
		ConsecutiveErrorThreshold: 3,
		ShortCooldown:             50 * time.Millisecond,
		QuotaCooldown:             100 * time.Millisecond,
		MaxCooldown:               1 * time.Second,
	}), v
}

func seed(t *testing.T, p *Pool, v *tokenvault.Vault, ids ...string) {
	t.Helper()
	for _, id := range ids {
		p.Add(Account{
			ID:       id,
			Label:    "label-" + id,
			Provider: "kiro",
			Region:   "us-east-1",
			Enabled:  true,
		})
		if _, err := v.Save(context.Background(), "kiro", id, tokenvault.Tokens{
			AccessToken:  "at-" + id,
			RefreshToken: "rt-" + id,
		}); err != nil {
			t.Fatalf("seed vault: %v", err)
		}
	}
}

func TestPick_EmptyPoolReturnsErrNoAccount(t *testing.T) {
	p, v := newPoolWithVault(t)
	if _, err := p.Pick(context.Background(), v); !errors.Is(err, ErrNoAccount) {
		t.Fatalf("want ErrNoAccount, got %v", err)
	}
}

func TestPick_DistributesAcross3Accounts(t *testing.T) {
	// Under weighted random (equal weights for fresh accounts), selection
	// is non-deterministic per call, but every account must be picked
	// somewhat evenly over a large sample. With 3 accounts and 300 picks,
	// each account should land near 100 picks (chi² well within tolerance).
	p, v := newPoolWithVault(t)
	seed(t, p, v, "a", "b", "c")

	counts := map[string]int{}
	for i := 0; i < 300; i++ {
		r, err := p.Pick(context.Background(), v)
		if err != nil {
			t.Fatal(err)
		}
		counts[r.ID]++
	}
	if len(counts) != 3 {
		t.Fatalf("expected all 3 accounts exercised, got %v", counts)
	}
	for _, id := range []string{"a", "b", "c"} {
		if counts[id] < 50 || counts[id] > 150 {
			t.Errorf("account %s share %d out of 300 drifts outside [50,150]; counts=%v", id, counts[id], counts)
		}
	}
}

func TestRecordFailure_QuotaCooldownSkipsAccount(t *testing.T) {
	p, v := newPoolWithVault(t)
	seed(t, p, v, "a", "b")

	p.RecordFailure("a", FailureQuota, "429")

	for i := 0; i < 5; i++ {
		r, err := p.Pick(context.Background(), v)
		if err != nil {
			t.Fatalf("pick %d: %v", i, err)
		}
		if r.ID == "a" {
			t.Fatalf("expected a to be on cooldown, but it was picked")
		}
	}

	time.Sleep(150 * time.Millisecond)

	// After QuotaCooldown the hard gate lifts, but the health ring still
	// records the recent rate-limit event. Give it a larger window to
	// pick 'a' via weighted random: with weight(a) ≈ 0.1 * low-rate and
	// weight(b) ≈ 1.0, P(a) ≈ 0.1; 60 trials gives P(never picked) ≈ 0.002.
	sawA := false
	for i := 0; i < 60; i++ {
		r, err := p.Pick(context.Background(), v)
		if err != nil {
			t.Fatal(err)
		}
		if r.ID == "a" {
			sawA = true
			break
		}
	}
	if !sawA {
		t.Fatalf("account a never came back after QuotaCooldown expired")
	}
}

func TestRecordFailure_TransientHitsThresholdThenCools(t *testing.T) {
	p, v := newPoolWithVault(t)
	seed(t, p, v, "a")

	p.RecordFailure("a", FailureTransient, "500")
	p.RecordFailure("a", FailureTransient, "500")
	if _, err := p.Pick(context.Background(), v); err != nil {
		t.Fatalf("after 2 errors, a should still be pickable, got %v", err)
	}
	p.RecordFailure("a", FailureTransient, "500")
	if _, err := p.Pick(context.Background(), v); !errors.Is(err, ErrNoAccount) {
		t.Fatalf("after 3 errors, a should be on cooldown (no other accounts), got %v", err)
	}
}

func TestRecordSuccess_ClearsCooldown(t *testing.T) {
	p, v := newPoolWithVault(t)
	seed(t, p, v, "a")

	for range 3 {
		p.RecordFailure("a", FailureTransient, "500")
	}
	if _, err := p.Pick(context.Background(), v); !errors.Is(err, ErrNoAccount) {
		t.Fatalf("cooldown should block pick; got %v", err)
	}
	p.RecordSuccess("a")
	if _, err := p.Pick(context.Background(), v); err != nil {
		t.Fatalf("after RecordSuccess cooldown should be cleared; got %v", err)
	}
}

func TestPick_DisabledAccountSkipped(t *testing.T) {
	p, v := newPoolWithVault(t)
	seed(t, p, v, "a", "b")

	a := p.accounts["a"]
	a.Enabled = false

	for range 5 {
		r, err := p.Pick(context.Background(), v)
		if err != nil {
			t.Fatal(err)
		}
		if r.ID == "a" {
			t.Fatalf("disabled account 'a' was picked")
		}
	}
}

func TestList_StableOrder(t *testing.T) {
	p, v := newPoolWithVault(t)
	seed(t, p, v, "z", "a", "m")
	snap := p.List()
	if len(snap) != 3 {
		t.Fatalf("want 3, got %d", len(snap))
	}
	want := []string{"a", "m", "z"}
	for i, acc := range snap {
		if acc.ID != want[i] {
			t.Errorf("index %d: want %s, got %s", i, want[i], acc.ID)
		}
	}
}

func TestM5_WeightedPickSkipsFailedAccount(t *testing.T) {
	p, v := newPoolWithVault(t)
	seed(t, p, v, "a", "b", "c")

	p.RecordFailure("b", FailureQuota, "429")

	counts := map[string]int{}
	for range 60 {
		r, err := p.Pick(context.Background(), v)
		if err != nil {
			t.Fatalf("pick: %v", err)
		}
		counts[r.ID]++
	}

	if counts["b"] != 0 {
		t.Errorf("expected b to be skipped (cooldown), got %d picks", counts["b"])
	}
	if counts["a"] == 0 || counts["c"] == 0 {
		t.Errorf("expected a and c each picked at least once; counts=%+v", counts)
	}
	// Under weighted random with equal fresh weights, a vs c should
	// balance within statistical tolerance ±50% across 60 trials.
	if diff := counts["a"] - counts["c"]; diff < -30 || diff > 30 {
		t.Errorf("a vs c imbalance beyond ±50%% of 60 picks; counts=%+v", counts)
	}
}
