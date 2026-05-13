#!/usr/bin/env bash
# Merge per-account kiro_tokens JSON files into one array, dedupe by email
# (matching the Python-side _upsert change by the parallel phase), and
# optionally import to kiroxy vault.
#
# Usage: ./merge_and_import.sh [--import <KIROXY_DB_PATH>]

set -u
OUT_DIR="/tmp/kiroxy_onb"
MERGED="$OUT_DIR/kiro_tokens_merged.json"

mkdir -p "$OUT_DIR"
cd "$(dirname "$0")"

IMPORT=""
DB_PATH=""
while [ $# -gt 0 ]; do
  case "$1" in
    --import)
      IMPORT=1
      DB_PATH="${2:-}"
      shift 2
      ;;
    *) echo "unknown arg: $1"; exit 1 ;;
  esac
done

python3 <<'PY'
import json, glob, sys, os
out_dir = '/tmp/kiroxy_onb'
entries = []
seen = set()
files = sorted(glob.glob(os.path.join(out_dir, '*.json')))
for f in files:
    # skip merged file from prior runs
    if f.endswith('kiro_tokens_merged.json'):
        continue
    try:
        data = json.load(open(f))
    except Exception as e:
        print(f'warn: {f}: {e}', file=sys.stderr)
        continue
    if not isinstance(data, list):
        continue
    for e in data:
        if not isinstance(e, dict):
            continue
        # dedupe by email if present (post-4c4d562 schema), else by accessToken prefix
        key = (e.get('email') or '').lower().strip()
        if not key:
            key = 'at:' + (e.get('accessToken') or '')[:24]
        if key in seen:
            continue
        seen.add(key)
        entries.append(e)

with open(os.path.join(out_dir, 'kiro_tokens_merged.json'), 'w') as f:
    json.dump(entries, f, indent=2)
    f.write('\n')
os.chmod(os.path.join(out_dir, 'kiro_tokens_merged.json'), 0o600)
print(f'merged {len(entries)} unique account(s) from {len(files)} file(s) → {out_dir}/kiro_tokens_merged.json')
PY

if [ -n "$IMPORT" ]; then
  if [ -z "$DB_PATH" ]; then
    echo "error: --import requires a path to the kiroxy DB"
    exit 1
  fi
  kx="$(cd ../.. && pwd)/kiroxy"
  if [ ! -x "$kx" ]; then
    echo "error: kiroxy binary not found at $kx; run 'make' in repo root first"
    exit 1
  fi
  echo
  echo "=== dry-run ==="
  KIROXY_DB_PATH="$DB_PATH" "$kx" import-accounts-json -file "$MERGED" -provider kiro -dry-run
  echo
  read -p "Proceed with real import? [y/N] " yn
  case "$yn" in
    [yY]*)
      KIROXY_DB_PATH="$DB_PATH" "$kx" import-accounts-json -file "$MERGED" -provider kiro
      echo
      KIROXY_DB_PATH="$DB_PATH" "$kx" list-accounts
      ;;
    *) echo "aborted; merged JSON still at $MERGED" ;;
  esac
fi
