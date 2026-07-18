package server

import (
	"fmt"
	"log"
	"net/http"
)

func (s *Server) handleMJPEGStream(w http.ResponseWriter, r *http.Request) {
	if guardCheck(r, w, s.tracker) {
		return
	}

	ip := extractIP(r)
	s.tracker.GetOrCreate(ip)

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary=bifrost")
	w.Header().Set("Cache-Control", "no-cache, private")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	sub := s.capture.Broadcaster().Subscribe()
	defer s.capture.Broadcaster().Unsubscribe(sub)

	log.Printf("[+] MJPEG client connected: %s", ip)

	for {
		select {
		case <-r.Context().Done():
			log.Printf("[-] MJPEG client disconnected: %s", ip)
			return
		case frame, ok := <-sub.C:
			if !ok {
				return
			}
			header := fmt.Sprintf("--bifrost\r\nContent-Type: image/jpeg\r\nContent-Length: %d\r\n\r\n", len(frame))
			if _, err := w.Write([]byte(header)); err != nil {
				return
			}
			if _, err := w.Write(frame); err != nil {
				return
			}
			if _, err := w.Write([]byte("\r\n")); err != nil {
				return
			}
			flusher.Flush()
			s.tracker.AddBytes(ip, uint64(len(frame)+len(header)+2))
		}
	}
}
