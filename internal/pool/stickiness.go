// kiroxy addition (not derived from upstream).
//
// Session stickiness pins a client session ID (X-Claude-Code-Session-Id)
// to a specific account for a short TTL window. This preserves upstream
// prompt-cache locality and session-state continuity across multi-turn
// conversations from a single client (e.g. claude-code, opencode).
//
// The pin is best-effort: a failed or cooled-down account invalidates
// the pin via Release(), and the next request in the same session falls
// through to the pool's normal selection policy.
package pool

import (
	"sync"
	"time"

	"github.com/nopperabbo/kiroxy/internal/safego"
)

// DefaultStickinessTTL is the per-session pin lifetime. 60s was chosen
// to match typical claude-code turn spacing without holding a pin open
// across a dormant conversation.
const DefaultStickinessTTL = 60 * time.Second

// stickySession is one pinned (session -> account) mapping.
type stickySession struct {
	accountID string
	expires   time.Time
}

// Stickiness is a bounded in-memory map of session pins with a
// background pruner. Zero value is NOT usable; construct via
// NewStickiness.
type Stickiness struct {
	mu       sync.RWMutex
	sessions map[string]stickySession
	ttl      time.Duration

	// pruneStop is closed by Stop(); the background pruner exits.
	pruneStop chan struct{}
	stopOnce  sync.Once

	// now is swappable in tests to fast-forward TTLs deterministically.
	now func() time.Time
}

// NewStickiness returns a Stickiness with the given TTL and starts a
// background pruner that sweeps expired entries every ttl/4 (minimum
// 10s). Pass ttl <= 0 to get DefaultStickinessTTL. Call Stop() when
// disposing the pool to terminate the pruner goroutine.
func NewStickiness(ttl time.Duration) *Stickiness {
	if ttl <= 0 {
		ttl = DefaultStickinessTTL
	}
	s := &Stickiness{
		sessions:  make(map[string]stickySession),
		ttl:       ttl,
		pruneStop: make(chan struct{}),
		now:       time.Now,
	}
	sweep := ttl / 4
	if sweep < 10*time.Second {
		sweep = 10 * time.Second
	}
	safego.Go("pool-stickiness-pruner", func() { s.runPruner(sweep) })
	return s
}

// Pick returns the account pinned to sessionID, or writes a new pin
// using the fallback picker. Empty sessionID bypasses the map entirely
// (no pin is written or read).
//
// The fallback closure is invoked at most once per call. It is
// responsible for the pool's normal selection logic (LRU / weighted).
// If fallback returns "" (no usable account), Pick returns "" and
// writes no pin.
func (s *Stickiness) Pick(sessionID string, fallback func() string) string {
	if sessionID == "" {
		return fallback()
	}

	// Fast path: read lock, check existing pin.
	s.mu.RLock()
	if existing, ok := s.sessions[sessionID]; ok {
		if existing.expires.After(s.now()) {
			s.mu.RUnlock()
			return existing.accountID
		}
	}
	s.mu.RUnlock()

	// Slow path: invoke fallback OUTSIDE the lock to avoid holding
	// stickiness's mutex while the pool re-evaluates candidates.
	picked := fallback()
	if picked == "" {
		return ""
	}

	// Re-acquire under write lock and double-check: another goroutine
	// may have raced us to the pin. Prefer the existing non-expired pin
	// so a single session converges to one account even under contention.
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.sessions[sessionID]; ok && existing.expires.After(s.now()) {
		return existing.accountID
	}
	s.sessions[sessionID] = stickySession{
		accountID: picked,
		expires:   s.now().Add(s.ttl),
	}
	return picked
}

// Release drops every pin that maps to accountID. Called when an
// account fails or goes on cooldown so subsequent session traffic
// re-picks a healthy account instead of hitting the same broken one.
func (s *Stickiness) Release(accountID string) {
	if accountID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for sid, sess := range s.sessions {
		if sess.accountID == accountID {
			delete(s.sessions, sid)
		}
	}
}

// Snapshot returns a copy of the current session -> account map.
// Intended for dashboard/debug use; not cheap under high churn.
func (s *Stickiness) Snapshot() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]string, len(s.sessions))
	now := s.now()
	for sid, sess := range s.sessions {
		if sess.expires.After(now) {
			out[sid] = sess.accountID
		}
	}
	return out
}

// Stop terminates the background pruner. Safe to call multiple times;
// subsequent calls are no-ops.
func (s *Stickiness) Stop() {
	s.stopOnce.Do(func() { close(s.pruneStop) })
}

// runPruner sweeps expired entries on a timer until Stop() closes the
// stop channel. The sweep is O(N) over the session map; N is expected
// to stay in the hundreds for realistic workloads.
func (s *Stickiness) runPruner(interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-s.pruneStop:
			return
		case <-t.C:
			s.prune()
		}
	}
}

// prune drops every expired entry in one pass under the write lock.
// Exported for tests that want deterministic pruning without relying
// on the ticker.
func (s *Stickiness) prune() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	for sid, sess := range s.sessions {
		if !sess.expires.After(now) {
			delete(s.sessions, sid)
		}
	}
}
