#!/usr/bin/env bash
# Batch runner for Phase G.FIX — sequential, fresh profile per account.
#
# Differences from run_pilot.sh:
#   - Each account gets a fresh --profile-dir under /tmp/kiroxy_profiles/
#     (avoids collision with pre-existing poisoned profile state in
#      tools/onboard/profiles_data/).
#   - Per-domain inter-run sleep to spread rate-limit pressure.
#
# Usage: ./run_batch.sh <pwd> <email1> [email2 ...]

set -u
PASSWORD="$1"; shift
OUT_DIR="/tmp/kiroxy_onb"
LOG_DIR="$OUT_DIR/logs"
PROF_DIR="/tmp/kiroxy_profiles"
mkdir -p "$OUT_DIR" "$LOG_DIR" "$PROF_DIR"

cd "$(dirname "$0")"
source .venv/bin/activate

OK=0
FAIL=0
SKIP=0
START_ALL=$(date +%s)
PREV_DOMAIN=""
SLEEP_SAME_DOMAIN=45
SLEEP_NEW_DOMAIN=15

for email in "$@"; do
  safe="${email//@/_at_}"
  out="$OUT_DIR/${safe}.json"
  log="$LOG_DIR/${safe}.log"
  prof="$PROF_DIR/${safe}"
  domain="${email#*@}"
  start=$(date +%s)

  printf '%s ▶ %-45s ' "$(date '+%H:%M:%S')" "$email"

  if [ -f "$out" ]; then
    echo "SKIP (already exists)"
    SKIP=$((SKIP+1))
    PREV_DOMAIN="$domain"
    continue
  fi

  # Inter-run pacing: longer pause if same Workspace domain as previous
  # (Kiro rate-limits by org; Google rate-limits by IP + account family).
  if [ -n "$PREV_DOMAIN" ] && [ "$OK" -gt 0 -o "$FAIL" -gt 0 ]; then
    if [ "$domain" = "$PREV_DOMAIN" ]; then
      sleep "$SLEEP_SAME_DOMAIN"
    else
      sleep "$SLEEP_NEW_DOMAIN"
    fi
  fi

  if echo "$PASSWORD" | python3 onboard.py \
      --email "$email" \
      --password - \
      --provider google \
      --output "$out" \
      --profile-dir "$prof" \
      --challenge-mode auto \
      --timeout-login-s 180 \
      --headless > "$log" 2>&1; then
    elapsed=$(( $(date +%s) - start ))
    OK=$((OK+1))
    arn=$(python3 -c "import json; d=json.load(open('$out')); print(d[0]['profileArn'].rsplit('/',1)[-1])" 2>/dev/null || echo '?')
    echo "OK ${elapsed}s  profile=$arn"
  else
    elapsed=$(( $(date +%s) - start ))
    FAIL=$((FAIL+1))
    last=$(tail -5 "$log" | grep -E "error:|FAIL|Timeout" | head -1 | cut -c1-80)
    echo "FAIL ${elapsed}s  ${last:-(see log)}"
  fi
  PREV_DOMAIN="$domain"
done

TOTAL_ELAPSED=$(( $(date +%s) - START_ALL ))
echo
echo "=================================================="
echo "  Total: $#  OK: $OK  FAIL: $FAIL  SKIP: $SKIP  Elapsed: ${TOTAL_ELAPSED}s"
echo "  Outputs: $OUT_DIR/"
echo "  Logs:    $LOG_DIR/"
echo "  Profile scratch: $PROF_DIR/"
echo "=================================================="
