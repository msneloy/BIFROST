package capture

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"

	"github.com/nelobster/bifrost/internal/config"
)

type Capture struct {
	config     *config.Config
	videoCmd   *exec.Cmd
	audioCmd   *exec.Cmd
	waylandCmd *exec.Cmd
	splitter   *FrameSplitter
	muxBuffer  *MuxBuffer
	streaming  bool
	mu         sync.Mutex
}

func New(cfg *config.Config) *Capture {
	return &Capture{
		config:    cfg,
		muxBuffer: NewMuxBuffer(30),
	}
}

func (c *Capture) MuxBuffer() *MuxBuffer {
	return c.muxBuffer
}

func (c *Capture) IsStreaming() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.streaming
}

func (c *Capture) Start(ctx context.Context) error {
	sessionType := detectSessionType()

	if sessionType == "wayland" {
		return c.startWayland(ctx)
	}
	return c.startX11Unified(ctx)
}

func (c *Capture) startX11Unified(ctx context.Context) error {
	display := os.Getenv("DISPLAY")
	if display == "" {
		display = ":0"
	}

	args := []string{
		"-y", "-loglevel", "warning",
		"-f", "x11grab", "-draw_mouse", "1",
		"-video_size", c.config.Resolution,
		"-framerate", fmt.Sprintf("%d", c.config.FPS),
		"-i", display,
	}

	audioSource := ""
	if !c.config.NoAudio {
		audioSource = detectPulseSource()
	}
	if audioSource != "" {
		args = append(args, "-f", "pulse", "-i", audioSource)
	}

	// MJPEG to stdout (for HTTP /stream)
	args = append(args,
		"-map", "0:v",
		"-f", "mjpeg", "-q:v", fmt.Sprintf("%d", c.config.Quality),
		"pipe:1",
	)

	// VP8 RTP for WebRTC video
	if !c.config.NoWebRTC {
		args = append(args,
			"-map", "0:v",
			"-c:v", "libvpx", "-b:v", "2M",
			"-deadline", "realtime", "-cpu-used", "4",
			"-f", "rtp", "udp://127.0.0.1:5004",
		)
	}

	// Opus RTP for WebRTC audio
	if audioSource != "" && !c.config.NoWebRTC {
		args = append(args,
			"-map", "1:a",
			"-c:a", "libopus", "-b:a", "64k",
			"-f", "rtp", "udp://127.0.0.1:5005",
		)
	}

	// MP3 to fd 3 for HTTP audio
	if audioSource != "" {
		args = append(args,
			"-map", "1:a",
			"-c:a", "libmp3lame", "-b:a", "128k",
			"-f", "mp3", "pipe:3",
		)
	}

	c.videoCmd = exec.CommandContext(ctx, "ffmpeg", args...)

	stdout, err := c.videoCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	var audioReader *os.File
	if audioSource != "" {
		audioR, audioW, pipeErr := os.Pipe()
		if pipeErr != nil {
			return fmt.Errorf("failed to create audio pipe: %w", pipeErr)
		}
		audioReader = audioR
		c.videoCmd.ExtraFiles = []*os.File{audioW}
	}

	c.videoCmd.Stderr = os.Stderr

	if err := c.videoCmd.Start(); err != nil {
		return fmt.Errorf("failed to start unified capture: %w", err)
	}

	// Frame splitter reads MJPEG from stdout, publishes to MuxBuffer
	c.splitter = NewFrameSplitter(stdout, c.muxBuffer)
	go func() {
		if err := c.splitter.Run(ctx); err != nil && err != context.Canceled {
			log.Printf("[!] Frame splitter error: %v", err)
		}
	}()

	// Audio reader reads MP3 from fd 3, publishes to MuxBuffer
	if audioReader != nil {
		go func() {
			defer audioReader.Close()
			buf := make([]byte, 8*1024) // 8KB chunks for lower latency
			for {
				n, err := audioReader.Read(buf)
				if n > 0 {
					chunk := make([]byte, n)
					copy(chunk, buf[:n])
					c.muxBuffer.PublishAudio(chunk)
				}
				if err != nil {
					break
				}
			}
		}()
	}

	log.Printf("[+] Unified capture started (X11: %s, %s @ %dfps, quality %d, audio=%v, webrtc=%v)",
		display, c.config.Resolution, c.config.FPS, c.config.Quality,
		audioSource != "", !c.config.NoWebRTC)

	c.mu.Lock()
	c.streaming = true
	c.mu.Unlock()

	return nil
}

