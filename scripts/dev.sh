#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT_DIR"

run_cmd() {
  clear
  echo "[dev] Running: go run ."
  go run . &
  child=$!
}

kill_child() {
  if [[ -n "${child-}" && $(ps -p $child -o pid=) ]]; then
    echo "[dev] Stopping process $child"
    kill $child 2>/dev/null || true
    wait $child 2>/dev/null || true
  fi
}

trap 'kill_child; exit 0' INT TERM EXIT

if command -v reflex >/dev/null 2>&1; then
  echo "[dev] Using reflex to watch and run"
  exec reflex -r '\.go$$' -s -- sh -c 'clear; go run .'
fi

if command -v watchexec >/dev/null 2>&1; then
  echo "[dev] Using watchexec to watch and run"
  exec watchexec -r -e go -- "bash -lc 'clear; go run .'"
fi

if command -v inotifywait >/dev/null 2>&1; then
  echo "[dev] Using inotifywait loop to watch and run"
  run_cmd
  while inotifywait -e modify -r .; do
    kill_child
    run_cmd
  done
fi

echo "[dev] No file-watcher found. Falling back to single run."
echo "Install 'reflex' or 'watchexec' or 'inotify-tools' for auto-reload."
go run .
