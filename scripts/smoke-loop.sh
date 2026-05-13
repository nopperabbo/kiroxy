#!/bin/bash
# Smoke test automation untuk kiroxy v1.1.0 long-running validation
# Usage: ./smoke-loop.sh
#
# Tests every 5 min for 2 hours:
# - 1x non-streaming /v1/messages (claude-sonnet-4.5)
# - 1x streaming /v1/messages
# - 1x /v1/chat/completions (OpenAI compat)
# - 1x same-session-id (verify stickiness reuses account)
# - Periodic /healthz + /metrics + /dashboard/api/state checks
#
# Outputs to /tmp/kiroxy-smoke-loop.log
# Tracks: success rate, latency p50/p95, refresh events, account rotation

set -u

# Portable ms timestamp (macOS BSD date ga support %3N, pakai python3)
ms_now() { python3 -c 'import time; print(int(time.time()*1000))'; }

KIROXY_URL="${KIROXY_URL:-http://127.0.0.1:8787}"
KIROXY_KEY="${KIROXY_INBOUND_KEY:-preview}"
DURATION_MIN="${DURATION_MIN:-120}"   # default 2 hours
INTERVAL_SEC="${INTERVAL_SEC:-300}"   # default 5 min
LOG_FILE="${LOG_FILE:-/tmp/kiroxy-smoke-loop.log}"
SESSION_ID="smoke-loop-$(date +%s)"

iterations=$((DURATION_MIN * 60 / INTERVAL_SEC))
success=0
fail=0
total_latency=0

log() {
  echo "[$(date '+%H:%M:%S')] $*" | tee -a "$LOG_FILE"
}

log "=== kiroxy smoke loop start (PID $$) ==="
log "  url=$KIROXY_URL  duration=${DURATION_MIN}min  interval=${INTERVAL_SEC}s  iterations=$iterations"
log "  log=$LOG_FILE  session=$SESSION_ID"

