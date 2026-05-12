#!/usr/bin/env python3
"""kiroxy batch onboarder — orchestrate N single-account onboards (G.3).

Reads an `email:password` file, drives `onboard.py` as a subprocess per
account, and persists per-email state so overnight / unattended runs can
resume after interruption. Single-threaded by design for v1: Google
rate-limits aggressively on IPs, and parallel onboarding burns through
the risk-score budget fast. Add --parallel in a future iteration once
we've measured what's safe.

Core guarantees:

  1. **Resume.** State file (`batch_state.json`) persists per-email status.
     Re-running the same command skips accounts that already succeeded and
     retries failed ones up to `--max-retries`.

  2. **Rate limit.** Between accounts, sleep `--cooldown-s` (default 60).
     Operators can tune for their proxy / account freshness.

  3. **Failure classification.** Parse exit code + stderr of the onboard
     subprocess.
       - transient → retry with backoff, up to --max-retries
       - hard      → mark failed immediately, move on

  4. **Safety rails.** Abort the whole batch if:
       - 3 consecutive hard fails in the last 5 attempts (IP likely flagged)
       - Camoufox crash rate > 20% (local/tooling issue, not Google)

  5. **Collision detection.** After each successful onboard, read the
     output JSON and verify the number of unique emails grew by 1
     (or stayed the same for resume). Flat count on a reported-success
     suggests the dedupe cascade missed — worth investigating.

  6. **Logs.** Per-account subprocess stderr is teed to
     `batch_logs/<safe_email>/<timestamp>.log` so post-mortem is possible
     without re-running.

CLI example:

    python batch.py \\
      --file email.txt \\
      --output ../../kiro_tokens.json \\
      --cooldown-s 60 \\
      --state batch_state.json \\
      --provider google \\
      --max-retries 2

    # Validate schema, print plan, don't launch browsers:
    python batch.py --file email.txt --dry-run
"""

from __future__ import annotations

import argparse
import enum
import json
import os
import re
import subprocess
import sys
import time
from dataclasses import asdict, dataclass, field
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Dict, List, Optional, Tuple

BASE_DIR = Path(__file__).resolve().parent
DEFAULT_ONBOARD_SCRIPT = BASE_DIR / "onboard.py"
DEFAULT_OUTPUT = BASE_DIR / "kiro_tokens.json"
DEFAULT_STATE_FILE = BASE_DIR / "batch_state.json"
DEFAULT_LOG_ROOT = BASE_DIR / "batch_logs"

# ──────────────────────────────────────────────────────────────────────────────
# Failure taxonomy
#
# Classification is done on the stderr of the onboard subprocess — we
# deliberately avoid parsing the log lines (they're meant for humans). Each
# `Kind` maps to a `Class`; the runner's retry policy keys on `Class`.
#
# Transient: worth retrying after a backoff.
#   - NETWORK      → timeout, connection reset, DNS
#   - BROWSER      → Camoufox crash, Playwright launch error
#   - TIMEOUT_WAIT → kiro:// redirect didn't arrive within --timeout-login-s
#
# Hard: no point retrying; surface and move on.
#   - BLOCKED      → Google explicit hard-block ("account sign-in blocked")
#   - TWO_FACTOR   → account requires 2FA we can't auto-solve
#   - WRONG_PASS   → auth denied
#   - CONSENT      → OAuth consent declined (accidentally hit "Cancel")
#   - UNKNOWN_HARD → exit-coded failure not matching any pattern
# ──────────────────────────────────────────────────────────────────────────────


class Class(enum.Enum):
    SUCCESS = "success"
    TRANSIENT = "transient"
    HARD = "hard"


class Kind(enum.Enum):
    SUCCESS = "success"
    NETWORK = "transient:network"
    BROWSER = "transient:browser"
    TIMEOUT_WAIT = "transient:timeout"
    BLOCKED = "hard:blocked"
    TWO_FACTOR = "hard:2fa"
    WRONG_PASS = "hard:wrong_password"
    CONSENT = "hard:consent_declined"
    UNKNOWN_HARD = "hard:unknown"


