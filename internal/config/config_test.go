package config

import (
	"os"
	"testing"
)

func TestParseResolution(t *testing.T) {
	tests := []struct {
		input        string
		wantW, wantH int
	}{
		{"1920x1080", 1920, 1080},
		{"1280x720", 1280, 720},
		{"bad", 1920, 1080},
		{"0x0", 1920, 1080},
		{"1920", 1920, 1080},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			w, h := parseResolution(tt.input)
			if w != tt.wantW || h != tt.wantH {
				t.Errorf("parseResolution(%q) = (%d, %d), want (%d, %d)", tt.input, w, h, tt.wantW, tt.wantH)
			}
		})
	}
}

func TestEnvInt(t *testing.T) {
	t.Run("set", func(t *testing.T) {
		os.Setenv("BIFROST_TEST_INT", "42")
		defer os.Unsetenv("BIFROST_TEST_INT")
		if got := envInt("BIFROST_TEST_INT", 10); got != 42 {
			t.Errorf("envInt with set env: got %d, want 42", got)
		}
	})
	t.Run("missing", func(t *testing.T) {
		if got := envInt("BIFROST_MISSING", 10); got != 10 {
			t.Errorf("envInt with missing env: got %d, want 10", got)
		}
	})
	t.Run("invalid", func(t *testing.T) {
		os.Setenv("BIFROST_TEST_BAD", "abc")
		defer os.Unsetenv("BIFROST_TEST_BAD")
		if got := envInt("BIFROST_TEST_BAD", 10); got != 10 {
			t.Errorf("envInt with invalid env: got %d, want 10", got)
		}
	})
}

func TestEnvStr(t *testing.T) {
	t.Run("set", func(t *testing.T) {
		os.Setenv("BIFROST_TEST_STR", "hello")
		defer os.Unsetenv("BIFROST_TEST_STR")
		if got := envStr("BIFROST_TEST_STR", "default"); got != "hello" {
			t.Errorf("envStr with set env: got %q, want %q", got, "hello")
		}
	})
	t.Run("missing", func(t *testing.T) {
		if got := envStr("BIFROST_MISSING", "default"); got != "default" {
			t.Errorf("envStr with missing env: got %q, want %q", got, "default")
		}
	})
}

func TestDetectLocalIP(t *testing.T) {
	cfg := &Config{}
	ip := cfg.DetectLocalIP()
	if ip == "" {
		t.Error("DetectLocalIP returned empty string")
	}
	if ip != "127.0.0.1" && len(ip) < 7 {
		t.Errorf("DetectLocalIP returned suspicious value: %q", ip)
	}
}
