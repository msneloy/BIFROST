#!/usr/bin/env bash
# guard.sh — Windows client rejection middleware

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"
source "$SCRIPT_DIR/tracker.sh"

DENIED_PAGE='<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>BIFROST - ACCESS DENIED</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{background:#0a0000;color:#fff;font-family:"Courier New",monospace;height:100vh;display:flex;align-items:center;justify-content:center;flex-direction:column}
.deny-box{border:2px solid #ff3333;padding:40px 60px;text-align:center;background:rgba(170,0,0,0.1)}
h1{color:#ff3333;font-size:48px;margin-bottom:20px;letter-spacing:8px}
p{color:#888;font-size:16px;margin-top:10px}
.icon{font-size:64px;margin-bottom:20px}
</style>
</head>
<body>
<div class="deny-box">
  <div class="icon">&#x1F6AB;</div>
  <h1>ACCESS DENIED</h1>
  <p>BIFROST does not support Windows clients.</p>
  <p>Please use a Linux or Android device.</p>
</div>
</body>
</html>'

# ─── Check if client should be rejected ───────────────────────────────────────
# Returns 0 if rejected (caller should send denied page and return),
# 1 if allowed (caller should proceed)
guard_check() {
    local client_ip="$1" user_agent="$2"

    # Case-insensitive check for "windows" in User-Agent
    local ua_lower="${user_agent,,}"
    if [[ "$ua_lower" == *"windows"* ]]; then
        tracker_log_rejection "$client_ip" "Windows" "Windows client rejected" "$user_agent"

        # Send the 403 denied page
        local body_len=${#DENIED_PAGE}
        printf "HTTP/1.1 403 Forbidden\r\n"
        printf "Content-Type: text/html; charset=utf-8\r\n"
        printf "Content-Length: %d\r\n" "$body_len"
        printf "Connection: close\r\n"
        printf "\r\n"
        printf "%s" "$DENIED_PAGE"

        log_warn "Rejected Windows client: $client_ip"
        return 0  # Rejected
    fi

    return 1  # Allowed
}