_CLASSIFICATION_TABLE = {
    # Transient markers — stderr substrings. Order matters: earlier wins.
    Kind.NETWORK: [
        "ConnectionError",
        "NetworkError",
        "ECONNREFUSED",
        "dns error",
        "network is unreachable",
        "proxy egress validation failed",
    ],
    Kind.BROWSER: [
        "BrowserDriverUnavailableError",
        "Camoufox",
        "playwright",
        "Target.setAutoAttach",
        "browser closed unexpectedly",
        "Browser closed unexpectedly",
    ],
    Kind.TIMEOUT_WAIT: [
        "waiting for the kiro:// redirect",
        "timed out waiting for",
        "TimeoutError",
    ],
    # Hard markers — these are terminal for this run.
    Kind.BLOCKED: [
        "hard-blocked",
        "BLOCKED",
        "sign-in is blocked",
    ],
    Kind.TWO_FACTOR: [
        "VERIFY_ITS_YOU",
        "2-step verification",
        "2FA required",
    ],
    Kind.WRONG_PASS: [
        "Wrong password",
        "couldn't find your Google Account",
        "password field not fillable",
    ],
    Kind.CONSENT: [
        "consent_declined",
        "Cancel",
    ],
}


def classify_failure(exit_code: int, stderr: str) -> Kind:
    """Map (exit_code, stderr snippet) to a Kind."""
    if exit_code == 0:
        return Kind.SUCCESS
    # exit 124 is /usr/bin/timeout's code — treat as transient.
    if exit_code == 124:
        return Kind.TIMEOUT_WAIT
    haystack = (stderr or "").lower()
    # Hard kinds first — they're more specific and we don't want to
    # accidentally retry a hard-block because it also mentions "network".
    hard_order = [Kind.BLOCKED, Kind.TWO_FACTOR, Kind.WRONG_PASS, Kind.CONSENT]
    transient_order = [Kind.BROWSER, Kind.NETWORK, Kind.TIMEOUT_WAIT]
    for kind in hard_order + transient_order:
        for marker in _CLASSIFICATION_TABLE.get(kind, []):
            if marker.lower() in haystack:
                return kind
    # Exit-code fallback: non-zero, no marker, counts as unknown hard fail.
    return Kind.UNKNOWN_HARD


def kind_class(kind: Kind) -> Class:
    if kind == Kind.SUCCESS:
        return Class.SUCCESS
    if kind.value.startswith("transient:"):
        return Class.TRANSIENT
    return Class.HARD


# ──────────────────────────────────────────────────────────────────────────────
# Credentials file parser
#
# Accepts the same format as kikirro's email.txt: `email:password` per
# line, `#` comments, blank lines OK. Email must look like an email
# (contains '@'); password can be any non-empty string (letters, digits,
# symbols — including colons, so the FIRST ':' is the separator).
# ──────────────────────────────────────────────────────────────────────────────


@dataclass(frozen=True)
class Credential:
    email: str
    password: str

    def redacted(self) -> str:
        return f"Credential(email={self.email!r}, password=<{len(self.password)} chars>)"


def parse_credentials_file(path: Path) -> Tuple[List[Credential], List[str]]:
    """Parse an email:password file. Returns (creds, skip_reasons).

    Dedupes by (lowercased) email — keeps the first entry.
    """
    if not path.exists():
        raise FileNotFoundError(path)
    creds: List[Credential] = []
    seen: set = set()
    reasons: List[str] = []
    for i, raw in enumerate(path.read_text(encoding="utf-8").splitlines(), start=1):
        # Strip only CR (Windows) — preserve trailing spaces in passwords
        # which are legitimate. Then decide if the line is blank/comment
        # using a trimmed copy, but partition on the un-trimmed raw.
        raw = raw.rstrip("\r\n")
        trimmed = raw.strip()
        if not trimmed or trimmed.startswith("#"):
            continue
        if ":" not in raw:
            reasons.append(f"line {i}: missing ':' separator")
            continue
        email_part, _, password_part = raw.partition(":")
        email = email_part.strip().lower()
        password = password_part  # preserve leading/trailing whitespace
        if "@" not in email or "." not in email.split("@", 1)[1]:
            reasons.append(f"line {i}: {email_part!r} is not a valid email")
            continue
        if not password.strip():
            reasons.append(f"line {i}: empty password for {email}")
            continue
        if email in seen:
            reasons.append(f"line {i}: duplicate email {email} (keeping first)")
            continue
        seen.add(email)
        creds.append(Credential(email=email, password=password))
    return creds, reasons


