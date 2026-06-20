package capture

import (
	"testing"
)

func TestDetectPulseAudioMonitor(t *testing.T) {
	// This test doesn't actually run a monitor but checks if the function returns something reasonable
	// or doesn't crash.
	monitor := DetectPulseAudioMonitor()
	t.Logf("Detected monitor: %s", monitor)
}

func TestNewCapturer(t *testing.T) {
	fps := 10
	quality := 40
	c := NewCapturer(fps, quality)
	if c == nil {
		t.Fatal("NewCapturer returned nil")
	}
	if c.fps != fps {
		t.Errorf("Expected FPS %d, got %d", fps, c.fps)
	}
}
