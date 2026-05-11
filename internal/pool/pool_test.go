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

func TestPick_LRUOrderAcross3Accounts(t *testing.T) {
	p, v := newPoolWithVault(t)
	seed(t, p, v, "a", "b", "c")

	pickID := func() string {
		r, err := p.Pick(context.Background(), v)
		if err != nil {
			t.Fatal(err)
		}
		return r.ID
	}

	first := pickID()
	second := pickID()
	third := pickID()
	if first == second || first == third || second == third {
		t.Fatalf("LRU gave duplicate picks across 3 accounts: %s %s %s", first, second, third)
	}

	fourth := pickID()
	if fourth != first {
		t.Fatalf("4th pick should cycle back to LRU-oldest (%s), got %s", first, fourth)
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

	sawA := false
	for i := 0; i < 10; i++ {
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

func TestM5_LoadTestLRURotationAndFailedSkip(t *testing.T) {
	p, v := newPoolWithVault(t)
	seed(t, p, v, "a", "b", "c")

	p.RecordFailure("b", FailureQuota, "429")

	counts := map[string]int{}
	for range 30 {
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
	if diff := counts["a"] - counts["c"]; diff < -1 || diff > 1 {
		t.Errorf("expected LRU to balance a vs c within 1; counts=%+v", counts)
	}
}
