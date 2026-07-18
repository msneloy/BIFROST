package main

import (
	"context"
	"fmt"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/nelobster/bifrost/internal/capture"
	"github.com/nelobster/bifrost/internal/config"
	bifrostgui "github.com/nelobster/bifrost/internal/gui"
	"github.com/nelobster/bifrost/internal/mdns"
	"github.com/nelobster/bifrost/internal/server"
	"github.com/nelobster/bifrost/internal/tracker"
	bifrostwebrtc "github.com/nelobster/bifrost/internal/webrtc"
)

func main() {
	// Parse config
	cfg := config.Parse()

	// Detect local IP
	cfg.LocalIP = cfg.DetectLocalIP()

	// Check dependencies
	cfg.CheckDeps()

	// Show banner
	showBanner(cfg)

	// Set up signal handling
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Initialize subsystems
	cap := capture.New(cfg)
	trk := tracker.New()
	var webrtcMgr *bifrostwebrtc.Manager

	// Start WebRTC RTP receiver if enabled
	if !cfg.NoWebRTC {
		rtpRecv, err := bifrostwebrtc.NewRTPReceiver(5004, 5005)
		if err != nil {
			log.Printf("[!] Failed to create RTP receiver: %v (WebRTC disabled)", err)
		} else {
			rtpRecv.Start(ctx)
			webrtcMgr = bifrostwebrtc.NewManager()
			webrtcMgr.SetVideoTrack(rtpRecv.VideoTrack())
			webrtcMgr.SetAudioTrack(rtpRecv.AudioTrack())
			log.Println("[+] WebRTC manager initialized")
		}
	}

	// Start capture
	if err := cap.Start(ctx); err != nil {
		log.Fatalf("[!] Failed to start capture: %v", err)
	}

	// Start HTTP server
	srv := server.New(cfg, cap, trk, playerHTML, adminHTML, webrtcMgr)
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

	// GUI or headless mode
	if !cfg.Headless {
		// Run native Fyne GUI (blocks until window closes)
		log.Println("[*] Launching GUI...")
		go func() {
			<-ctx.Done()
			bifrostgui.Quit()
		}()
		bifrostgui.Run(cfg, cap, trk)
	} else {
		log.Println("[+] Running in headless mode. Press Ctrl+C to stop.")
		<-ctx.Done()
	}

	// Graceful shutdown
	log.Println("[*] Shutting down...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	srv.Stop(shutdownCtx)
	cap.Stop()
	log.Println("[+] BIFROST stopped.")
}

func showBanner(cfg *config.Config) {
	fmt.Printf(`
  ╔══════════════════════════════════════════════════════════════╗
  ║                                                              ║
  ║   ██████╗ ██╗████████╗██████╗ ████████╗                     ║
  ║   ██╔══██╗██║╚══██╔══╝██╔══██╗╚══██╔══╝                     ║
  ║   ██████╔╝██║   ██║   ██████╔╝   ██║                        ║
  ║   ██╔══██╗██║   ██║   ██╔══██╗   ██║                        ║
  ║   ██████╔╝██║   ██║   ██████╔╝   ██║                        ║
  ║   ╚═════╝ ╚═╝   ╚═╝   ╚═════╝    ╚═╝                        ║
  ║                                                              ║
  ║   Browser Integrated Feed for Remote Observation             ║
  ║              & Screen Transmission                           ║
  ║                                                              ║
  ║   v%s (Go)                                              ║
  ║                                                              ║
  ╚══════════════════════════════════════════════════════════════╝

  Stream URL:    http://%s:%d
  Stream URL:    http://%s:%d/watch
  Admin Panel:   http://%s:%d/admin
  Frame URL:     http://%s:%d/frame
  Audio URL:     http://%s:%d/audio
  Health:        http://%s:%d/health
  Stats:         http://%s:%d/stats

  Resolution:    %s
  FPS:           %d
  Quality:       %d
  WebRTC:        %v
  Audio:         %v

`, config.Version,
		cfg.LocalIP, cfg.Port,
		cfg.LocalIP, cfg.Port,
		cfg.LocalIP, cfg.Port,
		cfg.LocalIP, cfg.Port,
		cfg.LocalIP, cfg.Port,
		cfg.LocalIP, cfg.Port,
		cfg.LocalIP, cfg.Port,
		cfg.Resolution, cfg.FPS, cfg.Quality,
		!cfg.NoWebRTC, !cfg.NoAudio,
	)
}
