package server

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nopperabbo/kiroxy/internal/auth"
	"github.com/nopperabbo/kiroxy/internal/kiroclient"
	"github.com/nopperabbo/kiroxy/internal/kiroproto"
)

// slowStreamClient emits a multi-frame AWS EventStream body with configurable
// per-frame delays, so the proxy must flush chunks incrementally.
type slowStreamClient struct {
	chunks   [][]byte
	delay    time.Duration
	ctxSeen  chan struct{}
	canceled atomic.Bool
	written  atomic.Int64
}

func (c *slowStreamClient) GenerateAssistantResponse(ctx context.Context, _ string, _ *kiroproto.Payload, _ string, _ string) (*kiroclient.Response, error) {
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		for _, chunk := range c.chunks {
			select {
			case <-ctx.Done():
				c.canceled.Store(true)
				_ = pw.CloseWithError(ctx.Err())
				return
			case <-time.After(c.delay):
			}
			n, err := pw.Write(chunk)
			if err != nil {
				c.canceled.Store(true)
				return
			}
			c.written.Add(int64(n))
		}
	}()
	if c.ctxSeen != nil {
		close(c.ctxSeen)
	}
	return &kiroclient.Response{
		StatusCode: http.StatusOK,
		Body:       pr,
		Header:     http.Header{"Content-Type": []string{"application/vnd.amazon.eventstream"}},
	}, nil
}

func buildStreamChunks(t *testing.T, parts []string) [][]byte {
	t.Helper()
	out := make([][]byte, 0, len(parts)+1)
	for _, p := range parts {
		var buf bytes.Buffer
		writeFrame(t, &buf, "assistantResponseEvent", map[string]any{"content": p})
		out = append(out, buf.Bytes())
	}
	var tail bytes.Buffer
	writeFrame(t, &tail, "messageStopEvent", map[string]any{"stopReason": "end_turn"})
	out = append(out, tail.Bytes())
	return out
}

// TestM3_StreamIncrementalDelivery verifies that the proxy does not buffer
// the entire upstream response before emitting SSE. We deliberately delay
// between upstream frames; if buffering occurred we'd see all chunks at once.
func TestM3_StreamIncrementalDelivery(t *testing.T) {
	parts := []string{"Hello ", "world ", "from ", "kiroxy"}
	stub := &slowStreamClient{
		chunks: buildStreamChunks(t, parts),
		delay:  80 * time.Millisecond,
	}
	authStub := &stubTokenGetter{creds: &auth.Credentials{AccessToken: "t", Region: "us-east-1"}}
	srv := New(Options{Auth: authStub, KiroClient: stub})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	body := `{
		"model":"claude-sonnet-4-5",
		"max_tokens":1024,
		"stream":true,
		"messages":[{"role":"user","content":"hi"}]
	}`
	req, _ := http.NewRequest("POST", ts.URL+"/v1/messages", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Claude-Code-Session-Id", "m3-stream")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		rb, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d body=%s", resp.StatusCode, rb)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("want text/event-stream, got %q", ct)
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var (
		events        []string
		firstDeltaAt  time.Time
		lastDeltaAt   time.Time
		sawMsgStart   bool
		sawMsgStop    bool
		start         = time.Now()
		textCollector strings.Builder
	)
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "event: message_start"):
			sawMsgStart = true
		case strings.HasPrefix(line, "event: message_stop"):
			sawMsgStop = true
		case strings.HasPrefix(line, "event: content_block_delta"):
			if firstDeltaAt.IsZero() {
				firstDeltaAt = time.Now()
			}
			lastDeltaAt = time.Now()
		case strings.HasPrefix(line, "data:") && strings.Contains(line, "text_delta"):
			events = append(events, line)
			if i := strings.Index(line, `"text":"`); i >= 0 {
				rest := line[i+len(`"text":"`):]
				if j := strings.Index(rest, `"`); j >= 0 {
					textCollector.WriteString(rest[:j])
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan: %v", err)
	}

	if !sawMsgStart {
		t.Error("missing event: message_start")
	}
	if !sawMsgStop {
		t.Error("missing event: message_stop")
	}
	if len(events) == 0 {
		t.Error("no content_block_delta text_delta events")
	}

	spread := lastDeltaAt.Sub(firstDeltaAt)
	if spread < 50*time.Millisecond {
		t.Errorf("deltas arrived within %v \u2014 proxy may be buffering (expected spread >= 50ms given 80ms/frame upstream cadence)", spread)
	}

	elapsed := time.Since(start)
	if elapsed < time.Duration(len(parts))*stub.delay/2 {
		t.Errorf("total elapsed %v is suspiciously short; stream may have been eagerly drained", elapsed)
	}

	got := textCollector.String()
	for _, part := range parts {
		if !strings.Contains(got, strings.TrimSpace(part)) {
			t.Errorf("streamed text missing %q; got %q", part, got)
		}
	}
}

// TestM3_ClientDisconnectCancelsUpstream verifies that closing the HTTP client
// connection mid-stream cancels the upstream request context.
func TestM3_ClientDisconnectCancelsUpstream(t *testing.T) {
	parts := []string{"one", "two", "three", "four", "five", "six", "seven", "eight"}
	stub := &slowStreamClient{
		chunks:  buildStreamChunks(t, parts),
		delay:   200 * time.Millisecond,
		ctxSeen: make(chan struct{}),
	}
	authStub := &stubTokenGetter{creds: &auth.Credentials{AccessToken: "t", Region: "us-east-1"}}
	srv := New(Options{Auth: authStub, KiroClient: stub})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	body := `{
		"model":"claude-sonnet-4-5",
		"max_tokens":1024,
		"stream":true,
		"messages":[{"role":"user","content":"hi"}]
	}`
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "POST", ts.URL+"/v1/messages", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Claude-Code-Session-Id", "m3-disconnect")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("initial request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status=%d", resp.StatusCode)
	}

	select {
	case <-stub.ctxSeen:
	case <-time.After(2 * time.Second):
		t.Fatal("stub client never observed the request context")
	}

	buf := make([]byte, 256)
	if _, err := resp.Body.Read(buf); err != nil {
		t.Fatalf("initial read: %v", err)
	}

	cancel()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if stub.canceled.Load() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("upstream context was not cancelled within 2s of client disconnect (written=%d bytes)",
		stub.written.Load())
}
