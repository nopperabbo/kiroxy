# Account Warmup Patterns for Multi-Account AI Gateways

> Librarian report, compiled 2026-05-13 (Makassar).
> Feeds the "Warmup Patterns" section of ENOWX_STUDY.md.

## Executive Summary

**Does warmup matter for Kiro? YES.** The kirodotdev/Kiro issue tracker has active ban waves with a community-observed threshold ~100 credits/day on new accounts (issue #6685). Some accounts banned at 20 credits. One ban cascaded to full AWS account closure (issue #6282). kiroxy today has NO account-age awareness — it rotates pure LRU + post-hoc cooldown only.

## Key Finding: Kiro Ban Thresholds (Community-Reported)

| Threshold | Evidence |
|---|---|
| ~100 credits/day on new Kiro accounts → ban risk | [kirodotdev/Kiro#6685](https://github.com/kirodotdev/Kiro/issues/6685) |
| 20-100 credits → newly-created accounts locked repeatedly | peer discussion |
| One ban cascaded to full AWS acct closure | [kirodotdev/Kiro#6282](https://github.com/kirodotdev/Kiro/issues/6282) |
| "60-minute credit limit exceeded" = rolling short-window quota | [UW-HARVEST/harvest#138](https://github.com/UW-HARVEST/harvest/pull/138) |

## Industry Warmup Curves — The Convergent Standard

**Gold standard comes from infra (Envoy, HAProxy) + email (SendGrid, SES), NOT from AI gateways.** AI gateways (LiteLLM, Portkey, OpenRouter, every Kiro peer) are all age-agnostic — they rely purely on reactive cooldown.

### Envoy slow_start Formula (reference pattern)
```
current_weight = max_weight × min(
  1.0,
  (time_since_addition / slow_start_window)^aggression
)
```
- `aggression < 1`: aggressive ramp (fast-at-start-slow-at-end)
- `aggression > 1`: conservative ramp (slow-at-start-fast-at-end)
- `min_weight_percent` floor to prevent zero weight

### SendGrid IP Warmup (41-day published curve)
Day 1: 50 / Day 7: 1,000 / Day 14: 10,000 / Day 30: 100,000 / Day 41: 1,000,000

### HAProxy slowstart
Time-based ratio to connection limits and weights.

## Peer AI Gateway Behavior — All Age-Agnostic

| Tool | Warmup support | What they do instead |
|---|---|---|
| LiteLLM | NO | Static weighted-random + reactive cooldown on errors |
| Portkey | NO | Static weighted-random |
| OpenRouter | NO (closed source, but docs imply) | Credit-based routing |
| petehsu/KiroProxy | NO | Probabilistic circuit breaker + 60s stickiness |
| Quorinex/Kiro-Go | NO | Weight-based pool, no age dimension |
| jwadow/kiro-gateway | NO | Global stickiness until failure |
| d-kuro/kirocc | N/A (single-account) | — |
| caidaoli/kiro2api | NO | Round-robin |

**Nobody in the AI-gateway space has shipped proper warmup yet.** This is green-field opportunity for kiroxy.

## Kiroxy Current State

- `Account.UpdatedAt` exists (last refresh) — **no `CreatedAt`**
- `internal/pool/pool.go` → pure LRU + 3-strike cooldown
- No daily request counter per account
- No age-based weight dampening
- `tools/onboard/warmup.py` exists but does **SESSION PRIMING** (browser cookies YouTube/Google before OAuth), NOT traffic warmup. Worth renaming to avoid confusion.

## Recommended Warmup Curve for Kiroxy (Kiro-specific)

Given the ~100 credits/day threshold and that Kiro accounts refill ~1000/mo (Pro tier), the minimum-viable warmup curve:

```
# Envoy form, conservative (aggression=0.8 = slow-at-start-fast-at-end)
WARMUP_WINDOW = 14 days
MIN_WEIGHT_PERCENT = 2%
AGGRESSION = 0.8

age_days = min(WARMUP_WINDOW, days_since_CreatedAt)
age_ratio = age_days / WARMUP_WINDOW
weight = max(MIN_WEIGHT_PERCENT, age_ratio^AGGRESSION)

# Plus a hard daily-request cap:
cap_per_day = 30 + (WARMUP_WINDOW_CAP - 30) × age_ratio
# Day 0 ≈ 24 req/day, Day 7 ≈ 115/day, Day 14+ = full capacity
```

## BACKLOG Seeds (ranked)

1. **P0 (minimum-viable warmup PR, ~120 LoC)**:
   - Add `Account.CreatedAt` field to schema
   - Add `ageWeight()` function in pool selector
   - Add daily-request counter per account
   - Weight-dampened LRU in `Pool.Pick()`

2. **P1 (hard cap gate)**:
   - Reject account selection if daily cap hit
   - Expose metric `kiroxy_account_daily_requests{account_id, age_days}`
   - Circuit-break account if upstream 429 received (separate from existing 3-strike cooldown)

3. **P2 (adaptive)**:
   - Warmup curve tuning knobs in config
   - Pre-emptive account retirement after N days (if Kiro usage-cap-hit pattern emerges)

## Honest Dead-Ends

- **No measured Kiro curve.** "100 credits/day on new accounts" is community-reported, not measured in a controlled study.
- **Google's detection thresholds** aren't documented anywhere.
- **SubAgent dispatch failed in this research session** — did all research inline with grep_app + direct fetches, so some claims are from sampled code rather than full repo reads.
- **OpenRouter has no public source** — their warmup posture is inferred from docs only.
