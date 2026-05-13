# enowXlabs Public Signals Dossier

> Librarian report, compiled 2026-05-13 (Makassar).
> Feeds the "enowXlabs Architecture Hypotheses" section of ENOWX_STUDY.md.

## Identity

- **Official URL:** https://enowxlabs.com
- **Legal entity:** "enowX Labs", Semarang, Indonesia (ToS §11)
- **Merchant of record:** Paddle.com Market Limited
- **Founded:** 2026
- **GitHub org:** https://github.com/enowX-Labs — 53 followers, 1 public repo (`enowxcord`, unrelated)
- **Discord:** https://discord.gg/bkD3xXeQYK (+ Telegram @enowxlabs, WhatsApp group)
- **Team size estimate (SPECULATION):** 1-3 people. Signals: 0 listed GitHub org members, 0 blog posts, 0 changelog posts, placeholder counters on homepage

## Product — What enowxai Actually Is

**CRITICAL CORRECTION TO OUR WORKING ASSUMPTION.** enowxai is **NOT a hosted multi-account proxy**. It is:

> **A self-hosted local binary (BYOA — Bring Your Own Accounts)** that runs on user's machine, automates Google sign-in against Kiro/CodeBuddy/Canva, exposes local OpenAI/Anthropic API at `localhost:1430` + dashboard at `localhost:1431`.

- Product tagline: "SelfHost AI Proxy BYOA"
- User supplies the Google `email:password` pairs; enowxai runs headless browser locally
- Install: `curl -sSL https://api.enowxlabs.com/install/enowx-ai | bash`
- Binary is **closed-source** — install script + HTML dashboard visible; core Go binary private

## Architecture

- **Language:** Go (single-file binary, linux/darwin/windows × amd64/arm64)
- **Runtime:**
  - Binary: `~/.local/bin/enowxai`
  - Config + SQLite DB
  - Auth scripts downloaded as `auth-scripts.tar.gz` from CDN (Python venv + Camoufox + `numpy<2`)
- **Auth engines:**
  1. **Rod** (default, Go-native Chrome) — "may trigger captcha on some IPs"
  2. **Camoufox** (Firefox anti-detect, ~300MB, optional)
- **Upstream providers:**
  - **Standard** = Kiro (AWS CodeWhisperer) — Sonnet/Haiku/DeepSeek/MiniMax/GLM/Qwen
  - **MAX** = CodeBuddy (Tencent) — Opus/GPT-5/Gemini/Kimi
  - **Canva** = image generation
- **Local services:** Proxy `:1430`, dashboard `:1431`, daemon default
- **MITM mode:** Generates local CA, writes `/etc/hosts`, installs cert into trust store (for Cursor/Trae/Antigravity interception)
- **Network:** HTTP/SOCKS5 upstream routing, `enowxai expose start` binds 0.0.0.0 with IP whitelist

## Public Technical Claims — ACTUAL STATEMENTS vs Working Assumption

| Working Assumption | Actual Enowxlabs Statement | Gap |
|---|---|---|
| "50+ accounts fully automated" | Never stated. Users add accounts manually. | Over-claimed |
| "Without residential proxy" | **Not claimed.** Proxy support IS offered; "Rod may trigger captcha on some IPs" | Over-claimed |
| "Without captcha" | Offloaded: Rod triggers captcha sometimes; Camoufox is opt-in mitigation | Over-claimed |
| "Without SMS/email verification" | **Explicit docs:** *"Use fresh Google accounts. The Google account must NOT have any pending verification (phone verification, 2FA, security prompts, etc.). Any verification step will cause the automated login to fail."* | **Problem offloaded to user entirely** |

## Smoking Guns

1. **v2.0.0 changelog added "Buy Gsuite pages"** — affiliate/resale of Google Workspace accounts to paying users. They monetize the procurement problem, not solve it technically.
2. **"Temp Mail HTML rendering fixes"** in v2.0.0 — user workflow needs disposable email for mass account creation.
3. **"Content filters — synced from server every 5 minutes. All users share the same filter list"** — server-pushed regex obfuscation rules (crowdsourced via Discord).
4. **Rod default, Camoufox opt-in** — targets low-volume individual user on residential IP, not scale operators.
5. **MITM with system-trust-store CA install** — high-trust operation hidden behind single CLI command; enables Cursor/Trae intercept.
6. **`auth-scripts.tar.gz` served at runtime** — Python automation NOT in binary; pulled fresh. Enowxlabs can change headless-login behavior server-side without binary update. Supply-chain chokepoint.
7. **`/apps/enowx-ai` page uses `Anxthxropic`** (with Xs) — trademark-dodge obfuscation against Anthropic legal surveillance.
8. **Pricing tiers (from ToS §3):** Free / One-Time / Subscription via Paddle. HWID-bound + ECDSA-signed + heartbeat.

## Business Model Hypothesis (SPECULATION)

Plausible income streams:
- **Primary:** Paddle-billed subscriptions to *other* apps on platform; enowxai = zero-CAC Discord funnel
- **Secondary:** Gsuite account affiliate/resale via v2.0.0 "Buy Gsuite" page
- **Tertiary:** Future paid enowxai tier (License Types scaffolding live in ToS)
- **NOT plausible:** Enowxlabs running own account fleet (BYOA model contradicts it)

## Community Footprint

| Platform | Finding |
|---|---|
| GitHub org | 53 followers; 1 unrelated repo; 0 listed members |
| GitHub code search | **0 hits** for enowxlabs in any peer Kiro-proxy project |
| HN | **0 real hits** (false-positive "enolalabs" match only) |
| Reddit | No references found in r/selfhosted, r/LocalLLaMA, r/ClaudeAI |
| Twitter/X | No linked handle |
| LinkedIn | No company page |
| Product Hunt | No launch found |
| YouTube | No demos found |
| Medium/Dev.to | No reviews found |
| **Discord** | **Primary community — gated distribution** |

**Assessment:** Community footprint is **almost entirely Discord-native**. Deliberate low-visibility pattern consistent with gray-area tool avoiding indexable surfaces.

## Honest Dead-Ends

- **Founder identity** — no masthead, no named contributors, no LinkedIn
- **Discord community size** — cannot probe without joining
- **Actual download counts** — self-reported "42.6K" not independently verifiable
- **Enowxai Go binary internals** — closed source; `api.enowxlabs.com/downloads/enowx-ai` returns `{"error":"missing authorization header"}`
- **Indonesian corporate registry** — not probed (out of scope for web-signal collection)

## Implications for kiroxy BACKLOG

1. **Reframe positioning** — kiroxy's moat is not "replicate 50+ account onboarding". Enowxlabs doesn't claim that either. Both are tools requiring user-supplied accounts.
2. **Consider MITM-mode parity** for closed clients (Cursor/Trae) — this IS a novel UX-level feature.
3. **Consider content-filter sync pattern** — server-distributed obfuscation rules. UX good, security audit required.
4. **Evaluate Camoufox integration** as optional dependency for hostile IP blocks.
5. **Evaluate CodeBuddy (Tencent) as second upstream provider** — enowxlabs reverse-engineered this already; interesting prior art.
6. **Don't fight Discord-gated distribution** — kiroxy's moat is transparency, OSS auditability, reproducible builds.
