package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/nelobster/bifrost/internal/capture"
	"github.com/nelobster/bifrost/internal/config"
	"github.com/nelobster/bifrost/internal/mdns"
	"github.com/nelobster/bifrost/internal/server"
	"github.com/nelobster/bifrost/internal/tracker"
	"github.com/nelobster/bifrost/internal/tui"
	bifrostwebrtc "github.com/nelobster/bifrost/internal/webrtc"
)

func main() {
	// Parse config
	cfg := config.Parse()

	// Detect local IP
	cfg.LocalIP = cfg.DetectLocalIP()

	// Check dependencies
	cfg.CheckDeps()

	// Set up signal handling
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Initialize subsystems
	cap := capture.New(cfg)
	trk := tracker.New()

	// Start WebRTC RTP receiver
	rtpRecv, err := bifrostwebrtc.NewRTPReceiver(5004, 5005)
	if err != nil {
		log.Fatalf("[!] Failed to start RTP receiver: %v", err)
	}
	rtpRecv.Start(ctx)
	webrtcMgr := bifrostwebrtc.NewManager()
	webrtcMgr.SetVideoTrack(rtpRecv.VideoTrack())
	webrtcMgr.SetAudioTrack(rtpRecv.AudioTrack())
	log.Println("[+] WebRTC initialized")

	// Start capture
	if err := cap.Start(ctx); err != nil {
		log.Fatalf("[!] Failed to start capture: %v", err)
	}

	// Start HTTP server
	srv := server.New(ctx, cfg, cap, trk, playerHTML, webrtcMgr)
	go func() {
		if err := srv.Start(ctx); err != nil {
			log.Printf("[!] HTTP server error: %v", err)
		}
	}()

	// Start mDNS
	mdnsCleanup := mdns.Register(ctx, cfg.LocalIP, "bifrost")
	defer mdnsCleanup()

	// Start client pruning goroutine
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				trk.Prune(30 * time.Second)
			}
		}
	}()

	// Build capture controller adapter for TUI
	captureCtrl := &captureAdapter{cap: cap, ctx: ctx}

	if cfg.Headless {
		log.Printf("[+] BIFROST v%s running in headless mode", config.Version)
		log.Printf("[+] Students connect to: http://bifrost.local:%d", cfg.Port)
		log.Printf("[+] Resolution: %s | FPS: %d | Audio: %v", cfg.Resolution, cfg.FPS, !cfg.NoAudio)
		<-ctx.Done()
	} else {
		// TUI mode
		p := tea.NewProgram(
			tui.New(cfg, captureCtrl, trk),
			tea.WithAltScreen(),
		)

		if _, err := p.Run(); err != nil {
			log.Fatalf("[!] TUI error: %v", err)
		}
	}

	// Graceful shutdown
	log.Println("[*] Shutting down...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	srv.Stop(shutdownCtx)
	cap.Stop()
	log.Println("[+] BIFROST stopped.")
}

// captureAdapter wraps capture.Capture to satisfy the tui.CaptureController interface.
type captureAdapter struct {
	cap *capture.Capture
	ctx context.Context
}

func (a *captureAdapter) IsStreaming() bool { return a.cap.IsStreaming() }
func (a *captureAdapter) Start() error      { return a.cap.Start(a.ctx) }
func (a *captureAdapter) Stop()             { a.cap.Stop() }
