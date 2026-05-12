# OPERATIONS.md — kiroxy self-host playbook

> Deployment patterns, runbooks, backup, monitoring for running kiroxy
> in production. Target operator: solo developer on home server,
> laptop, VPS, or cloud. Not platform team.
>
> Assumptions:
> - You've run through README.md quickstart and have kiroxy
>   listening on 127.0.0.1:8787 locally.
> - You have at least one Kiro account imported.
> - You want to keep running kiroxy reliably beyond "first demo".
>
> Companion: research-v4/READINESS.md (pre-ship audit),
> docs/TROUBLESHOOTING.md (symptom → fix).

---

## Table of contents

1. [Deployment patterns](#1-deployment-patterns)
2. [Reverse proxy & TLS](#2-reverse-proxy--tls)
3. [Private networking](#3-private-networking)
4. [Multi-device usage](#4-multi-device-usage)
5. [Monitoring](#5-monitoring)
6. [Backup & disaster recovery](#6-backup--disaster-recovery)
7. [Operational hygiene](#7-operational-hygiene)
8. [Upgrade & rollback](#8-upgrade--rollback)
9. [Runbooks](#9-runbooks)

---

## 1. Deployment patterns

### 1.1 Decision tree

```
Is the consumer exclusively you?
├── Yes, on this laptop  → §1.2 laptop binary
├── Yes, from multiple devices at home  → §1.3 home server
└── Yes, but from anywhere (including on-the-go)  → §1.4 cloud or §3 tunnel
```

### 1.2 Laptop binary (simplest)

Use case: single developer, single machine, everything local.

```bash
# Build or download
go install ./cmd/kiroxy    # or brew install kiroxy (when published)

# One-time setup
kiroxy add-account --label=me

# Run in a persistent session
kiroxy serve  # defaults: 127.0.0.1:8787
```

To keep it running across terminal sessions:

**launchd on macOS** (file `~/Library/LaunchAgents/dev.kiroxy.plist`):
```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>               <string>dev.kiroxy</string>
  <key>ProgramArguments</key>    <array>
    <string>/usr/local/bin/kiroxy</string>
    <string>serve</string>
  </array>
  <key>EnvironmentVariables</key><dict>
    <key>KIROXY_LOG_LEVEL</key>  <string>info</string>
  </dict>
  <key>RunAtLoad</key>           <true/>
  <key>KeepAlive</key>           <true/>
  <key>StandardOutPath</key>     <string>/tmp/kiroxy.log</string>
  <key>StandardErrorPath</key>   <string>/tmp/kiroxy.err.log</string>
</dict>
</plist>
```

Load with `launchctl load ~/Library/LaunchAgents/dev.kiroxy.plist`.

Tradeoffs: simple, but your Kiro subscription is pinned to your
laptop's battery + network.

### 1.3 Home server (recommended for multi-device)

Use case: kiroxy on an always-on machine (Raspberry Pi, NUC, spare
Mac mini, Synology / Unraid). Devices on your LAN call it directly.

**systemd service** (Linux), save as `/etc/systemd/system/kiroxy.service`:

```ini
[Unit]
Description=kiroxy — self-hosted Kiro proxy
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=kiroxy
Group=kiroxy
ExecStart=/usr/local/bin/kiroxy serve
Restart=on-failure
RestartSec=5
TimeoutStopSec=35
# 35s > KIROXY_SHUTDOWN_TIMEOUT default (30s)

Environment=KIROXY_BIND=127.0.0.1
Environment=KIROXY_PORT=8787
Environment=KIROXY_LOG_LEVEL=info
Environment=KIROXY_DB_PATH=/var/lib/kiroxy/tokens.db
EnvironmentFile=-/etc/kiroxy/env
# EnvironmentFile is optional (hence `-`); use it to stash
# KIROXY_API_KEY so it doesn't appear in systemctl status output.

# Hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
PrivateDevices=true
ReadWritePaths=/var/lib/kiroxy
CapabilityBoundingSet=
AmbientCapabilities=
RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX
MemoryDenyWriteExecute=true
LockPersonality=true
RestrictNamespaces=true
RestrictRealtime=true
SystemCallArchitectures=native
SystemCallFilter=@system-service

[Install]
WantedBy=multi-user.target
```

Setup:
```bash
sudo useradd --system --home-dir=/var/lib/kiroxy --shell=/usr/sbin/nologin kiroxy
sudo install -d -o kiroxy -g kiroxy -m 0700 /var/lib/kiroxy
sudo install -d -o root    -g root    -m 0755 /etc/kiroxy
echo "KIROXY_API_KEY=$(openssl rand -hex 32)" | sudo install -m 0600 /dev/stdin /etc/kiroxy/env

sudo systemctl daemon-reload
sudo systemctl enable --now kiroxy
sudo systemctl status kiroxy
```

Point LAN clients at `http://<server-ip>:8787` (through reverse proxy
if TLS desired — §2).

### 1.4 Cloud VPS (personal always-on, accessible from anywhere)

Use case: you want kiroxy reachable on-the-go without exposing your
home LAN.

Options, in order of simplicity:

#### 1.4.1 Fly.io single-machine

`fly.toml`:
```toml
app = "your-kiroxy"
primary_region = "sjc"   # closest to you; NOT Kiro's region

[build]
  dockerfile = "Dockerfile"

[http_service]
  internal_port = 8787
  force_https = true
  auto_stop_machines = false   # we want persistent vault
  auto_start_machines = true
  min_machines_running = 1

[[services.ports]]
  handlers = ["http"]
  port = 80
  force_https = true

[mounts]
  source = "kiroxy_data"
  destination = "/data"

[env]
  KIROXY_BIND = "0.0.0.0"
  KIROXY_PORT = "8787"
  KIROXY_LOG_LEVEL = "info"

[[services.tcp_checks]]
  interval = "15s"
  timeout = "2s"
  grace_period = "10s"
  port = 8787
```

Ship:
```bash
fly volumes create kiroxy_data --region sjc --size 1
fly secrets set KIROXY_API_KEY=$(openssl rand -hex 32)
fly deploy
fly ssh console -C "kiroxy add-account --label=me"
```

Tradeoffs: free tier works for single user; persistent volume survives
deploys; fly.io handles TLS + global Anycast.

#### 1.4.2 Hetzner / DigitalOcean droplet

Cheapest self-managed option (~$5/mo). Use cloud-init to install:

```yaml
#cloud-config
package_update: true
packages:
  - docker.io
  - docker-compose-plugin
users:
  - name: kiroxy
    shell: /usr/sbin/nologin
    system: true
write_files:
  - path: /etc/systemd/system/kiroxy.service
    content: |
      # (systemd unit from §1.3)
runcmd:
  - curl -LO https://github.com/YOU/kiroxy/releases/latest/download/kiroxy_linux_amd64.tar.gz
  - tar xzf kiroxy_linux_amd64.tar.gz -C /usr/local/bin/ kiroxy
  - chmod +x /usr/local/bin/kiroxy
  - install -d -o kiroxy -g kiroxy -m 0700 /var/lib/kiroxy
  - systemctl enable --now kiroxy
```

Expose via Caddy for TLS (§2).

#### 1.4.3 AWS EC2 with VPC endpoint

If you care about keeping traffic inside AWS to reduce latency to
`*.kiro.dev`:
- Spin up t4g.nano ($3/mo) in us-east-1 (same region as Kiro).
- VPC private routing to `*.amazonaws.com` for OIDC refresh works via
  AWS internal network.
- Kiro's `runtime.*.kiro.dev` may or may not be on AWS's internal
  network (not documented); treat as public egress.

Marginal latency gain (maybe 20-50ms TTFB improvement); not worth the
AWS-shaped ops overhead unless you're already in AWS.

### 1.5 Docker Compose (anywhere)

kiroxy ships with `docker-compose.yml` tuned for single-user:
- Read-only root filesystem, `/data` volume for vault
- `cap_drop: [ALL]`, `no-new-privileges:true`
- Loopback-only port mapping (`127.0.0.1:8787`)
- Healthcheck wired to `kiroxy healthcheck` subcommand

Start:
```bash
cp .env.example .env
# edit .env: set KIROXY_API_KEY
docker compose up -d
docker compose exec kiroxy kiroxy list-accounts
```

Tradeoffs: fewer rough edges than raw systemd + Go binary; update via
`docker compose pull && docker compose up -d`.

---

## 2. Reverse proxy & TLS

kiroxy does NOT terminate TLS itself — by design. Front it with a
reverse proxy that does.

### 2.1 Caddy (recommended)

Simplest. `Caddyfile`:
```caddyfile
kiroxy.example.com {
  reverse_proxy 127.0.0.1:8787 {
    # Stream-friendly
    flush_interval -1
    transport http {
      read_timeout 300s
      write_timeout 300s
    }
  }
  encode gzip

  # Optional: IP allowlist
  @notlocal not remote_ip 192.168.0.0/16 10.0.0.0/8
  respond @notlocal "forbidden" 403
}
```

Caddy auto-provisions Let's Encrypt certs. Streaming responses work
with `flush_interval -1` so SSE chunks aren't buffered.

### 2.2 nginx + certbot

```nginx
upstream kiroxy {
  server 127.0.0.1:8787;
  keepalive 32;
}

server {
  listen 443 ssl http2;
  server_name kiroxy.example.com;

  ssl_certificate     /etc/letsencrypt/live/kiroxy.example.com/fullchain.pem;
  ssl_certificate_key /etc/letsencrypt/live/kiroxy.example.com/privkey.pem;
  ssl_protocols       TLSv1.2 TLSv1.3;

  # Large SSE / chat bodies
  client_max_body_size 8m;
  proxy_read_timeout   300s;
  proxy_send_timeout   300s;

  # No response buffering; critical for streaming
  proxy_buffering      off;
  proxy_cache          off;

  location / {
    proxy_pass http://kiroxy;
    proxy_http_version 1.1;
    proxy_set_header Host              $host;
    proxy_set_header X-Real-IP         $remote_addr;
    proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header Connection "";
  }
}

server {
  listen 80;
  server_name kiroxy.example.com;
  return 301 https://$host$request_uri;
}
```

### 2.3 Traefik (docker-compose)

Add to `docker-compose.yml`:
```yaml
  traefik:
    image: traefik:v3
    command:
      - "--providers.docker=true"
      - "--providers.docker.exposedbydefault=false"
      - "--entrypoints.websecure.address=:443"
      - "--entrypoints.web.address=:80"
      - "--certificatesresolvers.le.acme.email=you@example.com"
      - "--certificatesresolvers.le.acme.storage=/le/acme.json"
      - "--certificatesresolvers.le.acme.tlschallenge=true"
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - le:/le
      - /var/run/docker.sock:/var/run/docker.sock:ro
    restart: unless-stopped

# Add labels to the kiroxy service:
services:
  kiroxy:
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.kiroxy.rule=Host(`kiroxy.example.com`)"
      - "traefik.http.routers.kiroxy.entrypoints=websecure"
      - "traefik.http.routers.kiroxy.tls.certresolver=le"
      - "traefik.http.services.kiroxy.loadbalancer.server.port=8787"

volumes:
  le:
  kiroxy-data:
```

---

## 3. Private networking

When kiroxy should be reachable by YOU from anywhere, but not by the
public internet.

### 3.1 Tailscale (simplest for personal use)

Install on your home server AND every client device. Tailscale gives
each machine a private IP in your "tailnet" — kiroxy binds to that
IP + loopback, nothing else.

```bash
# On kiroxy host
curl -fsSL https://tailscale.com/install.sh | sh
sudo tailscale up

# Find the tailnet IP
tailscale ip -4      # e.g. 100.64.1.5
```

Modify kiroxy to listen on loopback + tailnet:
```bash
# systemd env file or docker-compose env:
KIROXY_BIND=0.0.0.0   # then rely on Tailscale ACLs
# OR
KIROXY_BIND=100.64.1.5
```

Tailscale ACL (Admin Console → Access Controls):
```json
{
  "acls": [
    {"action": "accept", "src": ["autogroup:owner"], "dst": ["100.64.1.5:8787"]},
    {"action": "drop",   "src": ["*"],              "dst": ["100.64.1.5:8787"]}
  ]
}
```

Your laptop / phone just runs Tailscale and hits
`http://100.64.1.5:8787/v1/messages`. No port forwarding, no TLS, no
DNS. Works over cellular.

### 3.2 Cloudflare Tunnel (public, no port-forwarding)

Use case: you want a stable public URL (e.g. for a friend to share
your proxy) but don't want to open ports.

```bash
# On kiroxy host
# Install cloudflared, then:
cloudflared tunnel login
cloudflared tunnel create kiroxy
cloudflared tunnel route dns kiroxy kiroxy.example.com

cat > ~/.cloudflared/config.yml <<EOF
tunnel: kiroxy
credentials-file: /home/you/.cloudflared/kiroxy.json
ingress:
  - hostname: kiroxy.example.com
    service: http://127.0.0.1:8787
    originRequest:
      noTLSVerify: true        # local upstream is HTTP only
      connectTimeout: 30s
      # Critical for SSE streaming:
      disableChunkedEncoding: false
      noHappyEyeballs: true
  - service: http_status:404
EOF

sudo cloudflared service install
```

Tradeoffs: Cloudflare's edge adds 50-100ms, and you're trusting
Cloudflare with your traffic. But you get zero-trust access control
via Cloudflare Access if you want.

### 3.3 WireGuard (self-hosted mesh)

For the truly paranoid or anti-corporate. Setup is non-trivial.
`wg-easy` (github.com/WeeJeWel/wg-easy) is the most approachable
option; ship a docker-compose alongside kiroxy. Once the tunnel is
up, treat kiroxy's listen IP like §3.1.

---

## 4. Multi-device usage

### 4.1 Single kiroxy instance, multiple consumers

Default behavior. Every device points its downstream client
(claude-code, Cursor, opencode, aider) at the same kiroxy URL.

Tradeoffs:
- **Pro**: one Kiro subscription, simple state.
- **Con**: prompt-cache hits are best when the same conversation
  flow stays on one kiroxy; sessions from different devices don't
  share cache.

### 4.2 Per-device API keys

Today kiroxy supports ONE inbound `KIROXY_API_KEY`. Workaround:
issue the same key to all your devices, or put kiroxy behind a
reverse proxy that does per-device auth:

```caddyfile
kiroxy.example.com {
  @laptop header X-Device laptop
  handle @laptop {
    # Validate laptop-specific token, e.g. via basic_auth
    basic_auth * {
      laptop JHAREDHASHEDPASSWORD
    }
    reverse_proxy 127.0.0.1:8787 {
      header_up X-Api-Key "{env.KIROXY_API_KEY}"
    }
  }
}
```

Ugly, but works. v1.1 planned: native per-device keys.

### 4.3 Session affinity

kiroxy respects `X-Claude-Code-Session-Id` as `conversationId` — see
research-v4/PROTOCOL.md §5.2. If you use the same session ID across
multiple devices (e.g. you sync claude-code state), prompt caching
works. If each device has its own session, cache is per-device.

---

## 5. Monitoring

### 5.1 Prometheus scrape setup

Add to `prometheus.yml`:
```yaml
scrape_configs:
  - job_name: kiroxy
    scrape_interval: 30s
    metrics_path: /metrics
    static_configs:
      - targets: ['127.0.0.1:8787']
    # If kiroxy requires auth:
    authorization:
      credentials_file: /etc/prometheus/kiroxy-api-key
```

Or set `KIROXY_METRICS_PUBLIC=1` if your Prometheus is on a trusted
private network (e.g. tailnet). See docs/METRICS.md.

### 5.2 Grafana dashboard

Import `docs/METRICS.grafana.json` as a new dashboard. Panels
included:
- Request rate by status class
- Latency p50/p95/p99
- Upstream TTFB distribution
- Account pool health (available / cooldown / failed)
- Refresh attempt rate + outcomes
- Error rate by kind

### 5.3 Alert rules (recommended — not shipped today)

kiroxy v1.0.0 does not ship alert rules. Roll your own:
```yaml
# prometheus/kiroxy-alerts.yml
groups:
- name: kiroxy
  rules:
  - alert: KiroxyDown
    expr: up{job="kiroxy"} == 0
    for: 2m
    annotations:
      summary: kiroxy /metrics unreachable

  - alert: KiroxyPoolDepleted
    expr: kiroxy_accounts_available == 0 and kiroxy_accounts_cooldown > 0
    for: 5m
    annotations:
      summary: All kiroxy accounts in cooldown

  - alert: KiroxyHighErrorRate
    expr: sum(rate(kiroxy_request_errors_total[5m])) / sum(rate(kiroxy_requests_total[5m])) > 0.1
    for: 10m
    annotations:
      summary: kiroxy error rate >10% for 10 minutes

  - alert: KiroxyRefreshFailing
    expr: sum(rate(kiroxy_refresh_attempts_total{result=~"fail_.*"}[10m])) > 0.01
    for: 15m
    annotations:
      summary: Token refresh failing — imminent auth outage

  - alert: KiroxyUpstreamLatencyHigh
    expr: histogram_quantile(0.95, rate(kiroxy_upstream_ttfb_seconds_bucket[5m])) > 2
    for: 10m
    annotations:
      summary: kiroxy upstream TTFB p95 > 2s
```

### 5.4 Lightweight alerting (no Prometheus)

If you don't want to run Prometheus, use Healthchecks.io or ntfy.sh
to watch `/healthz`:

```bash
# Cron: every 5 min, ping healthchecks if kiroxy is healthy
*/5 * * * * curl -sf http://127.0.0.1:8787/healthz && \
            curl -fsSL -o /dev/null https://hc-ping.com/<uuid>
```

### 5.5 Log aggregation

For single-host: journald is enough (`journalctl -u kiroxy`).

For multi-host or richer querying: ship logs to Loki via promtail,
or Vector → any backend. kiroxy's JSON format is parse-ready.

---

## 6. Backup & disaster recovery

### 6.1 What to back up

| Artifact | Path (host) | Path (Docker) | Why |
|---|---|---|---|
| Token vault | `~/.kiroxy/tokens.db` | `/data/tokens.db` in kiroxy-data volume | Losing it = re-onboard all accounts |
| Systemd / launchd config | `/etc/systemd/system/kiroxy.service` | — | Reproducibility |
| Env file | `/etc/kiroxy/env` | `.env` | `KIROXY_API_KEY` |
| Onboarder profile dirs | `tools/onboard/profiles/` | — | Optional: retains warm Camoufox sessions |

### 6.2 Encrypted vault backup with age

```bash
#!/bin/bash
# /usr/local/bin/kiroxy-backup
set -euo pipefail
BACKUP_DIR=/backup/kiroxy
DATE=$(date +%Y%m%d-%H%M%S)
SRC=${KIROXY_DB_PATH:-$HOME/.kiroxy/tokens.db}

mkdir -p "$BACKUP_DIR"

# SQLite online backup (safe during writes)
sqlite3 "$SRC" ".backup '$BACKUP_DIR/tokens-$DATE.db'"

# Encrypt with age (recipient pubkey baked in)
age -r age1xxxxxx... -o "$BACKUP_DIR/tokens-$DATE.db.age" \
    "$BACKUP_DIR/tokens-$DATE.db"
rm "$BACKUP_DIR/tokens-$DATE.db"

# Upload to off-site (B2, R2, etc.)
rclone copy "$BACKUP_DIR/tokens-$DATE.db.age" remote:kiroxy-backup/

# Retention: keep last 7 locally, 30 remote
find "$BACKUP_DIR" -name "tokens-*.db.age" -mtime +7 -delete
rclone delete --min-age 30d remote:kiroxy-backup/
```

Cron:
```
0 3 * * * /usr/local/bin/kiroxy-backup >> /var/log/kiroxy-backup.log 2>&1
```

### 6.3 Continuous replication with Litestream

For zero-RPO, wrap the SQLite in Litestream
(github.com/benbjohnson/litestream):

```yaml
# /etc/litestream.yml
dbs:
  - path: /var/lib/kiroxy/tokens.db
    replicas:
      - type: s3
        bucket: my-kiroxy-backup
        region: us-east-1
        endpoint: https://<b2|r2|minio-endpoint>
        access-key-id: ...
        secret-access-key: ...
        retention: 720h   # 30 days
        sync-interval: 10s
```

Litestream streams WAL pages to S3-compatible storage. Restore:
```bash
litestream restore -o /var/lib/kiroxy/tokens.db \
  s3://my-kiroxy-backup/tokens.db
```

### 6.4 Disaster recovery

Full host loss:
1. Spin up new host.
2. Install kiroxy (§1).
3. Restore `tokens.db` from backup into the expected path.
4. Set permissions: `chmod 0600 tokens.db && chown kiroxy:kiroxy`.
5. Start kiroxy. `kiroxy list-accounts` should show your accounts.
6. Update downstream clients' base URL / API key if changed.

RTO target: 10 minutes if you have backup automation. RPO target: 5
min with Litestream, 24h with daily cron.

---

## 7. Operational hygiene

### 7.1 Token rotation schedule

kiroxy handles token refresh automatically. Operator action only
needed when:
- An account's `refresh_token` is revoked (detected as
  `ErrRefreshUnauthorized` — see `kiroxy list-accounts`; failed
  accounts stay in `failed` state).
- Kiro rotates its API shape (detected via sudden pool-wide 4xx
  rate spike).

Quarterly: `kiroxy list-accounts` and confirm all healthy. Refresh
failed ones via re-onboard.

### 7.2 Account pool size

Recommended: 2-3 accounts for personal use. Rationale:
- One account covers baseline.
- Two gives you a hot spare during refresh hiccups.
- Three means you can be on a long session while Kiro cools one for
  throttling and another for a transient 5xx.

Adding accounts: use a secondary Gmail / Workspace email for each.
Each account = one free-tier subscription.

### 7.3 When to add a new account

Signal: `kiroxy_account_cooldowns_total{reason="quota"}` fires more
than once/week. You're hitting free-tier quota; add capacity.

### 7.4 Inbound key rotation

```bash
# Generate new key
NEW=$(openssl rand -hex 32)

# Update env file atomically
sudo install -m 0600 -o root -g root <(echo "KIROXY_API_KEY=$NEW") /etc/kiroxy/env.new
sudo mv /etc/kiroxy/env.new /etc/kiroxy/env

# Restart kiroxy
sudo systemctl restart kiroxy

# Update all downstream clients with the new key
```

Constant-time compare in kiroxy means rotation is just
"swap env + restart". No window of inconsistency (restart is faster
than any client retry).

### 7.5 Log discipline

kiroxy's JSON logs do NOT include tokens, headers, or bodies. Verify
periodically:
```bash
journalctl -u kiroxy -S '1 hour ago' | grep -iE 'aor[a-z0-9]|aoa[a-z0-9]|bearer ' | wc -l
# Should be 0
```

If you ever enable debug logging (`KIROXY_LOG_LEVEL=debug`), narrow
it to a short window — debug includes request headers which may be
sensitive (e.g., user's own API keys from downstream clients).

### 7.6 Upstream health check

Manually probe kiroxy's path to Kiro:
```bash
# Direct upstream reach (from kiroxy host)
curl -o /dev/null -sS -w "%{http_code} %{time_total}s\n" \
  https://runtime.us-east-1.kiro.dev/
# Expect: 403 (no auth) in <500ms
```

Useful to distinguish "kiroxy down" from "Kiro down" during
incidents.

---

## 8. Upgrade & rollback

### 8.1 Binary upgrade (systemd)

```bash
# Download new release
wget https://github.com/YOU/kiroxy/releases/download/v1.0.1/kiroxy_linux_amd64.tar.gz
wget https://github.com/YOU/kiroxy/releases/download/v1.0.1/kiroxy_v1.0.1_checksums.txt

# Verify checksum
grep kiroxy_linux_amd64.tar.gz kiroxy_v1.0.1_checksums.txt | sha256sum -c -

# Install atomically
tar xzf kiroxy_linux_amd64.tar.gz
sudo install -m 0755 -o root -g root kiroxy /usr/local/bin/kiroxy.new
sudo mv /usr/local/bin/kiroxy /usr/local/bin/kiroxy.old
sudo mv /usr/local/bin/kiroxy.new /usr/local/bin/kiroxy

# Restart
sudo systemctl restart kiroxy
sudo systemctl status kiroxy

# Verify
kiroxy version    # should show v1.0.1
curl -s http://127.0.0.1:8787/healthz | jq
```

### 8.2 Rollback

```bash
sudo mv /usr/local/bin/kiroxy.old /usr/local/bin/kiroxy
sudo systemctl restart kiroxy
kiroxy version
```

The vault schema is forward-only; kiroxy never runs destructive
migrations. Rolling back to a version that matches or predates the
current vault schema is safe.

### 8.3 Docker upgrade

```bash
docker compose pull kiroxy
docker compose up -d kiroxy
docker compose logs -f kiroxy
```

The `kiroxy-data` volume persists across container recreations. To
roll back: `docker compose down && image: kiroxy:v1.0.0 && up -d`.

### 8.4 Pre-upgrade checklist

Before any upgrade:
- [ ] Back up vault (§6).
- [ ] Read CHANGELOG.md between current version and target.
- [ ] Note any env-var renames or new required flags.
- [ ] Verify downstream clients will tolerate upgrade window (SSE
      streams cut off; they retry).

---

## 9. Runbooks

Triage playbooks for common incidents.

### 9.1 "Pool depleted — everything in cooldown"

```bash
kiroxy list-accounts
# Observe: every account in `cooldown` or `failed` state
```

Diagnosis tree:
1. Are you rate-limited? If all `reason=quota`, wait it out (1h
   default). Add another account if recurring.
2. Are refresh tokens revoked? If `failed` state, check
   `kiroxy debug-refresh --id <account>` for specific error.
3. Is Kiro itself down? Check
   `curl -I https://runtime.us-east-1.kiro.dev/` — if DNS fails or
   TCP times out, wait for Kiro.

Immediate unstick:
```bash
# Force-clear cooldown on one account (manual intervention — don't
# spam, you'll hit quota again)
sqlite3 /var/lib/kiroxy/tokens.db \
  "UPDATE token_bundles SET cooldown_until=0 WHERE connection_id='me';"
sudo systemctl restart kiroxy
```

### 9.2 "Downstream client returns 502 from kiroxy"

Look at the kiroxy log for the request_id the client reports:
```bash
journalctl -u kiroxy | grep <request_id>
```

Expected log lines tell you:
- 502 `upstream_error` → Kiro rejected. See
  docs/TROUBLESHOOTING.md for specific error shapes.
- 502 `streaming_error` → frame parse failure. Retry usually
  recovers.
- 502 `upstream returned empty response` → thinking-only response
  with no visible text. Known issue; retry with streaming.

### 9.3 "Onboarder keeps failing"

Symptoms: `kiroxy add-account` hangs at Camoufox browser step, or
the final token scrape fails.

Diagnosis:
1. Camoufox profile corrupt? Rm `tools/onboard/profiles/`.
2. Kiro's login flow changed? Visit
   `https://auth.desktop.kiro.dev/login` manually and complete
   signup. Re-run onboarder; it now inherits the warm session.
3. CAPTCHA / botcheck? Kiro has put humans through these before;
   log in manually in a real browser first, then re-run.

### 9.4 "Upstream latency suddenly doubled"

Check:
- Kiro status page (https://status.kiro.dev if it exists, else
  `status.aws.amazon.com` for `Amazon Q Developer`).
- kiroxy's upstream TTFB percentile:
  `histogram_quantile(0.95, rate(kiroxy_upstream_ttfb_seconds_bucket[5m]))`
- Your host's network egress: `mtr runtime.us-east-1.kiro.dev`.

If it's kiroxy: `go tool pprof` via `/debug/pprof/` — NOT shipped
by default, but added in debug builds. Unlikely to be kiroxy.

### 9.5 "I changed my Kiro password and now everything's broken"

Changing your Kiro account password REVOKES all refresh tokens.
Expected behavior:
```bash
kiroxy debug-refresh --id me
# ERROR: auth: refresh token rejected (401/403)
```

Fix: re-onboard.
```bash
kiroxy remove-account me
kiroxy add-account --label=me
# (interactive Camoufox flow or import fresh JSON)
```

This is the single most common incident. Document it to yourself
and to anyone you share kiroxy with.

### 9.6 "Vault file went missing"

If `/var/lib/kiroxy/tokens.db` is gone:
- Restore from backup (§6).
- If no backup: re-onboard every account. Hours of work. DON'T
  let this happen; automate §6.2.

---

## Appendix A: Production-ready docker-compose (extended)

Beyond what kiroxy ships with, for real deployment:

```yaml
name: kiroxy-stack

services:
  kiroxy:
    image: ghcr.io/YOU/kiroxy:v1.0.1   # pin exact version
    restart: unless-stopped
    environment:
      KIROXY_API_KEY: ${KIROXY_API_KEY}
      KIROXY_LOG_LEVEL: info
    volumes:
      - kiroxy-data:/data
    networks: [kiroxy-net]
    healthcheck:
      test: ["CMD", "kiroxy", "healthcheck"]
      interval: 30s
      timeout: 5s
      start_period: 10s
      retries: 3
    read_only: true
    cap_drop: [ALL]
    security_opt: [no-new-privileges:true]
    logging:
      driver: json-file
      options: { max-size: 10m, max-file: "5" }

  caddy:
    image: caddy:2-alpine
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile:ro
      - caddy-data:/data
      - caddy-config:/config
    networks: [kiroxy-net]
    depends_on: [kiroxy]

  prometheus:
    image: prom/prometheus:v2.54.1
    restart: unless-stopped
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml:ro
      - ./kiroxy-alerts.yml:/etc/prometheus/kiroxy-alerts.yml:ro
      - prom-data:/prometheus
    networks: [kiroxy-net]
    ports: ["127.0.0.1:9090:9090"]

  grafana:
    image: grafana/grafana:11
    restart: unless-stopped
    volumes:
      - grafana-data:/var/lib/grafana
    networks: [kiroxy-net]
    ports: ["127.0.0.1:3000:3000"]
    environment:
      GF_AUTH_ANONYMOUS_ENABLED: "false"

  litestream:
    image: litestream/litestream:0.3
    restart: unless-stopped
    command: replicate
    volumes:
      - kiroxy-data:/var/lib/kiroxy:ro
      - ./litestream.yml:/etc/litestream.yml:ro
    networks: [kiroxy-net]

networks:
  kiroxy-net:
    driver: bridge

volumes:
  kiroxy-data:
  caddy-data:
  caddy-config:
  prom-data:
  grafana-data:
```

That's kiroxy + TLS + monitoring + continuous backup in one compose
file. If a future friend asks "how do you run this in prod?" — this
is the answer.

---

*Last updated 2026-05-13 from research-v4 Phase S. Tested on
Debian 12, macOS 14, Fly.io. Mileage varies elsewhere.*
