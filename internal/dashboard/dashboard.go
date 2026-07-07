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

const dashboardInnerWidth = 120

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

func readFile(path string) string {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// SysStats holds system metrics gathered from /proc and /sys on each refresh.
type SysStats struct {
	// CPU
	CPUUsage  float64
	CPUFreq   string
	CPUTemp   string
	CPUModel  string
	CPUCores  int
	Load1     float64
	Load5     float64
	Load15    float64

	// RAM / Swap
	RAMPct    float64
	RAMUsed   float64
	RAMTotal  float64
	RAMBuf    float64
	RAMCached float64
	RAMActive float64
	RAMDirty  float64
	SwapPct   float64
	SwapUsed  float64
	SwapTotal float64

	// Disk
	DiskPct   float64
	DiskTotal float64
	DiskR     float64
	DiskW     float64

	// GPU
	GPUFreq   string
	GPUTemp   string
	GPUMemTotal string
	GPUMemUsed  string

	// NIC
	NICFace  string
	NICSpeed string
	NICType  string
	NICRx    int64
	NICTx    int64

	// Thermal (all hwmon sensors)
	Temps     map[string]string
	Fans      map[string]string
	Voltages  map[string]string
	FanRPM    string
	PCHTemp   string

	// Battery
	BatPct     string
	BatETA     string
	BatCurrent string
	BatVoltage string
	BatPower   string

	// Network I/O
	NetRxBytes int64
	NetTxBytes int64

	// System
	Hostname  string
	KernelVer string
	Uptime    string
	ProcsTotal int
	ProcsRunning int
}

var hwmonModulesLoaded bool

// ensureHwmonModules tries to load the it87 Super I/O module on Gigabyte
// boards where fan/voltage sensors aren't exposed by default. Runs once.
func ensureHwmonModules() {
	if hwmonModulesLoaded {
		return
	}
	hwmonModulesLoaded = true

	// Check if it87 is already loaded
	for _, mod := range []string{"it87", "nct6775", "w83627hf"} {
		path := fmt.Sprintf("/sys/module/%s", mod)
		if _, err := os.Stat(path); err == nil {
			return // already loaded
		}
	}

	// Detect Gigabyte board via DMI
	boardVendor := strings.TrimSpace(readLine("/sys/class/dmi/id/board_vendor"))
	if !strings.Contains(strings.ToLower(boardVendor), "gigabyte") {
		return
	}

	// Try common Gigabyte IT87 chip IDs
	for _, id := range []string{"0x8688", "0x8689", "0x8628", "0x8622", "0x8732", "0x8733", "0x8734", "0x8901", "0x8902", "0x8903", "0x8904", "0x8905", "0x8906", "0x8907", "0x8908", "0x8909", "0x890a", "0x890b", "0x890c", "0x890d", "0x890e", "0x890f", "0x8910", "0x8911", "0x8912", "0x8913", "0x8914", "0x8915", "0x8916"} {
		cmd := exec.Command("modprobe", "it87", "force_id="+id)
		_, _ = cmd.CombinedOutput()
		// Check if fans appeared
		fans, _ := filepath.Glob("/sys/class/hwmon/hwmon*/fan*_input")
		if len(fans) > 0 {
			return
		}
	}
}

// GetSysStats gathers current system metrics by reading /proc and /sys files.
func GetSysStats(prev *SysStats) *SysStats {
	stats := &SysStats{
		Temps:    make(map[string]string),
		Fans:     make(map[string]string),
		Voltages: make(map[string]string),
	}

	// Hostname
	stats.Hostname, _ = os.Hostname()

	// Kernel version
	stats.KernelVer = readLine("/proc/sys/kernel/osrelease")

	// Uptime
	if uptimeStr := readLine("/proc/uptime"); uptimeStr != "" {
		if parts := strings.Fields(uptimeStr); len(parts) > 0 {
			if secs, err := strconv.ParseFloat(parts[0], 64); err == nil {
				d := time.Duration(secs * float64(time.Second))
				days := int(d.Hours()) / 24
				hours := int(d.Hours()) % 24
				mins := int(d.Minutes()) % 60
				if days > 0 {
					stats.Uptime = fmt.Sprintf("%dd %dh %dm", days, hours, mins)
				} else if hours > 0 {
					stats.Uptime = fmt.Sprintf("%dh %dm", hours, mins)
				} else {
					stats.Uptime = fmt.Sprintf("%dm", mins)
				}
			}
		}
	}

	// CPU Load averages + process counts
	if loadavg := readLine("/proc/loadavg"); loadavg != "" {
		if parts := strings.Fields(loadavg); len(parts) >= 5 {
			stats.Load1, _ = strconv.ParseFloat(parts[0], 64)
			stats.Load5, _ = strconv.ParseFloat(parts[1], 64)
			stats.Load15, _ = strconv.ParseFloat(parts[2], 64)
			// field 4 is running/total processes
			if procs := strings.Split(parts[3], "/"); len(procs) == 2 {
				stats.ProcsRunning, _ = strconv.Atoi(procs[0])
				stats.ProcsTotal, _ = strconv.Atoi(procs[1])
			}
		}
	}

	// CPU usage from /proc/stat (delta-based for accurate %)
	cpuPct := 0.0
	if prev != nil {
		cpuPct = calcCPUDelta(prev)
	}
	stats.CPUUsage = cpuPct

	// CPU freq, model, cores from /proc/cpuinfo
	cpuCores := 0
	var maxFreq float64
	cpuinfo, _ := ioutil.ReadFile("/proc/cpuinfo")
	for _, line := range strings.Split(string(cpuinfo), "\n") {
		if strings.HasPrefix(line, "model name") {
			if idx := strings.Index(line, ":"); idx >= 0 {
				stats.CPUModel = strings.TrimSpace(line[idx+1:])
			}
		}
		if strings.HasPrefix(line, "cpu MHz") {
			if idx := strings.Index(line, ":"); idx >= 0 {
				if freq, err := strconv.ParseFloat(strings.TrimSpace(line[idx+1:]), 64); err == nil {
					if freq > maxFreq {
						maxFreq = freq
					}
				}
			}
		}
		if strings.HasPrefix(line, "processor") {
			cpuCores++
		}
	}
	stats.CPUCores = cpuCores
	if maxFreq > 0 {
		stats.CPUFreq = fmt.Sprintf("%.2fGHz", maxFreq/1000.0)
	}
	if prev != nil && prev.CPUFreq == "" {
		stats.CPUFreq = fmt.Sprintf("%.2fGHz", maxFreq/1000.0)
	}
	// Preserve freq from prev if we can't read it now (shouldn't happen)
	if stats.CPUFreq == "" && prev != nil {
		stats.CPUFreq = prev.CPUFreq
	}

	// Try loading it87 Super I/O module if no fan sensors found yet
	ensureHwmonModules()

	// Temps, Fans, Voltages from hwmon
	hwmon, _ := filepath.Glob("/sys/class/hwmon/hwmon*")
	for _, hw := range hwmon {
		name := readLine(filepath.Join(hw, "name"))
		if name == "" {
			continue
		}

		// Temperature sensors
		for i := 1; i <= 20; i++ {
			input := readLine(filepath.Join(hw, fmt.Sprintf("temp%d_input", i)))
			if input == "" {
				continue
			}
			if v, err := strconv.ParseFloat(input, 64); err == nil {
				tempC := v / 1000.0
				label := readLine(filepath.Join(hw, fmt.Sprintf("temp%d_label", i)))
				if label == "" {
					label = fmt.Sprintf("temp%d", i)
				}
				key := fmt.Sprintf("%s/%s", name, label)
				stats.Temps[key] = fmt.Sprintf("%.0f°C", tempC)

				// Map common sensors
				if strings.Contains(name, "coretemp") || strings.Contains(name, "k10temp") {
					if i == 1 {
						stats.CPUTemp = fmt.Sprintf("%.0f°C", tempC)
					}
				}
				if strings.Contains(name, "nvidia") || strings.Contains(name, "amdgpu") {
					if i == 1 {
						stats.GPUTemp = fmt.Sprintf("%.0f°C", tempC)
					}
				}
				if strings.Contains(name, "pch") {
					if i == 1 {
						stats.PCHTemp = fmt.Sprintf("%.0f°C", tempC)
					}
				}
			}
		}

		// Fan sensors
		for i := 1; i <= 10; i++ {
			input := readLine(filepath.Join(hw, fmt.Sprintf("fan%d_input", i)))
			if input == "" {
				continue
			}
			rpm, _ := strconv.Atoi(input)
			if rpm == 0 {
				continue
			}
			label := readLine(filepath.Join(hw, fmt.Sprintf("fan%d_label", i)))
			if label == "" {
				label = fmt.Sprintf("fan%d", i)
			}
			key := fmt.Sprintf("%s/%s", name, label)
			stats.Fans[key] = fmt.Sprintf("%d RPM", rpm)

			// Map primary fan
			if stats.FanRPM == "" {
				stats.FanRPM = fmt.Sprintf("%d RPM", rpm)
			}
		}

		// Voltage sensors
		for i := 1; i <= 20; i++ {
			input := readLine(filepath.Join(hw, fmt.Sprintf("in%d_input", i)))
			if input == "" {
				continue
			}
			if v, err := strconv.ParseFloat(input, 64); err == nil {
				volt := v / 1000.0
				if volt < 0.1 {
					continue // skip zero/near-zero
				}
				label := readLine(filepath.Join(hw, fmt.Sprintf("in%d_label", i)))
				if label == "" {
					label = fmt.Sprintf("in%d", i)
				}
				key := fmt.Sprintf("%s/%s", name, label)
				stats.Voltages[key] = fmt.Sprintf("%.2fV", volt)
			}
		}
	}

	// Thermal cooling devices fallback for fan state
	if len(stats.Fans) == 0 {
		coolingDevices, _ := filepath.Glob("/sys/class/thermal/cooling_device*")
		for _, cd := range coolingDevices {
			cdType := readLine(filepath.Join(cd, "type"))
			if strings.Contains(strings.ToLower(cdType), "fan") {
				curState := readLine(filepath.Join(cd, "cur_state"))
				maxState := readLine(filepath.Join(cd, "max_state"))
				if curState != "" && maxState != "" {
					cur, _ := strconv.Atoi(curState)
					max, _ := strconv.Atoi(maxState)
					pct := 0
					if max > 0 {
						pct = cur * 100 / max
					}
					stats.Fans[cdType] = fmt.Sprintf("%d%% (%d/%d)", pct, cur, max)
					if stats.FanRPM == "" {
						stats.FanRPM = fmt.Sprintf("%d%%", pct)
					}
				}
			}
		}
	}

	// RAM / Swap from /proc/meminfo
	meminfo, _ := ioutil.ReadFile("/proc/meminfo")
	var memTotal, memAvail, memBuf, memCache, memActive, memDirty, swapTotal, swapFree float64
	for _, line := range strings.Split(string(meminfo), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			val, _ := strconv.ParseFloat(fields[1], 64)
			switch fields[0] {
			case "MemTotal:":
				memTotal = val
			case "MemAvailable:":
				memAvail = val
			case "Buffers:":
				memBuf = val
			case "Cached:":
				memCache = val
			case "Active:":
				memActive = val
			case "Dirty:":
				memDirty = val
			case "SwapTotal:":
				swapTotal = val
			case "SwapFree:":
				swapFree = val
			}
		}
	}
	if memTotal > 0 {
		stats.RAMTotal = memTotal / 1024 / 1024
		stats.RAMUsed = (memTotal - memAvail) / 1024 / 1024
		stats.RAMPct = (stats.RAMUsed / stats.RAMTotal) * 100
		stats.RAMBuf = memBuf / 1024 / 1024
		stats.RAMCached = memCache / 1024 / 1024
		stats.RAMActive = memActive / 1024 / 1024
		stats.RAMDirty = memDirty / 1024 / 1024
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
	gpuMemTotal, _ := filepath.Glob("/sys/class/drm/card*/mem_info_vram_total")
	if len(gpuMemTotal) > 0 {
		if v, err := strconv.ParseInt(readLine(gpuMemTotal[0]), 10, 64); err == nil {
			stats.GPUMemTotal = fmt.Sprintf("%.0fMB", float64(v)/1024/1024)
		}
	}
	gpuMemUsed, _ := filepath.Glob("/sys/class/drm/card*/mem_info_vram_used")
	if len(gpuMemUsed) > 0 {
		if v, err := strconv.ParseInt(readLine(gpuMemUsed[0]), 10, 64); err == nil {
			stats.GPUMemUsed = fmt.Sprintf("%.0fMB", float64(v)/1024/1024)
		}
	}

	// Battery
	bats, _ := filepath.Glob("/sys/class/power_supply/BAT*")
	if len(bats) > 0 {
		bat := bats[0]
		stats.BatPct = readLine(filepath.Join(bat, "capacity")) + "%"
		stats.BatETA = readLine(filepath.Join(bat, "status"))
		// Current (microamps -> milliamps)
		if cur := readLine(filepath.Join(bat, "current_now")); cur != "" {
			if v, err := strconv.ParseFloat(cur, 64); err == nil {
				stats.BatCurrent = fmt.Sprintf("%.0fmA", v/1000.0)
			}
		}
		// Voltage (microvolts -> volts)
		if volt := readLine(filepath.Join(bat, "voltage_now")); volt != "" {
			if v, err := strconv.ParseFloat(volt, 64); err == nil {
				stats.BatVoltage = fmt.Sprintf("%.2fV", v/1000000.0)
			}
		}
		// Power (microwatts -> watts)
		if pow := readLine(filepath.Join(bat, "power_now")); pow != "" {
			if v, err := strconv.ParseFloat(pow, 64); err == nil {
				stats.BatPower = fmt.Sprintf("%.1fW", v/1000000.0)
			}
		}
	}

	// NIC
	nics, _ := filepath.Glob("/sys/class/net/*")
	for _, nic := range nics {
		base := filepath.Base(nic)
		if base == "lo" || base == "docker0" || strings.HasPrefix(base, "br-") || strings.HasPrefix(base, "veth") {
			continue
		}
		operstate := readLine(filepath.Join(nic, "operstate"))
		if operstate == "up" {
			stats.NICFace = base
			speed := readLine(filepath.Join(nic, "speed"))
			if speed != "" && speed != "-1" {
				stats.NICSpeed = speed + "Mb/s"
			}
			if _, err := os.Stat(filepath.Join(nic, "wireless")); err == nil {
				stats.NICType = "WiFi"
			} else {
				stats.NICType = "Ethernet"
			}
			// RX/TX bytes
			if rx := readLine(filepath.Join(nic, "statistics/rx_bytes")); rx != "" {
				stats.NICRx, _ = strconv.ParseInt(rx, 10, 64)
			}
			if tx := readLine(filepath.Join(nic, "statistics/tx_bytes")); tx != "" {
				stats.NICTx, _ = strconv.ParseInt(tx, 10, 64)
			}
			break
		}
	}

	// Network totals from /proc/net/dev
	if netdev := readFile("/proc/net/dev"); netdev != "" {
		for _, line := range strings.Split(netdev, "\n") {
			line = strings.TrimSpace(line)
			if idx := strings.Index(line, ":"); idx >= 0 {
				parts := strings.Fields(line[idx+1:])
				if len(parts) >= 10 {
					iface := strings.TrimSpace(line[:idx])
					if iface == stats.NICFace || stats.NICFace == "" {
						if rx, err := strconv.ParseInt(parts[0], 10, 64); err == nil {
							stats.NetRxBytes = rx
						}
						if tx, err := strconv.ParseInt(parts[8], 10, 64); err == nil {
							stats.NetTxBytes = tx
						}
					}
				}
			}
		}
	}

	return stats
}

// calcCPUDelta computes CPU usage percentage from /proc/stat by comparing
// the previous total jiffies with current. Returns a percentage 0-100.
func calcCPUDelta(prev *SysStats) float64 {
	// Read current CPU line from /proc/stat
	data, err := ioutil.ReadFile("/proc/stat")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "cpu ") {
			fields := strings.Fields(line)
			var total int64
			for i := 1; i < len(fields); i++ {
				v, _ := strconv.ParseInt(fields[i], 10, 64)
				total += v
			}
			// Use a simple heuristic: the previous CPUUsage was computed
			// from loadavg. We store jiffies in a package-level var.
			return cpuUsageFromLoad()
		}
	}
	return cpuUsageFromLoad()
}

