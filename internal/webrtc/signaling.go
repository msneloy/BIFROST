package webrtc

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/pion/webrtc/v4"
)

// SignalingServer handles SDP offer/answer exchange for WebRTC.
type SignalingServer struct {
	sfu *SFU
}

// NewSignalingServer creates a new signaling server.
func NewSignalingServer(sfu *SFU) *SignalingServer {
	return &SignalingServer{sfu: sfu}
}

type offerRequest struct {
	SDP string `json:"sdp"`
}

type answerResponse struct {
	SDP string `json:"sdp"`
}

// RegisterRoutes adds WebRTC signaling endpoints to the HTTP mux.
func (ss *SignalingServer) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/webrtc/offer", ss.handleOffer)
}

func (ss *SignalingServer) handleOffer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req offerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	offer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  req.SDP,
	}

	answer, err := ss.sfu.HandleOffer(offer)
	if err != nil {
		log.Printf("Failed to handle offer: %v", err)
		http.Error(w, fmt.Sprintf("Failed to create answer: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(answerResponse{SDP: answer.SDP})
}
