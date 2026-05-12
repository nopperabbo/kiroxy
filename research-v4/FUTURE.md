# FUTURE.md — Kiro / Q Developer roadmap signals for kiroxy

> Forward-looking research. Date range surveyed: Jan 2025 – May 13 2026.
> Today: Wed May 13 2026. Migration deadline: **May 15 2026 (T-2 days).**
> All claims cite source + date. Speculation is labeled `(speculation)`.

---

## 1. Kiro IDE release signals

### What rolled out in the last ~10 months

Data pulled from [kiro.dev/blog](https://kiro.dev/blog) and [kiro.dev/changelog](https://kiro.dev/changelog/general/).

| Date | Milestone | Signal |
|---|---|---|
| Jul 14 2025 | Kiro public launch ([Introducing Kiro](https://kiro.dev/blog/introducing-kiro/)) | Preview branding |
| Aug–Sep 2025 | Pricing plans, Auto agent ([pricing-plans-are-live](https://kiro.dev/blog/pricing-plans-are-live/), [new-pricing-plans-and-auto](https://kiro.dev/blog/new-pricing-plans-and-auto/)) | End of free-for-all era |
| Oct 16 2025 | Waitlist ended ([waitlist-is-over](https://kiro.dev/blog/waitlist-is-over/)) | General public signup |
| Nov 17 2025 | **GA + Kiro CLI 1.0 + checkpointing + startup credits** ([general-availability](https://kiro.dev/blog/general-availability/)) | Exits preview. Kiro Preview clauses in AWS Service Terms §50.3 likely become less relevant. |
| Nov 24 2025 | Opus 4.5 ([introducing-opus-45](https://kiro.dev/blog/introducing-opus-45/)) | First model-drop-on-launch-day |
| Dec 2 2025 | **Kiro autonomous agent preview** at re:Invent ([autonomous-agent](https://kiro.dev/blog/introducing-kiro-autonomous-agent/), [AWS press](https://www.aboutamazon.com/news/aws/amazon-ai-frontier-agents-autonomous-kiro)) | Async long-running sandboxed agent, GitHub-native |
| Dec 3 2025 | Kiro **Powers** ([introducing-powers](https://kiro.dev/blog/introducing-powers/)) | Extension/registry primitive |
| Jan 14–16 2026 | IDE diagnostics + Run all tasks | Parallel execution infra |
| Feb 5 2026 | **Kiro 0.9**: custom subagents + ACP (JetBrains) + Opus 4.6 ([custom-subagents](https://kiro.dev/blog/custom-subagents-skills-and-enterprise-controls/), [adopts-acp](https://kiro.dev/blog/kiro-adopts-acp/)) | Multi-agent, multi-IDE footprint |
| Feb 10 2026 | **Open weight models** ([open-weight-models](https://kiro.dev/blog/open-weight-models/)): DeepSeek 3.2, MiniMax M2.1, Qwen3 Coder Next | First non-Claude models |
| Feb 17–20 2026 | Sonnet 4.6, GovCloud, new spec types ([govcloud](https://kiro.dev/blog/introducing-govcloud/)) | Regulated workloads |
| Mar 17 2026 | **CVE-2026-4295** RCE via crafted project dirs, fixed in 0.8.0 ([bulletin 2026-009-AWS](https://aws.amazon.com/security/security-bulletins/rss/2026-009-aws/)) | First public CVE. Kiro takes it seriously — auto-update enforcement will tighten. |
| Apr 13 2026 | **Kiro CLI 2.0**: Windows, headless mode, **API key auth** for Pro+/Power ([cli-2-0](https://kiro.dev/blog/cli-2-0/)) | First sanctioned programmatic access. Kiro now ships its own key-based auth path. |
| Apr 17 2026 | Opus 4.7 experimental ([opus-4-7](https://kiro.dev/blog/opus-4-7/)) | Available same week as Anthropic's direct API |
| Apr 23 2026 | Community hub, Kiro Labs | Commercial / ecosystem building |
| Apr 24 2026 | CLI 2.1 — Tool Search (dynamic MCP loading), Skills as `/slash` commands, device-flow auth | Device-flow auth matters for headless / SSH, and is exactly the surface a proxy onboarder uses. |
| Apr 27 2026 | CLI 2.2 — **Adaptive thinking preserved across turns** | Thinking content is state that flows through requests — cache semantics shift. |
| Apr 29 2026 | Endpoint migration notice posted via AWS Health ([jwadow#146](https://github.com/jwadow/kiro-gateway/issues/146)) | T-16 day warning |
| May 6 2026 | IDE **0.12.155**: parallel tasks, Quick Plan, Analyze Requirements | |
| May 7 2026 | **Kiro Web Preview** at [app.kiro.dev](https://app.kiro.dev) + Opus 4.7 with adaptive thinking across IDE/CLI ([web-preview](https://kiro.dev/changelog/web/introducing-kiro-web-preview/)) | The browser product lands. Autonomous agent productized. |
| May 8 2026 | New paid-tier sign-up bonus | User-acquisition push |

### Expected in the next 6 months (May – Nov 2026)

Evidence-based, not speculation:
- **Opus 4.7 graduates from Experimental to Active.** Pattern from Opus 4.5 (Nov 24 2025 → Active immediately) and Sonnet 4.6 (Feb 17 2026 → Active on launch) says roughly 1-2 months experimental for Claude models. Expected transition: **Jun–Jul 2026**.
- **More frontier agents.** AWS announced three frontier agents in Dec 2025 — Kiro Autonomous, AWS Security Agent, AWS DevOps Agent. Kiro is the developer-facing one; expect the two others to expose kiro.dev-hosted endpoints with similar auth ([frontier-agents](https://aws.amazon.com/ai/frontier-agents/)).
- **Kiro Web general availability.** Preview → GA cadence for other Kiro surfaces has been ~3 months (CLI preview Nov 2025 → GA same day, but Web launched as full preview May 2026). Expect GA **Aug–Sep 2026**.
- **Tokyo region request** (issue [#5905](https://github.com/kirodotdev/Kiro/issues/5905)) has community pressure but no AWS response. Historical AWS pattern: Bedrock took ~18 months from US/EU launch to Japan geographic CRIS. **Unlikely before Q4 2026.** `(speculation-based-on-historical-bedrock-rollout)`
- **API key auth for Pro tier** (not just Pro+/Power). Pro+ API keys shipped Apr 13 2026; extending to Pro is low-risk and high-demand.

---

## 2. AWS Developer Tools / Q Developer positioning

### Migration direction is clear

From [kiro.dev/docs/migrating-from-q-developer](https://kiro.dev/docs/migrating-from-q-developer/) (updated Apr 30 2026):

> "If you've been using Amazon Q Developer in your IDE, everything you rely on today is available in Kiro."

Kiro IDE **supersedes** Q Developer IDE extensions. Q Developer for Visual Studio and Eclipse has no Kiro replacement — users are told to use Kiro CLI or Kiro IDE standalone.

The feature table shows Q Developer IDE extensions have **none** of Kiro's differentiating features (steering, hooks, subagents, powers, specs). Kiro is the product, Q Developer is legacy.

### The kiro.dev endpoint migration is the final rename

Source: AWS Health event `AWS_CODEWHISPERER_PLANNED_LIFECYCLE_EVENT_9c3645c5...` posted to customers around Apr 29 2026 ([jwadow#146](https://github.com/jwadow/kiro-gateway/issues/146)):

> "Beginning May 15, 2026, Kiro will transition from the legacy `q.<region>.amazonaws.com` endpoint to new dedicated endpoints under the kiro.dev domain."

Endpoints:
- `runtime.<region>.kiro.dev` — inference, processing, streaming
- `management.<region>.kiro.dev` — configuration, lifecycle, access management
- `telemetry.<region>.kiro.dev` — metrics, observability
- `cli.kiro.dev` — CLI-specific

The AWS Health event is tagged `CODEWHISPERER` internally, confirming the CodeWhisperer → Q Developer → Kiro rename trail all points at one service.

### Still-legacy surface (flagged by docs)

From [kiro.dev/docs/web/firewalls](https://kiro.dev/docs/web/firewalls/):

> "The `q.<region>.amazonaws.com` endpoints are legacy and **will be deprecated in a future release**. Until deprecation is complete, you must still allowlist them alongside the `runtime`, `management`, and `telemetry` endpoints."

Kiro is keeping `q.*.amazonaws.com` alive for some functions even after May 15. Specifically:
- **`/ListAvailableModels` still lives on the old endpoint** (per [jwadow#146 testing](https://github.com/jwadow/kiro-gateway/issues/146) — Apr 29 2026, confirmed against runtime endpoint)
- VPC endpoint service names `com.amazonaws.<region>.q` and `com.amazonaws.us-east-1.codewhisperer` are unchanged ([vpc-endpoints docs](https://kiro.dev/docs/cli/privacy-and-security/vpc-endpoints/))

**Implication:** The May 15 2026 cutoff is **partial**. Expect a second migration 6-12 months later to move `/ListAvailableModels` and kill `q.*.amazonaws.com` entirely.

---

## 3. Endpoint migration & regional expansion

### Hard facts from real migration testing

From the `jwadow/kiro-gateway` migration PR #155 (confirmed working E2E through gateway, Apr 29 2026):

| Requirement | Old (`q.*.amazonaws.com`) | New (`runtime.*.kiro.dev`) |
|---|---|---|
| `Content-Type` | `application/json` OK | **`application/x-amz-json-1.0` required** |
| `x-amz-target` | optional | **required** (`AmazonCodeWhispererStreamingService.GenerateAssistantResponse`) |
| `profileArn` in body | only for Desktop auth | **mandatory for all auth types** (inc. SSO OIDC) |
| Model ID format | ARN OK (`anthropic.claude-sonnet-4-5-20250514-v1:0`) | **dot-format only** (`claude-sonnet-4.5`, `auto`) |
| Valid models | ARN list | `auto`, `claude-sonnet-4`, `claude-sonnet-4.5`, `claude-sonnet-4.6`, `claude-opus-4.5`, `claude-opus-4.6`, `claude-opus-4.7`, `claude-haiku-4.5` |
| `/ListAvailableModels` | works | **404 (still only on legacy)** |
| `profileArn` source for SSO OIDC | token data | kiro-cli `state` table key `api.codewhisperer.profile` |

Python `httpx` gotcha from the same PR: `json=` parameter overrides `Content-Type` back to `application/json`. Must use `content=json.dumps(...).encode()`.

### Regions

- **Inference regions (Kiro profile):** `us-east-1`, `eu-central-1` + `us-gov-east-1`, `us-gov-west-1` (GovCloud, Feb 2026)
- **IAM Identity Center regions for user identity:** 20 regions across US, EU, Asia Pacific, Canada, South America, GovCloud ([supported-regions](https://kiro.dev/docs/enterprise/supported-regions/)) — identity-only, not inference
- **Tokyo + Asia Pacific**: requested ([issue #5905](https://github.com/kirodotdev/Kiro/issues/5905), Feb 20 2026), no AWS response, not on public roadmap

### Signals for next regional expansion `(speculation)`

- Bedrock cross-region inference goes US, EU, GovCloud first, then Japan + Australia ~12-18 months later. Kiro is pinned to Bedrock, so next region is likely **ap-northeast-1 (Tokyo) with Japan geographic CRIS** — probably **Q4 2026 or Q1 2027**.
- Global cross-region inference is already used for **experimental** features ([data-protection docs](https://kiro.dev/docs/privacy-and-security/data-protection/#global-cross-region-inference-for-experimental-features)) — this is how Kiro routes to regions outside user's profile for Opus 4.7, GLM-5, DeepSeek 3.2, MiniMax M2.5/M2.1, Qwen3 Coder Next.
- The new endpoint scheme is **region-named but flat**: `runtime.<region>.kiro.dev`. Adding regions is a DNS + TLS-cert problem, not a code problem. AWS can add new regions in hours once Bedrock is ready in-region.

---

## 4. Model availability signals

### Claude cadence (Anthropic → Kiro)

From [kiro.dev/docs/models](https://kiro.dev/docs/models/):

| Model | Launch on Kiro | Anthropic release | Lag |
|---|---|---|---|
| Opus 4.7 | Apr 16 2026 (Experimental → Active by May 7) | Apr 2026 | **same week** |
| Sonnet 4.6 | Feb 17 2026 (Active on launch) | Feb 2026 | same week |
| Opus 4.6 | Feb 5 2026 | Feb 2026 | same week |
| Opus 4.5 | Nov 24 2025 | Nov 2025 | same week |
| Sonnet 4.5 | Sep 29 2025 | Sep 2025 | same week |
| Sonnet 4.0 | Sep 4 2025 | May 2025 | 4 months |

Claude models now land on Kiro on or near day 1. The Sonnet 4.0 gap reflects Kiro's pre-launch state; from GA forward, Kiro is a first-party Anthropic launch partner via Bedrock.

**Implication for kiroxy:** When Anthropic announces Claude 4.8 or Claude 5, assume Kiro gets it within days. Proxy model-ID mapping tables must be easy to extend.

### Non-Claude models

| Model | Launch | Region | Credit mult |
|---|---|---|---|
| Qwen3 Coder Next | Feb 10 2026 | us-east-1, eu-central-1 | 0.05x (cheapest) |
| MiniMax M2.1 | Feb 10 2026 | us-east-1, eu-central-1 | 0.15x |
| DeepSeek 3.2 | Feb 10 2026 | us-east-1 | 0.25x |
| MiniMax M2.5 | Mar 18 2026 | us-east-1, eu-central-1 | 0.25x |
| GLM-5 | Apr 2 2026 | us-east-1 | 0.5x |

All are still **Experimental** status with global cross-region inference. They're cheap (0.05x–0.5x credit multiplier) and available on **Free tier + all paid tiers**.

### Free vs Pro tier gating

- **Jan 17 2026**: Opus 4.5 removed from free tier (per [jwadow/kiro-gateway README](https://github.com/jwadow/kiro-gateway))
- **Free tier today** (from docs): Auto, Sonnet 4.5, Sonnet 4.0, Haiku 4.5, DeepSeek 3.2, MiniMax M2.5, GLM-5, MiniMax M2.1, Qwen3 Coder Next
- **Pro/Pro+/Power only**: all Opus variants (4.5/4.6/4.7), Sonnet 4.6

Pattern: older/more-expensive models get pulled from free tier first. **Expect Sonnet 4.5 to be paid-only once 4.6 is Active for ~3 months** `(speculation)`.

### Context caching / adaptive thinking

Opus 4.7's adaptive thinking (Apr 27–May 7 2026) preserves "thought content" across multi-turn conversations. This is new request/response state that proxies must pass through transparently or coherence breaks. No public caching API yet, but thinking-token-reuse is a form of implicit caching.

---

## 5. Feature flags / beta signals

### What's currently gated

| Feature | Gating | Notes |
|---|---|---|
| Kiro Web | Pro/Pro+/Power only | Invite-based preview via `kiro` GitHub label or `/kiro` comment |
| Autonomous agent | Pro/Pro+/Power + waitlist for teams | Free during preview, weekly limits |
| API key auth (headless) | Pro+/Power | Via `KIRO_API_KEY` env var |
| IAM Identity Center auth | Enterprise | Opus 4.7 initial experimental rollout was IAM-IC-only |
| Experimental models | All tiers but **global CRIS applies** | Data can cross regions |
| Opus 4.7 | All paid tiers + IAM-IC (broader social-login rollout "coming soon" as of Apr 17) | |

### Feature flags observed in Kiro API responses

From the jwadow/kiro-gateway PR #155 work:
- `profileArn` key found in kiro-cli's SQLite `state` table under `api.codewhisperer.profile`
- Auth endpoints differ by auth type:
  - Kiro Desktop: `https://prod.{region}.auth.desktop.kiro.dev/refreshToken`
  - AWS SSO OIDC: `https://oidc.{region}.amazonaws.com/token`
- clientId/clientSecret presence is the signal distinguishing the two auth types (per kiro-gateway README)

### Prompt caching evolution

Not explicitly documented in public Kiro docs as of May 13 2026. **But**:
- AWS Bedrock has had implicit prompt caching since mid-2025
- Kiro's Auto router mentions "caching" as one of its optimization levers ([kiro.dev landing](https://kiro.dev/))
- Tool Search (CLI 2.1, Apr 24 2026) is effectively a form of MCP-tool-definition caching — "loads MCP tools on demand instead of sending every definition with each request"

Expect Kiro to expose explicit prompt-cache controls within 6 months `(speculation)`, matching the Anthropic API's `cache_control` parameter.

---

## 6. Policy risks

### AWS TOS / Service Terms

1. **AWS Customer Agreement + AWS IP License** govern Kiro ([kiro.dev/license](https://kiro.dev/license/)).
2. **AWS Service Terms §1.24** lists Kiro as a generative-AI service subject to Bedrock abuse detection.
3. **AWS Service Terms §50.3** (the one flagged by [Kiro issue #2206](https://github.com/kirodotdev/Kiro/issues/2206)) says AWS may use Kiro (Preview) inputs/outputs for training unless you're on Q Developer Pro via IAM-IC or have opted out via AWS Organizations AI opt-out policy. Post-GA (Nov 17 2025) this clause likely still applies to Free + individual subscribers (per [data-protection docs](https://kiro.dev/docs/privacy-and-security/data-protection/#service-improvement)).
4. **AWS AUP** ([aws.amazon.com/aup](https://aws.amazon.com/aup/), Last updated Jul 1 2021) prohibits: illegal/fraudulent use, violating others' rights, attacking systems, spam. It does **not** explicitly prohibit third-party proxying, automated access, or multi-user fan-out.
5. **Amazon Bedrock abuse detection** ([docs](https://docs.aws.amazon.com/bedrock/latest/userguide/abuse-detection.html)) runs classifiers on inputs/outputs, identifies patterns, and may share **anonymized** metrics with third-party model providers. The unusual request signature of proxy traffic (long sessions, unusual prompts, tool-call patterns mismatching Kiro IDE harness) is exactly the kind of thing these classifiers flag.

### Anthropic ToS

From [Anthropic Commercial Terms](https://www.anthropic.com/legal/commercial-terms):
- **§D.4 Use Restrictions**: customer may not "reverse engineer or duplicate the Services" or "resell the Services except as expressly approved by Anthropic" or "support any third party's attempt at any of the conduct restricted."
- **§I.3.a Suspension**: Anthropic may suspend for Use-Restrictions violations or attacks on the service.

From [Anthropic Consumer Terms](https://claude.ai/legal) §3.7 (unchanged since Feb 2024):
> "Except when you are accessing our Services via an Anthropic API Key or where we otherwise explicitly permit it, to access the Services through automated or non-human means, whether through a bot, script, or otherwise."

### The Feb 2026 Anthropic enforcement action (critical)

Source: [The Register, Feb 20 2026](https://www.theregister.com/software/2026/02/20/anthropic-clarifies-ban-on-third-party-tool-access-to-claude/5014546), [SitePoint analysis](https://www.sitepoint.com/end-wrapper-era-anthropic-api-terms-saas/).

Anthropic revised terms in Feb 2026 to **explicitly** ban third-party harnesses with Claude subscriptions:

> "Using OAuth tokens obtained through Claude Free, Pro, or Max accounts in any other product, tool, or service — including the Agent SDK — is not permitted and constitutes a violation of the Consumer Terms of Service."

Anthropic engineer Thariq Shihipar (Jan 2026):
> "Third-party harnesses using Claude subscriptions create problems for users and are prohibited by our Terms of Service. They generate unusual traffic patterns without any of the usual telemetry that the Claude Code harness provides."

**Downstream consequence**: OpenCode pushed commits removing Claude Pro/Max/API-key support citing "anthropic legal requests" ([The Register](https://www.theregister.com/software/2026/02/20/anthropic-clarifies-ban-on-third-party-tool-access-to-claude/5014546)).

### What this means for kiroxy

Kiroxy is a **Kiro** proxy (not a direct Claude proxy), so Anthropic's Feb 2026 consumer-token clarification doesn't apply verbatim — Kiroxy uses Kiro's AWS-issued tokens, not Claude Pro/Max OAuth tokens. But:

1. **AWS Service Terms §D.4-equivalent language** for reselling AI services exists (see AUP §1.4 and Service Terms §1.4 on prohibited content + account suspension).
2. **Bedrock abuse detection** can flag kiroxy traffic patterns to Anthropic (as an upstream third-party provider). Anthropic may pressure AWS to enforce.
3. **Anthropic's enforcement pattern** says they'll go after anything that smells like token arbitrage. A Kiro free-tier account reselling Claude access is functionally similar to what Anthropic just banned.
4. **No public AWS enforcement history for Kiro proxies yet.** The jwadow/kiro-gateway repo has 1.4k stars since launch and no public takedowns as of May 13 2026. But absence of enforcement isn't a policy guarantee.

### Peer-project navigation

- **jwadow/kiro-gateway**: AGPL-3.0, README explicitly markets "Use free Claude models with any client." High enforcement exposure. No disclaimers about ToS in README.
- **d-kuro/kirocc**: more neutral framing, straight engineering project, [issue #57](https://github.com/d-kuro/kirocc/issues/57) tracks migration mechanically.
- **Quorinex** (not findable in my searches — name may be private or mis-remembered).

---

## 7. External dependency risks

### Camoufox (used by onboarder automation)

Status as of May 13 2026: **healthy but forked.**

- **Original maintainer `daijro`**: 6.6k stars, [last push Mar 16 2026](https://github.com/daijro/camoufox). README explicitly notes a year-long maintenance gap due to a "personal situation."
- **Active fork `CloverLabsAI/camoufox`**: maintained by `icepaq`, `heydryft`, `PopcornDev1`. Last push Apr 24 2026. [Discussion #571 (Apr 12 2026)](https://github.com/daijro/camoufox/discussions/571) lays out the plan:
  - Rebrand from "antidetect browser" to "**AI agent browser**" (reduces antibot-platform attention)
  - Doubles down on stealth as core feature
  - **Windows support dropped from internal roadmap** — community maintainer needed
  - Hardware spoofing + per-context fingerprints released as `cloverlabs-camoufox` pip package ([Camoufox 2.0 commit](https://github.com/daijro/camoufox/commit/c6a6c20670e73e5e3deaf8aa9347561f8c27be28), Mar 14 2026)
- Firefox 146 base, v147 patches being tracked.

**Risk summary for kiroxy:**
- **Fork fragmentation**: main `camoufox` pip (delayed) vs `cloverlabs-camoufox` (bleeding edge). Pin one explicitly; don't let auto-update flip.
- **Windows support degrading**. If kiroxy onboarder runs on Windows hosts, audit Camoufox Windows path quickly.
- **Rebrand to AI agent browser** means antibot-specific features may get less emphasis. For AWS/Cognito onboarding (the actual kiroxy use), general stealth is still the roadmap.

### OAuth / social-login providers

Kiro supports 4 auth methods:
- GitHub (individual)
- Google (individual)
- AWS Builder ID (individual)
- IAM Identity Center (enterprise)

Device-flow auth added in CLI 2.1 (Apr 24 2026) — this is the flow most useful for automated onboarding (URL + 6-digit code entered in separate browser). Previously browser-redirect-only.

Risks:
- **Google has strongest anti-automation stance** — Google OAuth flow + Camoufox will be the hardest to keep stable.
- **GitHub** is generally friendlier to automation (many legit CLI tools use device flow).
- **AWS Builder ID** uses AWS SSO OIDC — relatively stable, no major friction.

### Kiro team / AWS investment signals

**Hiring / investment indicators**:
- Constant shipping: ~50 blog posts in 10 months (see Section 1 table), 2+ releases per week on changelog
- Students tier (Mar 18 2026), Startup credits (Apr 7 2026), Ambassadors program (May 7 2026), community hub + Kiro Labs (Apr 23 2026) — all funnel expansion
- Enterprise governance push: MCP server + model governance (Mar 12 2026), identity and usage metrics (Feb 13 2026), GovCloud (Feb 2026)
- re:Invent 2025 front-and-center billing ([AWS press](https://www.aboutamazon.com/news/aws/amazon-ai-frontier-agents-autonomous-kiro)): Kiro was one of three "frontier agents" — this is AWS's marquee AI story

**No signals of retrenchment**: no layoff news, no deprecation warnings, no feature pullbacks. AWS is still doubling down as of May 2026.

---

## kiroxy 6-month preparation checklist

### Blocker (do before May 15 2026)

1. **Finish endpoint migration to `runtime.<region>.kiro.dev`** using the headers from [jwadow#146 testing](https://github.com/jwadow/kiro-gateway/issues/146):
   - `Content-Type: application/x-amz-json-1.0`
   - `x-amz-target: AmazonCodeWhispererStreamingService.GenerateAssistantResponse`
   - `profileArn` in request body (source: kiro-cli `state` table key `api.codewhisperer.profile` for SSO OIDC)
   - Dot-format model IDs only
   - If using Python `httpx`, replace `json=` with `content=json.dumps(...).encode()`
2. **Keep `q.<region>.amazonaws.com` support** for `/ListAvailableModels` until Kiro moves that call too. Add a feature flag `KIRO_Q_HOST_TEMPLATE` with old-endpoint default for that one API.
3. **Allowlist `*.kiro.dev`, `*.app.kiro.dev`, `*.kiro.aws.dev`, `cli.kiro.dev`** in any outbound firewall config you ship or document.

### Near-term (next 30 days)

4. **Add model ID table for new dot-format names**: `auto`, `claude-sonnet-4`, `claude-sonnet-4.5`, `claude-sonnet-4.6`, `claude-opus-4.5`, `claude-opus-4.6`, `claude-opus-4.7`, `claude-haiku-4.5`, plus open-weight `deepseek-3.2`, `minimax-m2.1`, `minimax-m2.5`, `glm-5`, `qwen3-coder-next`.
5. **Test Opus 4.7 adaptive-thinking flow end-to-end.** Thinking content must pass through across turns or coherence degrades. Validate against Kiro CLI 2.2.0 behavior.
6. **Pin Camoufox version explicitly** in onboarder. Decide: stable `camoufox` (delayed) or `cloverlabs-camoufox` (bleeding edge). Don't let auto-update flip you between them.
7. **Audit onboarder for Windows Camoufox dependency.** CloverLabsAI dropped Windows from their internal roadmap.

### Medium-term (next 90 days)

8. **Monitor Kiro Web Preview for auth/protocol signals.** Web product uses same backend; new endpoints or auth changes may land there first.
9. **Prepare for Sonnet 4.5 → paid-only move.** Pattern suggests it gets pulled from free tier ~Q3 2026 once 4.6 stabilizes. Proxy free-tier users should get a fallback path to Sonnet 4.0 or Haiku 4.5.
10. **Add Pro+ API-key auth path.** Kiro CLI 2.0 headless mode (`KIRO_API_KEY`) is the first sanctioned programmatic access. When Kiro extends it to Pro tier (likely Q3 2026), kiroxy should support it as a lower-friction alternative to OAuth token harvesting.
11. **Monitor issue [#2206](https://github.com/kirodotdev/Kiro/issues/2206)** — Kiro post-GA data-policy clarifications. The answer determines what classes of kiroxy users are training fodder.

### Longer-term (6 months)

12. **Watch for second endpoint migration**. The current `q.<region>.amazonaws.com` cutoff is partial (`/ListAvailableModels` still legacy). Expect a follow-up deprecation 6-12 months later.
13. **Watch for ap-northeast-1 (Tokyo) region.** Bedrock historical pattern + community pressure ([#5905](https://github.com/kirodotdev/Kiro/issues/5905)) suggests Q4 2026 or Q1 2027. Add region-detection logic that doesn't hard-code `us-east-1` vs `eu-central-1`.
14. **Monitor for prompt-cache / adaptive-thinking explicit API.** When Kiro exposes `cache_control`-style headers, proxy must pass them through untouched.
15. **Track AWS/Anthropic enforcement tempo.** Anthropic started enforcing third-party-harness bans in Feb 2026. Set a quarterly calendar reminder to re-read:
    - [AWS AUP](https://aws.amazon.com/aup/)
    - [AWS Service Terms §1.24, §50.3](https://aws.amazon.com/service-terms/)
    - [Anthropic Commercial Terms §D.4](https://www.anthropic.com/legal/commercial-terms)
    - [Anthropic Consumer Terms §3.7](https://claude.ai/legal)
16. **Plan a BYOK (Bring-Your-Own-Kiro-Account) compliance mode** as a defensive posture — per-user Kiro credentials instead of shared accounts. The [SitePoint analysis](https://www.sitepoint.com/end-wrapper-era-anthropic-api-terms-saas/) frames this as the only durable pattern across LLM providers.

### Canary signals to watch weekly

- [kiro.dev/changelog](https://kiro.dev/changelog/) RSS/Atom feed for endpoint or auth changes
- [github.com/kirodotdev/Kiro/issues](https://github.com/kirodotdev/Kiro/issues) for endpoint/auth threads
- [github.com/jwadow/kiro-gateway/issues](https://github.com/jwadow/kiro-gateway/issues) + [d-kuro/kirocc/issues](https://github.com/d-kuro/kirocc/issues) — peer gateways surface migration pain first
- [AWS security bulletins](https://aws.amazon.com/security/security-bulletins/rss/) — after CVE-2026-4295 (Mar 17 2026), assume more are coming
- AWS Health dashboard events tagged `CODEWHISPERER` or `KIRO` — how AWS announces breaking changes
