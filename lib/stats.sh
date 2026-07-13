#!/usr/bin/env bash
# stats.sh — System metrics from /proc and /sys (BASHTOP-style)

# ─── CPU usage from /proc/stat ────────────────────────────────────────────────
STATS_PREV_IDLE=0
STATS_PREV_TOTAL=0

get_cpu_usage() {
    local line
    line=$(head -1 /proc/stat 2>/dev/null) || { echo "0"; return; }

    local user nice sys idle iowait irq softirq steal
    read -r _ user nice sys idle iowait irq softirq steal <<< "$line"
    idle=$((idle + iowait))
    local total=$((user + nice + sys + idle + irq + softirq + steal))

    local diff_idle=$((idle - STATS_PREV_IDLE))
    local diff_total=$((total - STATS_PREV_TOTAL))

    STATS_PREV_IDLE=$idle
    STATS_PREV_TOTAL=$total

    if [[ $diff_total -gt 0 ]]; then
        echo $(( (diff_total - diff_idle) * 100 / diff_total ))
    else
        # Fallback: load average
        local load1
        load1=$(awk '{printf "%d", $1}' /proc/loadavg 2>/dev/null || echo 0)
        local ncpu
        ncpu=$(nproc 2>/dev/null || echo 1)
        local pct=$(( load1 * 100 / ncpu ))
        [[ $pct -gt 100 ]] && pct=100
        echo "$pct"
    fi
}

# ─── CPU frequency ────────────────────────────────────────────────────────────
get_cpu_freq() {
    local freq=""
    if [[ -f /sys/devices/system/cpu/cpu0/cpufreq/scaling_cur_freq ]]; then
        freq=$(cat /sys/devices/system/cpu/cpu0/cpufreq/scaling_cur_freq 2>/dev/null)
        freq=$((freq / 1000))  # Convert kHz to MHz
    fi
    echo "${freq:-0}"
}

# ─── CPU temperature ──────────────────────────────────────────────────────────
get_cpu_temp() {
    local temp=""
    # Try thermal zones first
    for zone in /sys/class/thermal/thermal_zone*/temp; do
        if [[ -f "$zone" ]]; then
            temp=$(cat "$zone" 2>/dev/null)
            temp=$((temp / 1000))
            break
        fi
    done
    # Try hwmon
    if [[ -z "$temp" ]]; then
        for hwmon in /sys/class/hwmon/hwmon*/temp1_input; do
            if [[ -f "$hwmon" ]]; then
                temp=$(cat "$hwmon" 2>/dev/null)
                temp=$((temp / 1000))
                break
            fi
        done
    fi
    echo "${temp:-0}"
}

# ─── Memory info ──────────────────────────────────────────────────────────────
get_mem_info() {
    local total=0 avail=0
    if [[ -f /proc/meminfo ]]; then
        total=$(awk '/^MemTotal:/{print $2}' /proc/meminfo)
        avail=$(awk '/^MemAvailable:/{print $2}' /proc/meminfo)
        # Fallback if MemAvailable not present
        [[ -z "$avail" ]] && avail=$(awk '/^MemFree:/{print $2}' /proc/meminfo)
    fi
    local used=$((total - avail))
    local pct=0
    [[ $total -gt 0 ]] && pct=$((used * 100 / total))

    local total_gb=$(awk "BEGIN{printf \"%.1f\", $total/1048576}")
    local used_gb=$(awk "BEGIN{printf \"%.1f\", $used/1048576}")
    echo "$pct|$used_gb|$total_gb"
}

# ─── Swap info ────────────────────────────────────────────────────────────────
get_swap_info() {
    local total=0 free=0
    if [[ -f /proc/meminfo ]]; then
        total=$(awk '/^SwapTotal:/{print $2}' /proc/meminfo)
        free=$(awk '/^SwapFree:/{print $2}' /proc/meminfo)
    fi
    local used=$((total - free))
    local pct=0
    [[ $total -gt 0 ]] && pct=$((used * 100 / total))

    local total_gb=$(awk "BEGIN{printf \"%.1f\", $total/1048576}")
    local used_gb=$(awk "BEGIN{printf \"%.1f\", $used/1048576}")
    echo "$pct|$used_gb|$total_gb"
}

