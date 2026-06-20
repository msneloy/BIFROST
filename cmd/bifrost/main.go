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
	"bifrost/internal/mdns"
	"bifrost/internal/server"
	"bifrost/internal/stream"
	"bifrost/internal/tracker"
	"bifrost/internal/watcher"
	"bifrost/web"
)

const (
	AppName    = "BIFROST"
	AppVersion = "0.1.0"
	Port       = 8080
)

func main() {
	// 0. Print Banner
	fmt.Print("\033[H\033[2J") // Clear screen
	fmt.Println(`
  ██████╗ ██╗███████╗██████╗  ██████╗ ███████╗████████╗
  ██╔══██╗██║██╔════╝██╔══██╗██╔═══██╗██╔════╝╚══██╔══╝
  ██████╔╝██║█████╗  ██████╔╝██║   ██║███████╗   ██║   
  ██╔══██╗██║██╔══╝  ██╔══██╗██║   ██║╚════██║   ██║   
  ██████╔╝██║██║     ██║  ██║╚██████╔╝███████║   ██║   
  v0.1.0  | Classroom Screen Broadcasting`)

	// 1. Cleanup orphan processes
	cleanupOrphans()

	// 2. Detect LAN IP
	localIP := getLocalIP()
	fmt.Printf("Primary URL:   http://bifrost.local:%d\n", Port)
	fmt.Printf("Direct IP URL: http://%s:%d\n", localIP, Port)

	// 3. Initialize components
	tr := tracker.New()
	broadcaster := stream.NewBroadcaster()
	broadcaster.SetHeader(nil) // Ensure no stale metadata for MJPEG session
	capturer := capture.NewCapturer(15, 40)

	// 4. Start mDNS
	stopMDNS := mdns.Register(localIP)

	// 5. Start Capture
	if err := capturer.Start(broadcaster); err != nil {
		log.Fatalf("Capture start failed: %v", err)
	}

	// 6. Init HTTP Server
	srv := server.New(tr, broadcaster, web.ViewerHTML)
	srv.Addr = fmt.Sprintf(":%d", Port)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v\n\n[TIP] Port %d is already in use.", err, Port)
		}
	}()

	// 7. Start Watcher (if in dev)
	if _, err := exec.LookPath("go"); err == nil {
		watcher.Start([]string{"cmd", "internal", "web"}, []string{"go", "build", "-o", "bifrost", "./cmd/bifrost"})
	}

	// Wait for shutdown
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs

	fmt.Print("\033[?25h") // Restore cursor
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
	processes := []string{"ffmpeg", "gst-launch-1.0", "avahi-publish"}
	for _, p := range processes {
		_ = exec.Command("pkill", "-9", p).Run()
	}
}
