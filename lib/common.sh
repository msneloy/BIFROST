#!/usr/bin/env bash
# common.sh — BIFROST shared utilities, colors, logging, temp dirs, cleanup

set -euo pipefail

# ─── Version ──────────────────────────────────────────────────────────────────
BIFROST_VERSION="0.2.0"
BIFROST_PORT="${BIFROST_PORT:-8080}"
BIFROST_FPS="${BIFROST_FPS:-30}"
BIFROST_QUALITY="${BIFROST_QUALITY:-40}"
BIFROST_RESOLUTION="${BIFROST_RESOLUTION:-1920x1080}"

# ─── Paths ────────────────────────────────────────────────────────────────────
BIFROST_TMP="/tmp/bifrost"
BIFROST_FRAMES="$BIFROST_TMP/frames"
BIFROST_AUDIO="$BIFROST_TMP/audio"
BIFROST_PIDS="$BIFROST_TMP/pids"
BIFROST_LOG="$BIFROST_TMP/bifrost.log"
BIFROST_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# ─── Colors (BIFROST red theme) ──────────────────────────────────────────────
RED='\033[0;31m'
BRIGHT_RED='\033[1;31m'
DARK_RED='\033[2;31m'
ORANGE='\033[0;33m'
YELLOW='\033[1;33m'
GREEN='\033[0;32m'
BRIGHT_GREEN='\033[1;32m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
WHITE='\033[1;37m'
GRAY='\033[0;37m'
DIM='\033[2m'
BOLD='\033[1m'
RESET='\033[0m'

# ─── Logging ──────────────────────────────────────────────────────────────────
log()      { echo -e "${DIM}$(date '+%H:%M:%S')${RESET} ${GREEN}[+]${RESET} $*"; }
log_warn() { echo -e "${DIM}$(date '+%H:%M:%S')${RESET} ${YELLOW}[!]${RESET} $*" >&2; }
log_err()  { echo -e "${DIM}$(date '+%H:%M:%S')${RESET} ${RED}[✗]${RESET} $*" >&2; }
log_info() { echo -e "${DIM}$(date '+%H:%M:%S')${RESET} ${CYAN}[i]${RESET} $*"; }

# ─── Dependency checking ──────────────────────────────────────────────────────
require_cmd() {
    if ! command -v "$1" &>/dev/null; then
        log_err "Required command '$1' not found. Install it first."
        return 1
    fi
}

check_deps() {
    local missing=0
    for cmd in ffmpeg socat python3; do
        if ! command -v "$cmd" &>/dev/null; then
            log_err "Required: $cmd — install it (e.g. apt-get install $cmd)"
            missing=1
        fi
    done
    if [[ $missing -eq 1 ]]; then
        exit 1
    fi
    # Optional
    for cmd in avahi-publish mediamtx; do
        if ! command -v "$cmd" &>/dev/null; then
            log_warn "Optional: $cmd not found — some features may be unavailable"
        fi
    done
}

# ─── Network ──────────────────────────────────────────────────────────────────
detect_local_ip() {
    local ip
    ip=$(ip -4 route get 1.1.1.1 2>/dev/null | awk '{print $7; exit}')
    if [[ -z "$ip" ]]; then
        ip=$(ip -4 addr show scope global 2>/dev/null | awk '/inet /{print $2; exit}' | cut -d/ -f1)
    fi
    echo "${ip:-127.0.0.1}"
}

# ─── Temp directories ─────────────────────────────────────────────────────────
setup_tmp() {
    mkdir -p "$BIFROST_FRAMES" "$BIFROST_AUDIO" "$BIFROST_PIDS"
    # Clean old frames
    rm -f "$BIFROST_FRAMES"/*.jpg "$BIFROST_FRAMES"/COUNTER
    echo "0" > "$BIFROST_FRAMES/COUNTER"
}

cleanup_tmp() {
    rm -rf "$BIFROST_TMP"
}

# ─── PID management ───────────────────────────────────────────────────────────
save_pid() {
    local name="$1" pid="$2"
    echo "$pid" > "$BIFROST_PIDS/$name"
}

read_pid() {
    local name="$1"
    if [[ -f "$BIFROST_PIDS/$name" ]]; then
        cat "$BIFROST_PIDS/$name"
    fi
}

kill_pid() {
    local name="$1"
    local pid
    pid=$(read_pid "$name") || return 0
    if [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null; then
        kill "$pid" 2>/dev/null || true
        # Wait briefly for graceful shutdown
        local i=0
        while kill -0 "$pid" 2>/dev/null && [[ $i -lt 20 ]]; do
            sleep 0.1
            ((i++))
        done
        # Force kill if still alive
        if kill -0 "$pid" 2>/dev/null; then
            kill -9 "$pid" 2>/dev/null || true
        fi
    fi
    rm -f "$BIFROST_PIDS/$name"
}

# ─── Orphan cleanup ───────────────────────────────────────────────────────────
cleanup_orphans() {
    pkill -9 -f 'ffmpeg.*(x11grab|kmsgrab|pulse)' 2>/dev/null || true
    pkill -9 -f 'gst-launch-1.0' 2>/dev/null || true
    pkill -9 -f 'avahi-publish.*bifrost' 2>/dev/null || true
    pkill -9 -f 'frame_splitter.py' 2>/dev/null || true
    pkill -9 -f 'mediamtx.*bifrost' 2>/dev/null || true
    sleep 0.3
}

# ─── Graceful shutdown ────────────────────────────────────────────────────────
shutdown_bifrost() {
    log "Shutting down BIFROST..."
    # Kill all tracked PIDs
    if [[ -d "$BIFROST_PIDS" ]]; then
        for pidfile in "$BIFROST_PIDS"/*; do
            [[ -f "$pidfile" ]] || continue
            local pid
            pid=$(cat "$pidfile")
            if [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null; then
                kill "$pid" 2>/dev/null || true
            fi
        done
    fi
    # Also kill orphans
    cleanup_orphans
    # Clean up temp
    cleanup_tmp
    log "BIFROST stopped."
}

# ─── ASCII banner ─────────────────────────────────────────────────────────────
show_banner() {
    local ip="$1"
    local port="$2"
    echo -e "${BRIGHT_RED}"
    cat << 'BANNER'
 ____  _ _  ____(_) | (_)
| __ )(_) | |/ ___| |_| |_ _   _
|  _ \| | | | |   |  _  | | | | |
| |_) | | | | |___| | | | | |_| |
|____/|_|_|_|\____|_| |_|_|\__, |
                            |___/
BANNER
    echo -e "${RESET}"
    echo -e "  ${DIM}v${BIFROST_VERSION} — Browser Integrated Feed for Remote Observation & Screen Transmission${RESET}"
    echo ""
    echo -e "  ${BRIGHT_RED}▶ Web Viewer:${RESET}   ${WHITE}http://${ip}:${port}${RESET}"
    echo -e "  ${BRIGHT_RED}▶ mDNS:${RESET}         ${WHITE}http://bifrost.local:${port}${RESET}"
    echo -e "  ${BRIGHT_RED}▶ Health:${RESET}       ${WHITE}http://${ip}:${port}/health${RESET}"
    echo ""
}
