#!/usr/bin/env bash
# mdns.sh — mDNS service registration via avahi-publish

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

MDNS_HOSTNAME="${BIFROST_MDNS_HOSTNAME:-bifrost}"

# ─── Find avahi-publish ───────────────────────────────────────────────────────
find_avahi_publish() {
    local candidates=(
        "avahi-publish"
        "$BIFROST_DIR/vendor/bin/avahi-publish"
        "/opt/bifrost/bin/avahi-publish"
        "/usr/bin/avahi-publish"
    )
    for candidate in "${candidates[@]}"; do
        if command -v "$candidate" &>/dev/null || [[ -x "$candidate" ]]; then
            echo "$candidate"
            return 0
        fi
    done
    return 1
}

# ─── Register mDNS hostname ───────────────────────────────────────────────────
mdns_register() {
    local ip="$1"
    local hostname="${2:-$MDNS_HOSTNAME}"

    local avahi_bin
    avahi_bin=$(find_avahi_publish) || {
        log_warn "avahi-publish not found — mDNS disabled"
        return 0
    }

    log_info "mDNS: registering ${hostname}.local → ${ip}"

    "$avahi_bin" -a -R "${hostname}.local" "$ip" &
    local pid=$!
    save_pid "mdns" "$pid"

    # Verify it started
    sleep 0.5
    if kill -0 "$pid" 2>/dev/null; then
        log "mDNS: ${hostname}.local registered"
    else
        log_warn "mDNS: failed to register (is avahi-daemon running?)"
    fi
}

# ─── Unregister mDNS ──────────────────────────────────────────────────────────
mdns_unregister() {
    kill_pid "mdns"
}
