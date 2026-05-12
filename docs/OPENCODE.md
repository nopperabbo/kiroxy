# Using kiroxy with opencode

This guide points the [opencode](https://opencode.ai) TUI agent at a
running kiroxy instance so you can use your Kiro IDE subscription from
the terminal.

## Prerequisites

- A kiroxy binary (built locally or pulled via Docker).
- At least one Kiro account imported into the vault. Triplet import:
  ```
  kiroxy import-accounts-json --file kiro_tokens.json
  ```
  or per-account:
  ```
  kiroxy add-account
  ```
- `opencode` installed. See `https://opencode.ai/docs/installation/`.

## Setup

1. Start kiroxy. Pick an inbound API key and export it so both kiroxy
   and opencode see it:

   ```
   export KIROXY_INBOUND_KEY="$(openssl rand -hex 16)"
   kiroxy serve -addr :8787
   ```

2. Generate the opencode provider snippet:

   ```
   kiroxy opencode-config -api-key "$KIROXY_INBOUND_KEY" > snippet.json
   ```

   `snippet.json` is a self-contained JSON document whose top-level
   key is `provider` (singular, per opencode schema).

3. Merge the snippet into `~/.config/opencode/opencode.json`. If the
   file does not exist, copy `snippet.json` in as-is. If it does exist,
   merge the `provider.kiroxy` entry alongside your existing providers.
   **Do not overwrite the whole file** unless kiroxy is your only
   provider.

   Example merged config:

   ```json
   {
     "$schema": "https://opencode.ai/config.json",
     "provider": {
       "anthropic": { "...": "your existing entry" },
       "kiroxy": {
         "npm": "@ai-sdk/anthropic",
         "name": "kiroxy (self-hosted Kiro proxy)",
         "options": {
           "baseURL": "http://localhost:8787",
           "apiKey": "{env:KIROXY_INBOUND_KEY}"
         },
         "models": {
           "claude-sonnet-4.5": { "name": "Claude Sonnet 4.5 (via kiroxy)" }
         }
       }
     }
   }
   ```

   Tip: you can swap the hardcoded `apiKey` in the snippet for
   `{env:KIROXY_INBOUND_KEY}` so the same config works across shells
   without committing your key.

4. Restart opencode (or reload its config).

5. Pick a model in the opencode TUI. Model refs use the
   `<provider-id>/<model-id>` form, so kiroxy's Sonnet 4.5 shows up as
   `kiroxy/claude-sonnet-4.5` in the picker.

## Verifying

- `kiroxy status` should show at least one account present and
  enabled.
- `curl -H "X-Api-Key: $KIROXY_INBOUND_KEY" http://localhost:8787/readyz`
  should return HTTP 200 with `"status":"ready"`.
- Start a chat in opencode. kiroxy's JSON logs should show an incoming
  `/v1/messages` POST with `model` set to one of the IDs in the table
  below.

## Troubleshooting

- **401 Unauthorized from kiroxy** — your opencode `apiKey` does not
  match `KIROXY_INBOUND_KEY`. Regenerate the snippet or update
  opencode.json.
- **Connection refused** — kiroxy is not running, is bound to a
  different port, or `baseURL` in opencode.json points somewhere
  else. Check `kiroxy serve`'s log line for the actual bind address.
- **Model not recognised by opencode** — the `models` key in
  opencode.json is a **map**, not an array. `{ "claude-...": {...} }`
  works. `[ "claude-..." ]` silently yields zero models.
- **opencode returns generic output for a specific model** — kiroxy's
  upstream resolver may have silently fallen back to the default
  (`claude-sonnet-4.6`). This happens for model IDs not in the
  [mapping table](#model-mapping). Use IDs exactly as shown in the
  left column.
- **Upstream 4xx from kiroxy** — account or token issue. Run
  `kiroxy status` and `kiroxy list-accounts` to see cooldown or
  disabled state. If an account is in cooldown, let it recover or
  import another.

## Model Mapping

kiroxy exposes Anthropic-API-form IDs. The Kiro desktop UI shows
different display labels (`kiro/<short-name>`) for the same underlying
models; those UI labels are **not** valid API IDs and will
silent-fallback to the default. Use only the left column in
opencode.json.

The opencode-config subcommand emits exactly the IDs below.

| API ID (use this in opencode) | Kiro UI label | Upstream Kiro model | Context | Notes |
|---|---|---|---|---|
| `claude-opus-4-7` | kiro/opus-4.7 | claude-opus-4.7 | 1M | Always 1M. Thinking not implicit. |
| `claude-opus-4-6` | kiro/opus-4.6 | claude-opus-4.6 | 1M | Always 1M. |
| `claude-opus-4.5` | kiro/opus-4.5 | claude-opus-4.5 | 200K | No 1M variant. |
| `claude-sonnet-4-6` | kiro/sonnet-4.6 | claude-sonnet-4.6 | 200K | Default fallback model. |
| `claude-sonnet-4-6[1m]` | kiro/sonnet-4.6 (1M) | claude-sonnet-4.6-1m | 1M (thinking) | `[1m]` suffix opts into 1M + thinking. |
| `claude-sonnet-4.5` | kiro/sonnet-4.5 | claude-sonnet-4.5 | 200K | |
| `claude-haiku-4.5` | kiro/haiku-4.5 | claude-haiku-4.5 | 200K | No 1M variant. |

### Models NOT emitted (would silent-fallback)

The resolver in `internal/models/models.go` has no entry for these
Kiro UI labels today, so putting them in opencode.json would silently
route every request to `claude-sonnet-4.6`:

- `kiro/auto`
- `kiro/sonnet-4`
- `kiro/deepseek-3.2`
- `kiro/glm-5`
- `kiro/minimax-m2.1`
- `kiro/minimax-m2.5`
- `kiro/qwen3-coder-next`

If you want to use any of these, extend `modelMapOrdered` in
`internal/models/models.go` with an explicit `Anthropic` + `Kiro`
entry and add the new ID to `knownModels` in
`cmd/kiroxy/opencode_config.go`. Ship a test in
`internal/models/models_test.go` that covers the new mapping.

## Multi-Account Pool

kiroxy picks an account per request with an LRU + cooldown policy.
To run multiple Kiro accounts:

```
kiroxy import-accounts-json --file kiro_tokens.json   # bulk triplet import
kiroxy add-account                                    # or one-by-one via OAuth
```

The pool spreads requests across all enabled accounts and pulls any
account that returns persistent upstream errors into cooldown. Check
`kiroxy status` for the current pool view.

## Flags Reference

```
kiroxy opencode-config [flags]

  -base-url string
        kiroxy base URL opencode will call (default "http://localhost:8787")
  -api-key string
        inbound API key (default: $KIROXY_INBOUND_KEY, else "changeme")
  -provider-name string
        provider id slug used in opencode.json (default "kiroxy")
  -models string
        optional comma-separated subset of API IDs; empty = emit all
  -output string
        write JSON to this file instead of stdout; empty = stdout
```

Notes:

- stdout is a clean JSON stream — safe to pipe through `jq`, `python
  -m json.tool`, or `sed`. All human-readable guidance goes to stderr.
- `-output` writes with mode `0600` since the file contains your
  inbound API key in plain text.
- If `-models` includes an entry that is not in the resolver-verified
  set, the entry is omitted and a warning is printed on stderr.
