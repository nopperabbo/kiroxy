// kiroxy addition (not derived from upstream).
//
// GetUsageLimits queries the AWS Q Developer / Kiro getUsageLimits REST
// endpoint to introspect per-account credit state. The response feeds
// pool selection (bias away from nearly-exhausted accounts) and
// dashboard visibility.
//
// Endpoint shape confirmed via peer kiro-account-manager Rust client:
//
//	GET https://q.<region>.amazonaws.com/getUsageLimits
//	    ?isEmailRequired=true
//	    &origin=AI_EDITOR
//	    &profileArn=<arn>          // omitted for Builder-ID accounts
//	    &resourceType=AGENTIC_REQUEST
//	Authorization: Bearer <access_token>
//
// Calling getUsageLimits does NOT consume credits — it's a management-plane
// metadata read, cheaper than a chat request. The monthly ledger is keyed
// by profileArn for Enterprise / Workspace and by Builder-ID identity for
// social accounts.
//
// Sources: aws/amazon-q-developer-cli GetUsageLimitsOutput schema;
// hj01857655/kiro-account-manager src-tauri/src/clients/kiro_q_client.rs;
// kiroxy research-v4/sources/rate-limiting-research.md §Per-Account.

package kiroclient

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"
)

// UsageLimits is the credit-ledger snapshot for a single Kiro account.
// All fields default to zero when the upstream response omits them; the
// pool selection path treats a nil UsageLimits as "unknown, full weight"
// so introspection failure never makes an account unusable for chat.
type UsageLimits struct {
	// MonthlyCreditsUsed is the consumed-credit count within the current
	// billing window. Corresponds to UsageLimitList.currentUsage in the
	// AWS SDK schema, filtered to resourceType=AGENTIC_REQUEST.
	MonthlyCreditsUsed int64 `json:"monthly_credits_used"`

	// MonthlyCreditsRemaining is MonthlyCap - MonthlyCreditsUsed, clamped
	// to >= 0. Computed client-side; not present in the upstream body.
	MonthlyCreditsRemaining int64 `json:"monthly_credits_remaining"`

	// MonthlyCap is the tier quota (Free=50 / Pro=1000 / Pro+=2000 /
	// Power=10000 etc.). Corresponds to UsageLimitList.totalUsageLimit.
	MonthlyCap int64 `json:"monthly_cap"`

	// PercentUsed is the ratio [0..1] as reported upstream, or computed
	// client-side from used/cap when the upstream field is absent.
	PercentUsed float64 `json:"percent_used"`

	// RollingHourUsed is the best-effort count of credits consumed in the
	// last 60 minutes. The upstream API does NOT expose this today; the
	// field is plumbed so the pool can populate it from its own
	// per-account request history (the 60-minute credit window is
	// evidenced by the "60-minute credit limit exceeded" error message
	// — see research-v4/sources/rate-limiting-research.md).
	RollingHourUsed int64 `json:"rolling_hour_used,omitempty"`

	// NextReset is when the monthly window rolls over. Zero when absent.
	NextReset time.Time `json:"next_reset,omitempty"`

	// DaysUntilReset is the integer day count the upstream returns.
	DaysUntilReset int `json:"days_until_reset,omitempty"`

	// LastQueryTime is the wall-clock time of this poll. Dashboards
	// display it to show freshness.
	LastQueryTime time.Time `json:"last_query_time"`
}

// IsExhausted reports true when the monthly cap is known and the
// remaining count is zero. Safe on nil receiver (returns false).
func (u *UsageLimits) IsExhausted() bool {
	if u == nil {
		return false
	}
	return u.MonthlyCap > 0 && u.MonthlyCreditsRemaining <= 0
}

// PercentRemaining returns (MonthlyCap - MonthlyCreditsUsed) / MonthlyCap
// clamped to [0..1]. Returns 1.0 when nil or MonthlyCap is zero (treat
// "unknown" as "not drained" so pool weighting doesn't penalize
// accounts that failed one poll).
func (u *UsageLimits) PercentRemaining() float64 {
	if u == nil || u.MonthlyCap <= 0 {
		return 1.0
	}
	r := float64(u.MonthlyCap-u.MonthlyCreditsUsed) / float64(u.MonthlyCap)
	switch {
	case r < 0:
		return 0
	case r > 1:
		return 1
	}
	return r
}

