.PHONY: build vet fmt test test-race run tidy all gate clean

export GOEXPERIMENT := jsonv2

BIN := kiroxy
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X main.version=$(VERSION)

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

gate: fmt vet build test
	@echo "GATE GREEN"

clean:
	rm -f $(BIN)
