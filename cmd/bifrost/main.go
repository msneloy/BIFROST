package main

import (
	"bifrost/internal/capture"
	"bifrost/internal/dashboard"
	"bifrost/internal/mdns"
	"bifrost/internal/server"
	"bifrost/internal/stream"
	"bifrost/internal/tracker"
	"bifrost/web"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

// Build:      go build -o bifrost ./cmd/bifrost
// Run:        ./bifrost
// Package:    dpkg-deb --build bifrost-package bifrost_0.1.0_amd64.deb
// Install:    sudo dpkg -i bifrost_0.1.0_amd64.deb
// Runtime deps: ffmpeg, avahi-daemon, avahi-utils, pipewire

const (
	AppName          = "BIFROST"
	AppVersion       = "0.1.0"
	StreamPort       = 8080
	StreamFPS        = 30
	JPEGQuality      = 80
	MaxClientRows    = 20
	MaxRejectedRows  = 5
	ClientTimeout    = 30 * time.Second
	DashboardRefresh = 1 * time.Second

	MDNSNamePrimary = "bifrost" // → bifrost.local
)

func getLocalIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1"
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}

func main() {
	// 1. ASCII Art & Startup Info
	dashboard.ClearScreen()
	fmt.Println("\033[38;5;196m")
	fmt.Println(`  ██████╗ ██╗███████╗██████╗  ██████╗ ███████╗████████╗`)
	fmt.Println(`  ██╔══██╗██║██╔════╝██╔══██╗██╔═══██╗██╔════╝╚══██╔══╝`)
	fmt.Println(`  ██████╔╝██║█████╗  ██████╔╝██║   ██║███████╗   ██║   `)
	fmt.Println(`  ██╔══██╗██║██╔══╝  ██╔══██╗██║   ██║╚════██║   ██║   `)
	fmt.Println(`  ██████╔╝██║██║     ██║  ██║╚██████╔╝███████║   ██║   `)
	fmt.Println("\033[0m")

	localIP := getLocalIP()
	primaryURL := fmt.Sprintf("http://%s.local:%d", MDNSNamePrimary, StreamPort)
	directIPURL := fmt.Sprintf("http://%s:%d", localIP, StreamPort)

	fmt.Printf("Starting %s v%s...\n", AppName, AppVersion)
	fmt.Printf("Primary URL:   %s\n", primaryURL)
	fmt.Printf("Direct IP URL: %s\n", directIPURL)

	// 2. Kill orphan processes
	exec.Command("pkill", "-9", "ffmpeg").Run()
	exec.Command("pkill", "-9", "gst-launch-1.0").Run()

	// Detect session type
	sessionType := os.Getenv("XDG_SESSION_TYPE")
	fmt.Printf("Session type: %s\n", sessionType)

	// Audio setup
	monitor := capture.DetectPulseAudioMonitor()
	if monitor != "" {
		fmt.Printf("Audio source: %s\n", monitor)
	} else {
		fmt.Println("Warning: PulseAudio monitor not found. Audio disabled.")
	}

	// Wait a moment for the user to read startup info
	time.Sleep(2 * time.Second)

	// Initialize components
	tr := tracker.New()
	videoStream := stream.NewBroadcaster()
	audioStream := stream.NewBroadcaster()

	// 6. mDNS
	cleanupMDNS := mdns.Register(MDNSNamePrimary, "", localIP)
	if cleanupMDNS == nil {
		log.Println("mDNS registration failed")
	}

	// 7. Screen capture
	capturer := capture.NewCapturer(StreamFPS)
	if err := capturer.Start(videoStream); err != nil {
		log.Fatalf("Failed to start screen capture: %v", err)
	}
	defer capturer.Stop()

	// 8. Audio capture
	if monitor != "" {
		audioCmd := capture.StartAudioBroadcaster(audioStream)
		if audioCmd != nil {
			defer audioCmd.Process.Kill()
		}
	}

	// 9. HTTP Server
	srv := server.New(tr, videoStream, audioStream, web.ViewerHTML)
	srv.Addr = fmt.Sprintf(":%d", StreamPort)

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			if strings.Contains(err.Error(), "permission denied") {
				log.Fatalf("Server error: %v\n\n[TIP] Port 80 is a privileged port on Linux. Please run BIFROST with sudo:\n      sudo ./bifrost\n\n", err)
			}
			if strings.Contains(err.Error(), "address already in use") {
				log.Fatalf("Server error: %v\n\n[TIP] Port %d is already in use. Stop the existing BIFROST process or use watch.sh to restart cleanly.\n", err, StreamPort)
			}
			log.Fatalf("Server error: %v", err)
		}
	}()

	// 10. Background tracker pruning
	go func() {
		for {
			time.Sleep(5 * time.Second)
			tr.Prune(ClientTimeout)
		}
	}()

	// 11. Dashboard renderer
	dashboard.ClearScreen()
	go dashboard.Render(tr, primaryURL, "", directIPURL, AppVersion)

	// Wait for SIGINT/SIGTERM
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs

	// Graceful Shutdown
	fmt.Print("\033[?25h") // Restore cursor
	fmt.Println("\nShutting down BIFROST...")

	capturer.Stop()
	exec.Command("pkill", "-9", "ffmpeg").Run()

	for _, cleanup := range cleanupMDNS {
		cleanup()
	}

	fmt.Println("BIFROST shutdown complete")
	os.Exit(0)
}
