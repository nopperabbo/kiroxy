# syntax=docker/dockerfile:1.7
#
# kiroxy — self-hosted Kiro→Anthropic proxy
#
# Multi-stage build:
#   1. `builder` — golang:1.26-alpine, compiles the static binary with
#      GOEXPERIMENT=jsonv2 (required by kirocc's encoding/json/v2 usage).
#   2. `runtime` — gcr.io/distroless/static-debian12:nonroot. No shell, no
#      package manager, no CVE-prone libc. The only user available is
#      `nonroot` (UID 65532).
#
# Final image is ~30 MiB and runs as UID 65532 on a read-only root FS.
# The SQLite token vault lives in the single writable volume at /data.

ARG GO_VERSION=1.26
ARG DISTROLESS_TAG=nonroot

# -----------------------------------------------------------------------------
# Stage 1 — build
# -----------------------------------------------------------------------------
FROM golang:${GO_VERSION}-alpine AS builder

# git is needed for `go mod download` against any VCS deps (kiroxy has none
# today, but keeping it future-proof costs ~2 MiB in a throwaway stage).
# ca-certificates so downloads over TLS succeed; copied to the runtime stage
# via distroless defaults.
# hadolint ignore=DL3018
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /src

# Enable Go 1.26 experimental encoding/json/v2 for every build step.
ENV GOEXPERIMENT=jsonv2 \
    CGO_ENABLED=0 \
    GOOS=linux

# Dependency layer first so subsequent source edits don't bust the module cache.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download

# Copy the source tree. .dockerignore keeps the context small.
COPY . .

# VERSION is injected by `make docker-build` / CI via --build-arg. Falls back
# to 'docker' if unset so a hand-run `docker build .` still produces a useful
# binary.
ARG VERSION=docker

# Static, stripped binary. -trimpath normalises build paths for reproducibility.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build \
        -trimpath \
        -ldflags "-s -w -X main.version=${VERSION}" \
        -o /out/kiroxy \
        ./cmd/kiroxy

# Create the /data directory with correct ownership so the runtime stage
# (which has no shell and no `mkdir`) can just COPY it over.
RUN mkdir -p /out/data && chown -R 65532:65532 /out/data

# -----------------------------------------------------------------------------
# Stage 2 — runtime (distroless, nonroot)
# -----------------------------------------------------------------------------
# hadolint ignore=DL3006
FROM gcr.io/distroless/static-debian12:${DISTROLESS_TAG} AS runtime

# OCI labels — help registries and tools surface image metadata.
LABEL org.opencontainers.image.title="kiroxy" \
      org.opencontainers.image.description="Self-hosted Kiro→Anthropic proxy" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.source="https://github.com/Quorinex/kiroxy" \
      org.opencontainers.image.vendor="kiroxy"

# Copy the binary and pre-chowned data dir from the builder stage.
COPY --from=builder /out/kiroxy /usr/local/bin/kiroxy
COPY --from=builder --chown=nonroot:nonroot /out/data /data

# Container-local defaults:
#   - KIROXY_BIND=0.0.0.0 because the container's network namespace is the
#     boundary — binding to loopback would make the port unreachable from
#     the host. Operators still control host-side exposure via `docker run -p`
#     or compose's `ports:` mapping.
#   - KIROXY_DB_PATH=/data/tokens.db so the vault persists in the volume.
ENV KIROXY_BIND=0.0.0.0 \
    KIROXY_PORT=8787 \
    KIROXY_DB_PATH=/data/tokens.db \
    KIROXY_LOG_LEVEL=info

EXPOSE 8787

# Distroless has no shell, so the HEALTHCHECK re-executes the same kiroxy
# binary with the `healthcheck` subcommand (see cmd/kiroxy/healthcheck.go).
# --start-period covers first-run vault migration; --interval is 30s to avoid
# log spam on aggressive orchestrators.
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["/usr/local/bin/kiroxy", "healthcheck"]

USER nonroot:nonroot
VOLUME ["/data"]

ENTRYPOINT ["/usr/local/bin/kiroxy"]
CMD ["serve"]