func (c *Capture) startWayland(ctx context.Context) error {
	scripts := []string{"mutter_capture.py", "portal_capture.py", "capture.py"}
	exe, _ := os.Executable()

	for _, script := range scripts {
		scriptPath := findScript(exe, script)
		if scriptPath == "" {
			continue
		}

		cmd := exec.CommandContext(ctx, "python3", scriptPath)
		if c.config.NoWebRTC {
			cmd.Args = append(cmd.Args, "--no-webrtc")
		}
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			continue
		}
		cmd.Stderr = os.Stderr

		if err := cmd.Start(); err != nil {
			log.Printf("[!] Failed to start %s: %v", script, err)
			continue
		}

		c.waylandCmd = cmd
		c.splitter = NewFrameSplitter(stdout, c.muxBuffer)
		go func() {
			if err := c.splitter.Run(ctx); err != nil && err != context.Canceled {
				log.Printf("[!] Frame splitter error: %v", err)
			}
		}()

		log.Printf("[+] Wayland capture started via %s", script)

		if !c.config.NoAudio {
			if source := detectPulseSource(); source != "" {
				c.startAudioCapture(ctx, source)
			} else {
				log.Println("[!] No PulseAudio source found — audio capture disabled")
			}
		}

		c.mu.Lock()
		c.streaming = true
		c.mu.Unlock()

		return nil
	}

	return fmt.Errorf("no Wayland capture script found (tried: %v)", scripts)
}

func (c *Capture) startAudioCapture(ctx context.Context, source string) {
	audioArgs := []string{
		"-y", "-loglevel", "warning",
		"-f", "pulse", "-i", source,
		"-c:a", "libmp3lame", "-b:a", "128k",
		"-f", "mp3", "pipe:1",
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", audioArgs...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Printf("[!] Failed to create audio pipe: %v", err)
		return
	}
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		log.Printf("[!] Failed to start audio capture: %v", err)
		return
	}

	c.audioCmd = cmd

	go func() {
		buf := make([]byte, 8*1024) // 8KB chunks for lower latency
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				chunk := make([]byte, n)
				copy(chunk, buf[:n])
				c.muxBuffer.PublishAudio(chunk)
			}
			if err != nil {
				break
			}
		}
	}()

	log.Printf("[+] Audio capture started for HTTP clients (source: %s)", source)
}

func (c *Capture) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.videoCmd != nil && c.videoCmd.Process != nil {
		c.videoCmd.Process.Kill()
		c.videoCmd.Wait()
	}
	if c.audioCmd != nil && c.audioCmd.Process != nil {
		c.audioCmd.Process.Kill()
		c.audioCmd.Wait()
	}
	if c.waylandCmd != nil && c.waylandCmd.Process != nil {
		c.waylandCmd.Process.Kill()
		c.waylandCmd.Wait()
	}
	c.streaming = false
	log.Println("[+] Capture stopped")
}

func detectSessionType() string {
	if v := os.Getenv("XDG_SESSION_TYPE"); v != "" {
		return v
	}
	return "x11"
}

func detectPulseSource() string {
	if runtime.GOOS != "linux" {
		return ""
	}
	out, err := exec.Command("pactl", "list", "short", "sources").Output()
	if err != nil {
		return ""
	}
	lines := strings.Split(string(out), "\n")
	var fallback string
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name := fields[1]
		if strings.Contains(name, "monitor") {
			if strings.Contains(line, "RUNNING") {
				return name
			}
			if fallback == "" {
				fallback = name
			}
		}
	}
	return fallback
}

func findScript(exePath, scriptName string) string {
	dir := exePath
	for i := 0; i < 5; i++ {
		idx := strings.LastIndex(dir, "/")
		if idx < 0 {
			break
		}
		dir = dir[:idx]
		candidate := dir + "/scripts/" + scriptName
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	if _, err := os.Stat("scripts/" + scriptName); err == nil {
		return "scripts/" + scriptName
	}
	return ""
}
