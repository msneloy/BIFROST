package dashboard

import (
	"bifrost/internal/tracker"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func bar(pct float64, width int) string {
	filled := int(math.Round(pct / 100.0 * float64(width)))
	if filled < 0 { filled = 0 }
	if filled > width { filled = width }
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

func readLine(path string) string {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(strings.Split(string(data), "\n")[0])
}

// System stats gathering
type SysStats struct {
	CPUUsage   float64
	CPUFreq    string
	CPUTemp    string
	RAMPct     float64
	RAMUsed    float64
	RAMTotal   float64
	SwapPct    float64
	SwapUsed   float64
	SwapTotal  float64
	DiskPct    float64
	DiskTotal  float64
	DiskR      float64
	DiskW      float64
	GPUFreq    string
	GPUTemp    string
	FanRPM     string
	PCHTemp    string
	NICFace    string
	NICSpeed   string
	NICType    string
	BatPct     string
	BatETA     string
}

func GetSysStats(prev *SysStats) *SysStats {
	stats := &SysStats{}

	// CPU Load (approx)
	loadavg := readLine("/proc/loadavg")
	if parts := strings.Fields(loadavg); len(parts) > 0 {
		if load, err := strconv.ParseFloat(parts[0], 64); err == nil {
			// rough approximation, assumes 8 cores for % display
			stats.CPUUsage = math.Min(load/8.0*100.0, 100.0) 
		}
	}
	
	// CPU Freq
	cpuinfo, _ := ioutil.ReadFile("/proc/cpuinfo")
	for _, line := range strings.Split(string(cpuinfo), "\n") {
		if strings.HasPrefix(line, "cpu MHz") {
			parts := strings.Split(line, ":")
			if len(parts) == 2 {
				if freq, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64); err == nil {
					stats.CPUFreq = fmt.Sprintf("%.1fGHz", freq/1000.0)
					break
				}
			}
		}
	}

	// Temps & Fans (hwmon)
	hwmon, _ := filepath.Glob("/sys/class/hwmon/hwmon*")
	for _, hw := range hwmon {
		name := readLine(filepath.Join(hw, "name"))
		if strings.Contains(name, "coretemp") || strings.Contains(name, "k10temp") {
			t := readLine(filepath.Join(hw, "temp1_input"))
			if v, err := strconv.ParseFloat(t, 64); err == nil {
				stats.CPUTemp = fmt.Sprintf("%.0f°C", v/1000.0)
			}
		}
		if name == "pch_skylake" || name == "pch_cannonlake" {
			t := readLine(filepath.Join(hw, "temp1_input"))
			if v, err := strconv.ParseFloat(t, 64); err == nil {
				stats.PCHTemp = fmt.Sprintf("%.0f°C", v/1000.0)
			}
		}
		// Fan
		if fan := readLine(filepath.Join(hw, "fan1_input")); fan != "" {
			stats.FanRPM = fan + " RPM"
		}
	}

	// RAM / Swap
	meminfo, _ := ioutil.ReadFile("/proc/meminfo")
	var memTotal, memAvailable, swapTotal, swapFree float64
	for _, line := range strings.Split(string(meminfo), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			val, _ := strconv.ParseFloat(fields[1], 64)
			switch fields[0] {
			case "MemTotal:": memTotal = val
			case "MemAvailable:": memAvailable = val
			case "SwapTotal:": swapTotal = val
			case "SwapFree:": swapFree = val
			}
		}
	}
	if memTotal > 0 {
		stats.RAMTotal = memTotal / 1024 / 1024
		stats.RAMUsed = (memTotal - memAvailable) / 1024 / 1024
		stats.RAMPct = (stats.RAMUsed / stats.RAMTotal) * 100
	}
	if swapTotal > 0 {
		stats.SwapTotal = swapTotal / 1024 / 1024
		stats.SwapUsed = (swapTotal - swapFree) / 1024 / 1024
		stats.SwapPct = (stats.SwapUsed / stats.SwapTotal) * 100
	}

	// Disk
	var stat syscall.Statfs_t
	syscall.Statfs("/", &stat)
	total := float64(stat.Blocks * uint64(stat.Bsize))
	free := float64(stat.Bavail * uint64(stat.Bsize))
	if total > 0 {
		stats.DiskTotal = total / 1024 / 1024 / 1024
		used := total - free
		stats.DiskPct = (used / total) * 100
	}

	// GPU
	gpuFreqs, _ := filepath.Glob("/sys/class/drm/card*/gt_act_freq_mhz")
	if len(gpuFreqs) > 0 {
		stats.GPUFreq = readLine(gpuFreqs[0]) + "MHz"
	}

	// Battery
	bats, _ := filepath.Glob("/sys/class/power_supply/BAT*")
	if len(bats) > 0 {
		bat := bats[0]
		stats.BatPct = readLine(filepath.Join(bat, "capacity")) + "%"
		status := readLine(filepath.Join(bat, "status"))
		stats.BatETA = status
	}

	// NIC
	nics, _ := filepath.Glob("/sys/class/net/*")
	for _, nic := range nics {
		base := filepath.Base(nic)
		if base != "lo" && base != "docker0" {
			operstate := readLine(filepath.Join(nic, "operstate"))
			if operstate == "up" {
				stats.NICFace = base
				speed := readLine(filepath.Join(nic, "speed"))
				if speed != "" {
					stats.NICSpeed = speed + "Mb/s"
				}
				if _, err := os.Stat(filepath.Join(nic, "wireless")); err == nil {
					stats.NICType = "WiFi"
				} else {
					stats.NICType = "Ethernet"
				}
				break
			}
		}
	}

	return stats
}

