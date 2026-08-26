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
	Resolution string
	NoAudio    bool
	Headless   bool

	// Derived
	LocalIP string
	Width   int
	Height  int
}

func Parse() *Config {
	cfg := &Config{}

	flag.IntVar(&cfg.Port, "port", envInt("BIFROST_PORT", 8080), "HTTP server port")
	flag.IntVar(&cfg.FPS, "fps", envInt("BIFROST_FPS", 30), "Capture frame rate")
	flag.StringVar(&cfg.Resolution, "resolution", envStr("BIFROST_RESOLUTION", "1920x1080"), "Capture resolution WxH")
	flag.BoolVar(&cfg.NoAudio, "no-audio", false, "Disable audio capture")
	flag.BoolVar(&cfg.Headless, "headless", false, "Run without TUI (for scripts/CI)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "BIFROST v%s — Browser Integrated Feed for Remote Observation & Screen Transmission\n\n", Version)
		fmt.Fprintf(os.Stderr, "Usage: bifrost [OPTIONS]\n\nOptions:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nDependencies:\n  Required: gstreamer1.0-tools + plugins\n")
		fmt.Fprintf(os.Stderr, "\nThe TUI starts automatically. Press 'q' to quit, 's' to start/stop stream.\n")
	}

	flag.Parse()

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
	return conn.LocalAddr().(*net.UDPAddr).IP.String()
}

func (c *Config) CheckDeps() {
	if _, err := exec.LookPath("gst-launch-1.0"); err != nil {
		fmt.Fprintln(os.Stderr, "ERROR: GStreamer is required but not found in PATH")
		fmt.Fprintln(os.Stderr, "  Install: sudo apt install gstreamer1.0-tools gstreamer1.0-plugins-base gstreamer1.0-plugins-good")
		os.Exit(1)
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
