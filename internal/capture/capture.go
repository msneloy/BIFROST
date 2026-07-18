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
	config      *config.Config
	videoCmd    *exec.Cmd
	audioCmd    *exec.Cmd
	waylandCmd  *exec.Cmd
	splitter    *FrameSplitter
	broadcaster *Broadcaster
	audioBcast  *AudioBroadcaster
	streaming   bool
	mu          sync.Mutex
}

func New(cfg *config.Config) *Capture {
	rb := NewRingBuffer(60)
	return &Capture{
		config:      cfg,
		broadcaster: NewBroadcaster(rb),
		audioBcast:  NewAudioBroadcaster(),
	}
}

func (c *Capture) Broadcaster() *Broadcaster {
	return c.broadcaster
}

func (c *Capture) AudioBroadcaster() *AudioBroadcaster {
	return c.audioBcast
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

	// Build single ffmpeg command with multiple outputs:
	//   stdout (fd 1) = MJPEG for HTTP /stream
	//   UDP 5004      = VP8 RTP for WebRTC video
	//   UDP 5005      = Opus RTP for WebRTC audio
	//   fd 3          = MP3 for HTTP /audio
	args := []string{
		"-y", "-loglevel", "warning",
		// Video input
		"-f", "x11grab", "-draw_mouse", "1",
		"-video_size", c.config.Resolution,
		"-framerate", fmt.Sprintf("%d", c.config.FPS),
		"-i", display,
	}

	// Audio input (if available and enabled)
	audioSource := ""
	if !c.config.NoAudio {
		audioSource = detectPulseSource()
	}
	if audioSource != "" {
		args = append(args,
			"-f", "pulse", "-i", audioSource,
		)
	}

	// Output 1: MJPEG to stdout (for HTTP /stream clients)
	args = append(args,
		"-map", "0:v",
		"-f", "mjpeg", "-q:v", fmt.Sprintf("%d", c.config.Quality),
		"pipe:1",
	)

	// Output 2: VP8 RTP to UDP 5004 (for WebRTC video)
	if !c.config.NoWebRTC {
		args = append(args,
			"-map", "0:v",
			"-c:v", "libvpx", "-b:v", "2M",
			"-deadline", "realtime", "-cpu-used", "4",
			"-f", "rtp", "udp://127.0.0.1:5004",
		)
	}

	// Output 3: Opus RTP to UDP 5005 (for WebRTC audio)
	if audioSource != "" && !c.config.NoWebRTC {
		args = append(args,
			"-map", "1:a",
			"-c:a", "libopus", "-b:a", "64k",
			"-f", "rtp", "udp://127.0.0.1:5005",
		)
	}

	// Output 4: MP3 to fd 3 (for HTTP /audio clients)
	if audioSource != "" {
		args = append(args,
			"-map", "1:a",
			"-c:a", "libmp3lame", "-b:a", "128k",
			"-f", "mp3", "pipe:3",
		)
	}

	c.videoCmd = exec.CommandContext(ctx, "ffmpeg", args...)

	// Set up stdout pipe (MJPEG)
	stdout, err := c.videoCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// Set up fd 3 pipe (MP3 audio) via ExtraFiles
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

	// Close the write end of the audio pipe in the parent process
	if audioReader != nil {
		// The write end is duplicated into ffmpeg's fd 3 by ExtraFiles.
		// We need to close our reference to the write end after Start().
		// But ExtraFiles[0] IS the write end — Go duplicates it for the child.
		// We can't close it directly here since it's in ExtraFiles.
		// The child process has its own copy, so we just read from audioReader.
	}

	// Start frame splitter for MJPEG (reads stdout)
	c.splitter = NewFrameSplitter(stdout, c.broadcaster)
	go func() {
		if err := c.splitter.Run(ctx); err != nil && err != context.Canceled {
			log.Printf("[!] Frame splitter error: %v", err)
		}
	}()

	// Start audio reader for MP3 (reads fd 3)
	if audioReader != nil {
		go func() {
			defer audioReader.Close()
			buf := make([]byte, 32*1024)
			for {
				n, err := audioReader.Read(buf)
				if n > 0 {
					chunk := make([]byte, n)
					copy(chunk, buf[:n])
					c.audioBcast.Publish(chunk)
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
		c.splitter = NewFrameSplitter(stdout, c.broadcaster)
		go func() {
			if err := c.splitter.Run(ctx); err != nil && err != context.Canceled {
				log.Printf("[!] Frame splitter error: %v", err)
			}
		}()

		log.Printf("[+] Wayland capture started via %s", script)

		// Start separate audio capture for HTTP /audio clients
		// (mutter_capture.py only outputs Opus RTP for WebRTC, not MP3 for HTTP)
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
		buf := make([]byte, 32*1024)
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				chunk := make([]byte, n)
				copy(chunk, buf[:n])
				c.audioBcast.Publish(chunk)
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
