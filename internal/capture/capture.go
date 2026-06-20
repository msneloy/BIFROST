package capture

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"
	"time"

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

func (c *Capturer) Start(broadcaster *stream.Broadcaster) (err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Hybrid Capture Engine (Wayland + X11 Aware)
	// We use FFmpeg with MJPEG output for zero-stall stability
	var pipelineStr string

	sessionType := os.Getenv("XDG_SESSION_TYPE")
	if sessionType == "wayland" {
		// Native Wayland capture via KMS (requires cap_sys_admin+ep on ffmpeg)
		pipelineStr = fmt.Sprintf("ffmpeg -loglevel error -f kmsgrab -follow_mouse 1 -i /dev/dri/card0 -vf 'hwdownload,format=bgr0,fps=%d,scale=1280:-1' -f mjpeg -q:v 2 -", c.fps)
		log.Printf("Detected Wayland session. Using kmsgrab bridge.")
	} else {
		// Standard X11 capture
		pipelineStr = fmt.Sprintf("ffmpeg -loglevel error -f x11grab -draw_mouse 1 -video_size 1280x720 -framerate %d -i :0.0 -f mjpeg -q:v 2 -", c.fps)
		log.Printf("Detected X11 session. Using x11grab bridge.")
	}

	// The capture loop now is a lightweight goroutine that just keeps the broadcaster alive
	// while waiting for browser-native push or binary capture.
	log.Printf("Capture engine ACTIVE (Monitoring for frames...)")

	go func() {
		for {
			select {
			case <-c.done:
				return
			case <-time.After(5 * time.Second):
				// Just a heartbeat
			}
		}
	}()

	log.Printf("Starting HYBRID capture: %s", pipelineStr)

	cmd := exec.Command("sh", "-c", pipelineStr)
	cmd.Env = os.Environ()
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}
	c.cmd = cmd

	// Feed stdout to broadcaster with JPEG boundary parsing
	go func() {
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

						// Priority check: If a browser-native bridge is active, stop binary capture
						if string(broadcaster.GetHeader()) == "BRIDGE" {
							log.Println("BRIDGE DETECTED: Binary capture yielding to browser-native stream.")
							return
						}

						// Live Diagnostic: Save one frame to project root to verify capture
						staticOnce.Do(func() {
							_ = os.WriteFile("debug_capture.jpg", frame, 0644)
							log.Println("SUCCESS: Captured live frame to debug_capture.jpg")
						})

						broadcaster.Publish(frame)

						content = content[start+frameLen:]
						buffer.Reset()
						buffer.Write(content)
					}
				}
				if err != nil {
					if err != io.EOF {
						log.Printf("Capture Stdout error: %v", err)
					}
					return
				}
			}
		}
	}()

	return nil
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

func DetectPulseAudioMonitor() string {
	return ""
}
