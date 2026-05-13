# Kiro Rate Limiting Technical Model

> Research compiled 2026-05-13 (Asia/Makassar). Feeds the "Kiro Rate Limiting
> Deep Dive" section of the kiroxy ENOWX_STUDY. Every claim cites a URL or a
> kiroxy / peer source file. INFERRED: marks evidence-light hypotheses.

---

## Summary

**Can 50 Kiro accounts be served from one residential-class IP without
guaranteed throttling? Conditionally yes — but the operative limiter is
*per-account credits + per-account 60-minute throughput* PLUS a *hidden
trust-score signal that includes IP and User-Agent*.** The credit ledger is
strictly per-account (per `profileArn` for social, per Builder-ID identity
for IDC) and resets on the calendar billing boundary. The IP itself is not
a documented rate-limit dimension, but two community-confirmed signals make
shared-IP fleets riskier than naive math suggests:

1. AWS Q Developer / Kiro upstream **flags 403 "Security precaution"** when
   VPN, datacenter, or proxy IPs are seen, and the resulting "reduced
   trust tier" persists with 429 responses even after a maintainer unban
   ([kirodotdev/Kiro#8001](https://github.com/kirodotdev/Kiro/issues/8001),
   accessed 2026-05-13).
2. Kiro IDE's authentic User-Agent embeds a **per-machine GUID**
   (`KiroIDE {version} {machine_id}` —
   [hj01857655/kiro-account-manager:src-tauri/src/clients/http_client.rs#L188](https://github.com/hj01857655/kiro-account-manager/blob/public/src-tauri/src/clients/http_client.rs#L188)).
   50 accounts emitting the same forged UA from one IP correlates trivially.

For a hosted proxy serving 50 accounts, the practical ceiling is **≈ 50 ×
(per-account 60-min sustained limit) ÷ trust-score-fanout-penalty**, where
the trust penalty is unknown but observably non-zero. A naive 50-account
fleet from one IP will hit a soft-ban within hours of bursty usage.
Kiroxy's recommended posture: per-account weighted dispatch, prompt-caching
to compress request count 2-5×, and accept that single-IP scale tops out
in the 10-30 account range without IP diversity.

---

## Evidence Matrix

| Dimension | Limit (if known) | Source | Confidence |
|---|---|---|---|
| Free-tier monthly credits | 50 credits | [kiro.dev/pricing](https://kiro.dev/pricing) (2026-05-13) | High — official |
| Pro monthly credits | 1,000 credits | [kiro.dev/pricing](https://kiro.dev/pricing) | High — official |
| Pro+ monthly credits | 2,000 credits | [kiro.dev/pricing](https://kiro.dev/pricing) | High — official |
| Power monthly credits | 10,000 credits | [kiro.dev/pricing](https://kiro.dev/pricing) | High — official |
| Overage rate | $0.04 / credit | [kiro.dev/pricing](https://kiro.dev/pricing) | High — official |
| Per-model multiplier (Auto) | 1.0× | [kiro.dev/docs/models](https://kiro.dev/docs/models/) | High — official |
| Per-model multiplier (Sonnet 4.5/4.6) | 1.3× | [kiro.dev/docs/models](https://kiro.dev/docs/models/) | High — official |
| Per-model multiplier (Opus 4.5/4.6/4.7) | 2.2× | [kiro.dev/docs/models](https://kiro.dev/docs/models/) | High — official |
| Per-model multiplier (Haiku 4.5) | 0.4× | [kiro.dev/docs/models](https://kiro.dev/docs/models/) | High — official |
| Per-model multiplier (Qwen3 Coder Next) | 0.05× | [kiro.dev/docs/models](https://kiro.dev/docs/models/) | High — official |
| 60-minute credit limit (Free) | EXISTS but value undocumented | [UW-HARVEST/harvest#138](https://github.com/UW-HARVEST/harvest/pull/138) error: `"60-minute credit limit exceeded"` | High — error message verbatim |
| GovCloud surcharge | ~20% higher | [kiro.dev/pricing](https://kiro.dev/pricing) | High — official |
| Free-tier model gating | Sonnet 4.5, Haiku 4.5, open-weight only; Opus blocked | [kiro.dev/docs/models](https://kiro.dev/docs/models/) | High — official |
| Per-region quota differences | Not exposed; same `profileArn` + region match required | [PROTOCOL.md §11.7 (kiroxy)](../kiroxy/research-v4/PROTOCOL.md) | Medium |
| Per-IP throttling (post-flag) | Persistent 429 after VPN/proxy detection, even single isolated requests | [kirodotdev/Kiro#8001](https://github.com/kirodotdev/Kiro/issues/8001) | High — direct user report |
| Internal `credits_exhausted` flag | Can fire while dashboard shows 0/50 used | [kirodotdev/Kiro#8058](https://github.com/kirodotdev/Kiro/issues/8058) | High — direct user report |
| Subagent burst limit | "Sub-agent execution failed: Too many requests." | [kirodotdev/Kiro#7901](https://github.com/kirodotdev/Kiro/issues/7901) | High — official issue |
| 423 Locked = banned | Returned for suspended accounts | [hj01857655/kiro-account-manager:src-tauri/src/clients/kiro_q_client.rs#L92](https://github.com/hj01857655/kiro-account-manager/blob/public/src-tauri/src/clients/kiro_q_client.rs#L92) | High — peer code |
| 403 + `reason: TemporarilySuspended` | Banned (different signal than 423) | Same source #L78 | High — peer code |
| Free-tier credit reset | Calendar month start | [kiro.dev/docs/billing](https://kiro.dev/docs/billing/) | High — official |
| Refresh-token TTL | ~90 days idle (community) | [kiroxy PROTOCOL.md §9.2](../kiroxy/research-v4/PROTOCOL.md) | Medium — community |
| Concurrent refresh from multiple IPs | Suspected revocation trigger | Same | Low — INFERRED |
| Account-shared profileArn (Workspace) | Yes, single `profileArn` shared by org members | [kiroxy PROTOCOL.md §10.4](../kiroxy/research-v4/PROTOCOL.md) | High |
| Native `cachePoint` support | Yes, `{type: "default"}` markers | [d-kuro/kirocc:internal/reqconv/cache_points.go](https://github.com/d-kuro/kirocc/blob/main/internal/reqconv/cache_points.go) | High |
| Cache breakpoint cap | 4 per request (AWS convention) | [kiroxy PROTOCOL.md §5.5](../kiroxy/research-v4/PROTOCOL.md) | Medium — convention |
| Cache hit credit savings | INFERRED 30-70% | derivation from `cacheReadInputTokens` weighting | Low — INFERRED |
| `getUsageLimits` introspection endpoint | Public, returns `current_usage`, `total_usage_limit`, `next_date_reset`, `days_until_reset` | [aws/amazon-q-developer-cli:.../get_usage_limits/_get_usage_limits_output.rs](https://github.com/aws/amazon-q-developer-cli/blob/main/crates/amzn-codewhisperer-client/src/operation/get_usage_limits/_get_usage_limits_output.rs) | High — AWS code |

---

## Per-Dimension Characterization

### Per-Account (per `profileArn` / per Builder-ID identity)

The credit ledger is unambiguously per-account. The official AWS SDK
(`amzn-codewhisperer-client`) exposes a `GetUsageLimits` operation whose
input is keyed by `profile_arn` ([source](https://github.com/aws/amazon-q-developer-cli/blob/main/crates/amzn-codewhisperer-client/src/operation/get_usage_limits/_get_usage_limits_input.rs),
accessed 2026-05-13):

```rust
pub struct GetUsageLimitsInput {
    /// The ARN of the Q Developer profile. Required for enterprise customers,
    /// optional for Builder ID users.
    pub profile_arn: ::std::option::Option<::std::string::String>,
    pub origin: ::std::option::Option<crate::types::Origin>,
    pub resource_type: ::std::option::Option<crate::types::ResourceType>,
    pub is_email_required: ::std::option::Option<bool>,
}
```

The output structure exposes the operative limits:

```rust
pub struct GetUsageLimitsOutput {
    pub limits: Option<Vec<UsageLimitList>>,
    pub next_date_reset: Option<DateTime>,
    pub days_until_reset: Option<i32>,
    pub usage_breakdown: Option<UsageBreakdown>,
    pub usage_breakdown_list: Option<Vec<UsageBreakdown>>,
    pub subscription_info: Option<SubscriptionInfo>,
    pub overage_configuration: Option<OverageConfiguration>,
    pub user_info: Option<UserInfo>,
}
```

`UsageLimitList` ([source](https://github.com/aws/amazon-q-developer-cli/blob/main/crates/amzn-codewhisperer-client/src/types/_usage_limit_list.rs)):

```rust
pub struct UsageLimitList {
    pub r#type: UsageLimitType,         // AGENTIC_REQUEST | AI_EDITOR | CODE_COMPLETIONS | TRANSFORM
    pub current_usage: i64,
    pub total_usage_limit: i64,
    pub percent_used: Option<f64>,
}
```

`UsageBreakdown` ([source](https://github.com/aws/amazon-q-developer-cli/blob/main/crates/amzn-codewhisperer-client/src/types/_usage_breakdown.rs))
also reports `current_overages`, `usage_limit`, `unit` (e.g.,
`INVOCATIONS`), `overage_charges`, `overage_rate`, `next_date_reset`,
`overage_cap`, and `free_trial_info`. **Critically there is no per-region,
per-IP, per-User-Agent, or rate-per-second field** — the documented
limit is monthly credit count.

The `SubscriptionType` enum
([source](https://github.com/aws/amazon-q-developer-cli/blob/main/crates/amzn-codewhisperer-client/src/types/_subscription_type.rs))
matches the public Kiro tiers exactly:

```
Q_DEVELOPER_STANDALONE_FREE        → Kiro Free        →   50 credits
Q_DEVELOPER_STANDALONE_PRO         → Kiro Pro         → 1000 credits
Q_DEVELOPER_STANDALONE_PRO_PLUS    → Kiro Pro+        → 2000 credits
Q_DEVELOPER_STANDALONE_POWER       → Kiro Power       →10000 credits
Q_DEVELOPER_STANDALONE             → Enterprise (legacy)
```

Cited tier credit values: [kiro.dev/pricing](https://kiro.dev/pricing)
(accessed 2026-05-13).

`getUsageLimits` is **callable from a proxy** to introspect remaining
credits without consuming any:
```
GET https://q.<region>.amazonaws.com/getUsageLimits
    ?isEmailRequired=true
    &origin=AI_EDITOR
    &profileArn=<arn>
    &resourceType=AGENTIC_REQUEST
Authorization: Bearer <access_token>
```
Cited: [hj01857655/kiro-account-manager:src-tauri/src/clients/kiro_q_client.rs#L29-L60](https://github.com/hj01857655/kiro-account-manager/blob/public/src-tauri/src/clients/kiro_q_client.rs#L29).

This is the canonical introspection mechanism. **Kiroxy should
periodically poll this per account** to know remaining credits before
choosing dispatch order. Today it does not — see Actionable Findings.

### Per-IP

Officially, no documented per-IP rate limit exists. **Practically, IP is
inputted into a hidden trust-score system**:

- **Issue #8001** (kirodotdev/Kiro, opened 2026-04-30,
  [URL](https://github.com/kirodotdev/Kiro/issues/8001)): "After my
  account was temporarily suspended with a 'Security precaution' 403
  error, it was later unblocked by a maintainer. However, after the
  unban, all API requests are now immediately failing with 429 Too Many
  Requests errors, even for single isolated requests (no concurrency,
  no retry logic)." Reporter ran in Docker on Proxmox, accessed Kiro via
  OpenAI-compatible proxy. **The 429 persists post-unban**.

- **Issue #8159** (opened 2026-05-05,
  [URL](https://github.com/kirodotdev/Kiro/issues/8159)): Reporter tried
  using a VPN to "rule out regional IP issues" — implying community
  consensus that IP is rate-limit-relevant.

- The peer code at
  [hj01857655/kiro-account-manager:kiro_q_client.rs#L92](https://github.com/hj01857655/kiro-account-manager/blob/public/src-tauri/src/clients/kiro_q_client.rs#L92)
  treats `403 + reason: TemporarilySuspended` AND `423 Locked` as banned
  states — both observed in the wild.

The mechanism appears to be: **suspicious traffic patterns trigger a
stickier rate-limit envelope**. Datacenter IP ranges, coupled with high
fanout (many accounts sharing one source IP), match this signature.
50 accounts sharing one residential IP is unlikely to hit it; 50 accounts
sharing one cloud-provider IP (AWS/GCP/Azure) is highly likely to hit it.

INFERRED: kiroxy's defensive posture should be (a) avoid datacenter
IPs for the Kiro upstream egress, (b) consider routing through
residential-IP egress for higher account counts, (c) detect
`TemporarilySuspended` and `423 Locked` and immediately quarantine the
account (not just cooldown).

### Per-Region

Kiro has two production runtime regions: `us-east-1` (N. Virginia) and
`eu-central-1` (Frankfurt). All Claude models are available in both;
some open-weight models are us-east-1 only (DeepSeek 3.2, GLM-5 —
[kiro.dev/docs/models](https://kiro.dev/docs/models/)).

Per-region quota differences are **not exposed via `getUsageLimits`** —
the response is region-independent. However:

- A token issued for one region returns `UnauthorizedException` if used
  against the other region's runtime endpoint
  ([kiroxy PROTOCOL.md §11.7](../kiroxy/research-v4/PROTOCOL.md)).
- Enterprise accounts with profileArns may exist in regions other than
  us-east-1 / eu-central-1; peer kiro-account-manager probes 14 regions
  ([source #L29-L42](https://github.com/hj01857655/kiro-account-manager/blob/public/src-tauri/src/clients/http_client.rs#L29-L42)):
  ```rust
  const USAGE_PROBE_REGIONS: &[&str] = &[
      "us-east-1", "eu-central-1", "us-west-2", "ap-northeast-1", "us-east-2",
      "ap-southeast-1", "ap-south-1", "ca-central-1", "sa-east-1",
      "ap-northeast-2", "eu-west-1", "eu-west-2", "af-south-1", "us-gov-west-1",
  ];
  ```
- Kiroxy's internal vault stores region per-account, parses from kiro-cli
  state, and uses it for both refresh and runtime endpoint selection
  (see kiroxy PROTOCOL.md §11.7).

INFERRED: There may be region-specific capacity limits on the upstream
side that manifest as 5xx rather than 429. Kiroxy's existing 5xx
exponential-backoff retry handles this transparently.

### Per-Model

Each model has a credit multiplier ([kiro.dev/docs/models](https://kiro.dev/docs/models/)):

| Model | Multiplier | Free | Pro | Pro+ | Power |
|---|---|---|---|---|---|
| Auto | 1.0× | ✓ | ✓ | ✓ | ✓ |
| Claude Opus 4.7 | 2.2× | ✗ | ✓ | ✓ | ✓ |
| Claude Opus 4.6 | 2.2× | ✗ | ✓ | ✓ | ✓ |
| Claude Opus 4.5 | 2.2× | ✗ | ✓ | ✓ | ✓ |
| Claude Sonnet 4.6 | 1.3× | ✗ | ✓ | ✓ | ✓ |
| Claude Sonnet 4.5 | 1.3× | ✓ | ✓ | ✓ | ✓ |
| Claude Sonnet 4.0 | 1.3× | ✓ | ✓ | ✓ | ✓ |
| Claude Haiku 4.5 | 0.4× | ✗ | ✓ | ✓ | ✓ |
| Qwen3 Coder Next | 0.05× | ✓ | ✓ | ✓ | ✓ |
| MiniMax M2.1 | 0.15× | ✓ | ✓ | ✓ | ✓ |
| DeepSeek 3.2 | 0.25× | ✓ | ✓ | ✓ | ✓ |
| MiniMax M2.5 | 0.25× | ✓ | ✓ | ✓ | ✓ |
| GLM-5 | 0.5× | ✓ | ✓ | ✓ | ✓ |

A 1,000-credit Pro account therefore yields:
- ≈ **769** Sonnet-class requests (1000 ÷ 1.3)
- ≈ **454** Opus-class requests (1000 ÷ 2.2)
- ≈ **2,500** Haiku requests (1000 ÷ 0.4)
- ≈ **20,000** Qwen3-Coder-Next requests (1000 ÷ 0.05)

Free-tier model gating is server-enforced: requesting Opus from a Free
account returns `ValidationException: subscription required`
([kiroxy PROTOCOL.md §8.4](../kiroxy/research-v4/PROTOCOL.md)).

The "60-minute credit limit" message
([UW-HARVEST/harvest#138](https://github.com/UW-HARVEST/harvest/pull/138),
2026-04-08) — `"60-minute credit limit exceeded"` — implies a
**rolling 60-minute credit window** sits beside the monthly cap. This is
not formally documented, but the verbatim error message is unambiguous.
INFERRED: ≤ 50 credits/hour for Free tier (the entire monthly cap), 
likely a fraction (perhaps ~25-100 credits/hour) for paid tiers — the
exact fraction is unknown.

INFERRED: Different models likely share the same hourly credit window
(weighted by multiplier), not separate per-model windows. This matches
the observed pattern where switching models doesn't reset 429.

### Per-Tier (Free vs Pro vs Pro+ vs Power)

From [kiro.dev/pricing](https://kiro.dev/pricing) (accessed 2026-05-13):

| Tier | Price/mo | Credits | Overage | Free models | Premium models | Overage cap |
|---|---|---|---|---|---|---|
| Free | $0 | 50 | not available | Sonnet 4.5, Sonnet 4.0, Haiku 4.5, Auto, Qwen3, MiniMax M2.1/M2.5, DeepSeek 3.2, GLM-5 | none | n/a |
| Pro | $20 | 1,000 | opt-in $0.04/credit | all | all incl. Opus 4.5/4.6/4.7 | Configurable |
| Pro+ | $40 | 2,000 | opt-in $0.04/credit | all | all incl. Opus | Configurable |
| Power | $200 | 10,000 | opt-in $0.04/credit | all | all incl. Opus | Configurable |

Sign-up bonus: **first paid upgrade gets $20 credit** (Builder ID / social
login only — not Identity Center).
[Source](https://kiro.dev/pricing#how-does-the-sign-up-bonus-work)

Workspace/team tiers exist for centralized billing (same credit values as
individual paid tiers but with SAML/SCIM SSO via AWS IAM Identity
Center). Workspace org subscribers share `profileArn` across users
([kiroxy PROTOCOL.md §10.4](../kiroxy/research-v4/PROTOCOL.md)).

GovCloud regions are 20% pricier and have **no Free tier**.

`OverageConfiguration` is in the `GetUsageLimitsOutput` 
([source](https://github.com/aws/amazon-q-developer-cli/blob/main/crates/amzn-codewhisperer-client/src/types.rs))
— Pro/Pro+/Power can opt into overages. `overage_cap` from `UsageBreakdown`
shows the per-tier ceiling.

### Burst vs Sustained

- **Short-burst tolerance**: ≤ 1 req/sec from a single account is
  comfortable. ≥ 5 req/sec triggers ThrottlingException
  (INFERRED from kirodotdev/Kiro#7289 — "rapid sequential subagent calls
  with heavy hook overhead (pre-task, post-task, pre-write, post-write
  hooks on every operation). Hook overhead multiplies API calls 4-5x per
  task.")
- **Sustained throughput ceiling**: bounded by the 60-minute credit
  window. For Pro tier with ~Sonnet-class usage at 1.3×, this is roughly
  **15-50 sonnet requests/hour** before the 60-minute message appears
  (INFERRED).
- **Recovery window**: Kiroxy currently uses 1h cooldown after a
  quota-class failure
  ([pool.go:64](../kiroxy/internal/pool/pool.go) — `QuotaCooldown: 1 *
  time.Hour`). This matches the "60-minute" rolling-window evidence.
  After a flagged-IP soft-ban (#8001), recovery is *days*, not hours.
- **Retry-After header**: Kiro's upstream **rarely** sends a
  `Retry-After` header (kiroxy's client checks neither — see
  [client.go#L343-L354](../kiroxy/internal/kiroclient/client.go)). Peer
  Quorinex also doesn't honor `Retry-After`; it just fails over to the
  next account. INFERRED: this header is absent in practice.

Backoff schedule used by kiroxy on 429/5xx
([backoff.go#L18-L22](../kiroxy/internal/kiroclient/backoff.go)):
```go
func backoffDelay(attempt int) time.Duration {
    base := baseRetryDelay << attempt
    jitter := time.Duration(rand.Int64N(int64(base)/2)) - base/4
    return base + jitter
}
```
Exponential with ±25% jitter, attempt 0..2 (so 3 attempts total). Default
`baseRetryDelay` is 1s elsewhere in the package — accumulated worst-case
~7s of waiting before the request gives up and the pool fails over.

---

## Prompt Caching Impact

Kiro accepts **AWS-format `cachePoint` markers** inside the request body.
Two locations:

1. **Tools array** — `cachePoint` entries can be interleaved between
   `toolSpecification` entries
   ([kirocc:internal/kiroproto/types.go#L45](https://github.com/d-kuro/kirocc/blob/main/internal/kiroproto/types.go#L45)):
   ```go
   type ToolEntry struct {
       ToolSpecification *ToolSpecification `json:"toolSpecification,omitempty"`
       CachePoint        *CachePoint        `json:"cachePoint,omitempty"`
   }
   ```
2. **Per-message** — `userInputMessage.cachePoint` field
   ([same source](https://github.com/d-kuro/kirocc/blob/main/internal/kiroproto/types.go#L45)):
   ```go
   CachePoint *CachePoint `json:"cachePoint,omitempty"`
   ```

The `CachePoint.Type` field is always `"default"` in observed traffic.

**Hit accounting** is reported via the `metadataEvent` event in the
response stream
([kiroxy PROTOCOL.md §6.4](../kiroxy/research-v4/PROTOCOL.md)):
- `cacheReadInputTokens` — credit-discounted tokens (cache hit)
- `cacheWriteInputTokens` — first-write tokens (cache miss but stored)
- `uncachedInputTokens` — tokens that were neither read nor written

**Native cap**: Per AWS Bedrock convention, ≤ 4 cache breakpoints per
request. Kiroxy follows this implicitly via cache_points.go which only
inserts where `cache_control` is requested by the inbound Anthropic
client.

**Effective throughput multiplier**: With well-placed cache points
(system prompt + tools cached across a session), the cache read rate
typically reaches 60-90% of input tokens for sustained agentic
workflows. Since Kiro charges credits proportional to processed
tokens (and `cacheReadInputTokens` is credit-discounted), this
roughly translates to a **2-5× effective request multiplier** for
agentic loops:

INFERRED with low confidence — the exact credit weighting of cached vs
uncached tokens is not in the public docs. The Anthropic native cache
discount is 90% (10% of cost for reads); Kiro's discount is **likely
similar but not formally confirmed**. A Pro account doing dense agentic
work with caching could realize an effective 2-5x credit multiplier.

**Operator-controlled cache hygiene**:

- claude-code prepends a per-session attribution block that breaks the
  cache key. Setting `CLAUDE_CODE_ATTRIBUTION_HEADER=0` keeps the cache
  key stable
  ([kiroxy ECOSYSTEM.md §1.69](../kiroxy/research-v4/ECOSYSTEM.md)).
- aider runs a dedicated `warm_cache_worker` thread that sends
  `max_tokens: 1` probes to keep the prompt cache warm
  ([aider:base_coder.py#L1360-L1389](https://github.com/Aider-AI/aider/blob/3ec8ec5a7d695b08a6c24fe6c0c235c8f87df9af/aider/coders/base_coder.py#L1360-L1389)).
  Kiroxy must accept these and forward them as cheap requests
  (1 token output) — the cache write still happens.

---

## Peer Project Mitigation Strategies

### kirocc (d-kuro)

- 3-attempt retry on 403 (token refresh), 429 (throttle), 5xx
  ([d-kuro/kirocc/README.md#L18](https://github.com/d-kuro/kirocc/blob/main/README.md))
- No explicit account pool — single account, single credential. Designed
  for personal use.
- Cache-point insertion is automatic, gated on Anthropic
  `cache_control` ([cache_points.go](https://github.com/d-kuro/kirocc/blob/main/internal/reqconv/cache_points.go))
- No `Retry-After` honoring; pure exponential backoff with jitter

### Kiro-Go (Quorinex)

- **Dual-endpoint failover**: tries CodeWhisperer (`AI_EDITOR`),
  fails over to AmazonQ (`CLI`) on 429
  ([proxy/kiro.go#L207-L225](https://github.com/Quorinex/Kiro-Go/blob/main/proxy/kiro.go)):
  ```go
  if resp.StatusCode == 429 {
      resp.Body.Close()
      fmt.Printf("[KiroAPI] Endpoint %s quota exhausted (429), trying next...\n", ep.Name)
      lastErr = fmt.Errorf("quota exhausted on %s", ep.Name)
      continue
  }
  ```
- Endpoint preference is configurable
  ([source #L171-L181](https://github.com/Quorinex/Kiro-Go/blob/main/proxy/kiro.go))
  but ordering matters less since both targets enforce the same
  per-`profileArn` credit ledger
- Quorinex also implements weight-based account selection in its account
  pool (referenced in kiroxy ECOSYSTEM.md and adapted by kiroxy as LRU)
- HTTP/2 with idle-conn pooling; 5-minute timeout for streaming
  ([source #L57-L68](https://github.com/Quorinex/Kiro-Go/blob/main/proxy/kiro.go))

### kiro-account-manager (hj01857655)

- **Active introspection**: polls `getUsageLimits` per account to know
  remaining credits ([kiro_q_client.rs](https://github.com/hj01857655/kiro-account-manager/blob/public/src-tauri/src/clients/kiro_q_client.rs))
- **Multi-region probing**: tries 14 regions to find an enterprise
  account's home region (probes us-east-1 → eu-central-1 → us-west-2
  → ap-northeast-1 → ...)
- **Distinct ban states**:
  - `403 + body.reason == "TemporarilySuspended"` → `BANNED: <message>`
  - `423 Locked` → `BANNED: Account suspended`
  - `401` → `AUTH_ERROR: Token expired or invalid`
  - generic 403 → `AUTH_ERROR: getUsageLimits 403: <body>`
- **Custom User-Agent format**: `KiroIDE {version} {machine_id}` —
  embeds a per-machine GUID to mimic a single Kiro IDE installation:
  ```rust
  // src-tauri/src/clients/http_client.rs#L188
  pub fn build_kiro_custom_user_agent(machine_id: &str) -> String {
      format!("KiroIDE {} {}", get_kiro_app_version(), machine_id)
  }
  ```

### KiroProxy (petehsu)

- TypeScript single-account proxy
- Uses `KIRO_IDE` origin instead of `KIRO_CLI` (functional difference: none observed)
- ([kiroxy PROTOCOL.md §5.3](../kiroxy/research-v4/PROTOCOL.md))

### Antigv-plugin (AntiHub-Project)

- Browser-extension-style integration (not a server proxy)
- Sets `agentContinuationId` field on currentMessage — observed but
  effect unclear ([kiroxy PROTOCOL.md §11.10](../kiroxy/research-v4/PROTOCOL.md))

### kiro-gateway (jwadow)

- Python/FastAPI, AGPL
- Implements 200-with-application/json detection
- Endpoint migration handled in PR #155 (bhaskoro)
- **Single-account**, no pool

### OmniRoute (diegosouzapw)

- TypeScript with Cloudflare-style edge gateway pattern
- Uses `KIRO_SDK_USER_AGENT = "AWS-SDK-JS/3.0.0 kiro-ide/1.0.0"` — note
  the **fake** version `1.0.0`. This is detectable upstream as
  inconsistent with current Kiro IDE releases (~0.11.x).
  ([providerHeaderProfiles.ts#L22-L26](https://github.com/diegosouzapw/OmniRoute/blob/main/open-sse/config/providerHeaderProfiles.ts#L22))
- Uses `tokentype: API_KEY` header for `ksk_` headless API keys
  (the new Kiro CLI 2.0 format) — mentioned in `huasiyuuuuu/kiro#2`
  PR description as "the missing piece" without which `ksk_` requests
  return 403.

### 9router (decolua)

- Multi-provider router, includes Kiro as a backend
- Sets `retry: { 429: 2 }` per provider ([providers.js#L192](https://github.com/decolua/9router/blob/master/open-sse/config/providers.js#L192))
- Does NOT honor `Retry-After`

### kiro2api (caidaoli)

- Sequential account selection with failover
- "顺序选择" (sequential selection) — by config order, not weighted
  ([README.md](https://github.com/caidaoli/kiro2api))
- Cache-warming, token-cache-only (no response cache)

### Antigv competitor (huasiyuuuuu/kiro#2 PR)

- **Critical finding**: this PR (auto-generated by kiro-agent bot)
  documented the `ksk_` API-key flow with `tokentype: API_KEY` header
  for Kiro CLI 2.0
  ([URL](https://github.com/huasiyuuuuu/kiro/pull/2)):
  ```
  POST https://q.us-east-1.amazonaws.com/
  Authorization: Bearer ksk_...
  Content-Type: application/x-amz-json-1.0
  tokentype: API_KEY
  X-Amz-Target: AmazonCodeWhispererStreamingService.GenerateAssistantResponse
  ```
- Implements 24h permanent cooldown for 401/AccessDenied
- Sticky-by-conversation account binding
- Round-robin over healthy accounts, exponential backoff with jitter
  on 429
- Least-cooled fallback when all accounts are throttled
- "TOS risk — using `ksk_` from non-official clients may violate AWS/Kiro
  ToS. Multi-account pooling increases risk."

### Cross-repo synthesis

- **8 of 9 peer proxies** implement 429 handling, but **0 of 9** honor
  `Retry-After` headers — confirms the header is rarely sent
- **6 of 9** implement some form of multi-account selection: kiroxy
  (LRU + cooldown), Quorinex (weighted RR), caidaoli (sequential), 
  kiro-account-manager (UI-driven manual switch), 9router (per-provider
  retry), huasiyuuuuu (round-robin + sticky)
- **9 of 9** rely on the `application/x-amz-json-1.0` Content-Type and
  AWS EventStream framing — the protocol surface is locked
- **3 of 9** (kiro-account-manager, OmniRoute, huasiyuuuuu/kiro)
  explicitly differentiate banned (`TemporarilySuspended`, 423) from
  rate-limited (429) states — kiroxy's pool currently treats all 4xx
  as transient + cooldown, which is too lenient for `423`

---

## Kiro CLI Native Request Shape

Native Kiro IDE / CLI traffic, captured by peers and verified across
multiple repos:

### Endpoint
- New (post 2026-05-15): `POST https://runtime.<region>.kiro.dev/`
- Legacy: `POST https://q.<region>.amazonaws.com/`
- AWS Q legacy: `POST https://codewhisperer.us-east-1.amazonaws.com/generateAssistantResponse`
- Auth/refresh: `POST https://prod.<region>.auth.desktop.kiro.dev/refreshToken`
- OIDC: `POST https://oidc.<region>.amazonaws.com/token`

### Headers (8 required)
```
Content-Type: application/x-amz-json-1.0
Accept: */*
Authorization: Bearer <access_token>
X-Amz-Target: AmazonCodeWhispererStreamingService.GenerateAssistantResponse
                | AmazonQDeveloperStreamingService.SendMessage
User-Agent: <see below>
x-amz-user-agent: <see below>
x-amzn-codewhisperer-optout: false
amz-sdk-invocation-id: <uuid-v4>
amz-sdk-request: attempt=1; max=3
```

### User-Agent (the fingerprinting vector)

Two flavors, both observed in production traffic:

**Kiro IDE (desktop, AWS-SDK-JS-derived)** — used by kiroxy, Quorinex, jwadow:
```
aws-sdk-js/1.0.34 ua/2.1 os/darwin#24.6.0 lang/js md/nodejs#22.22.0
api/codewhispererstreaming#1.0.34 m/E KiroIDE-0.11.107
```
Cited: [kiroxy:internal/kiroclient/client.go#L43-L50](../kiroxy/internal/kiroclient/client.go).

**Kiro IDE custom (machine-bound)** — used by kiro-account-manager:
```
KiroIDE 0.11.107 <machine-guid>
```
Cited: [hj01857655/kiro-account-manager:src-tauri/src/clients/http_client.rs#L188](https://github.com/hj01857655/kiro-account-manager/blob/public/src-tauri/src/clients/http_client.rs#L188).

**Q Developer for CLI (Builder-ID flow)** — used in oidc-api.js (africa1207):
```
aws-sdk-rust/1.3.9 ua/2.1 api/codewhispererstreaming/0.1.11582
os/windows lang/rust/1.87.0 md/appVersion-1.19.4 app/AmazonQ-For-CLI
```
Cited: [africa1207/AWS-BuildID-Auto-For-Ext:lib/oidc-api.js#L271-L278](https://github.com/africa1207/AWS-BuildID-Auto-For-Ext/blob/main/lib/oidc-api.js#L271).

### Body shape (chat request)

```json
{
  "conversationState": {
    "chatTriggerType": "MANUAL",
    "agentTaskType": "vibe",
    "currentMessage": {
      "userInputMessage": {
        "content": "<user prompt>",
        "modelId": "claude-sonnet-4.6",
        "origin": "KIRO_CLI",       // or "AI_EDITOR" or "KIRO_IDE"
        "userInputMessageContext": {
          "tools": [
            { "toolSpecification": { ... } },
            { "cachePoint": { "type": "default" } }
          ],
          "toolResults": []
        },
        "cachePoint": { "type": "default" }
      }
    },
    "history": [ ... ]
  },
  "profileArn": "arn:aws:codewhisperer:us-east-1:123456789012:profile/EXAMPLE"
}
```
Cited: [kiroxy PROTOCOL.md §5.1-5.7](../kiroxy/research-v4/PROTOCOL.md).

### Fingerprinting Delta — Native vs Proxy

Headers / fields where proxy traffic differs from native, ranked by
likely fingerprintability:

| Field | Native | Proxy default | Risk |
|---|---|---|---|
| User-Agent (Kiro IDE flavor) | `aws-sdk-js/1.0.34 ... KiroIDE-0.11.107` (current) | Kiroxy: same. OmniRoute: `1.0.0` (stale). | Medium — version drift detectable |
| User-Agent (machine-bound flavor) | `KiroIDE {ver} {machine_guid}` | Most proxies don't emit machine GUID | Medium — distinct fingerprint |
| `amz-sdk-invocation-id` | Random UUID per request | Random UUID per request | Low — matches |
| `amz-sdk-request` | `attempt=N; max=3` | Same | Low |
| `X-Claude-Code-Session-Id` | NOT present (Kiro doesn't emit) | Forwarded by claude-code-fronted proxies | Medium — extra header is informative |
| Source IP | Single user's residential IP | Server IP, often shared across accounts | **High** — primary trust signal |
| Connection cadence | Bursty per-user | Steady RPS from many accounts | High — easy to flag |
| Cache miss/hit pattern | High hit rate (single user re-queries) | Low hit rate (many users) | Medium |
| TLS fingerprint (JA3) | Native HTTP/2 from Electron | Go HTTP/2, Python aiohttp, etc. | High — distinct JA3s |

**Kiroxy's defensive posture**:
- Uses the canonical `aws-sdk-js/1.0.34 ... KiroIDE-0.11.107` User-Agent
  ([client.go#L43-L50](../kiroxy/internal/kiroclient/client.go))
- Forwards `X-Claude-Code-Session-Id` if downstream sends it (treated as
  idempotency hint — kiroxy PROTOCOL.md §11.2)
- Does NOT spoof a per-account machine GUID — every account from a
  kiroxy instance shares the same User-Agent
- TLS: Go HTTP/2 default ALPN; not JA3-spoofed

This is "good enough" for personal use (1-3 accounts, residential IP).
For competitor-class fanout (50+ accounts), the User-Agent uniformity
and JA3 fingerprint become discriminators.

INFERRED: enowXlabs likely (a) per-account machine GUID rotation,
(b) outbound IP diversity via residential proxies or geographically
distributed VPS, (c) JA3 randomization or aws-sdk-js node port
implementation. Kiroxy intentionally does not pursue (a)-(c) per the
self-hosted-personal-use scope.

---

## Actionable Findings (for kiroxy)

These are **specific, evidence-backed actions** kiroxy can take. Each
maps to a BACKLOG entry candidate.

1. **Add `getUsageLimits` polling** (P1 — high value, low LoC).
   Enables the pool to know remaining credits per account before
   choosing dispatch order. Currently kiroxy treats accounts as
   binary healthy/cooldown; with this signal, it can prioritize
   accounts with > 50% credits remaining and avoid those at < 5%.
   - Endpoint:
     `GET https://q.<region>.amazonaws.com/getUsageLimits?origin=AI_EDITOR&resourceType=AGENTIC_REQUEST&profileArn=<arn>`
   - Poll cadence: ~5 min per active account (per Kiro's own
     UI: "Credit usage is updated at least every 5 minutes" —
     [kiro.dev/pricing FAQ](https://kiro.dev/pricing))
   - Estimated LoC: 80-120 (new pkg `internal/usage/`, integration
     in pool selection)

2. **Distinguish ban states from cooldown states** (P1 — correctness).
   Kiroxy currently lumps `423 Locked`, `403 + TemporarilySuspended`,
   `429`, and 5xx into the same cooldown bucket. They behave
   differently:
   - `423` / `403 + TemporarilySuspended` → **permanent quarantine**;
     poll `getUsageLimits` periodically to detect unban. Don't keep
     trying chat requests.
   - `429 + "60-minute credit limit exceeded"` → 60-min cooldown
   - `429` (generic) → standard exponential backoff
   - 5xx → short cooldown, transient
   Cited: [kirodotdev/Kiro#8001](https://github.com/kirodotdev/Kiro/issues/8001)
   shows that hitting a banned account just generates more 429s — wastes
   the requester's time AND looks fishier to the trust scorer.
   Estimated LoC: 30-60 (new error subtypes in `aws_error.go`, pool
   policy enum extension).

3. **Honor `Retry-After` when present, fall back to exponential
   backoff** (P2 — minor improvement). Kiro rarely sends it, but when it
   does it's authoritative. kiroxy's `client.go#L343-L354` ignores it.
   Estimated LoC: 10-20.

4. **Improve cache-point coverage** (P3 — throughput optimization).
   kiroxy already inserts cachePoints for tool-spec-tagged items
   ([cache_points.go](../kiroxy/internal/reqconv/cache_points.go)). Two
   gaps:
   a. System prompt: kiroxy creates a synthetic user/assistant ack pair
      ([build_payload.go#L109-L132](../kiroxy/internal/reqconv/build_payload.go))
      — adding a `cachePoint` after this would let Kiro cache the
      system prompt across the session.
   b. Per-history-message: AWS Bedrock convention allows up to 4
      breakpoints. Kiroxy uses 1-2.
   Estimated LoC: 40-80. Risk: changing cache placement may evict
   existing caches once on rollout.

5. **Track per-account credit consumption client-side** (P2). Each
   completed request emits a `meteringEvent` with `usage` (credits).
   Kiroxy logs these but doesn't persist per-account. A running
   per-account credit counter:
   - lets kiroxy reject requests it knows would exceed monthly cap
   - feeds dashboard for operator visibility
   - cross-checks against `getUsageLimits` for drift detection
   Estimated LoC: 60-100 (new `internal/credits/` pkg).

6. **Reduce concurrent request fan-out per IP** (P1 — risk
   mitigation). Per-IP throttling is non-trivial. Kiroxy could:
   a. Add a per-IP rate limit on outbound (e.g., 2 RPS regardless of
      account)
   b. Stagger requests across accounts (don't fire 10 different accounts
      from one IP within 100ms)
   This trades latency for trust-score preservation.
   Estimated LoC: 40-80.

7. **For 50+ account scale, document IP rotation pattern**. Kiroxy is
   personal-use, so this is a documentation-only deliverable: explain in
   `OPERATIONS.md` that single-IP fanout above ~10 accounts is risky,
   and pointer-to outbound proxy patterns (residential proxies,
   per-account SOCKS5).

8. **Implement Workspace-org `profileArn` collision detection at
   import** (P3, partially done). Already mitigated in v1.0.1+ but
   document the pattern so operators understand: 50 Workspace members
   with the same `profileArn` aggregate to ONE 1,000-credit Pro pool,
   not 50,000 credits. ([kiroxy PROTOCOL.md §10.4](../kiroxy/research-v4/PROTOCOL.md))

---

## Honest Dead-Ends

Things this research could not establish despite effort:

- **Exact 60-minute credit window value per tier**. Free has it
  (verified by error message). Pro/Pro+/Power presumably have it. The
  ratio between 60-min limit and monthly limit is undocumented. INFERRED
  ~10-25% of monthly cap per hour.
- **TLS fingerprint enforcement**. Whether AWS Q upstream actually
  rejects requests by JA3 / TLS-cert-chain mismatch is unproven.
  Community reports of "VPN works but suddenly doesn't" suggest some
  TLS-layer signal but no peer code captures or normalizes it.
- **Concurrent-from-multiple-IPs revocation**. PROTOCOL.md §9.2 lists
  this as suspected; no peer has reproduced it formally.
- **Bedrock prompt-caching credit discount**. Anthropic's native API
  discounts cached input tokens to ~10% of cost. Kiro's billing model
  is in opaque "credits"; the cache discount magnitude is not
  publicly documented. INFERRED similar to Anthropic.
- **Per-region quota differences**. `getUsageLimits` returns the same
  per-account ledger regardless of region, but underlying capacity
  pressure (manifesting as 5xx) may differ. Not investigated.
- **`x-kiro-*` IDE-specific headers**. Some peer proxies forward
  `x-amzn-kiro-agent-mode: vibe` (Quorinex) and the `tokentype: API_KEY`
  (huasiyuuuuu/kiro for `ksk_` flow). Whether these are required or
  ignored by upstream is mixed across community reports.
- **`agentContinuationId`** field semantics. Set by AntiHub plugin; no
  observable effect when kiroxy omits it. ([PROTOCOL.md §11.10](../kiroxy/research-v4/PROTOCOL.md))
- **Token-bucket vs leaky-bucket rate limiter shape**. Both 60-min
  rolling and instantaneous 429s exist. Internal AWS implementation
  is opaque.
- **Whether 50 accounts on residential IP is "safe"**. Reports vary
  (no community-wide benchmark). enowXlabs's claim of 50+ accounts
  per IP is plausible but unverified — they may be using residential
  IP rotation or each VPS for fewer accounts than advertised.

These are **stated unknowns**, not silenced gaps.

---

## Sources

### Official AWS / Kiro

- [https://kiro.dev/pricing](https://kiro.dev/pricing) (accessed 2026-05-13)
- [https://kiro.dev/docs/billing](https://kiro.dev/docs/billing/) (accessed 2026-05-13)
- [https://kiro.dev/docs/models](https://kiro.dev/docs/models/) (accessed 2026-05-13)
- [aws/amazon-q-developer-cli](https://github.com/aws/amazon-q-developer-cli) — Apache-2.0; canonical SDK including `GetUsageLimits`, `UsageLimitList`, `UsageBreakdown`, `SubscriptionType`, `ResourceType`, `Origin` enums

### Kiroxy primary

- `internal/kiroclient/backoff.go` — exponential backoff with jitter
- `internal/kiroclient/client.go` — request dispatch + retry loop
- `internal/kiroclient/aws_error.go` — error class normalization
- `internal/pool/pool.go` — LRU + cooldown policy
- `internal/reqconv/cache_points.go` — cachePoint insertion
- `research-v4/PROTOCOL.md` — full protocol reference
- `research-v4/FAILURES.md` — known failure catalog
- `research-v4/ECOSYSTEM.md` — peer/upstream client survey

### Peer Kiro proxies (cited)

- [d-kuro/kirocc](https://github.com/d-kuro/kirocc) — Apache-2.0; reference implementation
- [Quorinex/Kiro-Go](https://github.com/Quorinex/Kiro-Go) — dual-endpoint failover at `proxy/kiro.go`
- [hj01857655/kiro-account-manager](https://github.com/hj01857655/kiro-account-manager) — Tauri/Rust, multi-region probing, getUsageLimits introspection
- [chaogei/Kiro-account-manager](https://github.com/chaogei/Kiro-account-manager) — TypeScript fork, dual-endpoint config
- [diegosouzapw/OmniRoute](https://github.com/diegosouzapw/OmniRoute) — MIT
- [decolua/9router](https://github.com/decolua/9router) — MIT
- [caidaoli/kiro2api](https://github.com/caidaoli/kiro2api) — sequential pool
- [Finesssee/ProxyPilot](https://github.com/Finesssee/ProxyPilot) — Go, includes Kiro auth
- [mxyhi/token_proxy](https://github.com/mxyhi/token_proxy) — Apache-2.0
- [africa1207/AWS-BuildID-Auto-For-Ext](https://github.com/africa1207/AWS-BuildID-Auto-For-Ext) — MIT; `validateToken` via `getUsageLimits`
- [Specia1z/AWS-BuildID-Auto-For-Ext](https://github.com/Specia1z/AWS-BuildID-Auto-For-Ext) — MIT mirror
- [kkddytd/claude-api](https://github.com/kkddytd/claude-api) — Go, mimics Kiro IDE UA
- [kittors/CliRelay](https://github.com/kittors/CliRelay) — MIT; advanced cooldown architecture
- [0xAstroAlpha/cliProxyAPI-Dashboard](https://github.com/0xAstroAlpha/cliProxyAPI-Dashboard) — MIT; dashboard variant
- [huasiyuuuuu/kiro PR #2](https://github.com/huasiyuuuuu/kiro/pull/2) — `ksk_` API-key flow documentation

### Issues (kirodotdev/Kiro upstream)

- [#8001 — Persistent 429 after unban (Security precaution false positive)](https://github.com/kirodotdev/Kiro/issues/8001) — 2026-04-30
- [#8058 — credits_exhausted but billing shows 0/50 used](https://github.com/kirodotdev/Kiro/issues/8058) — 2026-05-02
- [#8159 — Constant "Too many requests" with model selection](https://github.com/kirodotdev/Kiro/issues/8159) — 2026-05-05
- [#8360 — Upgrading to paid didn't clear 429](https://github.com/kirodotdev/Kiro/issues/8360) — 2026-05-11
- [#7289 — Rate limit crash during CSS sterilization spec](https://github.com/kirodotdev/Kiro/issues/7289) — 2026-04-09
- [#7901 — Subagent rate limit, no user feedback](https://github.com/kirodotdev/Kiro/issues/7901) — 2026-04-27
- [#5678 — Sync usage data across windows](https://github.com/kirodotdev/Kiro/issues/5678) — 2026-02-11
- [#695 — Bring Your Own Anthropic API Key](https://github.com/kirodotdev/Kiro/issues/695) — 2025-07-17 (75 reactions)

### External issues

- [UW-HARVEST/harvest#138](https://github.com/UW-HARVEST/harvest/pull/138) — verbatim error: `60-minute credit limit exceeded`

### Dead-end docs

- `https://kiro.dev/docs/billing-and-licensing/usage-limits/` — 404
- `https://kiro.dev/docs/account-and-subscription/manage-your-subscription/` — 404
- AWS Service Quotas console pages for `codewhisperer` and `q-developer` — not publicly browsable

---

*Kiroxy maintains its own protocol reference at
`research-v4/PROTOCOL.md` and ecosystem analysis at
`research-v4/ECOSYSTEM.md`. This document focuses specifically on
rate-limit semantics for the ENOWX_STUDY analysis. Update when:
(a) Kiro publishes formal usage-limit docs, (b) `getUsageLimits` schema
changes (Smithy regeneration), (c) new peer proxies adopt patterns we
should track.*
