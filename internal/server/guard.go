package server

import (
	"net/http"
	"strings"

	"github.com/nelobster/bifrost/internal/tracker"
)

const deniedPage = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>ACCESS DENIED - BIFROST</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{background:#0a0000;color:#ff3333;font-family:'Courier New',monospace;display:flex;justify-content:center;align-items:center;height:100vh;text-align:center}
.container{border:2px solid #ff3333;padding:40px 60px;border-radius:8px;background:rgba(170,0,0,0.1)}
.icon{font-size:64px;margin-bottom:20px}
h1{font-size:28px;margin-bottom:15px;letter-spacing:3px}
p{color:#999;font-size:14px;line-height:1.6}
</style>
</head>
<body>
<div class="container">
<div class="icon">&#9762;</div>
<h1>ACCESS DENIED</h1>
<p>BIFROST does not support Windows clients.</p>
<p>Please use a Linux or Android device.</p>
</div>
</body>
</html>`

func guardCheck(r *http.Request, w http.ResponseWriter, trk *tracker.Tracker) bool {
	ua := strings.ToLower(r.UserAgent())
	if strings.Contains(ua, "windows") {
		ip := extractIP(r)
		trk.LogRejection(ip, "Windows", "Windows client rejected", r.UserAgent())
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(deniedPage))
		return true
	}
	return false
}

func extractIP(r *http.Request) string {
	// Try X-Forwarded-For first
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	// Try X-Real-IP
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	// Fall back to RemoteAddr
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		return addr[:idx]
	}
	return addr
}
