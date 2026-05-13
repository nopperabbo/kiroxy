# Phase FEATURE-BOOST — Design

Date: 2026-05-13
Author: Sisyphus (Track 2 session, parallel with Track 1 BUG 1/2/3)
Goal: close feature-parity gap vs enowXlabs by shipping
session stickiness, weighted LRU with health score, and a Kiro CLI
request-shape audit. v1.0.0 is tagged; this targets v1.1.0-candidate.

---

## Coordination with Track 1 (parallel session)

Track 1 owns these mid-flight as of HEAD = `3521d1e`:

- `internal/kiroclient/client.go` (BUG 1)
- `internal/reqconv/*.go` (BUG 1)
- `cmd/kiroxy/import_json.go` (BUG 2)
- `internal/config/config.go` (BUG 3)
- mansion / variants handlers + dashboard.go (in flight per `git status`)

Track 2 (this session) is **strictly disjoint**:

- `internal/pool/*` (Track 1 does not touch)
- `internal/server/dashboard.go` (data-shape extension only — additive)
- `cmd/kiroxy/dashboard.go` (additive)

Package 3 (audit) is doc-only; we will NOT modify reqconv/kiroclient.
If discrepancies are found, file them in BACKLOG.md for Track 1 follow-up.

---

## Package 1 — Session Stickiness

### Problem

Round-robin LRU across N accounts means a multi-turn Claude Code
conversation can hit account A on turn 1, account B on turn 2, account
C on turn 3. Two failure modes:

1. **Prompt cache miss** — Anthropic / Kiro caches request prefix per
   account. Switching accounts mid-conversation invalidates the cache;
   each turn pays full prefix tokens.
2. **Kiro session state** — Kiro tracks per-account "session" semantics
   (FAIL-029 in research-v4 + the agentTaskType/conversationId fields
   we already wire in reqconv). Cross-account hops can confuse upstream.

### Solution

Pin `(X-Claude-Code-Session-Id) -> account_id` for a 60s window. Same
session within window always returns to the same account. After 60s
of idle, the pin expires and the next request re-picks via normal
LRU/weighted policy.

### Module: `internal/pool/stickiness.go` (new)

```go
type stickySession struct {
    accountID string
    expires   time.Time
}

type Stickiness struct {
    mu       sync.RWMutex
    sessions map[string]stickySession
    ttl      time.Duration
    enabled  bool

    // pruneStop is closed by Stop(); a background goroutine prunes
    // expired sessions every ttl/4 (default 15s).
    pruneStop chan struct{}
}

func NewStickiness(ttl time.Duration, enabled bool) *Stickiness
func (s *Stickiness) Pick(sessionID string, fallback func() string) string
func (s *Stickiness) Release(accountID string)  // clear pins on account failure
func (s *Stickiness) Stop()                     // graceful shutdown of pruner
func (s *Stickiness) Snapshot() map[string]string  // for dashboard / debug
```

**Behavior:**

- `Pick("", fb)` → `fb()` (no pin written; empty session does not own a slot)
- `Pick(id, fb)` where `id` exists & not expired → return existing pin
- `Pick(id, fb)` where `id` missing or expired → write `(fb(), now+ttl)` and return it
- `Release(accID)` → iterate map, drop any entries pinned to `accID`
  (called from `Pool.RecordFailure` so subsequent requests in the
  session re-pick a healthy account)
- Background goroutine prunes expired entries every `ttl/4` to bound
  memory under sustained churn.

### Pool integration

`Pool` gains:

```go
type Pool struct {
    ...
    stickiness *Stickiness  // nil when disabled
}

func (p *Pool) WithStickiness(s *Stickiness) *Pool  // chainable setter
```

`Pool.Pick(ctx, vault)` checks stickiness BEFORE the LRU/weighted scan:

```go
sessionID := logging.SessionIDFromContext(ctx)
if p.stickiness != nil && sessionID != "" {
    pinnedID := p.stickiness.Pick(sessionID, func() string {
        // fallback computes the LRU/weighted winner (see Package 2)
        return p.pickInternalLocked(now)
    })
    if a, ok := p.accounts[pinnedID]; ok && a.Enabled {
        h := p.health[pinnedID]
        if h == nil || !h.CooldownUntil.After(now) {
            // pin still valid; promote and return
            return p.servePickLocked(ctx, vault, a, now)
        }
    }
    // Pinned account became unhealthy after the pin was set.
    // Release and fall through to a fresh pick.
    p.stickiness.Release(pinnedID)
}
// normal LRU / weighted path
```

`Pool.RecordFailure` calls `p.stickiness.Release(id)` after applying
the cooldown so future requests in the same session re-pick.

### No new middleware

`logging.WithSessionID` is already called by `messages.HandleMessages`
(and OpenAI shim) before `s.auth.GetToken(ctx)`. The session ID is
already in the context that reaches `Pool.Pick`. We just READ it.

### Tests (`internal/pool/stickiness_test.go`)

