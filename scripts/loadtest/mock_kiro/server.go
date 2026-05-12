// Package main implements a mock Kiro CodeWhisperer upstream for load-testing
// kiroxy without consuming real AWS quota or requiring valid credentials.
//
// The mock emits AWS EventStream binary frames (CRC-32 IEEE, big-endian) that
// match the shape kiroxy's internal/kiroproto parser consumes. Frame layout:
//
//	+------------------+------------------+----+------+-------+----+
//	| totalLen (u32 BE)| hdrLen   (u32 BE)|crc | hdrs | payld | crc|
//	+------------------+------------------+----+------+-------+----+
//	        4 bytes            4 bytes     4       N      M      4
//
// preludeCRC covers bytes 0..7. messageCRC covers bytes 0..(totalLen-4).
// Both use crc32.IEEETable.
//
// Header format (one per header): nameLen(u8) | name | valueType(u8) | value.
// For string values (valueType=7) the value is length-prefixed u16 BE.
//
// The mock supports:
//
//	--addr          (default :9789)       listen address
//	--latency-ms    (default 0)           fixed latency added to each response
//	--error-rate    (default 0.0)         0.0..1.0 probability of 5xx
//	--stream-events (default 8)           number of assistantResponseEvent frames
//	--chunk-delay-ms (default 0)          delay between eventstream frames
//
// It is deliberately single-binary, stdlib-only, and self-contained. Point
// kiroxy's kiroclient at this server via WithBaseURL (requires a test harness
// that wires the option — kiroxy core does not currently expose a
// KIROXY_UPSTREAM_URL env var; see README.md for the documented gap).
package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"hash/crc32"
	"log"
	"math/rand/v2"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

var crc32Tab = crc32.IEEETable

// frameHeader encodes a single EventStream header. AWS defines ~10 value
// types; we only emit strings (7), which is sufficient for :message-type,
// :event-type, and :content-type.
type frameHeader struct {
	Name  string
	Value string // string-typed header value
}

// encodeFrame writes a single AWS EventStream frame to buf. It matches the
// byte layout consumed by internal/kiroproto/frame.go in kiroxy.
func encodeFrame(buf *bytes.Buffer, headers []frameHeader, payload []byte) {
	// Serialise headers block.
	var hb bytes.Buffer
	for _, h := range headers {
		if len(h.Name) > 255 {
			panic(fmt.Sprintf("header name too long: %q", h.Name))
		}
		hb.WriteByte(byte(len(h.Name)))
		hb.WriteString(h.Name)
		hb.WriteByte(7) // string value type
		_ = binary.Write(&hb, binary.BigEndian, uint16(len(h.Value)))
		hb.WriteString(h.Value)
	}

	hdrs := hb.Bytes()
	totalLen := uint32(12 + len(hdrs) + len(payload) + 4) // prelude + hdrs + payload + msgCRC
	hdrsLen := uint32(len(hdrs))

	var prelude [12]byte
	binary.BigEndian.PutUint32(prelude[0:4], totalLen)
	binary.BigEndian.PutUint32(prelude[4:8], hdrsLen)
	preludeCRC := crc32.Checksum(prelude[0:8], crc32Tab)
	binary.BigEndian.PutUint32(prelude[8:12], preludeCRC)

	// Message CRC covers the whole frame except the final 4 bytes.
	h := crc32.New(crc32Tab)
	h.Write(prelude[:])
	h.Write(hdrs)
	h.Write(payload)
	msgCRC := h.Sum32()

	buf.Write(prelude[:])
	buf.Write(hdrs)
	buf.Write(payload)
	_ = binary.Write(buf, binary.BigEndian, msgCRC)
}

// eventFrame builds a kiro-shaped assistant response event.
func eventFrame(eventType string, payload any) (headers []frameHeader, body []byte, err error) {
	body, err = json.Marshal(payload)
	if err != nil {
		return nil, nil, err
	}
	headers = []frameHeader{
		{":message-type", "event"},
		{":event-type", eventType},
		{":content-type", "application/json"},
	}
	return headers, body, nil
}

// config holds the parsed CLI flags.
type config struct {
	addr         string
	latencyMs    int
	errorRate    float64
	streamEvents int
	chunkDelayMs int
	logRequests  bool
	tokensIn     int
	tokensOut    int
	responseText string
	failAfter    int // if >0, return 5xx after this many requests and never recover (for breaker tests)
}

