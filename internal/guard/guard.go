package guard

import (
	"bifrost/internal/tracker"
	"fmt"
	"net/http"
	"strings"
)

const windowsRejectionHTML = `
<!DOCTYPE html>
<html>
<head>
    <title>ACCESS DENIED</title>
    <style>
        body {
            background: #0a0a0a;
            color: #ff2222;
            font-family: 'Courier New', Courier, monospace;
            display: flex;
            flex-direction: column;
            align-items: center;
            justify-content: center;
            height: 100vh;
            margin: 0;
            text-align: center;
        }
        .icon { font-size: 80px; margin-bottom: 20px; }
        .title { font-size: 28px; font-weight: bold; margin-bottom: 10px; }
        .msg { font-size: 14px; color: #888; margin-bottom: 30px; line-height: 1.5; }
        .error { font-size: 11px; color: #333; border: 1px solid #222; padding: 10px 20px; }
    </style>
</head>
<body>
    <div class="icon">🪟</div>
    <div class="title">ACCESS DENIED</div>
    <div class="msg">
        BIFROST does not support Windows.<br/>
        Please use a Linux or Android device.
    </div>
    <div class="error">Error 403 &mdash; Unsupported Operating System</div>
</body>
</html>
`

// RejectWindows is HTTP middleware that blocks clients with "windows" in
// their User-Agent header. It returns a styled 403 Forbidden page and
// logs the rejection to the tracker.
func RejectWindows(tr *tracker.Tracker, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ua := strings.ToLower(r.Header.Get("User-Agent"))
		if strings.Contains(ua, "windows") {
			tr.LogRejection(r.RemoteAddr, "Windows OS", "server_guard", ua)
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprint(w, windowsRejectionHTML)
			return
		}
		next(w, r)
	}
}
