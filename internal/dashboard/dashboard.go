package dashboard

import (
	"fmt"
	"strings"
	"time"

	"github.com/nelobster/bifrost/internal/stats"
	"github.com/nelobster/bifrost/internal/tracker"
)

const (
	width     = 120
	innerWidth = 116
)

func Run(ip, port, version string, statsCollector func() *stats.SystemStats, trk *tracker.Tracker) {
	for {
		s := statsCollector()
		render(s, ip, port, version, trk)
		time.Sleep(1 * time.Second)
	}
}

func render(s *stats.SystemStats, ip, port, version string, trk *tracker.Tracker) {
	now := time.Now().Format("15:04:05")
	var b strings.Builder

	// Clear screen
	b.WriteString("\033[H\033[2J")

	// Title
	b.WriteString(fmt.Sprintf("\033[1;31m  BIFROST v%s\033[0m", version))
	b.WriteString(fmt.Sprintf("  \033[90m|\033[0m  %s:%s", ip, port))
	b.WriteString(fmt.Sprintf("  \033[90m|\033[0m  [%s]", now))
	b.WriteString(fmt.Sprintf("  \033[90m|\033[0m  uptime: %s", s.Uptime))
	b.WriteString("\n\n")

	// System box
	b.WriteString(boxTop("SYSTEM"))
	b.WriteString(boxLine(fmt.Sprintf("CPU:  %s %3d%%  %dMHz  %d°C", bar(s.CPUUsage), s.CPUUsage, s.CPUFreqMHz, s.CPUTempC)))
	b.WriteString(boxLine(fmt.Sprintf("RAM:  %s %3d%%  %.1f/%.1fG", bar(s.MemPct), s.MemPct, s.MemUsedGB, s.MemTotalGB)))
	if s.GPUFreqMHz > 0 || s.GPUTempC > 0 {
		b.WriteString(boxLine(fmt.Sprintf("GPU:  %3dMHz  %d°C", s.GPUFreqMHz, s.GPUTempC)))
	}
	b.WriteString(boxLine(fmt.Sprintf("DISK: %s %3d%%  %s/%s", bar(s.DiskPct), s.DiskPct, s.DiskUsed, s.DiskTotal)))
	if s.NICName != "" {
		b.WriteString(boxLine(fmt.Sprintf("NIC:  %s %dMb/s %s", s.NICName, s.NICSpeed, s.NICType)))
	}
	if s.SwapTotalGB > 0 {
		b.WriteString(boxLine(fmt.Sprintf("SWAP: %s %3d%%  %.1f/%.1fG", bar(s.SwapPct), s.SwapPct, s.SwapUsedGB, s.SwapTotalGB)))
	}
	if s.FanRPM > 0 {
		b.WriteString(boxLine(fmt.Sprintf("FAN:  %d RPM", s.FanRPM)))
	}
	if s.BatPct != "N/A" {
		b.WriteString(boxLine(fmt.Sprintf("BAT:  %s%% %s", s.BatPct, s.BatStatus)))
	}
	b.WriteString(boxBottom())

	// Students box
	clients := trk.GetAll()
	active := 0
	for _, c := range clients {
		if c.Active {
			active++
		}
	}
	b.WriteString(boxTop(fmt.Sprintf("STUDENTS (%d active)", active)))
	if len(clients) == 0 {
		b.WriteString(boxLine("  No clients connected"))
	} else {
		b.WriteString(boxLine("  S   #   DEV   IP ADDRESS      OS/BROWSER       BANDWIDTH     TOTAL"))
		i := 0
		for _, c := range clients {
			if !c.Active {
				continue
			}
			i++
			icon := "💻"
			if c.Device == "mobile" {
				icon = "📱"
			}
			status := "●"
			osBrowser := c.OS + "/" + c.Browser
			bandwidth := formatBytes(c.Bytes) // simplified
			b.WriteString(boxLine(fmt.Sprintf("  %s   %d   %s   %-15s %-16s %-13s %s",
				status, i, icon, c.IP, osBrowser, bandwidth, formatBytes(c.Bytes))))
		}
	}
	b.WriteString(boxBottom())

	// Rejected box
	rejections := trk.Rejections()
	if len(rejections) > 0 {
		b.WriteString(boxTop("REJECTED"))
		for _, r := range rejections {
			b.WriteString(boxLine(fmt.Sprintf("  %s  %s  %s  %s", r.IP, r.OS, r.Reason, r.Time)))
		}
		b.WriteString(boxBottom())
	}

	fmt.Print(b.String())
}

func bar(pct int) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	filled := pct * 15 / 100
	empty := 15 - filled

	color := "\033[32m" // green
	if pct >= 90 {
		color = "\033[31m" // red
	} else if pct >= 70 {
		color = "\033[33m" // orange
	} else if pct >= 50 {
		color = "\033[93m" // yellow
	}

	return color + strings.Repeat("█", filled) + "\033[90m" + strings.Repeat("░", empty) + "\033[0m"
}

func boxTop(title string) string {
	padding := innerWidth - len(title) - 4
	if padding < 0 {
		padding = 0
	}
	return fmt.Sprintf("\033[90m╭── \033[1;31m%s\033[0m\033[90m %s╮\033[0m\n", title, strings.Repeat("─", padding))
}

func boxBottom() string {
	return fmt.Sprintf("\033[90m╰%s╯\033[0m\n", strings.Repeat("─", innerWidth))
}

func boxLine(content string) string {
	// Pad to innerWidth (approximate — strip ANSI for length calc)
	visibleLen := stripANSILen(content)
	padding := innerWidth - visibleLen - 2
	if padding < 0 {
		padding = 0
	}
	return fmt.Sprintf("\033[90m│\033[0m%s%s\033[90m│\033[0m\n", content, strings.Repeat(" ", padding))
}

func stripANSILen(s string) int {
	inEscape := false
	count := 0
	for _, r := range s {
		if r == '\033' {
			inEscape = true
			continue
		}
		if inEscape {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEscape = false
			}
			continue
		}
		count++
	}
	return count
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
	return fmt.Sprintf("%.1f%c", float64(b)/float64(div), "KMGTPE"[exp])
}