# ──────────────────────────────────────────────────────────────────────────────
# State machine
#
# JSON schema:
#   {
#     "accounts": {
#       "<email>": {
#         "status": "pending" | "in_progress" | "done" | "failed" | "skipped",
#         "attempts": <int>,
#         "last_kind": "<Kind.value>" | null,
#         "last_error": "<str>" | null,
#         "last_attempt_at": "<iso8601>" | null,
#         "completed_at": "<iso8601>" | null
#       }, ...
#     },
#     "run_started_at": "<iso8601>",
#     "schema_version": 1
#   }
# ──────────────────────────────────────────────────────────────────────────────


class Status(enum.Enum):
    PENDING = "pending"
    IN_PROGRESS = "in_progress"
    DONE = "done"
    FAILED = "failed"
    SKIPPED = "skipped"


@dataclass
class AccountState:
    status: str = Status.PENDING.value
    attempts: int = 0
    last_kind: Optional[str] = None
    last_error: Optional[str] = None
    last_attempt_at: Optional[str] = None
    completed_at: Optional[str] = None


@dataclass
class BatchState:
    accounts: Dict[str, AccountState] = field(default_factory=dict)
    run_started_at: str = ""
    schema_version: int = 1

    def to_dict(self) -> Dict[str, Any]:
        return {
            "accounts": {k: asdict(v) for k, v in self.accounts.items()},
            "run_started_at": self.run_started_at,
            "schema_version": self.schema_version,
        }

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "BatchState":
        accounts = {}
        for email, rec in (data.get("accounts") or {}).items():
            if not isinstance(rec, dict):
                continue
            accounts[email] = AccountState(
                status=str(rec.get("status") or Status.PENDING.value),
                attempts=int(rec.get("attempts") or 0),
                last_kind=rec.get("last_kind"),
                last_error=rec.get("last_error"),
                last_attempt_at=rec.get("last_attempt_at"),
                completed_at=rec.get("completed_at"),
            )
        return cls(
            accounts=accounts,
            run_started_at=str(data.get("run_started_at") or _now_iso()),
            schema_version=int(data.get("schema_version") or 1),
        )


def _now_iso() -> str:
    return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def load_state(path: Path) -> BatchState:
    if not path.exists():
        return BatchState(run_started_at=_now_iso())
    try:
        data = json.loads(path.read_text(encoding="utf-8"))
    except Exception as e:
        raise SystemExit(f"error: state file {path} is corrupt: {e}")
    if not isinstance(data, dict):
        raise SystemExit(f"error: state file {path} is not a JSON object")
    return BatchState.from_dict(data)


def save_state(path: Path, state: BatchState) -> None:
    """Atomic write of the state file."""
    path.parent.mkdir(parents=True, exist_ok=True)
    tmp = path.with_suffix(path.suffix + ".tmp")
    tmp.write_text(
        json.dumps(state.to_dict(), indent=2, sort_keys=True) + "\n",
        encoding="utf-8",
    )
    try:
        os.chmod(tmp, 0o600)
    except Exception:
        pass
    tmp.replace(path)


# ──────────────────────────────────────────────────────────────────────────────
# Safety thresholds
# ──────────────────────────────────────────────────────────────────────────────


HARD_FAIL_WINDOW = 5
HARD_FAIL_ABORT_THRESHOLD = 3
BROWSER_CRASH_RATE_ABORT = 0.20
BROWSER_CRASH_MIN_SAMPLE = 5


