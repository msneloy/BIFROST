package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"bifrost/internal/capture"
	"bifrost/internal/gui"
	"bifrost/internal/mdns"
	"bifrost/internal/player"
	"bifrost/internal/server"
	"bifrost/internal/stream"
	"bifrost/internal/tracker"
	bifrostwebrtc "bifrost/internal/webrtc"
)

const (
	AppName    = "BIFROST"
	AppVersion = "0.1.0"
	Port       = 8080
)

func main() {
	headless := false
	for _, arg := range os.Args[1:] {
		if arg == "--headless" || arg == "-headless" {
			headless = true
		}
	}

	if !headless {
		fmt.Print("\033[H\033[2J")
	}

	fmt.Println(`
  ██████╗ ██╗███████╗██████╗  ██████╗ ███████╗████████╗
  ██╔══██╗██║██╔════╝██╔══██╗██╔═══██╗██╔════╝╚══██╔══╝
  ██████╔╝██║█████╗  ██████╔╝██║   ██║███████╗   ██║
  ██╔══██╗██║██╔══╝  ██╔══██╗██║   ██║╚════██║   ██║
  ██████╔╝██║██║     ██║  ██║╚██████╔╝███████║   ██║
  v0.1.0  | Classroom Screen Broadcasting`)

	cleanupOrphans()

	localIP := getLocalIP()
	fmt.Printf("Primary URL:   http://bifrost.local:%d\n", Port)
	fmt.Printf("Direct IP URL: http://%s:%d\n", localIP, Port)
	fmt.Printf("Student page:  http://%s:%d/watch\n", localIP, Port)

	// Initialize components
	tr := tracker.New()
	broadcaster := stream.NewBroadcaster()
	broadcaster.SetHeader(nil)
	capturer := capture.NewCapturer(15, 40)

	// Start mDNS
	stopMDNS := mdns.Register(localIP)

	// Start WebRTC SFU
	sfu := bifrostwebrtc.NewSFU()
	signalingServer := bifrostwebrtc.NewSignalingServer(sfu)

	// Start RTP relay
	relay := bifrostwebrtc.NewRelay(sfu, 5004, 5005)
	if err := relay.Start(); err != nil {
		log.Printf("RTP relay warning: %v (WebRTC streaming disabled)", err)
	}

	sfu.OnPeerJoin(func(id string) {
		log.Printf("WebRTC peer connected: %s (total: %d)", id, sfu.PeerCount())
	})
	sfu.OnPeerLeave(func(id string) {
		log.Printf("WebRTC peer disconnected: %s (total: %d)", id, sfu.PeerCount())
	})

	// Start Capture
	if err := capturer.Start(broadcaster); err != nil {
		log.Fatalf("Capture failed: %v", err)
	}

	// Start HTTP server in background
	srv := server.New(tr, broadcaster, signalingServer, player.HTML)
	srv.Addr = fmt.Sprintf(":%d", Port)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v\n\n[TIP] Port %d is already in use.", err, Port)
		}
	}()

	// Start dev watcher
	if _, err := exec.LookPath("go"); err == nil {
		startWatcher()
	}

	// Setup signal handler for graceful shutdown
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		// Signal Fyne to quit, then cleanup happens after gui.Run returns
		fmt.Print("\033[?25h")
		fmt.Println("\nShutting down BIFROST...")
	}()

	// GUI runs on main goroutine (Fyne requirement) — blocks until quit
	gui.Run(tr, broadcaster, capturer, localIP, AppVersion, headless)

	// Cleanup after GUI exits
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	srv.Shutdown(ctx)
	capturer.Stop()
	relay.Stop()
	sfu.Close()
	for _, stop := range stopMDNS {
		stop()
	}

	fmt.Println("BIFROST shutdown complete.")
}

func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}
	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return "127.0.0.1"
}

func cleanupOrphans() {
	for _, p := range []string{"ffmpeg", "gst-launch-1.0", "avahi-publish"} {
		_ = exec.Command("pkill", "-9", p).Run()
	}
}

func startWatcher() {
	cmd := exec.Command("go", "build", "-o", "bifrost", "./cmd/bifrost")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}
