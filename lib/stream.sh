#!/usr/bin/env bash
# stream.sh — socat-based HTTP server with route handling

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"
source "$SCRIPT_DIR/tracker.sh"
source "$SCRIPT_DIR/guard.sh"

# ─── HTTP response helpers ────────────────────────────────────────────────────
http_response() {
    local status="$1" content_type="$2" body="$3"
    printf "HTTP/1.1 %s\r\nContent-Type: %s\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s" \
        "$status" "$content_type" "${#body}" "$body"
}

http_response_file() {
    local status="$1" content_type="$2" file="$3"
    local size
    size=$(stat -c%s "$file" 2>/dev/null || echo 0)
    printf "HTTP/1.1 %s\r\nContent-Type: %s\r\nContent-Length: %d\r\nConnection: close\r\n\r\n" \
        "$status" "$content_type" "$size"
    cat "$file"
}

http_response_nocache() {
    local status="$1" content_type="$2" body="$3"
    printf "HTTP/1.1 %s\r\nContent-Type: %s\r\nContent-Length: %d\r\nCache-Control: no-cache, no-store, must-revalidate\r\nConnection: close\r\n\r\n%s" \
        "$status" "$content_type" "${#body}" "$body"
}

http_204() {
    printf "HTTP/1.1 204 No Content\r\nConnection: close\r\n\r\n"
}

http_404() {
    local body="Not Found"
    printf "HTTP/1.1 404 Not Found\r\nContent-Type: text/plain\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s" \
        "${#body}" "$body"
}

http_json() {
    local status="$1" json="$2"
    printf "HTTP/1.1 %s\r\nContent-Type: application/json\r\nContent-Length: %d\r\nCache-Control: no-cache\r\nConnection: close\r\n\r\n%s" \
        "$status" "${#json}" "$json"
}

# ─── MJPEG streaming handler ─────────────────────────────────────────────────
handle_stream() {
    local client_ip="$1"
    tracker_get_or_create "$client_ip"

    printf "HTTP/1.1 200 OK\r\n"
    printf "Content-Type: multipart/x-mixed-replace; boundary=bifrost\r\n"
    printf "Cache-Control: no-cache, private\r\n"
    printf "Connection: close\r\n"
    printf "\r\n"

    local last_counter=0
    while true; do
        # Read current counter
        local counter
        counter=$(cat "$BIFROST_FRAMES/COUNTER" 2>/dev/null || echo "0")

        if [[ "$counter" -gt "$last_counter" ]]; then
            local frame_file
            frame_file=$(printf "%s/%08d.jpg" "$BIFROST_FRAMES" "$counter")
            if [[ -f "$frame_file" ]]; then
                local frame_size
                frame_size=$(stat -c%s "$frame_file" 2>/dev/null || echo 0)
                printf -- "--bifrost\r\nContent-Type: image/jpeg\r\nContent-Length: %d\r\n\r\n" "$frame_size"
                cat "$frame_file"
                printf "\r\n"

                # Track bytes
                tracker_add_bytes "$client_ip" "$((frame_size + 50))"  # +50 for headers
            fi
            last_counter=$counter
        fi

        # Check if client is still connected (non-blocking read with timeout)
        if ! read -t 0.033 -r _ &>/dev/null; then
            # No data from client = still connected (MJPEG is one-way)
            :
        fi

        sleep 0.033  # ~30fps target
    done
}

# ─── Audio streaming handler ──────────────────────────────────────────────────
handle_audio() {
    printf "HTTP/1.1 200 OK\r\n"
    printf "Content-Type: audio/mpeg\r\n"
    printf "Cache-Control: no-cache, private\r\n"
    printf "Connection: close\r\n"
    printf "\r\n"

    if [[ -f "$BIFROST_AUDIO/stream.mp3" ]]; then
        tail -c +1 -f "$BIFROST_AUDIO/stream.mp3" 2>/dev/null
    else
        # Wait for audio file to appear
        local waited=0
        while [[ ! -f "$BIFROST_AUDIO/stream.mp3" ]] && [[ $waited -lt 10 ]]; do
            sleep 0.5
            ((waited++))
        done
        if [[ -f "$BIFROST_AUDIO/stream.mp3" ]]; then
            tail -c +1 -f "$BIFROST_AUDIO/stream.mp3" 2>/dev/null
        fi
    fi
}

# ─── Single frame handler ─────────────────────────────────────────────────────
handle_frame() {
    local counter
    counter=$(cat "$BIFROST_FRAMES/COUNTER" 2>/dev/null || echo "0")

    if [[ "$counter" -eq 0 ]]; then
        http_204
        return
    fi

    local frame_file
    frame_file=$(printf "%s/%08d.jpg" "$BIFROST_FRAMES" "$counter")

    if [[ -f "$frame_file" ]]; then
        http_response_file "200 OK" "image/jpeg" "$frame_file"
    else
        http_204
    fi
}

