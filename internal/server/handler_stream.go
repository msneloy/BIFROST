package server

import (
	"log"
	"net/http"

	"github.com/nelobster/bifrost/internal/capture"
)

func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
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

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	sub := s.capture.MuxBuffer().Subscribe()
	defer s.capture.MuxBuffer().Unsubscribe(sub)

	log.Printf("[+] Stream client connected: %s", ip)

	for {
		select {
		case <-r.Context().Done():
			log.Printf("[-] Stream client disconnected: %s", ip)
			return
		case data, ok := <-sub.C:
			if !ok {
				return
			}
			if _, err := w.Write(data); err != nil {
				return
			}
			// Flush only on video frames — audio piggybacks on the next flush.
			if data[0] == capture.EntryTypeVideo {
				flusher.Flush()
			}
			s.tracker.AddBytes(ip, uint64(len(data)))
		}
	}
}
