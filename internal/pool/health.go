// kiroxy addition (not derived from upstream).
//
// Per-account rolling health metrics: success rate over last N requests,
// recent request count, recent rate-limit flag, exponentially-weighted
// average latency, and a composite weight used by Pool.Pick to bias
// traffic toward healthy accounts.
//
// This is a SIBLING to pool.HealthState, not a replacement. HealthState
// tracks the hard cooldown/consecutive-error gating that determines
// whether an account is pickable at all. AccountHealth tracks the soft
// weighting signal used AFTER the account is deemed pickable.

package pool

import (
	"time"

	"github.com/nopperabbo/kiroxy/internal/kiroclient"
)

const (
	// defaultRingSize is the success window size used by NewAccountHealth.
	// 100 is small enough that one rough day of errors erodes the rate
	// quickly but big enough that a lone transient doesn't dominate.
	defaultRingSize = 100

	// defaultEWMAAlpha is the smoothing factor for AvgLatency. Closer to 1
	// tracks recent samples tightly; 0.2 balances stability with
	// responsiveness across typical Kiro latencies (sub-second to ~20s).
	defaultEWMAAlpha = 0.2

	// rateLimitCooldownWindow is how long a recent rate-limit event
	// continues to deweight an account. Outside this window the penalty
	// lifts and the account competes on its baseline.
	rateLimitCooldownWindow = 30 * time.Minute

	// recentWindow is the span over which RequestsInWindow counts.
	recentWindow = 5 * time.Minute

	// recentBucketDuration controls the granularity of the recent-request
	// time ring. 30s buckets over 5m = 10 buckets; acceptable memory per
	// account.
	recentBucketDuration = 30 * time.Second

	// weightFloor is the minimum weight a live account can have. Keeps
	// weighted selection from ignoring an account entirely while it
	// recovers; the cooldown gate in Pool.Pick is the hard skip.
	weightFloor = 0.01

	// weightCeil is the maximum weight. A fresh account starts here.
	weightCeil = 1.0

	// usageDrainedThreshold is the PercentRemaining cutoff below which
	// an account is treated as "effectively drained": weighted selection
	// collapses it to the floor so we don't exhaust the last ~10% of
	// credits with latency-inducing retries. Aligns with the enowX
	// finding that Kiro's 60-minute rolling window typically trips
	// before the monthly cap is fully spent.
	usageDrainedThreshold = 0.10
)

// AccountHealth holds rolling metrics for one account. All access goes
// through methods that take p.mu externally; the struct itself has no
// internal locking to keep the hot path cheap.
type AccountHealth struct {
	// successRing is a circular buffer of the last N outcomes. Each
	// slot is true on success, false on failure.
	successRing []bool
	ringHead    int
	ringFilled  int

	// recentReqs is a time-bucketed ring over `recentWindow`; each
	// bucket counts requests that started in that bucket.
	recentReqs recentCounter

	// LastRateLimit is the wall-clock time of the most recent quota /
	// rate-limit failure for this account. Zero when none observed.
	LastRateLimit time.Time

	// AvgLatency is an EWMA (alpha=defaultEWMAAlpha) over request
	// round-trip durations. Zero until the first RecordLatency call.
	AvgLatency time.Duration

	// UsageLimits is the last-known credit ledger snapshot for this
	// account, populated by the background UsagePoller. Nil until the
	// first successful poll; treated by Weight() as "unknown, full
	// credit" so an introspection failure never makes an account
	// unusable for chat. Package 3 factors UsageLimits.PercentRemaining
	// into the composite weight to bias traffic away from accounts
	// nearing their monthly cap.
	UsageLimits *kiroclient.UsageLimits
}

// newAccountHealth returns a health tracker sized to defaultRingSize.
func newAccountHealth() *AccountHealth {
	return &AccountHealth{
		successRing: make([]bool, defaultRingSize),
		recentReqs:  newRecentCounter(recentWindow, recentBucketDuration),
	}
}

// recordSuccess pushes a success into the ring and bumps the recent
// request counter. Called under p.mu.
func (h *AccountHealth) recordSuccess(now time.Time, latency time.Duration) {
	h.pushOutcome(true)
	h.recentReqs.add(now, 1)
	if latency > 0 {
		h.updateEWMA(latency)
	}
}

// recordFailure pushes a failure into the ring, bumps recent requests,
// and records a rate-limit timestamp when the failure was quota-kind.
func (h *AccountHealth) recordFailure(now time.Time, kind FailureKind) {
	h.pushOutcome(false)
	h.recentReqs.add(now, 1)
	if kind == FailureQuota {
		h.LastRateLimit = now
	}
}

// recordLatency updates the EWMA without pushing into the outcome
// ring. Used when the caller wants to track duration for a
// non-terminal event (e.g. first-byte latency before deciding
// success/failure).
func (h *AccountHealth) recordLatency(latency time.Duration) {
	if latency > 0 {
		h.updateEWMA(latency)
	}
}

// pushOutcome writes one slot into the circular buffer.
func (h *AccountHealth) pushOutcome(success bool) {
	h.successRing[h.ringHead] = success
	h.ringHead = (h.ringHead + 1) % len(h.successRing)
	if h.ringFilled < len(h.successRing) {
		h.ringFilled++
	}
}

// updateEWMA blends the new sample into AvgLatency.
func (h *AccountHealth) updateEWMA(sample time.Duration) {
	if h.AvgLatency == 0 {
		h.AvgLatency = sample
		return
	}
	const a = defaultEWMAAlpha
	h.AvgLatency = time.Duration(a*float64(sample) + (1-a)*float64(h.AvgLatency))
}

