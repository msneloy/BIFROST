package capture

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"runtime"
	"sync"

	"bifrost/internal/stream"
)

var staticOnce sync.Once

type Capturer struct {
	fps     int
	quality int
	cmd     *exec.Cmd
	done    chan struct{}
	mu      sync.Mutex
}

func NewCapturer(fps, quality int) *Capturer {
	if fps <= 0 {
		fps = 15
	}
	return &Capturer{
		fps:     fps,
		quality: quality,
		done:    make(chan struct{}),
	}
}

func (c *Capturer) Start(broadcaster *stream.Broadcaster) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	sessionType := os.Getenv("XDG_SESSION_TYPE")

	if sessionType == "wayland" {
		return c.startMutter(broadcaster)
	}
	return c.startX11(broadcaster)
}

func (c *Capturer) startMutter(broadcaster *stream.Broadcaster) error {
	log.Println("Wayland detected — using Mutter ScreenCast (zero interaction)")

	helper := findFile(
		"scripts/mutter_capture.py",
		"../scripts/mutter_capture.py",
		"../../scripts/mutter_capture.py",
	)

	cmd := exec.Command("python3", helper)
	cmd.Env = os.Environ()
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}

	stderr, _ := cmd.StderrPipe()
	go pipeLog(stderr, "[portal]")

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("portal capture start: %w", err)
	}
	c.cmd = cmd

	log.Printf("Portal capture PID %d", cmd.Process.Pid)

	go c.readFrames(broadcaster, stdout, cmd)
	return nil
}

func (c *Capturer) startX11(broadcaster *stream.Broadcaster) error {
	display := os.Getenv("DISPLAY")
	if display == "" {
		display = ":0"
	}

	audioSource := detectPulseAudioSource()
	if audioSource == "" {
		audioSource = "default"
	}

	pipeline := fmt.Sprintf(
		"ffmpeg -y -loglevel warning -f x11grab -draw_mouse 1 -video_size 1280x720 -framerate %d -i %s "+
			"-f pulse -i %s "+
			"-c:v mjpeg -q:v %d -f mjpeg - "+
			"-c:a libmp3lame -b:a 128k -f mp3 /tmp/bifrost_audio.mp3",
		c.fps, display, audioSource, c.quality,
	)
	log.Printf("X11 — x11grab + pulse (%s)", audioSource)

	cmd := exec.Command("sh", "-c", pipeline)
	cmd.Env = os.Environ()
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}

	stderr, _ := cmd.StderrPipe()
	go pipeLog(stderr, "[ffmpeg]")

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("ffmpeg start: %w", err)
	}
	c.cmd = cmd

	log.Printf("Capture engine ACTIVE — PID %d", cmd.Process.Pid)

	go c.readFrames(broadcaster, stdout, cmd)
	return nil
}

func (c *Capturer) readFrames(broadcaster *stream.Broadcaster, stdout io.ReadCloser, cmd *exec.Cmd) {
	defer cmd.Wait()

	var buffer bytes.Buffer
	tempBuf := make([]byte, 64*1024)

	for {
		select {
		case <-c.done:
			return
		default:
			n, err := stdout.Read(tempBuf)
			if n > 0 {
				buffer.Write(tempBuf[:n])
				content := buffer.Bytes()
				for {
					start := bytes.Index(content, []byte{0xFF, 0xD8})
					if start == -1 {
						if buffer.Len() > 0 {
							lastByte := content[len(content)-1]
							buffer.Reset()
							buffer.WriteByte(lastByte)
						}
						break
					}

					end := bytes.Index(content[start:], []byte{0xFF, 0xD9})
					if end == -1 {
						newData := make([]byte, len(content[start:]))
						copy(newData, content[start:])
						buffer.Reset()
						buffer.Write(newData)
						break
					}

					frameLen := end + 2
					frame := make([]byte, frameLen)
					copy(frame, content[start:start+frameLen])

					if string(broadcaster.GetHeader()) == "BRIDGE" {
						log.Println("BRIDGE active — yielding to browser push.")
						return
					}

					staticOnce.Do(func() {
						_ = os.WriteFile("debug_capture.jpg", frame, 0644)
						log.Println("First frame saved to debug_capture.jpg")
					})

					broadcaster.Publish(frame)

					content = content[start+frameLen:]
					buffer.Reset()
					buffer.Write(content)
				}
			}
			if err != nil {
				if err != io.EOF {
					log.Printf("Capture read error: %v", err)
				}
				return
			}
		}
	}
}

func (c *Capturer) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	select {
	case <-c.done:
		return
	default:
		close(c.done)
	}

	if c.cmd != nil && c.cmd.Process != nil {
		_ = exec.Command("pkill", "-9", "-P", fmt.Sprintf("%d", c.cmd.Process.Pid)).Run()
		c.cmd.Process.Kill()
	}
}

func pipeLog(rc io.ReadCloser, prefix string) {
	buf := make([]byte, 1024)
	for {
		n, err := rc.Read(buf)
		if n > 0 {
			log.Printf("%s %s", prefix, string(bytes.TrimSpace(buf[:n])))
		}
		if err != nil {
			return
		}
	}
}

func findFile(paths ...string) string {
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	// Last resort: try relative to this source file
	if _, self, _, ok := runtime.Caller(0); ok {
		return self + "/../../../scripts/portal_capture.py"
	}
	return paths[0]
}

func detectPulseAudioSource() string {
	out, err := exec.Command("pactl", "list", "short", "sources").CombinedOutput()
	if err != nil {
		return ""
	}
	lines := bytes.Split(out, []byte("\n"))
	for _, line := range lines {
		if bytes.Contains(line, []byte("monitor")) {
			fields := bytes.Fields(line)
			if len(fields) >= 2 {
				return string(fields[1])
			}
		}
	}
	return ""
}