class AbortReason(enum.Enum):
    CONSECUTIVE_HARD_FAILS = "consecutive_hard_fails"
    BROWSER_CRASH_RATE = "browser_crash_rate"


def should_abort(recent_kinds: List[Kind]) -> Optional[AbortReason]:
    """Look at the tail of recent attempts; decide if the batch should abort.

    `recent_kinds` is the ordered list of every attempt's Kind from this
    run (newest last). We don't trim it — we just sample the last N.
    """
    if not recent_kinds:
        return None

    window = recent_kinds[-HARD_FAIL_WINDOW:]
    # Consecutive hard fails: tail of the window must be all-hard.
    hard_count = 0
    for k in reversed(window):
        if kind_class(k) == Class.HARD:
            hard_count += 1
        else:
            break
    if hard_count >= HARD_FAIL_ABORT_THRESHOLD:
        return AbortReason.CONSECUTIVE_HARD_FAILS

    # Browser crash rate over the entire run.
    if len(recent_kinds) >= BROWSER_CRASH_MIN_SAMPLE:
        crashes = sum(1 for k in recent_kinds if k == Kind.BROWSER)
        if crashes / len(recent_kinds) >= BROWSER_CRASH_RATE_ABORT:
            return AbortReason.BROWSER_CRASH_RATE

    return None


# ──────────────────────────────────────────────────────────────────────────────
# Collision detection
# ──────────────────────────────────────────────────────────────────────────────


def count_unique_accounts(output_path: Path) -> int:
    """How many distinct (email | profileArn-last-seg | token-prefix) entries?"""
    if not output_path.exists():
        return 0
    try:
        data = json.loads(output_path.read_text(encoding="utf-8"))
    except Exception:
        return 0
    if not isinstance(data, list):
        return 0
    from onboard import _dedupe_key  # reuse the canonical cascade
    keys = set()
    for e in data:
        if isinstance(e, dict):
            k = _dedupe_key(e)
            if k:
                keys.add(k)
    return len(keys)


# ──────────────────────────────────────────────────────────────────────────────
# Onboard subprocess driver
# ──────────────────────────────────────────────────────────────────────────────


def _safe_slug(s: str) -> str:
    return re.sub(r"[^A-Za-z0-9._-]", "_", s)


def run_onboard_once(
    cred: Credential,
    *,
    output_path: Path,
    provider: str,
    onboard_script: Path = DEFAULT_ONBOARD_SCRIPT,
    python_exe: Optional[str] = None,
    log_root: Path = DEFAULT_LOG_ROOT,
    extra_args: Optional[List[str]] = None,
) -> Tuple[int, str]:
    """Invoke `python onboard.py ...` for one account. Returns (exit_code, stderr).

    The password is passed via stdin so it's never visible in `ps`.
    stderr is teed to a per-account log file under `batch_logs/<email>/`.
    """
    python_exe = python_exe or sys.executable
    log_root.mkdir(parents=True, exist_ok=True)
    per_account_dir = log_root / _safe_slug(cred.email)
    per_account_dir.mkdir(parents=True, exist_ok=True)
    log_path = per_account_dir / f"{int(time.time())}.log"

    cmd = [
        python_exe,
        str(onboard_script),
        "--email", cred.email,
        "--password", "-",
        "--provider", provider,
        "--output", str(output_path),
    ]
    if extra_args:
        cmd.extend(extra_args)

    # Run in the onboarder's directory so relative paths (profiles_data/,
    # screenshots/) resolve where the operator expects.
    proc = subprocess.Popen(
        cmd,
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        cwd=str(onboard_script.parent),
        text=True,
    )
    try:
        # Feed password over stdin (onboard.py reads one line for --password -).
        stdout, stderr = proc.communicate(input=cred.password + "\n")
    except Exception as e:  # noqa: BLE001
        proc.kill()
        return -1, f"BatchDriver: subprocess communicate failed: {e}"

    # Persist the full log for post-mortem.
    try:
        log_path.write_text(
            f"# cmd: {' '.join(cmd)}\n# exit={proc.returncode}\n\n"
            f"# ── stdout ──\n{stdout}\n\n# ── stderr ──\n{stderr}\n",
            encoding="utf-8",
        )
        os.chmod(log_path, 0o600)
    except Exception:
        pass

    return proc.returncode, stderr


