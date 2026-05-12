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
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"local/kiroxy/internal/auth"
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
}

// New returns an empty Pool with the given policy.
func New(policy Policy) *Pool {
	return &Pool{
		accounts: make(map[string]*Account),
		health:   make(map[string]*HealthState),
		policy:   policy,
	}
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

// Remove deletes an account. No-op if absent.
func (p *Pool) Remove(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.accounts, id)
	delete(p.health, id)
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

// Pick selects the LRU-oldest enabled account that is not on cooldown, fetches
// its access token from the vault, and bumps its LastUsed timestamp.
//
// The token fetch is done inside the lock so we atomically promote the account
// from LRU tail without another goroutine seeing the same LastUsed. The vault
// lookup itself is non-blocking (SQLite local file, microseconds).
func (p *Pool) Pick(ctx context.Context, vault *tokenvault.Vault) (*PickResult, error) {
	p.mu.Lock()
	now := time.Now()

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
	defer p.mu.Unlock()
	a := p.accounts[id]
	if a == nil {
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
	switch kind {
	case FailureQuota:
		h.CooldownUntil = time.Now().Add(p.policy.QuotaCooldown)
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
		}
	}
	slog.Debug("pool: account fault",
		slog.String("account_id", id),
		slog.Int("consecutive", h.Consecutive),
		slog.String("reason", reason),
		slog.String("cooldown_until", h.CooldownUntil.Format(time.RFC3339)))
}

// ---- TokenGetter glue ----

// TokenGetter adapts Pool so the messages.Service (kirocc's handler) can ask
// "give me credentials for the next request" without knowing about accounts,
// cooldowns, or LRU order.
type TokenGetter struct {
	Pool  *Pool
	Vault *tokenvault.Vault
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
	creds := &auth.Credentials{
		AccessToken: p.Token,
		Region:      p.Region,
		AuthType:    p.Provider,
	}
	if b, err := tg.Vault.Get(ctx, p.Provider, p.ID); err == nil && b != nil && b.Metadata != "" {
		var md struct {
			ProfileArn string `json:"profile_arn"`
			AuthMethod string `json:"auth_method"`
		}
		if jerr := json.Unmarshal([]byte(b.Metadata), &md); jerr == nil {
			creds.ProfileARN = md.ProfileArn
			if md.AuthMethod != "" {
				creds.AuthType = md.AuthMethod
			}
		}
	}
	return creds, nil
}
