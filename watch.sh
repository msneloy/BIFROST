#!/usr/bin/env bash

# watch.sh – auto‑restart BIFROST on Go source changes
# Requires inotifywait (package: inotify-tools)

set -euo pipefail

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

cleanup() {
  if [[ -n "${BIFROST_PID-}" ]]; then
    echo "[watch] Stopping BIFROST (PID $BIFROST_PID)"
    kill "$BIFROST_PID" 2>/dev/null || true
    wait "$BIFROST_PID" 2>/dev/null || true
  fi
}
trap cleanup EXIT INT TERM

stop_existing_bifrost() {
  if command -v pkill >/dev/null 2>&1; then
    echo "[watch] Stopping any existing BIFROST process..."
    pkill -x bifrost 2>/dev/null || true
    pkill -f './bifrost' 2>/dev/null || true
    sleep 1
  fi
}

watch_paths=(cmd internal web go.mod go.sum)
watch_include='.*\.(go|html|tmpl|mod|sum)$'
watch_exclude='(^|/)\..*'

while true; do
  stop_existing_bifrost

  echo -e "\n[watch] Building BIFROST..."
  if ! go build -o bifrost ./cmd/bifrost; then
    echo "[watch] Build failed — waiting for changes..."
    inotifywait -qq -e modify,create,delete,move -r --exclude "$watch_exclude" --include "$watch_include" "${watch_paths[@]}" >/dev/null
    continue
  fi

  echo "[watch] Starting BIFROST..."
  ./bifrost &
  BIFROST_PID=$!

  echo "[watch] Watching for source changes..."
  inotifywait -qq -e modify,create,delete,move -r --exclude "$watch_exclude" --include "$watch_include" "${watch_paths[@]}" >/dev/null

  echo "[watch] Change detected — stopping BIFROST (PID $BIFROST_PID)"
  kill "$BIFROST_PID" 2>/dev/null || true
  wait "$BIFROST_PID" 2>/dev/null || true
  sleep 1
 done