# ──────────────────────────────────────────────────────────────────────────────
# Batch orchestrator (core loop)
# ──────────────────────────────────────────────────────────────────────────────


@dataclass
class BatchResult:
    succeeded: int = 0
    failed: int = 0
    skipped: int = 0
    aborted: Optional[AbortReason] = None
    recent_kinds: List[Kind] = field(default_factory=list)


def _should_skip_already_done(rec: AccountState, max_retries: int) -> bool:
    if rec.status == Status.DONE.value:
        return True
    if rec.status == Status.FAILED.value and rec.attempts > max_retries:
        # Exhausted retries AND the last outcome is a hard fail
        # (classifier-determined).
        if rec.last_kind and rec.last_kind.startswith("hard:"):
            return True
    return False


def _human_kind(kind: Kind) -> str:
    return kind.value.replace("transient:", "").replace("hard:", "")


def run_batch(
    creds: List[Credential],
    *,
    output_path: Path,
    state_path: Path,
    provider: str = "google",
    cooldown_s: float = 60.0,
    max_retries: int = 2,
    onboard_script: Path = DEFAULT_ONBOARD_SCRIPT,
    log_root: Path = DEFAULT_LOG_ROOT,
    python_exe: Optional[str] = None,
    extra_args: Optional[List[str]] = None,
    on_progress: Optional[Any] = None,
    sleep_fn: Optional[Any] = None,
    run_one: Optional[Any] = None,
) -> BatchResult:
    """Drive the whole batch. Returns BatchResult.

    `on_progress` is an optional callback (email, index, total, Kind) for
    tests / UI; defaults to stdout prints.

    `sleep_fn` is injected (time.sleep by default) so tests don't wait.

    `run_one` is injected (run_onboard_once by default) so tests don't
    spawn browsers.
    """
    sleep_fn = sleep_fn or time.sleep
    run_one = run_one or run_onboard_once

    state = load_state(state_path)
    if not state.run_started_at:
        state.run_started_at = _now_iso()

    result = BatchResult()

    # Pre-run snapshot for collision detection.
    pre_count = count_unique_accounts(output_path)

    total = len(creds)
    for i, cred in enumerate(creds, start=1):
        rec = state.accounts.setdefault(cred.email, AccountState())

        if _should_skip_already_done(rec, max_retries):
            msg = (
                f"[{i}/{total}] skip {cred.email} "
                f"(status={rec.status}, attempts={rec.attempts}, "
                f"last={rec.last_kind})"
            )
            _progress(on_progress, cred.email, i, total, Kind.SUCCESS, msg)
            result.skipped += 1
            continue

        rec.status = Status.IN_PROGRESS.value
        rec.attempts += 1
        rec.last_attempt_at = _now_iso()
        save_state(state_path, state)

        t0 = time.time()
        exit_code, stderr = run_one(
            cred,
            output_path=output_path,
            provider=provider,
            onboard_script=onboard_script,
            python_exe=python_exe,
            log_root=log_root,
            extra_args=extra_args,
        )
        elapsed = time.time() - t0
        kind = classify_failure(exit_code, stderr)
        rec.last_kind = kind.value
        result.recent_kinds.append(kind)

        if kind == Kind.SUCCESS:
            rec.status = Status.DONE.value
            rec.last_error = None
            rec.completed_at = _now_iso()
            result.succeeded += 1

            # Collision-sanity: did the output file actually grow?
            post_count = count_unique_accounts(output_path)
            if post_count == pre_count and result.succeeded > result.skipped:
                msg_suffix = (
                    f" [WARN: output count unchanged ({post_count}); "
                    f"possible dedupe-key collision]"
                )
            else:
                msg_suffix = ""
            pre_count = post_count

            msg = (
                f"[{i}/{total}] DONE {cred.email} "
                f"({elapsed:.1f}s, attempt={rec.attempts}){msg_suffix}"
            )
            _progress(on_progress, cred.email, i, total, kind, msg)
        else:
            klass = kind_class(kind)
            if klass == Class.TRANSIENT and rec.attempts <= max_retries:
                rec.status = Status.PENDING.value  # will retry on next pass
                msg = (
                    f"[{i}/{total}] TRANSIENT {cred.email} "
                    f"({_human_kind(kind)}, attempt={rec.attempts}/{max_retries+1}) "
                    f"— will retry"
                )
            else:
                rec.status = Status.FAILED.value
                rec.last_error = (stderr or "")[:500]
                result.failed += 1
                msg = (
                    f"[{i}/{total}] FAILED {cred.email} "
                    f"({_human_kind(kind)}, attempt={rec.attempts})"
                )
            _progress(on_progress, cred.email, i, total, kind, msg)

        save_state(state_path, state)

        # Safety check before cooling down.
        abort = should_abort(result.recent_kinds)
        if abort is not None:
            result.aborted = abort
            print(
                f"\n\u2717 ABORTING: {abort.value}. "
                f"See {state_path} for per-account state and "
                f"batch_logs/ for subprocess output.",
                file=sys.stderr,
                flush=True,
            )
            return result

        if i < total:
            sleep_fn(cooldown_s)

    return result


