package gui

import (
	"fmt"
	"sort"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"

	"bifrost/internal/dashboard"
)

// metricCard creates a card-like background rectangle with content on top.
func metricCard(inner fyne.CanvasObject) fyne.CanvasObject {
	card := canvas.NewRectangle(bgCard)
	card.Resize(fyne.NewSize(9999, 0))
	return container.NewStack(card, inner)
}

// Stats displays system statistics in the GUI.
type Stats struct {
	content     fyne.CanvasObject
	cpuBar      *canvas.Rectangle
	ramBar      *canvas.Rectangle
	swapBar     *canvas.Rectangle
	diskBar     *canvas.Rectangle
	cpuText     *canvas.Text
	cpuDetail   *canvas.Text
	loadText    *canvas.Text
	ramText     *canvas.Text
	ramDetail   *canvas.Text
	swapText    *canvas.Text
	gpuText     *canvas.Text
	gpuTempText *canvas.Text
	gpuMemText  *canvas.Text
	nicText     *canvas.Text
	nicIOText   *canvas.Text
	fanText     *canvas.Text
	tempText    *canvas.Text
	voltText    *canvas.Text
	diskText    *canvas.Text
	diskDetail  *canvas.Text
	batText     *canvas.Text
	batDetail   *canvas.Text
	sysText     *canvas.Text
	uptimeText  *canvas.Text
}

// NewStats creates a new system stats panel.
func NewStats() *Stats {
	s := &Stats{
		cpuBar:    canvas.NewRectangle(bgBar),
		ramBar:    canvas.NewRectangle(bgBar),
		swapBar:   canvas.NewRectangle(bgBar),
		diskBar:   canvas.NewRectangle(bgBar),
		cpuText:   canvas.NewText("--", textPrimary),
		cpuDetail: canvas.NewText("--", textDim),
		loadText:  canvas.NewText("--", textSecondary),
		ramText:   canvas.NewText("--", textPrimary),
		ramDetail: canvas.NewText("--", textDim),
		swapText:  canvas.NewText("--", textSecondary),
		gpuText:   canvas.NewText("--", textPrimary),
		gpuTempText: canvas.NewText("--", textSecondary),
		gpuMemText: canvas.NewText("--", textDim),
		nicText:   canvas.NewText("--", textPrimary),
		nicIOText: canvas.NewText("--", textDim),
		fanText:   canvas.NewText("--", textSecondary),
		tempText:  canvas.NewText("--", textDim),
		voltText:  canvas.NewText("--", textDim),
		diskText:  canvas.NewText("--", textPrimary),
		diskDetail: canvas.NewText("--", textDim),
		batText:   canvas.NewText("--", textSecondary),
		batDetail: canvas.NewText("--", textDim),
		sysText:   canvas.NewText("--", textDim),
		uptimeText: canvas.NewText("--", textDim),
	}

	for _, t := range []*canvas.Text{
		s.cpuText, s.cpuDetail, s.loadText, s.ramText, s.ramDetail, s.swapText,
		s.gpuText, s.gpuTempText, s.gpuMemText,
		s.nicText, s.nicIOText,
		s.fanText, s.tempText, s.voltText,
		s.diskText, s.diskDetail, s.batText, s.batDetail,
		s.sysText, s.uptimeText,
	} {
		t.TextSize = 11
	}

	s.cpuBar.Resize(fyne.NewSize(100, 6))
	s.ramBar.Resize(fyne.NewSize(100, 6))
	s.swapBar.Resize(fyne.NewSize(100, 6))
	s.diskBar.Resize(fyne.NewSize(100, 6))

	// Section: CPU
	cpuSection := container.NewVBox(
		sectionHeader("CPU"),
		s.cpuBar,
		s.cpuText,
		s.cpuDetail,
		s.loadText,
	)

	// Section: Memory
	memSection := container.NewVBox(
		sectionHeader("MEMORY"),
		s.ramBar,
		s.ramText,
		s.ramDetail,
		s.swapBar,
		s.swapText,
	)

	// Section: GPU
	gpuSection := container.NewVBox(
		sectionHeader("GPU"),
		s.gpuText,
		s.gpuTempText,
		s.gpuMemText,
	)

	// Section: Network
	netSection := container.NewVBox(
		sectionHeader("NETWORK"),
		s.nicText,
		s.nicIOText,
	)

	// Section: Storage
	diskSection := container.NewVBox(
		sectionHeader("STORAGE"),
		s.diskBar,
		s.diskText,
		s.diskDetail,
	)

	// Section: Sensors (fans, temps, voltages)
	sensorSection := container.NewVBox(
		sectionHeader("SENSORS"),
		s.fanText,
		s.tempText,
		s.voltText,
		s.batText,
		s.batDetail,
	)

	// System info at bottom
	sysSection := container.NewVBox(
		s.sysText,
		s.uptimeText,
	)

	scrollable := container.NewVScroll(
		container.NewVBox(
			container.NewPadded(cpuSection),
			container.NewPadded(memSection),
			container.NewPadded(gpuSection),
			container.NewPadded(netSection),
			container.NewPadded(diskSection),
			container.NewPadded(sensorSection),
			layout.NewSpacer(),
			sysSection,
		),
	)

	s.content = scrollable

	go s.refreshLoop()

	return s
}

