package tracker

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

type Client struct {
	IP         string
	Host       string
	MAC        string
	Bytes      uint64
	PrevBytes  uint64
	LastSeen   time.Time
	Latency    string
	OS         string
	Browser    string
	Resolution string
	Device     string
	GPU        string
	Battery    string
	Charging   string
	Active     bool
}

type Rejection struct {
	IP     string `json:"ip"`
	OS     string `json:"os"`
	Reason string `json:"reason"`
	Time   string `json:"time"`
	UA     string `json:"ua"`
}

type Tracker struct {
	mu         sync.RWMutex
	clients    map[string]*Client
	rejections []Rejection
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

	// Async DNS and MAC lookups
	go t.lookupDNS(ip)
	go t.lookupMAC(ip)

	return c
}

func (t *Tracker) UpdateTelemetry(ip, latency, os, browser, resolution, device, gpu, battery string) {
	c := t.GetOrCreate(ip)
	t.mu.Lock()
	defer t.mu.Unlock()

	if latency != "" {
		c.Latency = latency
	}
	if os != "" {
		c.OS = os
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
	if gpu != "" {
		c.GPU = gpu
	}
	if battery != "" {
		c.Battery = battery
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

func (t *Tracker) LogRejection(ip, os, reason, ua string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	r := Rejection{
		IP:     ip,
		OS:     os,
		Reason: reason,
		Time:   time.Now().Format("15:04:05"),
		UA:     ua,
	}
	t.rejections = append([]Rejection{r}, t.rejections...)
	if len(t.rejections) > 5 {
		t.rejections = t.rejections[:5]
	}
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
		"rejections":        len(t.rejections),
	}

	data, _ := json.Marshal(stats)
	return string(data)
}

func (t *Tracker) Rejections() []Rejection {
	t.mu.RLock()
	defer t.mu.RUnlock()
	result := make([]Rejection, len(t.rejections))
	copy(result, t.rejections)
	return result
}

func (t *Tracker) lookupDNS(ip string) {
	names, err := net.LookupAddr(ip)
	if err == nil && len(names) > 0 {
		t.mu.Lock()
		if c, ok := t.clients[ip]; ok {
			c.Host = strings.TrimSuffix(names[0], ".")
		}
		t.mu.Unlock()
	}
}

func (t *Tracker) lookupMAC(ip string) {
	data, err := os.ReadFile("/proc/net/arp")
	if err != nil {
		return
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 4 && fields[0] == ip {
			t.mu.Lock()
			if c, ok := t.clients[ip]; ok {
				c.MAC = fields[3]
			}
			t.mu.Unlock()
			return
		}
	}
}