1. `TestStickiness_PinsNewSession` — fresh ID writes pin, returns fallback value
2. `TestStickiness_ReturnsPinnedForSameSession` — second call returns pinned without invoking fallback
3. `TestStickiness_ExpiresAfterTTL` — fast-forward via direct map mutation; expired entry triggers fallback again
4. `TestStickiness_ReleaseClearsAccountPins` — multi-session pinning, Release(acc) drops only matching entries
5. `TestStickiness_EmptySessionUsesFallback` — empty string session never persists; fallback called every time
6. `TestPool_StickinessReturnsToSameAccount` — integration: same X-Claude-Code-Session-Id → same account across 5 picks; different session → may differ
7. `TestPool_StickinessReleasesOnFailure` — pin → RecordFailure(acc) → next pick with same session ID re-picks

### Commits

- c1: `feat(pool): session stickiness map with 60s TTL`
- c2: `feat(pool): wire stickiness to Pick + RecordFailure release`
- c3: `feat(pool): expose stickiness on Pool via constructor option`
- c4: `test(pool): stickiness TTL + failure-clear coverage`

---

## Package 2 — Weighted LRU with Health Score

### Problem

Plain LRU rotates accounts equally. If account A has 90% success rate
and account B has 30%, LRU still picks B every other turn — half of
client traffic goes to a flaky account. We want B to fade out
naturally as it accumulates failures, and re-enter once it recovers.

### Solution

Maintain rolling per-account health (success rate, recent rate-limit
events, recent load). Compute a weight in `(0.01, 1.0]` per account
and use weighted-random selection. Stickiness (Package 1) overrides
weighted selection within a session window.

### Module: `internal/pool/health.go` (new)

```go
type AccountHealth struct {
    // Rolling success ring buffer over last N requests.
    successRing  []bool
    ringHead     int
    ringFilled   bool
    ringSize     int  // capacity (default 100)

    // Rolling 5m request count (used for "recent load" decay).
    recentReqs   *timeRing  // time-bucketed sliding window

    LastRateLimit    time.Time
    ConsecutiveErrs  int
    AvgLatency       time.Duration  // EWMA, alpha=0.2
    HealthState  // embedded; Consecutive/CooldownUntil/LastError stay here
}

// SuccessRate returns ratio of true entries in the ring; defaults to 1.0
// when ring is empty (new accounts get a chance).
func (h *AccountHealth) SuccessRate() float64

// RequestsInLast5m returns the count from the time ring.
func (h *AccountHealth) RequestsInLast5m() int

// Weight in [0.01, 1.0]:
//   base = 1.0
//   * SuccessRate  // 0..1
//   * 0.1 if LastRateLimit < 30min ago, else 1.0
//   * max(0.3, 1.0 - reqs5m/100)
//   * max(0.1, 1.0 - 0.2 * ConsecutiveErrs)
//   clamped to [0.01, 1.0]
func (h *AccountHealth) Weight(now time.Time) float64
```

`HealthState` (existing struct in pool.go) is *embedded* into
`AccountHealth` rather than replaced, so existing tests
(`TestRecordFailure_*`, `TestRecordSuccess_*`) keep their semantics.

### Pool integration

The `health map[string]*HealthState` field is widened to
`map[string]*AccountHealth`. All call sites that access
`p.health[id]` go through the embedded `HealthState` fields and
keep compiling.

