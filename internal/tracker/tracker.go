package tracker

import (
	"bufio"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

type ClientInfo struct {
	IP         string
	Hostname   string
	MAC        string
	Bytes      int64
	PrevBytes  int64
	LastSeen   time.Time
	Latency    int
	OS         string
	Browser    string
	Resolution string
	DevType    string
	GPU        string
	BatPct     int
	Charging   bool
	Active     bool
}

type RejectedClient struct {
	IP        string
	OS        string
	Reason    string
	Time      time.Time
	UserAgent string
}

type Tracker struct {
	mu         sync.RWMutex
	Clients    map[string]*ClientInfo
	Rejections []RejectedClient
	TotalBytes int64
}

// New creates a new Tracker instance.
func New() *Tracker {
	return &Tracker{
		Clients:    make(map[string]*ClientInfo),
		Rejections: make([]RejectedClient, 0),
	}
}

// Mutex helpers for external packages

func (t *Tracker) Lock()    { t.mu.Lock() }
func (t *Tracker) Unlock()  { t.mu.Unlock() }
func (t *Tracker) RLock()   { t.mu.RLock() }
func (t *Tracker) RUnlock() { t.mu.RUnlock() }
func (t *Tracker) GetAllClients() []*ClientInfo {
	t.mu.RLock()
	defer t.mu.RUnlock()
	clients := make([]*ClientInfo, 0, len(t.Clients))
	for _, c := range t.Clients {
		clients = append(clients, c)
	}
	return clients
}

func (t *Tracker) GetClient(ip string) *ClientInfo {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	if client, exists := t.Clients[ip]; exists {
		client.LastSeen = time.Now()
		client.Active = true
		return client
	}
	
	client := &ClientInfo{
		IP:       ip,
		LastSeen: time.Now(),
		Active:   true,
	}
	t.Clients[ip] = client
	
	// Async hostname resolution
	go func(ip string) {
		names, err := net.LookupAddr(ip)
		if err == nil && len(names) > 0 {
			name := strings.TrimSuffix(names[0], ".")
			t.mu.Lock()
			if c, ok := t.Clients[ip]; ok {
				c.Hostname = name
			}
			t.mu.Unlock()
		}
	}(ip)
	
	// Async MAC resolution
	go func(ip string) {
		mac := lookupMAC(ip)
		if mac != "" {
			t.mu.Lock()
			if c, ok := t.Clients[ip]; ok {
				c.MAC = mac
			}
			t.mu.Unlock()
		}
	}(ip)
	
	return client
}

func (t *Tracker) AddBytes(ip string, n int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.TotalBytes += n
	if client, ok := t.Clients[ip]; ok {
		client.Bytes += n
	}
}

func (t *Tracker) LogRejection(ip, os, reason, ua string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	rej := RejectedClient{
		IP:        ip,
		OS:        os,
		Reason:    reason,
		Time:      time.Now(),
		UserAgent: ua,
	}
	t.Rejections = append([]RejectedClient{rej}, t.Rejections...)
	if len(t.Rejections) > 5 {
		t.Rejections = t.Rejections[:5]
	}
}

func (t *Tracker) Prune(timeout time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now()
	for _, client := range t.Clients {
		if now.Sub(client.LastSeen) > timeout {
			client.Active = false
		}
	}
}

func lookupMAC(ip string) string {
	file, err := os.Open("/proc/net/arp")
	if err != nil {
		return ""
	}
	defer file.Close()
	
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 4 && fields[0] == ip {
			return fields[3]
		}
	}
	return ""
}
