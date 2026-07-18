package server

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"streaming": s.capture.IsStreaming(),
		"clients":   s.tracker.CountActive(),
	})
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, s.tracker.StatsJSON())
}
