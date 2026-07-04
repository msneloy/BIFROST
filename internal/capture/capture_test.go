package capture

import (
	"testing"
)

func TestDetectPulseAudioSource(t *testing.T) {
	source := detectPulseAudioSource()
	t.Logf("Detected audio source: %s", source)
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
