package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/nelobster/bifrost/internal/capture"
	"github.com/nelobster/bifrost/internal/config"
	"github.com/nelobster/bifrost/internal/tracker"
	bifrostwebrtc "github.com/nelobster/bifrost/internal/webrtc"
)

type Server struct {
	config     *config.Config
	httpSrv    *http.Server
	capture    *capture.Capture
	tracker    *tracker.Tracker
	playerHTML []byte
	adminHTML  []byte
	webrtcMgr  *bifrostwebrtc.Manager
}

func New(cfg *config.Config, cap *capture.Capture, trk *tracker.Tracker, playerHTML []byte, adminHTML []byte, webrtcMgr *bifrostwebrtc.Manager) *Server {
	return &Server{
		config:     cfg,
		capture:    cap,
		tracker:    trk,
		playerHTML: playerHTML,
		adminHTML:  adminHTML,
		webrtcMgr:  webrtcMgr,
	}
}

func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	// Static / HTML
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/watch", s.handleIndex)

	// Streaming
	mux.HandleFunc("/stream", s.handleStream)
	mux.HandleFunc("/frame", s.handleSingleFrame)

	// Telemetry
	mux.HandleFunc("/ping", s.handlePing)
	mux.HandleFunc("/rejected", s.handleRejected)

	// Health / Stats
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/stats", s.handleStats)

	// WebRTC signaling
	mux.HandleFunc("/webrtc/offer", s.handleWebRTCOffer)
	mux.HandleFunc("/ice", s.handleICECandidate)
	mux.HandleFunc("/ice/poll", s.handleICEPoll)

	// Admin dashboard
	mux.HandleFunc("/admin", s.handleAdmin)
	mux.HandleFunc("/api/clients", s.handleAPIClients)
	mux.HandleFunc("/api/stats", s.handleAPIStats)

	s.httpSrv = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.config.Port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 0, // No timeout for streaming
	}

	log.Printf("[+] HTTP server listening on :%d", s.config.Port)

	// Wait for context cancellation
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
