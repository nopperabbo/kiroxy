# 79-Account Batch Run — Runbook & Status

> Live operational doc for the batch onboarded 2026-05-13 starting 18:48 WITA.
> Updates inline as state changes. Authoritative source: `/tmp/kiroxy_onb/BATCH_RUN.log`.

## Quick status check

```bash
OK=$(grep -c " OK " /tmp/kiroxy_onb/BATCH_RUN.log)
FAIL=$(grep -c " FAIL " /tmp/kiroxy_onb/BATCH_RUN.log)
SKIP=$(grep -c "SKIP " /tmp/kiroxy_onb/BATCH_RUN.log)
echo "OK=$OK  FAIL=$FAIL  SKIP=$SKIP"
ps -p $(cat /tmp/kiroxy_onb/BATCH.pid) > /dev/null && echo running || echo done
tail -5 /tmp/kiroxy_onb/BATCH_RUN.log
```

## When batch finishes (exit playbook)

Step 1 — confirm completion:
```bash
ps -p $(cat /tmp/kiroxy_onb/BATCH.pid) > /dev/null && echo "still running" || echo "DONE"
```

Step 2 — count outcomes:
```bash
ls /tmp/kiroxy_onb/*.json | grep -v merged | wc -l   # successful tokens
grep -c " FAIL " /tmp/kiroxy_onb/BATCH_RUN.log       # failures
```

Step 3 — merge per-account JSONs:
```bash
cd /Users/mac/Desktop/Self\ Hosted\ Proxy\ AI/kiroxy/tools/onboard
./merge_and_import.sh
# produces /tmp/kiroxy_onb/kiro_tokens_merged.json (mode 0600, deduped by email)
```

Step 4 — dry-run import to vault:
```bash
cd /Users/mac/Desktop/Self\ Hosted\ Proxy\ AI/kiroxy
./kiroxy import-accounts-json -file /tmp/kiroxy_onb/kiro_tokens_merged.json -provider kiro -dry-run
```

Step 5 — real import:
```bash
./kiroxy import-accounts-json -file /tmp/kiroxy_onb/kiro_tokens_merged.json -provider kiro
./kiroxy list-accounts
```

Step 6 — retry the FAILed accounts (operator-attended, with browser visible):
```bash
cd tools/onboard
./retry_failures.sh 'Masuk123'
# reads /tmp/kiroxy_onb/BATCH_RUN.log → /tmp/kiroxy_onb/FAILED.txt
# launches each in --challenge-mode manual; solve the Google challenge
# in the Camoufox window then press ENTER in the terminal.
```

After successful retries, re-run merge+import (Steps 3–5).

## Known FAIL patterns

| Pattern | Cause | Action |
|---|---|---|
| `connection_check` challenge | Google probing fingerprint at /signin/challenge/pwd | retry with `--challenge-mode manual` |
| `NS_ERROR_UNKNOWN_PROTOCOL` | Stale Firefox session-restore in profile dir | use a clean `--profile-dir` (batch runner does this already) |
| `Timeout: 600s` | Batch is unattended; challenge-mode=auto prompted then timed out | retry attended |
| `selector hidden 30s` (resolved) | Google `hiddenPassword` honeypot | already fixed in commit 9f9a4e6 |

## Workspace profileArn collision

All Workspace accounts in this batch return `profileArn=...EHGA3GRVQMUK` regardless of `@<domain>.tech` — Google Workspace orgs collapse to one Kiro CodeWhisperer profile.

This is **NOT a bug**. The Go-side `deriveAccountID` (4-layer cascade in commit f659632) uses the email address as the primary vault key, so each account becomes its own vault entry by email. Verified via dry-run: 30 entries → 30 unique vault keys derived `from email`.

## Cleanup

After successful import:
```bash
# Remove ephemeral profile dirs (each ~50-100MB):
rm -rf /tmp/kiroxy_profiles/

# Keep the per-account JSONs for ~7 days for re-import / debugging,
# then remove:
# rm -rf /tmp/kiroxy_onb/
```
