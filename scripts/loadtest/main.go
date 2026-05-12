// Package main implements a load-test harness for kiroxy.
//
// The harness issues concurrent requests against one of two targets:
//
//	--mode anthropic  POST /v1/messages on a kiroxy instance
//	--mode kiro       POST / on the mock_kiro server (eventstream directly)
//
// It records per-request outcomes (status, error, latency, first-byte time,
// bytes received, stream event count) and emits two artifacts:
//
//	<out>/requests.jsonl   one JSON object per request
//	<out>/summary.json     aggregate metrics (p50/p95/p99 ms, RPS, error rate)
//
// Flags:
//
//	--url          target base URL (default http://127.0.0.1:8787)
//	--mode         anthropic|kiro (default anthropic)
//	--concurrency  parallel workers (default 5)
//	--total        total requests to issue (default 100)
//	--duration     instead of total, run for N seconds (overrides --total)
//	--stream       set "stream":true in body (and parse SSE; anthropic mode only)
//	--api-key      send as X-Api-Key (anthropic mode)
//	--timeout      per-request timeout (default 120s)
//	--out          output directory (default ./results)
//	--scenario     name stamped into summary for easier labelling
//	--warmup       issue N warmup requests before timing starts (default 3)
//
// All output is stdlib-only. No YAML parser, no metrics library.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

type config struct {
	url         string
	mode        string
	concurrency int
	total       int
	duration    time.Duration
	stream      bool
	apiKey      string
	timeout     time.Duration
	out         string
	scenario    string
	warmup      int
	tokens      int // max_tokens in request body
}

func parseFlags() config {
	c := config{}
	flag.StringVar(&c.url, "url", "http://127.0.0.1:8787", "target base URL")
	flag.StringVar(&c.mode, "mode", "anthropic", "target mode: anthropic|kiro")
	flag.IntVar(&c.concurrency, "concurrency", 5, "parallel workers")
	flag.IntVar(&c.total, "total", 100, "total requests (ignored if --duration > 0)")
	flag.DurationVar(&c.duration, "duration", 0, "run for this long instead of --total")
	flag.BoolVar(&c.stream, "stream", false, "request SSE streaming (anthropic mode only)")
	flag.StringVar(&c.apiKey, "api-key", os.Getenv("KIROXY_API_KEY"), "X-Api-Key header (defaults to $KIROXY_API_KEY)")
	flag.DurationVar(&c.timeout, "timeout", 120*time.Second, "per-request timeout")
	flag.StringVar(&c.out, "out", "./results", "output directory")
	flag.StringVar(&c.scenario, "scenario", "adhoc", "scenario label for summary.json")
	flag.IntVar(&c.warmup, "warmup", 3, "warmup requests issued before timing")
	flag.IntVar(&c.tokens, "tokens", 256, "max_tokens in anthropic request")
	flag.Parse()

	if c.mode != "anthropic" && c.mode != "kiro" {
		fmt.Fprintf(os.Stderr, "unknown mode %q\n", c.mode)
		os.Exit(2)
	}
	if c.concurrency < 1 {
		c.concurrency = 1
	}
	return c
}

// result is one record in requests.jsonl.
type result struct {
	Index       int     `json:"index"`
	StartUnixMs int64   `json:"start_unix_ms"`
	Status      int     `json:"status"`
	Error       string  `json:"error,omitempty"`
	LatencyMs   float64 `json:"latency_ms"`
	TTFBMs      float64 `json:"ttfb_ms,omitempty"`
	Bytes       int64   `json:"bytes"`
	Events      int     `json:"events,omitempty"`
	Worker      int     `json:"worker"`
}

// summary aggregates results.
type summary struct {
	Scenario        string    `json:"scenario"`
	Mode            string    `json:"mode"`
	URL             string    `json:"url"`
	Stream          bool      `json:"stream"`
	Concurrency     int       `json:"concurrency"`
	StartedAt       time.Time `json:"started_at"`
	EndedAt         time.Time `json:"ended_at"`
	TotalRequests   int       `json:"total_requests"`
	Successes       int       `json:"successes"`
	Errors          int       `json:"errors"`
	ErrorRate       float64   `json:"error_rate"`
	TotalDurationS  float64   `json:"total_duration_s"`
	RPS             float64   `json:"rps"`
	LatencyP50Ms    float64   `json:"latency_p50_ms"`
	LatencyP95Ms    float64   `json:"latency_p95_ms"`
	LatencyP99Ms    float64   `json:"latency_p99_ms"`
	LatencyMaxMs    float64   `json:"latency_max_ms"`
	LatencyMeanMs   float64   `json:"latency_mean_ms"`
	TTFBP50Ms       float64   `json:"ttfb_p50_ms,omitempty"`
	TTFBP95Ms       float64   `json:"ttfb_p95_ms,omitempty"`
	BytesTotal      int64     `json:"bytes_total"`
	EventsTotal     int       `json:"events_total,omitempty"`
	SystemGoVersion string    `json:"sys_go_version"`
	SystemOS        string    `json:"sys_os"`
	SystemArch      string    `json:"sys_arch"`
	SystemCPU       int       `json:"sys_cpu"`
}

