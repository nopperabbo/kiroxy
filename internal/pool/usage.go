// kiroxy addition (not derived from upstream).
//
// Package pool — UsagePoller: background goroutine that periodically
// calls kiroclient.GetUsageLimits for every registered account and
// stashes the result on AccountHealth. Dashboard + pool selection read
// from the cached value.
//
// The poller is intentionally decoupled from the HTTP client via a
// function-typed PollFn so tests can inject deterministic fakes and so
// the pool package does not hard-depend on the kiroclient transport
// details. Production wiring lives in cmd/kiroxy/main.go where the
// kiroclient.GetUsageLimits is threaded in alongside its http.Client.
//
// Polling cadence default is 60s per account, in line with Kiro's own
// UI freshness band ("Credit usage is updated at least every 5 minutes"
// — kiro.dev/pricing FAQ). 60s stays well under that without generating
// management-plane load proportional to chat traffic.

package pool

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"local/kiroxy/internal/kiroclient"
	"local/kiroxy/internal/tokenvault"
)

// UsagePollFn is the upstream dependency the poller needs. It must be
// safe for concurrent calls (per-account polls run in sequence today,
// but a future operator knob could parallelize).
type UsagePollFn func(ctx context.Context, token, profileArn, region string) (*kiroclient.UsageLimits, error)

// UsagePollerConfig wires the background poller to the pool + vault
// and to the upstream call.
type UsagePollerConfig struct {
	// Pool is the account registry the poller walks.
	Pool *Pool

	// Vault supplies per-account access_token + metadata (profileArn).
	// Required: without it the poller cannot authenticate its calls.
	Vault *tokenvault.Vault

	// PollFn is the upstream getUsageLimits call. Nil disables the
	// poller entirely — Start becomes a no-op.
	PollFn UsagePollFn

	// Interval between full passes over the account list. Default 60s.
	// The Kiro UI updates credit usage at 5-minute granularity, so 60s
	// keeps the cache fresher than the upstream source itself without
	// over-loading the management plane.
	Interval time.Duration

	// Timeout is the per-account call deadline. Default 10s. One slow
	// account should not stall the entire pass.
	Timeout time.Duration

	// StartupDelay delays the FIRST pass by this duration. Zero means
	// start immediately. Production wiring sets this to a small jitter
	// so poller startup doesn't race the kiroclient boot.
	StartupDelay time.Duration
}

// UsagePoller polls getUsageLimits per account. Start once, Stop once.
type UsagePoller struct {
	cfg UsagePollerConfig

	// force receives account IDs that need an immediate out-of-band
	// poll (e.g. upstream just returned 429 on that account). Buffered
	// so the chat hot path does not block.
	force chan string

	// done is closed by Start's goroutine when it returns.
	done chan struct{}
	// stopOnce guards the Stop path against multiple closes.
	stopOnce sync.Once
	// cancel cancels the poller's own context; set inside Start.
	cancel context.CancelFunc
}

// NewUsagePoller constructs a poller with defaults applied. It does not
// start the goroutine — the caller must invoke Start(ctx).
func NewUsagePoller(cfg UsagePollerConfig) *UsagePoller {
	if cfg.Interval <= 0 {
		cfg.Interval = 60 * time.Second
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Second
	}
	return &UsagePoller{
		cfg:   cfg,
		force: make(chan string, 32),
		done:  make(chan struct{}),
	}
}

// Start launches the background goroutine. Safe to call at most once
// per poller. If PollFn is nil or Vault is nil, Start returns
// immediately without spawning anything (usage polling is a
// best-effort signal, not a correctness gate).
func (p *UsagePoller) Start(ctx context.Context) {
	if p == nil || p.cfg.PollFn == nil || p.cfg.Vault == nil || p.cfg.Pool == nil {
		close(p.done)
		return
	}
	runCtx, cancel := context.WithCancel(ctx)
	p.cancel = cancel
	go p.loop(runCtx)
}

// Stop signals the goroutine to exit and waits for it. Idempotent.
func (p *UsagePoller) Stop() {
	if p == nil {
		return
	}
	p.stopOnce.Do(func() {
		if p.cancel != nil {
			p.cancel()
		}
	})
	<-p.done
}

// ForcePoll enqueues an out-of-band poll for the given account. The
// request is coalesced: if the channel is full the enqueue is dropped,
// which is fine because the regular tick will cover it soon enough.
// Safe to call concurrently from any goroutine including the chat hot
// path. No-op when the poller is not running.
func (p *UsagePoller) ForcePoll(accountID string) {
	if p == nil || accountID == "" {
		return
	}
	select {
	case p.force <- accountID:
	default:
		// Channel full — regular tick will pick it up.
	}
}

