#!/usr/bin/env bash

# watch.sh – auto‑restart BIFROST on Go source changes
# Requires inotifywait (package: inotify-tools)

set -e

# Ensure inotifywait is available
if ! command -v inotifywait > /dev/null 2>&1; then
  echo "Error: inotifywait not found. Install with 'sudo apt-get install inotify-tools'" > /dev/stderr
  exit 1
fi

# Ensure Go is available
if ! command -v go > /dev/null 2>&1; then
  echo "Error: Go toolchain not found. Install with 'sudo apt-get install golang-go'" > /dev/stderr
  exit 1
fi

while true; do
  echo "\n[watch] Building BIFROST..."
  if ! go build -o bifrost ./cmd/bifrost; then
    echo "[watch] Build failed – waiting for changes..."
    # Wait for any .go file change before retrying (watch whole repo)
    inotifywait -e modify,create,delete -r . > /dev/null
    continue
  fi

  echo "[watch] Starting BIFROST..."
  ./bifrost &
  BIFROST_PID=$!

  # Watch for any Go source modifications
  inotifywait -e modify,create,delete -r *.go internal/**/*.go web/**/*.go >/dev/null

  echo "[watch] Change detected – stopping BIFROST (PID $BIFROST_PID)"
  kill $BIFROST_PID 2>/dev/null || true
  wait $BIFROST_PID 2>/dev/null || true
  # Loop will rebuild and restart
  sleep 1
done
