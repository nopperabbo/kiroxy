package server

import (
	"bytes"
	"context"
	"encoding/json"
	"hash/crc32"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"local/kiroxy/internal/auth"
	"local/kiroxy/internal/kiroclient"
	"local/kiroxy/internal/kiroproto"
)

type stubTokenGetter struct{ creds *auth.Credentials }

func (s *stubTokenGetter) GetToken(_ context.Context) (*auth.Credentials, error) {
	return s.creds, nil
}

type stubKiroClient struct {
	body      []byte
	lastToken string
	lastModel string
}

func (c *stubKiroClient) GenerateAssistantResponse(_ context.Context, token string, payload *kiroproto.Payload, _ string) (*kiroclient.Response, error) {
	c.lastToken = token
	if payload != nil {
		c.lastModel = payload.ConversationState.CurrentMessage.UserInputMessage.ModelID
	}
	return &kiroclient.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(c.body)),
		Header:     http.Header{"Content-Type": []string{"application/vnd.amazon.eventstream"}},
	}, nil
}

// buildSingleShotEventStream assembles one assistantResponseEvent frame with
// the given text, followed by a messageStopEvent frame. Enough to satisfy a
// non-streaming /v1/messages request end-to-end.
func buildSingleShotEventStream(t *testing.T, text string) []byte {
	t.Helper()
	var buf bytes.Buffer
	writeFrame(t, &buf, "assistantResponseEvent", map[string]any{"content": text})
	writeFrame(t, &buf, "messageStopEvent", map[string]any{"stopReason": "end_turn"})
	return buf.Bytes()
}

func writeFrame(t *testing.T, w io.Writer, eventType string, payload map[string]any) {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	// Build headers (name + type byte + length/value) per AWS EventStream.
	headers := appendStringHeader(nil, ":event-type", eventType)
	headers = appendStringHeader(headers, ":content-type", "application/json")
	headers = appendStringHeader(headers, ":message-type", "event")

	preludeLen := 12 // total_len(4) + headers_len(4) + prelude_crc(4)
	msgSuffix := 4   // message_crc
	totalLen := preludeLen + len(headers) + len(body) + msgSuffix

	buf := make([]byte, 0, totalLen)
	buf = appendUint32(buf, uint32(totalLen))
	buf = appendUint32(buf, uint32(len(headers)))
	buf = appendUint32(buf, crc32.ChecksumIEEE(buf[:8])) // prelude crc over total+headers_len
	buf = append(buf, headers...)
	buf = append(buf, body...)
	buf = appendUint32(buf, crc32.ChecksumIEEE(buf)) // whole-message crc excluding trailing slot
	if _, err := w.Write(buf); err != nil {
		t.Fatal(err)
	}
}

func appendUint32(dst []byte, v uint32) []byte {
	return append(dst, byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
}

func appendStringHeader(dst []byte, name, value string) []byte {
	dst = append(dst, byte(len(name)))
	dst = append(dst, []byte(name)...)
	dst = append(dst, 7) // string type
	dst = append(dst, byte(len(value)>>8), byte(len(value)))
	dst = append(dst, []byte(value)...)
	return dst
}

func TestM2_PostMessagesWithStubClient(t *testing.T) {
	stub := &stubKiroClient{body: buildSingleShotEventStream(t, "hello from stub kiro")}
	authStub := &stubTokenGetter{creds: &auth.Credentials{
		AccessToken: "fake-access-token",
		Region:      "us-east-1",
	}}

	srv := New(Options{
		Version:    "test",
		Auth:       authStub,
		KiroClient: stub,
	})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	body := `{
		"model":"claude-sonnet-4-5",
		"max_tokens":1024,
		"messages":[{"role":"user","content":"hi"}]
	}`
	req, _ := http.NewRequest("POST", ts.URL+"/v1/messages", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Claude-Code-Session-Id", "test-session-1")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d; body=%s", resp.StatusCode, respBody)
	}
	if !bytes.Contains(respBody, []byte("hello from stub kiro")) {
		t.Fatalf("response missing stub text; body=%s", respBody)
	}
	if stub.lastToken != "fake-access-token" {
		t.Fatalf("expected stub client to receive the auth token, got %q", stub.lastToken)
	}
}
