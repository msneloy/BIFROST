package server

import (
	"net/http"
)

func (s *Server) handleSingleFrame(w http.ResponseWriter, r *http.Request) {
	if guardCheck(r, w, s.tracker) {
		return
	}

	frame := s.capture.MuxBuffer().LatestVideo()
	if frame == nil {
		http.Error(w, "no frame available", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Write(frame)
}
