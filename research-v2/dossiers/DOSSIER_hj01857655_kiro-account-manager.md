# Dossier: hj01857655/kiro-account-manager

> Competitive analysis for kiroxy (Go Kiro proxy). Focus: UI/UX patterns and account lifecycle UX we can reuse — NOT code (different language, different threat model, incompatible license).
> Compiled 2026-05-12 against branch `public` (default). Latest release v1.8.6 (2026-05-10). `package.json` at HEAD is already 1.8.7.

---

## 1. Identity

| Field | Value |
|---|---|
| Repo | [hj01857655/kiro-account-manager](https://github.com/hj01857655/kiro-account-manager) |
| Stars / Forks / Watchers | **1.6k ⭐ · 273 🍴 · 7 👀** |
| Languages | Rust 51.0% · TypeScript 47.7% |
| Created | 2025-12-09 |
| Last push | 2026-05-12 (v1.8.7 in package.json, v1.8.6 released) |
| Releases | 30 total, monthly cadence |
| Open issues / PRs | 4 open issues · 1 open PR (`#84 Anthropic tool-use streaming fix`) |
| **License** | **CC BY-NC-SA 4.0** — NonCommercial + ShareAlike ([LICENSE](https://github.com/hj01857655/kiro-account-manager/blob/public/LICENSE)) |
| Activity signal | Very active (883 commits on `public` branch; multiple fixes per week) |
| UI language | **Simplified Chinese only** (README banner: "本项目仅支持简体中文界面") |

### License gotcha for kiroxy
`CC BY-NC-SA 4.0` is **unusable for a commercial/self-hosted network service**. We must treat this repo as **pattern/reference only** — no code lifting, no direct UI port, no copy-paste of strings. Even a translated screenshot reuse would trigger ShareAlike + NonCommercial. HALL_OF_SHAME.md references "MIT" but the repo itself is CC BY-NC-SA (they seem inconsistent — treat the LICENSE file as authoritative).

---

## 2. Scope

This is a **hybrid** project, not a pure desktop manager:
- **Desktop account manager** (primary) — Tauri app, multi-account table/card UI
- **Network proxy / API gateway** (co-located inside the same binary) — the `gateway/` module under `src-tauri/src/gateway` serves `/v1/messages`, `/v1/responses`, `/v1/chat/completions`, `/v1/models`, `/mcp` on `127.0.0.1:8765` by default

So it overlaps with kiroxy's territory more than the name suggests. kiroxy is Go + server-first; this is Rust + desktop-first **with** a built-in gateway. Key implication: its dashboard doesn't have to be a standalone webapp — it's a desktop window that talks to the gateway over Tauri IPC.

---

## 3. Architecture

**Stack** (verified from [`package.json`](https://github.com/hj01857655/kiro-account-manager/blob/public/package.json) and [`src-tauri/Cargo.toml`](https://github.com/hj01857655/kiro-account-manager/blob/public/src-tauri/Cargo.toml)):

Frontend:
- **React 18.2** + **TypeScript 5.9** + **Vite 5** (not Vue, not Svelte, not Solid — confirmed via `@vitejs/plugin-react`, `react`, `react-dom` deps and `.tsx` files like [`App.tsx`](https://github.com/hj01857655/kiro-account-manager/blob/public/src/App.tsx))
- **shadcn/ui + Radix UI** (full set: dialog, dropdown-menu, popover, progress, select, switch, tabs, tooltip, accordion, scroll-area, etc.)
- **TailwindCSS 4.0** via `@tailwindcss/vite`
- **lucide-react** for icons
- **react-hot-toast** + **sonner** for toasts (two toast libs coexist — likely legacy migration)
- **i18next** + **react-i18next** + **@lingui/cli** (yes — despite Chinese-only UI, i18n scaffolding is in place; see [`src/i18n.tsx`](https://github.com/hj01857655/kiro-account-manager/blob/public/src/i18n.tsx))
- **@tanstack/react-virtual** (row virtualization — important for large account lists)
- **next-themes** (theme provider; four themes per README)
- **cmdk** (command palette)
- **No charting library in deps** — all charts (`QuotaPieChart`, `UsageTrendChart` under `src/components/features/Home/`) are hand-rolled SVG

Desktop shell:
- **Tauri 2.x** with plugins: `tray-icon`, `updater`, `deep-link`, `single-instance`, `dialog`, `fs`, `http`, `opener`, `process`, `shell`, `log`
- Deep-link scheme `kiro://` for OAuth callback (see [`tauri.conf.json`](https://github.com/hj01857655/kiro-account-manager/blob/public/src-tauri/tauri.conf.json))
- CSP locked down: `default-src 'self'`, `frame-ancestors 'none'`, `connect-src` whitelists only `self`, `ipc:`, `http://ipc.localhost`, and the marketing site
- Window: 1400×820, min 1200×700, center, decorations, shadow

Rust backend (`src-tauri/src/`):
- `auth/` — OAuth flow (social + IdC), PKCE, deep-link callback
- `clients/` — HTTP client, Kiro service client
- `commands/` — Tauri IPC handlers (~20+ commands: `account_cmd.rs`, `auth_cmd.rs`, `gateway_cmd.rs`, `group_tag_cmd.rs`, `hooks_cmd.rs`, `mcp_cmd.rs`, `skills_cmd.rs`, `powers_cmd.rs`, `steering_cmd.rs`, `custom_agents_cmd.rs`, `proxy_cmd.rs`, `kiro_cli_cmd.rs`, `session_manager.rs`)
- `core/` — `account.rs` (data model + store), `auto_switch.rs`, `deep_link_handler.rs`, `protocol_registry.rs`
- `gateway/` — the full OpenAI/Anthropic-compatible proxy (router, proxy, converter, eventstream, stream, thinking_parser, load_balancer, token_cache, models)
- `kiro/`, `models/`, `services/session_storage.rs`, `tasks/`, `utils/`
- HTTP stack: `axum 0.8` + `reqwest 0.12` + `tokio-stream` (streaming SSE)
- DB: `rusqlite 0.32 bundled` (so SQLite is used, at least for kiro-cli import and session storage — see below)
- Crypto surface: `sha1`, `sha2`, `base64`, `hex`, `ciborium`, `serde_cbor`, `urlencoding` — **no `ring`, no `age`, no `rust-crypto`, no `keyring`, no `secret-service`**. Storage is plain JSON on disk (confirmed below).

---

## 4. Auth methods supported

Four providers, surfaced in one Login screen and also via the Import modal ([`src/components/features/Login/index.tsx`](https://github.com/hj01857655/kiro-account-manager/blob/public/src/components/features/Login/index.tsx)):

| Provider | Type | Notes |
|---|---|---|
| **Google** | Social OAuth | PKCE (S256) + code verifier |
| **GitHub** | Social OAuth | Same PKCE flow |
| **BuilderId** | IdC / AWS SSO OIDC | AWS IAM Identity Center, default region, no start URL |
| **Enterprise** | IdC / AWS SSO OIDC | Requires user-entered **Start URL** (e.g. `https://d-1234567890.awsapps.com/start`) and region (default `us-east-1`), shown in a dedicated modal |

All four flows land on the same backend router (`commands/auth_cmd.rs::kiro_login`) which dispatches to `login_social` or `login_idc` based on `ProviderConfig.auth_method`.

**Social flow** ([`auth_cmd.rs:login_social`](https://github.com/hj01857655/kiro-account-manager/blob/public/src-tauri/src/commands/auth_cmd.rs)):
1. `ensure_protocol_registration()` — registers `kiro://` deep-link on every login (defensive against multi-install drift)
2. Generate PKCE code verifier (32 random bytes, base64url) + SHA256 challenge ([`auth_social.rs`](https://github.com/hj01857655/kiro-account-manager/blob/public/src-tauri/src/auth/auth_social.rs))
3. `KiroAuthServiceClient.login()` opens system browser
4. Register a `DeepLinkCallbackWaiter` keyed on `state`, block until callback or timeout
5. Exchange code → tokens via `POST https://prod.us-east-1.auth.desktop.kiro.dev/oauth/token`
6. Fetch usage → banned-check → store account → emit `login-success` Tauri event
7. Frontend listens on `login-success` and auto-navigates to the accounts page

**Manual import** ([`ImportAccountModal.tsx`](https://github.com/hj01857655/kiro-account-manager/blob/public/src/components/features/AccountManager/ImportAccountModal.tsx)) has **four tabs**:
1. **JSON paste/upload** — array of objects with `refreshToken` (must start with `aor`), optional `provider`, `clientId/clientSecret` (IdC), `startUrl+region` (Enterprise); has ready-made templates for Social / BuilderId / Enterprise
2. **Kiro IDE** — auto-reads `~/.aws/sso/cache/kiro-auth-token.json` via `read_kiro_accounts` command, shows preview list, batch-imports
3. **kiro-cli** — reads a SQLite database (`.sqlite3`/`.db`) from the kiro-cli install dir; has auto-detect path plus manual browse; note: on Windows it warns explicitly that kiro-cli SQLite is not available there
4. (tab slot left empty in the grid for future)

**No onboarding automation.** No Playwright, no Puppeteer, no Camoufox, no browser-context injection. It's purely `open::that(url)` → OS browser → deep-link back. This is worth noting for kiroxy: they punted on the browser-automation UX entirely.

---

## 5. Token refresh strategy

Three layers:

1. **App-level auto-refresh** — settings toggle "Token 自动刷新" (README §⚙️ 系统设置). Backend runs on a timer; frontend no longer listens to `settings-changed`/`app-settings-changed` (see [`App.tsx`](https://github.com/hj01857655/kiro-account-manager/blob/public/src/App.tsx) comment: `后端自动运行，前端无需监听 settings-changed 和 app-settings-changed`).
2. **Manual per-account refresh** — the 🔑 Key button on each card calls `refresh_account_token`. Separate from the 🔄 RefreshCcw button which is "refresh quota/status only" (no token round-trip).
3. **Desktop refresh endpoint** — unified endpoint `POST {DESKTOP_AUTH_API}/refreshToken` ([`auth.rs:refresh_token_desktop`](https://github.com/hj01857655/kiro-account-manager/blob/public/src-tauri/src/auth/auth.rs)) with 3-retry, 1s backoff. 401 → maps to `AUTH_ERROR:` prefix → frontend silently marks account `invalid` (no modal spam), see [`index.tsx` AccountManager handleRefreshToken](https://github.com/hj01857655/kiro-account-manager/blob/public/src/components/features/AccountManager/index.tsx).

**Gateway-level refresh** — gateway has its own `TokenCache` ([`src-tauri/src/gateway/token_cache.rs`](https://github.com/hj01857655/kiro-account-manager/blob/public/src-tauri/src/gateway/)), separate concern from the UI layer.

**Failure-counter-based auto-disable** — `Account` carries `failure_count`, `last_failure_at`, `disabled_reason` ([`core/account.rs`](https://github.com/hj01857655/kiro-account-manager/blob/public/src-tauri/src/core/account.rs)). `is_available()` returns false if any of: capped, banned, invalid, expired, `disabled_reason.is_some()`. A `success_count` counter exists for the `balanced` load-balancer strategy.

---

## 6. Account lifecycle UX

Main page is [`AccountManager/index.tsx`](https://github.com/hj01857655/kiro-account-manager/blob/public/src/components/features/AccountManager/index.tsx). The flow, as a user experiences it:

**Add**
- Top-right toolbar → Upload icon → Import modal (4 tabs above). OR: sidebar → Desktop OAuth → pick a provider → browser opens → deep-link returns → auto-lands on Accounts.
- Empty state renders a centered cloud icon + "还没有账号" + a gradient "导入账号" CTA (see empty-state JSX in `index.tsx` — nice pattern, reuse-worthy).

**Label / Group / Tag**
- Each card shows inline badges: plan badge (Free/Pro/Enterprise, color-coded), provider badge (Google/GitHub/BuilderId/Enterprise), group chip (custom color from group def), tag pills.
- Edit remark = ✎ Edit2 icon on card → `EditAccountModal`.
- Batch edit tags+groups for N selected = top-bar "批量编辑 (N)" button → `BatchEditModal`.
- Tags/Groups are first-class entities with their own color field, managed in `GroupTagManager.tsx`. Relational link stored as `tagLinks: [{ tagId, tagName, linkedAt }]` — they keep denormalized `tagName` for offline rendering robustness.

**Verify**
- Refresh quota button (🔄 RefreshCcw) per card triggers `list_available_models` + usage fetch, surfaces errors inline on the card (`account.lastError` → "❌ ..." red banner under the progress bar).
- Network/timeout errors get a **multi-line formatted toast** with possible causes and steps (see `handleRefreshWithNotify` in `index.tsx`). Copy this idiom for kiroxy's dashboard.

**Disable / Soft remove**
- No explicit "disable" toggle; the app does it implicitly via status (`banned`, `invalid`, `expired`, `capped`) and by the `disabledReason` field. If you want to skip an account without deleting, you mark/untag it; the gateway pool reads `is_available()`.

**Remove (two modes)**
1. **Local delete** (`delete_account`) — removes from local JSON store only. Has **anti-foot-gun**: if the account being deleted is the currently-logged-in Kiro IDE account, it pops an extra red-tinted confirm: "⚠️ 您正在删除当前使用的账号！删除后 Kiro IDE 将无法使用" (see `handleDelete` in `index.tsx`). Same check exists for batch delete.
2. **Remote delete** (`delete_account_remote`) — `DELETE {DESKTOP_AUTH_API}/account` with the account's own bearer token ([`auth.rs:delete_account_desktop`](https://github.com/hj01857655/kiro-account-manager/blob/public/src-tauri/src/auth/auth.rs)). Irreversible. Confirmation text is blunt: "远程删除将从 AWS 服务端注销此账号！此操作不可恢复，账号将永久失效。"

**Error surfacing idioms (catalog these)**
- **Toast** (`react-hot-toast`, positioned top-center with 80px offset) for ephemeral success/failure.
- **Dialog/modal** (`DialogContext.showError/showInfo/showConfirm`) for anything the user must acknowledge.
- **Inline card badge** (red pill, "❌ {message}") for per-account persistent state.
- **Backend → frontend events** via Tauri `emit`/`listen`: `login-success`, `account-banned`, `account-token-invalid`, `sync-network-error` with typed payloads (see `App.tsx` listeners). Error toasts triggered from these, never from polled state.
- **AUTH_ERROR is silent** — any error string containing `AUTH_ERROR` is logged to console and silently marks the account. No popup. This is a smart anti-spam choice.

---

## 7. Dashboard layout (Home screen)

[`src/components/features/Home/index.tsx`](https://github.com/hj01857655/kiro-account-manager/blob/public/src/components/features/Home/index.tsx) is small and composes 8 sub-components (each in its own file):

Vertical layout, scrollable, two bg-glow decorations behind:

1. **Hero**: 12×12 gradient icon + `t('home.title')` + subtitle
2. **Stat strip**: `grid-cols-5` of `StatCard` tiles: Users (Total accounts), Shield (Active / Unavailable ratio), Zap (Pro+ plans), TrendingUp (usage %), Server (MCP tool count, warns if >50, clickable → KiroConfig page)
3. **Two-column row**: `CurrentAccountCard` (shows who Kiro IDE is currently logged in as + refresh button) | `QuotaOverviewCard` (aggregate across all accounts)
4. **Current-account detail** (`AccountQuotaDetail`) — only renders if `localToken && currentAccount` — shows the breakdown for the currently-active account with a refresh button
5. **Usage distribution bar** (`UsageDistribution`) — shown only if `tokens.length > 0`
6. **Two-column row**: `QuotaPieChart` | `UsageTrendChart` — both hand-rolled SVG, no recharts/chart.js/d3

Design language notes:
- Heavy use of `glass-card` / `glass-main` utility classes (backdrop-blur cards)
- Gradient backgrounds (`bg-gradient-to-br from-primary to-primary/80`) with colored drop-shadow (`shadow-primary/20`)
- `animate-float`, `animate-bounce-in`, `animate-stagger` (staggered delay-100/200/300/400/500) — this is a lot of motion. Kiroxy should probably tune down.
- Theme accent via `getThemeAccent(theme)` returns gradientFrom/gradientTo/shadow/text/ring/iconBadgeBg for each of 4 themes — keep the pattern (centralized accent map) but don't copy the values.

For kiroxy dashboard v2: this is a **much better reference** than the typical shadcn dashboard starter because every tile is domain-specific (quota, MCP tools, token expiry countdown) rather than generic CRUD.

---

## 8. Account health indicators

Status enum ([`src/utils/accountStatus.ts`](https://github.com/hj01857655/kiro-account-manager/blob/public/src/utils/accountStatus.ts) + [`core/account.rs::is_unavailable_status`](https://github.com/hj01857655/kiro-account-manager/blob/public/src-tauri/src/core/account.rs)):

| Normalized key | Tone | Label | Trigger |
|---|---|---|---|
| `active` | success (green) | 正常 | Status ok AND not capped |
| `capped` | warning (orange) | 封顶 | `currentUsage >= usageLimit` AND `overageStatus == DISABLED` |
| `banned` | danger (red) | 封禁 | Backend sets status `banned` / body contains `BANNED` |
| `invalid` | warning (orange) | 失效 | 401 from refresh → `AUTH_ERROR` |
| `expired` | warning (orange) | 过期 | Token expiry passed |
| `unknown` | warning | (raw label) | Fallback |

So it's **4-color** (green / orange / red / grey-fallback), not 3-color traffic-light. The `capped` state is a distinct concept from `banned` and is interesting: it's derived on the fly from `usageBreakdownList[0]` rather than stored.

Card visuals:
- Top-left green gradient bar if current account
- Border color swap: primary if selected → green if current → red if banned → orange if anything non-normal → default
- Status pill top-right with border + tinted background
- Progress bar color thresholds: `>80% red, >50% orange, else green`
- Overage warning line "⚡ 超额: {amount} ${charge}" if `currentOverages > 0`
- Token expiry countdown with a red bold pill if past expiry: `⚠️ Token: 05-12 14:30`
- Next reset date shown next to it: `05-15重置`
- `lastError` persistent red line under the progress
- Machine ID badge with copy button (truncated `abc12345...xxxx`)

**Rate-limit warning pattern**: not a traffic-light; it's an overage-charges line + capped-state status pill. Kiroxy could pick the "capped distinct from banned" idea.

---

## 9. Batch operations

Every batch op is gated by `selectedCount > 0` which switches the top header from "accounts title" to "已选中 N 个账号":

- **Bulk import** — 4-tab modal above, concurrent imports with `runConcurrent(items, handler, onProgress)` using `getConcurrency(items.length)` (adaptive concurrency per list size). Progress bar + current/total counter in a card.
- **Bulk refresh** — `batchRefreshAccounts(selectedIds, accounts)` with `autoRefreshing` flag + `refreshProgress.{current,total}` exposed for a 1.5px thin bar at the bottom of the header while running.
- **Bulk delete** — `delete_accounts(ids)`. Same "includes-current-account" foot-gun guard as single delete.
- **Bulk tag/group edit** — `BatchEditModal` for N selected.
- **Bulk export** — JSON download of selected only (`onExport(selectedIds)`).
- **Bulk remote logout** — per README, "批量远程注销", but in the current UI I see this per-card via context menu, not as a top-bar bulk action.

Refresh progress lives in `AccountHeader.tsx` as a gradient-fill 1.5px bar. Small, unobtrusive — good pattern for long-running work.

---

## 10. Export / backup

`handleExport(selectedIds)` → Tauri command → writes a JSON file via the fs plugin. **Plaintext JSON**. No age, no encryption, no passphrase prompt, no Keychain.

The exported shape matches the import JSON templates (refreshToken, provider, clientId/clientSecret/region/startUrl, machineId, accessToken). Import is idempotent (dedupes by `user_id`, falls back to email+provider+refreshToken match).

No backup/restore UI as a separate feature — export IS the backup mechanism.

---

## 11. Secret storage

**Plain JSON on disk.** Not encrypted. Not Keychain. Not age. Not argon2+aes.

Storage location ([`core/account.rs::AccountStore::get_storage_path`](https://github.com/hj01857655/kiro-account-manager/blob/public/src-tauri/src/core/account.rs)):

```
$DATA_DIR/.kiro-account-manager/accounts.json
// macOS: ~/Library/Application Support/.kiro-account-manager/accounts.json
// Linux: ~/.local/share/.kiro-account-manager/accounts.json
// Windows: %APPDATA%/.kiro-account-manager/accounts.json
```

Serialized via `serde_json::to_string_pretty`. Sibling files: `groups-tags.json`, `gateway-config.json`, `logs/gateway-request-log.jsonl`.

Gateway client API keys stored the same way, in `gateway-config.json`. They support a `#disabled#` prefix convention to soft-disable a key without deleting it (see `effective_client_api_keys` in [`gateway/mod.rs`](https://github.com/hj01857655/kiro-account-manager/blob/public/src-tauri/src/gateway/mod.rs)).

SQLite (rusqlite bundled) is used but only for **session storage** (`services/session_storage.rs`) and **kiro-cli import** (reading kiro-cli's own SQLite DB). Not for account tokens.

**Implication for kiroxy's G.2 item (age vs Keychain)**: this project offers no prior art that solves the problem — they simply shipped plaintext. If kiroxy wants credentials encryption, we're the ones defining that best practice for this ecosystem. Keychain is the "right" answer on desktop; for a server-side process like kiroxy, age with a passphrase or a detached keyfile is the saner pick.

The CSP in `tauri.conf.json` is strict (no inline script outside `'unsafe-inline'`, no eval), so at the app boundary they're disciplined — but disk-at-rest is wide open. Anyone with read access to the user profile can exfiltrate every token.

---

## 12. Onboarding automation

None. They do not drive a headless browser. The flow is:

1. `open::that(authorize_url)` opens the system browser
2. User authorizes
3. Provider redirects to `kiro://callback?code=...&state=...`
4. Tauri single-instance + deep-link plugins route that back into the running app
5. A waiter keyed by `state` unblocks the pending login in Rust

For Enterprise / IdC, there's an additional device-code-like flow inside `auth::providers::idc_provider.login()` (not fully inspected — lives under `src-tauri/src/auth/providers/`).

So if kiroxy wants to support "the user pastes no tokens, just clicks a button" — this repo is **not** a reference. Our Camoufox-automated onboarding is still the differentiator.

---

## 13. Unique UX patterns worth stealing (or inspiring from)

1. **Dual view toggle (card / table) persisted in localStorage** — `viewMode: 'card' | 'table'` key `accountViewMode`. Card is better for 5-20 accounts, table for 50+. Kiroxy should do both.
2. **Two refresh buttons per account** — separate "refresh token" (Key icon) and "refresh quota" (RefreshCcw icon). Users' mental model is different; don't overload one button.
3. **Anti-foot-gun confirmations** that detect the currently-active account and render a stronger warning. A small effort, huge quality-of-life.
4. **Routed sidebar with per-route persistence** — [`routes.tsx`](https://github.com/hj01857655/kiro-account-manager/blob/public/src/routes.tsx) uses `React.lazy` + a `mountedRouteIds` list. Heavy pages stay mounted after first visit (display:none swap), light pages unmount. See `shouldPersistRoute` and `getMountedRouteIds` in `src/utils/routePersistence.ts`. Good trick for dashboard tabs that have live data.
5. **Backend-pushed events** instead of polling: `account-banned`, `account-token-invalid`, `sync-network-error` with counts. Kiroxy's Go server should push these via SSE/WebSocket to the dashboard.
6. **Multi-line formatted error toasts** with bulleted causes + numbered remediation steps. Much better than a one-liner.
7. **Typed status normalizer** that accepts either a raw string OR a full Account object — reduces bugs at call sites.
8. **Gradient accent per theme** centralized in `getThemeAccent(theme)` — one source of truth for colors.
9. **"封禁" (banned) vs "封顶" (capped) distinction** — don't collapse these into one "unavailable" state; users need to know if it's quota or account-level.
10. **Four-tab Import modal** — JSON / Kiro IDE cache / kiro-cli SQLite / (reserved). Copy the tab structure, and in kiroxy's case: API / CLI token / kiro-auth-token.json / future Camoufox.
11. **Group+Tag as separate entities with color picker**, not just string tags. Groups are mutually exclusive (one per account); tags are many-to-many. This maps to real ops patterns ("rotation pool A" vs "test" vs "prod").
12. **The `#disabled#` prefix convention for API keys** — lets users disable a gateway API key without losing it. Simple idiom worth adopting.
13. **Hand-rolled SVG charts** — kept bundle small. Given kiroxy will likely run in a browser, we can afford a chart lib, but look at `QuotaPieChart` / `UsageTrendChart` as a taste reference for scale (small, clean, no overkill).
14. **Context menu on right-click a card** (`ContextMenu.tsx`) — switch, refresh, edit, delete, all in one place without crowding the card surface.
15. **`react-virtual` for the table view** — they anticipate hundreds of accounts. Do the same.
16. **MCP tool count on the dashboard with a `>50` warning** — novel; exposes an ecosystem-level gotcha in the UI.

---

## 14. Known weaknesses (from last 20 issues — all are in Chinese; translated to key takeaways)

Read issues #70, #67, #81, #82, #83, #85, #86, #87, #88, #89, #90 (plus PR #84 and older). Themes:

- **Gateway tool-call bugs dominate recent issues** — #82/#83/#84/#90 are all about `/v1/messages`, `/v1/responses`, `/v1/chat/completions` streaming tool-call format mismatches. They're playing whack-a-mole with spec compliance across Anthropic / OpenAI Responses / Chat Completions. (Direct kiroxy takeaway: our converter needs a solid SSE event conformance test matrix on day one.)
- **Image handling is inconsistent per endpoint** (#82): `/v1/messages` loses images; `/v1/responses` works; `/v1/chat/completions` works.
- **Compact/context-compression also inconsistent per endpoint** (#82).
- **v1.8.7 regression: Social login (GitHub) no longer adds account to KAM** (#81). The user observed "老版本可以" (older versions worked) — old classic.
- **Switch-account UX confusion when account has 0 quota** (#86): "怎么切换账号？？？" — the switch UI didn't gracefully handle out-of-quota state. Closed as (presumably) a docs/explanation issue, three angry question marks say it's a real UX gap.
- **LAN-access reverse-proxy returning empty body** (#87): user enabled "allow remote access", set allowlist, but GETs to `192.168.1.10:8765/v1/models` return "Empty reply from server". User also pointed out a copy bug: "当前入口一直是 http://localhost:8765" even when remote-access is on — the UI lies about the binding. Shipped fix same day.
- **Switched account reports `Access denied. Please check your authentication.`** (#85): user hit 50-point accounts, new accounts after switch can't be used, but 550-point-remaining account still works. Hints at machine_id binding / rotation bugs.
- **Kiro IDE cache JSON format drift breaking import** (#85): when the user selects `.aws/sso/cache/*.json` directly, schema mismatch. They only support their specific shape.
- **ARM / Apple Silicon / Linux ARM builds missing** (#88, #89) — closed quickly as "not in current build matrix" / "no hardware".
- **macOS style glitches** — README explicitly admits the maintainer has no macOS hardware; users must patch themselves.
- **Windows MSI "同一版本已安装"** — fixed in 1.8.3 with covering upgrade, still surfaces in FAQ.
- **License drift** — HALL_OF_SHAME.md claims MIT but LICENSE is CC BY-NC-SA 4.0. Someone forked and stripped attribution + personally attacked the author.

Overall: the core desktop UX is stable. The **gateway layer** is fragile and is where they bleed time. A proxy-first project like kiroxy would inherit exactly those same bugs if it reimplemented naively — this repo's PR history (#67, #70, #84) is a free regression-test checklist.

---

## 15. What kiroxy can learn (ranked)

**High-signal for Dashboard v2 (P3)**:
1. **Steal the Home layout structure**: Hero → 5-tile stat strip → current-account+quota-overview row → per-account detail → distribution bar → two-chart row. Each in its own component file. Our Go backend already has the data points (request counts, account list, quotas) — this is purely a frontend reorganization.
2. **Stack choice**: React 18 + Vite + Tailwind 4 + shadcn/ui + Radix is the lowest-risk path. Don't pick Vue/Svelte just to be different.
3. **Backend-pushed events via SSE/WebSocket** (they use Tauri events; we'd use SSE): `account-banned`, `token-invalid`, `gateway-error`, `quota-warning`. Keeps the dashboard reactive without polling.
4. **Card + Table dual view persisted in localStorage** with `@tanstack/react-virtual` on the table for large pools.
5. **Two separate refresh actions per account**: one for token, one for quota. Different icons, different tooltip.
6. **Status taxonomy** with 4+ distinct states (`active`, `capped`, `banned`, `invalid`, `expired`) — not a 3-color traffic light. Give `capped` its own warning tone distinct from `banned`.
7. **Empty-state hero** with CTA button inside the accounts list — tight, inviting, not a blank page.
8. **Four-tab Import modal** shape: JSON / kiro-auth-token.json / kiro-cli SQLite / (slot for Camoufox). Same modal, different data sources.
9. **Tags + Groups as first-class entities** with custom colors (groups = mutually exclusive, tags = many-to-many). Map groups to gateway pool selection.
10. **Anti-foot-gun confirmations** when the action would affect the currently-active account.
11. **The `#disabled#` prefix convention** for API keys — soft-disable without delete.

**For G.2 (credential encryption)**:

12. This repo is NOT a reference — they ship plaintext. Kiroxy should pick age-based encryption (passphrase or detached keyfile) as the default, Keychain as the opt-in on macOS. This is differentiating, not blocked.

**For the gateway side** (not the dossier's primary scope but bleeds over):

13. Their three-endpoint proxy (`/v1/messages`, `/v1/responses`, `/v1/chat/completions`) and their **bug history on tool-call streaming** is a free conformance test matrix. Before shipping kiroxy's converter, build tests that reproduce their #82/#83/#84/#90 bugs against our Go implementation.
14. **TokenCache as a separate module** with its own mutex — avoids contention between refresh loop and request path. Mirror this layering on our Go side.
15. **Load balancer strategies** (`round_robin`, `balanced` with success_count, and the `pool`/`single`/`group` account-mode split) — maps cleanly onto our account-pool backlog.

**Do not copy**:

- The license (kiroxy is commercial-compatible; CC BY-NC-SA 4.0 is not)
- Chinese-only strings (kiroxy should be English-first with optional i18n)
- The "everything in one Tauri desktop binary" architecture (kiroxy is server-first)
- Plaintext JSON for tokens (we must do better)
- Two toast libs (`react-hot-toast` + `sonner`) — pick one
- The amount of card animation (tune it down — a lot of `animate-stagger` / `animate-float` / `animate-bounce-in` stacked)

---

## 16. Screenshots (from README — images present, I did not render them)

README references six PNG/WebP screenshots in [`screenshots/`](https://github.com/hj01857655/kiro-account-manager/tree/public/screenshots):

- `首页.webp` — Home (the Hero + stats + charts page described in §7)
- `账号管理.webp` — AccountManager (cards + header + filter chips described in §6)
- `桌面授权.webp` — Desktop OAuth login screen (the 4 provider buttons + "waiting for authorization" modal described in §4)
- `规则管理.webp` — Rules management (KiroConfig: MCP servers, Steering, Hooks, Skills, Custom Agents, Powers — each in a sub-tab)
- `设置.png` — Settings (4 themes, model lock, agent autonomous mode, token auto-refresh, proxy config)
- `关于.png` — About

Images exist in the repo — I did not binary-fetch them so I can't describe specific pixel details beyond structure. If design reference is needed, either view them on GitHub directly at [this tree](https://github.com/hj01857655/kiro-account-manager/tree/public/screenshots) or have the frontend agent pull them.

---

## 17. Appendix — key file index (for future drill-downs)

- [`README.md`](https://github.com/hj01857655/kiro-account-manager/blob/public/README.md) — feature list, download matrix, screenshots
- [`package.json`](https://github.com/hj01857655/kiro-account-manager/blob/public/package.json) — frontend deps
- [`src-tauri/Cargo.toml`](https://github.com/hj01857655/kiro-account-manager/blob/public/src-tauri/Cargo.toml) — backend deps, profiles
- [`src-tauri/tauri.conf.json`](https://github.com/hj01857655/kiro-account-manager/blob/public/src-tauri/tauri.conf.json) — CSP, deep-link, updater pubkey, bundle targets
- [`src/App.tsx`](https://github.com/hj01857655/kiro-account-manager/blob/public/src/App.tsx) — event wiring, route persistence
- [`src/routes.tsx`](https://github.com/hj01857655/kiro-account-manager/blob/public/src/routes.tsx) — 8 sidebar routes, lazy-loaded
- [`src/components/features/Home/index.tsx`](https://github.com/hj01857655/kiro-account-manager/blob/public/src/components/features/Home/index.tsx) — dashboard layout
- [`src/components/features/AccountManager/index.tsx`](https://github.com/hj01857655/kiro-account-manager/blob/public/src/components/features/AccountManager/index.tsx) — list page
- [`src/components/features/AccountManager/AccountCard.tsx`](https://github.com/hj01857655/kiro-account-manager/blob/public/src/components/features/AccountManager/AccountCard.tsx) — card visuals
- [`src/components/features/AccountManager/ImportAccountModal.tsx`](https://github.com/hj01857655/kiro-account-manager/blob/public/src/components/features/AccountManager/ImportAccountModal.tsx) — 4-tab import
- [`src/components/features/Login/index.tsx`](https://github.com/hj01857655/kiro-account-manager/blob/public/src/components/features/Login/index.tsx) — 4-provider OAuth
- [`src/utils/accountStatus.ts`](https://github.com/hj01857655/kiro-account-manager/blob/public/src/utils/accountStatus.ts) — status taxonomy
- [`src-tauri/src/core/account.rs`](https://github.com/hj01857655/kiro-account-manager/blob/public/src-tauri/src/core/account.rs) — `Account`, `AccountStore`, `GroupTagStore`, normalization + merge, storage paths
- [`src-tauri/src/auth/auth.rs`](https://github.com/hj01857655/kiro-account-manager/blob/public/src-tauri/src/auth/auth.rs) — desktop refresh/delete endpoints
- [`src-tauri/src/auth/auth_social.rs`](https://github.com/hj01857655/kiro-account-manager/blob/public/src-tauri/src/auth/auth_social.rs) — PKCE helpers
- [`src-tauri/src/commands/auth_cmd.rs`](https://github.com/hj01857655/kiro-account-manager/blob/public/src-tauri/src/commands/auth_cmd.rs) — `kiro_login`, `login_social`, `login_idc`, `handle_kiro_social_callback`
- [`src-tauri/src/gateway/mod.rs`](https://github.com/hj01857655/kiro-account-manager/blob/public/src-tauri/src/gateway/mod.rs) — axum router, `GatewayConfig`, `GatewayStatus`, request log JSONL, `#disabled#` key convention

---

_Dossier written for kiroxy research-v2 by the Librarian. Treat as evidence-indexed notes, not a spec._
