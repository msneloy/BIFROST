#!/usr/bin/env bash
# bifrost.sh — BIFROST: Browser Integrated Feed for Remote Observation & Screen Transmission
# A zero-configuration classroom screen broadcasting server.
# Compatible with all Linux distributions.

set -euo pipefail

# ─── Resolve script directory ─────────────────────────────────────────────────
BIFROST_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$BIFROST_DIR/lib/common.sh"

# ─── CLI flags ────────────────────────────────────────────────────────────────
HEADLESS=false
NO_AUDIO=false
NO_WEBRTC=false
PORT="${BIFROST_PORT:-8080}"
FPS="${BIFROST_FPS:-30}"
QUALITY="${BIFROST_QUALITY:-40}"
RESOLUTION="${BIFROST_RESOLUTION:-1920x1080}"

show_help() {
    cat << EOF
BIFROST v${BIFROST_VERSION} — Browser Integrated Feed for Remote Observation & Screen Transmission

Usage: bifrost [OPTIONS]

Options:
  --port PORT         HTTP server port (default: 8080)
  --fps FPS           Capture frame rate (default: 30)
  --quality Q         JPEG quality 1-100 (default: 40)
  --resolution WxH    Capture resolution (default: 1920x1080)
  --headless          Skip TUI dashboard, log to stdout only
  --no-audio          Disable audio capture
  --no-webrtc         Disable WebRTC (MJPEG only)
  --help              Show this help message

Dependencies:
  Required: ffmpeg, socat, python3
  Optional: avahi-daemon + avahi-utils (mDNS), mediamtx (WebRTC)

EOF
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --port)      PORT="$2"; shift 2 ;;
        --fps)       FPS="$2"; shift 2 ;;
        --quality)   QUALITY="$2"; shift 2 ;;
        --resolution) RESOLUTION="$2"; shift 2 ;;
        --headless)  HEADLESS=true; shift ;;
        --no-audio)  NO_AUDIO=true; shift ;;
        --no-webrtc) NO_WEBRTC=true; shift ;;
        --help|-h)   show_help; exit 0 ;;
        *)           log_err "Unknown option: $1"; show_help; exit 1 ;;
    esac
done

# ─── Startup ──────────────────────────────────────────────────────────────────

# Setup temp directories
setup_tmp

# Kill orphaned processes from previous runs
cleanup_orphans

# Check dependencies
check_deps

# Detect local IP
LOCAL_IP=$(detect_local_ip)

# Show banner
show_banner "$LOCAL_IP" "$PORT"

# ─── Graceful shutdown on SIGINT/SIGTERM ──────────────────────────────────────
trap shutdown_bifrost EXIT INT TERM

# ─── Initialize subsystems ───────────────────────────────────────────────────

# 1. Start screen capture
log "Starting screen capture (${RESOLUTION} @ ${FPS}fps, quality ${QUALITY})..."
source "$BIFROST_DIR/lib/capture.sh"
SESSION_TYPE=$(detect_session_type)
start_video_capture "$SESSION_TYPE" "$FPS" "$QUALITY" "$RESOLUTION"

# 2. Start audio capture
if [[ "$NO_AUDIO" == "false" ]]; then
    AUDIO_SOURCE=$(detect_pulse_source)
    start_audio_capture "$AUDIO_SOURCE"
fi

# 3. Start mDNS
source "$BIFROST_DIR/lib/mdns.sh"
mdns_register "$LOCAL_IP"

# 4. Start WebRTC (optional)
if [[ "$NO_WEBRTC" == "false" ]]; then
    source "$BIFROST_DIR/lib/webrtc.sh"
    start_webrtc
fi

# 5. Start HTTP server
source "$BIFROST_DIR/lib/stream.sh"
start_http_server "$PORT"

# Wait a moment for capture to produce first frames
sleep 1

# 6. Start TUI dashboard (unless headless)
if [[ "$HEADLESS" == "true" ]]; then
    log_info "Running in headless mode (no TUI dashboard)"
    log "BIFROST is running. Press Ctrl+C to stop."
    # In headless mode, just wait for signals
    while true; do
        sleep 60
    done
else
    # Start dashboard (this blocks until SIGINT)
    source "$BIFROST_DIR/lib/dashboard.sh"
    start_dashboard "$LOCAL_IP" "$PORT" "$BIFROST_VERSION"
fi
