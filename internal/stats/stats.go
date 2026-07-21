package stats

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
)

type SystemStats struct {
	CPUUsage    int
	CPUFreqMHz  int
	CPUTempC    int
	CPUModel    string
	CPUCores    int
	LoadAvg     float64
	MemTotalGB  float64
	MemUsedGB   float64
	MemPct      int
	SwapTotalGB float64
	SwapUsedGB  float64
	SwapPct     int
	GPUFreqMHz  int
	GPUTempC    int
	DiskTotal   string
	DiskUsed    string
	DiskPct     int
	DiskModel   string
	NICName     string
	NICSpeed    int
	NICType     string
	BatPct      string
	BatStatus   string
	FanRPM      int
	Uptime      string
	UptimeSecs  int64
	BoardVendor string
	BoardModel  string
}

var prevIdle, prevTotal uint64

func Collect() *SystemStats {
	s := &SystemStats{}
	s.CPUUsage = getCPUUsage()
	s.CPUFreqMHz = getCPUFreq()
	s.CPUTempC = getCPUTemp()
	s.CPUModel = getCPUModel()
	s.CPUCores = getCPUCores()
	s.LoadAvg = getLoadAvg()
	s.MemTotalGB, s.MemUsedGB, s.MemPct = getMemInfo()
	s.SwapTotalGB, s.SwapUsedGB, s.SwapPct = getSwapInfo()
	s.GPUFreqMHz, s.GPUTempC = getGPUInfo()
	s.DiskTotal, s.DiskUsed, s.DiskPct = getDiskInfo()
	s.DiskModel = getDiskModel()
	s.NICName, s.NICSpeed, s.NICType = getNICInfo()
	s.BatPct, s.BatStatus = getBatteryInfo()
	s.FanRPM = getFanRPM()
	s.Uptime, s.UptimeSecs = getUptime()
	s.BoardVendor, s.BoardModel = getBoardInfo()
	return s
}

func readFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func readFileInt(path string) int {
	s := readFile(path)
	if s == "" {
		return 0
	}
	n, _ := strconv.Atoi(s)
	return n
}

func getCPUUsage() int {
	data := readFile("/proc/stat")
	if data == "" {
		return 0
	}
	lines := strings.Split(data, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "cpu ") {
			fields := strings.Fields(line)
			if len(fields) < 5 {
				return 0
			}
			var totals [4]uint64
			for i := 0; i < 4; i++ {
				totals[i], _ = strconv.ParseUint(fields[i+1], 10, 64)
			}
			idle := totals[3]
			total := totals[0] + totals[1] + totals[2] + totals[3]

			if prevTotal > 0 {
				dIdle := idle - prevIdle
				dTotal := total - prevTotal
				if dTotal > 0 {
					prevIdle = idle
					prevTotal = total
					return int(float64(dTotal-dIdle) / float64(dTotal) * 100)
				}
			}
			prevIdle = idle
			prevTotal = total
			return 0
		}
	}
	return 0
}

func getCPUFreq() int {
	return readFileInt("/sys/devices/system/cpu/cpu0/cpufreq/scaling_cur_freq") / 1000
}

func getCPUTemp() int {
	// Try thermal zones
	matches, _ := filepath.Glob("/sys/class/thermal/thermal_zone*/temp")
	for _, m := range matches {
		t := readFileInt(m)
		if t > 0 {
			return t / 1000
		}
	}
	// Try hwmon
	matches, _ = filepath.Glob("/sys/class/hwmon/hwmon*/temp*_input")
	for _, m := range matches {
		t := readFileInt(m)
		if t > 0 {
			return t / 1000
		}
	}
	return 0
}

