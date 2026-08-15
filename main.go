package main

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/nelobster/bifrost/internal/capture"
	"github.com/nelobster/bifrost/internal/config"
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
	srv := server.New(ctx, cfg, cap, trk, playerHTML, adminHTML, webrtcMgr)
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

	// Open admin panel in browser (unless --no-browser flag is set)
	if !cfg.NoBrowser {
		adminURL := fmt.Sprintf("http://localhost:%d/admin", cfg.Port)
		log.Printf("[*] Opening admin panel: %s", adminURL)
		// Small delay to let the HTTP server bind before opening the browser
		time.AfterFunc(500*time.Millisecond, func() { openBrowser(adminURL) })
	} else {
		log.Printf("[+] Running headless. Admin panel: http://%s:%d/admin", cfg.LocalIP, cfg.Port)
	}

	// Wait for shutdown signal
	<-ctx.Done()

	// Graceful shutdown
	log.Println("[*] Shutting down...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	srv.Stop(shutdownCtx)
	cap.Stop()
	log.Println("[+] BIFROST stopped.")
}

// openBrowser opens the given URL in the system's default browser.
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return
	}
	if err := cmd.Start(); err != nil {
		log.Printf("[!] Could not open browser: %v", err)
	}
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

  Admin Panel:   http://%s:%d/admin
  Student URL:   http://bifrost.local:%d/watch (mDNS)
  Student IP:    http://%s:%d/watch
  Health:        http://%s:%d/health

  Resolution:    %s
  FPS:           %d
  Quality:       %d
  WebRTC:        %v
  Audio:         %v

`, config.Version,
		cfg.LocalIP, cfg.Port,
		cfg.Port,
		cfg.LocalIP, cfg.Port,
		cfg.LocalIP, cfg.Port,
		cfg.Resolution, cfg.FPS, cfg.Quality,
		!cfg.NoWebRTC, !cfg.NoAudio,
	)
}
