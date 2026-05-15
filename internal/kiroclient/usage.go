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
//
// Upstream shape (verified 2026-05-15 against q.us-east-1.amazonaws.com):
//
//	{
//	  "subscriptionInfo": {
//	    "subscriptionTitle": "KIRO PRO",
//	    "type": "Q_DEVELOPER_STANDALONE_PRO",
//	    "overageCapability": "OVERAGE_CAPABLE" | "OVERAGE_INCAPABLE",
//	    "subscriptionManagementTarget": "MANAGE",
//	    "upgradeCapability": "UPGRADE_CAPABLE"
//	  },
//	  "usageBreakdownList": [
//	    {
//	      "resourceType": "CREDIT",
//	      "currentUsage": 132,
//	      "currentUsageWithPrecision": 132.29,
//	      "usageLimit": 1000,
//	      "overageCap": 10000,
//	      "overageRate": 0.04,
//	      "currentOverages": 0,
//	      "currency": "USD",
//	      "unit": "INVOCATIONS",
//	      "displayName": "Credit",
//	      "nextDateReset": 1.780272E9
//	    }
//	  ],
//	  "userInfo": {"email": "<account>", "userId": "<sso uuid>"},
//	  "daysUntilReset": 0,
//	  "nextDateReset": 1.780272E9
//	}
//
// Earlier kiroxy code expected `limits[].type=AGENTIC_REQUEST` (a stale
// schema sketch). Real upstream uses usageBreakdownList[]+resourceType=CREDIT.
type UsageLimits struct {
	// MonthlyCreditsUsed is the consumed-credit count within the current
	// billing window. Maps to usageBreakdownList[resourceType=CREDIT].currentUsage.
	MonthlyCreditsUsed int64 `json:"monthly_credits_used"`

	// MonthlyCreditsRemaining is MonthlyCap - MonthlyCreditsUsed, clamped
	// to >= 0. Computed client-side; not present in the upstream body.
	MonthlyCreditsRemaining int64 `json:"monthly_credits_remaining"`

	// MonthlyCap is the tier quota (Free=50 / Pro=1000 / Pro+=2000 /
	// Power=10000). Maps to usageBreakdownList[resourceType=CREDIT].usageLimit.
	MonthlyCap int64 `json:"monthly_cap"`

	// PercentUsed is the ratio [0..1] computed client-side from used/cap.
	PercentUsed float64 `json:"percent_used"`

	// RollingHourUsed is plumbed for future request-history bookkeeping
	// (not exposed by the upstream API today).
	RollingHourUsed int64 `json:"rolling_hour_used,omitempty"`

	// NextReset is when the monthly window rolls over. Zero when absent.
	NextReset time.Time `json:"next_reset,omitempty"`

	// DaysUntilReset is the integer day count the upstream returns.
	DaysUntilReset int `json:"days_until_reset,omitempty"`

	// LastQueryTime is the wall-clock time of this poll. Dashboards
	// display it to show freshness.
	LastQueryTime time.Time `json:"last_query_time"`

	// SubscriptionTitle is the human-readable plan label, e.g. "KIRO PRO",
	// "KIRO FREE", "KIRO POWER". Sourced from subscriptionInfo.subscriptionTitle.
	// Empty string when the upstream omits the field.
	SubscriptionTitle string `json:"subscription_title,omitempty"`

	// SubscriptionType is the canonical SKU id, e.g. Q_DEVELOPER_STANDALONE_PRO,
	// Q_DEVELOPER_STANDALONE_FREE. Stable across language/UI changes; safer
	// than SubscriptionTitle for routing decisions. Sourced from
	// subscriptionInfo.type.
	SubscriptionType string `json:"subscription_type,omitempty"`

	// OverageCapable is true when the account can spend past its monthly
	// cap (paid overage). Maps from subscriptionInfo.overageCapability ==
	// "OVERAGE_CAPABLE". Free-tier accounts are typically not overage-capable.
	OverageCapable bool `json:"overage_capable,omitempty"`

	// OverageRate is the per-invocation overage cost (e.g. $0.04). Sourced
	// from usageBreakdownList[].overageRate. Zero when the account has no
	// overage facility.
	OverageRate float64 `json:"overage_rate,omitempty"`

	// OverageCap is the maximum overage the account can run up before
	// hard-stopping. Maps from usageBreakdownList[].overageCap. Zero when
	// no overage facility.
	OverageCap int64 `json:"overage_cap,omitempty"`

	// CurrentOverages is the count of invocations charged at OverageRate
	// in the current billing window. Maps from usageBreakdownList[].currentOverages.
	CurrentOverages int64 `json:"current_overages,omitempty"`

	// Currency is the ISO-4217 currency code for OverageRate (e.g. "USD").
	Currency string `json:"currency,omitempty"`

	// Email is the human-readable account identifier surfaced by upstream
	// userInfo.email. Useful for dashboards distinguishing accounts that
	// share a connection_id token but are reported with different emails.
	// May be empty when isEmailRequired=false or the account has none.
	Email string `json:"email,omitempty"`
}

