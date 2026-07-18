package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/nelobster/bifrost/internal/stats"
)

func (s *Server) handleAdmin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Write(s.adminHTML)
}

func (s *Server) handleAPIClients(w http.ResponseWriter, r *http.Request) {
	clients := s.tracker.GetAll()
	type clientJSON struct {
		IP        string `json:"ip"`
		Host      string `json:"host"`
		OS        string `json:"os"`
		Browser   string `json:"browser"`
		Device    string `json:"device"`
		Active    bool   `json:"active"`
		Bandwidth string `json:"bandwidth"`
		Total     string `json:"total"`
	}
	result := make([]clientJSON, 0, len(clients))
	for _, c := range clients {
		result = append(result, clientJSON{
			IP:        c.IP,
			Host:      c.Host,
			OS:        c.OS,
			Browser:   c.Browser,
			Device:    c.Device,
			Active:    c.Active,
			Bandwidth: formatBytesRate(c.Bytes),
			Total:     formatBytesTotal(c.Bytes),
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"clients": result})
}

func (s *Server) handleAPIStats(w http.ResponseWriter, r *http.Request) {
	sys := stats.Collect()

	result := map[string]interface{}{
		"streaming": s.capture.IsStreaming(),
		"clients":   s.tracker.CountActive(),
		"uptime":    sys.Uptime,
		"hostname":  sys.CPUModel,
	}

	// CPU
	result["cpu_usage"] = sys.CPUUsage
	result["cpu_freq"] = sys.CPUFreqMHz
	result["cpu_temp"] = sys.CPUTempC
	result["cpu_cores"] = sys.CPUCores

	// Memory
	result["mem_pct"] = sys.MemPct
	result["mem_used"] = fmt.Sprintf("%.1fG", sys.MemUsedGB)
	result["mem_total"] = fmt.Sprintf("%.1fG", sys.MemTotalGB)

	// Disk
	result["disk_pct"] = sys.DiskPct
	result["disk_used"] = sys.DiskUsed
	result["disk_total"] = sys.DiskTotal

	// Swap
	result["swap_pct"] = sys.SwapPct
	result["swap_used"] = fmt.Sprintf("%.1fG", sys.SwapUsedGB)
	result["swap_total"] = fmt.Sprintf("%.1fG", sys.SwapTotalGB)

	// GPU
	if sys.GPUFreqMHz > 0 {
		result["gpu_freq"] = fmt.Sprintf("%d MHz", sys.GPUFreqMHz)
	}
	if sys.GPUTempC > 0 {
		result["gpu_temp"] = sys.GPUTempC
	}

	// NIC
	result["nic_name"] = sys.NICName
	result["nic_speed"] = sys.NICSpeed
	result["nic_type"] = sys.NICType

	// Fan
	if sys.FanRPM > 0 {
		result["fan"] = sys.FanRPM
	}

	// Battery
	if sys.BatPct != "N/A" {
		result["battery_pct"] = sys.BatPct
		result["battery_status"] = sys.BatStatus
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func formatBytesRate(b uint64) string {
	// Simplified: just show total for now
	return "--"
}

func formatBytesTotal(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%dB", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%c", float64(b)/float64(div), "KMGTPE"[exp])
}
