package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/nelobster/bifrost/internal/capture"
	"github.com/nelobster/bifrost/internal/config"
	"github.com/nelobster/bifrost/internal/tracker"
	bifrostwebrtc "github.com/nelobster/bifrost/internal/webrtc"
	"github.com/pion/webrtc/v4"
)

type Server struct {
	config     *config.Config
	httpSrv    *http.Server
	capture    *capture.Capture
	tracker    *tracker.Tracker
	playerHTML []byte
	webrtcMgr  *bifrostwebrtc.Manager
	ctx        context.Context
}

func New(ctx context.Context, cfg *config.Config, cap *capture.Capture, trk *tracker.Tracker, playerHTML []byte, webrtcMgr *bifrostwebrtc.Manager) *Server {
	return &Server{
		config:     cfg,
		capture:    cap,
		tracker:    trk,
		playerHTML: playerHTML,
		webrtcMgr:  webrtcMgr,
		ctx:        ctx,
	}
}

func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/ping", s.handlePing)
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/webrtc/offer", s.handleWebRTCOffer)
	mux.HandleFunc("/ice", s.handleICECandidate)
	mux.HandleFunc("/ice/poll", s.handleICEPoll)

	s.httpSrv = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.config.Port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 0,
	}

	log.Printf("[+] HTTP server listening on :%d", s.config.Port)

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.httpSrv.Shutdown(shutdownCtx)
	}()

	if err := s.httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	if s.httpSrv != nil {
		return s.httpSrv.Shutdown(ctx)
	}
	return nil
}

// ─── Handlers ──────────────────────────────────────────────────

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Write(s.playerHTML)
}

func (s *Server) handlePing(w http.ResponseWriter, r *http.Request) {
	ip := extractIP(r)
	q := r.URL.Query()
	s.tracker.UpdateTelemetry(ip,
		q.Get("latency"), q.Get("os"), q.Get("browser"),
		q.Get("resolution"), q.Get("device"),
	)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"streaming": s.capture.IsStreaming(),
		"clients":   s.tracker.CountActive(),
	})
}

// ─── WebRTC signaling ──────────────────────────────────────────

func (s *Server) handleWebRTCOffer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.webrtcMgr == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "WebRTC not available"})
		return
	}

	var req struct{ SDP string }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	ip := extractIP(r)
	log.Printf("[WebRTC] Offer from %s", ip)

	answer, err := s.webrtcMgr.CreatePeerFromOffer(ip, req.SDP)
	if err != nil {
		log.Printf("[WebRTC] Failed: %v", err)
		http.Error(w, "peer connection failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"sdp": answer})
}

func (s *Server) handleICECandidate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.webrtcMgr == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	var req struct {
		Candidate     string  `json:"candidate"`
		SDPMLineIndex *uint16 `json:"sdpMLineIndex"`
		SDPMid        *string `json:"sdpMid"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	ip := extractIP(r)
	s.webrtcMgr.AddICECandidate(ip, webrtc.ICECandidateInit{
		Candidate:     req.Candidate,
		SDPMLineIndex: req.SDPMLineIndex,
		SDPMid:        req.SDPMid,
	})
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
}

// ─── Helpers ───────────────────────────────────────────────────

func extractIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.TrimSpace(strings.Split(xff, ",")[0])
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		return addr[:idx]
	}
	return addr
}
