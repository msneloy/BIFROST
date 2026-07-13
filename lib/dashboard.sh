#!/usr/bin/env bash
# dashboard.sh — BASHTOP-style TUI dashboard with system stats and client monitoring

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"
source "$SCRIPT_DIR/stats.sh"
source "$SCRIPT_DIR/tracker.sh"

# ─── Constants ────────────────────────────────────────────────────────────────
DASH_WIDTH=120
DASH_INNER=$((DASH_WIDTH - 4))  # Account for box borders + padding

# ─── Progress bar ─────────────────────────────────────────────────────────────
bar() {
    local pct=$1 width=$2
    local filled=$((pct * width / 100))
    local empty=$((width - filled))

    # Color based on usage
    local color="$GREEN"
    if [[ $pct -ge 90 ]]; then
        color="$BRIGHT_RED"
    elif [[ $pct -ge 70 ]]; then
        color="$ORANGE"
    elif [[ $pct -ge 50 ]]; then
        color="$YELLOW"
    fi

    printf "${color}"
    printf '█%.0s' $(seq 1 $filled 2>/dev/null) || true
    printf "${DIM}"
    printf '░%.0s' $(seq 1 $empty 2>/dev/null) || true
    printf "${RESET}"
}

# ─── Horizontal line ──────────────────────────────────────────────────────────
hline() {
    local char="${1:-─}" width="${2:-$DASH_INNER}"
    printf '%*s' "$width" '' | tr ' ' "$char"
}

# ─── Box drawing ──────────────────────────────────────────────────────────────
box_top()    { printf "╭── %s $BRIGHT_RED$(hline '─' $((DASH_INNER - ${#1} - 4)))${RESET}╮\n" "$1"; }
box_bottom() { printf "╰$(hline '─' $DASH_INNER)╯\n"; }
box_line()   { printf "│%-$((DASH_INNER))s│\n" "$1"; }

