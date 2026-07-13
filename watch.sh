#!/usr/bin/env bash
# watch.sh — Auto-restart BIFROST on source changes
# Requires inotifywait (package: inotify-tools)

set -euo pipefail

DIR="$(cd "$(dirname "$0")" && pwd)"

# Ensure inotifywait is available
if ! command -v inotifywait &>/dev/null; then
    echo "Error: inotifywait not found. Install with 'sudo apt-get install inotify-tools'" >&2
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

stop_existing() {
    if command -v pkill &>/dev/null; then
        pkill -x bifrost.sh 2>/dev/null || true
        pkill -f 'bifrost.sh' 2>/dev/null || true
        sleep 1
    fi
}

watch_paths=(lib web scripts bifrost.sh)
watch_include='.*\.(sh|py|html)$'
watch_exclude='(^|/)\..*'

while true; do
    stop_existing

    echo "[watch] Starting BIFROST..."
    bash "$DIR/bifrost.sh" --headless &
    BIFROST_PID=$!

    echo "[watch] Watching for source changes..."
    inotifywait -qq -e modify,create,delete,move -r \
        --exclude "$watch_exclude" --include "$watch_include" \
        "${watch_paths[@]}" >/dev/null

    echo "[watch] Change detected — restarting BIFROST"
    kill "$BIFROST_PID" 2>/dev/null || true
    wait "$BIFROST_PID" 2>/dev/null || true
    sleep 1
done
