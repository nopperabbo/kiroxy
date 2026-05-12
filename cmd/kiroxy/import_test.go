package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"local/kiroxy/internal/tokenvault"
)

func TestParseTriplets_HappyPath(t *testing.T) {
	in := strings.NewReader(`
# a comment
alice@example.com:rt-alice-xxxx:sig-alice
bob@example.com:rt-bob-xxxxxxxxxxx:sig-bob
carol@example.com:rt-carol-xxxxx:
`)
	trips, errs := parseTriplets(in)
	if len(trips) != 3 {
		t.Fatalf("want 3 triplets, got %d (errs=%v)", len(trips), errs)
	}
	if trips[2].signature != "" {
		t.Errorf("empty signature should parse as empty, got %q", trips[2].signature)
	}
	if len(errs) != 0 {
		t.Errorf("want no errors, got %v", errs)
	}
}

func TestParseTriplets_InvalidLinesSkipped(t *testing.T) {
	in := strings.NewReader(`
alice@example.com:rt-alice-xxxx:sig
notanemail:rt-no-email:sig
bob@example.com:short:sig
carol@example.com
dup@example.com:rt-dup-xxxxxxxxxxx:sig
dup@example.com:rt-dup-other-xxxxx:sig
eve@example.com:rt-eve-xxxxxxxxxxx:sig
`)
	trips, errs := parseTriplets(in)
	wantValid := []string{"alice@example.com", "dup@example.com", "eve@example.com"}
	if len(trips) != len(wantValid) {
		t.Fatalf("want %d valid, got %d (trips=%+v errs=%v)", len(wantValid), len(trips), trips, errs)
	}
	for i, want := range wantValid {
		if trips[i].email != want {
			t.Errorf("triplet[%d]: want %s, got %s", i, want, trips[i].email)
		}
	}
	if len(errs) < 4 {
		t.Errorf("expected \u22654 skip reasons (bad email, short rt, no colon, duplicate), got %d: %v", len(errs), errs)
	}
}

func TestParseTriplets_EmptyInput(t *testing.T) {
	trips, errs := parseTriplets(strings.NewReader(""))
	if len(trips) != 0 || len(errs) != 0 {
		t.Errorf("empty input: want empty results, got trips=%v errs=%v", trips, errs)
	}
}

func TestImportOne_AddsThenUpdates(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	v, err := tokenvault.Open(ctx, filepath.Join(dir, "vault.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()

	out, err := importOne(ctx, v, "kiro", triplet{
		email: "alice@example.com", refreshToken: "rt-alice-first", signature: "sig1",
	})
	if err != nil || out != outcomeAdded {
		t.Fatalf("first import: want outcomeAdded nil-err, got %v %v", out, err)
	}

	out, err = importOne(ctx, v, "kiro", triplet{
		email: "alice@example.com", refreshToken: "rt-alice-second", signature: "sig2",
	})
	if err != nil || out != outcomeUpdated {
		t.Fatalf("second import: want outcomeUpdated nil-err, got %v %v", out, err)
	}

	b, err := v.Get(ctx, "kiro", "alice@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if b.RefreshToken != "rt-alice-second" {
		t.Errorf("want refresh_token rotated to second, got %q", b.RefreshToken)
	}
	if b.Generation != 2 {
		t.Errorf("want generation bumped to 2, got %d", b.Generation)
	}
	if !strings.Contains(b.Metadata, "sig2") {
		t.Errorf("metadata not updated to latest sig, got %s", b.Metadata)
	}
}

func TestRunImportAccounts_StdinIntegration(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("KIROXY_DB_PATH", filepath.Join(dir, "tokens.db"))

	input := "x@example.com:rt-x-xxxxxxxxxxx:sig-x\n"
	oldStdin := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r
	_, _ = io.Copy(w, bytes.NewBufferString(input))
	w.Close()
	defer func() { os.Stdin = oldStdin }()

	if err := runImportAccounts(context.Background(), []string{"--stdin"}); err != nil {
		t.Fatalf("runImportAccounts: %v", err)
	}

	v, err := tokenvault.Open(context.Background(), filepath.Join(dir, "tokens.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	b, err := v.Get(context.Background(), "kiro", "x@example.com")
	if err != nil {
		t.Fatalf("account not imported: %v", err)
	}
	if b.RefreshToken != "rt-x-xxxxxxxxxxx" {
		t.Errorf("refresh token round-trip: got %q", b.RefreshToken)
	}
}

func TestRunImportAccounts_MissingSource(t *testing.T) {
	err := runImportAccounts(context.Background(), []string{})
	if err == nil || !strings.Contains(err.Error(), "either --file") {
		t.Fatalf("want clear error when neither flag given, got %v", err)
	}
}
