package tokenvault

import (
	"context"
	"strings"
	"testing"
)

func TestInboundKey_CreateAndVerify(t *testing.T) {
	v := openTestVault(t)
	ctx := context.Background()

	id, plain, err := v.CreateInboundKey(ctx, "ci")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !strings.HasPrefix(plain, "kxy_") {
		t.Fatalf("want kxy_ prefix, got %q", plain)
	}
	if id == "" {
		t.Fatalf("want non-empty id")
	}

	gotID, err := v.VerifyInboundKey(ctx, plain)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if gotID != id {
		t.Fatalf("want id=%s, got %s", id, gotID)
	}
}

func TestInboundKey_VerifyRejectsUnknown(t *testing.T) {
	v := openTestVault(t)
	ctx := context.Background()

	_, _, err := v.CreateInboundKey(ctx, "real")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := v.VerifyInboundKey(ctx, "kxy_unknownkey"); err == nil {
		t.Fatalf("want error for unknown key")
	}
	if _, err := v.VerifyInboundKey(ctx, "no-prefix"); err != ErrInboundKeyInvalid {
		t.Fatalf("want ErrInboundKeyInvalid for missing prefix, got %v", err)
	}
}

func TestInboundKey_RevokeBlocksVerify(t *testing.T) {
	v := openTestVault(t)
	ctx := context.Background()

	id, plain, err := v.CreateInboundKey(ctx, "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := v.RevokeInboundKey(ctx, id); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if _, err := v.VerifyInboundKey(ctx, plain); err != ErrInboundKeyRevoked {
		t.Fatalf("want ErrInboundKeyRevoked, got %v", err)
	}
	if err := v.RevokeInboundKey(ctx, "doesnotexist"); err != ErrInboundKeyNotFound {
		t.Fatalf("want ErrInboundKeyNotFound, got %v", err)
	}
}

func TestInboundKey_ListAndCount(t *testing.T) {
	v := openTestVault(t)
	ctx := context.Background()

	id1, _, _ := v.CreateInboundKey(ctx, "first")
	_, _, _ = v.CreateInboundKey(ctx, "second")
	if err := v.RevokeInboundKey(ctx, id1); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	keys, err := v.ListInboundKeys(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("want 2 keys, got %d", len(keys))
	}
	for _, k := range keys {
		if len(k.Tail) != 4 {
			t.Fatalf("want 4-char tail, got %q", k.Tail)
		}
	}
	active, total, err := v.CountInboundKeys(ctx)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if total != 2 || active != 1 {
		t.Fatalf("want active=1 total=2, got active=%d total=%d", active, total)
	}
}

func TestInboundKey_HashNeverInList(t *testing.T) {
	v := openTestVault(t)
	ctx := context.Background()

	_, plain, _ := v.CreateInboundKey(ctx, "secret-test")
	keys, err := v.ListInboundKeys(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("want 1 key, got %d", len(keys))
	}
	for _, field := range []string{keys[0].ID, keys[0].Label, keys[0].Tail} {
		if strings.Contains(plain, field) && field != keys[0].Tail {
			t.Fatalf("plaintext leaked into id/label: %q vs %q", plain, field)
		}
	}
	if !strings.HasSuffix(plain, keys[0].Tail) {
		t.Fatalf("tail mismatch: plain=%q tail=%q", plain, keys[0].Tail)
	}
}

func TestInboundKey_LastUsedAtTouches(t *testing.T) {
	v := openTestVault(t)
	ctx := context.Background()

	_, plain, _ := v.CreateInboundKey(ctx, "")
	keys, _ := v.ListInboundKeys(ctx)
	if !keys[0].LastUsedAt.IsZero() {
		t.Fatalf("expect zero last_used_at before verify")
	}
	if _, err := v.VerifyInboundKey(ctx, plain); err != nil {
		t.Fatalf("verify: %v", err)
	}
	keys2, _ := v.ListInboundKeys(ctx)
	if keys2[0].LastUsedAt.IsZero() {
		t.Fatalf("expect non-zero last_used_at after verify")
	}
}
