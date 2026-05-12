# Dossier — v1 Tier 1 Delta Recheck (2026-05-10 → 2026-05-12)

_Reconstructed from librarian agent findings (2026-05-12); original file wiped by concurrent session fs interaction._

Focus: what **changed** in the six v1-surveyed Kiro projects since the prior survey on 2026-05-10/11. Assumes reader has v1 dossiers.

---

## 1. jwadow/kiro-gateway (Python, AGPL-3.0, 1311★)

**Verdict: essentially dormant.**

- 3 commits in the last 2 weeks, all CLA/contributor-paperwork chores.
- Real feature work (multi-account system, MCP tool emulation, payload-size guards) shipped in late April and then paused.
- Open issues of note:
  - **#146** — `*.kiro.dev` endpoint migration (AWS deprecating `q.<region>.amazonaws.com`). Deadline pressure on whole ecosystem; 3-day window as of 2026-05-12.
  - **#153** — `Write` tool truncation bug. Known; no fix yet.
- No new features to recheck. v1 dossier still accurate.
- **Competitive impact for kiroxy: none in this window.**

## 2. Quorinex/Kiro-Go (Go, MIT, 479★) — **MOST ACTIVE**

Three shipped features kiroxy doesn't have:

1. **SOCKS5 / HTTP outbound proxy support** (commit landed 2026-05-11, after v1 dossier close).
2. **Claude top-level `thinking` config routing** (PR #40) — explicit routing path for extended-thinking requests.
3. **Accurate `input_tokens` via `contextUsageEvent` parsing** (PR #37) — replaces estimator with upstream-reported token counts.

Additional changes:
- Prompt-cache improvement fixes.
- `claude-opus-4.7` model entry added.
- `handler.go` **still 89 KB** — the god-file Quorinex-v1-dossier flagged is unchanged.
- v1.0.6 release tag.
- 1 open PR: SQLite dependency bump.

**Competitive impact for kiroxy:**
- **`contextUsageEvent` parsing is new table stakes.** Our estimator is now the losing position. **Add to P0 for v0.4.0 or v1.0.0.**
- Top-level `thinking` config routing is becoming standard. **Add to P1.**
- Outbound proxy per-request is a hexos/kirocc pattern Quorinex now ships too. **Promotes backlog item "Hexos outbound proxy pool" from P3 to P2.**

## 3. justlovemaki/AIClient2API (JS, GPL-3.0, 7723★)

**Verdict: major release, actively iterating.**

- **v3.0.0** shipped 2026-05-04 with:
  - "AI self-discovery architecture" at `/api/help` — the proxy describes its own capabilities for LLM clients.
  - `--no-ui` headless mode.
  - Image generation support.
  - Unified 429-retry logic across providers.
- 13 releases in 2 weeks.
- **PR #585** — Kiro compat improvements: tool-name aliasing + request throttling.
- Open issues: OAuth flow bugs, proxy request shape mismatches, AWS region migration (ecosystem-wide), a few image-gen edges.

**Competitive impact for kiroxy:**
- GPL-3.0 blocks code reuse. Concepts only.
- **"AI self-discovery" pattern (`/api/help`)** is interesting — LLM clients introspecting what they can ask of the proxy. Could fit kiroxy as a low-LoC `/capabilities` endpoint. Novel.
- Unified 429-retry with provider-specific backoff tables. kiroxy currently has rudimentary retry; this is a reference.

## 4. kadangkesel/hexos (TS, MIT, archived)

**Verdict: DEAD. Archived 2026-05-10.**

- Final commit literally `"last of commit's, bye!!!"` on 2026-05-10.
- Pre-archive sprint added Devin AI support (48 models), Windsurf (via Connect Streaming protobuf), Fireworks AI, Cloudflare Workers AI.
- **No one has forked to continue** (v1 dossier's salvage-file plan stands).
- 1 fork total (by `nopperabbo`, which is our own account per agent check — i.e., kiroxy's parent dir held the hexos donor during research).

**Competitive impact for kiroxy:**
- Confirms hexos is not a live threat. Reference-only, as v1 concluded.
- The Devin / Windsurf / Connect Streaming work is **potential donor material** if kiroxy ever goes multi-provider. MIT licensed, file-level reuse OK.

## 5. d-kuro/kirocc (Go, Apache-2.0, ~15★)

**Verdict: nearly stale.**

- 1 commit in 2 weeks: SQLite dep bump.
- Open issue #60 — social-login region parsing bug; unattended for 24h+ as of 2026-05-12.
- Release cadence paused after v0.1.0.
- Maintainer may be on break.

**Competitive impact for kiroxy: none.** Our kirocc grafts (`reqconv`, `respconv`, `kiroproto`, tracing) are stable; no upstream drift to worry about.

## 6. hnewcity/KiroaaS (Rust+TS+Py, AGPL-3.0, 96★)

**Verdict: big architectural move — vendored jwadow's python-backend.**

- Commit `85ec9ef` (2026-05-11): **+29,553 / -4,624**. Not a refactor — a vendor-bump that imports jwadow's entire `python-backend/kiro/` package:
  - `account_manager.py` (826 lines).
  - MCP tools emulation.
  - Payload guards.
  - Test suite.
- UI shifted: `AccountCard` component replaced with `ModelsCard`.
- Added i18n to frontend.
- Release builds had an app-data-directory bug, fixed 2026-05-12.

**Competitive impact for kiroxy:**
- Confirms the **Python ecosystem moves as one bloc** — jwadow is the de-facto upstream for `KiroaaS`. Our Go-side isolation is strategically correct.
- jwadow's `account_manager.py` is 826 lines and now widely deployed. If we ever want to cross-reference upstream multi-account semantics, that's the file.

---

## Three Actionable Items for kiroxy (falling out of this delta)

### 1. `*.kiro.dev` endpoint migration (URGENT — 3-day window)

AWS is deprecating `q.<region>.amazonaws.com` in favor of `*.kiro.dev` endpoints. **Deadline: 2026-05-15.** jwadow issue #146 tracks this. Verify kiroxy's `internal/kiroclient` + `internal/auth` use the new endpoint set. If we still resolve to `q.<region>.amazonaws.com`, ship a patch before 2026-05-15.

### 2. `contextUsageEvent` parsing for accurate `input_tokens`

Quorinex PR #37 replaces estimator with upstream-reported counts. Our estimator is now the weakest position in the Go ecosystem. **Promote to P0 for v0.4.0 or v1.0.0.** Implementation: extend `internal/kiroproto/eventstream.go` to recognize the `contextUsageEvent` frame and thread the count through `internal/respconv/usage.go`.

### 3. Claude top-level `thinking` config routing

Quorinex PR #40. Becoming table stakes. **Add to P1 for v1.0.0 or v1.1.0.** Implementation: extend `internal/reqconv/build_payload.go` to detect top-level `thinking: {...}` and route accordingly (Kiro-side thinking mode selection).

---

## What did NOT change

- No new Kiro-specific proxy project appeared in the window.
- Quorinex's `handler.go` god-file (89 KB) remains unchanged — the v1 recommendation to replace it with kirocc's reqconv/respconv still stands.
- jwadow's AGPL-3.0 license unchanged; still blocks our foundation adoption.
- hexos license (MIT) unchanged; salvage plan stands.
- kirocc's single-user-by-design posture unchanged.

---
_Compiled 2026-05-12 Asia/Makassar. Sources: GitHub API commits/issues/PRs, all cross-verified with commit SHAs. Some fetches hit API rate limits; agent used partial data but cross-checked with release notes._