// SuccessRate returns the ratio of true entries in the ring, or 1.0
// when the ring is empty (fresh accounts get a chance).
func (h *AccountHealth) SuccessRate() float64 {
	if h.ringFilled == 0 {
		return 1.0
	}
	hits := 0
	// Iterate only the filled portion; unfilled slots are false but
	// shouldn't skew a new account's rate downward.
	for i := 0; i < h.ringFilled; i++ {
		if h.successRing[i] {
			hits++
		}
	}
	return float64(hits) / float64(h.ringFilled)
}

// RequestsInWindow returns the count of requests within the last
// `recentWindow` ending at now.
func (h *AccountHealth) RequestsInWindow(now time.Time) int {
	return h.recentReqs.count(now)
}

// Weight computes the composite selection weight in [weightFloor,
// weightCeil]. Factors:
//   - success rate (0..1)
//   - rate-limit penalty: 0.1x within rateLimitCooldownWindow, else 1.0
//   - recent-load decay: scales from 1.0 to 0.3 as reqs-in-window goes
//     from 0 to 100+
//   - usage-remaining penalty: linear above usageDrainedThreshold,
//     collapsed to the floor below it. Nil UsageLimits means "unknown",
//     which gets full credit so an introspection failure never makes
//     an account unpickable.
//
// Callers MUST hold p.mu when invoking; the ring/counter are not
// concurrent-safe on their own.
func (h *AccountHealth) Weight(now time.Time) float64 {
	w := weightCeil * h.SuccessRate()

	if !h.LastRateLimit.IsZero() && now.Sub(h.LastRateLimit) < rateLimitCooldownWindow {
		w *= 0.1
	}

	reqs := h.RequestsInWindow(now)
	load := 1.0 - float64(reqs)/100.0
	if load < 0.3 {
		load = 0.3
	}
	w *= load

	w *= usageFactor(h.UsageLimits)

	if w < weightFloor {
		w = weightFloor
	}
	if w > weightCeil {
		w = weightCeil
	}
	return w
}

// usageFactor translates the UsageLimits snapshot into a [0..1]
// multiplier for the composite weight. Nil or unknown-cap accounts
// return 1.0 (no penalty). Accounts below usageDrainedThreshold
// (< 10% remaining) are treated as drained and collapse to the floor
// fraction so weighted selection effectively skips them. Above the
// threshold the factor scales linearly — a 50%-remaining account
// competes at roughly half the strength of a fresh one, which spreads
// load across the fleet instead of exhausting one account at a time.
func usageFactor(u *kiroclient.UsageLimits) float64 {
	if u == nil || u.MonthlyCap <= 0 {
		return 1.0
	}
	r := u.PercentRemaining()
	if r <= usageDrainedThreshold {
		return weightFloor / weightCeil
	}
	return r
}

// ---- recentCounter: time-bucketed sliding window ----

// recentCounter is a ring of fixed-duration buckets. Adds go into the
// bucket for `now`; reads sum the buckets whose bucket-start falls
// inside the window. Memory is O(window / bucketDuration).
type recentCounter struct {
	bucketDur time.Duration
	buckets   []recentBucket
}

type recentBucket struct {
	bucketStart time.Time
	count       int
}

func newRecentCounter(window, bucketDur time.Duration) recentCounter {
	n := int(window / bucketDur)
	if n < 1 {
		n = 1
	}
	return recentCounter{
		bucketDur: bucketDur,
		buckets:   make([]recentBucket, n),
	}
}

// add increments the bucket that covers `now`. If the bucket is older
// than bucketDur, it resets first (implicit expiry).
func (r *recentCounter) add(now time.Time, n int) {
	bs := now.Truncate(r.bucketDur)
	idx := r.index(bs)
	b := &r.buckets[idx]
	if !b.bucketStart.Equal(bs) {
		b.bucketStart = bs
		b.count = 0
	}
	b.count += n
}

// count sums all buckets whose bucketStart is within `window` of now.
func (r *recentCounter) count(now time.Time) int {
	cutoff := now.Add(-time.Duration(len(r.buckets)) * r.bucketDur)
	total := 0
	for _, b := range r.buckets {
		if b.bucketStart.After(cutoff) {
			total += b.count
		}
	}
	return total
}

// index picks the ring slot for a truncated bucket start.
func (r *recentCounter) index(bucketStart time.Time) int {
	// UnixNano is fine for modulo; negative time values are not expected.
	slot := bucketStart.UnixNano() / int64(r.bucketDur)
	n := int64(len(r.buckets))
	m := slot % n
	if m < 0 {
		m += n
	}
	return int(m)
}

// ---- helper: lock-aware snapshot for dashboard ----

// HealthSnapshot bundles the weight-relevant fields into a copyable
// struct. Returned by Pool.HealthSnapshot for dashboard consumption;
// fields are JSON-tagged for direct serialization.
type HealthSnapshot struct {
	AccountID        string        `json:"account_id"`
	SuccessRate      float64       `json:"success_rate"`
	Weight           float64       `json:"weight"`
	RequestsInWindow int           `json:"requests_last_5m"`
	AvgLatency       time.Duration `json:"avg_latency_ns,omitempty"`
	LastRateLimit    time.Time     `json:"last_rate_limit,omitempty"`

	// Usage fields. Zero / empty when no successful poll has landed yet.
	UsageKnown        bool      `json:"usage_known"`
	UsageCap          int64     `json:"usage_cap,omitempty"`
	UsageUsed         int64     `json:"usage_used,omitempty"`
	UsageRemaining    int64     `json:"usage_remaining,omitempty"`
	UsagePercentUsed  float64   `json:"usage_percent_used,omitempty"`
	UsageLastPolled   time.Time `json:"usage_last_polled,omitempty"`
	UsageDaysUntilRst int       `json:"usage_days_until_reset,omitempty"`
}