func Render(tr *tracker.Tracker, primaryURL, fallbackURL, ipURL string, version string) {
	fmt.Print("\033[?25l") // Hide cursor
	
	var prevStats *SysStats
	
	for {
		stats := GetSysStats(prevStats)
		prevStats = stats
		
		var out strings.Builder
		out.WriteString("\033[H") // Home

		// Header
		out.WriteString("\033[38;5;196m╭─────────────────────────────────────────────────────────────╮\033[0m\n")
		out.WriteString("\033[38;5;196m│\033[0m \033[1;31m ██████╗ ██╗███████╗██████╗  ██████╗ ███████╗████████╗\033[0m     \033[38;5;196m│\033[0m\n")
		out.WriteString("\033[38;5;196m│\033[0m \033[1;31m ██╔══██╗██║██╔════╝██╔══██╗██╔═══██╗██╔════╝╚══██╔══╝\033[0m     \033[38;5;196m│\033[0m\n")
		out.WriteString("\033[38;5;196m│\033[0m \033[1;31m ██████╔╝██║█████╗  ██████╔╝██║   ██║███████╗   ██║   \033[0m     \033[38;5;196m│\033[0m\n")
		out.WriteString("\033[38;5;196m│\033[0m \033[1;31m ██╔══██╗██║██╔══╝  ██╔══██╗██║   ██║╚════██║   ██║   \033[0m     \033[38;5;196m│\033[0m\n")
		out.WriteString("\033[38;5;196m│\033[0m \033[1;31m ██████╔╝██║██║     ██║  ██║╚██████╔╝███████║   ██║   \033[0m     \033[38;5;196m│\033[0m\n")
		out.WriteString(fmt.Sprintf("\033[38;5;196m│\033[0m  v%-6s │ %-38s [%s] \033[38;5;196m│\033[0m\n", version, primaryURL, time.Now().Format("15:04:05")))
		out.WriteString(fmt.Sprintf("\033[38;5;196m│\033[0m           │ %-34s (direct IP)  \033[38;5;196m│\033[0m\n", ipURL))
		if fallbackURL != "" {
			out.WriteString(fmt.Sprintf("\033[38;5;196m│\033[0m           │ %-34s (fallback)  \033[38;5;196m│\033[0m\n", fallbackURL))
		}
		out.WriteString("\033[38;5;196m╰─────────────────────────────────────────────────────────────╯\033[0m\n")

		// System
		out.WriteString("\033[38;5;196m╭── SYSTEM ───────────────────────────────────────────────────╮\033[0m\n")
		out.WriteString(fmt.Sprintf("\033[38;5;196m│\033[0m  CPU: %s %3.0f%% %-6s %-6s  RAM:  %s %.1f/%.1fG  \033[38;5;196m│\033[0m\n", bar(stats.CPUUsage, 10), stats.CPUUsage, stats.CPUFreq, stats.CPUTemp, bar(stats.RAMPct, 10), stats.RAMUsed, stats.RAMTotal))
		out.WriteString(fmt.Sprintf("\033[38;5;196m│\033[0m  GPU: %s %-6s %-6s      DISK: %s %.0fG R/W  \033[38;5;196m│\033[0m\n", bar(0, 10), stats.GPUFreq, stats.GPUTemp, bar(stats.DiskPct, 10), stats.DiskTotal))
		out.WriteString(fmt.Sprintf("\033[38;5;196m│\033[0m  FAN: %s %-12s   SWAP: %s %.1fG      \033[38;5;196m│\033[0m\n", bar(0, 10), stats.FanRPM, bar(stats.SwapPct, 10), stats.SwapTotal))
		out.WriteString(fmt.Sprintf("\033[38;5;196m│\033[0m  NIC: %-6s %-6s %-8s  BAT:  %-4s %-8s       \033[38;5;196m│\033[0m\n", stats.NICFace, stats.NICSpeed, stats.NICType, stats.BatPct, stats.BatETA))
		out.WriteString("\033[38;5;196m╰─────────────────────────────────────────────────────────────╯\033[0m\n")

		tr.RLock()
		
		var clients []*tracker.ClientInfo
		activeCount := 0
		var totalBandwidth float64
		for _, c := range tr.Clients {
			if c.Active {
				activeCount++
			}
			clients = append(clients, c)
		}
		
		sort.Slice(clients, func(i, j int) bool {
			return clients[i].LastSeen.After(clients[j].LastSeen)
		})

		out.WriteString(fmt.Sprintf("\033[38;5;196m╭── STUDENT STREAM MONITORING (%2d active) ───────────────────╮\033[0m\n", activeCount))
		out.WriteString("\033[38;5;196m│\033[0m \033[1;37mS\033[0m \033[38;5;196m│\033[0m \033[1;37m#\033[0m \033[38;5;196m│\033[0m \033[1;37mDEV\033[0m \033[38;5;196m│\033[0m \033[1;37mIP ADDRESS    \033[0m \033[38;5;196m│\033[0m \033[1;37mBANDWIDTH      \033[0m \033[38;5;196m│\033[0m \033[1;37mUPLINK  \033[0m \033[38;5;196m│\033[0m\n")

		for i := 0; i < len(clients) && i < 20; i++ {
			c := clients[i]
			
			status := "\033[90m○\033[0m"
			if c.Active { status = "\033[92m●\033[0m" }

			dev := "💻"
			if strings.Contains(strings.ToLower(c.DevType), "mobile") || strings.Contains(strings.ToLower(c.OS), "android") {
				dev = "📱"
			}

			bw := float64(c.Bytes - c.PrevBytes) / 1024.0 / 1024.0 // MB/s
			c.PrevBytes = c.Bytes
			totalBandwidth += bw
			
			bwBar := bar(math.Min(bw/10.0*100.0, 100), 7)
			
			uplink := float64(c.Bytes) / 1024.0 / 1024.0 // MB

			bg := ""
			if i%2 == 1 { bg = "\033[48;5;233m" }

			out.WriteString(fmt.Sprintf("%s\033[38;5;196m│\033[0m %s \033[38;5;196m│\033[0m %-1d \033[38;5;196m│\033[0m %s  \033[38;5;196m│\033[0m %-14s \033[38;5;196m│\033[0m %s %4.1fM/s \033[38;5;196m│\033[0m %6.1fM \033[38;5;196m│\033[0m\n", bg, status, i+1, dev, c.IP, bwBar, bw, uplink))
		}

		out.WriteString("\033[38;5;196m│ ──────────────────────────────────────────────────────────  │\033[0m\n")
		
		totalMB := float64(tr.TotalBytes) / 1024.0 / 1024.0
		out.WriteString(fmt.Sprintf("\033[1;48;5;232;38;5;196m│ Σ │   │     │                │ %s %4.1fM/s │ %6.1fM │\033[0m\n", bar(math.Min(totalBandwidth/50.0*100, 100), 7), totalBandwidth, totalMB))
		out.WriteString("\033[38;5;196m╰─────────────────────────────────────────────────────────────╯\033[0m\n")

		if len(tr.Rejections) > 0 {
			out.WriteString("\033[38;5;196m╭── REJECTED CLIENTS ─────────────────────────────────────────╮\033[0m\n")
			for _, r := range tr.Rejections {
				out.WriteString(fmt.Sprintf("\033[38;5;196m│\033[0m ⛔ \033[38;5;196m│\033[0m \033[38;5;196m%-14s\033[0m \033[38;5;196m│\033[0m \033[38;5;196m%-21s\033[0m \033[38;5;196m│\033[0m \033[38;5;196m%s\033[0m       \033[38;5;196m│\033[0m\n", r.IP, r.Reason, r.Time.Format("15:04:05")))
			}
			out.WriteString("\033[38;5;196m╰─────────────────────────────────────────────────────────────╯\033[0m\n")
		}

		tr.RUnlock()

		// Clear rest of screen
		out.WriteString("\033[J")
		fmt.Print(out.String())

		time.Sleep(1 * time.Second)
	}
}

func ClearScreen() {
	cmd := exec.Command("clear")
	cmd.Stdout = os.Stdout
	cmd.Run()
}