// counters are process-global. atomic-incremented under load.
var (
	reqCount atomic.Int64
	errCount atomic.Int64
	inflight atomic.Int64
)

func parseFlags() config {
	c := config{}
	flag.StringVar(&c.addr, "addr", envOr("MOCK_KIRO_ADDR", ":9789"), "listen address")
	flag.IntVar(&c.latencyMs, "latency-ms", envOrInt("MOCK_KIRO_LATENCY_MS", 0), "fixed latency before first byte (ms)")
	flag.Float64Var(&c.errorRate, "error-rate", envOrFloat("MOCK_KIRO_ERROR_RATE", 0), "0.0..1.0 probability of 5xx response")
	flag.IntVar(&c.streamEvents, "stream-events", envOrInt("MOCK_KIRO_STREAM_EVENTS", 8), "assistantResponseEvent frames per streamed response")
	flag.IntVar(&c.chunkDelayMs, "chunk-delay-ms", envOrInt("MOCK_KIRO_CHUNK_DELAY_MS", 0), "per-frame delay during streaming (ms)")
	flag.BoolVar(&c.logRequests, "log", false, "log each request line")
	flag.IntVar(&c.tokensIn, "tokens-in", 42, "reported input tokens in metadataEvent")
	flag.IntVar(&c.tokensOut, "tokens-out", 64, "reported output tokens in metadataEvent")
	flag.StringVar(&c.responseText, "text", "mock kiroxy response", "canned response text (spread across stream events)")
	flag.IntVar(&c.failAfter, "fail-after", 0, "return 5xx after N successful requests (0=disabled)")
	flag.Parse()
	if c.errorRate < 0 || c.errorRate > 1 {
		fmt.Fprintln(os.Stderr, "error-rate must be in [0,1]")
		os.Exit(2)
	}
	return c
}

func envOr(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}

func envOrInt(k string, fallback int) int {
	v := os.Getenv(k)
	if v == "" {
		return fallback
	}
	var n int
	_, err := fmt.Sscanf(v, "%d", &n)
	if err != nil {
		return fallback
	}
	return n
}

func envOrFloat(k string, fallback float64) float64 {
	v := os.Getenv(k)
	if v == "" {
		return fallback
	}
	var n float64
	_, err := fmt.Sscanf(v, "%f", &n)
	if err != nil {
		return fallback
	}
	return n
}

type server struct {
	cfg config
}

// splitText splits s into n roughly-equal chunks. Used to emit multiple
// assistantResponseEvent frames, mirroring how a real Kiro response streams.
func splitText(s string, n int) []string {
	if n <= 1 || len(s) == 0 {
		return []string{s}
	}
	out := make([]string, 0, n)
	size := (len(s) + n - 1) / n
	for i := 0; i < len(s); i += size {
		end := i + size
		if end > len(s) {
			end = len(s)
		}
		out = append(out, s[i:end])
	}
	return out
}

