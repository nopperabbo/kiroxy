// Tests for the GetUsageLimits client. These run against an httptest
// fake — no network calls reach AWS / Kiro.

package kiroclient

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	tu "local/kiroxy/internal/testutil"
)

// successBody is the canonical happy-path JSON shape returned by
// q.<region>.amazonaws.com/getUsageLimits, distilled from
// amzn-codewhisperer-client/_get_usage_limits_output.rs and observed by
// peer kiro-account-manager. Embeds a Pro-tier 1000-credit ledger with
// 487 used so we can verify all three derived fields (remaining,
// percent_used, exhaustion).
const successBody = `{
  "limits": [
    {
      "type": "AGENTIC_REQUEST",
      "currentUsage": 487,
      "totalUsageLimit": 1000,
      "percentUsed": 48.7
    },
    {
      "type": "CODE_COMPLETIONS",
      "currentUsage": 11,
      "totalUsageLimit": 999999
    }
  ],
  "nextDateReset": 1717200000,
  "daysUntilReset": 13
}`

func mustParseUsage(t *testing.T, body string) *UsageLimits {
	t.Helper()
	u, err := parseUsageLimitsBody(body)
	if err != nil {
		t.Fatalf("parseUsageLimitsBody: %v", err)
	}
	return u
}

func TestGetUsageLimits_ParsesAgenticBucket(t *testing.T) {
	u := mustParseUsage(t, successBody)

	if u.MonthlyCap != 1000 {
		t.Errorf("MonthlyCap: got %d, want 1000", u.MonthlyCap)
	}
	if u.MonthlyCreditsUsed != 487 {
		t.Errorf("MonthlyCreditsUsed: got %d, want 487", u.MonthlyCreditsUsed)
	}
	if u.MonthlyCreditsRemaining != 513 {
		t.Errorf("MonthlyCreditsRemaining: got %d, want 513", u.MonthlyCreditsRemaining)
	}
	if u.PercentUsed < 48.69 || u.PercentUsed > 48.71 {
		t.Errorf("PercentUsed: got %f, want ~48.7", u.PercentUsed)
	}
	if u.DaysUntilReset != 13 {
		t.Errorf("DaysUntilReset: got %d, want 13", u.DaysUntilReset)
	}
	if u.NextReset.IsZero() {
		t.Error("NextReset should be parsed from epoch seconds")
	}
	if u.LastQueryTime.IsZero() {
		t.Error("LastQueryTime must be set")
	}
}

func TestGetUsageLimits_PercentRemainingComputesWhenAbsent(t *testing.T) {
	body := `{"limits":[{"type":"AGENTIC_REQUEST","currentUsage":250,"totalUsageLimit":1000}]}`
	u := mustParseUsage(t, body)
	if u.PercentUsed != 0.25 {
		t.Errorf("PercentUsed should fall back to used/cap, got %f", u.PercentUsed)
	}
	if got := u.PercentRemaining(); got < 0.749 || got > 0.751 {
		t.Errorf("PercentRemaining: got %f, want 0.75", got)
	}
}

func TestGetUsageLimits_HandlesEmptyAgenticBucket(t *testing.T) {
	// No AGENTIC_REQUEST entry means the account is provisioned only for
	// editor-style features; usage struct should be zero but valid.
	body := `{"limits":[{"type":"CODE_COMPLETIONS","currentUsage":5,"totalUsageLimit":100}]}`
	u := mustParseUsage(t, body)
	if u.MonthlyCap != 0 {
		t.Errorf("missing AGENTIC_REQUEST should leave MonthlyCap zero, got %d", u.MonthlyCap)
	}
	if got := u.PercentRemaining(); got != 1.0 {
		t.Errorf("zero cap PercentRemaining must default to 1.0 (no penalty), got %f", got)
	}
}

