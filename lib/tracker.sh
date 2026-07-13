#!/usr/bin/env bash
# tracker.sh — Client tracking with bash associative arrays

# ─── Client data stores (associative arrays) ──────────────────────────────────
declare -A TRACKER_HOST
declare -A TRACKER_MAC
declare -A TRACKER_BYTES
declare -A TRACKER_PREV_BYTES
declare -A TRACKER_LAST_SEEN
declare -A TRACKER_LATENCY
declare -A TRACKER_OS
declare -A TRACKER_BROWSER
declare -A TRACKER_RESOLUTION
declare -A TRACKER_DEVTYPE
declare -A TRACKER_GPU
declare -A TRACKER_BAT
declare -A TRACKER_CHARGING
declare -A TRACKER_ACTIVE

# Rejection log
declare -a REJECTION_IP=()
declare -a REJECTION_OS=()
declare -a REJECTION_REASON=()
declare -a REJECTION_TIME=()
declare -a REJECTION_UA=()
MAX_REJECTIONS=5

TRACKER_TOTAL_BYTES=0
TRACKER_START_TIME=$(date +%s)

# ─── Get or create client ─────────────────────────────────────────────────────
tracker_get_or_create() {
    local ip="$1"
    if [[ -z "${TRACKER_LAST_SEEN[$ip]:-}" ]]; then
        TRACKER_HOST[$ip]=""
        TRACKER_MAC[$ip]=""
        TRACKER_BYTES[$ip]=0
        TRACKER_PREV_BYTES[$ip]=0
        TRACKER_ACTIVE[$ip]=1
        TRACKER_LATENCY[$ip]=""
        TRACKER_OS[$ip]=""
        TRACKER_BROWSER[$ip]=""
        TRACKER_RESOLUTION[$ip]=""
        TRACKER_DEVTYPE[$ip]=""
        TRACKER_GPU[$ip]=""
        TRACKER_BAT[$ip]=""
        TRACKER_CHARGING[$ip]=""

        # Async DNS lookup
        _tracker_lookup_dns "$ip" &
        # Async MAC lookup
        _tracker_lookup_mac "$ip" &
    fi
    TRACKER_LAST_SEEN[$ip]=$(date +%s)
    TRACKER_ACTIVE[$ip]=1
}

# ─── Update client from /ping telemetry ──────────────────────────────────────
tracker_update_client() {
    local ip="$1" latency="$2" os="$3" browser="$4" resolution="$5" device="$6" gpu="$7" battery="$8"
    tracker_get_or_create "$ip"
    [[ -n "$latency" ]]    && TRACKER_LATENCY[$ip]="$latency"
    [[ -n "$os" ]]         && TRACKER_OS[$ip]="$os"
    [[ -n "$browser" ]]    && TRACKER_BROWSER[$ip]="$browser"
    [[ -n "$resolution" ]] && TRACKER_RESOLUTION[$ip]="$resolution"
    [[ -n "$device" ]]     && TRACKER_DEVTYPE[$ip]="$device"
    [[ -n "$gpu" ]]        && TRACKER_GPU[$ip]="$gpu"
    [[ -n "$battery" ]]    && TRACKER_BAT[$ip]="$battery"
}

# ─── Add bytes transferred ────────────────────────────────────────────────────
tracker_add_bytes() {
    local ip="$1" n="$2"
    TRACKER_BYTES[$ip]=$(( ${TRACKER_BYTES[$ip]:-0} + n ))
    TRACKER_TOTAL_BYTES=$(( TRACKER_TOTAL_BYTES + n ))
}

# ─── Log rejection ────────────────────────────────────────────────────────────
tracker_log_rejection() {
    local ip="$1" os="$2" reason="$3" ua="$4"
    # Prepend (newest first)
    REJECTION_IP=("$ip" "${REJECTION_IP[@]:0:$((MAX_REJECTIONS-1))}")
    REJECTION_OS=("$os" "${REJECTION_OS[@]:0:$((MAX_REJECTIONS-1))}")
    REJECTION_REASON=("$reason" "${REJECTION_REASON[@]:0:$((MAX_REJECTIONS-1))}")
    REJECTION_TIME=("$(date '+%H:%M:%S')" "${REJECTION_TIME[@]:0:$((MAX_REJECTIONS-1))}")
    REJECTION_UA=("$ua" "${REJECTION_UA[@]:0:$((MAX_REJECTIONS-1))}")
}