// buildAnthropicBody returns the JSON body for a kiroxy /v1/messages call.
func buildAnthropicBody(stream bool, tokens int) []byte {
	body := map[string]any{
		"model":      "claude-sonnet-4-5",
		"max_tokens": tokens,
		"messages": []map[string]any{
			{"role": "user", "content": "Return a single word."},
		},
	}
	if stream {
		body["stream"] = true
	}
	b, _ := json.Marshal(body)
	return b
}

// buildKiroBody returns a minimal payload accepted by the mock; shape is not
// validated by the mock itself, but we match the real Kiro request envelope.
func buildKiroBody() []byte {
	return []byte(`{"conversationState":{"chatTriggerType":"MANUAL","agentTaskType":"vibe","currentMessage":{"userInputMessage":{"content":"hello","modelId":"mock-sonnet","origin":"KIRO_CLI"}}}}`)
}

// countAnthropicSSEEvents reads a kiroxy SSE body and returns the number of
// `event:` lines consumed.
func countAnthropicSSEEvents(r io.Reader) (int, int64, error) {
	br := bufio.NewReader(r)
	n := 0
	var total int64
	for {
		line, err := br.ReadSlice('\n')
		total += int64(len(line))
		if bytes.HasPrefix(line, []byte("event:")) {
			n++
		}
		if err == io.EOF {
			return n, total, nil
		}
		if err == bufio.ErrBufferFull {
			// Skip rest of the line.
			for {
				more, e := br.ReadSlice('\n')
				total += int64(len(more))
				if e == io.EOF {
					return n, total, nil
				}
				if e != nil {
					return n, total, e
				}
				if len(more) > 0 && more[len(more)-1] == '\n' {
					break
				}
			}
			continue
		}
		if err != nil {
			return n, total, err
		}
	}
}

// countKiroEventstreamFrames reads AWS EventStream frames from r and returns
// the count. It does NOT validate CRCs; kiroxy's kiroproto does that. For the
// harness we only need to drain the body and count logical events.
func countKiroEventstreamFrames(r io.Reader) (int, int64, error) {
	br := bufio.NewReader(r)
	n := 0
	var total int64
	var prelude [12]byte
	for {
		_, err := io.ReadFull(br, prelude[:])
		if err == io.EOF {
			return n, total, nil
		}
		if err != nil {
			return n, total, err
		}
		total += 12
		totalLen := uint32(prelude[0])<<24 | uint32(prelude[1])<<16 | uint32(prelude[2])<<8 | uint32(prelude[3])
		if totalLen < 16 || totalLen > 8*1024*1024 {
			return n, total, fmt.Errorf("bad frame length %d", totalLen)
		}
		rest := make([]byte, totalLen-12)
		m, err := io.ReadFull(br, rest)
		total += int64(m)
		if err != nil {
			return n, total, err
		}
		n++
	}
}

// issueAnthropic sends a kiroxy /v1/messages request.
func issueAnthropic(ctx context.Context, client *http.Client, cfg config) (int, int64, int, time.Duration, error) {
	body := buildAnthropicBody(cfg.stream, cfg.tokens)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.url+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return 0, 0, 0, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")
	if cfg.apiKey != "" {
		req.Header.Set("X-Api-Key", cfg.apiKey)
	}
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	defer resp.Body.Close()
	ttfb := time.Since(start)

	if !cfg.stream {
		b, err := io.ReadAll(resp.Body)
		return resp.StatusCode, int64(len(b)), 0, ttfb, err
	}
	events, bytesRead, err := countAnthropicSSEEvents(resp.Body)
	return resp.StatusCode, bytesRead, events, ttfb, err
}

