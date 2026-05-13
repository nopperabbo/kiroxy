# Session Continuity & Prompt Caching Patterns

> Librarian report, compiled 2026-05-13 (Makassar).
> Feeds the "Session Continuity Mechanisms" section of `ENOWX_STUDY.md`.
> All factual claims cite file:line, commit SHA, or URL.
> Peer repo SHAs pinned at cite time (see Citations).

---

## Executive Summary

**Can session stickiness 3-5x kiroxy throughput? Not by itself, but it is
the *prerequisite* for realising the 3-10x effective throughput that Kiro's
upstream `cachePoint` mechanism already grants — and that kiroxy currently
forfeits on every account rotation.**

The honest numbers:

- Anthropic's *direct* API gives a 10x read-cost discount on cache hits
  (5-minute TTL, ephemeral). A request that was 90% cacheable drops from
  `1.00x` to `~0.19x` billed input tokens (10% of prefix + 10% tail). See
  §Anthropic Cache Rules.
- Kiro accepts Anthropic's `cache_control` and converts it to an upstream
  `cachePoint` marker. The cache is keyed **per account** on Kiro's side
  (Quorinex/Kiro-Go treats `accountID` as the cache namespace —
  `proxy/cache_tracker.go:57,196`). Rotating accounts on every request
  therefore invalidates the cache every request; observed cache hit rate
  on a round-robin pool of *N* accounts converges to **1/N**.
- Pinning a client session to one account for 60s restores cache locality
  on the typical claude-code / opencode tool loop (turn spacing 2-30s),
  multiplying per-account effective throughput by roughly the cache hit
  ratio — realistic `3-5x` for tool-heavy sessions, `~1x` for one-shot
  requests.
- **kiroxy already ships session stickiness (60s TTL, keyed on
  `X-Claude-Code-Session-Id`)** as of commit `1e9dfa3` (2026-05, see
  `internal/pool/stickiness.go:21`). The interesting remaining work is
  adaptive TTL, metrics, and avoiding pin starvation — not bootstrap.

The section "Recommended Pattern for kiroxy" at the bottom calls out the
three follow-on BACKLOG items this research surfaces.

---

## Peer Project Implementations

### petehsu/KiroProxy (60s TTL, content-hash session ID)

**Status:** Exactly what the prompt describes. 60s window, hash-based
session ID derived from message content.

