// Package pool manages a set of upstream provider accounts and selects one for
// each request. The selection policy is least-recently-used with cooldowns on
// accounts that have recently returned quota or repeated server errors.
//
// Derived from github.com/Quorinex/Kiro-Go @ 940dc782cb0a9a0d095abc6f407adf21ccc24ae2
// (pool/account.go, MIT). The original used weighted round-robin; we swap in
// LRU per personal-use build plan (single caller, one account at a time is
// fine; LRU just means "spread usage across accounts equally").
package pool

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/nopperabbo/kiroxy/internal/auth"
	"github.com/nopperabbo/kiroxy/internal/kiroclient"
	"github.com/nopperabbo/kiroxy/internal/logging"
	"github.com/nopperabbo/kiroxy/internal/metrics"
	"github.com/nopperabbo/kiroxy/internal/tokenvault"
)

// ErrNoAccount is returned by Pick when the pool is empty or every account is
// on cooldown or has run out of quota.
var ErrNoAccount = errors.New("pool: no usable account available")

// Account is one upstream account's metadata. The AccessToken is NOT stored
// here; it lives in the tokenvault and is fetched at the moment of use.
type Account struct {
	ID           string
	Label        string
	Provider     string
	Region       string
	Enabled      bool
	LastUsed     time.Time
	RequestCount int64
	ErrorCount   int64
}

// HealthState tracks the consecutive-error count and current cooldown window.
type HealthState struct {
	Consecutive   int
	CooldownUntil time.Time
	LastError     string
}

// Policy is the knobs that govern cooldown timing. Defaults are reasonable for
// personal use; tests may override.
type Policy struct {
	ConsecutiveErrorThreshold int
	ShortCooldown             time.Duration
	QuotaCooldown             time.Duration
	MaxCooldown               time.Duration
}

// DefaultPolicy returns the personal-use cooldown defaults.
func DefaultPolicy() Policy {
	return Policy{
		ConsecutiveErrorThreshold: 3,
		ShortCooldown:             1 * time.Minute,
		QuotaCooldown:             1 * time.Hour,
		MaxCooldown:               2 * time.Hour,
	}
}

// Pool owns the set of accounts, their health, and the LRU selection order.
type Pool struct {
	mu       sync.Mutex
	accounts map[string]*Account
	health   map[string]*HealthState
	policy   Policy

	// ext holds rolling AccountHealth used for the weighted-random
	// selection path. Sibling to `health`, not a replacement: `health`
	// drives cooldown gating, `ext` drives soft weighting.
	ext map[string]*AccountHealth

	// metricsSink is nil-safe. When set, cooldown transitions emit
	// account_cooldowns_total increments keyed by reason.
	metricsSink *metrics.Sink

	// stickiness pins session ID -> account for a short TTL. Nil when
	// disabled; Pool.Pick falls back to the normal LRU scan.
	stickiness *Stickiness

	// rng is used by the weighted-pick path. Not concurrency-safe on its
	// own; every access must hold p.mu.
	rng *rand.Rand
}

// New returns an empty Pool with the given policy.
func New(policy Policy) *Pool {
	return &Pool{
		accounts: make(map[string]*Account),
		health:   make(map[string]*HealthState),
		ext:      make(map[string]*AccountHealth),
		policy:   policy,
		// Seed is process-unique; deterministic tests override via
		// Pool.setRNGForTest.
		rng: rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), uint64(time.Now().UnixNano()>>32))),
	}
}

// SetMetricsSink attaches a metrics sink. Passing nil disables emission.
// Safe to call at most once at boot; not intended for hot-path reconfiguration.
func (p *Pool) SetMetricsSink(s *metrics.Sink) {
	p.metricsSink = s
}

// SetStickiness attaches a Stickiness tracker. Pass nil to disable.
// Safe to call at most once at boot. The Pool takes ownership of the
// Stickiness in the sense that Pool.Remove triggers a Release() on it;
// the caller retains the Stop() obligation.
func (p *Pool) SetStickiness(s *Stickiness) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stickiness = s
}

