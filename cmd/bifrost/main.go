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
	"bifrost/internal/dashboard"
	"bifrost/internal/mdns"
	"bifrost/internal/server"
	"bifrost/internal/stream"
	"bifrost/internal/tracker"
)

const (
	AppName    = "BIFROST"
	AppVersion = "0.1.0"
	Port       = 8080
)

func main() {
	fmt.Print("\033[H\033[2J")
	fmt.Println(`
  ██████╗ ██╗███████╗██████╗  ██████╗ ███████╗████████╗
  ██╔══██╗██║██╔════╝██╔══██╗██╔═══██╗██╔════╝╚══██╔══╝
  ██████╔╝██║█████╗  ██████╔╝██║   ██║███████╗   ██║
  ██╔══██╗██║██╔══╝  ██╔══██╗██║   ██║╚════██║   ██║
  ██████╔╝██║██║     ██║  ██║╚██████╔╝███████║   ██║
  v0.1.0  | Classroom Screen Broadcasting`)

	cleanupOrphans()

	localIP := getLocalIP()

	// Initialize components
	tr := tracker.New()
	broadcaster := stream.NewBroadcaster()
	broadcaster.SetHeader(nil)
	capturer := capture.NewCapturer(15, 40)

	// Start mDNS
	stopMDNS := mdns.Register(localIP)

	// Start Capture (video + audio, single pipeline for sync)
	if err := capturer.Start(broadcaster); err != nil {
		log.Fatalf("Capture failed: %v", err)
	}

	// Start HTTP streaming server
	srv := server.New(tr, broadcaster)
	srv.Addr = fmt.Sprintf(":%d", Port)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v\n\n[TIP] Port %d is already in use.", err, Port)
		}
	}()

	// Start dev watcher for auto-rebuild
	if _, err := exec.LookPath("go"); err == nil {
		startWatcher()
	}

	// Launch terminal dashboard (blocks — runs until SIGINT/SIGTERM)
	fmt.Print("\033[?25l")
	go func() {
		time.Sleep(500 * time.Millisecond)
		dashboard.Start(tr, broadcaster, localIP, AppVersion)
	}()

	// Wait for shutdown
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs

	fmt.Print("\033[?25h")
	fmt.Println("\nShutting down BIFROST...")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	srv.Shutdown(ctx)
	capturer.Stop()
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