func sectionHeader(label string) fyne.CanvasObject {
	title := canvas.NewText(label, accentRed)
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.TextSize = 10
	underline := canvas.NewRectangle(accentRed)
	underline.Resize(fyne.NewSize(9999, 1))
	return container.NewVBox(title, underline)
}

func (s *Stats) refreshLoop() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		stats := dashboard.GetSysStats(nil)

		fyne.Do(func() {
			s.sysText.Text = fmt.Sprintf("%s  |  Kernel %s  |  %d cores", stats.Hostname, stats.KernelVer, stats.CPUCores)
			s.sysText.Refresh()

			cpuModel := stats.CPUModel
			if len(cpuModel) > 50 {
				cpuModel = cpuModel[:50]
			}
			s.cpuText.Text = fmt.Sprintf("%.0f%%  %s  %s", stats.CPUUsage, stats.CPUFreq, stats.CPUTemp)
			s.cpuText.Refresh()

			s.cpuDetail.Text = cpuModel
			s.cpuDetail.Refresh()

			s.loadText.Text = fmt.Sprintf("Load: %.1f / %.1f / %.1f   Procs: %d", stats.Load1, stats.Load5, stats.Load15, stats.ProcsTotal)
			s.loadText.Refresh()

			s.ramText.Text = fmt.Sprintf("%.1f / %.1f GB   %.0f%%", stats.RAMUsed, stats.RAMTotal, stats.RAMPct)
			s.ramText.Refresh()

			s.ramDetail.Text = fmt.Sprintf("Buf %.1fG  Cache %.1fG  Active %.1fG  Dirty %.0fMB", stats.RAMBuf, stats.RAMCached, stats.RAMActive, stats.RAMDirty)
			s.ramDetail.Refresh()

			s.swapText.Text = fmt.Sprintf("Swap: %.1f / %.1f GB   %.0f%%", stats.SwapUsed, stats.SwapTotal, stats.SwapPct)
			s.swapText.Refresh()

			gpu := stats.GPUFreq
			if gpu == "" {
				gpu = "--"
			}
			s.gpuText.Text = fmt.Sprintf("Freq: %s", gpu)
			s.gpuText.Refresh()

			gpuTemp := stats.GPUTemp
			if gpuTemp == "" {
				gpuTemp = "--"
			}
			s.gpuTempText.Text = fmt.Sprintf("Temp: %s", gpuTemp)
			s.gpuTempText.Refresh()

			gpuMem := "--"
			if stats.GPUMemUsed != "" && stats.GPUMemTotal != "" {
				gpuMem = fmt.Sprintf("%s / %s", stats.GPUMemUsed, stats.GPUMemTotal)
			}
			s.gpuMemText.Text = fmt.Sprintf("VRAM: %s", gpuMem)
			s.gpuMemText.Refresh()

			nic := stats.NICFace
			if nic == "" {
				nic = "--"
			}
			s.nicText.Text = fmt.Sprintf("%s  %s  %s", nic, stats.NICSpeed, stats.NICType)
			s.nicText.Refresh()

			s.nicIOText.Text = fmt.Sprintf("RX %s   TX %s", formatBytes(stats.NICRx), formatBytes(stats.NICTx))
			s.nicIOText.Refresh()

			// Fans
			if len(stats.Fans) > 0 {
				keys := make([]string, 0, len(stats.Fans))
				for k := range stats.Fans {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				var parts []string
				for _, k := range keys {
					parts = append(parts, fmt.Sprintf("%s: %s", k, stats.Fans[k]))
				}
				fanLine := joinTruncate(parts, 90)
				s.fanText.Text = fanLine
			} else {
				s.fanText.Text = "No fan sensors detected"
			}
			s.fanText.Refresh()

			// All temps
			if len(stats.Temps) > 0 {
				keys := make([]string, 0, len(stats.Temps))
				for k := range stats.Temps {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				var parts []string
				for _, k := range keys {
					parts = append(parts, fmt.Sprintf("%s: %s", k, stats.Temps[k]))
				}
				tempLine := joinTruncate(parts, 90)
				s.tempText.Text = tempLine
			} else {
				s.tempText.Text = "No temp sensors"
			}
			s.tempText.Refresh()

			// Voltages
			if len(stats.Voltages) > 0 {
				keys := make([]string, 0, len(stats.Voltages))
				for k := range stats.Voltages {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				var parts []string
				for _, k := range keys {
					parts = append(parts, fmt.Sprintf("%s: %s", k, stats.Voltages[k]))
				}
				voltLine := joinTruncate(parts, 90)
				s.voltText.Text = voltLine
			} else {
				s.voltText.Text = "No voltage sensors"
			}
			s.voltText.Refresh()

			s.diskText.Text = fmt.Sprintf("%.0f GB   %.0f%% used", stats.DiskTotal, stats.DiskPct)
			s.diskText.Refresh()

			s.diskDetail.Text = fmt.Sprintf("Free: %.0f GB", stats.DiskTotal*(1-stats.DiskPct/100))
			s.diskDetail.Refresh()

			bat := stats.BatPct
			if bat == "" {
				bat = "--"
			}
			s.batText.Text = fmt.Sprintf("Battery: %s  %s", bat, stats.BatETA)
			s.batText.Refresh()

			batDetail := ""
			if stats.BatCurrent != "" || stats.BatVoltage != "" || stats.BatPower != "" {
				batDetail = fmt.Sprintf("%s  %s  %s", stats.BatCurrent, stats.BatVoltage, stats.BatPower)
			}
			s.batDetail.Text = batDetail
			s.batDetail.Refresh()

			s.uptimeText.Text = fmt.Sprintf("Uptime: %s", stats.Uptime)
			s.uptimeText.Refresh()

			// Gradient bars
			cpuFill := float32(stats.CPUUsage / 100.0)
			s.cpuBar.FillColor = gradientColor(cpuFill)
			s.cpuBar.Refresh()

			ramFill := float32(stats.RAMPct / 100.0)
			s.ramBar.FillColor = gradientColor(ramFill)
			s.ramBar.Refresh()

			swapFill := float32(stats.SwapPct / 100.0)
			s.swapBar.FillColor = gradientColor(swapFill)
			s.swapBar.Refresh()

			diskFill := float32(stats.DiskPct / 100.0)
			s.diskBar.FillColor = gradientColor(diskFill)
			s.diskBar.Refresh()
		})
	}
}

func (s *Stats) Content() fyne.CanvasObject {
	return s.content
}

func formatBytes(b int64) string {
	if b >= 1073741824 {
		return fmt.Sprintf("%.1f GB", float64(b)/1073741824.0)
	}
	if b >= 1048576 {
		return fmt.Sprintf("%.1f MB", float64(b)/1048576.0)
	}
	if b >= 1024 {
		return fmt.Sprintf("%.1f KB", float64(b)/1024.0)
	}
	return fmt.Sprintf("%d B", b)
}

func joinTruncate(parts []string, maxLen int) string {
	result := ""
	for _, p := range parts {
		if result != "" {
			result += "   "
		}
		if len(result)+len(p) > maxLen {
			result += "..."
			break
		}
		result += p
	}
	return result
}
