# BIFROST Makefile
# Provides standard build, test, lint, and release targets.

BINARY  := bifrost
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -s -w -X github.com/nelobster/bifrost/internal/config.Version=$(VERSION)
GOFLAGS := -trimpath

.PHONY: all build test lint clean install run dev release

all: build

## build: Compile the binary (static, no CGO)
build:
	CGO_ENABLED=0 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o bin/$(BINARY) .

## test: Run all tests
test:
	go test -v -race -count=1 ./...

## test-cover: Run tests with coverage
test-cover:
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out
	@echo "Open HTML report: go tool cover -html=coverage.out"

## lint: Run go vet
lint:
	go vet ./...

## clean: Remove build artifacts
clean:
	rm -rf bin/ coverage.out dist/

## install: Install binary to GOPATH/bin
install:
	go install $(GOFLAGS) -ldflags "$(LDFLAGS)" .

## run: Build and run (headless mode for quick testing)
run: build
	./bin/$(BINARY) --headless

## dev: Run with hot-reload (auto-restarts on code changes)
dev:
	$(shell go env GOPATH)/bin/air -- --headless

## release: Build release binaries for Linux amd64/arm64 (static)
release:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-linux-amd64 .
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-linux-arm64 .

## help: Show this help
help:
	@grep -E '^## ' Makefile | sed 's/## /  /'
