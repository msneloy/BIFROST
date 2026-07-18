package server

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/pion/webrtc/v4"
)

func (s *Server) handleWebRTCOffer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.webrtcMgr == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "WebRTC not enabled"})
		return
	}

	var req struct {
		SDP string `json:"sdp"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	ip := extractIP(r)
	log.Printf("[WebRTC] Received offer from %s", ip)

	answer, err := s.webrtcMgr.CreatePeerFromOffer(ip, req.SDP)
	if err != nil {
		log.Printf("[WebRTC] Failed to create peer: %v", err)
		http.Error(w, "failed to create peer connection", http.StatusInternalServerError)
		return
	}

	log.Printf("[WebRTC] Sent answer to %s", ip)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"sdp": answer})
}

func (s *Server) handleICECandidate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.webrtcMgr == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	var req struct {
		Candidate     string `json:"candidate"`
		SDPMLineIndex *uint16 `json:"sdpMLineIndex"`
		SDPMid        *string `json:"sdpMid"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	candidate := webrtc.ICECandidateInit{
		Candidate:     req.Candidate,
		SDPMLineIndex: req.SDPMLineIndex,
		SDPMid:        req.SDPMid,
	}

	// The peerID should be tracked per-connection; for now use IP as key
	ip := extractIP(r)
	if err := s.webrtcMgr.AddICECandidate(ip, candidate); err != nil {
		log.Printf("[WebRTC] ICE candidate error: %v", err)
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleICEPoll(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if s.webrtcMgr == nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"candidates": []interface{}{}})
		return
	}

	ip := extractIP(r)
	candidates := s.webrtcMgr.PendingICE(ip)

	if candidates == nil {
		candidates = make([]webrtc.ICECandidateInit, 0)
	}

	json.NewEncoder(w).Encode(map[string]interface{}{"candidates": candidates})
	_ = time.Now() // ensure time package is referenced
}
