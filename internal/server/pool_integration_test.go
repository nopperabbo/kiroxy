package server

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"local/kiroxy/internal/kiroclient"
	"local/kiroxy/internal/kiroproto"
	"local/kiroxy/internal/pool"
	"local/kiroxy/internal/tokenvault"
)

// trackingKiroClient records the access token used for every request, so the
// integration test can verify LRU rotation across the pool.
type trackingKiroClient struct {
	mu         sync.Mutex
	tokensSeen []string
	body       []byte
	fail       func(token string) bool
	failReason string
	failCount  atomic.Int32
}

func (c *trackingKiroClient) GenerateAssistantResponse(ctx context.Context, token string, _ *kiroproto.Payload, _ string) (*kiroclient.Response, error) {
	c.mu.Lock()
	c.tokensSeen = append(c.tokensSeen, token)
	c.mu.Unlock()
	if c.fail != nil && c.fail(token) {
		c.failCount.Add(1)
		return &kiroclient.Response{
			StatusCode: http.StatusTooManyRequests,
			Body:       io.NopCloser(bytes.NewReader([]byte(c.failReason))),
			Header:     http.Header{},
		}, nil
	}
	return &kiroclient.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(c.body)),
		Header:     http.Header{"Content-Type": []string{"application/vnd.amazon.eventstream"}},
	}, nil
}

func (c *trackingKiroClient) seenTokens() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, len(c.tokensSeen))
	copy(out, c.tokensSeen)
	return out
}

func setupPoolServer(t *testing.T, ids ...string) (*httptest.Server, *trackingKiroClient, *pool.Pool, *tokenvault.Vault) {
	t.Helper()
	ctx := context.Background()

	dir := t.TempDir()
	v, err := tokenvault.Open(ctx, filepath.Join(dir, "vault.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = v.Close() })

	p := pool.New(pool.Policy{
		ConsecutiveErrorThreshold: 3,
		ShortCooldown:             50 * time.Millisecond,
		QuotaCooldown:             500 * time.Millisecond,
		MaxCooldown:               2 * time.Second,
	})
	for _, id := range ids {
		if _, err := v.Save(ctx, "kiro", id, tokenvault.Tokens{
			AccessToken:  "at-" + id,
			RefreshToken: "rt-" + id,
		}); err != nil {
			t.Fatal(err)
		}
		p.Add(pool.Account{
			ID: id, Label: id, Provider: "kiro", Region: "us-east-1", Enabled: true,
		})
	}

	stub := &trackingKiroClient{body: buildSingleShotEventStream(t, "hi from "+ids[0])}
	tg := &pool.TokenGetter{Pool: p, Vault: v}
	srv := New(Options{Auth: tg, KiroClient: stub})
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts, stub, p, v
}

// postMessages exercises the /v1/messages endpoint. The sessionID arg
// lets callers vary the X-Claude-Code-Session-Id header so session
// stickiness doesn't pin every request in the test to one account.
func postMessages(t *testing.T, baseURL, sessionID string) *http.Response {
	t.Helper()
	body := `{
		"model":"claude-sonnet-4-5",
		"max_tokens":1024,
		"messages":[{"role":"user","content":"hi"}]
	}`
	req, _ := http.NewRequest("POST", baseURL+"/v1/messages", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Claude-Code-Session-Id", sessionID)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	return resp
}

// TestM5_WeightedPickDistributesAcross3AccountsViaHTTP is the M5 gate
// test. Post-v1.1 pool uses weighted random selection (equal weights
// for fresh accounts), so distribution is statistical rather than
// strict LRU. Every account must be picked; the spread must stay
// within ±50% of the uniform expectation across N=60 calls.
func TestM5_WeightedPickDistributesAcross3AccountsViaHTTP(t *testing.T) {
	ts, stub, _, _ := setupPoolServer(t, "a", "b", "c")

	const N = 60
	for i := 0; i < N; i++ {
		// Distinct session IDs so stickiness doesn't pin every call to
		// the same account. With 60 unique IDs each writes its own pin
		// at the weighted-selector's fallback account.
		resp := postMessages(t, ts.URL, fmt.Sprintf("sess-%d", i))
		if resp.StatusCode != 200 {
			t.Fatalf("status=%d", resp.StatusCode)
		}
		_ = resp.Body.Close()
	}

	seen := stub.seenTokens()
	if len(seen) != N {
		t.Fatalf("want %d calls, got %d", N, len(seen))
	}

	counts := map[string]int{}
	for _, tok := range seen {
		counts[tok]++
	}
	for _, id := range []string{"at-a", "at-b", "at-c"} {
		if counts[id] == 0 {
			t.Errorf("weighted pick never chose token %s; counts=%v", id, counts)
		}
	}
	// Uniform expectation = 20 per account; ±50% = [10, 30].
	for tok, n := range counts {
		if n < 10 || n > 30 {
			t.Errorf("weighted distribution imbalance: %s picked %d times, want 10-30", tok, n)
		}
	}
}

// TestM5_FailedAccountSkippedAfter3Errors exercises the cooldown path via HTTP.
// Account "b" is marked as failing via the stub; after recording 3 quota
// errors through the pool, subsequent requests must skip "b".
func TestM5_FailedAccountSkippedAfter3Errors(t *testing.T) {
	ts, stub, p, _ := setupPoolServer(t, "a", "b", "c")

	stub.fail = func(token string) bool { return token == "at-b" }
	stub.failReason = "429 quota"

	p.RecordFailure("b", pool.FailureQuota, "429 quota")

	counts := map[string]int{}
	for i := 0; i < 30; i++ {
		resp := postMessages(t, ts.URL, fmt.Sprintf("sess-%d", i))
		_ = resp.Body.Close()
	}
	for _, tok := range stub.seenTokens() {
		counts[tok]++
	}
	if counts["at-b"] != 0 {
		t.Errorf("account b should be on quota cooldown and skipped; seen %d times", counts["at-b"])
	}
	if counts["at-a"] == 0 || counts["at-c"] == 0 {
		t.Errorf("other accounts should rotate; counts=%+v", counts)
	}
}
