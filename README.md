# kiroxy

A single-user, self-hosted proxy that exposes your Kiro IDE (Amazon Q Developer / AWS CodeWhisperer) subscription as an Anthropic Messages API-compatible endpoint.

Built on `Quorinex/Kiro-Go` (MIT), with converter code from `d-kuro/kirocc` (Apache-2.0) and a token vault ported from `kadangkesel/hexos` (MIT). See `NOTICE` for full attribution.

> **Status:** building v0.1.0-mvp. See `../BUILD_PLAN.md`.

## What it does

- Accepts `POST /v1/messages` in Anthropic format (SSE streaming supported).
- Translates to Kiro's `generateAssistantResponse` with AWS EventStream framing.
- Rotates across multiple Kiro accounts with cooldown + health tracking.
- Manages OAuth refresh tokens safely under concurrent load.


## Build requirement

kiroxy inherits the kirocc converter packages, which use Go 1.26's experimental
`encoding/json/v2` package. Use the provided Makefile (which pins
`GOEXPERIMENT=jsonv2`) or export it manually:

```bash
export GOEXPERIMENT=jsonv2
go build -o kiroxy ./cmd/kiroxy
```

Or simply:

```bash
make build
make gate   # build + vet + fmt + test
```

## Quick start (after build completes)

```bash
go build -o kiroxy ./cmd/kiroxy
export KIROXY_API_KEY="$(openssl rand -hex 32)"
./kiroxy add-account               # M9: browse-based OAuth device flow
./kiroxy serve                     # default bind 127.0.0.1:8787

curl -sN http://127.0.0.1:8787/v1/messages \
  -H "X-Api-Key: $KIROXY_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"claude-sonnet-4-5","max_tokens":1024,"stream":true,
       "messages":[{"role":"user","content":"Hello"}]}'
```

## Environment variables

See `.env.example`. Defaults bind to loopback only.

## License

MIT. See `LICENSE`. Attribution for derived code: `NOTICE`.

## Legal notice for users

This is a personal-use tool. Operating multi-account pools against commercial APIs may violate the upstream provider's Terms of Service. You accept that risk.
