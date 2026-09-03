# BIFROST Makefile
# Provides standard build, test, lint, release, and automatic LOC targets.

BINARY  := bifrost
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -s -w -X github.com/nelobster/bifrost/internal/config.Version=$(VERSION)
GOFLAGS := -trimpath

.PHONY: all build test lint clean install run tui dev release loc help

all: build

## loc: Recalculate lines of code and update README.md automatically
loc:
	@GO_LOC=$$(git ls-files '*.go' | xargs wc -l | grep total | awk '{print $$1}'); \
	TOTAL_LOC=$$(git ls-files '*.go' Makefile README.md .air.toml | xargs wc -l | grep total | awk '{print $$1}'); \
	echo "Updating README.md metrics (Go LOC: $$GO_LOC, Total LOC: $$TOTAL_LOC)..."; \
	sed -i "s/Go_LOC-[0-9]*/Go_LOC-$$GO_LOC/g" README.md; \
	sed -i "s/Total_LOC-[0-9]*/Total_LOC-$$TOTAL_LOC/g" README.md; \
	sed -i "s/\*\*Go Codebase\*\*: [0-9]* lines/\*\*Go Codebase\*\*: $$GO_LOC lines/g" README.md; \
	sed -i "s/\*\*Total Repository\*\*: [0-9]* lines/\*\*Total Repository\*\*: $$TOTAL_LOC lines/g" README.md

## build: Compile the binary (static, no CGO) and update LOC
build: loc
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

## run: Build and run in headless mode
run: build
	./bin/$(BINARY) --headless

## tui: Build and run interactively with full TUI
tui: build
	./bin/$(BINARY)

## dev: Run with hot-reload in headless mode (auto-restarts on code changes)
dev:
	$(shell go env GOPATH)/bin/air

## release: Build release binaries for Linux amd64/arm64 (static)
release: loc
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-linux-amd64 .
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-linux-arm64 .

## help: Show this help
help:
	@grep -E '^## ' Makefile | sed 's/## /  /'
