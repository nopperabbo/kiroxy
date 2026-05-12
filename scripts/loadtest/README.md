# kiroxy Load Test Harness

A self-contained, stdlib-only Go harness for measuring kiroxy's request-path
performance and spotting regressions across releases. Includes a protocol-
correct mock Kiro CodeWhisperer upstream so you can run reproducible
benchmarks without burning real credits.

## Contents

```
scripts/loadtest/
├── main.go              Harness (concurrent HTTP client with metrics)
├── scenarios.yaml       Editable scenario definitions
├── run.sh               Orchestrates a full pass: builds, starts mock, runs scenarios
├── analyze.sh           Renders a markdown report from a run directory
├── go.mod               Own module (no deps; stays off kiroxy's go.mod)
├── README.md            This file
└── mock_kiro/
    ├── server.go        Canned Kiro upstream (eventstream frames, CRCs valid)
    ├── go.mod           Own module (no deps)
    └── README.md        Mock-specific notes
```

## Requirements

- **Go 1.22+** on PATH. The harness and mock are their own Go modules,
  deliberately separate from kiroxy's `go.mod`, and do NOT require the
  `GOEXPERIMENT=jsonv2` flag kiroxy itself needs.
- **bash 3.2+** (macOS stock bash works; Linux default bash works).
- **python3 3.8+** for `analyze.sh` report rendering (ubiquitous; drop if
  you only need `summary.json` consumption by other tools).
- **curl** (orchestration uses it for the health probe).

Nothing else. No container runtime, no `jq`, no `yq`, no Python libraries.

## Quick start

From this directory:

```bash
# Build & run every scenario against the mock (no kiroxy needed).
./run.sh --mode kiro

# Results land under ./results/<timestamp>/
ls results/*/report.md | tail -1 | xargs cat
```

A full run (6 scenarios including a 60-second endurance test, excluding the
5-minute duration scenario) takes ~3 minutes on an M1. The 5-minute
scenario (`sustained_5m`) is off the default list; add `--scenarios` to run it.

## Two run modes

The harness supports two target modes; `run.sh` picks between them via
`--mode`:

### `--mode kiro` (default)

Harness targets the mock_kiro server directly. No kiroxy involved. This
mode:

- Runs anywhere (laptop, CI, developer box)
- Needs zero credentials
- Measures the **eventstream pipeline** as an upper bound
- Is what `docs/BENCHMARKS.md` measures

Use this for:
- CI regression detection
- Harness self-tests
- Reference numbers for "what's the best we could do"

### `--mode kiroxy`

Harness targets a running kiroxy instance at `$KIROXY_URL` (default
`http://127.0.0.1:8787`). Requires:

- A running `kiroxy serve` in another terminal
- At least one valid account in the vault
- Real Kiro quota to burn (each request consumes one upstream call)

Use this for:
- Measuring actual end-to-end latency
- Catching kiroxy-specific regressions (middleware, translation)
- Pre-release verification against a staging account

> **Note:** `run.sh` will NOT start kiroxy for you in `--mode kiroxy`. That
> would require credential provisioning the harness doesn't own. Start it
> yourself, point the harness at it.

## Scenarios

Defined in `scenarios.yaml`. The stock set:

| name | concurrency | requests | description |
|---|---:|---:|---|
| `light` | 1 | 10 | Sanity — sequential, non-streaming |
| `sustained` | 5 | 100 | Warm path |
| `burst` | 20 | 100 | Contention |
| `streaming` | 5 | 20 | SSE stability |
| `sustained_5m` | 5 | 5 min duration | Endurance |
| `mock_light` | 1 | 10 | Upper-bound, mock target |
| `mock_burst` | 20 | 100 | Upper-bound, mock target |

Add your own by editing `scenarios.yaml`. The parser understands this
minimal shape:

```yaml
scenarios:
  my_scenario:
    concurrency: 10
    total: 500            # OR duration: "30s"
    stream: false
    mode: anthropic       # OR kiro
    description: "..."
```

## Invoking the harness directly

`run.sh` is a convenience wrapper; for custom invocations call `main` itself:

```bash
go build -o lh .
./lh --url http://127.0.0.1:8787 \
     --mode anthropic \
     --concurrency 10 \
     --total 500 \
     --stream \
     --api-key "$KIROXY_API_KEY" \
     --out ./results/custom \
     --scenario my_custom_run
```

All flags:

| flag | default | purpose |
|---|---|---|
| `--url` | `http://127.0.0.1:8787` | target base URL |
| `--mode` | `anthropic` | `anthropic` (kiroxy `/v1/messages`) or `kiro` (mock `/`) |
| `--concurrency` | 5 | parallel workers |
| `--total` | 100 | total requests (ignored if `--duration > 0`) |
| `--duration` | 0 | run for this long instead of `--total`, e.g. `30s`, `5m` |
| `--stream` | false | request SSE (anthropic mode only) |
| `--api-key` | `$KIROXY_API_KEY` | `X-Api-Key` header |
| `--timeout` | 120s | per-request timeout |
| `--out` | `./results` | output directory |
| `--scenario` | `adhoc` | label stamped into summary.json |
| `--warmup` | 3 | warmup requests issued before timing |
| `--tokens` | 256 | `max_tokens` in anthropic request body |

## Mock_kiro

See `mock_kiro/README.md` for the server's own docs. Quick reference:

```bash
# Build + run
(cd mock_kiro && go build -o /tmp/mock_kiro .)
/tmp/mock_kiro --addr :9789 --stream-events 8 --latency-ms 0

# Flags
--addr            listen address (default :9789)
--latency-ms      inject fixed latency per request (default 0)
--error-rate      0.0..1.0 probability of 429 (default 0)
--stream-events   assistantResponseEvent frames per response (default 8)
--chunk-delay-ms  delay between streamed frames (default 0)
--fail-after      return 5xx after N successes (0=disabled); useful for
                  breaker tests
--tokens-in       reported input tokens in metadataEvent (default 42)
--tokens-out      reported output tokens in metadataEvent (default 64)
--text            canned response text (split across stream events)
--log             log each request line
```

