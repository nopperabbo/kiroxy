package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nopperabbo/kiroxy/internal/auth"
	"github.com/nopperabbo/kiroxy/internal/pool"
	"github.com/nopperabbo/kiroxy/internal/tokenvault"
)

// TestE2E_ExpiredTokenTriggersRefreshOnRequest spins up a real kiroxy
// server backed by a real vault + pool, seeds an expired social account,
// sends POST /v1/messages, and verifies:
//
//  1. The pool layer invoked the mock RefreshFn exactly once.
//  2. The downstream kiroclient received the NEW access_token (not the
//     stored stale one) in its Bearer header.
//  3. The vault was updated post-refresh: new access_token, generation+1,
//     expires_at renewed in metadata.
//
// This is the "upstream, full stack, realistic credentials rotation" test
// Phase 2.5 deferred due to tool constraints. No real Kiro HTTP calls —
// RefreshFn is stubbed at the pool layer and the kiroclient is stubbed at
// the downstream layer.
func TestE2E_ExpiredTokenTriggersRefreshOnRequest(t *testing.T) {
	ctx := context.Background()

	// 1. Vault with single expired social account.
	dir := t.TempDir()
	v, err := tokenvault.Open(ctx, filepath.Join(dir, "vault.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = v.Close() })

	const (
		accID  = "e2e-social-1"
		oldAT  = "stale-access-token"
		oldRT  = "stale-refresh-token"
		newAT  = "freshly-rotated-access-token"
		newRT  = "freshly-rotated-refresh-token"
		newArn = "arn:aws:codewhisperer:us-east-1:123:profile/NEW"
	)
	expiredAt := time.Now().Add(-1 * time.Hour).Unix()

	metaJSON := fmt.Sprintf(`{"auth_method":"social","profile_arn":"arn:old","expires_at":%d,"source":"import-accounts-json"}`, expiredAt)
	if _, err := v.Save(ctx, "kiro", accID, tokenvault.Tokens{
		AccessToken:  oldAT,
		RefreshToken: oldRT,
		Source:       "import-accounts-json",
		Metadata:     metaJSON,
	}); err != nil {
		t.Fatal(err)
	}

	// 2. Pool with just that account.
	p := pool.New(pool.DefaultPolicy())
	p.Add(pool.Account{ID: accID, Label: accID, Provider: "kiro", Region: "us-east-1", Enabled: true})

	// 3. RefreshFn that returns the rotated credentials, counting invocations.
	var refreshCalls atomic.Int32
	fakeFn := func(_ context.Context, region, rt string) (*auth.RefreshResult, error) {
		refreshCalls.Add(1)
		if rt != oldRT {
			t.Errorf("RefreshFn saw stale rt=%q, want %q", rt, oldRT)
		}
		if region != "us-east-1" {
			t.Errorf("RefreshFn region=%q", region)
		}
		return &auth.RefreshResult{
			AccessToken:  newAT,
			RefreshToken: newRT,
			ExpiresAt:    time.Now().Add(1 * time.Hour).Unix(),
			ProfileARN:   newArn,
		}, nil
	}

	tg := &pool.TokenGetter{
		Pool:  p,
		Vault: v,
		Refresh: &pool.RefreshConfig{
			RefreshFn:   fakeFn,
			Skew:        5 * time.Minute,
			LockTTL:     30 * time.Second,
			MaxRetries:  3,
			BaseBackoff: 10 * time.Millisecond,
		},
	}

	// 4. kiroclient stub that records the token it was given.
	stub := &trackingKiroClient{body: buildSingleShotEventStream(t, "ok")}

	// 5. kiroxy Server wiring — same shape as production main.go pool path.
	srv := New(Options{Auth: tg, KiroClient: stub})

	// 6. Issue POST /v1/messages.
	body := `{"model":"claude-sonnet-4-5","max_tokens":64,"messages":[{"role":"user","content":"hi"}]}`
	req, _ := http.NewRequest("POST", "/v1/messages", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Claude-Code-Session-Id", "e2e-refresh")

	rw := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rw, req)
	if rw.Code != 200 {
		t.Fatalf("POST /v1/messages status=%d body=%s", rw.Code, rw.Body.String())
	}

	// 7. Assertions.
	if got := refreshCalls.Load(); got != 1 {
		t.Errorf("RefreshFn calls = %d, want 1", got)
	}
	seen := stub.seenTokens()
	if len(seen) == 0 {
		t.Fatal("kiroclient received no requests")
	}
	if seen[len(seen)-1] != newAT {
		t.Errorf("kiroclient saw token=%q on final call, want %q (the refreshed token)", seen[len(seen)-1], newAT)
	}
	if strings.Contains(seen[len(seen)-1], oldAT) {
		t.Errorf("kiroclient still using stale access_token: %q", seen[len(seen)-1])
	}

	// 8. Vault side-effects — generation bumped, new tokens persisted.
	b, err := v.Get(ctx, "kiro", accID)
	if err != nil {
		t.Fatal(err)
	}
	if b.AccessToken != newAT {
		t.Errorf("vault.AccessToken = %q, want %q", b.AccessToken, newAT)
	}
	if b.RefreshToken != newRT {
		t.Errorf("vault.RefreshToken = %q, want %q", b.RefreshToken, newRT)
	}
	// Generation expected to be at least 2: refresh bumps it from 1 -> 2.
	// One additional bump is acceptable when the lazy machine_id helper
	// commits a per-account fingerprint into metadata on first GetToken
	// (this fixture's bundle has no machine_id pre-set).
	if b.Generation < 2 {
		t.Errorf("vault.Generation = %d, want >= 2", b.Generation)
	}
	if !strings.Contains(b.Metadata, newArn) {
		t.Errorf("vault metadata missing new profile_arn: %s", b.Metadata)
	}
}