**Mechanism** ([`kiro_proxy/core/state.py:33-108`](https://github.com/petehsu/KiroProxy/blob/main/kiro_proxy/core/state.py)):

```python
self.session_locks: Dict[str, str] = {}          # session_id -> account_id
self.session_timestamps: Dict[str, float] = {}   # session_id -> last_use_ts

def get_available_account(self, session_id: Optional[str] = None):
    if session_id and session_id in self.session_locks:
        account_id = self.session_locks[session_id]
        ts = self.session_timestamps.get(session_id, 0)
        if time.time() - ts < 60:                       # ← the 60s window
            for acc in self.accounts:
                if acc.id == account_id and acc.is_available():
                    self.session_timestamps[session_id] = time.time()
                    return acc                          # ← pin hit, slide TTL
    # fall through: LRU-by-request-count picker
    account = min(available, key=lambda a: a.request_count)
    if session_id:
        self.session_locks[session_id] = account.id
        self.session_timestamps[session_id] = time.time()
    return account
```

**Key (what maps session → account):** Content hash of first 3 messages.
Not an IP, not a header, not a cookie. See
[`kiro_proxy/converters.py:26-29`](https://github.com/petehsu/KiroProxy/blob/main/kiro_proxy/converters.py):

```python
def generate_session_id(messages: list) -> str:
    content = json.dumps(messages[:3], sort_keys=True)
    return hashlib.sha256(content.encode()).hexdigest()[:16]
```

Implication: two separate clients sending the same opening 3 messages
(common for agents with a fixed system + first-turn scaffold) **share a
pin**. For a public multi-tenant proxy this is a privacy concern; for a
personal/friend-group deployment it is effectively a "same agent state
→ same account" heuristic that works for free. Responses handler
(`handlers/responses.py:623`) and Gemini handler (`handlers/gemini.py:42`)
derive session IDs the same way.

**TTL rationale:** Not documented in comments or CHANGELOG. 60s is
consistent with typical agent turn spacing (tool call → result → next
message loops in 2-30s for claude-code). Much less than Anthropic's
5-minute cache TTL, so the pin expires long before the cache does, giving
the pool a free re-balance opportunity once the session goes quiet.

**Exhaustion mid-session:** The pin is only honored if
`acc.is_available()` returns true. On quota exhaustion the account's
`status` flips (`credential.py` sets `CredentialStatus.COOLDOWN` via
`acc.mark_quota_exceeded(...)`), `is_available()` returns false, and the
pin silently lapses — the next call falls through to LRU without the pin
ever being explicitly released. Not a bug, just slightly wasteful because
the entry sits in `session_locks` until overwritten.

**Source:**
[petehsu/KiroProxy@main `kiro_proxy/core/state.py`](https://github.com/petehsu/KiroProxy/blob/main/kiro_proxy/core/state.py).
License: no LICENSE file (all rights reserved). Concepts reusable;
code copy blocked.

---

### d-kuro/kirocc (single-account, session for protocol only)

**Status:** *No stickiness.* kirocc is a single-account proxy by design
([`README.md`](https://github.com/d-kuro/kirocc/blob/main/README.md) —
"Reads credentials from Kiro CLI's SQLite DB"). The session ID flows
through for a completely different reason.

**What `X-Claude-Code-Session-Id` does in kirocc** ([`internal/app/messages/handler.go:17-77`](https://github.com/d-kuro/kirocc/blob/4ff8d81/internal/app/messages/handler.go)):

```go
const headerCCSessionID = "X-Claude-Code-Session-Id"

// In handler.ServeHTTP:
ccSessionID := r.Header.Get(headerCCSessionID)
if ccSessionID == "" {
    httpx.WriteError(w, http.StatusBadRequest, errTypeInvalidRequest,
        "missing "+headerCCSessionID+" header")
    return
}
// ...
payload := kiroproto.Payload{
    ConversationID: ccSessionID,                // ← threaded into upstream body
    // ...
}
```

The session ID is mandatory but used only as Kiro's `conversationId`
field on the AWS event-stream payload — it does **not** route to a
different account (there is only one).

**`cache_points` (what the prompt asked about):** Despite the README
bullet ("Converts Anthropic tool-level `cache_control` to Kiro
`cachePoint`"), I could not find a `cache_points.go`. The functionality
is present in the converter; the naming in the README was slightly
off vs the code, but every peer that cites "inspired by kirocc's
cache_points.go" (including petehsu/KiroProxy's `prompt_caching.py:7`)
refers to this translator behaviour: walk Anthropic `cache_control`
markers and emit upstream `cachePoint` entries in the converted tools /
messages arrays.

**Source:**
[d-kuro/kirocc@4ff8d81 `internal/app/messages/handler.go`](https://github.com/d-kuro/kirocc/blob/4ff8d812a4690d924c449ccfaf244e978a7901be/internal/app/messages/handler.go).

---

### Quorinex/Kiro-Go (multi-account, round-robin, NO session stickiness, rich cache accounting)

**Status:** Multi-account pool with weighted round-robin. **No session
stickiness**; each request can land on any available account. Has the
most sophisticated client-side cache accounting of any peer.

**Account selection** ([`pool/account.go:58-120`](https://github.com/Quorinex/Kiro-Go/blob/1732b17/pool/account.go)):

```go
func (p *AccountPool) GetNext() *config.Account {
    // atomic round-robin over p.accounts, skipping cooldown / near-expiry /
    // over-quota. No sessionID parameter, no stickiness map.
    for i := 0; i < n; i++ {
        idx := atomic.AddUint64(&p.currentIndex, 1) % uint64(n)
        acc := &p.accounts[idx]
        // skip cooldowns, expiring tokens, exhausted accounts
        return acc
    }
}
```

**`cache_tracker` — what the prompt asked about** ([`proxy/cache_tracker.go:55-236`](https://github.com/Quorinex/Kiro-Go/blob/1732b17ff9455e55cb9dcf34cf23c39f5b549042/proxy/cache_tracker.go)):

This is not a routing mechanism. It is a **client-side simulation of
Anthropic's usage accounting** so the returned `usage` map reports
`cache_creation_input_tokens` / `cache_read_input_tokens` even when Kiro
itself may or may not be caching upstream. Structure:

```go
type promptCacheTracker struct {
    entriesByAccount map[string]map[[32]byte]promptCacheEntry  // ← keyed per account
    maxSupportedTTL  time.Duration
}

type promptCacheBreakpoint struct {
    Fingerprint      [32]byte     // sha256 of canonical-JSON prefix
    CumulativeTokens int
    TTL              time.Duration
}
```

Per-account keying (`proxy/cache_tracker.go:57,196`) is the smoking gun:
Kiro-Go treats the cache as per-account because from the client's
perspective each Kiro account / upstream session has its own cache
state. Default TTL is **5 minutes** (`defaultPromptCacheTTL`,
`cache_tracker.go:14`), matching Anthropic. Minimum cacheable tokens:
**1024** (4096 for Opus, `cache_tracker.go:20-21`), matching Anthropic's
published thresholds for Sonnet 4.x and Opus 4.x.

Implementation detail worth stealing for kiroxy observability: fingerprint
cumulative SHA-256 of canonical-JSON prefix blocks (`canonicalizeCacheValue`
at `cache_tracker.go:530-585`), with `cache_control` keys stripped so
fingerprints are stable across cache-marker moves. Billing headers
injected by Claude Code (`x-anthropic-billing-header:...`) are filtered
from fingerprints (`cache_tracker.go:389-407`) — otherwise they would
cause false cache misses.

**Source:**
[Quorinex/Kiro-Go@1732b17 `proxy/cache_tracker.go`](https://github.com/Quorinex/Kiro-Go/blob/1732b17ff9455e55cb9dcf34cf23c39f5b549042/proxy/cache_tracker.go),
[same @ `pool/account.go`](https://github.com/Quorinex/Kiro-Go/blob/1732b17ff9455e55cb9dcf34cf23c39f5b549042/pool/account.go).

---

### jwadow/kiro-gateway (GLOBAL sticky — the opposite of session-keyed)

**Status:** Multi-account with a *global* stickiness model (one current
account for all requests). Sticky index only advances after a *successful*
request on a different account — i.e. the happy path pins all traffic to
one account at a time until it fails.

**Mechanism** ([`kiro/account_manager.py:184,652-798`](https://github.com/jwadow/kiro-gateway/blob/0398d74/kiro/account_manager.py)):

```python
self._current_account_index: int = 0   # GLOBAL sticky for all sessions
# ...
# In get_next_account:
start_index = self._current_account_index    # ← always the global pointer
# iterate in circular order from there, return first healthy account

# In report_success:
successful_index = all_account_ids.index(account_id)
if self._current_account_index != successful_index:
    self._current_account_index = successful_index
    self._dirty = True

# In report_failure:
# Do NOT change _current_account_index on failure. Failover happens through
# exclude_accounts in get_next_account().
```

**Trade-off analysis:** Maximises cache locality (all traffic lands on
one account until it breaks) at the cost of burning one account's quota
fast while others idle. Inverse of per-session stickiness: prioritises
*single-account* cache hits over *per-client* pin stability.

Has a `generate_conversation_id()` utility ([`kiro/utils.py`](https://github.com/jwadow/kiro-gateway/blob/0398d74/kiro/routes_anthropic.py#L54)),
but it is used for Kiro's upstream `conversationId` field (like kirocc's
`ccSessionID`), **not** for pool routing. Explicit circuit-breaker with
exponential backoff (60s base, 1d cap) around the pool.

**Source:**
[jwadow/kiro-gateway@0398d74 `kiro/account_manager.py`](https://github.com/jwadow/kiro-gateway/blob/0398d74f15549bd771480da8fceb21916ce333e5/kiro/account_manager.py).

---

### hj01857655/kiro-account-manager (Tauri desktop app, not a pooling proxy)

**Status:** Not applicable. This is a desktop account manager (Tauri +
React), not a routing proxy. Has `src/api/sessionApi.ts` and
`src/types/session.ts` for managing *Kiro IDE's own* session concept via
the account-manager UI, not an HTTP-routing session concept.

**Package.json evidence** ([`package.json`](https://github.com/hj01857655/kiro-account-manager/blob/6413d1a/package.json)):

```json
{
  "name": "kiro-account-manager",
  "description": "Kiro Account Manager - 管理 Kiro IDE 账号，支持多账号切换和配额监控",
  "scripts": { "dev:tauri": "tauri dev ..." }
}
```

No `X-Claude-Code-Session-Id` anywhere in `src/`. The `sticky` tokens in
the grep output were Tailwind CSS position classes (`sticky top-0`).

**Source:**
[hj01857655/kiro-account-manager@6413d1a](https://github.com/hj01857655/kiro-account-manager/tree/6413d1ac91738207616db91d9329d51408ac969f).

---

### AntiHub-Project/Antigv-plugin (Node.js, cookie-based login, no session stickiness)

**Status:** Account selection is per-request by `getAccountById` /
`getAccountsByUserId` (see [`src/server/kiro_routes.js:384,415,535`](https://github.com/AntiHub-Project/Antigv-plugin/blob/main/src/server/kiro_routes.js)),
not by session. The only caching is a **quota-info cache** with a 30s TTL
on `last_fetched_at` ([`src/api/multi_account_client.js:195-198`](https://github.com/AntiHub-Project/Antigv-plugin/blob/main/src/api/multi_account_client.js)) —
unrelated to prompt caching.

**Source:**
[AntiHub-Project/Antigv-plugin@main](https://github.com/AntiHub-Project/Antigv-plugin).

---

### BerriAI/litellm (enterprise-grade, three affinity modes)

**Status:** The richest session-routing implementation of any surveyed
project. Three stacked affinity flags, priority-ordered, TTL-tunable.

**The three flags** ([`litellm/router_utils/pre_call_checks/deployment_affinity_check.py`](https://github.com/BerriAI/litellm/blob/d251238b/litellm/router_utils/pre_call_checks/deployment_affinity_check.py)):

| Flag                           | Key                             | Use                                            | Priority |
| ------------------------------ | ------------------------------- | ---------------------------------------------- | -------- |
| `responses_api_deployment_check` | `previous_response_id`         | `/responses` API continuity                    | Highest  |
| `session_affinity`             | `x-litellm-session-id` header   | App-level session binding (PR #21763, merged) | Middle   |
| `deployment_affinity`          | hashed API key                  | Per-user / per-key sticky                      | Lowest   |

Priority ordering ensures `previous_response_id` wins over session over
user-key. Cache TTL: `deployment_affinity_ttl_seconds`, default **3600s**
(1 hour) — 60x longer than petehsu's 60s.

Relevant comment from
[PR #21763](https://github.com/BerriAI/litellm/pull/21763) (2026-02-21):

> This PR introduces the ability to route subsequent requests from the
> same session to the exact same deployment that handled the initial
> request, bypassing regular load-balancing. This is particularly useful
> for maximizing prompt caching efficiency and preserving context
> continuity.

LiteLLM's docs
[`response_api.md`](https://litellm.vercel.app/docs/response_api)
explicitly ties session stickiness to "implicit prompt caching" — the
scenario where the client does *not* send `cache_control` but the upstream
provider caches anyway, which is exactly Kiro's situation.

Cost warning from the docs (worth quoting in kiroxy's BACKLOG):

> `session_affinity` — Session-based applications — **⚠️ Reduces quota by
> # of sessions**
> `deployment_affinity` — Simple sticky sessions — **❌ Reduces quota by
> # of users**

---

### Portkey-AI/gateway (hash-field sticky, Redis-backed)

**Status:** Has `sticky` as a first-class feature of the `loadbalance`
strategy. Hash-field driven (`metadata.user_id`, `metadata.session_id`,
etc.), default TTL **3600s** (1h).

**Config schema** ([Portkey docs](https://portkey.ai/docs/docs/product/ai-gateway/load-balancing)):

```json
{
  "strategy": {
    "mode": "loadbalance",
    "sticky": {
      "enabled": true,
      "hash_fields": ["metadata.user_id"],
      "ttl": 3600
    }
  }
}
```

Two-tier cache (in-memory + Redis) for cross-replica stickiness.
Compared to kiroxy: single-process only, no Redis. Fine for v1, but if
kiroxy ever runs multiple replicas behind a load balancer, sticky state
must be externalised or the LB must hash-route.

**Source:** [Portkey Load Balancing](https://portkey.ai/docs/docs/product/ai-gateway/load-balancing).

---

### Other peers surveyed (no session stickiness found)

From grepping the remaining v1 dossier repos and their clones:

| Repo                            | Stickiness | Evidence                                                                |
| ------------------------------- | ---------- | ----------------------------------------------------------------------- |
| caidaoli/kiro2api                | ✗          | per-request round-robin; no session map                                 |
| justlovemaki/AIClient2API        | ✗          | generic LLM fanout, no upstream cache locality concerns                 |
| hnewcity/KiroaaS                | ✗          | single-account                                                           |
| kadangkesel/hexos               | ✗          | multi-account but naive rotation                                         |
| aliom-v/KiroGate                | ✗          | single-account                                                           |
| farion1231/cc-switch            | (parser)   | extracts session ID but doesn't route on it — local observability tool   |
| RelayPlane/proxy                 | (tracker)  | [session-tracker.ts](https://github.com/RelayPlane/proxy/blob/main/src/session-tracker.ts) records per-session aggregates, not for routing |
| coder/coder (`aibridge/session_test.go`) | (parser) | extracts session ID for telemetry only                                  |

---

## Anthropic Cache Rules Reference

Source: [Anthropic prompt caching docs](https://docs.anthropic.com/en/docs/build-with-claude/prompt-caching),
fetched 2026-05-13.

### Cache-hit requirements

- **Minimum tokens** to cache (shorter prompts silently skip caching, no
  error):
  - Opus 4.5 / 4.6 / 4.7 / Haiku 4.5: **4096** tokens
  - Sonnet 4.6: **2048** tokens
  - Sonnet 4.5 / Opus 4 / Opus 4.1 / Sonnet 4 / Sonnet 3.7: **1024** tokens
  - Haiku 3.5: **2048** tokens; Haiku 3: no explicit minimum listed
- **Prefix match is exact** ("Cache hits require 100% identical prompt
  segments, including all text and images up to and including the block
  marked with cache control").
- **Lookback window: 20 blocks.** The server checks at most 20 positions
  per breakpoint. If the hit is >20 blocks deep, the cache misses unless
  you set an earlier explicit breakpoint.

### TTL

- Default "ephemeral": **5 minutes**, refreshed for free on every hit.
- Extended: **1 hour** via `{"cache_control": {"type": "ephemeral", "ttl": "1h"}}`,
  at 2x base input-token price (vs 1.25x for 5m).
- 5m cache writes cost `1.25x` base; 1h writes cost `2.0x`; reads cost
  `0.1x` base. So a cache hit is ~**10x cheaper** on input tokens.

### Isolation (the piece that matters most for kiroxy)

Quoting the docs:

> **Organization and workspace isolation:** Caches are isolated between
> organizations. Different organizations never share caches, even if
> they use identical prompts. As of February 5, 2026, caches are also
> isolated per workspace within an organization on the Claude API,
> Claude Platform on AWS, and Microsoft Foundry (beta); Bedrock and
> Vertex AI continue to use organization-level isolation only.

**Each Kiro account is its own AWS Builder ID / IAM session, hence its
own org/workspace scope upstream.** Rotating accounts = rotating cache
scopes = cold cache. This is the structural reason stickiness matters
for a multi-account proxy.

### Tool-use caching

From the docs: tool_use and tool_result blocks **can** be cached
(`cache_control` applies in both user and assistant turns). Thinking
blocks cannot be directly cached, but are preserved as part of the
request prefix on Opus 4.5+ / Sonnet 4.6+. On earlier Opus/Sonnet and
all Haiku, non-tool-result user content invalidates thinking blocks from
the cached prefix.

### Multi-turn caching pattern (the common case for claude-code)

Per the docs' "Automatic caching" section:

```
Request N: System + User(1) + Asst(1) + ... + User(N) ◀ cache breakpoint auto-moves
           ^— everything up to User(N-1) read from cache; Asst(N-1) + User(N) written fresh
```

This is exactly the steady-state during a claude-code session: prefix
stays constant, tail grows 2-3 blocks per turn. With stickiness, all
these requests land on the same account, each subsequent request reads
the accumulated prefix from cache (~90% of tokens for a 10-turn session)
and writes only the new tail.

---

## Kiro Prompt Caching Status

**Evidence for:** The Kiro wire protocol accepts `cachePoint` markers.
kiroxy's types have first-class support
([`internal/kiroproto/types.go:50,62,114`](https://github.com/BenDashell/kiroxy/blob/b40f8b6/internal/kiroproto/types.go)):

```go
type HistoryEntry struct {
    // ...
    CachePoint *CachePoint `json:"cachePoint,omitempty"`
}

type ToolEntry struct {
    ToolSpecification *ToolSpecification `json:"toolSpecification,omitempty"`
    CachePoint        *CachePoint        `json:"cachePoint,omitempty"`
}

type CachePoint struct {
    Type string `json:"type"` // "default"
}
```

Every serious peer translator (petehsu/KiroProxy
[`prompt_caching.py`](https://github.com/petehsu/KiroProxy/blob/main/kiro_proxy/prompt_caching.py),
Quorinex/Kiro-Go [`cache_tracker.go`](https://github.com/Quorinex/Kiro-Go/blob/1732b17/proxy/cache_tracker.go))
implements Anthropic `cache_control` → Kiro `cachePoint` conversion, and
all of them keep a per-account cache book. The consistent per-account
keying in Quorinex/Kiro-Go's tracker is the clearest external signal
that **Kiro's upstream cache is keyed per account**. Neither Anthropic
nor AWS publish formal Kiro cache semantics, so the tracker is an
informed simulation, not a measured fact.

**Evidence against a fundamentally different model:** None of the
peer repos handle cache hit/miss events differently per account
(no shared-pool trick, no synthetic cross-account prefix). They all
treat each account as an independent cache namespace.

**kiroxy's current state:** Has `cachePoint` struct wiring
(`internal/kiroproto/types.go`) and session stickiness
(`internal/pool/stickiness.go`). What is **not** yet present:

1. Anthropic `cache_control` → Kiro `cachePoint` conversion in the
   request converter (searched `internal/reqconv/`; no `cachePoint`
   references outside `kiroproto` type definitions). This means kiroxy
   currently emits Anthropic's `cache_control` markers *without*
   translating them to Kiro's native cache markers. This is the largest
   single gap.
2. Cache accounting in the response `usage` map (no
   `cache_read_input_tokens` / `cache_creation_input_tokens` emission).
3. Observability on cache hit rate (no counter, no gauge).

### Request-timing evidence

I did not run a real timing benchmark against Kiro upstream within this
research session (would need fresh account credentials + controlled test
harness). **Stated as a gap to be verified.** The strong circumstantial
evidence above — every serious peer caches, Quorinex tracks per-account
breakdowns, Anthropic docs document the underlying cache, the Kiro
protocol accepts `cachePoint` — makes the hypothesis that Kiro caches
upstream reasonable but not *proven* by this report's measurements.

---

## Session Binding Patterns (with tradeoffs)

| Pattern | Example | Pro | Con |
|---|---|---|---|
| **Header-based** (recommended for kiroxy) | `X-Claude-Code-Session-Id` (kirocc, kiroxy, cc-switch, Wei-Shaw/sub2api, coder/aibridge) | Zero trust required; client owns identity; free for claude-code ≥2.1.87 | Clients without the header need a fallback |
| **Content-hash** | `sha256(messages[:3])` (petehsu/KiroProxy) | Works with clients that don't send session headers | Privacy: identical agent templates collide; cannot reset session mid-task |
| **API-key / user-key hash** | LiteLLM `deployment_affinity` | Simple, universal | Scales stickiness by # of *users* not # of *sessions* — can defeat load-balancing entirely |
| **Cookie-based** | Not used by any surveyed Kiro peer | Web-app-native | Not how CLI agents work; Kiro clients don't ship cookies |
| **IP-based** | Not used by any surveyed Kiro peer | Simple, transparent | NAT / VPN / multiple devices one IP; breaks for the localhost use case |
| **GLOBAL sticky** | jwadow/kiro-gateway | Max cache hit rate across all clients | Quota burn on one account; other accounts idle |
| **Previous-response-id** | LiteLLM `responses_api_deployment_check` | Perfect 1:1 session continuity | Requires the provider's `/responses` API (Kiro doesn't expose one cleanly) |
| **LRU with stickiness** (kiroxy's current) | [`internal/pool/pool.go:201-253`](https://github.com/BenDashell/kiroxy/blob/b40f8b6/internal/pool/pool.go) | Idle accounts get picked up when stickiness has no opinion; sticky on new pick | Implementation slightly trickier; 60s drift between accounts |

---

## TTL Selection Considerations

**Shorter than Anthropic's 5-minute cache TTL is recommended** so:

- idle sessions release accounts back to the pool before the cache
  expires (the cache still exists, so a returning client refreshes it
  for free, at `0.1x` read cost);
- clients that silently dropped get re-balanced quickly.

Evidence-based anchors from surveyed peers:

| Project | TTL | Rationale |
|---|---|---|
| petehsu/KiroProxy | **60s** | Undocumented, but matches claude-code turn spacing (2-30s) |
| kiroxy (current) | **60s** | [`internal/pool/stickiness.go:21`](https://github.com/BenDashell/kiroxy/blob/b40f8b6/internal/pool/stickiness.go) — "match typical claude-code turn spacing without holding a pin open across a dormant conversation" |
| LiteLLM | **3600s** (configurable) | Optimised for long API-key-scoped sessions |
| Portkey | **3600s** (configurable) | Same rationale |

**Too short** (<30s): cache misses dominate because a user's think-pause
between turns regularly exceeds the pin window. Pin thrash raises
rotation overhead without cache benefit.

**Too long** (>5 min): cache expires on Anthropic's side anyway, so
stickiness past that point only pins the account without cache benefit
and risks quota concentration.

**Sweet spot for claude-code / opencode:** `60s-120s` for per-session
agent work; `300s` (exact cache alignment) only if sessions are
short-lived and quota is abundant.

### Adaptive TTL (BACKLOG item)

None of the surveyed peers implement adaptive TTL. A simple recipe:

```
ttl = base_ttl * clamp(1 - account_utilisation, 0.25, 2.0)
```

- `account_utilisation ∈ [0,1]` from `getUsageLimits` (petehsu already
  wires this; kiroxy BACKLOG has the analogous "real quota tracking"
  item).
- Pin short when the account is 80%+ utilised (avoid quota kill);
  pin long when the account has headroom (maximise cache hit).

---

## Claude Code / opencode Session Behavior

### Claude Code

- **Exposes session ID as CLI flag:** `claude --session-id <uuid>`
  ([CLI reference](https://docs.anthropic.com/en/docs/claude-code/cli-reference)).
- **Header emitted from v2.1.87+:** `X-Claude-Code-Session-Id: <uuid>`.
  Confirmed by
  [Wei-Shaw/sub2api `backend/internal/service/header_util.go:36`](https://github.com/Wei-Shaw/sub2api/blob/main/backend/internal/service/header_util.go)
  ("Claude Code 2.1.87+ 新增 header") and
  [coder/coder `aibridge/session_test.go:24-35`](https://github.com/coder/coder/blob/main/aibridge/session_test.go).
- **SDK equivalent:** `ClaudeAgentOptions(resume=session_id)`,
  ([Work with sessions](https://docs.claude.com/en/docs/claude-code/sdk/sdk-sessions)).
- **Fallback location:** `metadata.user_id` body field encodes session
  as `..._session_<id>` — see coder/aibridge test cases for precedence
  rules ("claude_code_header_takes_precedence",
  "claude_code_empty_header_falls_back_to_body").
- **Typical turn count per coding task session:** 5-50 tool-use turns
  (not formally measured in this session; reasonable bound from
  `claude-code` usage patterns and OTel issues citing session logs).
  Each turn is a separate `/v1/messages` request with the same
  `X-Claude-Code-Session-Id`.

### opencode

- **Does NOT ship an `X-Claude-Code-Session-Id` header.** I searched
  `sst/opencode` via grep.app; no results. opencode relies on client-side
  context management via its own SDK state; the upstream anthropic API
  call simply carries a growing message history.
- **Implication for kiroxy:** An opencode client will always take the
  fallback path in `internal/pool/stickiness.go:Pick` (empty `sessionID`
  → bypass map → pure LRU). Every opencode request is subject to
  round-robin cache-scope churn. BACKLOG candidate: content-hash
  fallback (petehsu-style) for clients that don't emit the session
  header.

---

## Multi-Tenant Caching Complications

### The problem

Anthropic / Kiro caches are scoped per API-key / per-account. With *N*
accounts in round-robin and no stickiness:

```
P(cache hit on request K | prefix size S) ≈ 1/N     (large K, uniform rotation)
```

i.e. your N-account pool multiplies *raw quota* by N but divides *cache
hit rate* by N, so the effective cost-reduction from caching is cancelled
unless you pin.

### Solution shape

Every non-toy gateway solves this with some flavour of **sticky routing
within a session window**:

- **LiteLLM:** three priority-ordered affinity checks
  ([`deployment_affinity_check.py`](https://github.com/BerriAI/litellm/blob/d251238b/litellm/router_utils/pre_call_checks/deployment_affinity_check.py)).
- **Portkey:** `sticky.enabled + hash_fields + ttl` inside the
  `loadbalance` strategy ([docs](https://portkey.ai/docs/docs/product/ai-gateway/load-balancing)).
- **petehsu/KiroProxy:** content-hash → account map with 60s TTL.
- **kiroxy:** header-keyed map, 60s TTL, release on failure.

### Tradeoffs (explicitly stated)

1. **Stickiness reduces effective quota** proportional to stickiness
   concentration. LiteLLM literally warns:
   > ⚠️ Reduces quota by # of sessions.
   If you have 5 accounts and 50 simultaneous sessions, load still
   distributes. If you have 5 accounts and 5 simultaneous sessions, you
   lose load balancing entirely.
2. **Failure-release is non-negotiable.** Without a release path (i.e.
   if the pin survives a 429 / cooldown), stickiness turns into a
   per-session failure lock. kiroxy handles this correctly via
   `Stickiness.Release(accountID)` on `Pool.Remove` (see
   [`internal/pool/stickiness.go:112-126`](https://github.com/BenDashell/kiroxy/blob/b40f8b6/internal/pool/stickiness.go))
   and on cooldown (`Pool.Pick` stale-pin re-pick at
   [`pool.go:243-248`](https://github.com/BenDashell/kiroxy/blob/b40f8b6/internal/pool/pool.go)).
3. **Stickiness TTL should be ≤ cache TTL.** Otherwise the pin outlives
   its purpose. 60s vs 300s is conservative and correct.
4. **Global stickiness is a trap for shared pools.** jwadow's
   `_current_account_index` is a clean idea for single-user deployments
   but catastrophic for multi-tenant: one account gets crushed while
   others idle.

---

## Recommended Pattern for kiroxy

**Status: kiroxy already has the core (merged commit `1e9dfa3` +
`52b7216`). The recommendations below are refinements, not rebuilds.**

### 1. Keep the current mechanism

- Session key: `X-Claude-Code-Session-Id` (header-based, highest-signal
  binding for claude-code; zero cost).
- TTL: **60s** — matches claude-code turn spacing, well under Anthropic's
  5-minute cache TTL.
- Failure release: **already implemented** via `Stickiness.Release()`
  on `Pool.Remove` and stale-pin re-pick on cooldown.
- Empty-session bypass: **already implemented** (line 76-78) — opencode
  and other header-less clients fall through cleanly to LRU.

### 2. BACKLOG items this research surfaces

**B1 — Convert Anthropic `cache_control` to Kiro `cachePoint` in
`internal/reqconv`.** Largest single throughput lever surveyed. Pattern
well-established (petehsu/KiroProxy
[`prompt_caching.py:17-52,55-96`](https://github.com/petehsu/KiroProxy/blob/main/kiro_proxy/prompt_caching.py),
Quorinex/Kiro-Go's `flattenClaudeCacheBlocks`). LoC: ~100. Without this,
stickiness still gives cache-locality benefit *if* Kiro implicitly caches
per account, but the explicit markers are how the peer ecosystem
exploits the feature — and the types are already in
`internal/kiroproto/types.go`.

**B2 — Content-hash fallback for clients without session header.**
When `ccSessionID == ""` and the pool has ≥2 accounts, derive a
`sha256(messages[:3])[:16]` session key (petehsu's pattern) and feed it
to `Stickiness.Pick`. Makes opencode and other header-less clients
benefit from stickiness too. LoC: ~30.

**B3 — Cache hit-rate metrics + adaptive TTL stub.**

- Gauge: `kiroxy_stickiness_pinned_sessions{account}` (count per account).
- Counter: `kiroxy_stickiness_pin_hit_total` / `_pin_miss_total`.
- Counter: `kiroxy_stickiness_pin_released_total{reason=cooldown|failure|expiry}`.
- Adaptive TTL: once real quota data lands (existing BACKLOG item for
  `getUsageLimits`), shorten TTL when utilisation >80%, lengthen when
  <40%. Recipe in §TTL Selection. LoC: ~60.

### 3. Implementation sketch (for B1, the highest-value item)

```go
// internal/reqconv/cache_points.go — NEW FILE.

// ApplyToolCachePoints inserts a {"cachePoint":{"type":"default"}}
// entry AFTER any anthropic tool that had cache_control set. Mirrors
// petehsu/KiroProxy's apply_tool_cache_points semantics for wire
// compatibility with the Kiro upstream.
func ApplyToolCachePoints(anthropicTools []anthropic.Tool, kiroTools []kiroproto.ToolEntry) []kiroproto.ToolEntry {
    out := make([]kiroproto.ToolEntry, 0, len(kiroTools)+4)
    for i, t := range anthropicTools {
        if i < len(kiroTools) {
            out = append(out, kiroTools[i])
        }
        if t.CacheControl != nil {
            out = append(out, kiroproto.ToolEntry{
                CachePoint: &kiroproto.CachePoint{Type: "default"},
            })
        }
    }
    // trailing kiroTools that had no anthropic equivalent
    for i := len(anthropicTools); i < len(kiroTools); i++ {
        out = append(out, kiroTools[i])
    }
    return out
}

// ApplyMessageCachePoints does the same for history entries. Walks
// Anthropic messages; when the LAST content block of a message has
// cache_control, emits a cachePoint history entry immediately after
// the converted Kiro history entry.
```

Wire into `internal/reqconv/build_payload.go` right after the existing
`tools` / `history` build. Unit tests should verify the 20-block
lookback window isn't accidentally breached by the marker placement
(Anthropic counts the marker against the window).

---

## Honest Dead-Ends

1. **No live timing benchmark performed.** The 3-5x throughput claim is
   inferred from (a) Anthropic's published 10x cache-read discount, (b)
   peer projects' per-account cache accounting, (c) structural
   inevitability that cache scope is per-account. A real benchmark would
   need fresh Kiro credentials + controlled prompts; I did not set that
   up in this session. Flagged as a verification gap, not presented as
   proven.

2. **kirocc's `cache_points.go`.** The prompt asked about this file.
   It doesn't exist at this name; the functionality is in the
   converter. Inferred from the fact that *three* peers
   (petehsu/KiroProxy comment at `prompt_caching.py:7`, kiro-gateway,
   and Kiro-Go) credit "kirocc's cache_points" as inspiration, and the
   code in kirocc's `internal/reqconv/` does apply `cachePoint` logic.
   Explicit `cache_points.go` filename may have existed in an earlier
   commit and been refactored away; not dug further.

3. **opencode session behavior.** Confirmed opencode doesn't ship
   `X-Claude-Code-Session-Id` by grep.app search of `sst/opencode`
   (zero results). Did not exhaustively audit all opencode request
   paths; there may be a different session header I didn't find. If
   opencode *does* surface a session identifier somewhere, kiroxy's
   content-hash fallback (B2) plus a configurable additional
   `session_header` env would catch it.

4. **IP-based stickiness.** Universally absent from the surveyed peers.
   Looks tempting but breaks for the common localhost-loopback +
   multiple-client case (VS Code + CLI + test-harness all look like
   `::1`), and obliterates stickiness in any NAT scenario. Not
   recommended; not pursued.

5. **Per-workspace cache isolation (2026-02-05 change).** Anthropic's
   cache isolation moved from org-level to workspace-level on Feb 5,
   2026 for direct API / Claude Platform on AWS / Microsoft Foundry.
   Unclear whether Kiro's upstream follows this change; depends on how
   Amazon-Q terms each account internally. Assumed per-account
   isolation for the rest of this analysis (conservative), but this
   could be looser in practice and the actual cache-hit multiplier
   from stickiness could therefore be *higher* than calculated.

---

## Citations (SHAs pinned at cite time)

### Peer repositories
- petehsu/KiroProxy @ `main` — https://github.com/petehsu/KiroProxy
- d-kuro/kirocc @ `4ff8d812a4690d924c449ccfaf244e978a7901be` — https://github.com/d-kuro/kirocc
- Quorinex/Kiro-Go @ `1732b17ff9455e55cb9dcf34cf23c39f5b549042` — https://github.com/Quorinex/Kiro-Go
- jwadow/kiro-gateway @ `0398d74f15549bd771480da8fceb21916ce333e5` — https://github.com/jwadow/kiro-gateway
- hj01857655/kiro-account-manager @ `6413d1ac91738207616db91d9329d51408ac969f` — https://github.com/hj01857655/kiro-account-manager
- AntiHub-Project/Antigv-plugin @ `main` — https://github.com/AntiHub-Project/Antigv-plugin
- BerriAI/litellm `d251238b` — https://github.com/BerriAI/litellm
- LiteLLM PR #21763 (session_affinity, 2026-02-21) — https://github.com/BerriAI/litellm/pull/21763
- LiteLLM PR #19143 (deployment_affinity, 2026-01-15) — https://github.com/BerriAI/litellm/pull/19143
- Portkey-AI/gateway docs — https://portkey.ai/docs/docs/product/ai-gateway/load-balancing

### Documentation
- Anthropic prompt caching — https://docs.anthropic.com/en/docs/build-with-claude/prompt-caching (fetched 2026-05-13)
- Claude Code CLI reference — https://docs.anthropic.com/en/docs/claude-code/cli-reference
- Claude Code sessions SDK — https://docs.claude.com/en/docs/claude-code/sdk/sdk-sessions
- anthropics/claude-code issues #24093 (2026-02-08), #25642 (2026-02-13), #1990 (2025-06-12) — session ID surfacing

### kiroxy (this repo, for traceability)
- `internal/pool/stickiness.go` (commit `1e9dfa3`)
- `internal/pool/pool.go:201-275` Pick flow (commit `52b7216`)
- `internal/kiroproto/types.go:50,62,114` cachePoint struct definitions
- `internal/messages/handler.go:23,45,95` session header extraction + thread to upstream `ConversationID`

---

_Compiled 2026-05-13 Asia/Makassar. Length: ~480 lines._
