#!/usr/bin/env bash
# Retry failed accounts with --challenge-mode manual (operator solves
# Google challenges by hand).
#
# Reads failed emails from /tmp/kiroxy_onb/FAILED.txt (regenerated each run).
# For each, launches onboard.py in manual mode so the operator can sign in
# by hand in the Camoufox window.
#
# Usage: ./retry_failures.sh <password>

set -u
PASSWORD="$1"
OUT_DIR="/tmp/kiroxy_onb"
LOG_DIR="$OUT_DIR/logs_retry"
PROF_DIR="/tmp/kiroxy_profiles_retry"
FAIL_LIST="$OUT_DIR/FAILED.txt"

mkdir -p "$OUT_DIR" "$LOG_DIR" "$PROF_DIR"

cd "$(dirname "$0")"
source .venv/bin/activate

# Rebuild FAIL list from BATCH_RUN.log.
if [ ! -f "$OUT_DIR/BATCH_RUN.log" ]; then
  echo "error: $OUT_DIR/BATCH_RUN.log missing — was batch ever run?"
  exit 1
fi

grep " FAIL " "$OUT_DIR/BATCH_RUN.log" \
  | awk '{print $3}' \
  | sort -u > "$FAIL_LIST"

N=$(wc -l < "$FAIL_LIST" | tr -d ' ')
if [ "$N" = "0" ]; then
  echo "no FAILs in BATCH_RUN.log; nothing to retry"
  exit 0
fi

echo "retry plan: $N account(s) in --challenge-mode manual"
cat "$FAIL_LIST"
echo
echo "For each, a Camoufox window will open. Sign in manually when prompted,"
echo "then press ENTER in this terminal. Do NOT use --headless (need to see the window)."
echo
read -p "Proceed? [y/N] " yn
case "$yn" in
  [yY]*) ;;
  *) echo "aborted"; exit 0 ;;
esac

OK=0
FAIL=0
while IFS= read -r email; do
  [ -z "$email" ] && continue
  safe="${email//@/_at_}"
  out="$OUT_DIR/${safe}.json"
  log="$LOG_DIR/${safe}.log"
  prof="$PROF_DIR/${safe}"

  if [ -f "$out" ]; then
    echo "SKIP ${email} (already retrieved)"
    continue
  fi

  printf '%s ▶ %s (manual) ... ' "$(date '+%H:%M:%S')" "$email"

  if echo "$PASSWORD" | python3 onboard.py \
      --email "$email" \
      --password - \
      --provider google \
      --output "$out" \
      --profile-dir "$prof" \
      --challenge-mode manual \
      --timeout-login-s 600 \
      > "$log" 2>&1; then
    OK=$((OK+1))
    echo "OK"
  else
    FAIL=$((FAIL+1))
    echo "FAIL — see $log"
  fi
done < "$FAIL_LIST"

echo
echo "retry summary: OK=$OK  FAIL=$FAIL"
