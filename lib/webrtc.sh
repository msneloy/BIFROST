#!/usr/bin/env bash
# webrtc.sh — WebRTC gateway via mediamtx (optional)

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

MEDIAMTX_PORT="${BIFROST_WEBRTC_PORT:-8889}"

# ─── Check if mediamtx is available ───────────────────────────────────────────
webrtc_available() {
    command -v mediamtx &>/dev/null
}

# ─── Create mediamtx config ───────────────────────────────────────────────────
_create_mediamtx_config() {
    local config_dir="$BIFROST_TMP"
    local config_file="$config_dir/mediamtx.yml"

    cat > "$config_file" << 'YAML'
# BIFROST mediamtx configuration
rtmp: no
rtsp: no
hls: no
webrtc: yes
webrtcPort: 8889
webrtcICERestrike: true

paths:
  bifrost:
    source: publisher
    sourceFingerprint: ""
YAML

    echo "$config_file"
}

# ─── Start mediamtx WebRTC gateway ────────────────────────────────────────────
start_webrtc() {
    if ! webrtc_available; then
        log_warn "mediamtx not found — WebRTC disabled (MJPEG-only mode)"
        return 0
    fi

    log_info "WebRTC: starting mediamtx gateway"

    local config_file
    config_file=$(_create_mediamtx_config)

    mediamtx "$config_file" 2>"$BIFROST_TMP/mediamtx.log" &
    save_pid "webrtc" $!

    # Wait for it to start
    sleep 1
    if kill -0 "$(read_pid webrtc)" 2>/dev/null; then
        log "WebRTC: mediamtx listening on :${MEDIAMTX_PORT}"
    else
        log_err "WebRTC: mediamtx failed to start (check $BIFROST_TMP/mediamtx.log)"
    fi
}

# ─── Stop WebRTC gateway ──────────────────────────────────────────────────────
stop_webrtc() {
    kill_pid "webrtc"
}
