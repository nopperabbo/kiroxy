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
	"sort"
	"sync"
	"time"

	"local/kiroxy/internal/auth"
	"local/kiroxy/internal/logging"
	"local/kiroxy/internal/metrics"
	"local/kiroxy/internal/tokenvault"
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

	// metricsSink is nil-safe. When set, cooldown transitions emit
	// account_cooldowns_total increments keyed by reason.
	metricsSink *metrics.Sink

	// stickiness pins session ID -> account for a short TTL. Nil when
	// disabled; Pool.Pick falls back to the normal LRU scan.
	stickiness *Stickiness
}

// New returns an empty Pool with the given policy.
func New(policy Policy) *Pool {
	return &Pool{
		accounts: make(map[string]*Account),
		health:   make(map[string]*HealthState),
		policy:   policy,
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
}

// Remove deletes an account. No-op if absent. Also releases any
// session pins to this account so in-flight sessions re-pick.
func (p *Pool) Remove(id string) {
	p.mu.Lock()
	stick := p.stickiness
	delete(p.accounts, id)
	delete(p.health, id)
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

	// selectLRU scans the current account set for the LRU-oldest healthy
	// candidate. Called under p.mu; may be invoked twice in the stickiness
	// path (once as fallback inside Stickiness.Pick, once on stale-pin
	// re-pick) but p.mu is held throughout so the scan is consistent.
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
		best = selectLRU()
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

// RecordSuccess clears consecutive errors and cooldown for the account.
func (p *Pool) RecordSuccess(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	h := p.health[id]
	if h == nil {
		return
	}
	h.Consecutive = 0
	h.CooldownUntil = time.Time{}
	h.LastError = ""
}

// FailureKind classifies an upstream error for cooldown purposes.
type FailureKind int

const (
	FailureTransient FailureKind = iota
	FailureQuota
)

// RecordFailure increments the account's error counters and sets a cooldown.
// Quota errors use the longer QuotaCooldown; transient errors accumulate
// toward ConsecutiveErrorThreshold before cooling down.
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
	// Cooldown was either already present or newly set; we only want to
	// emit the metric on the transition from "no cooldown" (or expired)
	// to a fresh future cooldown, so stash the previous value.
	prev := h.CooldownUntil
	cooldownJustApplied := false
	switch kind {
	case FailureQuota:
		h.CooldownUntil = time.Now().Add(p.policy.QuotaCooldown)
		cooldownJustApplied = h.CooldownUntil.After(prev)
	default:
		if h.Consecutive >= p.policy.ConsecutiveErrorThreshold {
			cool := p.policy.ShortCooldown
			if mul := time.Duration(h.Consecutive - p.policy.ConsecutiveErrorThreshold + 1); mul > 1 {
				cool = time.Duration(int64(cool) * int64(mul))
			}
			if cool > p.policy.MaxCooldown {
				cool = p.policy.MaxCooldown
			}
			h.CooldownUntil = time.Now().Add(cool)
			cooldownJustApplied = h.CooldownUntil.After(prev)
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
	// Release any session pins that were routing traffic to the failed
	// account, so the NEXT request in those sessions re-picks a healthy
	// account via Pool.Pick's normal selection path. Stickiness handles
	// a nil accountID gracefully; we gate on stick != nil here anyway so
	// disabled stickiness is a pure no-op.
	if stick != nil {
		stick.Release(id)
	}
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
	}
	if b != nil {
		creds.AccessToken = b.AccessToken
		if b.Metadata != "" {
			md := parseAccountMetadata(b.Metadata)
			creds.ProfileARN = md.ProfileArn
			if md.AuthMethod != "" {
				creds.AuthType = md.AuthMethod
			}
		}
	}
	return creds, nil
}
