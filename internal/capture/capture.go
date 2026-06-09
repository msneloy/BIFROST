package capture

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"bifrost/internal/stream"
)

type Capturer struct {
	fps     int
	quality int
	cmd     *exec.Cmd
	done    chan struct{}
}

func NewCapturer(fps int, quality int) *Capturer {
	return &Capturer{
		fps:     fps,
		quality: quality,
		done:    make(chan struct{}),
	}
}

// Start begins screen capture.
// Under Wayland/COSMIC: uses cosmic-screenshot for per-frame capture.
// Under X11: uses ffmpeg x11grab for efficient MJPEG pipe capture.
// Streams MJPEG frames to the provided broadcaster.
func (c *Capturer) Start(broadcaster *stream.Broadcaster) error {
	sessionType := os.Getenv("XDG_SESSION_TYPE")
	waylandDisplay := os.Getenv("WAYLAND_DISPLAY")

	// Under Wayland, use cosmic-screenshot which works with the native display server
	if sessionType == "wayland" || waylandDisplay != "" {
		if _, err := exec.LookPath("cosmic-screenshot"); err == nil {
			log.Println("Wayland session detected: Using cosmic-screenshot capture")
			return c.startWaylandCapture(broadcaster)
		}
		log.Println("WARN: Wayland session but no cosmic-screenshot found. Trying X11 grab.")
	}

	display := os.Getenv("DISPLAY")
	if display == "" {
		display = ":0"
	}
	log.Printf("Capturing display: %s", display)

	qscale := (100 - c.quality) * 30 / 100
	if qscale < 1 {
		qscale = 1
	} else if qscale > 31 {
		qscale = 31
	}

	cmd := exec.Command("ffmpeg",
		"-fflags", "nobuffer",
		"-flags", "low_delay",
		"-f", "x11grab",
		"-framerate", strconv.Itoa(c.fps),
		"-i", display,
		"-q:v", strconv.Itoa(qscale),
		"-f", "image2pipe",
		"-vcodec", "mjpeg",
		"pipe:1",
	)
	log.Printf("ffmpeg cmd (qscale: %d): %v", qscale, cmd.Args)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("ffmpeg stderr pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("ffmpeg stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("ffmpeg start: %w", err)
	}
	c.cmd = cmd

	// Log ffmpeg stderr
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			log.Println("[ffmpeg]", scanner.Text())
		}
	}()

	// Parse MJPEG frames from stdout and publish them
	go func() {
		defer cmd.Wait()
		frameCount := 0
		totalBytes := int64(0)
		scanner := bufio.NewScanner(stdout)
		buf := make([]byte, 1024*1024)
		scanner.Buffer(buf, 20*1024*1024)
		scanner.Split(func(data []byte, atEOF bool) (int, []byte, error) {
			if atEOF && len(data) == 0 {
				return 0, nil, nil
			}
			start := bytes.Index(data, []byte{0xFF, 0xD8})
			if start == -1 {
				return len(data), nil, nil
			}
			end := bytes.Index(data[start:], []byte{0xFF, 0xD9})
			if end == -1 {
				return start, nil, nil
			}
			end += start + 2
			frame := make([]byte, end-start)
			copy(frame, data[start:end])
			return end, frame, nil
		})
		for scanner.Scan() {
			frame := scanner.Bytes()
			if frameCount == 0 {
				log.Printf("First frame: %d bytes", len(frame))
			}
			frameCount++
			totalBytes += int64(len(frame))
			broadcaster.Publish(frame)
		}
		log.Printf("Capture stats: %d frames, %d total bytes", frameCount, totalBytes)
		if err := scanner.Err(); err != nil {
			log.Println("Capture scanner error:", err)
		}
		log.Println("WARN: ffmpeg capture goroutine exited")
	}()
	return nil
}