func TestGetUsageLimits_IsExhaustedAndPercentRemaining(t *testing.T) {
	cases := []struct {
		name         string
		used, cap    int64
		wantExhaust  bool
		wantRemainOk float64
	}{
		{"fresh", 0, 1000, false, 1.0},
		{"half", 500, 1000, false, 0.5},
		{"exhausted_exact", 1000, 1000, true, 0.0},
		{"exhausted_overage", 1100, 1000, true, 0.0},
		{"unknown", 0, 0, false, 1.0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			remaining := c.cap - c.used
			if remaining < 0 {
				remaining = 0
			}
			u := &UsageLimits{
				MonthlyCreditsUsed:      c.used,
				MonthlyCap:              c.cap,
				MonthlyCreditsRemaining: remaining,
			}
			if got := u.IsExhausted(); got != c.wantExhaust {
				t.Errorf("IsExhausted: got %v, want %v", got, c.wantExhaust)
			}
			if got := u.PercentRemaining(); got != c.wantRemainOk {
				t.Errorf("PercentRemaining: got %f, want %f", got, c.wantRemainOk)
			}
		})
	}
}

func TestUsageLimits_NilSafe(t *testing.T) {
	var u *UsageLimits
	if u.IsExhausted() {
		t.Error("nil receiver IsExhausted should be false")
	}
	if got := u.PercentRemaining(); got != 1.0 {
		t.Errorf("nil receiver PercentRemaining should be 1.0, got %f", got)
	}
}

// TestGetUsageLimits_E2ESuccess wires GetUsageLimits to a fake server
// and confirms the request shape and successful round-trip.
func TestGetUsageLimits_E2ESuccess(t *testing.T) {
	const wantArn = "arn:aws:codewhisperer:us-east-1:123:profile/test"
	srv := tu.NewTCP4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method: got %q, want GET", r.Method)
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			t.Errorf("missing Bearer auth: %q", r.Header.Get("Authorization"))
		}
		if r.URL.Query().Get("origin") != "AI_EDITOR" {
			t.Errorf("origin query: got %q, want AI_EDITOR", r.URL.Query().Get("origin"))
		}
		if r.URL.Query().Get("resourceType") != "AGENTIC_REQUEST" {
			t.Errorf("resourceType query: got %q, want AGENTIC_REQUEST", r.URL.Query().Get("resourceType"))
		}
		if r.URL.Query().Get("isEmailRequired") != "true" {
			t.Errorf("isEmailRequired query: got %q, want true", r.URL.Query().Get("isEmailRequired"))
		}
		if r.URL.Query().Get("profileArn") != wantArn {
			t.Errorf("profileArn query: got %q, want %q", r.URL.Query().Get("profileArn"), wantArn)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, successBody)
	}))
	defer srv.Close()
	t.Setenv("KIROXY_USAGE_LIMITS_URL", srv.URL+"/getUsageLimits")

	u, err := GetUsageLimits(context.Background(), &http.Client{Timeout: 5 * time.Second}, "tok", wantArn, "us-east-1")
	if err != nil {
		t.Fatalf("GetUsageLimits: %v", err)
	}
	if u.MonthlyCap != 1000 || u.MonthlyCreditsRemaining != 513 {
		t.Errorf("parse drift: cap=%d remaining=%d", u.MonthlyCap, u.MonthlyCreditsRemaining)
	}
}

// TestGetUsageLimits_BuilderIDOmitsProfileArn verifies that an empty
// profileArn is NOT serialized (Builder-ID accounts have no
// profileArn; the Smithy contract treats it as optional).
func TestGetUsageLimits_BuilderIDOmitsProfileArn(t *testing.T) {
	srv := tu.NewTCP4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := r.URL.Query()["profileArn"]; ok {
			t.Errorf("Builder-ID call must omit profileArn key entirely; got %v", r.URL.Query())
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, successBody)
	}))
	defer srv.Close()
	t.Setenv("KIROXY_USAGE_LIMITS_URL", srv.URL+"/getUsageLimits")

	if _, err := GetUsageLimits(context.Background(), &http.Client{Timeout: 5 * time.Second}, "tok", "", "us-east-1"); err != nil {
		t.Fatalf("GetUsageLimits (builder-id): %v", err)
	}
}

