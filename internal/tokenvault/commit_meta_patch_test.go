package tokenvault

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func openForMetaTest(t *testing.T) *Vault {
	t.Helper()
	dir := t.TempDir()
	v, err := Open(context.Background(), filepath.Join(dir, "vault.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = v.Close() })
	return v
}

func TestCommitWithMetaPatch_MergesExistingMetadata(t *testing.T) {
	ctx := context.Background()
	v := openForMetaTest(t)

	if _, err := v.Save(ctx, "kiro", "acct1", Tokens{
		AccessToken:  "at-initial",
		RefreshToken: "rt-initial",
		Source:       "import-accounts-json",
		Metadata:     `{"auth_method":"social","profile_arn":"arn:initial","expires_in":3600}`,
	}); err != nil {
		t.Fatal(err)
	}
	rt, gen, err := v.Reserve(ctx, "kiro", "acct1", 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if rt != "rt-initial" {
		t.Fatalf("Reserve returned rt=%q", rt)
	}
	_, err = v.CommitWithMetaPatch(ctx, "kiro", "acct1", gen, Tokens{
		AccessToken:  "at-rotated",
		RefreshToken: "rt-rotated",
		Source:       "pool-refresh",
	}, map[string]any{
		"expires_at":   time.Now().Unix() + 3600,
		"refreshed_at": "2026-05-12T15:00:00Z",
	})
	if err != nil {
		t.Fatalf("CommitWithMetaPatch: %v", err)
	}
	b, err := v.Get(ctx, "kiro", "acct1")
	if err != nil {
		t.Fatal(err)
	}
	if b.AccessToken != "at-rotated" {
		t.Errorf("AccessToken = %q, want at-rotated", b.AccessToken)
	}
	if b.Generation != 2 {
		t.Errorf("Generation = %d, want 2", b.Generation)
	}
	if !strings.Contains(b.Metadata, `"auth_method":"social"`) {
		t.Errorf("lost auth_method: %s", b.Metadata)
	}
	if !strings.Contains(b.Metadata, `"profile_arn":"arn:initial"`) {
		t.Errorf("lost profile_arn: %s", b.Metadata)
	}
	if !strings.Contains(b.Metadata, `"refreshed_at":"2026-05-12T15:00:00Z"`) {
		t.Errorf("missing refreshed_at: %s", b.Metadata)
	}
}

func TestCommitWithMetaPatch_StaleGenerationRejected(t *testing.T) {
	ctx := context.Background()
	v := openForMetaTest(t)

	if _, err := v.Save(ctx, "kiro", "acct1", Tokens{
		AccessToken:  "at-v1",
		RefreshToken: "rt-v1",
		Source:       "test",
		Metadata:     `{"auth_method":"social"}`,
	}); err != nil {
		t.Fatal(err)
	}
	_, genA, err := v.Reserve(ctx, "kiro", "acct1", 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.Commit(ctx, "kiro", "acct1", genA, Tokens{
		AccessToken:  "at-v2",
		RefreshToken: "rt-v2",
		Source:       "test",
	}); err != nil {
		t.Fatalf("first Commit: %v", err)
	}
	_, cerr := v.CommitWithMetaPatch(ctx, "kiro", "acct1", genA, Tokens{
		AccessToken:  "at-v3-should-be-rejected",
		RefreshToken: "rt-v3-should-be-rejected",
		Source:       "pool-refresh",
	}, map[string]any{"expires_at": int64(999)})
	if cerr == nil {
		t.Fatal("expected ErrGenerationStale, got nil")
	}
	if !errors.Is(cerr, ErrGenerationStale) {
		t.Errorf("want ErrGenerationStale, got %v", cerr)
	}
	b, err := v.Get(ctx, "kiro", "acct1")
	if err != nil {
		t.Fatal(err)
	}
	if b.AccessToken != "at-v2" {
		t.Errorf("bundle mutated by stale commit: AccessToken = %q", b.AccessToken)
	}
	if b.Generation != 2 {
		t.Errorf("generation advanced on stale commit: %d", b.Generation)
	}
}

func TestCommitWithMetaPatch_MalformedExistingMetadata(t *testing.T) {
	ctx := context.Background()
	v := openForMetaTest(t)

	if _, err := v.Save(ctx, "kiro", "acct1", Tokens{
		AccessToken:  "at-initial",
		RefreshToken: "rt-initial",
		Source:       "test",
		Metadata:     "not-json{",
	}); err != nil {
		t.Fatal(err)
	}
	_, gen, err := v.Reserve(ctx, "kiro", "acct1", 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.CommitWithMetaPatch(ctx, "kiro", "acct1", gen, Tokens{
		AccessToken:  "at-rotated",
		RefreshToken: "rt-rotated",
		Source:       "pool-refresh",
	}, map[string]any{"expires_at": int64(1234567)}); err != nil {
		t.Fatalf("CommitWithMetaPatch on malformed meta: %v", err)
	}
	b, _ := v.Get(ctx, "kiro", "acct1")
	if !strings.Contains(b.Metadata, `"expires_at":1234567`) {
		t.Errorf("expected patch-only metadata, got %s", b.Metadata)
	}
}