func (c *Capturer) startWaylandCapture(broadcaster *stream.Broadcaster) error {
	startX, startY, width, height := getPrimaryDisplayGeometry()
	log.Printf("Primary display geometry: %d,%d %dx%d", startX, startY, width, height)

	go func() {
		for {
			select {
			case <-c.done:
				log.Println("Wayland capture loop stopped")
				return
			default:
				start := time.Now()
				ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
				cmd := exec.CommandContext(ctx, "cosmic-screenshot", "--interactive=false", "--notify=false", "-s", "/dev/shm")
				out, err := cmd.Output()
				cancel()
				if err != nil {
					log.Printf("cosmic-screenshot failed or timed out: %v", err)
					// Backoff sleep on failure to prevent DBus/portal flooding
					time.Sleep(500 * time.Millisecond)
					continue
				}

				filePath := strings.TrimSpace(string(out))
				if filePath == "" {
					time.Sleep(100 * time.Millisecond)
					continue
				}

				f, err := os.Open(filePath)
				if err != nil {
					log.Printf("Failed to open captured image %s: %v", filePath, err)
					continue
				}

				img, err := png.Decode(f)
				f.Close()
				os.Remove(filePath)

				if err != nil {
					log.Printf("Failed to decode PNG image: %v", err)
					continue
				}

				var croppedImg image.Image = img
				type subImager interface {
					SubImage(r image.Rectangle) image.Image
				}
				if si, ok := img.(subImager); ok {
					croppedImg = si.SubImage(image.Rect(startX, startY, startX+width, startY+height))
				}

				var buf bytes.Buffer
				err = jpeg.Encode(&buf, croppedImg, &jpeg.Options{Quality: c.quality})
				if err != nil {
					log.Printf("Failed to encode JPEG: %v", err)
					continue
				}

				broadcaster.Publish(buf.Bytes())

				elapsed := time.Since(start)
				// Cap Wayland capture to maximum 6 FPS (minimum 160ms delay) to prevent DBus portal rate-limiting!
				targetDelay := time.Second / time.Duration(c.fps)
				if targetDelay < 160*time.Millisecond {
					targetDelay = 160 * time.Millisecond
				}
				delay := targetDelay - elapsed
				if delay > 0 {
					time.Sleep(delay)
				} else {
					time.Sleep(50 * time.Millisecond) // yield CPU
				}
			}
		}
	}()

	return nil
}

func (c *Capturer) Stop() {
	close(c.done)
	if c.cmd != nil && c.cmd.Process != nil {
		c.cmd.Process.Kill()
	}
}

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func stripANSI(str string) string {
	return ansiRegex.ReplaceAllString(str, "")
}

func getPrimaryDisplayGeometry() (int, int, int, int) {
	// Defaults to 1920x1080 if not found
	width, height := 1920, 1080
	startX, startY := 0, 0

	cmd := exec.Command("cosmic-randr", "list")
	out, err := cmd.Output()
	if err != nil {
		return startX, startY, width, height
	}

	cleanedOut := stripANSI(string(out))
	lines := strings.Split(cleanedOut, "\n")
	var positionStr string
	var currentModeStr string
	isPrimary := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		if !strings.HasPrefix(line, " ") && strings.Contains(line, "(") {
			if isPrimary && currentModeStr != "" {
				w, h := parseMode(currentModeStr)
				x, y := parsePosition(positionStr)
				if w > 0 && h > 0 {
					return x, y, w, h
				}
			}
			isPrimary = false
			positionStr = ""
			currentModeStr = ""
		}

		if strings.HasPrefix(trimmed, "Position:") {
			positionStr = trimmed
		}
		if strings.HasPrefix(trimmed, "Xwayland primary: true") {
			isPrimary = true
		}
		if strings.Contains(positionStr, "0,0") && !isPrimary {
			isPrimary = true
		}
		if strings.Contains(trimmed, "(current)") {
			currentModeStr = trimmed
		}
	}

	if isPrimary && currentModeStr != "" {
		w, h := parseMode(currentModeStr)
		x, y := parsePosition(positionStr)
		if w > 0 && h > 0 {
			return x, y, w, h
		}
	}

	return startX, startY, width, height
}

func parseMode(modeLine string) (int, int) {
	fields := strings.Fields(modeLine)
	if len(fields) == 0 {
		return 0, 0
	}
	parts := strings.Split(fields[0], "x")
	if len(parts) != 2 {
		return 0, 0
	}
	w, _ := strconv.Atoi(parts[0])
	h, _ := strconv.Atoi(parts[1])
	return w, h
}

func parsePosition(posLine string) (int, int) {
	fields := strings.Fields(posLine)
	if len(fields) < 2 {
		return 0, 0
	}
	parts := strings.Split(fields[1], ",")
	if len(parts) != 2 {
		return 0, 0
	}
	x, _ := strconv.Atoi(parts[0])
	y, _ := strconv.Atoi(parts[1])
	return x, y
}

func DetectPulseAudioMonitor() string {
	cmd := exec.Command("pactl", "list", "sources", "short")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "monitor") {
			fields := strings.Fields(line)
			if len(fields) > 1 {
				return fields[1]
			}
		}
	}
	return ""
}

func StartAudioBroadcaster(b *stream.Broadcaster) *exec.Cmd {
	monitor := DetectPulseAudioMonitor()
	if monitor == "" {
		return nil
	}
	cmd := exec.Command("ffmpeg",
		"-f", "pulse",
		"-i", monitor,
		"-ac", "2",
		"-ar", "44100",
		"-f", "mp3",
		"pipe:1",
	)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil
	}
	if err := cmd.Start(); err != nil {
		return nil
	}
	go func() {
		defer cmd.Wait()
		buf := make([]byte, 4096)
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				chunk := make([]byte, n)
				copy(chunk, buf[:n])
				b.Publish(chunk)
			}
			if err != nil {
				break
			}
		}
	}()
	return cmd
}
