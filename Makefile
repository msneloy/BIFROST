SHELL := /bin/bash

.PHONY: dev run build

dev:
	@echo "Starting dev watcher (restarts on .go changes)..."
	@scripts/dev.sh

run:
	@go run .

build:
	@echo "Building binary ./bin/bifrost"
	@mkdir -p bin
	@go build -o bin/bifrost .
