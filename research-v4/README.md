# research-v4 — kiroxy Operational Depth & Production Readiness

> **Layer 4** of the kiroxy research stack. Complementary to v1/v2/v3, not
> duplicative. Purpose: derisk production ship + produce concrete operator
> playbooks so "ship to friends" becomes provable, not subjective.
>
> Compiled overnight 2026-05-13 over an 8-hour autonomous session.
> All factual claims are cited inline to URL, commit SHA, file:line, or
> peer issue number. Dead-ends are documented as such, not guessed around.

---

## What each layer covers

| Layer | Focus | Target reader | Outcome |
|---|---|---|---|
| `research/` (v1) | Competitive dossiers on 8 Kiro proxy peers | Product lead | Positioning map |
| `research-v2/` | Tier 2 gateway delta + gap matrix | Product lead | v1.0.0 feature priorities |
| `research-v3/` | UX/visual reference gallery | Design lead | v2.0 design system tokens |
| `research-v4/` | **Protocol + Ops + Failures + Ecosystem + Security + Future + Readiness** | **Operator + SRE** | **Ship-to-friends checklist** |

v4 is for the operator who will run kiroxy in production and for the
contributor who will fix the next bug. It is NOT competitive analysis or
UX work.

## Contents

| File | Topic | Lines | What you get |
|---|---|---:|---|
| [PROTOCOL.md](./PROTOCOL.md) | Kiro / CodeWhisperer wire protocol reference | ~1000 | Every endpoint, header, body field, event type, error shape — cited to peer source |
| [OPERATIONS.md](./OPERATIONS.md) | Self-host deployment playbook | ~800 | Homelab + cloud + private-network recipes, runbooks, backup, monitoring |
| [FAILURES.md](./FAILURES.md) | Peer-documented failure catalog | ~700 | 40+ production failures with symptoms, root causes, and kiroxy mitigation status |
| [ECOSYSTEM.md](./ECOSYSTEM.md) | Downstream client expectations | ~500 | How claude-code / Cursor / Cline / opencode / aider expect a proxy to behave |
| [SECURITY.md](./SECURITY.md) | Peer security audit + kiroxy audit | ~500 | Secret handling, network posture, input validation, recommended checks |
| [FUTURE.md](./FUTURE.md) | Kiro / AWS roadmap signals | ~500 | Endpoint migration patterns, model trajectory, policy risks, 6-month checklist |
| [READINESS.md](./READINESS.md) | Production-readiness audit + ship-to-friends checklist | ~450 | Binary checklist operator can run through |

## How to use

**If you're the operator shipping kiroxy:**
1. Read `READINESS.md` first. Work the ship-to-friends checklist.
2. Fall back to `OPERATIONS.md` for deployment questions.
3. Reach for `FAILURES.md` when something breaks.

**If you're a contributor fixing a bug:**
1. Check `FAILURES.md` first — your bug may already be catalogued with
   peer evidence.
2. Consult `PROTOCOL.md` to understand the wire layer before guessing.
3. Check `ECOSYSTEM.md` if the bug is client-specific.

**If you're planning v1.1+:**
1. `FUTURE.md` tells you what AWS is shipping next.
2. `SECURITY.md` tells you what to audit before v1.1 GA.
3. `READINESS.md` tells you which checklist items still fail today.

## Method

- 9 parallel librarian/explore subagents ran during first 30 min.
- Every peer project read at source level, not marketing level:
  jwadow/kiro-gateway, Quorinex/Kiro-Go, AntiHub-Project/Antigv-plugin,
  petehsu/KiroProxy, hj01857655/kiro-account-manager, d-kuro/kirocc,
  diegosouzapw/OmniRoute, decolua/9router, aliom-v/KiroaaS,
  caidaoli/kiro2api, BerriAI/litellm, Portkey-AI/gateway.
- Every claim cited or marked as speculation.
- Every finding cross-referenced to kiroxy's current state (mitigated,
  partially mitigated, or tracked in BACKLOG).

## Companion docs in repo root

- `BACKLOG.md` — post-MVP items, some surfaced by this research
- `OVERNIGHT_LOG.md` — session-by-session build log
- `BUILD_LOG.md` — phase-by-phase engineering decisions
- `docs/ARCHITECTURE.md` — internal design
- `docs/TROUBLESHOOTING.md` — operator-facing diagnostics (complements FAILURES.md)
- `docs/METRICS.md` — Prometheus metric reference (complements OPERATIONS.md)

## Anti-scope

This layer explicitly does NOT cover:
- Competitive positioning (see research-v2/)
- UX/visual design (see research-v3/)
- Feature prioritization (see BACKLOG.md + CHANGELOG.md)
- Marketing copy of any kind

---

*Compiled 2026-05-13 Asia/Makassar during Phase S autonomous session.*