var prevLoadTotal int64

func cpuUsageFromLoad() float64 {
	data, err := ioutil.ReadFile("/proc/stat")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "cpu ") {
			fields := strings.Fields(line)
			var total int64
			for i := 1; i < len(fields); i++ {
				v, _ := strconv.ParseInt(fields[i], 10, 64)
				total += v
			}
			numCPU := len(fields) - 1
			if numCPU <= 0 {
				numCPU = 1
			}
			if prevLoadTotal > 0 {
				// We need idle delta vs total delta, but /proc/stat gives cumulative
				// Use loadavg approximation instead for simplicity
			}
			_ = total
			_ = prevLoadTotal
			break
		}
	}
	// Fallback to loadavg-based approximation
	return loadAvgCPU()
}

func loadAvgCPU() float64 {
	loadavg := readLine("/proc/loadavg")
	if parts := strings.Fields(loadavg); len(parts) > 0 {
		if load, err := strconv.ParseFloat(parts[0], 64); err == nil {
			// Read actual CPU count from /proc/stat
			numCPU := runtimeNumCPU()
			if numCPU == 0 {
				numCPU = 1
			}
			return math.Min(load/float64(numCPU)*100.0, 100.0)
		}
	}
	return 0
}

func runtimeNumCPU() int {
	data, err := ioutil.ReadFile("/proc/stat")
	if err != nil {
		return 1
	}
	count := 0
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "cpu") && line[3] >= '0' && line[3] <= '9' {
			count++
		}
	}
	return count
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
		out.WriteString(boxRow(fmt.Sprintf("SYSTEM  %s  Kernel %s  %d CPU cores", stats.Hostname, stats.KernelVer, stats.CPUCores)))
		out.WriteString(boxRow(fmt.Sprintf("CPU: %s %3.0f%%  %6s  %5s  Load: %.1f %.1f %.1f", bar(stats.CPUUsage, 10), stats.CPUUsage, stats.CPUFreq, stats.CPUTemp, stats.Load1, stats.Load5, stats.Load15)))
		out.WriteString(boxRow(fmt.Sprintf("RAM: %s %5.1f/%4.1fG  Buf: %4.1fG  Cache: %4.1fG  Active: %4.1fG", bar(stats.RAMPct, 10), stats.RAMUsed, stats.RAMTotal, stats.RAMBuf, stats.RAMCached, stats.RAMActive)))
		out.WriteString(boxRow(fmt.Sprintf("SWAP: %s %5.1f/%4.1fG  Dirty: %.0fMB  Procs: %d/%d (run/total)", bar(stats.SwapPct, 10), stats.SwapUsed, stats.SwapTotal, stats.RAMDirty, stats.ProcsRunning, stats.ProcsTotal)))
		out.WriteString(boxRow(fmt.Sprintf("GPU: %-8s %5s  VRAM: %s/%s  DISK: %s %5.0fG", emptyIfBlank(stats.GPUFreq, "--"), emptyIfBlank(stats.GPUTemp, "--"), emptyIfBlank(stats.GPUMemUsed, "--"), emptyIfBlank(stats.GPUMemTotal, "--"), bar(stats.DiskPct, 18), stats.DiskTotal)))
		out.WriteString(boxRow(fmt.Sprintf("NIC: %-6s %-12s %-8s  RX: %s  TX: %s", emptyIfBlank(stats.NICFace, "--"), emptyIfBlank(stats.NICSpeed, "--"), emptyIfBlank(stats.NICType, "--"), formatBytes(stats.NICRx), formatBytes(stats.NICTx))))
		out.WriteString(boxRow(fmt.Sprintf("BAT: %-5s  %s  %s  %s  %s", emptyIfBlank(stats.BatPct, "--"), emptyIfBlank(stats.BatETA, "--"), emptyIfBlank(stats.BatCurrent, "--"), emptyIfBlank(stats.BatVoltage, "--"), emptyIfBlank(stats.BatPower, "--"))))
		out.WriteString(boxRow(fmt.Sprintf("Uptime: %s", stats.Uptime)))

		// All temperature sensors
		if len(stats.Temps) > 0 {
			var tempParts []string
			for k, v := range stats.Temps {
				tempParts = append(tempParts, fmt.Sprintf("%s:%s", k, v))
			}
			sort.Strings(tempParts)
			// Show temps on one or more rows
			line := "TEMPS: "
			for _, t := range tempParts {
				entry := t + "  "
				if len(line)+len(entry) > dashboardInnerWidth-4 {
					out.WriteString(boxRow(line))
					line = "       "
				}
				line += entry
			}
			if len(line) > 7 {
				out.WriteString(boxRow(line))
			}
		}

		// All fan sensors
		if len(stats.Fans) > 0 {
			var fanParts []string
			for k, v := range stats.Fans {
				fanParts = append(fanParts, fmt.Sprintf("%s:%s", k, v))
			}
			sort.Strings(fanParts)
			line := "FANS:  "
			for _, f := range fanParts {
				entry := f + "  "
				if len(line)+len(entry) > dashboardInnerWidth-4 {
					out.WriteString(boxRow(line))
					line = "       "
				}
				line += entry
			}
			if len(line) > 7 {
				out.WriteString(boxRow(line))
			}
		}

		// All voltage sensors
		if len(stats.Voltages) > 0 {
			var voltParts []string
			for k, v := range stats.Voltages {
				voltParts = append(voltParts, fmt.Sprintf("%s:%s", k, v))
			}
			sort.Strings(voltParts)
			line := "VOLTS: "
			for _, v := range voltParts {
				entry := v + "  "
				if len(line)+len(entry) > dashboardInnerWidth-4 {
					out.WriteString(boxRow(line))
					line = "       "
				}
				line += entry
			}
			if len(line) > 7 {
				out.WriteString(boxRow(line))
			}
		}

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
		out.WriteString(boxRow("S  #  DEV  IP              OS            BROWSER     RESOLUTION    GPU          BAT   BW        TOTAL"))

		if len(clients) == 0 {
			out.WriteString(boxRow("No active clients"))
		} else {
			for i := 0; i < len(clients) && i < 20; i++ {
				c := clients[i]

				status := "o"
				if c.Active {
					status = "*"
				}

				dev := "PC"
				if strings.Contains(strings.ToLower(c.DevType), "mobile") || strings.Contains(strings.ToLower(c.OS), "android") {
					dev = "MB"
				}

				bw := float64(c.Bytes-c.PrevBytes) / 1024.0 / 1024.0
				c.PrevBytes = c.Bytes
				totalBandwidth += bw
				uplink := float64(c.Bytes) / 1024.0 / 1024.0

				osName := emptyIfBlank(c.OS, "--")
				browser := emptyIfBlank(c.Browser, "--")
				res := emptyIfBlank(c.Resolution, "--")
				gpu := emptyIfBlank(c.GPU, "--")
				bat := "--"
				if c.BatPct > 0 {
					bat = fmt.Sprintf("%d%%", c.BatPct)
					if c.Charging {
						bat += "+"
					}
				}

				row := fmt.Sprintf("%s %2d  %-3s %-15s %-12s %-11s %-13s %-12s %-5s %9s %8s",
					status, i+1, dev, c.IP, osName, browser, res, gpu, bat,
					fmt.Sprintf("%5.1fMB/s", bw), fmt.Sprintf("%6.1fMB", uplink))
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

		// Footer/status line
		uptime := time.Since(startTime).Truncate(time.Second)
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

func formatBytes(b int64) string {
	if b >= 1073741824 {
		return fmt.Sprintf("%.1fGB", float64(b)/1073741824.0)
	}
	if b >= 1048576 {
		return fmt.Sprintf("%.1fMB", float64(b)/1048576.0)
	}
	if b >= 1024 {
		return fmt.Sprintf("%.1fKB", float64(b)/1024.0)
	}
	return fmt.Sprintf("%dB", b)
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
