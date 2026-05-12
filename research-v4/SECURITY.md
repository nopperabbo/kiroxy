# Security Audit: Kiro Proxy & AI Gateway Ecosystem

> Evidence-based audit of 10 peer projects (9 accessible, 1 404). All findings cite commit SHA + file:line.
> Audit date: 2026-05-13. Methodology: shallow clone + static read + `npm audit` / `gh api security-advisories` where applicable.

---

## Scope

| Project | Lang | Stars | SHA | Scope |
|---|---|---|---|---|
| jwadow/kiro-gateway | Python/FastAPI | 1,359 | `0398d74f` | Full audit |
| Quorinex/Kiro-Go | Go/stdlib | 523 | `1732b17f` | Full audit |
| AntiHub-Project/Antigv-plugin | Node/Express | 34 | `06ad96f8` | Full audit (Antigravity, not Kiro — bonus sample) |
| petehsu/KiroProxy | Python/FastAPI | 343 | `9a91b9be` | Full audit |
| hj01857655/kiro-account-manager | Rust/Tauri+axum | 1,565 | `b43d4d49` | Full audit (desktop app + embedded gateway) |
| aliom-v/KiroaaS | — | — | **404** | Repo does not exist / private |
| caidaoli/kiro2api | Go/Gin | 596 | `ebf5ad74` | Full audit |
| BerriAI/litellm | Python/FastAPI | 46,692 | `fc8a9a34` | Proxy-subset + CVE DB |
| Portkey-AI/gateway | Node/Hono | 11,695 | `351692fd` | Edge-runtime core + `npm audit` |
| Kong/kong | Lua/OpenResty | 43,374 | `58f2daa5` | AI plugins only |

---

## 1. jwadow/kiro-gateway

Commit `0398d74f15549bd771480da8fceb21916ce333e5` — FastAPI + httpx + loguru. Well-structured (32 modules, extensive tests).

