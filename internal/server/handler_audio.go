package server

import (
	"log"
	"net/http"
)

func (s *Server) handleAudioStream(w http.ResponseWriter, r *http.Request) {
	if guardCheck(r, w, s.tracker) {
		return
	}

	ip := extractIP(r)
	log.Printf("[+] Audio client connected: %s", ip)

	w.Header().Set("Content-Type", "audio/mpeg")
	w.Header().Set("Cache-Control", "no-cache, private")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	sub := s.capture.AudioBroadcaster().Subscribe()
	defer s.capture.AudioBroadcaster().Unsubscribe(sub)

	for {
		select {
		case <-r.Context().Done():
			log.Printf("[-] Audio client disconnected: %s", ip)
			return
		case chunk, ok := <-sub.C:
			if !ok {
				return
			}
			if _, err := w.Write(chunk); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}
