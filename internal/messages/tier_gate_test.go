// kiroxy addition (not derived from upstream).
//
// Tests for tier gating at the messages handler. Cover the four gate
// states: looker absent (test fixture), no snapshot, mismatch warn-only,
// mismatch strict-refuse. Strict refuse is the only path that returns
// true (refused); all others fall through.

package messages

import (
	"context"
	"errors"
	"testing"

	"github.com/nopperabbo/kiroxy/internal/auth"
	"github.com/nopperabbo/kiroxy/internal/kiroclient"
)

// fakeAuthNoLooker satisfies TokenGetter only; UsageLimitsLooker assertion
// in checkTierGate must fall through cleanly.
type fakeAuthNoLooker struct{}

func (f *fakeAuthNoLooker) GetToken(ctx context.Context) (*auth.Credentials, error) {
	return nil, errors.New("not used in this test")
}

// fakeAuthWithLooker satisfies both TokenGetter and UsageLimitsLooker.
// snapshot may be nil to simulate "polling not yet ready" or carry a
// SubscriptionType so the gate has a real tier to compare against.
type fakeAuthWithLooker struct {
	snapshot *kiroclient.UsageLimits
}

func (f *fakeAuthWithLooker) GetToken(ctx context.Context) (*auth.Credentials, error) {
	return nil, errors.New("not used in this test")
}

func (f *fakeAuthWithLooker) GetUsage(accountID string) *kiroclient.UsageLimits {
	return f.snapshot
}

func TestCheckTierGate_NoCreds(t *testing.T) {
	s := &Service{auth: &fakeAuthWithLooker{}}
	if s.checkTierGate(context.Background(), "trace", nil, "claude-sonnet-4.6") {
		t.Fatal("nil creds must never refuse")
	}
}

func TestCheckTierGate_LookerAbsent(t *testing.T) {
	s := &Service{auth: &fakeAuthNoLooker{}}
	creds := &auth.Credentials{AccountID: "acct-1"}
	if s.checkTierGate(context.Background(), "trace", creds, "claude-sonnet-4.6") {
		t.Fatal("looker-absent must never refuse — fail-open contract")
	}
}

func TestCheckTierGate_NoSnapshot(t *testing.T) {
	s := &Service{auth: &fakeAuthWithLooker{snapshot: nil}}
	creds := &auth.Credentials{AccountID: "acct-1"}
	if s.checkTierGate(context.Background(), "trace", creds, "claude-sonnet-4.6") {
		t.Fatal("no-snapshot must never refuse — fail-open contract")
	}
}

func TestCheckTierGate_TierSatisfied(t *testing.T) {
	s := &Service{auth: &fakeAuthWithLooker{
		snapshot: &kiroclient.UsageLimits{SubscriptionType: "Q_DEVELOPER_STANDALONE_PRO"},
	}}
	creds := &auth.Credentials{AccountID: "acct-1"}
	if s.checkTierGate(context.Background(), "trace", creds, "claude-sonnet-4.6") {
		t.Fatal("Pro account / Sonnet model must not refuse")
	}
}

func TestCheckTierGate_TierMismatchWarnOnly(t *testing.T) {
	t.Setenv("KIROXY_TIER_STRICT", "")
	s := &Service{auth: &fakeAuthWithLooker{
		snapshot: &kiroclient.UsageLimits{SubscriptionType: "Q_DEVELOPER_STANDALONE_FREE"},
	}}
	creds := &auth.Credentials{AccountID: "acct-1"}
	if s.checkTierGate(context.Background(), "trace", creds, "claude-sonnet-4.6") {
		t.Fatal("warn-only mode must not refuse, even on tier mismatch")
	}
}

func TestCheckTierGate_TierMismatchStrictRefuses(t *testing.T) {
	t.Setenv("KIROXY_TIER_STRICT", "1")
	s := &Service{auth: &fakeAuthWithLooker{
		snapshot: &kiroclient.UsageLimits{SubscriptionType: "Q_DEVELOPER_STANDALONE_FREE"},
	}}
	creds := &auth.Credentials{AccountID: "acct-1"}
	if !s.checkTierGate(context.Background(), "trace", creds, "claude-sonnet-4.6") {
		t.Fatal("strict-mode tier mismatch must refuse")
	}
}

func TestCheckTierGate_HaikuOnFreeAllowed(t *testing.T) {
	t.Setenv("KIROXY_TIER_STRICT", "1")
	s := &Service{auth: &fakeAuthWithLooker{
		snapshot: &kiroclient.UsageLimits{SubscriptionType: "Q_DEVELOPER_STANDALONE_FREE"},
	}}
	creds := &auth.Credentials{AccountID: "acct-1"}
	if s.checkTierGate(context.Background(), "trace", creds, "claude-haiku-4.5") {
		t.Fatal("Free account / Haiku must not refuse, even in strict mode")
	}
}