// SubscriptionTier is a coarse tier classification derived from
// SubscriptionType + MonthlyCap. Used by routing/UI logic that wants to
// reason about Free vs Pro vs Power without hardcoding SKU strings.
type SubscriptionTier string

const (
	SubscriptionTierUnknown SubscriptionTier = "unknown"
	SubscriptionTierFree    SubscriptionTier = "free"
	SubscriptionTierPro     SubscriptionTier = "pro"
	SubscriptionTierProPlus SubscriptionTier = "pro_plus"
	SubscriptionTierPower   SubscriptionTier = "power"
)

// Tier classifies the subscription. Falls back to MonthlyCap-based heuristic
// when SubscriptionType is unrecognized (covers future SKU additions).
// Returns SubscriptionTierUnknown when both signals are absent.
func (u *UsageLimits) Tier() SubscriptionTier {
	if u == nil {
		return SubscriptionTierUnknown
	}
	switch u.SubscriptionType {
	case "Q_DEVELOPER_STANDALONE_FREE":
		return SubscriptionTierFree
	case "Q_DEVELOPER_STANDALONE_PRO":
		return SubscriptionTierPro
	case "Q_DEVELOPER_STANDALONE_PRO_PLUS":
		return SubscriptionTierProPlus
	case "Q_DEVELOPER_STANDALONE_POWER":
		return SubscriptionTierPower
	}
	// Fallback heuristic by cap. Caps observed in the wild:
	// Free=50, Pro=1000, Pro+=2000, Power=10000.
	switch {
	case u.MonthlyCap <= 0:
		return SubscriptionTierUnknown
	case u.MonthlyCap <= 100:
		return SubscriptionTierFree
	case u.MonthlyCap <= 1500:
		return SubscriptionTierPro
	case u.MonthlyCap <= 5000:
		return SubscriptionTierProPlus
	default:
		return SubscriptionTierPower
	}
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

// parseUsageLimitsBody decodes the JSON response into UsageLimits.
//
// Upstream shape lives under usageBreakdownList[].resourceType=CREDIT
// (verified live 2026-05-15). subscriptionInfo carries plan metadata,
// usageBreakdownList[] carries per-resource usage. Older versions of
// this file expected limits[].type=AGENTIC_REQUEST — that path is dead.
//
// Missing or differently-shaped fields are tolerated: callers get a
// sparse UsageLimits with zero MonthlyCap rather than an error, so a
// one-off shape drift does not disable the polling pipeline.
func parseUsageLimitsBody(body string) (*UsageLimits, error) {
	if body == "" {
		return nil, errors.New("getUsageLimits: empty body")
	}

	var raw struct {
		SubscriptionInfo struct {
			SubscriptionTitle string `json:"subscriptionTitle"`
			Type              string `json:"type"`
			OverageCapability string `json:"overageCapability"`
		} `json:"subscriptionInfo"`
		UsageBreakdownList []struct {
			ResourceType    string  `json:"resourceType"`
			CurrentUsage    int64   `json:"currentUsage"`
			UsageLimit      int64   `json:"usageLimit"`
			OverageCap      int64   `json:"overageCap"`
			OverageRate     float64 `json:"overageRate"`
			CurrentOverages int64   `json:"currentOverages"`
			Currency        string  `json:"currency"`
		} `json:"usageBreakdownList"`
		UserInfo struct {
			Email string `json:"email"`
		} `json:"userInfo"`
		// NextDateReset is *float64 not *int64 because upstream emits this
		// field as scientific-notation float (1.780272E9) not pure integer.
		// Go's int64 unmarshaler rejects scientific notation; float64
		// accepts both forms with 53-bit mantissa precision (good through
		// year 285,000 epoch seconds).
		NextDateReset  *float64 `json:"nextDateReset"`
		DaysUntilReset *int     `json:"daysUntilReset"`
	}
	if err := json.Unmarshal([]byte(body), &raw); err != nil {
		return nil, fmt.Errorf("getUsageLimits: parse json: %w", err)
	}

	u := &UsageLimits{
		LastQueryTime:     time.Now(),
		SubscriptionTitle: raw.SubscriptionInfo.SubscriptionTitle,
		SubscriptionType:  raw.SubscriptionInfo.Type,
		OverageCapable:    raw.SubscriptionInfo.OverageCapability == "OVERAGE_CAPABLE",
		Email:             raw.UserInfo.Email,
	}

	for _, b := range raw.UsageBreakdownList {
		if b.ResourceType != "CREDIT" {
			continue
		}
		u.MonthlyCreditsUsed = b.CurrentUsage
		u.MonthlyCap = b.UsageLimit
		u.OverageCap = b.OverageCap
		u.OverageRate = b.OverageRate
		u.CurrentOverages = b.CurrentOverages
		u.Currency = b.Currency
		break
	}

	if u.MonthlyCap > 0 {
		u.PercentUsed = float64(u.MonthlyCreditsUsed) / float64(u.MonthlyCap)
	}
	remaining := u.MonthlyCap - u.MonthlyCreditsUsed
	if remaining < 0 {
		remaining = 0
	}
	u.MonthlyCreditsRemaining = remaining

	if raw.NextDateReset != nil {
		ts := int64(*raw.NextDateReset)
		if ts > 10_000_000_000 {
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