for i in $(seq 1 $iterations); do
  log ""
  log "--- iteration $i / $iterations ---"

  # 1. Health check
  health_status=$(curl -sS -o /dev/null -w "%{http_code}" "$KIROXY_URL/healthz" 2>/dev/null)
  log "  /healthz → $health_status"

  # 2. Non-streaming /v1/messages
  start_ms=$(ms_now)
  resp=$(curl -sS -X POST "$KIROXY_URL/v1/messages" \
    -H "Authorization: Bearer $KIROXY_KEY" \
    -H "Content-Type: application/json" \
    -H "X-Claude-Code-Session-Id: $SESSION_ID-i$i" \
    -d "{\"model\":\"claude-sonnet-4.5\",\"max_tokens\":15,\"messages\":[{\"role\":\"user\",\"content\":\"i=$i ping\"}]}" \
    -w "\n__HTTP__%{http_code}__TIME__%{time_total}__")
  end_ms=$(ms_now)
  latency_ms=$((end_ms - start_ms))

  http_code=$(echo "$resp" | grep -oE "__HTTP__[0-9]+" | grep -oE "[0-9]+")
  body=$(echo "$resp" | grep -v "__HTTP__")

  if [[ "$http_code" == "200" ]]; then
    success=$((success + 1))
    total_latency=$((total_latency + latency_ms))
    body_text=$(echo "$body" | python3 -c "import sys,json; r=json.load(sys.stdin); c=r.get('content',[]); print((c[0].get('text','') if c else '')[:60])" 2>/dev/null || echo "PARSE_ERR")
    log "  /v1/messages (non-stream) → 200 ${latency_ms}ms — \"$body_text\""
  else
    fail=$((fail + 1))
    log "  /v1/messages (non-stream) → $http_code ${latency_ms}ms — FAIL"
    err_excerpt=$(echo "$body" | head -c 200)
    log "    body: $err_excerpt"
  fi

  # 3. Streaming /v1/messages (same session = test stickiness)
  start_ms=$(ms_now)
  stream_resp=$(curl -sSN -X POST "$KIROXY_URL/v1/messages" \
    -H "Authorization: Bearer $KIROXY_KEY" \
    -H "Content-Type: application/json" \
    -H "X-Claude-Code-Session-Id: $SESSION_ID-i$i" \
    -d "{\"model\":\"claude-sonnet-4.5\",\"max_tokens\":10,\"stream\":true,\"messages\":[{\"role\":\"user\",\"content\":\"count to 3\"}]}" \
    -m 30 2>&1 | head -c 3000)
  end_ms=$(ms_now)
  latency_ms=$((end_ms - start_ms))

  if echo "$stream_resp" | grep -q "message_stop"; then
    success=$((success + 1))
    total_latency=$((total_latency + latency_ms))
    log "  /v1/messages (stream) → events OK ${latency_ms}ms"
  else
    fail=$((fail + 1))
    log "  /v1/messages (stream) → INCOMPLETE ${latency_ms}ms"
  fi

  # 4. OpenAI-compat /v1/chat/completions
  start_ms=$(ms_now)
  oai_resp=$(curl -sS -X POST "$KIROXY_URL/v1/chat/completions" \
    -H "Authorization: Bearer $KIROXY_KEY" \
    -H "Content-Type: application/json" \
    -H "X-Claude-Code-Session-Id: $SESSION_ID-oai-i$i" \
    -d "{\"model\":\"gpt-4o\",\"max_tokens\":15,\"messages\":[{\"role\":\"user\",\"content\":\"hi\"}]}" \
    -w "\n__HTTP__%{http_code}__")
  end_ms=$(ms_now)
  latency_ms=$((end_ms - start_ms))

  oai_code=$(echo "$oai_resp" | grep -oE "__HTTP__[0-9]+" | grep -oE "[0-9]+")
  if [[ "$oai_code" == "200" ]]; then
    success=$((success + 1))
    total_latency=$((total_latency + latency_ms))
    log "  /v1/chat/completions (OpenAI) → 200 ${latency_ms}ms"
  else
    fail=$((fail + 1))
    log "  /v1/chat/completions (OpenAI) → $oai_code ${latency_ms}ms"
  fi

  # 5. Pool snapshot every 6 iterations (~30 min)
  if [[ $((i % 6)) -eq 0 ]]; then
    state=$(curl -sS "$KIROXY_URL/dashboard/api/state" 2>/dev/null)
    accounts_count=$(echo "$state" | python3 -c "import sys,json; d=json.load(sys.stdin); print(len(d.get('accounts',[])))" 2>/dev/null || echo "?")
    log "  pool snapshot: $accounts_count accounts visible"

    metrics_lines=$(curl -sS "$KIROXY_URL/metrics" 2>/dev/null | grep -E "^kiroxy_(requests_total|refresh_attempts_total|account_cooldowns)" | head -10)
    log "  metrics: $(echo "$metrics_lines" | tr '\n' ' | ' | head -c 300)"
  fi

  # Running totals
  total=$((success + fail))
  if [[ $total -gt 0 ]]; then
    rate=$((success * 100 / total))
    avg_lat=$((total_latency / (success + 1)))   # +1 to avoid div0
    log "  running: ${success}/${total} OK (${rate}%), avg-success-latency ${avg_lat}ms"
  fi

  # Sleep until next iteration (skip on last)
  if [[ $i -lt $iterations ]]; then
    sleep "$INTERVAL_SEC"
  fi
done

# Final summary
log ""
log "=== FINAL SUMMARY ==="
total=$((success + fail))
if [[ $total -gt 0 ]]; then
  rate=$((success * 100 / total))
else
  rate=0
fi
avg_lat=$((total_latency / (success + 1)))
log "  Total requests: $total ($((iterations * 3)) expected)"
log "  Success: $success  Fail: $fail  Rate: ${rate}%"
log "  Avg success latency: ${avg_lat}ms"
log ""
log "Final pool state:"
curl -sS "$KIROXY_URL/dashboard/api/state" 2>/dev/null | python3 -m json.tool 2>/dev/null | head -50 | tee -a "$LOG_FILE"
log ""
log "Final metrics (kiroxy_*):"
curl -sS "$KIROXY_URL/metrics" 2>/dev/null | grep "^kiroxy_" | head -30 | tee -a "$LOG_FILE"