func getCPUModel() string {
	data := readFile("/proc/cpuinfo")
	for _, line := range strings.Split(data, "\n") {
		if strings.HasPrefix(line, "model name") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return "Unknown"
}

func getCPUCores() int {
	data := readFile("/proc/cpuinfo")
	count := 0
	for _, line := range strings.Split(data, "\n") {
		if strings.HasPrefix(line, "processor") {
			count++
		}
	}
	if count == 0 {
		count = 1
	}
	return count
}

func getLoadAvg() float64 {
	data := readFile("/proc/loadavg")
	if data == "" {
		return 0
	}
	fields := strings.Fields(data)
	if len(fields) > 0 {
		f, _ := strconv.ParseFloat(fields[0], 64)
		return math.Round(f*100) / 100
	}
	return 0
}

func getMemInfo() (totalGB, usedGB float64, pct int) {
	data := readFile("/proc/meminfo")
	if data == "" {
		return
	}
	var memTotal, memAvail uint64
	for _, line := range strings.Split(data, "\n") {
		if strings.HasPrefix(line, "MemTotal:") {
			fmt.Sscanf(strings.TrimPrefix(line, "MemTotal:"), "%d kB", &memTotal)
		} else if strings.HasPrefix(line, "MemAvailable:") {
			fmt.Sscanf(strings.TrimPrefix(line, "MemAvailable:"), "%d kB", &memAvail)
		}
	}
	if memTotal == 0 {
		return
	}
	totalGB = float64(memTotal) / 1048576
	usedGB = float64(memTotal-memAvail) / 1048576
	pct = int(float64(memTotal-memAvail) / float64(memTotal) * 100)
	return
}

func getSwapInfo() (totalGB, usedGB float64, pct int) {
	data := readFile("/proc/meminfo")
	if data == "" {
		return
	}
	var swapTotal, swapFree uint64
	for _, line := range strings.Split(data, "\n") {
		if strings.HasPrefix(line, "SwapTotal:") {
			fmt.Sscanf(strings.TrimPrefix(line, "SwapTotal:"), "%d kB", &swapTotal)
		} else if strings.HasPrefix(line, "SwapFree:") {
			fmt.Sscanf(strings.TrimPrefix(line, "SwapFree:"), "%d kB", &swapFree)
		}
	}
	if swapTotal == 0 {
		return
	}
	totalGB = float64(swapTotal) / 1048576
	usedGB = float64(swapTotal-swapFree) / 1048576
	pct = int(float64(swapTotal-swapFree) / float64(swapTotal) * 100)
	return
}

func getGPUInfo() (freqMHz, tempC int) {
	// AMD GPU frequency
	matches, _ := filepath.Glob("/sys/class/drm/card*/device/pp_dpm_sclk")
	for _, m := range matches {
		data := readFile(m)
		for _, line := range strings.Split(data, "\n") {
			if strings.Contains(line, "*") {
				re := regexp.MustCompile(`(\d+)Mhz`)
				if matches := re.FindStringSubmatch(line); len(matches) > 1 {
					freqMHz, _ = strconv.Atoi(matches[1])
				}
			}
		}
	}
	// GPU temperature from hwmon
	matches, _ = filepath.Glob("/sys/class/hwmon/hwmon*/temp*_input")
	for _, m := range matches {
		label := strings.TrimSuffix(m, "_input") + "_label"
		data := readFile(label)
		if strings.ToLower(data) == "gpu" || strings.ToLower(data) == "edge" {
			t := readFileInt(m)
			if t > 0 {
				tempC = t / 1000
			}
		}
	}
	return
}

func getDiskInfo() (total, used string, pct int) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs("/", &stat); err != nil {
		return "N/A", "N/A", 0
	}
	totalBytes := stat.Blocks * uint64(stat.Bsize)
	freeBytes := stat.Bavail * uint64(stat.Bsize)
	usedBytes := totalBytes - freeBytes
	pct = int(float64(usedBytes) / float64(totalBytes) * 100)
	total = formatBytes(totalBytes)
	used = formatBytes(usedBytes)
	return
}

func getDiskModel() string {
	// Try reading from sysfs for the root disk
	matches, _ := filepath.Glob("/sys/block/*/device/model")
	for _, m := range matches {
		data := readFile(m)
		if data != "" {
			return data
		}
	}
	// Fallback: try lsblk
	out, err := exec.Command("lsblk", "-dno", "MODEL", "/dev/sda").Output()
	if err == nil {
		model := strings.TrimSpace(string(out))
		if model != "" {
			return model
		}
	}
	return "Unknown"
}

func getBoardInfo() (vendor, model string) {
	vendor = readFile("/sys/class/dmi/id/board_vendor")
	model = readFile("/sys/class/dmi/id/board_name")
	if vendor == "" {
		vendor = "Unknown"
	}
	if model == "" {
		model = "Unknown"
	}
	return
}

func getNICInfo() (name string, speed int, nicType string) {
	entries, err := os.ReadDir("/sys/class/net")
	if err != nil {
		return "N/A", 0, "N/A"
	}
	for _, entry := range entries {
		n := entry.Name()
		if n == "lo" || strings.HasPrefix(n, "docker") || strings.HasPrefix(n, "br-") || strings.HasPrefix(n, "veth") {
			continue
		}
		name = n
		// Speed
		speed = readFileInt("/sys/class/net/" + n + "/speed")
		if speed < 0 {
			speed = 0
		}
		// WiFi detection
		if _, err := os.Stat("/sys/class/net/" + n + "/wireless"); err == nil {
			nicType = "WiFi"
		} else if _, err := os.Stat("/sys/class/net/" + n + "/phy80211"); err == nil {
			nicType = "WiFi"
		} else {
			nicType = "Ethernet"
		}
		break
	}
	return
}

func getBatteryInfo() (pct, status string) {
	matches, _ := filepath.Glob("/sys/class/power_supply/BAT*/capacity")
	if len(matches) == 0 {
		return "N/A", "N/A"
	}
	pct = readFile(matches[0])
	statusFile := strings.TrimSuffix(matches[0], "capacity") + "status"
	status = readFile(statusFile)
	return
}

func getFanRPM() int {
	matches, _ := filepath.Glob("/sys/class/hwmon/hwmon*/fan*_input")
	for _, m := range matches {
		rpm := readFileInt(m)
		if rpm > 0 {
			return rpm
		}
	}
	return 0
}

func getUptime() (string, int64) {
	data := readFile("/proc/uptime")
	if data == "" {
		return "N/A", 0
	}
	fields := strings.Fields(data)
	if len(fields) == 0 {
		return "N/A", 0
	}
	secs, _ := strconv.ParseFloat(fields[0], 64)
	totalSecs := int64(secs)
	days := totalSecs / 86400
	hours := (totalSecs % 86400) / 3600
	mins := (totalSecs % 3600) / 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh", days, hours), totalSecs
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, mins), totalSecs
	}
	return fmt.Sprintf("%dm", mins), totalSecs
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
	return fmt.Sprintf("%.0f%c", float64(b)/float64(div), "KMGTPE"[exp])
}
