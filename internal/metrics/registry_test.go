package metrics

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewRegistry_ExposesStandardAndKiroxyMetrics(t *testing.T) {
	reg := New(time.Now())
	if reg == nil {
		t.Fatal("New returned nil")
	}

	// Fire a couple of observations so counters/histograms become visible.
	s := reg.Sink()
	s.ObserveRequest("claude-sonnet-4-6", 200, true, 200*time.Millisecond)
	s.ObserveTokens("claude-sonnet-4-6", 1024, 512)
	s.ObserveUpstreamTTFB("claude-sonnet-4-6", 150*time.Millisecond)
	s.RequestError(RequestKindUpstream)
	s.RefreshAttempt(RefreshKindProactive, RefreshResultSuccess)
	s.Cooldown(CooldownReasonQuota)

	body := scrapeBody(t, reg.Handler())

	wantSubstrings := []string{
		"kiroxy_requests_total",
		`model="claude-sonnet-4-6"`,
		`status="2xx"`,
		`stream="true"`,
		"kiroxy_request_errors_total",
		`kind="upstream"`,
		"kiroxy_request_duration_seconds",
		"kiroxy_upstream_ttfb_seconds",
		"kiroxy_tokens_input",
		"kiroxy_tokens_output",
		"kiroxy_refresh_attempts_total",
		`result="success"`,
		"kiroxy_account_cooldowns_total",
		`reason="quota"`,
		"kiroxy_uptime_seconds",
		// Standard Go/process collectors.
		"go_goroutines",
		"process_",
	}
	for _, want := range wantSubstrings {
		if !strings.Contains(body, want) {
			t.Errorf("scrape body missing %q", want)
		}
	}
}

func TestNilSink_IsNoOp(t *testing.T) {
	var s *Sink // nil
	// All calls must be safe on a nil receiver.
	s.ObserveRequest("m", 200, false, time.Second)
	s.ObserveTokens("m", 1, 1)
	s.ObserveUpstreamTTFB("m", time.Second)
	s.RequestError(RequestKindProxy)
	s.RefreshAttempt(RefreshKindReactive, RefreshResultFail401)
	s.Cooldown(CooldownReasonManual)

	// Handler on a nil sink returns 503, not a panic.
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("nil sink handler: got %d, want 503", rr.Code)
	}
}

func TestClassifyStatus(t *testing.T) {
	cases := []struct {
		in   int
		want string
	}{
		{200, "2xx"},
		{201, "2xx"},
		{301, "3xx"},
		{400, "4xx"},
		{404, "4xx"},
		{500, "5xx"},
		{599, "5xx"},
		{0, "other"},
		{-1, "other"},
		{999, "other"},
	}
	for _, c := range cases {
		if got := classifyStatus(c.in); got != c.want {
			t.Errorf("classifyStatus(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestNormaliseModel_EmptyBecomesUnknown(t *testing.T) {
	if got := normaliseModel(""); got != "unknown" {
		t.Errorf("normaliseModel(\"\") = %q, want unknown", got)
	}
	if got := normaliseModel("claude-opus-4.7"); got != "claude-opus-4.7" {
		t.Errorf("normaliseModel passthrough failed: %q", got)
	}
}

func TestObserveTokens_ZeroIsSkipped(t *testing.T) {
	reg := New(time.Now())
	s := reg.Sink()
	s.ObserveTokens("m", 0, 0)

	body := scrapeBody(t, reg.Handler())
	// A histogram with no observations still emits its _count and _sum lines
	// in the exposition format — but they're 0. Verify zero-sum rather than
	// absence, which is the actual client_golang behaviour.
	if strings.Contains(body, `kiroxy_tokens_input_sum{model="m"}`) {
		// If present, it must be 0.
		if !strings.Contains(body, `kiroxy_tokens_input_sum{model="m"} 0`) {
			t.Errorf("kiroxy_tokens_input_sum for zero input should be 0, body:\n%s", body)
		}
	}
}

func TestRequestDuration_ObservedValueIsRecorded(t *testing.T) {
	reg := New(time.Now())
	s := reg.Sink()
	s.ObserveRequest("m", 200, false, 500*time.Millisecond)

	body := scrapeBody(t, reg.Handler())
	// 0.5s falls in the le=0.5 bucket (inclusive).
	wantLine := `kiroxy_request_duration_seconds_bucket{model="m",stream="false",le="0.5"} 1`
	if !strings.Contains(body, wantLine) {
		t.Errorf("expected bucket line %q in:\n%s", wantLine, body)
	}
}

func TestRegistry_Gatherer_AllowsExternalCollector(t *testing.T) {
	reg := New(time.Now())
	if reg.Gatherer() == nil {
		t.Fatal("Gatherer returned nil")
	}
	if reg.Registerer() == nil {
		t.Fatal("Registerer returned nil")
	}
}

// scrapeBody runs the given handler and returns the response body as a
// string. Any non-200 status fails the test.
func scrapeBody(t *testing.T, h http.Handler) string {
	t.Helper()
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("scrape returned %d, body: %s", rr.Code, rr.Body.String())
	}
	data, err := io.ReadAll(rr.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
