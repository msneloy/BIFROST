#!/usr/bin/env bash
# capture.sh — Screen and audio capture via ffmpeg / GStreamer

# Source common utilities
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

# ─── PulseAudio source detection ──────────────────────────────────────────────
detect_pulse_source() {
    if ! command -v pactl &>/dev/null; then
        echo ""
        return
    fi
    local sources
    sources=$(pactl list short sources 2>/dev/null) || { echo ""; return; }
    echo "$sources" | awk '/monitor/{print $2; exit}'
}

# ─── Session type detection ───────────────────────────────────────────────────
detect_session_type() {
    echo "${XDG_SESSION_TYPE:-x11}"
}

# ─── Display geometry ─────────────────────────────────────────────────────────
get_display_geometry() {
    local session
    session=$(detect_session_type)
    if [[ "$session" == "wayland" ]]; then
        # Try xdpyinfo or fallback to common resolution
        if command -v xdpyinfo &>/dev/null; then
            xdpyinfo 2>/dev/null | awk '/dimensions/{print $2}' && return
        fi
        echo "1920x1080"
    else
        # X11: use xrandr
        if command -v xrandr &>/dev/null; then
            xrandr 2>/dev/null | awk '/\*/{print $1; exit}' && return
        fi
        echo "1920x1080"
    fi
}

# ─── Start video capture ─────────────────────────────────────────────────────
start_video_capture() {
    local session="$1"
    local fps="$2"
    local quality="$3"
    local resolution="$4"
    local frame_dir="$BIFROST_FRAMES"

    mkdir -p "$frame_dir"

    if [[ "$session" == "wayland" ]]; then
        # Wayland: use Python Mutter/Portal capture
        local capture_script=""
        for candidate in \
            "$BIFROST_DIR/scripts/mutter_capture.py" \
            "$BIFROST_DIR/scripts/portal_capture.py" \
            "$BIFROST_DIR/scripts/capture.py"; do
            if [[ -f "$candidate" ]]; then
                capture_script="$candidate"
                break
            fi
        done

        if [[ -n "$capture_script" ]]; then
            log_info "Wayland capture: $capture_script"
            python3 "$capture_script" 2>"$BIFROST_TMP/capture.log" | \
                python3 "$SCRIPT_DIR/frame_splitter.py" &
            save_pid "video_capture" $!
        else
            log_err "No Wayland capture script found"
            return 1
        fi
    else
        # X11: use ffmpeg x11grab
        local display="${DISPLAY:-:0}"
        local display_geom
        display_geom=$(get_display_geometry)

        log_info "X11 capture: $display @ ${resolution:-$display_geom} ${fps}fps"
        ffmpeg -y -loglevel warning \
            -f x11grab -draw_mouse 1 \
            -video_size "${resolution:-$display_geom}" \
            -framerate "$fps" \
            -i "$display" \
            -f mjpeg -q:v "$quality" \
            pipe:1 2>"$BIFROST_TMP/capture.log" | \
            python3 "$SCRIPT_DIR/frame_splitter.py" &
        save_pid "video_capture" $!
    fi
}

# ─── Start audio capture ─────────────────────────────────────────────────────
start_audio_capture() {
    local audio_source="$1"

    if [[ -z "$audio_source" ]]; then
        log_warn "No PulseAudio monitor source — audio disabled"
        return 0
    fi

    log_info "Audio capture: $audio_source"
    ffmpeg -y -loglevel warning \
        -f pulse -i "$audio_source" \
        -c:a libmp3lame -b:a 128k \
        -f mp3 "$BIFROST_AUDIO/stream.mp3" \
        -c:a libmp3lame -b:a 128k \
        -f mp3 pipe:1 2>"$BIFROST_TMP/audio.log" &
    save_pid "audio_capture" $!
}

# ─── Start WebRTC RTP relay (additional ffmpeg outputs) ───────────────────────
start_webrtc_relay() {
    local session="$1"
    local fps="$2"
    local quality="$3"
    local resolution="$4"
    local audio_source="$5"

    # This adds VP8+Opus RTP outputs to a secondary ffmpeg process
    # that reads from the display again (for WebRTC via mediamtx)
    local display="${DISPLAY:-:0}"
    local display_geom
    display_geom=$(get_display_geometry)

    local audio_args=()
    if [[ -n "$audio_source" ]]; then
        audio_args=(-f pulse -i "$audio_source" -map 1:a -c:a libopus -b:a 64k -f rtp "udp://127.0.0.1:5005")
    fi

    log_info "WebRTC relay: VP8→UDP:5004, Opus→UDP:5005"
    ffmpeg -y -loglevel warning \
        -f x11grab -draw_mouse 1 \
        -video_size "${resolution:-$display_geom}" \
        -framerate "$fps" \
        -i "$display" \
        "${audio_args[@]}" \
        -map 0:v -c:v libvpx -b:v 2M -deadline realtime -cpu-used 4 -f rtp "udp://127.0.0.1:5004" \
        2>"$BIFROST_TMP/webrtc_relay.log" &
    save_pid "webrtc_relay" $!
}

# ─── Stop capture ─────────────────────────────────────────────────────────────
stop_capture() {
    kill_pid "video_capture"
    kill_pid "audio_capture"
    kill_pid "webrtc_relay"
}