// Add registers an account in the pool. If an account with the same ID exists,
// its metadata is replaced and health is reset.
func (p *Pool) Add(a Account) {
	if a.LastUsed.IsZero() {
		a.LastUsed = time.Time{}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	cp := a
	p.accounts[a.ID] = &cp
	p.health[a.ID] = &HealthState{}
	p.ext[a.ID] = newAccountHealth()
}

// Remove deletes an account. No-op if absent. Also releases any
// session pins to this account so in-flight sessions re-pick.
func (p *Pool) Remove(id string) {
	p.mu.Lock()
	stick := p.stickiness
	delete(p.accounts, id)
	delete(p.health, id)
	delete(p.ext, id)
	p.mu.Unlock()
	if stick != nil {
		stick.Release(id)
	}
}

// List returns a snapshot of all accounts with their health state, sorted by
// label then ID for stable output.
func (p *Pool) List() []AccountStatus {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]AccountStatus, 0, len(p.accounts))
	for id, a := range p.accounts {
		h := p.health[id]
		if h == nil {
			h = &HealthState{}
		}
		out = append(out, AccountStatus{
			Account:       *a,
			Consecutive:   h.Consecutive,
			CooldownUntil: h.CooldownUntil,
			LastError:     h.LastError,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Label != out[j].Label {
			return out[i].Label < out[j].Label
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// AccountStatus bundles Account + HealthState for the inspection API.
type AccountStatus struct {
	Account
	Consecutive   int
	CooldownUntil time.Time
	LastError     string
}

// Count returns the total number of accounts (including disabled ones).
func (p *Pool) Count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.accounts)
}

// PickResult is what the caller receives when Pick succeeds: enough info to
// call upstream and later report success/failure back.
type PickResult struct {
	ID       string
	Provider string
	Region   string
	Token    string
}

// Pick selects an enabled account, fetches its access token from the vault,
// and bumps its LastUsed timestamp. Selection order:
//
//  1. If session stickiness is configured AND the context carries a session
//     ID (via logging.WithSessionID), the pinned account for that session
//     is returned when healthy.
//  2. Otherwise the LRU-oldest enabled account not on cooldown wins.
//
// When stickiness returns a pinned ID that is no longer usable (account
// removed / disabled / on cooldown), the pin is released and step 2 runs.
//
// The token fetch is done inside the lock so we atomically promote the account
// from LRU tail without another goroutine seeing the same LastUsed. The vault
// lookup itself is non-blocking (SQLite local file, microseconds).
func (p *Pool) Pick(ctx context.Context, vault *tokenvault.Vault) (*PickResult, error) {
	p.mu.Lock()
	now := time.Now()

	// Decay Consecutive counters on accounts whose cooldown has fully
	// elapsed. Without this, an account that recovered after a cooldown
	// keeps its old Consecutive count, and the NEXT failure compounds
	// onto it — the cooldown duration multiplier (Consecutive - threshold + 1)
	// then over-penalizes a fresh blip as if the prior cooldown never
	// happened. Cooldown expiry IS the recovery signal; reset it.
	for _, h := range p.health {
		if h == nil || h.CooldownUntil.IsZero() {
			continue
		}
		if now.After(h.CooldownUntil) && h.Consecutive > 0 {
			h.Consecutive = 0
			h.CooldownUntil = time.Time{}
			h.LastError = ""
		}
	}

	// selectWeighted scans the current account set, computes each
	// candidate's health weight, and returns one by weighted random
	// selection. Falls back to LRU-oldest when every live candidate has
	// collapsed to the weight floor (so we still pick SOMETHING
	// deterministic-ish rather than RNG-tossing among weight=0.01s).
	// Called under p.mu.
	selectWeighted := func() *Account {
		type cand struct {
			a *Account
			w float64
		}
		var cands []cand
		for _, a := range p.accounts {
			if !a.Enabled {
				continue
			}
			h := p.health[a.ID]
			if h != nil && h.CooldownUntil.After(now) {
				continue
			}
			w := weightCeil
			if ah := p.ext[a.ID]; ah != nil {
				w = ah.Weight(now)
			}
			cands = append(cands, cand{a: a, w: w})
		}
		if len(cands) == 0 {
			return nil
		}

		// Total weight sanity check: if every candidate is at or near the
		// floor (0.01), weighted random would effectively be uniform
		// random with high variance. Prefer deterministic LRU in that
		// degenerate case.
		var sumW float64
		degenerate := true
		for _, c := range cands {
			sumW += c.w
			if c.w > weightFloor*2 {
				degenerate = false
			}
		}
		if degenerate {
			var best *Account
			for _, c := range cands {
				if best == nil || c.a.LastUsed.Before(best.LastUsed) {
					best = c.a
				}
			}
			return best
		}

		target := p.rng.Float64() * sumW
		for _, c := range cands {
			target -= c.w
			if target <= 0 {
				return c.a
			}
		}
		// Floating-point drift can leave target > 0 after the loop;
		// return the last candidate for correctness.
		return cands[len(cands)-1].a
	}

	// selectLRU is kept for the stickiness fallback path so a fresh
	// session writes its pin at an account with predictable semantics.
	// Once the pin exists, all subsequent picks for that session bypass
	// this scan entirely.
	selectLRU := func() *Account {
		var best *Account
		for _, a := range p.accounts {
			if !a.Enabled {
				continue
			}
			h := p.health[a.ID]
			if h != nil && h.CooldownUntil.After(now) {
				continue
			}
			if best == nil || a.LastUsed.Before(best.LastUsed) {
				best = a
			}
		}
		return best
	}

	var best *Account
	sessionID := logging.SessionIDFromContext(ctx)
	if p.stickiness != nil && sessionID != "" {
		pinnedID := p.stickiness.Pick(sessionID, func() string {
			a := selectLRU()
			if a == nil {
				return ""
			}
			return a.ID
		})
		if pinnedID != "" {
			if a, ok := p.accounts[pinnedID]; ok && a.Enabled {
				h := p.health[pinnedID]
				if h == nil || !h.CooldownUntil.After(now) {
					best = a
				}
			}
			if best == nil {
				// Pin points at an account we can no longer use
				// (removed, disabled, or on cooldown). Drop it and
				// re-pick so the session migrates to a healthy one.
				p.stickiness.Release(pinnedID)
				best = selectLRU()
			}
		}
	} else {
		best = selectWeighted()
	}

	if best == nil {
		p.mu.Unlock()
		return nil, ErrNoAccount
	}

	bundle, err := vault.Get(ctx, best.Provider, best.ID)
	if err != nil {
		p.mu.Unlock()
		return nil, fmt.Errorf("pool pick: vault lookup for %s/%s: %w", best.Provider, best.ID, err)
	}
	best.LastUsed = now
	best.RequestCount++
	result := &PickResult{
		ID:       best.ID,
		Provider: best.Provider,
		Region:   best.Region,
		Token:    bundle.AccessToken,
	}
	p.mu.Unlock()
	return result, nil
}

// RecordSuccess clears consecutive errors and cooldown for the account
// and pushes a success into the rolling health ring.
func (p *Pool) RecordSuccess(id string) {
	p.RecordSuccessWithLatency(id, 0)
}

// RecordSuccessWithLatency is RecordSuccess that also feeds the EWMA
// latency tracker. Pass 0 to skip the latency update.
func (p *Pool) RecordSuccessWithLatency(id string, latency time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	h := p.health[id]
	if h == nil {
		return
	}
	h.Consecutive = 0
	h.CooldownUntil = time.Time{}
	h.LastError = ""
	if ah := p.ext[id]; ah != nil {
		ah.recordSuccess(time.Now(), latency)
	}
}

// FailureKind classifies an upstream error for cooldown purposes.
type FailureKind int

const (
	FailureTransient FailureKind = iota
	FailureQuota
)

// RecordFailure increments the account's error counters and sets a cooldown.
// Quota errors use the longer QuotaCooldown; transient errors accumulate
// toward ConsecutiveErrorThreshold before cooling down. Also feeds the
// rolling health ring so subsequent weighted selection biases away from
// the failing account even before the cooldown threshold trips.
func (p *Pool) RecordFailure(id string, kind FailureKind, reason string) {
	p.mu.Lock()
	a := p.accounts[id]
	if a == nil {
		p.mu.Unlock()
		return
	}
	a.ErrorCount++
	h := p.health[id]
	if h == nil {
		h = &HealthState{}
		p.health[id] = h
	}
	h.Consecutive++
	h.LastError = reason
	if ah := p.ext[id]; ah != nil {
		ah.recordFailure(time.Now(), kind)
	}
	// Cooldown was either already present or newly set; we only want to
	// emit the metric on the transition from "no cooldown" (or expired)
	// to a fresh future cooldown. Comparing the new value to the prior
	// value is wrong — every quota failure sets CooldownUntil = now+TTL
	// which is always > a stale prev, inflating the metric on every call.
	// The right test: prev was either zero or already in the past.
	prev := h.CooldownUntil
	now := time.Now()
	wasInactive := prev.IsZero() || !prev.After(now)
	cooldownJustApplied := false
	switch kind {
	case FailureQuota:
		h.CooldownUntil = now.Add(p.policy.QuotaCooldown)
		cooldownJustApplied = wasInactive
	default:
		if h.Consecutive >= p.policy.ConsecutiveErrorThreshold {
			cool := p.policy.ShortCooldown
			if mul := time.Duration(h.Consecutive - p.policy.ConsecutiveErrorThreshold + 1); mul > 1 {
				cool = time.Duration(int64(cool) * int64(mul))
			}
			if cool > p.policy.MaxCooldown {
				cool = p.policy.MaxCooldown
			}
			h.CooldownUntil = now.Add(cool)
			cooldownJustApplied = wasInactive
		}
	}
	slog.Debug("pool: account fault",
		slog.String("account_id", id),
		slog.Int("consecutive", h.Consecutive),
		slog.String("reason", reason),
		slog.String("cooldown_until", h.CooldownUntil.Format(time.RFC3339)))
	sink := p.metricsSink
	stick := p.stickiness
	p.mu.Unlock()
	// Emit outside the lock — the metric write can involve atomic ops and
	// we don't want metric collection to contend with the pool's hot path.
	if cooldownJustApplied {
		sink.Cooldown(cooldownReasonFor(kind))
	}
	// Release any session pins ONLY when the account just transitioned into
	// cooldown. Releasing on every transient failure (including ones that
	// haven't crossed ConsecutiveErrorThreshold) defeats stickiness for the
	// second turn even though the account is still healthy enough to serve
	// the next request — the user pays a fresh weighted-pick on every blip.
	if stick != nil && cooldownJustApplied {
		stick.Release(id)
	}
}

// RecordUnauthorized marks an account as unrecoverable due to a dead
// refresh_token. Unlike RecordFailure which uses the consecutive-error
// cooldown ladder, this applies the maximum cooldown immediately and tags
// the metric with CooldownReasonUnauthorized so operators can distinguish
// "transient throttling" from "go re-authenticate this account".
//
// Without this, an account with a revoked refresh_token would tight-loop:
// each Pick re-tries refresh, fails with ErrRefreshUnauthorized, and the
// account stays at full weight ready to be picked again immediately.
func (p *Pool) RecordUnauthorized(id string, reason string) {
	p.mu.Lock()
	a := p.accounts[id]
	if a == nil {
		p.mu.Unlock()
		return
	}
	a.ErrorCount++
	h := p.health[id]
	if h == nil {
		h = &HealthState{}
		p.health[id] = h
	}
	prev := h.CooldownUntil
	now := time.Now()
	wasInactive := prev.IsZero() || !prev.After(now)
	h.Consecutive++
	h.LastError = reason
	h.CooldownUntil = now.Add(p.policy.MaxCooldown)
	slog.Warn("pool: account unauthorized — refresh_token dead",
		slog.String("account_id", id),
		slog.String("reason", reason),
		slog.String("cooldown_until", h.CooldownUntil.Format(time.RFC3339)))
	sink := p.metricsSink
	stick := p.stickiness
	p.mu.Unlock()
	if wasInactive {
		sink.Cooldown(metrics.CooldownReasonUnauthorized)
	}
	if stick != nil {
		stick.Release(id)
	}
}

// RecordStructuralError marks an account as broken at the API contract level
// (UnknownOperationException, ValidationException, etc.). Unlike transient
// errors which recover via retry/rotate, structural errors signal the
// account's request shape is fundamentally incompatible with the upstream
// — typically because metadata is missing/wrong (e.g. Builder ID account
// with auth_method NULL routing to wrong AmazonQ target) or the account
// has been migrated/deprecated server-side.
//
// Applies the maximum cooldown immediately so the bad account stops
// pulling traffic. Does NOT delete the account — operator must inspect
// the dashboard, fix or remove. Tagged metric so structural errors are
// distinguishable from quota/transient on the alerting side.
func (p *Pool) RecordStructuralError(id string, reason string) {
	p.mu.Lock()
	a := p.accounts[id]
	if a == nil {
		p.mu.Unlock()
		return
	}
	a.ErrorCount++
	h := p.health[id]
	if h == nil {
		h = &HealthState{}
		p.health[id] = h
	}
	prev := h.CooldownUntil
	now := time.Now()
	wasInactive := prev.IsZero() || !prev.After(now)
	h.Consecutive++
	h.LastError = reason
	h.CooldownUntil = now.Add(p.policy.MaxCooldown)
	slog.Warn("pool: account structural error — needs operator attention",
		slog.String("account_id", id),
		slog.String("reason", reason),
		slog.String("cooldown_until", h.CooldownUntil.Format(time.RFC3339)))
	sink := p.metricsSink
	stick := p.stickiness
	p.mu.Unlock()
	if wasInactive {
		sink.Cooldown(metrics.CooldownReasonStructural)
	}
	if stick != nil {
		stick.Release(id)
	}
}

// RecordLatency blends one latency sample into the per-account EWMA
// without altering success/failure state. Useful for tracking
// first-byte / time-to-first-event before the final outcome is
// known. No-op if the account has no health slot.
func (p *Pool) RecordLatency(id string, latency time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if ah := p.ext[id]; ah != nil {
		ah.recordLatency(latency)
	}
}

// HealthSnapshots returns a slice of per-account HealthSnapshot for
// the dashboard / debug endpoints. Ordered by account ID for stable
// output. O(N) over the account map; not intended for hot paths.
func (p *Pool) HealthSnapshots() []HealthSnapshot {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := time.Now()
	out := make([]HealthSnapshot, 0, len(p.accounts))
	for id := range p.accounts {
		ah := p.ext[id]
		if ah == nil {
			out = append(out, HealthSnapshot{AccountID: id, SuccessRate: 1.0, Weight: weightCeil})
			continue
		}
		hs := HealthSnapshot{
			AccountID:        id,
			SuccessRate:      ah.SuccessRate(),
			Weight:           ah.Weight(now),
			RequestsInWindow: ah.RequestsInWindow(now),
			AvgLatency:       ah.AvgLatency,
			LastRateLimit:    ah.LastRateLimit,
		}
		if u := ah.UsageLimits; u != nil && u.MonthlyCap > 0 {
			hs.UsageKnown = true
			hs.UsageCap = u.MonthlyCap
			hs.UsageUsed = u.MonthlyCreditsUsed
			hs.UsageRemaining = u.MonthlyCreditsRemaining
			hs.UsagePercentUsed = u.PercentUsed
			hs.UsageLastPolled = u.LastQueryTime
			hs.UsageDaysUntilRst = u.DaysUntilReset
			hs.SubscriptionTitle = u.SubscriptionTitle
			hs.SubscriptionTier = string(u.Tier())
			hs.OverageCapable = u.OverageCapable
			hs.OverageRate = u.OverageRate
			hs.OverageCap = u.OverageCap
			hs.CurrentOverages = u.CurrentOverages
			hs.Currency = u.Currency
			hs.Email = u.Email
		}
		out = append(out, hs)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].AccountID < out[j].AccountID })
	return out
}

