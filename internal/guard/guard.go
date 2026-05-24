package guard

import (
	"bifrost/internal/tracker"
	"net/http"
	"strings"
)

const windowsRejectionPage = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>BIFROST - Access Denied</title>
    <style>
        body {
            background-color: #0a0a0a;
            color: #888;
            font-family: monospace;
            display: flex;
            flex-direction: column;
            align-items: center;
            justify-content: center;
            height: 100vh;
            margin: 0;
            text-align: center;
        }
        .emoji {
            font-size: 5rem;
            margin-bottom: 20px;
        }
        h1 {
            color: #ff2222;
            font-size: 3rem;
            margin: 0 0 10px 0;
        }
        p {
            font-size: 1.2rem;
            margin: 5px 0;
        }
        .error-code {
            font-size: 0.9rem;
            margin-top: 30px;
            color: #555;
        }
    </style>
</head>
<body>
    <div class="emoji">🪟</div>
    <h1>ACCESS DENIED</h1>
    <p>BIFROST does not support Windows.</p>
    <p>Please use a Linux or Android device.</p>
    <div class="error-code">Error 403 &mdash; Unsupported Operating System</div>
</body>
</html>`

func RejectWindows(tracker *tracker.Tracker, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ua := strings.ToLower(r.Header.Get("User-Agent"))
		if strings.Contains(ua, "windows") {
			ip := r.RemoteAddr
			if idx := strings.LastIndex(ip, ":"); idx != -1 {
				ip = ip[:idx]
			}
			tracker.LogRejection(ip, "Windows", "Windows UA detected", ua)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(windowsRejectionPage))
			return
		}
		next(w, r)
	}
}