`RecordSuccess` / `RecordFailure` extend to also push into the rolling
ring + recent-req counter. New helper `RecordLatency(id, dur)` updates
EWMA from `kiroclient` request paths (optional wiring; default-zero
when not called doesn't break weights).

`Pool.Pick` (after stickiness check):

```go
candidates := []*Account{}
weights := []float64{}
sumW := 0.0
for _, a := range p.accounts {
    if !a.Enabled { continue }
    h := p.health[a.ID]
    if h != nil && h.CooldownUntil.After(now) { continue }
    w := 1.0
    if h != nil { w = h.Weight(now) }
    candidates = append(candidates, a)
    weights = append(weights, w)
    sumW += w
}
if len(candidates) == 0 { return nil, ErrNoAccount }

// Below 0.01 * len(candidates), all accounts are degraded; fall back to
// LRU order so we still pick *something* deterministic-ish.
if sumW <= 0.01 * float64(len(candidates)) {
    sort.Slice(candidates, func(i, j int) bool {
        return candidates[i].LastUsed.Before(candidates[j].LastUsed)
    })
    return p.servePickLocked(ctx, vault, candidates[0], now)
}

target := p.rng.Float64() * sumW
for i, w := range weights {
    target -= w
    if target <= 0 {
        return p.servePickLocked(ctx, vault, candidates[i], now)
    }
}
// floating-point fallthrough; pick last
return p.servePickLocked(ctx, vault, candidates[len(candidates)-1], now)
```

`p.rng` is a `*rand.Rand` seeded once at `New`. The mutex already
protects it.

### Dashboard surface

`server.DashboardAccount` gains:

```go
SuccessRate    float64 `json:"success_rate"`     // 0..1
Weight         float64 `json:"weight"`           // current weight
RequestsLast5m int     `json:"requests_last_5m"`
AvgLatencyMs   int64   `json:"avg_latency_ms,omitempty"`
```

`cmd/kiroxy/dashboard.go::DashboardSnapshot` populates them via
`pool.GetHealthSnapshot()` (new method that returns a map keyed by
account ID). Mansion + variants get the new fields for free via the
existing `/dashboard/api/state` consumer; visualization changes are
deferred (Track 1 owns mansion/variants right now).

### Tests (`internal/pool/health_test.go`)

1. `TestHealth_WeightDecreasesOnFailures` — push 10 failures into ring, weight < 0.5
2. `TestHealth_WeightRecoversAfterCooldown` — old rate-limit > 30min, multiplier returns to 1.0
3. `TestHealth_RingWrapsAtCapacity` — push 200 events into 100-slot ring, ring still reflects last 100
4. `TestHealth_RecentRequestsExpire` — bucket older than 5m drops from count
5. `TestPool_WeightedPickFavorsHealthy` — A weight=1.0, B weight=0.1 → over 1000 picks A wins ≥ 80% (chi² 0.95 confidence)
6. `TestPool_WeightedPickFallsBackWhenAllCold` — all accounts weight ≈ 0.01 → LRU order returns
7. `TestPool_HealthSnapshotMatchesDashboard` — snapshot keys match List() IDs

### Commits

- c5: `feat(pool): AccountHealth with rolling success ring + 5m load`
- c6: `feat(pool): weighted random Pick using health weight`
- c7: `feat(server): expose health in DashboardAccount JSON shape`
- c8: `test(pool): weighted pick distribution + degraded fallback`

---

## Package 3 — Kiro CLI Request Shape Audit (doc-only)

### Why doc-only

Track 1 is mid-flight in `internal/reqconv/*.go` and
`internal/kiroclient/client.go` for BUG 1 (the upstream 403). Editing
those files in parallel would cause merge conflicts and risk
regressing Track 1's fix.

### Deliverable

`.sisyphus/plans/kiro-cli-shape-audit.md` — a side-by-side comparison
of:

- **Reference:** documented Kiro CLI shape from
  `research-v4/PROTOCOL.md` (and `FAILURES.md` entries that mention
  field names, e.g. FAIL-016 body size, FAIL-029 conversationId).
- **Actual outbound:** kiroxy's `BuildPayload` output for a known
  fixture request, with the `KIROXY_TAP` environment variable
  capturing the wire bytes.

### Scope of audit doc

For each of the following, document `match | drift | unknown`:

- **HTTP method + URL path**
- **Headers:** `Authorization`, `User-Agent`, `Content-Type`,
  `X-Amz-User-Agent`, `Connection`, `Accept`
- **Body top-level keys:** `profileArn`, `conversationId`,
  `modelId`, `agentTaskType`, `messages`, `system`, `tools`,
  `toolConfig`, `inferenceConfig`, `additionalModelRequestFields`
- **Body field naming:** camelCase vs snake_case consistency
- **Optional vs omitted:** when a value is zero, do we send `""` or
  drop the key? Kiro CLI tends to drop.

### Outcome

If discrepancies are found, file each as a P0/P1 entry in BACKLOG.md
referencing this audit doc. Track 1 (or the next session) picks them
up; Track 2 does not modify reqconv/kiroclient in this phase.

If no discrepancies: ship the audit doc as a clean reference and
note "kiroxy outbound matches Kiro CLI shape as of <commit>".

### Commits

- c9: `docs(audit): kiro-cli request-shape side-by-side reference`
- (no c10/c11/c12 in Track 2 — any fixes are Track 1 follow-ups)

---

## Budget

- Package 1: ~2h (1 new file + 2 modified, 7 tests)
- Package 2: ~3h (1 new file + 2 modified, 7 tests, dashboard glue)
- Package 3: ~1h (audit doc only, no code)

Total: ~6h, matches operator brief.

## Hard rules

- No edits to: `internal/kiroclient/client.go`, `internal/reqconv/*.go`,
  `cmd/kiroxy/import_json.go`, `internal/config/config.go`.
- `make gate` green before each commit (GOEXPERIMENT=jsonv2 already
  exported in Makefile).
- No `git push`. No tag bump. No release notes; operator decides
  v1.1.0 cut.

## Failure modes guarded against

- **Goroutine leak from Stickiness pruner** — `Stop()` is wired into
  `Pool` shutdown path (or, if Pool has no shutdown, the pruner is
  short-lived and respects context cancellation in tests).
- **Stickiness retains entry after account removed from pool** —
  `Pool.Remove(id)` calls `s.stickiness.Release(id)`.
- **Weighted pick degenerate when no healthy accounts** — fallback to
  LRU among the same candidate set; if even LRU set is empty,
  `ErrNoAccount` (preserves existing API contract).
- **RNG concurrency** — `*rand.Rand` is *not* safe for concurrent use.
  We always call it under `p.mu`, which is already held in `Pick`.
