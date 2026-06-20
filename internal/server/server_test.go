package server

import (
	"bifrost/internal/stream"
	"bifrost/internal/tracker"
	"testing"
)

func TestNew(t *testing.T) {
	tr := tracker.New()
	vs := stream.NewBroadcaster()
	html := "<html></html>"

	srv := New(tr, vs, html)
	if srv == nil {
		t.Fatal("New server returned nil")
	}
	if srv.Addr == "" {
		// Default addr might be empty until set in main
	}
}