def _progress(cb, email: str, i: int, total: int, kind: Kind, msg: str) -> None:
    if cb:
        cb(email, i, total, kind, msg)
    else:
        print(msg, flush=True)


# ──────────────────────────────────────────────────────────────────────────────
# CLI
# ──────────────────────────────────────────────────────────────────────────────


def _summary(result: BatchResult, total: int, state_path: Path, creds: List[Credential]) -> str:
    lines = [
        "",
        "─" * 60,
        f"Batch complete: {result.succeeded}/{total} succeeded "
        f"(failed={result.failed}, skipped={result.skipped}"
        + (
            f", aborted={result.aborted.value}" if result.aborted else ""
        )
        + ")",
    ]
    state = load_state(state_path)
    failed_emails = [
        (email, rec)
        for email, rec in state.accounts.items()
        if rec.status == Status.FAILED.value
        and email in {c.email for c in creds}
    ]
    if failed_emails:
        lines.append("")
        lines.append("Failed:")
        for email, rec in failed_emails:
            lines.append(
                f"  {email} — {rec.last_kind or 'unknown'} "
                f"(attempts={rec.attempts})"
            )
        lines.append("")
        lines.append(f"Resume: re-run the same command. Transient failures will")
        lines.append(f"be retried up to --max-retries; hard failures are sticky.")
    return "\n".join(lines)


def _parse_args(argv: Optional[List[str]] = None) -> argparse.Namespace:
    p = argparse.ArgumentParser(
        prog="batch",
        description="Drive N single-account Kiro onboards with state + rate limit.",
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )
    p.add_argument("--file", required=True, help="path to email:password file")
    p.add_argument("--output", default=str(DEFAULT_OUTPUT),
                   help=f"output JSON path (default: {DEFAULT_OUTPUT})")
    p.add_argument("--state", default=str(DEFAULT_STATE_FILE),
                   help=f"state file (default: {DEFAULT_STATE_FILE})")
    p.add_argument("--provider", choices=["google", "github"], default="google",
                   help="social IDP (default: google)")
    p.add_argument("--cooldown-s", type=float, default=60.0,
                   help="seconds to sleep between accounts (default: 60)")
    p.add_argument("--max-retries", type=int, default=2,
                   help="transient retry ceiling per account (default: 2)")
    p.add_argument("--onboard-script", default=str(DEFAULT_ONBOARD_SCRIPT),
                   help=f"onboard.py path (default: {DEFAULT_ONBOARD_SCRIPT})")
    p.add_argument("--log-root", default=str(DEFAULT_LOG_ROOT),
                   help=f"batch log directory (default: {DEFAULT_LOG_ROOT})")
    p.add_argument("--dry-run", action="store_true",
                   help="parse + validate + print plan; do not launch browsers")
    p.add_argument("--challenge-mode", choices=["auto", "manual", "skip"],
                   default="auto",
                   help="passed through to onboard.py (default: auto)")
    p.add_argument("--headless", action="store_true",
                   help="passed through to onboard.py")
    p.add_argument("--timeout-login-s", type=int, default=120,
                   help="passed through to onboard.py (default: 120)")
    return p.parse_args(argv)