// loop is the poller's main goroutine.
func (p *UsagePoller) loop(ctx context.Context) {
	defer close(p.done)

	if p.cfg.StartupDelay > 0 {
		select {
		case <-ctx.Done():
			return
		case <-time.After(p.cfg.StartupDelay):
		}
	}

	// First pass right away so the dashboard has data before the 60s
	// tick fires for the first time.
	p.pollAll(ctx)

	ticker := time.NewTicker(p.cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.pollAll(ctx)
		case id := <-p.force:
			p.pollOne(ctx, id)
		}
	}
}

// pollAll iterates every registered account and polls each in
// sequence. Sequential is intentional: 50 accounts x ~500ms upstream
// RTT = ~25s, still well under the 60s cadence, and keeps the
// management-plane request fanout modest.
func (p *UsagePoller) pollAll(ctx context.Context) {
	accounts := p.cfg.Pool.List()
	for _, a := range accounts {
		if ctx.Err() != nil {
			return
		}
		if !a.Enabled {
			continue
		}
		p.pollOne(ctx, a.ID)
	}
}

// pollOne fetches fresh token + metadata for a single account and
// calls the upstream. Silently dropped errors are routine (transient
// 5xx, account cooldown). Banned responses are logged loud and the
// cached UsageLimits gets a sentinel (exhausted=true, cap=used) so the
// pool's weighted selection deweights the account immediately.
func (p *UsagePoller) pollOne(ctx context.Context, accountID string) {
	a, ok := p.cfg.Pool.accountCopy(accountID)
	if !ok {
		return
	}
	if !a.Enabled {
		return
	}

	ctxT, cancel := context.WithTimeout(ctx, p.cfg.Timeout)
	defer cancel()

	bundle, err := p.cfg.Vault.Get(ctxT, a.Provider, a.ID)
	if err != nil || bundle == nil {
		slog.Debug("usage poller: vault lookup failed",
			slog.String("account_id", a.ID),
			slog.Any("err", err))
		return
	}
	md := parseAccountMetadata(bundle.Metadata)
	region := a.Region
	if region == "" {
		region = "us-east-1"
	}

	u, perr := p.cfg.PollFn(ctxT, bundle.AccessToken, md.ProfileArn, region)
	if perr == nil && u != nil {
		p.cfg.Pool.SetUsage(a.ID, u)
		slog.Debug("usage poller: updated",
			slog.String("account_id", a.ID),
			slog.Int64("remaining", u.MonthlyCreditsRemaining),
			slog.Int64("cap", u.MonthlyCap))
		return
	}

	var ue *kiroclient.UsageError
	if errors.As(perr, &ue) && ue.IsBanned() {
		slog.Warn("usage poller: account appears banned upstream",
			slog.String("account_id", a.ID),
			slog.String("reason", ue.Reason),
			slog.Int("status", ue.Status))
		// Stamp a sentinel so the pool knows the account is drained.
		// Using (cap=used) keeps MonthlyCreditsRemaining=0; Package 3
		// deweights that to the floor. We do NOT set a hard cooldown
		// here — the chat path handles banned states via existing
		// RecordFailure when requests actually fail. This keeps the
		// usage poller purely advisory.
		p.cfg.Pool.SetUsage(a.ID, &kiroclient.UsageLimits{
			MonthlyCap:              1,
			MonthlyCreditsUsed:      1,
			MonthlyCreditsRemaining: 0,
			LastQueryTime:           time.Now(),
		})
		return
	}

	// Transient / unauthorized / unknown: keep the stale cache so a
	// one-off blip does not erase the last good reading.
	slog.Debug("usage poller: call failed, keeping stale cache",
		slog.String("account_id", a.ID),
		slog.Any("err", perr))
}

// ---- Pool helpers for usage-cache access ----

// accountCopy returns a snapshot copy of the Account row under the
// pool lock. Intended for callers that need to read basic account
// metadata without holding the lock themselves. Returns (zeroed,
// false) when the account is absent.
func (p *Pool) accountCopy(id string) (Account, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	a := p.accounts[id]
	if a == nil {
		return Account{}, false
	}
	return *a, true
}

// SetUsage stores the latest UsageLimits sample for an account. Called
// by the usage poller; safe for concurrent callers. No-op when the
// account has been removed between the poll and the store.
func (p *Pool) SetUsage(id string, u *kiroclient.UsageLimits) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if ah := p.ext[id]; ah != nil {
		ah.UsageLimits = u
	}
}

// GetUsage returns a pointer to the cached UsageLimits for an account.
// Treated as read-only by callers — do not mutate the returned struct.
// Returns nil when the account is missing or has never been polled.
func (p *Pool) GetUsage(id string) *kiroclient.UsageLimits {
	p.mu.Lock()
	defer p.mu.Unlock()
	if ah := p.ext[id]; ah != nil {
		return ah.UsageLimits
	}
	return nil
}
