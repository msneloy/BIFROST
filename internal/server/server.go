package server

import (
	"bifrost/internal/stream"
	"bifrost/internal/tracker"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var startTime = time.Now()

func getIP(r *http.Request) string {
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return ip
}

func New(
	tr *tracker.Tracker,
	stream *stream.Broadcaster,
	viewerHTML string,
) *http.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/", recoverMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(viewerHTML))
	}))

	mux.HandleFunc("/frame", recoverMiddleware(func(w http.ResponseWriter, r *http.Request) {
		ip := getIP(r)

		frame := stream.GetLastFrame()
		if frame == nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Content-Length", strconv.Itoa(len(frame)))
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

		n, err := w.Write(frame)
		if err == nil && n > 0 {
			tr.AddBytes(ip, int64(n))
		}
	}))

	mux.HandleFunc("/stream", recoverMiddleware(func(w http.ResponseWriter, r *http.Request) {
		ip := getIP(r)
		w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary=boundary")
		w.Header().Set("Cache-Control", "no-cache, private")
		w.Header().Set("Pragma", "no-cache")

		ch := stream.Subscribe(100)
		defer stream.Unsubscribe(ch)

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}

		for {
			select {
			case <-r.Context().Done():
				return
			case chunk, ok := <-ch:
				if !ok {
					return
				}
				header := fmt.Sprintf("--boundary\r\nContent-Type: image/jpeg\r\nContent-Length: %d\r\n\r\n", len(chunk))
				n1, _ := fmt.Fprint(w, header)
				n2, _ := w.Write(chunk)
				n3, _ := fmt.Fprint(w, "\r\n")

				tr.AddBytes(ip, int64(n1+n2+n3))
				flusher.Flush()
			}
		}
	}))

	// Audio endpoint is now redundant as it's muxed in /stream
	mux.HandleFunc("/audio", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusGone)
	})

	mux.HandleFunc("/ping", recoverMiddleware(func(w http.ResponseWriter, r *http.Request) {
		ip := getIP(r)
		client := tr.GetClient(ip)

		q := r.URL.Query()
		if lat, err := strconv.Atoi(q.Get("latency")); err == nil {
			client.Latency = lat
		}
		if os := q.Get("os"); os != "" {
			client.OS = os
		}
		if browser := q.Get("browser"); browser != "" {
			client.Browser = browser
		}
		if res := q.Get("resolution"); res != "" {
			client.Resolution = res
		}
		if dev := q.Get("device"); dev != "" {
			client.DevType = dev
		}
		if gpu := q.Get("gpu"); gpu != "" {
			client.GPU = gpu
		}
		if bat, err := strconv.Atoi(q.Get("battery")); err == nil {
			client.BatPct = bat
		}
		if charging := q.Get("charging"); charging != "" {
			client.Charging = (charging == "true")
		}

		w.WriteHeader(http.StatusOK)
	}))

	mux.HandleFunc("/rejected", recoverMiddleware(func(w http.ResponseWriter, r *http.Request) {
		ip := getIP(r)
		os := r.URL.Query().Get("os")
		reason := r.URL.Query().Get("reason")
		ua := r.Header.Get("User-Agent")
		tr.LogRejection(ip, os, reason, ua)
		w.WriteHeader(http.StatusOK)
	}))

	mux.HandleFunc("/push", recoverMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil || len(body) == 0 {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		// Signal capture to stop if it's running
		stream.SetHeader([]byte("BRIDGE"))

		// Publish the frame received from the browser
		stream.Publish(body)
		w.WriteHeader(http.StatusOK)
	}))

	mux.HandleFunc("/stats", recoverMiddleware(func(w http.ResponseWriter, r *http.Request) {
		tr.RLock()
		defer tr.RUnlock()

		activeClients := make([]tracker.ClientInfo, 0)
		for _, c := range tr.Clients {
			if c.Active {
				activeClients = append(activeClients, *c)
			}
		}

		res := map[string]interface{}{
			"total_transmitted": tr.TotalBytes,
			"pub_total":         stream.Total,
			"pub_rate":          stream.GetPubRate(),
			"clients":           activeClients,
			"rejections":        tr.Rejections,
			"uptime":            time.Since(startTime).String(),
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-cache")
		json.NewEncoder(w).Encode(res)
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
func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}
	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return "127.0.0.1"
}
