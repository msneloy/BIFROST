package tracker

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

type Client struct {
	IP         string
	Host       string
	MAC        string
	Bytes      uint64
	LastSeen   time.Time
	Latency    string
	OS         string
	Browser    string
	Resolution string
	Device     string
	Active     bool
}

type Tracker struct {
	mu         sync.RWMutex
	clients    map[string]*Client
	totalBytes uint64
	startTime  time.Time
}

func New() *Tracker {
	return &Tracker{
		clients:   make(map[string]*Client),
		startTime: time.Now(),
	}
}

func (t *Tracker) GetOrCreate(ip string) *Client {
	t.mu.Lock()
	defer t.mu.Unlock()

	if c, ok := t.clients[ip]; ok {
		c.LastSeen = time.Now()
		c.Active = true
		return c
	}

	c := &Client{
		IP:       ip,
		LastSeen: time.Now(),
		Active:   true,
	}
	t.clients[ip] = c

	go t.lookupDNS(ip)
	go t.lookupMAC(ip)

	return c
}

func (t *Tracker) UpdateTelemetry(ip, latency, osName, browser, resolution, device string) {
	c := t.GetOrCreate(ip)
	t.mu.Lock()
	defer t.mu.Unlock()

	if latency != "" {
		c.Latency = latency
	}
	if osName != "" {
		c.OS = osName
	}
	if browser != "" {
		c.Browser = browser
	}
	if resolution != "" {
		c.Resolution = resolution
	}
	if device != "" {
		c.Device = device
	}
	c.LastSeen = time.Now()
	c.Active = true
}

func (t *Tracker) AddBytes(ip string, n uint64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if c, ok := t.clients[ip]; ok {
		c.Bytes += n
	}
	t.totalBytes += n
}

func (t *Tracker) CountActive() int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	count := 0
	for _, c := range t.clients {
		if c.Active {
			count++
		}
	}
	return count
}

func (t *Tracker) Prune(timeout time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()

	cutoff := time.Now().Add(-timeout)
	for _, c := range t.clients {
		if c.LastSeen.Before(cutoff) {
			c.Active = false
		}
	}
}

func (t *Tracker) GetAll() []*Client {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make([]*Client, 0, len(t.clients))
	for _, c := range t.clients {
		result = append(result, c)
	}
	return result
}

func (t *Tracker) StatsJSON() string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	active := 0
	for _, c := range t.clients {
		if c.Active {
			active++
		}
	}

	stats := map[string]interface{}{
		"total_transmitted": fmt.Sprintf("%d bytes", t.totalBytes),
		"clients":           active,
		"uptime":            int(time.Since(t.startTime).Seconds()),
	}

	data, _ := json.Marshal(stats)
	return string(data)
}

func (t *Tracker) lookupDNS(ip string) {
	names, err := net.LookupAddr(ip)
	if err != nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if c, ok := t.clients[ip]; ok && len(names) > 0 {
		c.Host = strings.TrimSuffix(names[0], ".")
	}
}

func (t *Tracker) lookupMAC(ip string) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return
	}
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			if strings.Contains(addr.String(), ip) {
				t.mu.Lock()
				if c, ok := t.clients[ip]; ok {
					c.MAC = iface.HardwareAddr.String()
				}
				t.mu.Unlock()
				return
			}
		}
	}
}

func formatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%dB", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%cB", float64(b)/float64(div), "KMGTPE"[exp])
}