// handleGenerate emits a canned stream of kiro events to the response.
func (s *server) handleGenerate(w http.ResponseWriter, r *http.Request) {
	inflight.Add(1)
	defer inflight.Add(-1)
	n := reqCount.Add(1)

	if s.cfg.logRequests {
		log.Printf("mock: %s %s target=%s auth=%s",
			r.Method, r.URL.Path, r.Header.Get("X-Amz-Target"),
			truncate(r.Header.Get("Authorization"), 20))
	}

	// Canned failure modes.
	if s.cfg.failAfter > 0 && int(n) > s.cfg.failAfter {
		errCount.Add(1)
		http.Error(w, `{"__type":"InternalServerException","message":"mock fail-after tripped"}`, http.StatusInternalServerError)
		return
	}
	if s.cfg.errorRate > 0 && rand.Float64() < s.cfg.errorRate {
		errCount.Add(1)
		http.Error(w, `{"__type":"ThrottlingException","message":"mock injected throttle"}`, http.StatusTooManyRequests)
		return
	}
	if s.cfg.latencyMs > 0 {
		time.Sleep(time.Duration(s.cfg.latencyMs) * time.Millisecond)
	}

	// Validate the AWS RPC envelope minimally; we accept anything that looks
	// like a kiro request but log mismatches so the harness can spot them.
	if ct := r.Header.Get("Content-Type"); !strings.Contains(ct, "amz-json") && ct != "application/json" {
		log.Printf("mock: unexpected content-type %q", ct)
	}
	if target := r.Header.Get("X-Amz-Target"); target != "" &&
		!strings.Contains(target, "GenerateAssistantResponse") {
		log.Printf("mock: unexpected X-Amz-Target %q", target)
	}

	// Stream eventstream frames.
	w.Header().Set("Content-Type", "application/vnd.amazon.eventstream")
	w.Header().Set("X-Amzn-Requestid", fmt.Sprintf("mock-%d", n))
	w.WriteHeader(http.StatusOK)

	flusher, _ := w.(http.Flusher)

	chunks := splitText(s.cfg.responseText, s.cfg.streamEvents)
	var buf bytes.Buffer

	// 1. initial-response.
	hdrs, body, _ := eventFrame("initial-response", map[string]any{
		"conversationId": fmt.Sprintf("mock-conv-%d", n),
	})
	encodeFrame(&buf, hdrs, body)
	flushBuf(w, &buf, flusher)

	// 2. assistantResponseEvent frames.
	for i, chunk := range chunks {
		hdrs, body, _ := eventFrame("assistantResponseEvent", map[string]any{
			"content": chunk,
			"modelId": "mock-sonnet",
		})
		encodeFrame(&buf, hdrs, body)
		flushBuf(w, &buf, flusher)

		if s.cfg.chunkDelayMs > 0 && i < len(chunks)-1 {
			time.Sleep(time.Duration(s.cfg.chunkDelayMs) * time.Millisecond)
		}
	}

	// 3. metadataEvent (token usage).
	hdrs, body, _ = eventFrame("metadataEvent", map[string]any{
		"tokenUsage": map[string]any{
			"uncachedInputTokens":   s.cfg.tokensIn,
			"outputTokens":          s.cfg.tokensOut,
			"totalTokens":           s.cfg.tokensIn + s.cfg.tokensOut,
			"cacheReadInputTokens":  0,
			"cacheWriteInputTokens": 0,
		},
	})
	encodeFrame(&buf, hdrs, body)
	flushBuf(w, &buf, flusher)

	// 4. messageMetadataEvent (stop-signal).
	hdrs, body, _ = eventFrame("messageMetadataEvent", map[string]any{
		"conversationId": fmt.Sprintf("mock-conv-%d", n),
		"utteranceId":    fmt.Sprintf("mock-utt-%d", n),
	})
	encodeFrame(&buf, hdrs, body)
	flushBuf(w, &buf, flusher)
}

func flushBuf(w http.ResponseWriter, buf *bytes.Buffer, f http.Flusher) {
	if buf.Len() == 0 {
		return
	}
	_, _ = w.Write(buf.Bytes())
	buf.Reset()
	if f != nil {
		f.Flush()
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// statsHandler returns process counters for the harness to sanity-check
// against its own client-side counts.
func statsHandler(w http.ResponseWriter, r *http.Request) {
	_ = r
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]int64{
		"requests": reqCount.Load(),
		"errors":   errCount.Load(),
		"inflight": inflight.Load(),
	})
}

// healthHandler lets orchestration scripts wait for the mock to be ready.
func healthHandler(w http.ResponseWriter, r *http.Request) {
	_ = r
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func main() {
	cfg := parseFlags()
	s := &server{cfg: cfg}

	mux := http.NewServeMux()
	// Real Kiro exposes a single POST / endpoint with X-Amz-Target routing.
	// We accept that and also POST /generate for direct testing.
	mux.HandleFunc("POST /", s.handleGenerate)
	mux.HandleFunc("POST /generate", s.handleGenerate)
	mux.HandleFunc("GET /healthz", healthHandler)
	mux.HandleFunc("GET /stats", statsHandler)

	srv := &http.Server{
		Addr:              cfg.addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      5 * time.Minute, // long enough for slow streams
	}

	log.Printf("mock_kiro: listening on %s (latency=%dms error_rate=%.2f stream_events=%d)",
		cfg.addr, cfg.latencyMs, cfg.errorRate, cfg.streamEvents)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("mock_kiro: %v", err)
	}
}