def _proxy_warning() -> None:
    if os.environ.get("KIROXY_ONBOARD_PROXY"):
        print(
            f"proxy: KIROXY_ONBOARD_PROXY set "
            f"(len={len(os.environ['KIROXY_ONBOARD_PROXY'])})",
            flush=True,
        )
        return
    banner = (
        "\n"
        "══════════════════════════════════════════════════════════════════\n"
        " ⚠  KIROXY_ONBOARD_PROXY is not set.\n"
        "\n"
        "    Running a batch of ≥5 accounts from the same IP will likely\n"
        "    trigger Google's IP-based flagging after a handful of\n"
        "    onboards. Expected success rate on fresh accounts is:\n"
        "\n"
        "      with residential proxy : 65-80%\n"
        "      without                : 25-45% and drops steeply as\n"
        "                                 the batch progresses\n"
        "\n"
        "    Set KIROXY_ONBOARD_PROXY (or run with --proxy on onboard.py)\n"
        "    before batching more than 2-3 accounts. See\n"
        "    tools/onboard/README.md for proxy recommendations.\n"
        "══════════════════════════════════════════════════════════════════\n"
    )
    print(banner, file=sys.stderr, flush=True)


def main(argv: Optional[List[str]] = None) -> int:
    args = _parse_args(argv)

    cred_path = Path(args.file).expanduser().resolve()
    output_path = Path(args.output).expanduser().resolve()
    state_path = Path(args.state).expanduser().resolve()
    onboard_script = Path(args.onboard_script).expanduser().resolve()
    log_root = Path(args.log_root).expanduser().resolve()

    try:
        creds, reasons = parse_credentials_file(cred_path)
    except FileNotFoundError:
        print(f"error: credentials file not found: {cred_path}", file=sys.stderr)
        return 1

    if reasons:
        print(f"parsed {len(creds)} credentials, {len(reasons)} lines skipped:",
              flush=True)
        for r in reasons:
            print(f"  skip: {r}", flush=True)

    if not creds:
        print("error: no valid credentials parsed", file=sys.stderr)
        return 1

    if args.dry_run:
        print(
            f"dry-run: {len(creds)} accounts would be onboarded with "
            f"provider={args.provider}, cooldown={args.cooldown_s}s, "
            f"max-retries={args.max_retries}",
            flush=True,
        )
        for c in creds:
            print(f"  would onboard: {c.email}", flush=True)
        return 0

    _proxy_warning()

    # Pass-through args for onboard.py. We ALWAYS pass challenge-mode
    # because batch mode's default (auto) differs from onboard.py's
    # default in intent (we want machine-friendly behavior when possible).
    extra_args = ["--challenge-mode", args.challenge_mode,
                  "--timeout-login-s", str(args.timeout_login_s)]
    if args.headless:
        extra_args.append("--headless")

    print(
        f"starting batch: {len(creds)} accounts, state={state_path.name}, "
        f"output={output_path.name}",
        flush=True,
    )

    try:
        result = run_batch(
            creds,
            output_path=output_path,
            state_path=state_path,
            provider=args.provider,
            cooldown_s=args.cooldown_s,
            max_retries=args.max_retries,
            onboard_script=onboard_script,
            log_root=log_root,
            extra_args=extra_args,
        )
    except KeyboardInterrupt:
        print("\n⏸ interrupted; state persisted; re-run to resume", flush=True)
        return 130

    print(_summary(result, total=len(creds), state_path=state_path, creds=creds),
          flush=True)
    if result.aborted is not None:
        return 2
    if result.failed > 0:
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
