#!/usr/bin/env bash
# dev.sh — Development auto-restart for BIFROST
set -uo pipefail

DIR="$(cd "$(dirname "$0")" && pwd)"
LOG="/tmp/bifrost/bifrost.log"
PIDFILE="/tmp/bifrost/pids/dev"

stop_bifrost() {
    if [[ -f "$PIDFILE" ]]; then
        local pid
        pid=$(cat "$PIDFILE")
        kill -9 "$pid" 2>/dev/null || true
        pkill -9 -P "$pid" 2>/dev/null || true
        rm -f "$PIDFILE"
    fi
    pkill -9 -f 'bifrost.sh' 2>/dev/null || true
    pkill -9 -f 'ffmpeg.*(x11grab|kmsgrab)' 2>/dev/null || true
    pkill -9 -f 'frame_splitter.py' 2>/dev/null || true
    sleep 0.2
}

start_bifrost() {
    stop_bifrost
    mkdir -p /tmp/bifrost/frames /tmp/bifrost/audio /tmp/bifrost/pids
    bash "$DIR/bifrost.sh" --headless > "$LOG" 2>&1 &
    echo $! > "$PIDFILE"
    echo "[dev] BIFROST started (PID $!)"
}

trap 'stop_bifrost; exit 0' INT TERM

# Initial start
start_bifrost

echo "[dev] Watching for changes (inotifywait)..."

while true; do
    inotifywait -qq -r -e modify,create,delete,move \
        --include '\.(sh|py|html)$' \
        "$DIR/lib" "$DIR/web" "$DIR/scripts" "$DIR/bifrost.sh" 2>/dev/null || break
    echo "[dev] Change detected — restarting..."
    sleep 0.3
    start_bifrost
done

echo "[dev] inotifywait exited, stopping..."
stop_bifrost