// issueKiro sends a mock_kiro request.
func issueKiro(ctx context.Context, client *http.Client, cfg config) (int, int64, int, time.Duration, error) {
	body := buildKiroBody()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.url+"/", bytes.NewReader(body))
	if err != nil {
		return 0, 0, 0, 0, err
	}
	req.Header.Set("Content-Type", "application/x-amz-json-1.0")
	req.Header.Set("X-Amz-Target", "AmazonCodeWhispererStreamingService.GenerateAssistantResponse")
	req.Header.Set("Authorization", "Bearer mock-token")
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	defer resp.Body.Close()
	ttfb := time.Since(start)
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, int64(len(b)), 0, ttfb, nil
	}
	events, bytesRead, err := countKiroEventstreamFrames(resp.Body)
	return resp.StatusCode, bytesRead, events, ttfb, err
}

// worker issues requests from jobs until the channel closes.
func worker(ctx context.Context, id int, cfg config, client *http.Client, jobs <-chan int, out chan<- result) {
	for idx := range jobs {
		res := result{Index: idx, Worker: id, StartUnixMs: time.Now().UnixMilli()}
		reqCtx, cancel := context.WithTimeout(ctx, cfg.timeout)
		t0 := time.Now()

		var status int
		var nBytes int64
		var events int
		var ttfb time.Duration
		var err error
		if cfg.mode == "anthropic" {
			status, nBytes, events, ttfb, err = issueAnthropic(reqCtx, client, cfg)
		} else {
			status, nBytes, events, ttfb, err = issueKiro(reqCtx, client, cfg)
		}
		cancel()

		res.LatencyMs = float64(time.Since(t0).Microseconds()) / 1000.0
		res.TTFBMs = float64(ttfb.Microseconds()) / 1000.0
		res.Status = status
		res.Bytes = nBytes
		res.Events = events
		if err != nil && !errors.Is(err, io.EOF) {
			res.Error = err.Error()
		}
		out <- res
	}
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	rank := p / 100.0 * float64(len(sorted)-1)
	lo := int(math.Floor(rank))
	hi := int(math.Ceil(rank))
	if lo == hi {
		return sorted[lo]
	}
	frac := rank - float64(lo)
	return sorted[lo] + (sorted[hi]-sorted[lo])*frac
}

// aggregate turns a slice of results into a summary.
func aggregate(cfg config, start, end time.Time, results []result) summary {
	s := summary{
		Scenario:        cfg.scenario,
		Mode:            cfg.mode,
		URL:             cfg.url,
		Stream:          cfg.stream,
		Concurrency:     cfg.concurrency,
		StartedAt:       start,
		EndedAt:         end,
		TotalRequests:   len(results),
		SystemGoVersion: runtime.Version(),
		SystemOS:        runtime.GOOS,
		SystemArch:      runtime.GOARCH,
		SystemCPU:       runtime.NumCPU(),
	}
	if len(results) == 0 {
		return s
	}
	s.TotalDurationS = end.Sub(start).Seconds()
	if s.TotalDurationS > 0 {
		s.RPS = float64(len(results)) / s.TotalDurationS
	}

	latencies := make([]float64, 0, len(results))
	ttfbs := make([]float64, 0, len(results))
	var sumLat float64
	for _, r := range results {
		if r.Error == "" && r.Status >= 200 && r.Status < 300 {
			s.Successes++
		} else {
			s.Errors++
		}
		latencies = append(latencies, r.LatencyMs)
		if r.TTFBMs > 0 {
			ttfbs = append(ttfbs, r.TTFBMs)
		}
		sumLat += r.LatencyMs
		s.BytesTotal += r.Bytes
		s.EventsTotal += r.Events
	}
	s.ErrorRate = float64(s.Errors) / float64(len(results))
	sort.Float64s(latencies)
	sort.Float64s(ttfbs)
	s.LatencyP50Ms = percentile(latencies, 50)
	s.LatencyP95Ms = percentile(latencies, 95)
	s.LatencyP99Ms = percentile(latencies, 99)
	s.LatencyMaxMs = latencies[len(latencies)-1]
	s.LatencyMeanMs = sumLat / float64(len(results))
	if len(ttfbs) > 0 {
		s.TTFBP50Ms = percentile(ttfbs, 50)
		s.TTFBP95Ms = percentile(ttfbs, 95)
	}
	return s
}

func writeJSONL(path string, results []result) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, r := range results {
		if err := enc.Encode(r); err != nil {
			return err
		}
	}
	return nil
}

