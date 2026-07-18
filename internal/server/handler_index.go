package server

import (
	"net/http"
)

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if guardCheck(r, w, s.tracker) {
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Write(s.playerHTML)
}