# ─── Client connection handler ────────────────────────────────────────────────
handle_connection() {
    local request_line=""
    local client_ip=""
    local method="" path="" version=""
    local user_agent="" content_length=0
    local query_string=""

    # Read HTTP request
    read -r request_line || return

    # Parse request line
    method=$(echo "$request_line" | awk '{print $1}')
    local raw_path
    raw_path=$(echo "$request_line" | awk '{print $2}')
    version=$(echo "$request_line" | awk '{print $3}')

    # Extract query string
    if [[ "$raw_path" == *"?"* ]]; then
        path="${raw_path%%\?*}"
        query_string="${raw_path#*\?}"
    else
        path="$raw_path"
    fi

    # Read headers
    while IFS= read -r header; do
        header=$(echo "$header" | tr -d '\r')
        [[ -z "$header" ]] && break

        case "${header,,}" in
            user-agent:*) user_agent="${header#*: }" ;;
            content-length:*) content_length="${header#*: }" ;;
        esac
    done

    # Get client IP (from socat, typically in REMOTE_ADDR or we parse it)
    client_ip="${REMOTE_ADDR:-127.0.0.1}"
    # Also try to extract from X-Forwarded-For or connection info
    if [[ -z "$client_ip" || "$client_ip" == "127.0.0.1" ]]; then
        client_ip=$(echo "$PEER" 2>/dev/null | grep -oP '\d+\.\d+\.\d+\.\d+' | head -1)
        client_ip="${client_ip:-127.0.0.1}"
    fi

    # ─── Route handling ───────────────────────────────────────────────────
    case "$method $path" in
        "GET /"|"GET /watch")
            # Windows guard
            if guard_check "$client_ip" "$user_agent"; then
                return
            fi
            http_response_file "200 OK" "text/html; charset=utf-8" "$BIFROST_DIR/web/player.html"
            ;;

        "GET /stream")
            if guard_check "$client_ip" "$user_agent"; then
                return
            fi
            tracker_get_or_create "$client_ip"
            handle_stream "$client_ip"
            ;;

        "GET /frame")
            if guard_check "$client_ip" "$user_agent"; then
                return
            fi
            tracker_get_or_create "$client_ip"
            handle_frame
            ;;

        "GET /audio")
            handle_audio
            ;;

        "GET /ping")
            # Parse telemetry query params
            local latency="" os="" browser="" resolution="" device="" gpu="" battery=""
            if [[ -n "$query_string" ]]; then
                IFS='&' read -ra params <<< "$query_string"
                for param in "${params[@]}"; do
                    local key="${param%%=*}"
                    local val="${param#*=}"
                    val=$(printf '%b' "${val//%/\\x}" 2>/dev/null || echo "$val")  # URL decode
                    case "$key" in
                        latency)    latency="$val" ;;
                        os)         os="$val" ;;
                        browser)    browser="$val" ;;
                        resolution) resolution="$val" ;;
                        device)     device="$val" ;;
                        gpu)        gpu="$val" ;;
                        battery)    battery="$val" ;;
                    esac
                done
            fi
            tracker_update_client "$client_ip" "$latency" "$os" "$browser" "$resolution" "$device" "$gpu" "$battery"
            http_204
            ;;

        "GET /rejected")
            local reason="Unknown"
            if [[ -n "$query_string" ]]; then
                IFS='&' read -ra params <<< "$query_string"
                for param in "${params[@]}"; do
                    local key="${param%%=*}"
                    local val="${param#*=}"
                    val=$(printf '%b' "${val//%/\\x}" 2>/dev/null || echo "$val")
                    [[ "$key" == "reason" ]] && reason="$val"
                done
            fi
            tracker_log_rejection "$client_ip" "Unknown" "$reason" "$user_agent"
            http_204
            ;;

        "GET /health")
            local active
            active=$(tracker_count_active)
            local streaming="true"
            if [[ ! -f "$BIFROST_FRAMES/COUNTER" ]]; then
                streaming="false"
            fi
            http_json "200 OK" "{\"streaming\":$streaming,\"clients\":$active}"
            ;;

        "GET /stats")
            local stats_json
            stats_json=$(tracker_get_stats_json)
            http_json "200 OK" "$stats_json"
            ;;

        "POST /webrtc/offer")
            # Read request body
            local body=""
            if [[ "$content_length" -gt 0 ]]; then
                read -r -N "$content_length" body
            fi
            # Forward to mediamtx if available
            if command -v mediamtx &>/dev/null; then
                local answer
                answer=$(curl -s -X POST http://127.0.0.1:8889/mwhip/webrtc/bifrost \
                    -H "Content-Type: application/sdp" \
                    -d "$body" 2>/dev/null)
                if [[ -n "$answer" ]]; then
                    http_json "200 OK" "{\"sdp\":$(echo "$answer" | jq -Rs .)}"
                else
                    http_json "502 Bad Gateway" "{\"error\":\"mediamtx unavailable\"}"
                fi
            else
                http_json "501 Not Implemented" "{\"error\":\"WebRTC not available\"}"
            fi
            ;;

        *)
            http_404
            ;;
    esac
}

# ─── Start HTTP server ────────────────────────────────────────────────────────
start_http_server() {
    local port="$1"
    log_info "HTTP server starting on port $port"

    # Export variables needed by handler
    export BIFROST_DIR BIFROST_TMP BIFROST_FRAMES BIFROST_AUDIO
    export BIFROST_VERSION BIFROST_PORT

    # Start socat HTTP server
    socat TCP-LISTEN:"$port",reuseaddr,fork \
        SYSTEM:"bash '$SCRIPT_DIR/stream.sh' --handle" &
    save_pid "http_server" $!
    log "HTTP server listening on :$port"
}

stop_http_server() {
    kill_pid "http_server"
}

# ─── Main (when called directly or via socat fork) ────────────────────────────
if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
    if [[ "${1:-}" == "--handle" ]]; then
        # socat fork mode: handle a single connection
        handle_connection
    fi
fi