# ─── GPU info ─────────────────────────────────────────────────────────────────
get_gpu_info() {
    local freq="" temp=""
    # DRM card frequency
    for card in /sys/class/drm/card*/device/pp_dpm_sclk; do
        if [[ -f "$card" ]]; then
            freq=$(awk '/\*/{gsub(/MHz/,"",$1); print $1; exit}' "$card" 2>/dev/null)
            break
        fi
    done
    # GPU temperature from hwmon (often at index 2 or specific GPU hwmon)
    for hwmon in /sys/class/hwmon/hwmon*/temp*_input; do
        if [[ -f "$hwmon" ]]; then
            local label_file="${hwmon%_*}_label"
            if [[ -f "$label_file" ]]; then
                local label
                label=$(cat "$label_file" 2>/dev/null)
                if [[ "${label,,}" == *"gpu"* || "${label,,}" == *"edge"* ]]; then
                    temp=$(cat "$hwmon" 2>/dev/null)
                    temp=$((temp / 1000))
                    break
                fi
            fi
        fi
    done
    echo "${freq:-0}|${temp:-0}"
}

# ─── Disk info ────────────────────────────────────────────────────────────────
get_disk_info() {
    local usage
    usage=$(df -h / 2>/dev/null | awk 'NR==2{print $2"|"$3"|"$5}' || echo "0|0|0%")
    echo "$usage"
}

# ─── NIC info ─────────────────────────────────────────────────────────────────
get_nic_info() {
    local iface="" speed="" type="Ethernet"
    for nic in /sys/class/net/*/; do
        local name
        name=$(basename "$nic")
        # Skip loopback and virtual interfaces
        [[ "$name" == "lo" ]] && continue
        [[ "$name" == docker0 ]] && continue
        [[ "$name" == br-* ]] && continue
        [[ "$name" == veth* ]] && continue
        [[ "$name" == virbr* ]] && continue

        iface="$name"
        # Speed
        if [[ -f "$nic/speed" ]]; then
            speed=$(cat "$nic/speed" 2>/dev/null)
            [[ "$speed" == "-1" ]] && speed=""
        fi
        # Type detection
        if [[ -d "/sys/class/net/$name/wireless" ]] || [[ -d "/sys/class/net/$name/phy80211" ]]; then
            type="WiFi"
        fi
        break
    done
    echo "${iface:-lo}|${speed:-0}|$type"
}

# ─── Battery info ─────────────────────────────────────────────────────────────
get_battery_info() {
    local pct="" status="" energy_now="" energy_full=""
    for bat in /sys/class/power_supply/BAT*/; do
        if [[ -d "$bat" ]]; then
            if [[ -f "$bat/capacity" ]]; then
                pct=$(cat "$bat/capacity" 2>/dev/null)
            fi
            if [[ -f "$bat/status" ]]; then
                status=$(cat "$bat/status" 2>/dev/null)
            fi
            break
        fi
    done
    echo "${pct:-N/A}|${status:-N/A}"
}

# ─── Fan RPM ──────────────────────────────────────────────────────────────────
get_fan_rpm() {
    local rpm=""
    # Try hwmon fans
    for fan in /sys/class/hwmon/hwmon*/fan*_input; do
        if [[ -f "$fan" ]]; then
            rpm=$(cat "$fan" 2>/dev/null)
            [[ "$rpm" -gt 0 ]] && break
            rpm=""
        fi
    done
    # Fallback: cooling devices
    if [[ -z "$rpm" ]]; then
        for cool in /sys/class/thermal/cooling_device*/cur_state; do
            if [[ -f "$cool" ]]; then
                local state
                state=$(cat "$cool" 2>/dev/null)
                [[ "$state" -gt 0 ]] && rpm="$((state * 100))"
            fi
        done
    fi
    echo "${rpm:-0}"
}

# ─── System uptime ────────────────────────────────────────────────────────────
get_uptime() {
    local upseconds
    upseconds=$(awk '{print int($1)}' /proc/uptime 2>/dev/null || echo 0)
    local days=$((upseconds / 86400))
    local hours=$(( (upseconds % 86400) / 3600 ))
    local mins=$(( (upseconds % 3600) / 60 ))
    if [[ $days -gt 0 ]]; then
        echo "${days}d ${hours}h"
    elif [[ $hours -gt 0 ]]; then
        echo "${hours}h ${mins}m"
    else
        echo "${mins}m"
    fi
}

# ─── CPU model name ───────────────────────────────────────────────────────────
get_cpu_model() {
    awk -F': ' '/model name/{print $2; exit}' /proc/cpuinfo 2>/dev/null || echo "Unknown"
}

# ─── CPU core count ───────────────────────────────────────────────────────────
get_cpu_cores() {
    nproc 2>/dev/null || awk '/^processor/{n++} END{print n}' /proc/cpuinfo 2>/dev/null || echo "1"
}

# ─── Load average ─────────────────────────────────────────────────────────────
get_load_avg() {
    awk '{print $1}' /proc/loadavg 2>/dev/null || echo "0"
}
