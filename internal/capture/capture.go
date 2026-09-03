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
	audioCmd  *exec.Cmd  // separate process — avoids GStreamer clock conflicts
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

// startX11 captures the screen via GStreamer's ximagesrc and encodes directly
// to VP8/Opus RTP — same pipeline approach as Wayland, no ffmpeg needed.
func (c *Capture) startX11(ctx context.Context) error {
	display := os.Getenv("DISPLAY")
	if display == "" {
		display = ":0"
	}

	// Build GStreamer pipeline: ximagesrc → VP8 RTP + fakesink (same as Wayland)
	pipeline := fmt.Sprintf(
		"ximagesrc display-name=%s ! videoconvert ! videorate ! video/x-raw,framerate=%d/1 ! tee name=t "+
			"t. ! queue max-size-buffers=1 leaky=downstream ! vp8enc threads=4 deadline=1 cpu-used=8 ! rtpvp8pay ! udpsink host=127.0.0.1 port=5004 sync=false "+
			"t. ! queue max-size-buffers=1 leaky=downstream ! fakesink sync=false",
		display, c.config.FPS,
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
		return fmt.Errorf("GStreamer X11: %w", err)
	}

	log.Printf("[+] X11 capture started (%s, %s @ %dfps)", display, c.config.Resolution, c.config.FPS)
	c.setStreaming(true)
	return nil
}

// startWayland captures the screen on GNOME/Wayland using the Mutter
// ScreenCast D-Bus API + GStreamer. Video and audio run as separate
// gst-launch-1.0 processes to avoid PipeWire/PulseAudio clock conflicts.
func (c *Capture) startWayland(ctx context.Context) error {
	conn, nodeID, err := mutterScreenCastNode()
	if err != nil {
		return fmt.Errorf("Mutter ScreenCast: %w", err)
	}
	c.dbusConn = conn

	// Wait for PipeWire to actually register the node before GStreamer tries
	// to open it. Without this delay there is a race where gst-launch-1.0
	// receives "stream error: target not found" because the node ID returned
	// by the D-Bus signal hasn't been published to the PipeWire registry yet.
	if err := waitForPipeWireNode(ctx, nodeID, 5*time.Second); err != nil {
		conn.Close()
		return fmt.Errorf("PipeWire node %d never became ready: %w", nodeID, err)
	}

	// ── Video pipeline (PipeWire → VP8 → RTP :5004) ──────────────────────
	videoPipeline := fmt.Sprintf(
		"pipewiresrc path=%d ! videoconvert ! videorate ! video/x-raw,framerate=%d/1 ! tee name=t "+
			"t. ! queue max-size-buffers=1 leaky=downstream ! vp8enc threads=4 deadline=1 cpu-used=8 ! rtpvp8pay ! udpsink host=127.0.0.1 port=5004 sync=false "+
			"t. ! queue max-size-buffers=1 leaky=downstream ! fakesink sync=false",
		nodeID, c.config.FPS,
	)

	c.videoCmd = exec.CommandContext(ctx, "gst-launch-1.0", append([]string{"-q"}, strings.Fields(videoPipeline)...)...)
	c.videoCmd.Stderr = os.Stderr
	if err := c.videoCmd.Start(); err != nil {
		return fmt.Errorf("GStreamer video: %w", err)
	}
	log.Printf("[+] Wayland capture started (PipeWire node %d, %dfps)", nodeID, c.config.FPS)

	// ── Audio pipeline (PulseAudio monitor → Opus → RTP :5005) ───────────
	// Run as a SEPARATE process so its PulseAudio clock doesn't conflict
	// with pipewiresrc's PipeWire clock inside a shared GStreamer pipeline.
	if !c.config.NoAudio {
		source := detectPulseSource()
		if source != "" {
			audioPipeline := fmt.Sprintf(
				"pulsesrc device=%s ! audioconvert ! opusenc bitrate=64000 audio-type=restricted-lowdelay ! rtpopuspay ! udpsink host=127.0.0.1 port=5005 sync=false",
				source,
			)
			c.audioCmd = exec.CommandContext(ctx, "gst-launch-1.0", append([]string{"-q"}, strings.Fields(audioPipeline)...)...)
			c.audioCmd.Stderr = os.Stderr
			if err := c.audioCmd.Start(); err != nil {
				log.Printf("[capture] Audio pipeline failed to start: %v", err)
			} else {
				log.Printf("[+] Audio capture started (%s, Opus)", source)
				go func() {
					c.audioCmd.Wait() //nolint:errcheck
					log.Println("[capture] Audio pipeline exited")
				}()
			}
		} else {
			log.Println("[capture] No audio source found, skipping audio")
		}
	}

	c.setStreaming(true)

	// Watchdog: mark streaming=false when video pipeline exits.
	go func() {
		c.videoCmd.Wait() //nolint:errcheck
		c.setStreaming(false)
		log.Println("[capture] GStreamer video pipeline exited")
	}()

	return nil
}

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

// waitForPipeWireNode polls pw-cli until the given node ID appears in the
// PipeWire registry, or until the deadline is reached. This prevents the
// GStreamer pipeline from starting before the node is actually available.
func waitForPipeWireNode(ctx context.Context, nodeID uint32, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		out, err := exec.CommandContext(ctx, "pw-cli", "info", fmt.Sprintf("%d", nodeID)).Output()
		if err == nil && len(out) > 0 {
			log.Printf("[capture] PipeWire node %d is ready", nodeID)
			// Give PipeWire an extra moment to fully negotiate the stream
			// format before GStreamer connects.
			time.Sleep(500 * time.Millisecond)
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("timeout after %s", timeout)
}

// Stop kills all capture processes.
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
		c.audioCmd = nil
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