// UsageErrorKind classifies why getUsageLimits failed so callers can
// react appropriately (quarantine a banned account, retry a transient
// 5xx, invalidate-and-refresh a 401 token).
type UsageErrorKind int

const (
	// UsageErrKindUnknown is the fallback bucket.
	UsageErrKindUnknown UsageErrorKind = iota
	// UsageErrKindUnauthorized: 401, or 403 without a ban reason. Token
	// is dead or lacks scope. Caller should refresh and retry.
	UsageErrKindUnauthorized
	// UsageErrKindBanned: 403 + reason="TemporarilySuspended" OR 423
	// Locked. Account is suspended upstream; caller should quarantine
	// rather than cool-down-and-retry.
	UsageErrKindBanned
	// UsageErrKindThrottled: 429. Upstream rate limit hit the management
	// plane too (rare; chat usually throttles first).
	UsageErrKindThrottled
	// UsageErrKindTransient: 5xx or network error. Retry later.
	UsageErrKindTransient
)

// UsageError bundles the upstream failure shape.
type UsageError struct {
	Status int
	Kind   UsageErrorKind
	Reason string
	Body   string
}

func (e *UsageError) Error() string {
	return fmt.Sprintf("getUsageLimits: status=%d kind=%d reason=%q body=%s",
		e.Status, e.Kind, e.Reason, e.Body)
}

// IsBanned is a typed shortcut for callers that want to quarantine.
func (e *UsageError) IsBanned() bool {
	return e != nil && e.Kind == UsageErrKindBanned
}

// IsTransient reports whether the caller should retry later.
func (e *UsageError) IsTransient() bool {
	return e != nil && e.Kind == UsageErrKindTransient
}

// usageLimitsURL returns the REST endpoint for getUsageLimits. Defaults
// to q.<region>.amazonaws.com (the management plane; the new
// runtime.<region>.kiro.dev endpoint is chat/streaming only). Override
// via KIROXY_USAGE_LIMITS_URL for tests and unusual deployments. The
// override must be the full URL including scheme.
func usageLimitsURL(region string) string {
	if override := os.Getenv("KIROXY_USAGE_LIMITS_URL"); override != "" {
		return override
	}
	return fmt.Sprintf("https://q.%s.amazonaws.com/getUsageLimits", region)
}

// usageBodyLimit caps the response body read. A well-formed usage
// response is well under 4 KiB; 64 KiB leaves headroom for future AWS
// schema drift without opening a memory DoS vector.
const usageBodyLimit = 64 * 1024

// GetUsageLimits queries the per-account credit ledger. The call is
// idempotent, non-consuming, and safe to invoke concurrently across
// different accounts.
//
// profileArn may be empty for Builder-ID accounts; in that case the
// query string omits the field entirely (Enterprise / Workspace
// accounts require it).
//
// On success the returned UsageLimits has LastQueryTime set to the
// wall-clock return time. On HTTP failures the returned error is a
// *UsageError with Kind classified so the pool can quarantine banned
// accounts vs. retry transient ones.
func GetUsageLimits(ctx context.Context, httpClient *http.Client, token, profileArn, region string) (*UsageLimits, error) {
	if httpClient == nil {
		return nil, errors.New("kiroclient: nil httpClient")
	}
	if token == "" {
		return nil, errors.New("kiroclient: empty access token")
	}
	if region == "" {
		return nil, errors.New("kiroclient: empty region")
	}

	base := usageLimitsURL(region)
	q := url.Values{}
	q.Set("isEmailRequired", "true")
	q.Set("origin", "AI_EDITOR")
	q.Set("resourceType", "AGENTIC_REQUEST")
	if profileArn != "" {
		q.Set("profileArn", profileArn)
	}

	sep := "?"
	if _, err := url.Parse(base); err == nil {
		if u, _ := url.Parse(base); u != nil && u.RawQuery != "" {
			sep = "&"
		}
	}
	fullURL := base + sep + q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("getUsageLimits: new request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgentValue)
	req.Header.Set("x-amz-user-agent", amzUserAgentValue)
	req.Header.Set("amz-sdk-request", "attempt=1; max=1")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, &UsageError{Kind: UsageErrKindTransient, Reason: err.Error()}
	}
	body := readLimitedBody(resp.Body, usageBodyLimit)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		ue := &UsageError{Status: resp.StatusCode, Body: body}
		switch resp.StatusCode {
		case http.StatusUnauthorized:
			ue.Kind = UsageErrKindUnauthorized
		case http.StatusForbidden:
			reason := extractUsageReason(body)
			if reason == "TemporarilySuspended" {
				ue.Kind = UsageErrKindBanned
				ue.Reason = reason
			} else {
				ue.Kind = UsageErrKindUnauthorized
				ue.Reason = reason
			}
		case http.StatusLocked:
			ue.Kind = UsageErrKindBanned
			ue.Reason = "423 Locked"
		case http.StatusTooManyRequests:
			ue.Kind = UsageErrKindThrottled
		default:
			if resp.StatusCode >= 500 {
				ue.Kind = UsageErrKindTransient
			}
		}
		return nil, ue
	}

	return parseUsageLimitsBody(body)
}