# ─── Prune inactive clients ───────────────────────────────────────────────────
tracker_prune() {
    local timeout="${1:-30}"
    local now
    now=$(date +%s)
    for ip in "${!TRACKER_LAST_SEEN[@]}"; do
        local age=$(( now - ${TRACKER_LAST_SEEN[$ip]:-0} ))
        if [[ $age -gt $timeout ]]; then
            TRACKER_ACTIVE[$ip]=0
        fi
    done
}

# ─── Count active clients ────────────────────────────────────────────────────
tracker_count_active() {
    local count=0
    for ip in "${!TRACKER_ACTIVE[@]}"; do
        [[ "${TRACKER_ACTIVE[$ip]}" == "1" ]] && ((count++))
    done
    echo "$count"
}

# ─── Get all clients as formatted table rows ──────────────────────────────────
tracker_get_all() {
    local idx=0
    for ip in $(printf '%s\n' "${!TRACKER_ACTIVE[@]}" | sort); do
        ((idx++))
        local status="○"
        [[ "${TRACKER_ACTIVE[$ip]}" == "1" ]] && status="●"

        local dev_icon="💻"
        [[ "${TRACKER_DEVTYPE[$ip]}" == "mobile" ]] && dev_icon="📱"

        # Calculate bandwidth (bytes since last call)
        local bytes=${TRACKER_BYTES[$ip]:-0}
        local prev=${TRACKER_PREV_BYTES[$ip]:-0}
        local delta=$(( bytes - prev ))
        TRACKER_PREV_BYTES[$ip]=$bytes

        local bw_human
        if [[ $delta -gt 1048576 ]]; then
            bw_human=$(awk "BEGIN{printf \"%.1fM\", $delta/1048576}")
        elif [[ $delta -gt 1024 ]]; then
            bw_human=$(awk "BEGIN{printf \"%.1fK\", $delta/1024}")
        else
            bw_human="${delta}B"
        fi

        local total_human
        if [[ $bytes -gt 1048576 ]]; then
            total_human=$(awk "BEGIN{printf \"%.1fM\", $bytes/1048576}")
        elif [[ $bytes -gt 1024 ]]; then
            total_human=$(awk "BEGIN{printf \"%.1fK\", $bytes/1024}")
        else
            total_human="${bytes}B"
        fi

        local os="${TRACKER_OS[$ip]:--}"
        local browser="${TRACKER_BROWSER[$ip]:--}"
        local latency="${TRACKER_LATENCY[$ip]:--}"

        printf "%s|%d|%s|%s|%s/%s|%s|%s|%s|%s\n" \
            "$status" "$idx" "$dev_icon" "$ip" "$os" "$browser" "$bw_human" "$total_human" "$latency"
    done
}

# ─── Get stats as JSON ───────────────────────────────────────────────────────
tracker_get_stats_json() {
    local now
    now=$(date +%s)
    local uptime=$(( now - TRACKER_START_TIME ))
    local active
    active=$(tracker_count_active)
    local total_human
    if [[ $TRACKER_TOTAL_BYTES -gt 1048576 ]]; then
        total_human=$(awk "BEGIN{printf \"%.1f\", $TRACKER_TOTAL_BYTES/1048576}")
    elif [[ $TRACKER_TOTAL_BYTES -gt 1024 ]]; then
        total_human=$(awk "BEGIN{printf \"%.1f\", $TRACKER_TOTAL_BYTES/1024}")
    else
        total_human="$TRACKER_TOTAL_BYTES"
    fi

    printf '{"total_transmitted":"%s bytes","clients":%d,"uptime":%d,"rejections":%d}' \
        "$TRACKER_TOTAL_BYTES" "$active" "$uptime" "${#REJECTION_IP[@]}"
}

# ─── DNS reverse lookup (background) ──────────────────────────────────────────
_tracker_lookup_dns() {
    local ip="$1"
    local host
    host=$(getent hosts "$ip" 2>/dev/null | awk '{print $2; exit}')
    if [[ -n "$host" ]]; then
        TRACKER_HOST[$ip]="$host"
    else
        TRACKER_HOST[$ip]="$ip"
    fi
}

# ─── MAC address lookup from /proc/net/arp ───────────────────────────────────
_tracker_lookup_mac() {
    local ip="$1"
    local mac=""
    if [[ -f /proc/net/arp ]]; then
        mac=$(awk -v ip="$ip" '$1 == ip {print $4; exit}' /proc/net/arp)
    fi
    TRACKER_MAC[$ip]="${mac:-unknown}"
}
