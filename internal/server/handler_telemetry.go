package server

import (
	"net/http"
)

func (s *Server) handlePing(w http.ResponseWriter, r *http.Request) {
	ip := extractIP(r)
	q := r.URL.Query()

	s.tracker.UpdateTelemetry(ip,
		q.Get("latency"),
		q.Get("os"),
		q.Get("browser"),
		q.Get("resolution"),
		q.Get("device"),
		q.Get("gpu"),
		q.Get("battery"),
	)

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRejected(w http.ResponseWriter, r *http.Request) {
	ip := extractIP(r)
	os := r.URL.Query().Get("os")
	reason := r.URL.Query().Get("reason")
	ua := r.UserAgent()

	s.tracker.LogRejection(ip, os, reason, ua)

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusNoContent)
}
