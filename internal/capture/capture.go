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
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/nelobster/bifrost/internal/config"
)

// Capture manages ffmpeg/gst-launch processes for screen and audio capture.
// All output goes to RTP (VP8 video + Opus audio) for WebRTC delivery.
type Capture struct {
	config    *config.Config
	videoCmd  *exec.Cmd
	dbusConn  *dbus.Conn // kept alive for Wayland Mutter session
	streaming bool
	mu        sync.Mutex
}

func New(cfg *config.Config) *Capture {
	return &Capture{config: cfg}
}

func (c *Capture) IsStreaming() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.streaming
}

// Start begins screen capture. Output is VP8 RTP to UDP 5004 and Opus RTP to
// UDP 5005, consumed by the WebRTC RTPReceiver.
func (c *Capture) Start(ctx context.Context) error {
	if detectSessionType() == "wayland" {
		return c.startWayland(ctx)
	}
	return c.startX11(ctx)
}

// startX11 captures the screen via ffmpeg's x11grab and encodes directly to
// VP8/Opus RTP.
func (c *Capture) startX11(ctx context.Context) error {
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

	// VP8 RTP → UDP 5004 (WebRTC video)
	args = append(args,
		"-map", "0:v",
		"-c:v", "libvpx", "-b:v", "2M",
		"-deadline", "realtime", "-cpu-used", "4",
		"-f", "rtp", "udp://127.0.0.1:5004",
	)

	// Opus RTP → UDP 5005 (WebRTC audio)
	if audioSource != "" {
		args = append(args,
			"-map", "1:a",
			"-c:a", "libopus", "-b:a", "64k",
			"-f", "rtp", "udp://127.0.0.1:5005",
		)
	}

	c.videoCmd = exec.CommandContext(ctx, "ffmpeg", args...)
	c.videoCmd.Stderr = os.Stderr

	if err := c.videoCmd.Start(); err != nil {
		return fmt.Errorf("failed to start X11 capture: %w", err)
	}

	log.Printf("[+] X11 capture started (%s, %s @ %dfps, audio=%v)",
		display, c.config.Resolution, c.config.FPS, audioSource != "")
	c.setStreaming(true)
	return nil
}

// startWayland captures the screen on GNOME/Wayland using the Mutter
// ScreenCast D-Bus API + GStreamer. Single pipeline for video+audio so they
// share the same GStreamer clock for perfect sync.
func (c *Capture) startWayland(ctx context.Context) error {
	conn, nodeID, err := mutterScreenCastNode()
	if err != nil {
		return fmt.Errorf("Mutter ScreenCast: %w", err)
	}
	c.dbusConn = conn

	// Build pipeline matching the original mutter_capture.py exactly.
	// tee with two branches: VP8→RTP and fakesink (keeps pipeline flowing).
	pipeline := fmt.Sprintf(
		"pipewiresrc path=%d ! videoconvert ! videorate ! video/x-raw,framerate=%d/1 ! tee name=t "+
			"t. ! queue max-size-buffers=1 leaky=downstream ! vp8enc threads=4 deadline=1 cpu-used=8 ! rtpvp8pay ! udpsink host=127.0.0.1 port=5004 sync=false "+
			"t. ! queue max-size-buffers=1 leaky=downstream ! fakesink sync=false",
		nodeID, c.config.FPS,
	)

	if !c.config.NoAudio {
		source := detectPulseSource()
		if source != "" {
			pipeline += fmt.Sprintf(
				" pulsesrc device=%s ! audioconvert ! opusenc bitrate=64000 audio-type=restricted-lowdelay ! rtpopuspay ! udpsink host=127.0.0.1 port=5005 sync=false",
				source,
			)
		}
	}

	c.videoCmd = exec.CommandContext(ctx, "gst-launch-1.0", append([]string{"-q"}, strings.Fields(pipeline)...)...)
	c.videoCmd.Stderr = os.Stderr

	if err := c.videoCmd.Start(); err != nil {
		return fmt.Errorf("GStreamer: %w", err)
	}

	log.Printf("[+] Wayland capture started (PipeWire node %d, %dfps)", nodeID, c.config.FPS)
	c.setStreaming(true)
	return nil
}

// mutterScreenCastNode uses the GNOME Mutter ScreenCast D-Bus API to start a
// screen capture session and returns the D-Bus connection (must be kept alive)
// and the PipeWire node ID.
func mutterScreenCastNode() (*dbus.Conn, uint32, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, 0, fmt.Errorf("D-Bus session bus: %w", err)
	}

	const dest = "org.gnome.Mutter.ScreenCast"
	root := dbus.ObjectPath("/org/gnome/Mutter/ScreenCast")

	// CreateSession({})
	var sessionPath dbus.ObjectPath
	err = conn.Object(dest, root).Call(
		dest+".CreateSession", 0,
		map[string]dbus.Variant{},
	).Store(&sessionPath)
	if err != nil {
		conn.Close()
		return nil, 0, fmt.Errorf("CreateSession: %w", err)
	}

	// RecordMonitor("", {})
	var streamPath dbus.ObjectPath
	err = conn.Object(dest, sessionPath).Call(
		dest+".Session.RecordMonitor", 0,
		"", map[string]dbus.Variant{},
	).Store(&streamPath)
	if err != nil {
		conn.Close()
		return nil, 0, fmt.Errorf("RecordMonitor: %w", err)
	}

	// Subscribe to PipeWireStreamAdded signal before calling Start
	rule := fmt.Sprintf("type='signal',interface='%s.Session',member='PipeWireStreamAdded'", dest)
	conn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0, rule)

	sigChan := make(chan *dbus.Signal, 4)
	conn.Signal(sigChan)
	defer conn.RemoveSignal(sigChan)

	// Start()
	err = conn.Object(dest, sessionPath).Call(
		dest+".Session.Start", 0,
	).Store()
	if err != nil {
		conn.Close()
		return nil, 0, fmt.Errorf("Start: %w", err)
	}

	// Wait for PipeWireStreamAdded signal (contains the node ID)
	select {
	case sig := <-sigChan:
		if len(sig.Body) < 1 {
			conn.Close()
			return nil, 0, fmt.Errorf("PipeWireStreamAdded signal had no body")
		}
		nodeID, ok := sig.Body[0].(uint32)
		if !ok {
			conn.Close()
			return nil, 0, fmt.Errorf("PipeWireStreamAdded body[0] is %T, not uint32", sig.Body[0])
		}
		log.Printf("[capture] Mutter PipeWire stream added: node %d", nodeID)
		return conn, nodeID, nil
	case <-time.After(10 * time.Second):
		conn.Close()
		return nil, 0, fmt.Errorf("timeout waiting for PipeWireStreamAdded signal")
	}
}

func (c *Capture) setStreaming(v bool) {
	c.mu.Lock()
	c.streaming = v
	c.mu.Unlock()
}

// Stop kills all capture processes.
func (c *Capture) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.videoCmd != nil && c.videoCmd.Process != nil {
		c.videoCmd.Process.Kill()
		c.videoCmd.Wait()
	}
	if c.dbusConn != nil {
		c.dbusConn.Close()
		c.dbusConn = nil
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
	var fallback string
	for _, line := range strings.Split(string(out), "\n") {
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
