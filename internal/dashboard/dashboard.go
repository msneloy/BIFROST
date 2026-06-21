package dashboard

import (
	"bifrost/internal/stream"
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

// bar renders a text progress bar of the given width using filled (█) and
// empty (░) block characters based on the percentage value.
func bar(pct float64, width int) string {
	filled := int(math.Round(pct / 100.0 * float64(width)))
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

const dashboardInnerWidth = 66

var startTime = time.Now()

func padRight(value string, width int) string {
	if len(value) > width {
		return value[:width]
	}
	return fmt.Sprintf("%-*s", width, value)
}

func boxTop() string {
	return fmt.Sprintf("\033[38;5;196m╭%s╮\033[0m\n", strings.Repeat("─", dashboardInnerWidth))
}

func boxBottom() string {
	return fmt.Sprintf("\033[38;5;196m╰%s╯\033[0m\n", strings.Repeat("─", dashboardInnerWidth))
}

func boxRow(content string) string {
	return fmt.Sprintf("\033[38;5;196m│\033[0m %s \033[38;5;196m│\033[0m\n", padRight(content, dashboardInnerWidth-2))
}

func boxSeparator() string {
	return fmt.Sprintf("\033[38;5;196m│%s│\033[0m\n", strings.Repeat("─", dashboardInnerWidth))
}

func boxFooter(content string) string {
	return fmt.Sprintf("\033[48;5;232;38;5;196m│\033[0m %s \033[48;5;232;38;5;196m│\033[0m\n", padRight(content, dashboardInnerWidth-2))
}

func readLine(path string) string {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(strings.Split(string(data), "\n")[0])
}

// SysStats holds system metrics gathered from /proc and /sys on each refresh.
type SysStats struct {
	CPUUsage  float64
	CPUFreq   string
	CPUTemp   string
	RAMPct    float64
	RAMUsed   float64
	RAMTotal  float64
	SwapPct   float64
	SwapUsed  float64
	SwapTotal float64
	DiskPct   float64
	DiskTotal float64
	DiskR     float64
	DiskW     float64
	GPUFreq   string
	GPUTemp   string
	FanRPM    string
	PCHTemp   string
	NICFace   string
	NICSpeed  string
	NICType   string
	BatPct    string
	BatETA    string
}

// GetSysStats gathers current system metrics by reading /proc and /sys files.
// CPU usage is approximated from /proc/loadavg assuming 8 cores. Hardware
// temperatures and fan speeds come from /sys/class/hwmon. Previous stats
// are passed for delta calculations (bandwidth).
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
			case "MemTotal:":
				memTotal = val
			case "MemAvailable:":
				memAvailable = val
			case "SwapTotal:":
				swapTotal = val
			case "SwapFree:":
				swapFree = val
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

// Start runs the terminal dashboard in an infinite loop, refreshing every
// second. It renders the BIFROST banner, system stats, connected client
// table, rejection log, and footer with uptime and totals.
func Start(tr *tracker.Tracker, broadcaster *stream.Broadcaster, ip string, version string) {
	primaryURL := fmt.Sprintf("http://bifrost.local:8080")
	fallbackURL := ""
	ipURL := fmt.Sprintf("http://%s:8080", ip)
	fmt.Print("\033[?25l") // Hide cursor

	var prevStats *SysStats

	for {
		stats := GetSysStats(prevStats)
		prevStats = stats

		var out strings.Builder
		out.WriteString("\033[H") // Home

		// Header
		out.WriteString(boxTop())
		out.WriteString(boxRow(fmt.Sprintf("BIFROST v%s", version)))
		out.WriteString(boxRow("Browser Integrated Feed for Remote Observation"))
		out.WriteString(boxRow("& Screen Transmission"))
		out.WriteString(boxRow(primaryURL))
		out.WriteString(boxRow(ipURL + " (direct IP)"))
		if fallbackURL != "" {
			out.WriteString(boxRow(fallbackURL + " (fallback)"))
		}
		out.WriteString(boxBottom())

		// System
		out.WriteString(boxTop())
		out.WriteString(boxRow("SYSTEM"))
		out.WriteString(boxRow(fmt.Sprintf("CPU: %s %3.0f%%  %6s  %5s", bar(stats.CPUUsage, 10), stats.CPUUsage, stats.CPUFreq, stats.CPUTemp)))
		out.WriteString(boxRow(fmt.Sprintf("RAM: %s %5.1f/%4.1fG  SWAP: %s %5.1fG", bar(stats.RAMPct, 10), stats.RAMUsed, stats.RAMTotal, bar(stats.SwapPct, 10), stats.SwapTotal)))
		out.WriteString(boxRow(fmt.Sprintf("GPU: %-8s %5s  DISK: %s %5.0fG", emptyIfBlank(stats.GPUFreq, "--"), emptyIfBlank(stats.GPUTemp, "--"), bar(stats.DiskPct, 18), stats.DiskTotal)))
		out.WriteString(boxRow(fmt.Sprintf("NIC: %-6s %-12s %-8s  FAN: %-12s", emptyIfBlank(stats.NICFace, "--"), emptyIfBlank(stats.NICSpeed, "--"), emptyIfBlank(stats.NICType, "--"), emptyIfBlank(stats.FanRPM, "--"))))
		out.WriteString(boxRow(fmt.Sprintf("BAT: %-5s  %s", emptyIfBlank(stats.BatPct, "--"), emptyIfBlank(stats.BatETA, "--"))))
		out.WriteString(boxBottom())

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

		out.WriteString(boxTop())
		out.WriteString(boxRow(fmt.Sprintf("STUDENT STREAM MONITORING (%d active)", activeCount)))
		out.WriteString(boxRow("S  #  DEV  IP ADDRESS       BANDWIDTH     UPLINK"))

		if len(clients) == 0 {
			out.WriteString(boxRow("No active clients"))
		} else {
			for i := 0; i < len(clients) && i < 20; i++ {
				c := clients[i]

				// Use plain ASCII symbols for alignment (avoid ANSI/emoji in padded content)
				status := "o"
				if c.Active {
					status = "*"
				}

				dev := "PC"
				if strings.Contains(strings.ToLower(c.DevType), "mobile") || strings.Contains(strings.ToLower(c.OS), "android") {
					dev = "MB"
				}

				bw := float64(c.Bytes-c.PrevBytes) / 1024.0 / 1024.0 // MB/s
				c.PrevBytes = c.Bytes
				totalBandwidth += bw
				uplink := float64(c.Bytes) / 1024.0 / 1024.0 // MB

				row := fmt.Sprintf("%s %2d  %-3s %-15s %9s %8s",
					status, i+1, dev, c.IP, fmt.Sprintf("%5.1fMB/s", bw), fmt.Sprintf("%6.1fMB", uplink))
				out.WriteString(fmt.Sprintf("\033[38;5;196m│\033[0m %s \033[38;5;196m│\033[0m\n", padRight(row, dashboardInnerWidth-2)))
			}
		}

		out.WriteString(boxSeparator())
		totalMB := float64(tr.TotalBytes) / 1024.0 / 1024.0
		totalPubMB := float64(broadcaster.Total) / 1024.0 / 1024.0
		pubRate := float64(broadcaster.GetPubRate()) / 1024.0 / 1024.0
		out.WriteString(boxRow(fmt.Sprintf("Σ %6.1fMB (Pub: %6.1fMB) R:%4.1fMB/s", totalMB, totalPubMB, pubRate)))
		out.WriteString(boxBottom())

		if len(tr.Rejections) > 0 {
			out.WriteString(fmt.Sprintf("\033[38;5;196m╭── REJECTED CLIENTS %s╮\033[0m\n", strings.Repeat("─", dashboardInnerWidth-18)))
			for _, r := range tr.Rejections {
				out.WriteString(boxRow(fmt.Sprintf("⛔ %-15s %-22s %s", r.IP, r.Reason, r.Time.Format("15:04:05"))))
			}
			out.WriteString(boxBottom())
		}

		tr.RUnlock()

		// Footer/status line — uptime, active clients, total MB, log size
		uptime := time.Since(startTime).Truncate(time.Second)
		// log file info
		logInfo := "no log"
		if fi, err := os.Stat("/tmp/bifrost.log"); err == nil {
			logInfo = fmt.Sprintf("log:%dKB", fi.Size()/1024)
		}
		footer := fmt.Sprintf("Up:%s  Clients:%d  Total:%s  %s  %s", uptime.String(), activeCount, fmt.Sprintf("%6.1fMB", totalMB), logInfo, time.Now().Format("15:04:05"))
		out.WriteString(boxFooter(footer))

		// Clear rest of screen
		out.WriteString("\033[J")
		fmt.Print(out.String())

		time.Sleep(1 * time.Second)
	}
}

func emptyIfBlank(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func ClearScreen() {
	cmd := exec.Command("clear")
	cmd.Stdout = os.Stdout
	cmd.Run()
}
