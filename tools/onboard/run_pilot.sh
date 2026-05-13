#!/usr/bin/env bash
# Pilot runner for Phase G.FIX — sequential, one account at a time.
# Each account writes to its own JSON; one log per account.
# Usage: ./run_pilot.sh <pwd> <email1> [email2 ...]

set -u
PASSWORD="$1"; shift
OUT_DIR="/tmp/kiroxy_onb"
LOG_DIR="$OUT_DIR/logs"
mkdir -p "$OUT_DIR" "$LOG_DIR"

cd "$(dirname "$0")"
source .venv/bin/activate

OK=0
FAIL=0
START_ALL=$(date +%s)

for email in "$@"; do
  safe="${email//@/_at_}"
  out="$OUT_DIR/${safe}.json"
  log="$LOG_DIR/${safe}.log"
  start=$(date +%s)

  printf '%s ▶ %-45s ' "$(date '+%H:%M:%S')" "$email"
  
  if [ -f "$out" ]; then
    echo "SKIP (already exists)"
    OK=$((OK+1))
    continue
  fi

  # Drop into the venv'd python; password via stdin.
  if echo "$PASSWORD" | python3 onboard.py \
      --email "$email" \
      --password - \
      --provider google \
      --output "$out" \
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
    last=$(tail -3 "$log" | tr '\n' ' ' | cut -c1-100)
    echo "FAIL ${elapsed}s  $last"
  fi
done

TOTAL_ELAPSED=$(( $(date +%s) - START_ALL ))
echo
echo "=================================================="
echo "  Total: $#  OK: $OK  FAIL: $FAIL  Elapsed: ${TOTAL_ELAPSED}s"
echo "  Outputs: $OUT_DIR/"
echo "  Logs:    $LOG_DIR/"
echo "=================================================="