// ---- TokenGetter glue ----

// TokenGetter adapts Pool so the messages.Service (kirocc's handler) can ask
// "give me credentials for the next request" without knowing about accounts,
// cooldowns, or LRU order.
type TokenGetter struct {
	Pool    *Pool
	Vault   *tokenvault.Vault
	Refresh *RefreshConfig
}

// RecordFailure forwards to Pool.RecordFailure so callers holding only a
// *TokenGetter (the messages.Service does) can mark an account as failed
// without needing a direct *Pool reference. accountID is the ID returned
// on the auth.Credentials struct; a nil/unknown id is tolerated by the
// pool (the underlying RecordFailure returns silently).
//
// The quota flag maps to FailureKind:
//   - quota=true:  FailureQuota (immediate QuotaCooldown, default 1h),
//     for throttling / rate-limit / quota-exhausted errors.
//   - quota=false: FailureTransient (accumulates toward
//     ConsecutiveErrorThreshold before cooldown),
//     for 5xx gateway / InternalServer* / network blips.
//
// This signature satisfies messages.FailureRecorder so *TokenGetter is
// usable as both a TokenGetter and a FailureRecorder via duck-typing,
// without either package importing the other's types.
func (tg *TokenGetter) RecordFailure(accountID string, quota bool, reason string) {
	if accountID == "" || tg.Pool == nil {
		return
	}
	kind := FailureTransient
	if quota {
		kind = FailureQuota
	}
	tg.Pool.RecordFailure(accountID, kind, reason)
}

