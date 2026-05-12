// kiroxy addition (not derived from upstream).

package pool

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"local/kiroxy/internal/metrics"
)

// Stats is a point-in-time summary of pool health. Accounts are classified
// into three disjoint buckets:
//
//   - Available: Enabled AND not currently in a cooldown window.
//   - Cooldown:  Enabled AND in an active cooldown (quota / repeated errors).
//   - Failed:    Disabled.
//
// Total equals len(accounts).
type Stats struct {
	Total     int
	Available int
	Cooldown  int
	Failed    int
}

// Snapshot returns current pool health counts. O(N) over the account map;
// callers should not invoke this from a hot path. It's scrape-time only
// (Prometheus pulls) and the worst realistic account count is a few dozen.
func (p *Pool) Snapshot() Stats {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := time.Now()
	var out Stats
	out.Total = len(p.accounts)
	for id, a := range p.accounts {
		if !a.Enabled {
			out.Failed++
			continue
		}
		h := p.health[id]
		if h != nil && h.CooldownUntil.After(now) {
			out.Cooldown++
			continue
		}
		out.Available++
	}
	return out
}

// RegisterPoolGauges attaches accounts_available / accounts_cooldown /
// accounts_failed GaugeFunc collectors to the given registry. The closures
// call Pool.Snapshot() at scrape time so the exposed values are always fresh
// without requiring a background poller.
//
// Returns the first registration error (rare; duplicate Describe keys).
func RegisterPoolGauges(r prometheus.Registerer, p *Pool) error {
	if r == nil || p == nil {
		return nil
	}
	gauges := []struct {
		name string
		help string
		fn   func() float64
	}{
		{"kiroxy_accounts_available", "Pool accounts currently usable (enabled and not on cooldown).",
			func() float64 { return float64(p.Snapshot().Available) }},
		{"kiroxy_accounts_cooldown", "Pool accounts currently in a cooldown window.",
			func() float64 { return float64(p.Snapshot().Cooldown) }},
		{"kiroxy_accounts_failed", "Pool accounts administratively disabled.",
			func() float64 { return float64(p.Snapshot().Failed) }},
	}
	for _, g := range gauges {
		if err := r.Register(prometheus.NewGaugeFunc(
			prometheus.GaugeOpts{Name: g.name, Help: g.help},
			g.fn,
		)); err != nil {
			return err
		}
	}
	return nil
}

// cooldownReasonFor translates a pool FailureKind + the resulting
// HealthState into the CooldownReason the metrics package expects.
// Called only when a cooldown is actually applied (not on every failure).
func cooldownReasonFor(kind FailureKind) metrics.CooldownReason {
	switch kind {
	case FailureQuota:
		return metrics.CooldownReasonQuota
	default:
		return metrics.CooldownReasonConsecutiveErrors
	}
}
