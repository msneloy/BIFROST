package server

import (
	"bifrost/internal/guard"
	"bifrost/internal/stream"
	"bifrost/internal/tracker"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"log"
)

type trackingResponseWriter struct {
	http.ResponseWriter
	tracker *tracker.Tracker
	ip      string
}

func (w *trackingResponseWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	if n > 0 {
		w.tracker.AddBytes(w.ip, int64(n))
	}
	return n, err
}

func (w *trackingResponseWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func getIP(r *http.Request) string {
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return ip
}

func New(
	tr *tracker.Tracker,
	videoStream *stream.Broadcaster,
	audioStream *stream.Broadcaster,
	viewerHTML string,
) *http.Server {
	mux := http.NewServeMux()

	trackMiddleware := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			ip := getIP(r)
			tw := &trackingResponseWriter{
				ResponseWriter: w,
				tracker:        tr,
				ip:             ip,
			}
			next(tw, r)
		}
	}

	mux.HandleFunc("/", recoverMiddleware(guard.RejectWindows(tr, trackMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(viewerHTML))
	}))))

	mux.HandleFunc("/stream", recoverMiddleware(guard.RejectWindows(tr, trackMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary=frame")
		w.Header().Set("Cache-Control", "no-cache, private")
		w.Header().Set("Pragma", "no-cache")
		
		ch := videoStream.Subscribe(2)
		defer videoStream.Unsubscribe(ch)

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}

		for {
			select {
			case <-r.Context().Done():
				return
			case frame, ok := <-ch:
				if !ok {
					return
				}
				header := fmt.Sprintf("--frame\r\nContent-Type: image/jpeg\r\nContent-Length: %d\r\n\r\n", len(frame))
				if _, err := w.Write([]byte(header)); err != nil {
					log.Println("write header error:", err)
					return
				}
				if _, err := w.Write(frame); err != nil {
					log.Println("write frame error:", err)
					return
				}
				if _, err := w.Write([]byte("\r\n")); err != nil {
					log.Println("write newline error:", err)
					return
				}
				flusher.Flush()
			}
		}
	}))))

	mux.HandleFunc("/audio", recoverMiddleware(guard.RejectWindows(tr, trackMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/mpeg")
		w.Header().Set("Transfer-Encoding", "chunked")
		w.Header().Set("Cache-Control", "no-cache")

		ch := audioStream.Subscribe(50)
		defer audioStream.Unsubscribe(ch)

		flusher, ok := w.(http.Flusher)
		
		for {
			select {
			case <-r.Context().Done():
				return
			case chunk := <-ch:
				w.Write(chunk)
				if ok {
					flusher.Flush()
				}
			}
		}
	}))))

	mux.HandleFunc("/ping", recoverMiddleware(trackMiddleware(func(w http.ResponseWriter, r *http.Request) {
		ip := getIP(r)
		client := tr.GetClient(ip)
		
		q := r.URL.Query()
		if lat, err := strconv.Atoi(q.Get("latency")); err == nil {
			client.Latency = lat
		}
		if os := q.Get("os"); os != "" { client.OS = os }
		if browser := q.Get("browser"); browser != "" { client.Browser = browser }
		if res := q.Get("resolution"); res != "" { client.Resolution = res }
		if dev := q.Get("device"); dev != "" { client.DevType = dev }
		if gpu := q.Get("gpu"); gpu != "" { client.GPU = gpu }
		if bat, err := strconv.Atoi(q.Get("battery")); err == nil { client.BatPct = bat }
		if charging := q.Get("charging"); charging != "" { client.Charging = (charging == "true") }
		
		w.WriteHeader(http.StatusOK)
	})))

	mux.HandleFunc("/rejected", recoverMiddleware(func(w http.ResponseWriter, r *http.Request) {
		ip := getIP(r)
		os := r.URL.Query().Get("os")
		reason := r.URL.Query().Get("reason")
		ua := r.Header.Get("User-Agent")
		tr.LogRejection(ip, os, reason, ua)
		w.WriteHeader(http.StatusOK)
	}))

	mux.HandleFunc("/health", recoverMiddleware(func(w http.ResponseWriter, r *http.Request) {
		tr.RLock()
		activeClients := 0
		for _, c := range tr.Clients {
			if c.Active {
				activeClients++
			}
		}
		tr.RUnlock()
		
		res := map[string]interface{}{
			"streaming": true,
			"clients":   activeClients,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(res)
	}))

	return &http.Server{
		Handler: mux,
		ConnState: func(conn net.Conn, state http.ConnState) {
			if state == http.StateNew {
				if tc, ok := conn.(*net.TCPConn); ok {
					tc.SetNoDelay(true)
				}
			}
		},
	}
}

// recoverMiddleware wraps an http.HandlerFunc to recover from panics and log them.
func recoverMiddleware(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        defer func() {
            if err := recover(); err != nil {
                log.Printf("panic recovered in handler: %v", err)
                http.Error(w, "Internal Server Error", http.StatusInternalServerError)
            }
        }()
        next(w, r)
    }
}
