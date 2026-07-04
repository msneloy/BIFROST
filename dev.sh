#!/usr/bin/env bash
# dev.sh — auto-rebuild and restart BIFROST on source changes
set -uo pipefail

DIR="$(cd "$(dirname "$0")" && pwd)"
BINARY="$DIR/bifrost"
LOG="/tmp/bifrost.log"
PIDFILE="/tmp/bifrost.pid"

stop_bifrost() {
    if [[ -f "$PIDFILE" ]]; then
        local pid
        pid=$(cat "$PIDFILE")
        kill -9 "$pid" 2>/dev/null || true
        # Also kill child ffmpeg
        pkill -9 -P "$pid" 2>/dev/null || true
        rm -f "$PIDFILE"
    fi
    pkill -9 -f 'ffmpeg.*(x11grab|kmsgrab)' 2>/dev/null || true
    sleep 0.2
}

start_bifrost() {
    stop_bifrost
    rm -f /tmp/bifrost_audio.mp3
    "$BINARY" > "$LOG" 2>&1 &
    echo $! > "$PIDFILE"
    echo "[dev] BIFROST started (PID $!)"
}

build() {
    echo "[dev] Building..."
    if go build -o "$BINARY" "$DIR/cmd/bifrost" 2>&1; then
        echo "[dev] Build OK"
        return 0
    else
        echo "[dev] Build FAILED"
        return 1
    fi
}

trap 'stop_bifrost; exit 0' INT TERM

# Initial build and start
build && start_bifrost

echo "[dev] Watching for changes (inotifywait)..."

while true; do
    inotifywait -qq -r -e modify,create,delete,move \
        --include '\.(go|html|tmpl|mod)$' \
        "$DIR/cmd" "$DIR/internal" 2>/dev/null || break
    echo "[dev] Change detected — rebuilding..."
    sleep 0.3
    build && start_bifrost
done

echo "[dev] inotifywait exited, stopping..."
stop_bifrost