Endpoints:

- `POST /` — CodeWhisperer-shaped entry point (what kiroxy talks to)
- `POST /generate` — identical alias, handy for ad-hoc testing
- `GET /healthz` — orchestration health probe
- `GET /stats` — JSON: `{requests, errors, inflight}` since start

## Output format

Each scenario run produces:

### `summary.json`

One JSON object with aggregate metrics. All fields:

```jsonc
{
  "scenario": "burst",
  "mode": "anthropic",
  "url": "http://127.0.0.1:8787",
  "stream": false,
  "concurrency": 20,
  "started_at": "2026-05-12T14:51:01Z",
  "ended_at":   "2026-05-12T14:51:02Z",
  "total_requests": 100,
  "successes": 100,
  "errors": 0,
  "error_rate": 0.0,
  "total_duration_s": 0.85,
  "rps": 117.6,
  "latency_p50_ms":  25.3,
  "latency_p95_ms":  82.1,
  "latency_p99_ms":  95.0,
  "latency_max_ms":  112.4,
  "latency_mean_ms": 30.7,
  "ttfb_p50_ms": 24.2,
  "ttfb_p95_ms": 80.5,
  "bytes_total": 1574798,
  "events_total": 1000,
  "sys_go_version": "go1.26.2",
  "sys_os": "darwin",
  "sys_arch": "arm64",
  "sys_cpu": 8
}
```

### `requests.jsonl`

One JSON object per request, index-ordered. All fields:

```jsonc
{"index":0, "start_unix_ms":..., "status":200, "latency_ms":28.4,
 "ttfb_ms":26.1, "bytes":1574, "events":10, "worker":3}
```

For custom analysis, pipe through `jq`:

```bash
# Per-worker latency mean
jq -s 'group_by(.worker) |
       map({worker: .[0].worker, mean: (map(.latency_ms) | add/length)})' \
       results/<stamp>/burst/requests.jsonl
```

## Analyze a past run

```bash
./analyze.sh ./results/20260512_145101 > report.md
```

Produces the same markdown table `run.sh` embeds at the end of every run.

## Interpreting results

See `docs/BENCHMARKS.md` for:

- Reference baselines on known hardware
- What "healthy" looks like per scenario
- What to investigate when numbers regress

## Limitations / gaps

### kiroxy doesn't currently redirect its upstream

The mock_kiro server is protocol-correct — kiroxy's `internal/kiroproto`
parser validates its CRC-32 checksums and decodes its frames — but there's
no env var to point kiroxy's upstream at `http://127.0.0.1:9789` instead of
`https://q.us-east-1.amazonaws.com`. The `WithBaseURL()` option in
`internal/kiroclient` is test-only.

This means `--mode anthropic` in the harness still talks to real AWS even
with the mock running. Until kiroxy exposes a `KIROXY_UPSTREAM_URL` env
var (see `BACKLOG.md`), the mock is useful as:

- Reference upper-bound throughput (`--mode kiro`)
- Harness self-test
- Manual integration if you run a kiroxy build with `WithBaseURL()` wired up

### No histogram export

`summary.json` exposes p50/p95/p99/max only. If you need full latency
histograms (e.g. to feed Prometheus or generate tdigest), parse
`requests.jsonl` with the tool of your choice.

### Single-machine only

Both harness and mock run in one process pair on one box. No distributed
load generation. For real multi-node load testing, use k6 or vegeta
pointing at the same endpoints — the harness's JSONL format is
deliberately simple so k6/vegeta output can be compared against it.

## Extending

### New scenario

Edit `scenarios.yaml`. No code change needed.

### New metric

Edit `main.go`:

1. Add the field to `result` struct (for per-request metrics) or `summary`
   (for aggregates)
2. Populate it in `worker()` or `aggregate()` respectively
3. Add it to the `writeJSON` / `writeJSONL` output (automatic — fields are
   emitted by `encoding/json` via struct tags)
4. Add it to `analyze.sh`'s markdown table if you want it in the report

### New target

The harness currently supports two target shapes (`anthropic`, `kiro`).
Adding a third (e.g. `openai` for an OpenAI-compatible shim) is one new
`issue*` function + one new case in `worker()`. Keep the transport logic
stdlib-only to match the existing style.

## Troubleshooting

### `main module (local/kiroxy/scripts/loadtest) does not contain package local/kiroxy/scripts/loadtest/mock_kiro`

You ran `go build ./mock_kiro` from `scripts/loadtest/`. The mock has its
own go.mod; build it from inside its own directory:

```bash
(cd mock_kiro && go build .)
```

Or just use `run.sh`, which handles this.

### Mock returns 429s for everything when error-rate is 0

Check `mock.log` (written under the run output dir). If the mock crashed,
`run.sh` will say `mock_kiro did not come up`. Look for port conflicts
(`--mock-port` to pick a different one).

### `run.sh` hangs during sustained_5m

It's supposed to. The scenario runs for 5 minutes. Ctrl-C is safe; the
mock will be killed by the cleanup trap, partial results are kept.

### High variance in the numbers

Close your browser, disable Spotlight indexing for this directory, and
turn off any IDE that's running language-server indexing. On a laptop with
a loaded workload, expect ±20% variance. On a quiet CI box, ±5%.

### p50 is higher than expected

Check `--warmup`. Default is 3 requests; on a cold process, the first few
measurements absorb connection setup and TLS (for real kiroxy). Bump to
10 if you want tighter numbers for short scenarios.
