#!/usr/bin/env bash
# analyze.sh - render a markdown report from a load-test run directory.
#
# Usage:
#   ./analyze.sh <run-dir>
#
# Walks every subdir containing a summary.json and emits a markdown report
# comparing scenarios. Intended to be run by run.sh but safe to run standalone
# on any past run directory.

set -euo pipefail

RUN_DIR="${1:-}"
if [[ -z "$RUN_DIR" || ! -d "$RUN_DIR" ]]; then
  echo "usage: $0 <run-dir>" >&2
  exit 2
fi

if ! command -v python3 >/dev/null 2>&1; then
  echo "error: python3 not found on PATH (needed for JSON parsing)" >&2
  exit 127
fi

RUN_DIR="$(cd "$RUN_DIR" && pwd)"
cat <<HDR
# kiroxy load-test report

Run directory: \`$RUN_DIR\`
Generated:     $(date -u +"%Y-%m-%dT%H:%M:%SZ")

## Summary per scenario

| scenario | mode | conc | requests | ok | err | err% | dur (s) | RPS | p50 | p95 | p99 | max | TTFB p50 | TTFB p95 |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
HDR

python3 - "$RUN_DIR" <<'PY'
import json, os, sys, glob
run = sys.argv[1]
rows = []
for p in sorted(glob.glob(os.path.join(run, "*", "summary.json"))):
    with open(p) as f:
        s = json.load(f)
    rows.append(s)

def fmt(x, spec=".1f", zero_as_dash=False):
    if x is None:
        return "-"
    if zero_as_dash and x == 0:
        return "-"
    try:
        return format(x, spec)
    except Exception:
        return str(x)

for s in rows:
    print("| {name} | {mode} | {conc} | {total} | {ok} | {err} | {errp} | {dur} | {rps} | {p50} | {p95} | {p99} | {mx} | {t50} | {t95} |".format(
        name=s.get("scenario", "?"),
        mode=s.get("mode", "?") + ("/stream" if s.get("stream") else ""),
        conc=s.get("concurrency", 0),
        total=s.get("total_requests", 0),
        ok=s.get("successes", 0),
        err=s.get("errors", 0),
        errp=fmt(s.get("error_rate", 0) * 100, ".1f") + "%",
        dur=fmt(s.get("total_duration_s", 0), ".2f"),
        rps=fmt(s.get("rps", 0), ".1f"),
        p50=fmt(s.get("latency_p50_ms", 0), ".1f"),
        p95=fmt(s.get("latency_p95_ms", 0), ".1f"),
        p99=fmt(s.get("latency_p99_ms", 0), ".1f"),
        mx=fmt(s.get("latency_max_ms", 0), ".1f"),
        t50=fmt(s.get("ttfb_p50_ms", 0), ".1f", zero_as_dash=True),
        t95=fmt(s.get("ttfb_p95_ms", 0), ".1f", zero_as_dash=True),
    ))
PY

cat <<DOC

## Environment

| field | value |
|---|---|
DOC

python3 - "$RUN_DIR" <<'PY'
import json, os, sys, glob
run = sys.argv[1]
# Prefer any summary.json for system info — they all record the same host.
files = sorted(glob.glob(os.path.join(run, "*", "summary.json")))
if not files:
    sys.exit(0)
with open(files[0]) as f:
    s = json.load(f)
fields = [
    ("go version", s.get("sys_go_version", "?")),
    ("OS",         s.get("sys_os", "?")),
    ("arch",       s.get("sys_arch", "?")),
    ("CPUs",       s.get("sys_cpu", "?")),
]
for k, v in fields:
    print(f"| {k} | {v} |")
PY

cat <<'DOC'

## Interpretation

Rows of interest, in priority order:

1. **error%** — anything > 0 during light/sustained scenarios is a red flag.
   Burst can tolerate a small non-zero rate from connection queueing.
2. **p99 latency** — sudden jumps vs. a prior baseline are the first sign
   of regression. Streaming scenarios naturally have higher p99 because
   the latency metric covers the full response.
3. **RPS** — for a fixed scenario on the same host, this should be stable
   ±10% across runs. Large drops indicate throughput regression.

A healthy baseline looks like:
- `light`     p99 < 10× the mock latency (or < 50ms if mock latency is 0)
- `sustained` p99 < 5× p50
- `burst`     err% < 1%, p99 < 10× p50
- `streaming` p50 roughly equal to the stream duration (chunk_delay * events)

Compare against `docs/BENCHMARKS.md` for the reference numbers.
DOC
