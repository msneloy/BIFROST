package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/nelobster/bifrost/internal/capture"
	"github.com/nelobster/bifrost/internal/config"
	"github.com/nelobster/bifrost/internal/mdns"
	"github.com/nelobster/bifrost/internal/server"
	"github.com/nelobster/bifrost/internal/tracker"
	bifrostwebrtc "github.com/nelobster/bifrost/internal/webrtc"
)

// killProcessUsingPort tries to find processes listening on the given TCP port
// and sends them SIGTERM, waiting briefly and escalating to SIGKILL if needed.
func killProcessUsingPort(port int) error {
	// Try lsof first (prints PIDs)
	pidList := []int{}
	if out, err := exec.Command("lsof", "-ti", fmt.Sprintf("tcp:%d", port)).Output(); err == nil {
		for _, f := range strings.Fields(string(out)) {
			if p, err := strconv.Atoi(strings.TrimSpace(f)); err == nil {
				pidList = append(pidList, p)
			}
		}
	} else {
		// Fallback: parse `ss -ltnp` output for pid= entries
		if out, err := exec.Command("ss", "-ltnp").Output(); err == nil {
			re := regexp.MustCompile(`pid=(\d+),`)
			for _, line := range strings.Split(string(out), "\n") {
				if strings.Contains(line, fmt.Sprintf(":"+"%d", port)) {
					for _, m := range re.FindAllStringSubmatch(line, -1) {
						if len(m) > 1 {
							if p, err := strconv.Atoi(m[1]); err == nil {
								pidList = append(pidList, p)
							}
						}
					}
				}
			}
		}
	}

	// Deduplicate
	seen := map[int]bool{}
	final := []int{}
	for _, p := range pidList {
		if p == os.Getpid() {
			continue
		}
		if !seen[p] {
			seen[p] = true
			final = append(final, p)
		}
	}
	if len(final) == 0 {
		return nil
	}

	for _, pid := range final {
		log.Printf("[*] Killing process %d listening on port %d", pid, port)
		_ = syscall.Kill(pid, syscall.SIGTERM)
		// wait up to 3s for exit
		waited := 0
		for waited < 30 {
			time.Sleep(100 * time.Millisecond)
			waited++
			if syscall.Kill(pid, 0) != nil {
				break
			}
		}
		if syscall.Kill(pid, 0) == nil {
			log.Printf("[!] Process %d did not exit; sending SIGKILL", pid)
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	}
	return nil
}

func main() {
	// Parse config
	cfg := config.Parse()

	// Detect local IP
	cfg.LocalIP = cfg.DetectLocalIP()

	// Check dependencies
	cfg.CheckDeps()

	// Show banner
	showBanner(cfg)

	// Ensure single instance: check PID file and kill previous instance if running
	pidFile := filepath.Join(os.TempDir(), "bifrost.pid")
	if data, err := os.ReadFile(pidFile); err == nil {
		if prevPid, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil {
			if prevPid > 0 && prevPid != os.Getpid() {
				if err := syscall.Kill(prevPid, 0); err == nil {
					log.Printf("[*] Found previous BIFROST pid %d — terminating it", prevPid)
					_ = syscall.Kill(prevPid, syscall.SIGTERM)
					// wait briefly for shutdown
					waited := 0
					for waited < 30 {
						time.Sleep(100 * time.Millisecond)
						waited++
						if syscall.Kill(prevPid, 0) != nil {
							break
						}
					}
					// force kill if still alive
					if syscall.Kill(prevPid, 0) == nil {
						log.Printf("[!] Previous process %d did not exit; sending SIGKILL", prevPid)
						_ = syscall.Kill(prevPid, syscall.SIGKILL)
					}
				}
			}
		}
	}
	// write current pid
	_ = os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0644)
	defer os.Remove(pidFile)

	// Set up signal handling (include SIGTSTP to ensure port released on Ctrl+Z)
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGTSTP)
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
	// Ensure the configured HTTP port is free by killing any process using it
	if err := killProcessUsingPort(cfg.Port); err != nil {
		log.Printf("[!] Failed to clear port %d: %v", cfg.Port, err)
	}
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

	// Launch terminal UI
	go runTUI(ctx, cancel, cfg, cap, trk)

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

	Admin TUI:     run in this terminal (interactive)
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

// runTUI provides a minimal terminal UI for controlling and viewing status.
func runTUI(ctx context.Context, stop func(), cfg *config.Config, cap *capture.Capture, trk *tracker.Tracker) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("--- BIFROST Admin TUI ---")
	fmt.Println("Commands: (s)tatus, (t)oggle stream, (q)uit")

	for {
		fmt.Print("bifrost> ")
		line, err := reader.ReadString('\n')
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				time.Sleep(100 * time.Millisecond)
				continue
			}
		}
		cmd := strings.TrimSpace(line)
		switch cmd {
		case "s", "status":
			fmt.Printf("Streaming: %v\n", cap.IsStreaming())
			fmt.Printf("Active clients: %d\n", trk.CountActive())
		case "t", "toggle":
			if cap.IsStreaming() {
				cap.Stop()
				fmt.Println("Stopped streaming")
			} else {
				if err := cap.Start(ctx); err != nil {
					fmt.Printf("Failed to start capture: %v\n", err)
				} else {
					fmt.Println("Started streaming")
				}
			}
		case "q", "quit", "exit":
			fmt.Println("Quitting...")
			stop()
			return
		default:
			fmt.Println("Unknown command. Use: s, t, q")
		}
	}
}
