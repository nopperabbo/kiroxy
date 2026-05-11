package server

import (
	"bytes"
	"context"
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

func postMessages(t *testing.T, baseURL string) *http.Response {
	t.Helper()
	body := `{
		"model":"claude-sonnet-4-5",
		"max_tokens":1024,
		"messages":[{"role":"user","content":"hi"}]
	}`
	req, _ := http.NewRequest("POST", baseURL+"/v1/messages", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Claude-Code-Session-Id", "m5-pool")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	return resp
}

// TestM5_LRURotationAcross3AccountsViaHTTP is the M5 gate test.
func TestM5_LRURotationAcross3AccountsViaHTTP(t *testing.T) {
	ts, stub, _, _ := setupPoolServer(t, "a", "b", "c")

	const N = 30
	for range N {
		resp := postMessages(t, ts.URL)
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
			t.Errorf("LRU never picked token %s", id)
		}
	}
	want := N / 3
	for tok, n := range counts {
		if n < want-1 || n > want+1 {
			t.Errorf("LRU imbalance: %s picked %d times, want ~%d", tok, n, want)
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
	for range 30 {
		resp := postMessages(t, ts.URL)
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
