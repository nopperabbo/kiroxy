#!/usr/bin/env bash
# run.sh - orchestrate a full load-test pass.
#
# Two modes:
#
#   --mode kiro    (default, isolated)
#     Starts mock_kiro locally, runs harness against it in --mode kiro.
#     Produces reference upper-bound numbers for the eventstream pipeline.
#     No real kiroxy binary or credentials needed.
#
#   --mode kiroxy  (end-to-end)
#     Requires a running kiroxy on $KIROXY_URL (default http://127.0.0.1:8787)
#     with at least one account configured. Runs the anthropic-mode scenarios
#     against it. Does NOT start kiroxy for you — that is out of scope for
#     this harness because it would require credential provisioning.
#
# Usage:
#   ./run.sh                          # all scenarios, mode=kiro, default out dir
#   ./run.sh --mode kiroxy            # hit a real kiroxy
#   ./run.sh --scenarios light,burst  # subset
#   ./run.sh --out /tmp/lt_run1       # custom output dir

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &> /dev/null && pwd)"
cd "$SCRIPT_DIR"

# Defaults.
MODE="kiro"
KIROXY_URL="${KIROXY_URL:-http://127.0.0.1:8787}"
MOCK_PORT="${MOCK_PORT:-9790}"
OUT_BASE="${LOADTEST_OUT:-./results}"
SCENARIOS=""
MOCK_LATENCY_MS=0
MOCK_ERROR_RATE=0
KEEP_MOCK=0

usage() {
  cat <<EOF
Usage: $0 [options]

Options:
  --mode <kiro|kiroxy>     target mode (default: kiro)
  --kiroxy-url <url>       kiroxy base URL (default: \$KIROXY_URL or http://127.0.0.1:8787)
  --mock-port <port>       port for mock_kiro (default: 9790)
  --mock-latency-ms <N>    inject latency into mock responses (default: 0)
  --mock-error-rate <F>    inject error rate into mock (default: 0.0)
  --scenarios <list>       comma-sep scenario names (default: all applicable)
  --out <dir>              output directory (default: ./results)
  --keep-mock              do not kill mock_kiro on exit (useful for debugging)
  -h, --help               show this help

Scenarios available (see scenarios.yaml):
  light sustained burst streaming sustained_5m mock_light mock_burst

Example:
  $0 --mode kiro --scenarios mock_light,mock_burst --out ./results/\$(date +%Y%m%d_%H%M%S)
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --mode)            MODE="$2"; shift 2 ;;
    --kiroxy-url)      KIROXY_URL="$2"; shift 2 ;;
    --mock-port)       MOCK_PORT="$2"; shift 2 ;;
    --mock-latency-ms) MOCK_LATENCY_MS="$2"; shift 2 ;;
    --mock-error-rate) MOCK_ERROR_RATE="$2"; shift 2 ;;
    --scenarios)       SCENARIOS="$2"; shift 2 ;;
    --out)             OUT_BASE="$2"; shift 2 ;;
    --keep-mock)       KEEP_MOCK=1; shift ;;
    -h|--help)         usage; exit 0 ;;
    *) echo "unknown arg: $1" >&2; usage; exit 2 ;;
  esac
done

if [[ "$MODE" != "kiro" && "$MODE" != "kiroxy" ]]; then
  echo "error: --mode must be kiro or kiroxy (got: $MODE)" >&2
  exit 2
fi

# Ensure a Go toolchain is available.
if ! command -v go >/dev/null 2>&1; then
  echo "error: go not found on PATH" >&2
  exit 127
fi

# Build binaries.
echo ">>> building harness + mock_kiro"
BIN_DIR="$(mktemp -d -t kiroxy-loadtest-XXXXXX)"
trap 'cleanup' EXIT

MOCK_PID=""
cleanup() {
  if [[ -n "$MOCK_PID" && "$KEEP_MOCK" -eq 0 ]]; then
    kill "$MOCK_PID" 2>/dev/null || true
    wait "$MOCK_PID" 2>/dev/null || true
  fi
  rm -rf "$BIN_DIR"
}

(cd mock_kiro && go build -o "$BIN_DIR/mock_kiro" .)
go build -o "$BIN_DIR/harness" .

# Prepare output directory.
RUN_STAMP="$(date +%Y%m%d_%H%M%S)"
OUT_DIR="$OUT_BASE/$RUN_STAMP"
mkdir -p "$OUT_DIR"
echo ">>> output: $OUT_DIR"

# Start mock_kiro if we're in kiro mode. Also start it in kiroxy mode so
# operators can redirect kiroxy at it if their build supports it (currently
# requires WithBaseURL in source; see README.md).
echo ">>> starting mock_kiro on :$MOCK_PORT"
"$BIN_DIR/mock_kiro" \
  --addr "127.0.0.1:$MOCK_PORT" \
  --latency-ms "$MOCK_LATENCY_MS" \
  --error-rate "$MOCK_ERROR_RATE" \
  --stream-events 8 > "$OUT_DIR/mock_kiro.log" 2>&1 &
MOCK_PID=$!

# Wait for mock to be ready.
for i in {1..20}; do
  if curl -sf "http://127.0.0.1:$MOCK_PORT/healthz" >/dev/null 2>&1; then
    break
  fi
  if [[ $i -eq 20 ]]; then
    echo "error: mock_kiro did not come up" >&2
    cat "$OUT_DIR/mock_kiro.log" >&2
    exit 1
  fi
  sleep 0.1