// parseUsageLimitsBody decodes the JSON response and selects the
// AGENTIC_REQUEST limit (the only type Kiro chat consumes). Missing or
// differently-shaped fields are tolerated — callers get a sparse
// UsageLimits rather than an error so one-off shape drift does not
// disable the polling pipeline.
func parseUsageLimitsBody(body string) (*UsageLimits, error) {
	if body == "" {
		return nil, errors.New("getUsageLimits: empty body")
	}

	var raw struct {
		Limits []struct {
			Type            string   `json:"type"`
			CurrentUsage    int64    `json:"currentUsage"`
			TotalUsageLimit int64    `json:"totalUsageLimit"`
			PercentUsed     *float64 `json:"percentUsed"`
		} `json:"limits"`
		NextDateReset  *int64 `json:"nextDateReset"`
		DaysUntilReset *int   `json:"daysUntilReset"`
	}
	if err := json.Unmarshal([]byte(body), &raw); err != nil {
		return nil, fmt.Errorf("getUsageLimits: parse json: %w", err)
	}

	u := &UsageLimits{LastQueryTime: time.Now()}
	for _, l := range raw.Limits {
		if l.Type != "AGENTIC_REQUEST" {
			continue
		}
		u.MonthlyCreditsUsed = l.CurrentUsage
		u.MonthlyCap = l.TotalUsageLimit
		switch {
		case l.PercentUsed != nil:
			u.PercentUsed = *l.PercentUsed
		case u.MonthlyCap > 0:
			u.PercentUsed = float64(u.MonthlyCreditsUsed) / float64(u.MonthlyCap)
		}
		break
	}

	remaining := u.MonthlyCap - u.MonthlyCreditsUsed
	if remaining < 0 {
		remaining = 0
	}
	u.MonthlyCreditsRemaining = remaining

	if raw.NextDateReset != nil {
		// AWS DateTime in JSON is either epoch seconds or epoch
		// milliseconds depending on the operation. Both peer code and
		// Smithy conventions for this endpoint emit epoch seconds; guard
		// against future drift by clamping implausible millisecond
		// values.
		ts := *raw.NextDateReset
		if ts > 10_000_000_000 { // > Nov 2286 if seconds, so treat as ms
			u.NextReset = time.UnixMilli(ts)
		} else {
			u.NextReset = time.Unix(ts, 0)
		}
	}
	if raw.DaysUntilReset != nil {
		u.DaysUntilReset = *raw.DaysUntilReset
	}
	return u, nil
}

// extractUsageReason pulls the "reason" string from an AWS error body.
// Returns "" on any parse failure.
func extractUsageReason(body string) string {
	var m struct {
		Reason string `json:"reason"`
	}
	_ = json.Unmarshal([]byte(body), &m)
	return m.Reason
}