# ─── Truncate/pad string to exact width ──────────────────────────────────────
pad() {
    local str="$1" width="$2"
    # Strip ANSI codes for length calculation
    local clean
    clean=$(echo -e "$str" | sed 's/\x1b\[[0-9;]*m//g')
    local len=${#clean}
    if [[ $len -gt $width ]]; then
        echo -e "${str:0:$width}"
    else
        local padding=$((width - len))
        printf "%s%${padding}s" "$str" ""
    fi
}

# ─── Format a stat line with bar ──────────────────────────────────────────────
stat_line() {
    local label="$1" pct="$2" detail="$3" width="${4:-12}"
    local bar_str
    bar_str=$(bar "$pct" "$width")
    local pct_str
    pct_str=$(printf "%3d%%" "$pct")
    echo -e "  ${BOLD}${label}:${RESET} ${bar_str} ${pct_str}  ${DIM}${detail}${RESET}"
}

# ─── Render dashboard ─────────────────────────────────────────────────────────
render_dashboard() {
    local ip="$1" port="$2" version="$3"
    local now
    now=$(date '+%H:%M:%S')

    # Gather stats
    local cpu_pct cpu_freq cpu_temp
    cpu_pct=$(get_cpu_usage)
    cpu_freq=$(get_cpu_freq)
    cpu_temp=$(get_cpu_temp)

    local mem_info swap_info
    mem_info=$(get_mem_info)
    swap_info=$(get_swap_info)

    local IFS='|'
    read -r mem_pct mem_used mem_total <<< "$mem_info"
    read -r swap_pct swap_used swap_total <<< "$swap_info"
    IFS=

    local gpu_info
    gpu_info=$(get_gpu_info)
    IFS='|'
    read -r gpu_freq gpu_temp <<< "$gpu_info"
    IFS=

    local disk_info
    disk_info=$(get_disk_info)
    IFS='|'
    read -r disk_total disk_used disk_pct <<< "$disk_info"
    IFS=
    disk_pct=${disk_pct%%%}

    local nic_info
    nic_info=$(get_nic_info)
    IFS='|'
    read -r nic_iface nic_speed nic_type <<< "$nic_info"
    IFS=

    local bat_info
    bat_info=$(get_battery_info)
    IFS='|'
    read -r bat_pct bat_status <<< "$bat_info"
    IFS=

    local fan_rpm
    fan_rpm=$(get_fan_rpm)

    local uptime_str
    uptime_str=$(get_uptime)

    local active_clients
    active_clients=$(tracker_count_active)

    # Move cursor to top-left and clear
    printf '\033[H\033[2J'

    # ─── Title bar ───────────────────────────────────────────────────────
    local title=" BIFROST v${version} "
    local url=" ${ip}:${port} "
    local time=" [${now}] "
    local uptime=" uptime: ${uptime_str} "
    local remaining=$((DASH_INNER - ${#title} - ${#url} - ${#time} - ${#uptime}))
    printf "╭${BRIGHT_RED}$(pad "$title" $((remaining/2 + ${#title})))${RESET}${DIM}$(pad "$url" $((remaining/2)))${RESET}$(pad "$time" ${#time})$(pad "$uptime" ${#uptime})${RESET}╮\n"

    # ─── System stats ────────────────────────────────────────────────────
    box_top "SYSTEM"
    echo -e "│$(pad "" $DASH_INNER)│" | head -c -1

    # CPU line
    local cpu_bar_width=15
    local cpu_bar
    cpu_bar=$(bar "$cpu_pct" "$cpu_bar_width")
    local cpu_detail="${cpu_freq}MHz ${cpu_temp}°C"
    box_line "$(pad "  CPU: ${cpu_bar} $(printf '%3d%%' $cpu_pct)  ${DIM}${cpu_detail}${RESET}" $DASH_INNER)"

    # RAM line
    local mem_bar
    mem_bar=$(bar "$mem_pct" "$cpu_bar_width")
    local mem_detail="${mem_used}/${mem_total}G"
    box_line "$(pad "  RAM: ${mem_bar} $(printf '%3d%%' $mem_pct)  ${DIM}${mem_detail}${RESET}" $DASH_INNER)"

    # GPU line
    local gpu_bar=0
    [[ "$gpu_freq" != "0" ]] && gpu_bar=30  # Approximate
    local gpu_bar_str
    gpu_bar_str=$(bar "$gpu_bar" "$cpu_bar_width")
    local gpu_detail="${gpu_freq}MHz ${gpu_temp}°C"
    box_line "$(pad "  GPU: ${gpu_bar_str} --  ${DIM}${gpu_detail}${RESET}" $DASH_INNER)"

    # DISK line
    local disk_bar
    disk_bar=$(bar "$disk_pct" "$cpu_bar_width")
    local disk_detail="${disk_used}/${disk_total}"
    box_line "$(pad "  DISK: ${disk_bar} $(printf '%3d%%' $disk_pct)  ${DIM}${disk_detail}${RESET}" $DASH_INNER)"

    # NIC + SWAP + BAT + FAN line
    local swap_bar
    swap_bar=$(bar "$swap_pct" 8)
    local bat_str=""
    if [[ "$bat_pct" != "N/A" ]]; then
        bat_str="BAT: ${bat_pct}% ${bat_status}"
    fi
    local fan_str="FAN: ${fan_rpm} RPM"
    local nic_str="NIC: ${nic_iface} ${nic_speed}Mb/s ${nic_type}"

    box_line "$(pad "  ${DIM}${nic_str}${RESET}" $((DASH_INNER/2)))$(pad "SWAP: ${swap_bar} $(printf '%3d%%' $swap_pct)  ${DIM}${swap_used}/${swap_total}G${RESET}" $((DASH_INNER/2)))"
    box_line "$(pad "  ${DIM}${fan_str}${RESET}" $((DASH_INNER/2)))$(pad "  ${DIM}${bat_str}${RESET}" $((DASH_INNER/2)))"

    box_bottom

    # ─── Client table ────────────────────────────────────────────────────
    box_top "STUDENTS (${active_clients} active)"

    # Header
    local header=$(printf "  ${BOLD}S │ # │ DEV │ IP ADDRESS     │ OS/BROWSER    │ BANDWIDTH      │ TOTAL${RESET}")
    box_line "$(pad "$header" $DASH_INNER)"

    # Separator
    box_line "$(pad "  $(hline '─' $((DASH_INNER - 4)))" $DASH_INNER)"

    # Client rows
    local client_data
    client_data=$(tracker_get_all)
    local row_count=0
    while IFS='|' read -r status idx dev ip_addr os_browser bw total latency; do
        [[ -z "$status" ]] && continue
        ((row_count++))
        [[ $row_count -gt 20 ]] && break

        # Color the status indicator
        local status_colored
        if [[ "$status" == "●" ]]; then
            status_colored="${GREEN}●${RESET}"
        else
            status_colored="${DIM}○${RESET}"
        fi

        # Build bandwidth bar
        local bw_display="${bw}/s"

        local line=$(printf "  %b │ %s │ %s │ %-14s │ %-12s │ %s │ %s" \
            "$status_colored" "$idx" "$dev" "$ip_addr" "$os_browser" "$bw_display" "$total")
        box_line "$(pad "$line" $DASH_INNER)"
    done <<< "$client_data"

    # If no clients
    if [[ $row_count -eq 0 ]]; then
        box_line "$(pad "  ${DIM}No connected students${RESET}" $DASH_INNER)"
    fi

    box_bottom

    # ─── Rejected clients ────────────────────────────────────────────────
    if [[ ${#REJECTION_IP[@]} -gt 0 ]]; then
        box_top "REJECTED"
        for i in $(seq 0 $(( ${#REJECTION_IP[@]} - 1 ))); do
            local rline=$(printf "  %s │ %s │ %s │ %s" \
                "${REJECTION_IP[$i]}" "${REJECTION_OS[$i]}" "${REJECTION_REASON[$i]}" "${REJECTION_TIME[$i]}")
            box_line "$(pad "$rline" $DASH_INNER)"
        done
        box_bottom
    fi

    # Footer
    echo ""
    echo -e "  ${DIM}Press Ctrl+C to stop BIFROST${RESET}"
}

# ─── Dashboard refresh loop ───────────────────────────────────────────────────
start_dashboard() {
    local ip="$1" port="$2" version="$3"
    local refresh_interval="${4:-1}"

    log_info "Dashboard started (refresh: ${refresh_interval}s)"

    while true; do
        render_dashboard "$ip" "$port" "$version"
        sleep "$refresh_interval"
    done
}
