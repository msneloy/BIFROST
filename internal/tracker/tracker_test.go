package tracker

import (
	"testing"
	"time"
)

func TestTrackerGetOrCreate(t *testing.T) {
	trk := New()

	c1 := trk.GetOrCreate("192.168.1.1")
	if c1 == nil {
		t.Fatal("expected non-nil client")
	}
	if c1.IP != "192.168.1.1" {
		t.Errorf("expected IP 192.168.1.1, got %s", c1.IP)
	}
	if !c1.Active {
		t.Error("expected client to be active")
	}

	c2 := trk.GetOrCreate("192.168.1.1")
	if c1 != c2 {
		t.Error("expected same client instance for same IP")
	}

	c3 := trk.GetOrCreate("192.168.1.2")
	if c3 == c1 {
		t.Error("expected different client for different IP")
	}
}

func TestTrackerCountActive(t *testing.T) {
	trk := New()

	if trk.CountActive() != 0 {
		t.Errorf("expected 0 active clients, got %d", trk.CountActive())
	}

	trk.GetOrCreate("192.168.1.1")
	trk.GetOrCreate("192.168.1.2")
	trk.GetOrCreate("192.168.1.3")

	if trk.CountActive() != 3 {
		t.Errorf("expected 3 active clients, got %d", trk.CountActive())
	}
}

func TestTrackerAddBytes(t *testing.T) {
	trk := New()
	trk.GetOrCreate("192.168.1.1")

	trk.AddBytes("192.168.1.1", 1024)
	trk.AddBytes("192.168.1.1", 2048)

	clients := trk.GetAll()
	if len(clients) != 1 {
		t.Fatalf("expected 1 client, got %d", len(clients))
	}
	if clients[0].Bytes != 3072 {
		t.Errorf("expected 3072 bytes, got %d", clients[0].Bytes)
	}
}

func TestTrackerPrune(t *testing.T) {
	trk := New()

	c := trk.GetOrCreate("192.168.1.1")
	c.LastSeen = time.Now().Add(-1 * time.Minute)

	trk.Prune(30 * time.Second)

	clients := trk.GetAll()
	if len(clients) != 1 {
		t.Fatalf("expected 1 client, got %d", len(clients))
	}
	if clients[0].Active {
		t.Error("expected pruned client to be inactive")
	}
}

func TestTrackerUpdateTelemetry(t *testing.T) {
	trk := New()

	trk.UpdateTelemetry("192.168.1.1", "50ms", "Linux", "Firefox", "1920x1080", "desktop")

	clients := trk.GetAll()
	if len(clients) != 1 {
		t.Fatalf("expected 1 client, got %d", len(clients))
	}

	c := clients[0]
	if c.Latency != "50ms" {
		t.Errorf("expected latency 50ms, got %s", c.Latency)
	}
	if c.OS != "Linux" {
		t.Errorf("expected OS Linux, got %s", c.OS)
	}
	if c.Browser != "Firefox" {
		t.Errorf("expected browser Firefox, got %s", c.Browser)
	}
}

func TestTrackerStatsJSON(t *testing.T) {
	trk := New()
	trk.GetOrCreate("192.168.1.1")

	json := trk.StatsJSON()
	if json == "" {
		t.Fatal("expected non-empty stats JSON")
	}
	if len(json) < 10 {
		t.Errorf("suspiciously short JSON: %s", json)
	}
}

func TestTrackerGetAll(t *testing.T) {
	trk := New()

	trk.GetOrCreate("192.168.1.1")
	trk.GetOrCreate("192.168.1.2")

	all := trk.GetAll()
	if len(all) != 2 {
		t.Errorf("expected 2 clients, got %d", len(all))
	}
}

func TestTrackerConcurrentAccess(t *testing.T) {
	trk := New()

	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func(n int) {
			for j := 0; j < 100; j++ {
				trk.GetOrCreate("192.168.1.1")
				trk.AddBytes("192.168.1.1", 1)
				trk.CountActive()
			}
			done <- struct{}{}
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	clients := trk.GetAll()
	if len(clients) != 1 {
		t.Errorf("expected 1 client after concurrent access, got %d", len(clients))
	}
}