func writeJSON(path string, v any) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// runWarmup issues a handful of throwaway requests to prime keep-alive
// connections and fill any lazy state.
func runWarmup(ctx context.Context, client *http.Client, cfg config) {
	for i := 0; i < cfg.warmup; i++ {
		rctx, cancel := context.WithTimeout(ctx, cfg.timeout)
		if cfg.mode == "anthropic" {
			_, _, _, _, _ = issueAnthropic(rctx, client, cfg)
		} else {
			_, _, _, _, _ = issueKiro(rctx, client, cfg)
		}
		cancel()
	}
}

func main() {
	cfg := parseFlags()

	if err := os.MkdirAll(cfg.out, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "mkdir:", err)
		os.Exit(1)
	}

	// Share one transport so keep-alive across workers is realistic.
	tr := &http.Transport{
		MaxIdleConns:        cfg.concurrency * 2,
		MaxIdleConnsPerHost: cfg.concurrency * 2,
		MaxConnsPerHost:     cfg.concurrency * 2,
		IdleConnTimeout:     60 * time.Second,
	}
	client := &http.Client{Transport: tr, Timeout: cfg.timeout}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if cfg.warmup > 0 {
		fmt.Fprintf(os.Stderr, "warmup: %d requests...\n", cfg.warmup)
		runWarmup(ctx, client, cfg)
	}

	jobs := make(chan int, cfg.concurrency*2)
	out := make(chan result, cfg.concurrency*2)

	var wg sync.WaitGroup
	for i := 0; i < cfg.concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			worker(ctx, id, cfg, client, jobs, out)
		}(i)
	}

	// Producer.
	started := time.Now()
	var issued atomic.Int64
	producerDone := make(chan struct{})
	go func() {
		defer close(jobs)
		defer close(producerDone)
		if cfg.duration > 0 {
			deadline := time.Now().Add(cfg.duration)
			var i int
			for time.Now().Before(deadline) {
				jobs <- i
				i++
				issued.Store(int64(i))
			}
			return
		}
		for i := 0; i < cfg.total; i++ {
			jobs <- i
			issued.Store(int64(i + 1))
		}
	}()

	// Collect results as they come in; also periodically print progress.
	collectDone := make(chan []result, 1)
	go func() {
		results := make([]result, 0, 1024)
		tick := time.NewTicker(2 * time.Second)
		defer tick.Stop()
		for {
			select {
			case r, ok := <-out:
				if !ok {
					collectDone <- results
					return
				}
				results = append(results, r)
			case <-tick.C:
				elapsed := time.Since(started).Seconds()
				fmt.Fprintf(os.Stderr, "progress: issued=%d completed=%d elapsed=%.1fs rps=%.1f\n",
					issued.Load(), len(results), elapsed, float64(len(results))/math.Max(elapsed, 0.001))
			}
		}
	}()

	<-producerDone
	wg.Wait()
	close(out)
	results := <-collectDone
	ended := time.Now()

	// Sort by issue order for stable jsonl output.
	sort.Slice(results, func(i, j int) bool { return results[i].Index < results[j].Index })

	s := aggregate(cfg, started, ended, results)

	if err := writeJSONL(filepath.Join(cfg.out, "requests.jsonl"), results); err != nil {
		fmt.Fprintln(os.Stderr, "write jsonl:", err)
		os.Exit(1)
	}
	if err := writeJSON(filepath.Join(cfg.out, "summary.json"), s); err != nil {
		fmt.Fprintln(os.Stderr, "write summary:", err)
		os.Exit(1)
	}

	fmt.Printf("=== %s (%s) ===\n", s.Scenario, s.Mode)
	fmt.Printf("requests  : %d total, %d ok, %d err (%.2f%% err)\n",
		s.TotalRequests, s.Successes, s.Errors, s.ErrorRate*100)
	fmt.Printf("duration  : %.2fs, %.2f rps\n", s.TotalDurationS, s.RPS)
	fmt.Printf("latency   : p50=%.1fms p95=%.1fms p99=%.1fms max=%.1fms mean=%.1fms\n",
		s.LatencyP50Ms, s.LatencyP95Ms, s.LatencyP99Ms, s.LatencyMaxMs, s.LatencyMeanMs)
	if s.TTFBP50Ms > 0 {
		fmt.Printf("ttfb      : p50=%.1fms p95=%.1fms\n", s.TTFBP50Ms, s.TTFBP95Ms)
	}
	fmt.Printf("bytes     : %d total\n", s.BytesTotal)
	if s.EventsTotal > 0 {
		fmt.Printf("events    : %d total (%.1f per request)\n", s.EventsTotal, float64(s.EventsTotal)/float64(s.TotalRequests))
	}
	fmt.Printf("results   : %s\n", cfg.out)
}