// TestGetUsageLimits_BannedAccountClassified verifies that a
// 403+TemporarilySuspended response is classified as Banned (the pool
// will quarantine, not cooldown-and-retry).
func TestGetUsageLimits_BannedAccountClassified(t *testing.T) {
	srv := tu.NewTCP4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = fmt.Fprint(w, `{"reason":"TemporarilySuspended","message":"account locked"}`)
	}))
	defer srv.Close()
	t.Setenv("KIROXY_USAGE_LIMITS_URL", srv.URL+"/getUsageLimits")

	_, err := GetUsageLimits(context.Background(), &http.Client{Timeout: 5 * time.Second}, "tok", "", "us-east-1")
	var ue *UsageError
	if !errors.As(err, &ue) {
		t.Fatalf("expected *UsageError, got %T (%v)", err, err)
	}
	if !ue.IsBanned() {
		t.Errorf("403+TemporarilySuspended must classify as banned, got kind=%d", ue.Kind)
	}
	if ue.Reason != "TemporarilySuspended" {
		t.Errorf("expected reason TemporarilySuspended, got %q", ue.Reason)
	}
}

// TestGetUsageLimits_LockedAccountClassified verifies 423 -> banned.
func TestGetUsageLimits_LockedAccountClassified(t *testing.T) {
	srv := tu.NewTCP4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusLocked)
	}))
	defer srv.Close()
	t.Setenv("KIROXY_USAGE_LIMITS_URL", srv.URL+"/getUsageLimits")

	_, err := GetUsageLimits(context.Background(), &http.Client{Timeout: 5 * time.Second}, "tok", "", "us-east-1")
	var ue *UsageError
	if !errors.As(err, &ue) || !ue.IsBanned() {
		t.Fatalf("423 must classify as banned, got %v", err)
	}
}

// TestGetUsageLimits_TransientServerError surfaces 5xx as transient so
// the poller retries on the next tick.
func TestGetUsageLimits_TransientServerError(t *testing.T) {
	srv := tu.NewTCP4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	t.Setenv("KIROXY_USAGE_LIMITS_URL", srv.URL+"/getUsageLimits")

	_, err := GetUsageLimits(context.Background(), &http.Client{Timeout: 5 * time.Second}, "tok", "", "us-east-1")
	var ue *UsageError
	if !errors.As(err, &ue) || !ue.IsTransient() {
		t.Fatalf("500 must classify as transient, got %v", err)
	}
}

// TestGetUsageLimits_UnauthorizedClassified maps 401 to Unauthorized so
// the caller refreshes the token instead of quarantining.
func TestGetUsageLimits_UnauthorizedClassified(t *testing.T) {
	srv := tu.NewTCP4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	t.Setenv("KIROXY_USAGE_LIMITS_URL", srv.URL+"/getUsageLimits")

	_, err := GetUsageLimits(context.Background(), &http.Client{Timeout: 5 * time.Second}, "tok", "", "us-east-1")
	var ue *UsageError
	if !errors.As(err, &ue) {
		t.Fatalf("expected *UsageError, got %T", err)
	}
	if ue.Kind != UsageErrKindUnauthorized {
		t.Errorf("401 must classify as Unauthorized, got kind=%d", ue.Kind)
	}
}

// TestGetUsageLimits_RejectsEmptyToken protects the caller from
// accidentally querying with an empty bearer.
func TestGetUsageLimits_RejectsEmptyToken(t *testing.T) {
	_, err := GetUsageLimits(context.Background(), &http.Client{}, "", "", "us-east-1")
	if err == nil || !strings.Contains(err.Error(), "empty access token") {
		t.Fatalf("empty token should fail loud, got %v", err)
	}
}

// TestUsageLimitsURL_DefaultsToManagementPlane confirms the default
// endpoint stays on q.<region>.amazonaws.com (the management plane);
// the runtime.<region>.kiro.dev endpoint serves chat only.
func TestUsageLimitsURL_DefaultsToManagementPlane(t *testing.T) {
	t.Setenv("KIROXY_USAGE_LIMITS_URL", "")
	got := usageLimitsURL("eu-central-1")
	want := "https://q.eu-central-1.amazonaws.com/getUsageLimits"
	if got != want {
		t.Errorf("default url: got %q, want %q", got, want)
	}
}

// TestUsageLimitsURL_OverrideHonored covers the test/override path.
func TestUsageLimitsURL_OverrideHonored(t *testing.T) {
	t.Setenv("KIROXY_USAGE_LIMITS_URL", "https://example.test/usage")
	if got := usageLimitsURL("any"); got != "https://example.test/usage" {
		t.Errorf("override ignored: got %q", got)
	}
}

// _ = url.Values{} silences the import when no test directly uses url.
var _ = url.Values{}