// RecordStructuralError forwards a structural-error signal to the pool. A
// structural error is an upstream rejection that does NOT recover via
// retry/rotate (UnknownOperationException, AccessDeniedException, etc.) —
// the account itself is incompatible with the upstream contract and needs
// operator attention. Maps to messages.StructuralRecorder via duck typing.
//
// No-op when accountID is empty or the pool is unset (test/fake paths).
func (tg *TokenGetter) RecordStructuralError(accountID string, reason string) {
	if accountID == "" || tg.Pool == nil {
		return
	}
	tg.Pool.RecordStructuralError(accountID, reason)
}

// GetUsage forwards to Pool.GetUsage so callers holding only a *TokenGetter
// (the messages.Service does) can read the most-recent UsageLimits snapshot
// for an account without holding a direct *Pool reference. Maps to
// messages.UsageLimitsLooker via duck typing.
//
// Returns nil when accountID is empty, the pool is unset (test/fake paths),
// or the UsagePoller has not yet recorded a snapshot for that account.
// Callers MUST treat nil as "tier unknown, fail open" — never block on it.
func (tg *TokenGetter) GetUsage(accountID string) *kiroclient.UsageLimits {
	if accountID == "" || tg.Pool == nil {
		return nil
	}
	return tg.Pool.GetUsage(accountID)
}