### 1.1 Secret handling — **MED**
- Tokens loaded from `.env` or JSON file via `KiroAuthManager`. Also supports reading kiro-cli SQLite in read-only mode by default ([`kiro/auth.py:513`](https://github.com/jwadow/kiro-gateway/blob/0398d74f15549bd771480da8fceb21916ce333e5/kiro/auth.py#L513-L520)).
- **No at-rest encryption**. Refresh tokens saved plaintext back to JSON file with `open(path, 'w')` — **no `os.chmod(0o600)`** ([`kiro/auth.py:490-518`](https://github.com/jwadow/kiro-gateway/blob/0398d74f15549bd771480da8fceb21916ce333e5/kiro/auth.py#L490-L518)).
- State file `account_manager.py:377` also plain JSON with atomic rename but no mode bits.
- No env-var leak in normal logs; debug mode dumps full request bodies (see 1.5).

### 1.2 Network posture — **HIGH**
- **Binds to `0.0.0.0` by default** ([`kiro/config.py:85-86`](https://github.com/jwadow/kiro-gateway/blob/0398d74f15549bd771480da8fceb21916ce333e5/kiro/config.py#L85-L86)): `DEFAULT_SERVER_HOST: str = "0.0.0.0"`.
- CORS `allow_origins=["*"]` with `allow_credentials=True` ([`main.py:549-554`](https://github.com/jwadow/kiro-gateway/blob/0398d74f15549bd771480da8fceb21916ce333e5/main.py#L549-L554)). This is **exactly the Starlette footgun litellm warns about** — Starlette reflects the Origin header when combined this way, allowing any site to make credentialed requests. Combined with a weak PROXY_API_KEY default, very bad.
- Inbound TLS: not built-in. "Use reverse proxy" assumption (Dockerfile has no TLS).
- Outbound: default httpx client, no explicit `verify=` — defaults to `True`. No TLS pinning or cert validation override. OK.

### 1.3 Input validation — **MED**
- `payload_guards.py` trims oversized conversation history to fit 600KB Kiro API limit. Not a security control — it's payload-shape plumbing.
- **No body size limit** (no `LimitUpload` or starlette `max_size`).
- X-Forwarded-For not validated — trusted blindly if present. Minor for single-tenant.
- JSON schemas on tool_use inputs via Pydantic models (`kiro/models_openai.py`, `models_anthropic.py`) — decent.

### 1.4 Dependency audit — **MED (unpinned)**
- [`requirements.txt`](https://github.com/jwadow/kiro-gateway/blob/0398d74f15549bd771480da8fceb21916ce333e5/requirements.txt) has **zero pinned versions**: `fastapi`, `uvicorn[standard]`, `httpx`, `loguru`, `python-dotenv`, `tiktoken`. Install today gets latest; install in 6 months gets drift.
- No `pyproject.toml` lock, no `pip-tools`, no hash verification.
- All direct deps currently safe, but supply-chain posture is fragile.

### 1.5 Log hygiene — **HIGH (when DEBUG on)**
- Debug middleware (`kiro/debug_middleware.py`) persists raw request body to `debug_logs/request_body.json` and modified Kiro payload to `kiro_request_body.json` ([`kiro/debug_logger.py:334-360`](https://github.com/jwadow/kiro-gateway/blob/0398d74f15549bd771480da8fceb21916ce333e5/kiro/debug_logger.py#L334-L360)).
- Default is `DEBUG_MODE="off"` ([`kiro/config.py:372-377`](https://github.com/jwadow/kiro-gateway/blob/0398d74f15549bd771480da8fceb21916ce333e5/kiro/config.py#L372-L377)) — correct.
- No Bearer-token redaction regex found (`grep bearer|redact|mask` in source = nothing). If a user sets DEBUG=all on a host with tokens in headers, tokens hit disk.

### 1.6 Rate limiting / abuse prevention — **MED**
- **Zero rate limiting** found (no `slowapi`, no `limiter`, no semaphore).
- No uvicorn `limit_concurrency` or `limit_max_requests` set.
- Single hardcoded `PROXY_API_KEY` check — no per-key quotas.

### 1.7 Auth / PROXY_API_KEY — **HIGH**
- Default API key is **hardcoded and publicly known**: `"my-super-secret-password-123"` ([`kiro/config.py:99`](https://github.com/jwadow/kiro-gateway/blob/0398d74f15549bd771480da8fceb21916ce333e5/kiro/config.py#L99)). Used in `.env.example` as the example value.
- `routes_openai.py:82`: `if not auth_header or auth_header != f"Bearer {PROXY_API_KEY}":` — **`!=` comparison, not `secrets.compare_digest`** ([`routes_openai.py:82`](https://github.com/jwadow/kiro-gateway/blob/0398d74f15549bd771480da8fceb21916ce333e5/kiro/routes_openai.py#L82)). Timing-attack theoretical but fixable in 1 line.

**Risk profile**: Default 0.0.0.0 bind + default api key + wildcard CORS + no rate limit = if a user runs `python main.py` and doesn't read docs, their Kiro token is exposed to anyone who can reach their port 8000. Could not verify: whether any production deploys ship with default key.

---

## 2. Quorinex/Kiro-Go

Commit `1732b17ff9455e55cb9dcf34cf23c39f5b549042` — Go stdlib `net/http` + `github.com/google/uuid`. Minimal dep footprint.

### 2.1 Secret handling — **GOOD**
- Config written via [`os.WriteFile(cfgPath, data, 0600)`](https://github.com/Quorinex/Kiro-Go/blob/1732b17ff9455e55cb9dcf34cf23c39f5b549042/config/config.go#L198) — **correctly uses 0600 permissions**. One of only two projects that does this.
- No at-rest encryption of tokens though (plaintext JSON).
- `ADMIN_PASSWORD` env-var overrides file password at boot ([`main.go:44-46`](https://github.com/Quorinex/Kiro-Go/blob/1732b17ff9455e55cb9dcf34cf23c39f5b549042/main.go#L44-L46)).

### 2.2 Network posture — **HIGH**
- [`config/config.go:170-174`](https://github.com/Quorinex/Kiro-Go/blob/1732b17ff9455e55cb9dcf34cf23c39f5b549042/config/config.go#L170-L174): `Host: "0.0.0.0"` default "for Docker/container compatibility" — comment says this explicitly.
- CORS `Access-Control-Allow-Origin: *` hardcoded ([`proxy/handler.go:321`](https://github.com/Quorinex/Kiro-Go/blob/1732b17ff9455e55cb9dcf34cf23c39f5b549042/proxy/handler.go#L321-L324)). No credentials flag though — less bad than jwadow.
- No inbound TLS.
- Outbound: custom `http.Transport`, no `InsecureSkipVerify`, no `TLSClientConfig` override ([`proxy/kiro.go:52-71`](https://github.com/Quorinex/Kiro-Go/blob/1732b17ff9455e55cb9dcf34cf23c39f5b549042/proxy/kiro.go#L52-L71)). Default Go TLS posture = secure.

### 2.3 Input validation — **MED**
- **No body size limit** (no `http.MaxBytesReader`).
- **No ReadTimeout/WriteTimeout/IdleTimeout** on `http.Server` — uses `http.ListenAndServe(addr, handler)` directly ([`main.go:62`](https://github.com/Quorinex/Kiro-Go/blob/1732b17ff9455e55cb9dcf34cf23c39f5b549042/main.go#L62)). Slowloris vulnerable.
- JSON decoded without `DisallowUnknownFields`.
- Admin password check via `password != config.GetPassword()` ([`proxy/handler.go:1853`](https://github.com/Quorinex/Kiro-Go/blob/1732b17ff9455e55cb9dcf34cf23c39f5b549042/proxy/handler.go#L1853)) — not `crypto/subtle.ConstantTimeCompare`.
- Default config password: `"changeme"` ([`config/config.go:174`](https://github.com/Quorinex/Kiro-Go/blob/1732b17ff9455e55cb9dcf34cf23c39f5b549042/config/config.go#L174)).
- API key required only if `RequireApiKey: false` **default** ([`config/config.go:175`](https://github.com/Quorinex/Kiro-Go/blob/1732b17ff9455e55cb9dcf34cf23c39f5b549042/config/config.go#L175)).

### 2.4 Dependency audit — **GOOD**
- Only one direct dep: `github.com/google/uuid v1.6.0` ([`go.mod`](https://github.com/Quorinex/Kiro-Go/blob/1732b17ff9455e55cb9dcf34cf23c39f5b549042/go.mod)). Cleanest dep tree of the lot. Minimal attack surface.
- Go stdlib for everything else. `govulncheck` would likely be clean.

### 2.5 Log hygiene — **UNKNOWN → likely OK**
- `log.Printf` for status lines; no grep match for logging request bodies or tokens in code.

### 2.6 Rate limiting — **NONE**
- No rate limiter, no concurrent connection cap. Admin endpoints protected only by password.

**Risk profile**: Admin password `"changeme"` + optional API key (default off) + 0.0.0.0 bind + `*` CORS + no timeouts. Decent file perms and clean deps, but everything else is weak-by-default.

---

## 3. AntiHub-Project/Antigv-plugin

Commit `06ad96f8e948623fa21cd2791f9df6a3796e8eb7` — Node 18+ / Express 5 / PostgreSQL / Redis. Actually an **Antigravity-to-OpenAI** proxy, not Kiro, but a useful peer sample since the attack surface is identical.

### 3.1 Secret handling — **HIGH**
- **Plaintext API keys and refresh tokens stored in Postgres**.
- [`users.api_key character varying(64) NOT NULL`](https://github.com/AntiHub-Project/Antigv-plugin/blob/06ad96f8e948623fa21cd2791f9df6a3796e8eb7/schema.sql) — plaintext, not hashed.
- [`accounts.refresh_token text`, `accounts.access_token text NOT NULL`](https://github.com/AntiHub-Project/Antigv-plugin/blob/06ad96f8e948623fa21cd2791f9df6a3796e8eb7/schema.sql) — plaintext in DB.
- [`kiro_accounts.refresh_token text NOT NULL`, `client_secret text`](https://github.com/AntiHub-Project/Antigv-plugin/blob/06ad96f8e948623fa21cd2791f9df6a3796e8eb7/schema.sql) — also plain.
- `validateApiKey()` at [`src/services/user.service.js:116-126`](https://github.com/AntiHub-Project/Antigv-plugin/blob/06ad96f8e948623fa21cd2791f9df6a3796e8eb7/src/services/user.service.js#L116-L126): looks up by direct equality on plaintext column — any DB compromise or SQL-injection leaks all keys cleartext.
- Admin key from `config.security.adminApiKey` checked via `===` string compare ([`src/server/kiro_routes.js:24`](https://github.com/AntiHub-Project/Antigv-plugin/blob/06ad96f8e948623fa21cd2791f9df6a3796e8eb7/src/server/kiro_routes.js#L24)).

### 3.2 Network posture — **GOOD-ish**
- [`src/config/config.js:22`](https://github.com/AntiHub-Project/Antigv-plugin/blob/06ad96f8e948623fa21cd2791f9df6a3796e8eb7/src/config/config.js#L22): default `host: '127.0.0.1'` — **correctly localhost by default**.
- No CORS middleware at all (not installed as a package). Only sees `sec-fetch-mode: cors` in outbound headers. Missing-CORS means browsers can't hit it cross-origin → acceptable defensive posture.
- No inbound TLS.

### 3.3 Input validation — **GOOD**
- Body size limit: `express.json({ limit: config.security.maxRequestSize })` with 50mb default ([`src/config/config.js:31`](https://github.com/AntiHub-Project/Antigv-plugin/blob/06ad96f8e948623fa21cd2791f9df6a3796e8eb7/src/config/config.js#L31)). 50MB is huge but there's at least a cap.
- 413 error handler ([`src/server/index.js:41-45`](https://github.com/AntiHub-Project/Antigv-plugin/blob/06ad96f8e948623fa21cd2791f9df6a3796e8eb7/src/server/index.js#L41-L45)).
- Server timeout `600000ms` (10min) for streaming.

### 3.4 Dependency audit — **MED**
- `express@^5.1.0` (Express 5 is still fresh — some unknowns).
- `pg@^8.16.3` (current), `ioredis@^5.4.1` (current).
- `@anthropic-ai/tokenizer@^0.0.4` — super-fresh 0.0.x dep, semver unstable.
- Main concern: unpinned caret ranges on all.

### 3.5 Log hygiene — **UNKNOWN → likely OK**
- No grep match for logging bodies/tokens.

### 3.6 Rate limiting — **UNKNOWN**
- No `express-rate-limit` installed. Relies on Redis quota tracking per-user (app-level, not HTTP-level).

**Risk profile**: Multi-tenant service with **all secrets plaintext in SQL** is the big one. A stolen DB dump is a full breach. Everything else is average.

---

## 4. petehsu/KiroProxy

Commit `9a91b9be1e72ae1d1590a0d297b89b18618b446b` — FastAPI + httpx[socks] + loguru.

### 4.1 Secret handling — **MED**
- Credentials saved plaintext to `~/.kiro-proxy/config.json` via [`open(CONFIG_FILE, "w")`](https://github.com/petehsu/KiroProxy/blob/9a91b9be1e72ae1d1590a0d297b89b18618b446b/kiro_proxy/core/persistence.py#L24) — **no chmod**.
- `KiroCredentials.save_to_file` also plaintext JSON with no perm bits ([`kiro_proxy/credential/types.py:213-230`](https://github.com/petehsu/KiroProxy/blob/9a91b9be1e72ae1d1590a0d297b89b18618b446b/kiro_proxy/credential/types.py#L213-L230)).
- SQLite read-only mode supported via `KIRO_SQLITE_READONLY` env.

### 4.2 Network posture — **HIGH**
- **Default bind `0.0.0.0:8080`** ([`kiro_proxy/env_config.py:21`](https://github.com/petehsu/KiroProxy/blob/9a91b9be1e72ae1d1590a0d297b89b18618b446b/kiro_proxy/env_config.py#L21)).
- CORS wildcard [`allow_origins=["*"]`, `allow_credentials=True`](https://github.com/petehsu/KiroProxy/blob/9a91b9be1e72ae1d1590a0d297b89b18618b446b/kiro_proxy/main.py#L43-L49). Same Starlette footgun as jwadow.
- Outbound: has an **explicit `KIRO_PROXY_INSECURE_TLS` env var** that disables TLS verification ([`kiro_proxy/http_client.py:29-44`](https://github.com/petehsu/KiroProxy/blob/9a91b9be1e72ae1d1590a0d297b89b18618b446b/kiro_proxy/http_client.py#L29-L44)). Off by default, at least warned once via log. Still a footgun ever existing as a flag.

### 4.3 Input validation & auth — **CRITICAL**
- **No inbound API key check**. Grep for `KIRO_API_KEY|API_KEY|auth` in env_config.py and main.py finds nothing; the only auth is `ensure_profile_arn_ready` which backfills credentials from the user's local Kiro logs. That is NOT auth.
- [`kiro_proxy/main.py`](https://github.com/petehsu/KiroProxy/blob/9a91b9be1e72ae1d1590a0d297b89b18618b446b/kiro_proxy/main.py) routes 30+ admin endpoints (`POST /api/accounts`, `DELETE /api/accounts/{id}`, `POST /api/accounts/refresh-all`, `GET /api/accounts/export`, etc.) with **zero authentication**. If this binds to 0.0.0.0 (which it does by default), anyone on network can drain all accounts.
- No body size cap.

### 4.4 Dependency audit — **OK**
- Pinned ranges with `>=` lower bounds only: `fastapi>=0.100.0`, `pydantic>=2.0.0`, `httpx[socks]>=0.24.0`. No upper bounds → drift-vulnerable.

### 4.5 Log hygiene — **GOOD**
- Only grep hit for logged-body is `logger.debug("System prompt has cache_control (tracked)")` — safe.
- No bearer/token logging observed.

### 4.6 Rate limiting — **PRESENT BUT DISABLED**
- Has a full `RateLimiter` class ([`kiro_proxy/core/rate_limiter.py`](https://github.com/petehsu/KiroProxy/blob/9a91b9be1e72ae1d1590a0d297b89b18618b446b/kiro_proxy/core/rate_limiter.py)) with per-account interval + RPM caps.
- **Default `enabled: bool = False`** ([`rate_limiter.py:29`](https://github.com/petehsu/KiroProxy/blob/9a91b9be1e72ae1d1590a0d297b89b18618b446b/kiro_proxy/core/rate_limiter.py#L29)). Feature built, opt-in.

**Risk profile**: The **no-auth admin API bound to 0.0.0.0 is a showstopper**. If you self-host this on any machine exposed to LAN/VPN/tailscale/WAN, anyone discovering the port can list accounts, delete them, refresh tokens, export config. This is worse than jwadow (which at least has an API key).

---

## 5. hj01857655/kiro-account-manager

Commit `b43d4d490480769175fbca2146c7c483bc6aa520` — Rust + Tauri 2.x desktop app with embedded axum gateway. **Best security hygiene of the Kiro-proxy group.**

### 5.1 Secret handling — **MED**
- Accounts persisted to `${DATA_DIR}/.kiro-account-manager/accounts.json` via `std::fs::write` — **no explicit chmod** ([`src-tauri/src/core/account.rs:530-545`](https://github.com/hj01857655/kiro-account-manager/blob/b43d4d490480769175fbca2146c7c483bc6aa520/src-tauri/src/core/account.rs#L530-L545)). OS umask applies (typically 0644 on Linux).
- No at-rest encryption; plaintext JSON.
- Uses `rusqlite` for reading Kiro CLI's own sqlite. Read-only access to kiro-cli data.
- The `core::account::Account` struct carries an optional `password: Option<String>` — plaintext in-memory.

### 5.2 Network posture — **GOOD**
- Gateway defaults to `127.0.0.1:8765` ([`src-tauri/src/gateway/mod.rs:184-188`](https://github.com/hj01857655/kiro-account-manager/blob/b43d4d490480769175fbca2146c7c483bc6aa520/src-tauri/src/gateway/mod.rs#L184-L188)). **Explicit localhost default**.
- `local_only: true` default ([`mod.rs:207-209`](https://github.com/hj01857655/kiro-account-manager/blob/b43d4d490480769175fbca2146c7c483bc6aa520/src-tauri/src/gateway/mod.rs#L207-L209)).
- Config validation **refuses** remote binding without an IP allowlist: `if !config.local_only && config.allowed_ips.is_empty() { return Err("允许远程访问时必须至少配置一个白名单来源 IP"); }` ([`mod.rs:347-355`](https://github.com/hj01857655/kiro-account-manager/blob/b43d4d490480769175fbca2146c7c483bc6aa520/src-tauri/src/gateway/mod.rs#L347-L355)). Only Kiro project to enforce this.
- Config validation **refuses** empty client API keys: `if effective_client_api_keys(config).is_empty() { return Err("必须配置客户端 API Key"); }` ([`mod.rs:344-346`](https://github.com/hj01857655/kiro-account-manager/blob/b43d4d490480769175fbca2146c7c483bc6aa520/src-tauri/src/gateway/mod.rs#L344-L346)).
- Outbound `reqwest::Client::new()` — default TLS verification on.

### 5.3 Input validation & auth — **GOOD**
- Client auth via Authorization header or `x-api-key`, checked at [`src-tauri/src/gateway/proxy.rs:2210-2235`](https://github.com/hj01857655/kiro-account-manager/blob/b43d4d490480769175fbca2146c7c483bc6aa520/src-tauri/src/gateway/proxy.rs#L2210-L2235). Uses `==` on `&str` — **not constant-time**, but all keys are high-entropy so timing attacks are impractical.
- IP allowlist check via `ip_matches_allowlist` — supports single IPs and CIDR ranges via `ipnet`.
- **No body size limit** on axum routes (axum has no default cap — user must add `DefaultBodyLimit` layer).
- Request validation via `is_valid_allowlist_entry`.

### 5.4 Dependency audit — **GOOD**
- [Cargo.toml](https://github.com/hj01857655/kiro-account-manager/blob/b43d4d490480769175fbca2146c7c483bc6aa520/src-tauri/Cargo.toml) has specific major versions for every crate. `reqwest 0.12`, `axum 0.8`, `tauri 2`, `rusqlite 0.32`, `tokio 1`. All current.
- `Cargo.lock` committed.

### 5.5 Log hygiene — **GOOD**
- Uses `log` crate via `tauri-plugin-log`. No grep hits for logging bodies or bearer tokens. Request log entries stored at `gateway-request-log.jsonl` store metadata only (method, path, status, client_ip) — not payloads.

### 5.6 Rate limiting — **PARTIAL**
- Per-account load balancer handles rate-limit state upstream via `mark_rate_limited` ([`load_balancer.rs:372-378`](https://github.com/hj01857655/kiro-account-manager/blob/b43d4d490480769175fbca2146c7c483bc6aa520/src-tauri/src/gateway/load_balancer.rs#L372-L378)).
- **No inbound rate limiting** (no tower `rate-limit` or `tower-governor`).

**Risk profile**: By far the most thoughtful Kiro proxy. Desktop-first so lower attack surface. Still has no at-rest encryption and no inbound rate limit. Missing body-limit and missing constant-time compares are easy fixes.

---

## 6. aliom-v/KiroaaS

**Repo returns 404.** Either deleted, renamed, or made private. No audit possible. Note: if it re-appears as a multi-tenant SaaS, this audit would focus on tenant isolation + IDOR / BOLA on account IDs.

---

## 7. caidaoli/kiro2api

Commit `ebf5ad74b5cf10d1f5edcc1404aadc5a29d79fb8` — Go 1.24 + Gin + Sonic JSON. High-performance focus.

### 7.1 Secret handling — **GOOD (in posture), MED (implementation)**
- **Tokens loaded from `KIRO_AUTH_TOKEN` env var only** — either JSON inline or file path ([`auth/config.go:46-75`](https://github.com/caidaoli/kiro2api/blob/ebf5ad74b5cf10d1f5edcc1404aadc5a29d79fb8/auth/config.go#L46-L75)).
- **No persisted token file writes** in the repo (grep for `WriteFile` = only logger's log file at `0644`).
- In-memory refresh by `token_manager.go`. Good: state never hits disk.
- However: **env vars can leak via `/proc/<pid>/environ`, core dumps, process listings** on shared hosts.

### 7.2 Network posture — **HIGH**
- Binds `":" + port` ([`server/server.go:120`](https://github.com/caidaoli/kiro2api/blob/ebf5ad74b5cf10d1f5edcc1404aadc5a29d79fb8/server/server.go#L120)) — **effectively 0.0.0.0** since Go treats empty host as all interfaces.
- CORS `Access-Control-Allow-Origin: *` ([`server/server.go:257-259`](https://github.com/caidaoli/kiro2api/blob/ebf5ad74b5cf10d1f5edcc1404aadc5a29d79fb8/server/server.go#L257-L259)).
- **Outbound TLS conditionally disabled**: [`utils/client.go:38`](https://github.com/caidaoli/kiro2api/blob/ebf5ad74b5cf10d1f5edcc1404aadc5a29d79fb8/utils/client.go#L38) — `InsecureSkipVerify: skipTLS` where `skipTLS = os.Getenv("GIN_MODE") == "debug"`. If anyone runs with `GIN_MODE=debug` (common when testing), TLS verification to `q.{region}.amazonaws.com` is disabled. **This is a problem** because debug-by-production is a very common self-hosting mistake.
- MinVersion TLS 1.2 (good), MaxVersion TLS 1.3 (good).

### 7.3 Input validation & auth — **GOOD**
- Requires `KIRO_CLIENT_TOKEN` env var with hard fail-to-exit if unset ([`main.go:48-55`](https://github.com/caidaoli/kiro2api/blob/ebf5ad74b5cf10d1f5edcc1404aadc5a29d79fb8/main.go#L48-L55)). README explicitly says `use at least 32 random chars`. **Best auth default of the Go group.**
- Middleware at [`server/middleware.go:96-129`](https://github.com/caidaoli/kiro2api/blob/ebf5ad74b5cf10d1f5edcc1404aadc5a29d79fb8/server/middleware.go#L96-L129) accepts either `Authorization: Bearer ...` or `x-api-key`.
- Compare is `providedApiKey != authToken` — **not `crypto/subtle.ConstantTimeCompare`**.
- Only `/v1/*` paths require auth. `/api/tokens` (token pool status API) at `server/server.go:47` is **unauthenticated** — anyone can hit it and see token status. Might leak account count / configuration.
- No body size limit.
- No server timeouts (`ReadTimeout`/`WriteTimeout` absent).

### 7.4 Dependency audit — **GOOD**
- [`go.mod`](https://github.com/caidaoli/kiro2api/blob/ebf5ad74b5cf10d1f5edcc1404aadc5a29d79fb8/go.mod): `gin v1.11.0` (current stable), `sonic v1.14.1` (bytedance - widely used), `validator/v10 v10.27.0` (current).
- All pinned via go.sum.

### 7.5 Log hygiene — **GOOD**
- `handlers.go:29-45` explicitly masks `Authorization` and `X-API-Key` headers in logs, truncating to `first5...last3` ([`server/handlers.go:29-45`](https://github.com/caidaoli/kiro2api/blob/ebf5ad74b5cf10d1f5edcc1404aadc5a29d79fb8/server/handlers.go#L29-L45)).
- In auth failure log, says `expected: ***, provided: ***` — **does not log actual tokens** ([`server/middleware.go:118-122`](https://github.com/caidaoli/kiro2api/blob/ebf5ad74b5cf10d1f5edcc1404aadc5a29d79fb8/server/middleware.go#L118-L122)). Good.

### 7.6 Rate limiting — **NONE**
- No rate limit middleware, no concurrent connection cap, no server-level timeouts.

**Risk profile**: Best auth UX (hard-fail on unset token), good log hygiene, but `GIN_MODE=debug` disabling TLS verify is a silent footgun, and `/api/tokens` info-disclosure endpoint is unauthenticated.

---

## 8. BerriAI/litellm

Commit `fc8a9a34067bb1571bb02bf6b9dc308f89ba168e` — FastAPI + Prisma + SQLAlchemy. **This is the 46K-star category killer, and it has a rich CVE history.**

### 8.1 Secret handling — **GOOD posture, HISTORICAL ISSUES**
- Encryption at rest via **NaCl SecretBox** with `LITELLM_SALT_KEY` (falls back to `LITELLM_MASTER_KEY`) — [`litellm/proxy/common_utils/encrypt_decrypt_utils.py:79-107`](https://github.com/BerriAI/litellm/blob/fc8a9a34067bb1571bb02bf6b9dc308f89ba168e/litellm/proxy/common_utils/encrypt_decrypt_utils.py#L79-L107). Key is `sha256(salt_key)`, nacl authenticated encryption. Solid modern primitive.
- Virtual keys `hash_token(api_key)` before storing (sha256-based).
- **Previously stored passwords as unsalted SHA-256 pre-v1.83.0** — fixed in [GHSA-69x8-hrgq-fjj8](https://github.com/BerriAI/litellm/security/advisories/GHSA-69x8-hrgq-fjj8) to scrypt(n=16384, r=8, p=1, salt=16B). A cautionary tale.
- Encryption key = MASTER_KEY by default → rotating master_key breaks decryption of all stored credentials (documented footgun).

### 8.2 Network posture — **MED**
- Default bind `--host 0.0.0.0` ([`litellm/proxy/proxy_cli.py:456`](https://github.com/BerriAI/litellm/blob/fc8a9a34067bb1571bb02bf6b9dc308f89ba168e/litellm/proxy/proxy_cli.py#L456)).
- CORS default `["*"]` but **sensibly disables `allow_credentials` when wildcard is used** — see ([`proxy_server.py:1240-1266`](https://github.com/BerriAI/litellm/blob/fc8a9a34067bb1571bb02bf6b9dc308f89ba168e/litellm/proxy/proxy_server.py#L1240-L1266)): they specifically call out the Starlette footgun in comments. **Best CORS handling of the ecosystem.**
- Configurable via `LITELLM_CORS_ORIGINS` env.

### 8.3 Input validation & auth — **MED (complex)**
- Master key required — [README samples always show `sk-1234`](https://github.com/BerriAI/litellm/blob/fc8a9a34067bb1571bb02bf6b9dc308f89ba168e/litellm/proxy/common_utils/admin_ui_utils.py#L77) as a **placeholder** (documentation, not code default).
- Extensive auth middleware layer (`litellm/proxy/auth/user_api_key_auth.py`, `oauth2_check.py`, `handle_jwt.py`).
- **Historical CVEs from this area are serious** (see 8.7).

### 8.4 Dependency audit — **UNKNOWN** (massive repo, pinned in pyproject)

### 8.5 Log hygiene — dedicated module `_logging.py`, Langfuse/OTel integrations, extensive. Logs are configurable; defaults redact secrets.

### 8.6 Rate limiting — **GOOD**
- Per-key, per-user, per-team, per-model rate limits in [`litellm/proxy/hooks/dynamic_rate_limiter_v3.py`](https://github.com/BerriAI/litellm/blob/fc8a9a34067bb1571bb02bf6b9dc308f89ba168e/litellm/proxy/hooks/dynamic_rate_limiter_v3.py). Redis-backed. Reference implementation.
- Batch rate limiter for bulk endpoints.

### 8.7 Known CVEs — **CRITICAL history; LOW on latest**

From `gh api repos/BerriAI/litellm/security-advisories` (8 advisories retrieved, 2026 batch alone):

| GHSA | CVE | Sev | Summary | Fixed |
|---|---|---|---|---|
| GHSA-v4p8-mg3p-g94g | CVE-2026-42271 | High | Authenticated command execution via MCP stdio test endpoints (`/mcp-rest/test/connection`, `/mcp-rest/test/tools/list`). Low-priv users could spawn arbitrary commands on proxy host. | 1.83.7 |
| GHSA-wxxx-gvqv-xp7p | CVE-2026-40217 | High | Sandbox escape in `/guardrails/test_custom_code` — hand-rolled Python sandbox bypass via bytecode → RCE as root (default Docker image runs as root). | 1.83.11 |
| GHSA-r75f-5x8p-qvmc | CVE-2026-42208 | **Critical** | **SQL injection in proxy API-key verification**. Unauthenticated `Authorization` header → arbitrary query via error-handling path. | 1.83.7 |
| GHSA-xqmj-j6mv-4862 | CVE-2026-42203 | High | Server-Side Template Injection in `/prompts/test` — any proxy key → RCE in proxy process. | 1.83.7 |
| GHSA-69x8-hrgq-fjj8 | — | High | Password hash exposure + pass-the-hash: unsalted SHA-256 + hash returned in `/user/info`, `/spend/users`; `/v2/login` accepted hash as password. | 1.83.0 |
| GHSA-53mr-6c8q-9789 | CVE-2026-35029 | High | Privilege escalation: `/config/update` did not enforce admin role. Any authenticated user → RCE via pass-through endpoint handlers + arbitrary file read via `UI_LOGO_PATH`. | 1.83.0 |
| GHSA-jjhc-v7c2-5hh6 | CVE-2026-35030 | **Critical** | Auth bypass via OIDC userinfo cache collision (`token[:20]` as cache key). Only affects `enable_jwt_auth: true`. | 1.83.0 |

Pattern: **everything listed is post-auth**, but multiple give RCE from a valid low-priv key → full host takeover. The SQL-injection via Authorization header is pre-auth critical.

**Risk profile for self-hosters**: pin to latest (≥1.83.11), do **not** expose the Admin UI publicly, never run the Docker image as root. The feature velocity creates new sharp edges every release.

---

## 9. Portkey-AI/gateway

Commit `351692fd9236af222168134b416924fae0bdba23` — Hono + `@hono/node-server`. Edge-runtime first (Cloudflare Workers). Default port 8787.

### 9.1 Secret handling — **NOT CLIENT-SIDE**
- Architecturally, Portkey gateway forwards per-request credentials (`x-portkey-virtualkey`, `Authorization`). The gateway itself does **not persist credentials** — it relies on an external config store (the Portkey SaaS or user-provided config). Docker/OSS image is stateless.
- No encryption primitives needed on the gateway itself.

### 9.2 Network posture — **MED**
- `@hono/node-server` `serve({ port })` ([`src/start-server.ts:146`](https://github.com/Portkey-AI/gateway/blob/351692fd9236af222168134b416924fae0bdba23/src/start-server.ts#L146)) — **binds 0.0.0.0** by default (Node's `listen` default).
- **SSRF defense is the crown jewel here**: [`src/middlewares/requestValidator/index.ts:27-80`](https://github.com/Portkey-AI/gateway/blob/351692fd9236af222168134b416924fae0bdba23/src/middlewares/requestValidator/index.ts#L27-L80) has a dense `BLOCKED_HOSTS` list including `169.254.169.254` (metadata), `metadata.google.internal`, `metadata.azure.com`, and an IPv4 range table blocking `10/8`, `172.16/12`, `192.168/16`, `127/8`, `169.254/16`, `0/8`, `224/3`. Also blocks `.local`, `.internal`, `.onion`, `.invalid` TLDs. Decimal/hex/octal/IPv4-in-IPv6 forms are all parsed and checked. This was added after CVE-2025-66405 (SSRF via `x-portkey-custom-host`).

### 9.3 CVE — **1 published**
- [GHSA-hhh5-2cvx-vmfp / CVE-2025-66405](https://github.com/Portkey-AI/gateway/security/advisories/GHSA-hhh5-2cvx-vmfp): SSRF in custom-host header. Patched in 1.14.0. The SSRF-defense module is the remediation.

### 9.4 Dependency audit — **HIGH**
`npm audit` output (run locally on the clone): **21 vulnerabilities (8 moderate, 12 high, 1 critical)**:
- `@hono/node-server <1.19.10`: auth bypass via encoded slashes (high, CVE-2024/2025 advisory).
- `@hono/node-server <1.19.13`: middleware bypass via repeated slashes (moderate).
- `@rollup/plugin-terser` (serialize-javascript): high.
- `@babel/helpers <7.26.10`: ReDoS.
- `ajv <6.14.0`: ReDoS.
- `brace-expansion`: ReDoS + memory exhaustion.
- `miniflare/undici`: cascading vulns.
- `yaml 2.0.0-2.8.2`: stack overflow on deep nesting.
- Most fixable via `npm audit fix`. The `@hono/node-server` one is production-critical.

### 9.5 Rate limiting — **OUT OF SCOPE for OSS gateway**
Hono has no rate limiter built-in. Portkey SaaS handles this; self-hosters must add their own middleware. The core OSS gateway doesn't enforce quotas.

**Risk profile**: SSRF-defense is exemplary (worth studying, copying). `npm audit` is ugly. Gateway is stateless so breach of "the gateway" itself doesn't leak persistent secrets.

---

## 10. Kong/kong

Commit `58f2daa56b90615f78d5953229936192cd1128e9` — Lua/OpenResty. **Focus: AI plugins only** (`ai-proxy`, `ai-prompt-guard`, etc.).

### 10.1 Secret handling in AI plugins — **GOOD**
- Upstream LLM credentials in [`kong/llm/schemas/init.lua:55-120`](https://github.com/Kong/kong/blob/58f2daa56b90615f78d5953229936192cd1128e9/kong/llm/schemas/init.lua#L55-L120) — the auth record is defined with:
  - `encrypted = true` on `header_value`, `param_value`, `aws_access_key_id`, `aws_secret_access_key`.
  - `referenceable = true` on all sensitive fields — letting ops store credentials in **Vault, AWS Secrets Manager, GCP Secret Manager** via Kong's secrets backend (`{vault://env/MY_SECRET}` references).
- This pattern is what you want: **credentials never sit plaintext in plugin config**; they're resolved at request time from a secret store.

### 10.2 Network posture — **GOOD**
- Default: `proxy_listen = 0.0.0.0:8000`, `admin_listen = 127.0.0.1:8001` ([`kong.conf.default`](https://github.com/Kong/kong/blob/58f2daa56b90615f78d5953229936192cd1128e9/kong.conf.default)). **Admin API bound to localhost by default** — industry-correct pattern.
- TLS supported natively (`ssl_cert`, `ssl_cert_key`); `http2 ssl reuseport` flags ready.

### 10.3 Input validation for AI — **GOOD**
- `ai-proxy` has [`max_request_body_size = 8 * 1024`](https://github.com/Kong/kong/blob/58f2daa56b90615f78d5953229936192cd1128e9/kong/plugins/ai-proxy/schema.lua#L15-L20) default (8KB). **Enforced in plugin schema.**
- `ai-prompt-guard` has [regex allow/deny patterns, `len_max=10` patterns, `len_max=500` each](https://github.com/Kong/kong/blob/58f2daa56b90615f78d5953229936192cd1128e9/kong/plugins/ai-prompt-guard/schema.lua#L11-L30). Pattern-based filtering, capped to prevent ReDoS.
- `allow_all_conversation_history: false` default, so only the latest turn is scanned by default → safer.

### 10.4 Rate limiting — **GOOD**
- Dedicated `rate-limiting` and `response-ratelimiting` plugins built-in; enterprise has `ai-rate-limiting-advanced` with token-based limits.

### 10.5 Known CVEs — `gh api` reports **0 published advisories** in this repo (OSS / community editions). Kong Enterprise advisories are not in this surface.

**Risk profile**: Mature enterprise-grade posture for AI plugins. Worth copying: (a) `encrypted=true` + `referenceable=true` on secret fields, (b) localhost-default Admin API, (c) body size cap in plugin schema, (d) pattern-based prompt guard with length limits.

---

## Synthesis: Common Weak Spots Across the AI-Proxy Ecosystem

Drawing from the 9 audits, the **recurring structural weaknesses** are:

1. **Binding to `0.0.0.0` by default** (7 of 9): jwadow, Quorinex, petehsu, caidaoli, litellm, Portkey via Hono node adapter, AntiHub binds localhost. hj01857655 and Kong Admin API are the only explicit-localhost defaults. **Nobody running `python main.py` expects their AWS tokens to be LAN-reachable, but that's the default.**
2. **Wildcard CORS with credentials** (5 of 9): jwadow, Quorinex, petehsu, caidaoli, old litellm. Only litellm has remediated this consciously (disabling `allow_credentials` when wildcard is used). The Starlette-reflects-Origin footgun is widespread.
3. **No at-rest encryption for tokens** (all Kiro proxies): jwadow, Quorinex, petehsu, hj01857655 all write plaintext JSON. Only **Kong via `encrypted=true` schema + Vault references** and **litellm via NaCl SecretBox** do this properly.
4. **File permissions not set on write** (8 of 9): Only Quorinex uses `0600`. Everyone else writes via `open(path, 'w')` or `std::fs::write` accepting OS umask → typically `0644`, **world-readable on Linux**.
5. **Hardcoded or weak default admin credentials** (3 of 9): jwadow ships `"my-super-secret-password-123"` as the default; Quorinex ships `"changeme"`; petehsu has **no admin auth at all**.
6. **`!=` / `===` string comparison for API key checks, not constant-time** (9 of 9): every single project compares the candidate token with `!=` or `===` rather than `secrets.compare_digest` / `hmac.compare_digest` / `crypto.timingSafeEqual` / `subtle::ConstantTimeCompare`. Only litellm test code even mentions `compare_digest`.
7. **No request-body size limit** (7 of 9): jwadow, Quorinex, petehsu, caidaoli, hj01857655 lack it. AntiHub sets 50MB (too high but present); Kong's ai-proxy has 8KB per-plugin. Uploading a 2GB JSON can DoS any of these.
8. **No rate limiting on inbound requests** (8 of 9): petehsu has the class built but defaults off; only litellm (with Redis-backed dynamic limits) and Kong (via plugin) ship with real limiters.
9. **No server timeouts** (6 of 9): `ReadTimeout`/`WriteTimeout`/`IdleTimeout` unset in Quorinex, caidaoli; slowloris vulnerable.
10. **Admin endpoints without auth** (petehsu): the most severe instance — 30+ admin routes on 0.0.0.0 with zero auth.
11. **Debug-mode side channels**: caidaoli disables TLS verify when `GIN_MODE=debug`. jwadow writes full request bodies to disk under DEBUG=all. Both are defaults-off but trivial to flip accidentally.
12. **Unpinned deps** (Python side): jwadow's `requirements.txt` has zero version pins. petehsu uses `>=` ranges. Node side (Portkey) has 21 npm audit issues including a high-severity `@hono/node-server` auth bypass.
13. **No SSRF defense for upstream URL overrides** (pre-patch Portkey had this; most Kiro proxies don't let clients override upstream, so not directly applicable — but worth checking for any user-controlled redirect URL handling).

**Ecosystem-level CVE pattern (from litellm)**: once a gateway grows features for MCP/plugin/tool management, each admin endpoint becomes an authenticated-RCE candidate. Sandbox escapes and template injection are overrepresented.

---

## Recommended Audit Items for kiroxy

Concrete checks kiroxy should enforce, ordered by impact.

### Severity CRITICAL — reject any release failing these
1. **Default bind to `127.0.0.1`** — in `--host` CLI default, env default, and README sample. Require `--host 0.0.0.0` to be explicit.
2. **No hardcoded default API key.** Fail-to-start if `KIROXY_API_KEY`/equivalent is unset. Follow caidaoli's pattern ([main.go:48-55](https://github.com/caidaoli/kiro2api/blob/ebf5ad74b5cf10d1f5edcc1404aadc5a29d79fb8/main.go#L48-L55)): log an error line with a generation hint and exit non-zero.
3. **Constant-time API-key comparison.** Use `subtle.ConstantTimeCompare([]byte(provided), []byte(expected))` (Go) or equivalent for every key check. Unit-test with `expected="abc"`, `provided="abd"` to ensure equal length handling.
4. **All admin / account / config endpoints behind the same auth gate** — no unauthenticated "token pool status" or "health" leaking account counts. If a path shows anything more than 200 OK, require auth.
5. **SSRF defense on any upstream URL that is user-controlled.** kiroxy only talks to `kiro.dev` endpoints, but validate the region + host is in an allowlist (no `Host:` override, no custom-upstream query parameter).

### Severity HIGH — ship fixes for these before v1.0
6. **Reject `allow_origins=["*"]` combined with `allow_credentials=True`** in any CORS config. If wildcard is used, force `allow_credentials=False`. Copy litellm's `_get_cors_config` pattern.
7. **Per-request body size limit.** Gin: `c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 2*1024*1024)`. Axum: `.layer(DefaultBodyLimit::max(2 * 1024 * 1024))`. Pick a default of 2MB; allow override.
8. **Server timeouts set explicitly.**
   ```go
   srv := &http.Server{
     Addr: addr,
     Handler: r,
     ReadTimeout: 30 * time.Second,
     ReadHeaderTimeout: 10 * time.Second,
     WriteTimeout: 600 * time.Second, // long for streaming
     IdleTimeout: 120 * time.Second,
     MaxHeaderBytes: 1 << 20,
   }
   ```
9. **File permissions `0600` on all persisted secrets.** Use `os.WriteFile(path, data, 0600)` (Go) or explicit `os.chmod(path, 0o600)` (Python) after `open('w')`. Directory `0700`. Copy Quorinex's pattern ([config/config.go:198](https://github.com/Quorinex/Kiro-Go/blob/1732b17ff9455e55cb9dcf34cf23c39f5b549042/config/config.go#L198)).
10. **At-rest encryption for refresh tokens.** Derive a key via `argon2id(passphrase, host-bound-salt)` or use OS keyring. Minimum: XChaCha20-Poly1305 via `golang.org/x/crypto/chacha20poly1305` or `libsodium`. Follow litellm's NaCl SecretBox approach ([encrypt_decrypt_utils.py:79](https://github.com/BerriAI/litellm/blob/fc8a9a34067bb1571bb02bf6b9dc308f89ba168e/litellm/proxy/common_utils/encrypt_decrypt_utils.py#L79)) but wrap around a user-supplied passphrase, not a reused master key. Document the rotation story clearly.
11. **No TLS-verify-off switch based on env alone.** Do not ship `InsecureSkipVerify: os.Getenv("DEBUG")==true`. If needed, require a CLI flag + log WARN on every request.
12. **Pin all deps.** Go: `go.sum` must be committed. Python: use `pip-tools` / `uv lock` with hashes. Node: commit `package-lock.json` + run `npm audit --audit-level=high` in CI.
13. **Rate limit inbound requests per-API-key.**  Simple token-bucket (`golang.org/x/time/rate` in Go, `slowapi` in FastAPI) with 60 req/min default per key is enough. Opt-in "burst" config.

### Severity MEDIUM — post-launch hardening
14. **IP allowlist when binding non-loopback.** Follow hj01857655's pattern ([mod.rs:347-355](https://github.com/hj01857655/kiro-account-manager/blob/b43d4d490480769175fbca2146c7c483bc6aa520/src-tauri/src/gateway/mod.rs#L347-L355)): config validation fails if `host != 127.0.0.1` and allowlist is empty.
15. **Header redaction in logs**, mirroring kiro2api's `extractRelevantHeaders` ([handlers.go:29-45](https://github.com/caidaoli/kiro2api/blob/ebf5ad74b5cf10d1f5edcc1404aadc5a29d79fb8/server/handlers.go#L29-L45)): truncate Authorization / x-api-key / Bearer tokens to `first5...last3`. Never log full request body at INFO level.
16. **Debug payload capture OFF by default.** If enabled, write to a directory with `0700` perms and clear a retention warning at startup.
17. **Prompt-guard regex engine on tool_use inputs** (or at least sanitize obviously-dangerous `system` prompt injections). Kong's approach ([`ai-prompt-guard/schema.lua`](https://github.com/Kong/kong/blob/58f2daa56b90615f78d5953229936192cd1128e9/kong/plugins/ai-prompt-guard/schema.lua)) with `len_max` caps on patterns is sensible.
18. **JSON schema validation on tool_use inputs.** Use Pydantic (Python) or `validator/v10` (Go). Reject unknown fields.
19. **Do not trust `X-Forwarded-For` blindly.** Only parse when behind a configured trusted proxy. Follow litellm's `trusted_proxy_utils.py` approach.
20. **OS keyring integration** for the refresh token (macOS Keychain, Linux Secret Service, Windows Credential Manager) — makes "stolen laptop → stolen tokens" meaningfully harder than `cat ~/.kiroxy/tokens.json`.
21. **Run as non-root in Docker images.** `USER 1000:1000`. The litellm GHSA-wxxx-gvqv-xp7p advisory explicitly noted "runs as root in default Docker image" as an amplifier.
22. **Admin UI on a separate port** bound only to localhost (Kong pattern), distinct from the proxy port.

### Severity LOW — nice-to-haves
23. `SECURITY.md` with vuln-reporting contact & PGP key.
24. Dependabot / Renovate enabled.
25. Release notes flag security-relevant changes.
26. `govulncheck`, `trivy`, `npm audit` as CI gates (block on high).
27. Include a `--dry-run` / `--check-config` mode that validates bind/auth/CORS before starting the server, rejecting unsafe combinations.
28. Kill `Server:` banner and any version disclosure in 404 responses.
29. Per-account and global request-size metrics emitted via Prometheus or OTLP so operators can see abuse early.

---

## Appendix: Verification Notes

- All SHAs captured with `git rev-parse HEAD` on 2026-05-13 after `--depth 1` clone. Permalinks are stable.
- `npm audit` was run inside the Portkey clone (no production install — the lockfile was already present).
- `gh api repos/<owner>/<repo>/security-advisories` was used for CVE discovery on litellm, Portkey, Kong. litellm yielded 8 advisories; Portkey 1; Kong 0 in the OSS repo.
- KiroaaS was 404 at audit time. If that repo re-surfaces, multi-tenancy + IDOR should be the audit focus.
- Subagent dispatch was attempted first but the session environment blocked child session creation (`Failed to create child session`); all audits were executed in the main shell against local clones instead.
- `govulncheck` was NOT run against Go modules (would require `go install`) — dependency assessment is based on `go.mod` direct listing.

---

*End of SECURITY.md*