done

# In kiroxy mode, probe the kiroxy endpoint so we fail fast if it is down.
if [[ "$MODE" == "kiroxy" ]]; then
  if ! curl -sf "$KIROXY_URL/healthz" >/dev/null 2>&1; then
    echo "error: kiroxy at $KIROXY_URL/healthz is not responding" >&2
    echo "       run 'kiroxy serve' in another terminal, or pass --kiroxy-url" >&2
    exit 1
  fi
fi

# Parse scenarios.yaml with awk into a pipe-delimited record file so we can
# stay compatible with macOS bash 3.2 (no associative arrays). Format:
#   name|concurrency|total|duration|stream|mode|description
SCEN_INDEX="$BIN_DIR/scenarios.idx"
awk '
  BEGIN { cur=""; conc=""; total=""; dur=""; stream="false"; smode=""; desc="" }
  function flush() {
    if (cur != "") {
      printf "%s|%s|%s|%s|%s|%s|%s\n", cur, conc, total, dur, stream, smode, desc
    }
    cur=""; conc=""; total=""; dur=""; stream="false"; smode=""; desc=""
  }
  # strip inline comments
  { sub(/#.*$/, "") }
  # top-level scenario key: two spaces + name:
  /^  [A-Za-z_][A-Za-z0-9_]*:[[:space:]]*$/ {
    flush()
    name = $1
    sub(/:$/, "", name)
    cur = name
    next
  }
  # key: value inside a scenario (four spaces indent)
  /^    [a-z_]+:/ {
    if (cur == "") next
    line = $0
    sub(/^    /, "", line)
    n = index(line, ":")
    k = substr(line, 1, n-1)
    v = substr(line, n+1)
    sub(/^[[:space:]]+/, "", v)
    sub(/[[:space:]]+$/, "", v)
    gsub(/^"|"$/, "", v)
    if (k == "concurrency") conc = v
    else if (k == "total") total = v
    else if (k == "duration") dur = v
    else if (k == "stream") stream = v
    else if (k == "mode") smode = v
    else if (k == "description") desc = v
  }
  END { flush() }
' scenarios.yaml > "$SCEN_INDEX"

scen_field() {
  # $1 scenario name, $2 field index (2..7)
  awk -F'|' -v n="$1" -v f="$2" '$1 == n { print $f; exit }' "$SCEN_INDEX"
}
scen_exists() {
  awk -F'|' -v n="$1" '$1 == n { print "1"; exit }' "$SCEN_INDEX"
}

# Determine scenario list.
if [[ -z "$SCENARIOS" ]]; then
  if [[ "$MODE" == "kiro" ]]; then
    # Reference scenarios + the anthropic ones translated to kiro target.
    # We only run scenarios that have mode=kiro when targeting the mock, else
    # override target to kiro (so light/sustained/burst/streaming work in
    # isolation too).
    SCENARIOS="mock_light,mock_burst,light,sustained,burst,streaming"
  else
    SCENARIOS="light,sustained,burst,streaming,sustained_5m"
  fi
fi
IFS=',' read -r -a SCEN_LIST <<< "$SCENARIOS"

# Run each scenario.
ANY_FAILED=0
for name in "${SCEN_LIST[@]}"; do
  if [[ "$(scen_exists "$name")" != "1" ]]; then
    echo "skip unknown scenario: $name" >&2
    continue
  fi
  s_conc="$(scen_field "$name" 2)"
  s_total="$(scen_field "$name" 3)"
  s_dur="$(scen_field "$name" 4)"
  s_stream="$(scen_field "$name" 5)"
  s_mode="$(scen_field "$name" 6)"
  s_desc="$(scen_field "$name" 7)"

  # Resolve target URL based on scenario mode and run mode.
  scen_mode="$s_mode"
  if [[ "$MODE" == "kiro" && "$scen_mode" == "anthropic" ]]; then
    # Anthropic scenario forced into kiro mode (harness mode=kiro against mock).
    target_url="http://127.0.0.1:$MOCK_PORT"
    harness_mode="kiro"
  elif [[ "$scen_mode" == "kiro" ]]; then
    target_url="http://127.0.0.1:$MOCK_PORT"
    harness_mode="kiro"
  else
    target_url="$KIROXY_URL"
    harness_mode="anthropic"
  fi

  scen_out="$OUT_DIR/$name"
  mkdir -p "$scen_out"

  args=(
    "--url" "$target_url"
    "--mode" "$harness_mode"
    "--concurrency" "$s_conc"
    "--scenario" "$name"
    "--out" "$scen_out"
  )
  if [[ -n "$s_dur" ]]; then
    args+=("--duration" "$s_dur")
  else
    args+=("--total" "$s_total")
  fi
  if [[ "$s_stream" == "true" ]]; then
    args+=("--stream")
  fi

  echo ""
  echo "=== scenario: $name ==="
  echo "target: $target_url mode=$harness_mode"
  echo "note:   $s_desc"
  if "$BIN_DIR/harness" "${args[@]}"; then
    :
  else
    echo "scenario $name failed" >&2
    ANY_FAILED=1
  fi
done

# Always emit a combined summary.
echo ""
echo ">>> combined summary: $OUT_DIR"
./analyze.sh "$OUT_DIR" > "$OUT_DIR/report.md" || true
echo "report: $OUT_DIR/report.md"

exit "$ANY_FAILED"
