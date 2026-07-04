package server

import (
	"bifrost/internal/stream"
	"bifrost/internal/tracker"
	"testing"
)

func TestNew(t *testing.T) {
	tr := tracker.New()
	vs := stream.NewBroadcaster()

	srv := New(tr, vs)
	if srv == nil {
		t.Fatal("New server returned nil")
	}
}