// GetToken implements messages.TokenGetter. It picks an account from the
// pool, loads the current access_token from the vault, and enriches the
// returned Credentials with any Kiro-specific fields stored in
// Bundle.Metadata (profile_arn, auth_method). The metadata JSON is
// tolerant: missing/empty/malformed fields leave Credentials fields zero.
func (tg *TokenGetter) GetToken(ctx context.Context) (*auth.Credentials, error) {
	p, err := tg.Pool.Pick(ctx, tg.Vault)
	if err != nil {
		return nil, err
	}

	// Load authoritative bundle from vault so we can read metadata and
	// (optionally) refresh before returning.
	b, _ := tg.Vault.Get(ctx, p.Provider, p.ID)

	// Phase 2.5: proactive refresh for social accounts near expiry.
	if b != nil && tg.Refresh != nil && tg.Refresh.RefreshFn != nil {
		md := parseAccountMetadata(b.Metadata)
		if md.AuthMethod == "social" && needsRefresh(md, tg.Refresh.effectiveSkew(), time.Now()) {
			refreshed, rerr := refreshOne(ctx, tg.Vault, tg.Refresh, p.Provider, p.ID, p.Region, metrics.RefreshKindProactive)
			if rerr != nil {
				if errors.Is(rerr, auth.ErrRefreshUnauthorized) {
					tg.Pool.RecordUnauthorized(p.ID, rerr.Error())
				}
				return nil, rerr
			}
			if refreshed != nil {
				b = refreshed
			}
		}
	}

	creds := &auth.Credentials{
		AccessToken: p.Token,
		Region:      p.Region,
		AuthType:    p.Provider,
		AccountID:   p.ID,
	}
	if b != nil {
		creds.AccessToken = b.AccessToken
		if b.Metadata != "" {
			md := parseAccountMetadata(b.Metadata)
			creds.ProfileARN = md.ProfileArn
			if md.AuthMethod != "" {
				creds.AuthType = md.AuthMethod
			}
			creds.MachineID = md.MachineID
		}
	}
	if creds.MachineID == "" && b != nil {
		creds.MachineID = ensureMachineID(ctx, tg.Vault, p.Provider, p.ID, b)
	}
	return creds, nil
}

// ensureMachineID lazy-generates a per-account UUID and persists it to
// vault metadata when the bundle has no machine_id yet. The generated ID
// is appended to outbound User-Agent headers so the per-account
// fingerprint matches what native Kiro IDE installs send (a unique UUID
// per install). Returns the existing machine_id when one is already
// stored or empty when persistence fails — callers fail open and the
// outbound UA degrades to the bare KiroIDE-<ver> form.
func ensureMachineID(ctx context.Context, vault *tokenvault.Vault, provider, connectionID string, b *tokenvault.Bundle) string {
	if vault == nil || b == nil {
		return ""
	}
	id := uuid.NewString()
	patch := map[string]any{"machine_id": id}
	tokens := tokenvault.Tokens{
		AccessToken:  b.AccessToken,
		RefreshToken: b.RefreshToken,
		Source:       b.Source,
	}
	if _, err := vault.CommitWithMetaPatch(ctx, provider, connectionID, b.Generation, tokens, patch); err != nil {
		slog.Debug("pool: persist machine_id failed; passing through without it",
			slog.String("account_id", connectionID), slog.String("err", err.Error()))
		return ""
	}
	return id
}
