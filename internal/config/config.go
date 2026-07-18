package config

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

const Version = "0.3.0"

type Config struct {
	Port       int
	FPS        int
	Quality    int
	Resolution string
	Headless   bool
	NoAudio    bool
	NoWebRTC   bool

	// Derived
	LocalIP string
	Width   int
	Height  int
}

func Parse() *Config {
	cfg := &Config{}

	flag.IntVar(&cfg.Port, "port", envInt("BIFROST_PORT", 8080), "HTTP server port")
	flag.IntVar(&cfg.FPS, "fps", envInt("BIFROST_FPS", 30), "Capture frame rate")
	flag.IntVar(&cfg.Quality, "quality", envInt("BIFROST_QUALITY", 40), "JPEG quality 1-100")
	flag.StringVar(&cfg.Resolution, "resolution", envStr("BIFROST_RESOLUTION", "1920x1080"), "Capture resolution WxH")
	flag.BoolVar(&cfg.Headless, "headless", false, "Skip TUI dashboard")
	flag.BoolVar(&cfg.NoAudio, "no-audio", false, "Disable audio capture")
	flag.BoolVar(&cfg.NoWebRTC, "no-webrtc", false, "Disable WebRTC (MJPEG only)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "BIFROST v%s — Browser Integrated Feed for Remote Observation & Screen Transmission\n\n", Version)
		fmt.Fprintf(os.Stderr, "Usage: bifrost [OPTIONS]\n\nOptions:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nDependencies:\n  Required: ffmpeg\n  Optional: avahi-daemon + avahi-utils (mDNS), python3 + GStreamer (Wayland)\n")
	}

	flag.Parse()

	// Parse resolution into width/height
	w, h := parseResolution(cfg.Resolution)
	cfg.Width = w
	cfg.Height = h

	return cfg
}

func (c *Config) DetectLocalIP() string {
	conn, err := net.Dial("udp", "1.1.1.1:53")
	if err != nil {
		return "127.0.0.1"
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}

func (c *Config) CheckDeps() {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		fmt.Fprintln(os.Stderr, "ERROR: ffmpeg is required but not found in PATH")
		fmt.Fprintln(os.Stderr, "  Install: apt install ffmpeg")
		os.Exit(1)
	}
	if _, err := exec.LookPath("avahi-publish"); err != nil {
		fmt.Println("[!] avahi-publish not found — mDNS discovery disabled (install avahi-utils)")
	}
}

func parseResolution(res string) (int, int) {
	parts := strings.SplitN(res, "x", 2)
	if len(parts) != 2 {
		return 1920, 1080
	}
	w, _ := strconv.Atoi(parts[0])
	h, _ := strconv.Atoi(parts[1])
	if w <= 0 || h <= 0 {
		return 1920, 1080
	}
	return w, h
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
