.PHONY: build vet fmt test test-race run tidy all gate clean \
        docker-build docker-run docker-compose-up docker-compose-down docker-clean \
        vuln release-dry-run

export GOEXPERIMENT := jsonv2

BIN := kiroxy
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X main.version=$(VERSION)

# Docker image coordinates. Override with `make docker-build IMAGE=foo:bar`.
IMAGE  ?= kiroxy:$(VERSION)
LATEST ?= kiroxy:local

build:
	go build -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/kiroxy

vet:
	go vet ./...

fmt:
	@unfmt=$$(gofmt -l .); \
	if [ -n "$$unfmt" ]; then \
	  echo "gofmt: unformatted files:"; echo "$$unfmt"; exit 1; \
	fi

test:
	go test ./...

test-race:
	go test -race -timeout 120s ./...

run: build
	./$(BIN) serve

tidy:
	go mod tidy

# `gate` is the canonical pre-commit check. Set KIROXY_CI_STRICT=1 to also
# require govulncheck (opt-in; CI sets this). Default behaviour unchanged.
gate: fmt vet build test $(if $(filter 1,$(KIROXY_CI_STRICT)),vuln)
	@echo "GATE GREEN"

clean:
	rm -f $(BIN)

# ---------------------------------------------------------------------------
# Docker targets
# ---------------------------------------------------------------------------
# All targets degrade gracefully when docker is absent — a missing `docker`
# binary triggers a clear error rather than a Make syntax failure.

docker-build:
	@command -v docker >/dev/null 2>&1 || { echo "docker not found in PATH" >&2; exit 1; }
	docker build \
	  --build-arg VERSION=$(VERSION) \
	  -t $(IMAGE) \
	  -t $(LATEST) \
	  .

docker-run: docker-build
	@command -v docker >/dev/null 2>&1 || { echo "docker not found in PATH" >&2; exit 1; }
	docker run --rm \
	  -p 127.0.0.1:8787:8787 \
	  -v kiroxy-data:/data \
	  --name kiroxy \
	  --read-only \
	  --cap-drop=ALL \
	  --security-opt=no-new-privileges:true \
	  --tmpfs /tmp:size=16m,mode=1777 \
	  -e KIROXY_API_KEY=$$KIROXY_API_KEY \
	  -e KIROXY_LOG_LEVEL=$${KIROXY_LOG_LEVEL:-info} \
	  $(LATEST)

docker-compose-up:
	@command -v docker >/dev/null 2>&1 || { echo "docker not found in PATH" >&2; exit 1; }
	docker compose up -d --build
	docker compose ps

docker-compose-down:
	@command -v docker >/dev/null 2>&1 || { echo "docker not found in PATH" >&2; exit 1; }
	docker compose down

docker-clean:
	@command -v docker >/dev/null 2>&1 || { echo "docker not found in PATH" >&2; exit 1; }
	-docker rm -f kiroxy 2>/dev/null
	-docker rmi $(IMAGE) $(LATEST) 2>/dev/null
	@echo "Note: named volume 'kiroxy-data' is preserved. Run 'docker volume rm kiroxy-data' to wipe the vault."

# ---------------------------------------------------------------------------
# Vulnerability scanning (govulncheck)
# ---------------------------------------------------------------------------
# Opt-in dependency of `gate` when KIROXY_CI_STRICT=1 (set by the CI
# workflow's strict lane). Locally, run `make vuln` on demand. Degrades
# gracefully if govulncheck is not on PATH: prints an install hint and
# exits 0 so local developers are not blocked.

vuln:
	@if command -v govulncheck >/dev/null 2>&1; then \
	  echo "govulncheck ./..."; \
	  govulncheck ./...; \
	else \
	  echo "govulncheck not found on PATH; skipping."; \
	  echo "  install with:  go install golang.org/x/vuln/cmd/govulncheck@latest"; \
	  echo "  then ensure \$$(go env GOBIN) or \$$(go env GOPATH)/bin is on PATH."; \
	fi
