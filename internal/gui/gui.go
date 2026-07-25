package gui

import (
	"context"
	"fmt"
	"image/color"
	"os"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/nelobster/bifrost/internal/capture"
	"github.com/nelobster/bifrost/internal/config"
	"github.com/nelobster/bifrost/internal/stats"
	"github.com/nelobster/bifrost/internal/tracker"
)

var (
	theApp    fyne.App
	theWindow fyne.Window
)

type GUI struct {
	cfg *config.Config
	cap *capture.Capture
	trk *tracker.Tracker

	// Header
	statusDot   *canvas.Circle
	statusLabel *canvas.Text
	uptimeLabel *canvas.Text
	urlLabel    *canvas.Text

	// Stats
	cpuValue, ramValue, diskValue, gpuValue *canvas.Text
	swapValue, fanValue, nicValue, batValue *canvas.Text
	cpuSub, ramSub, diskSub, gpuSub         *canvas.Text
	swapSub, nicSub                         *canvas.Text

	// Clients
	clientList *widget.List
	clients    []*tracker.Client
	clientsMu  sync.RWMutex
	clientCount *canvas.Text

	// Controls
	btnToggle  *widget.Button
	btnRestart *widget.Button
}

func Run(cfg *config.Config, cap *capture.Capture, trk *tracker.Tracker) {
	g := &GUI{cfg: cfg, cap: cap, trk: trk}

	theApp = app.NewWithID("com.nelobster.bifrost")
	theApp.Settings().SetTheme(&bifrostTheme{})

	theWindow = theApp.NewWindow("BIFROST")
	theWindow.Resize(fyne.NewSize(1400, 900))
	theWindow.SetMaster()

	g.buildUI()

	go g.refreshStats()
	go g.refreshClients()

	theWindow.ShowAndRun()
}

func Quit() {
	if theApp != nil {
		fyne.CurrentApp().Driver().CanvasForObject(theWindow.Content()).Refresh(theWindow.Content())
		theApp.Quit()
	}
}

func (g *GUI) buildUI() {
	header := g.buildHeader()
	statsSection := g.buildStatsSection()
	clientsSection := g.buildClientsSection()

	mainSplit := container.NewHSplit(statsSection, clientsSection)
	mainSplit.SetOffset(0.55)

	fullLayout := container.NewVBox(header, mainSplit)
	theWindow.SetContent(fullLayout)
}

func (g *GUI) buildHeader() *fyne.Container {
	// Logo
	logoText := canvas.NewText("BIFROST", ColorAccent)
	logoText.TextSize = 22
	logoText.TextStyle = fyne.TextStyle{Bold: true}

	versionText := canvas.NewText("  v0.3.0", ColorText3)
	versionText.TextSize = 11

	subtitleText := canvas.NewText("Browser Integrated Feed for Remote Observation & Screen Transmission", ColorText3)
	subtitleText.TextSize = 10

	logoRow := container.NewHBox(logoText, versionText)
	headerLeft := container.NewVBox(logoRow, subtitleText)

	// Status indicator
	g.statusDot = canvas.NewCircle(ColorText3)
	g.statusDot.Resize(fyne.NewSize(10, 10))

	g.statusLabel = canvas.NewText("CONNECTING", ColorText3)
	g.statusLabel.TextSize = 11
	g.statusLabel.TextStyle = fyne.TextStyle{Bold: true}

	statusRow := container.NewHBox(g.statusDot, layout.NewSpacer(), g.statusLabel)

	// Status badge container
	statusBadge := container.NewPadded(statusRow)

	// Uptime
	g.uptimeLabel = canvas.NewText("--", ColorText2)
	g.uptimeLabel.TextSize = 11

	// URL
	g.urlLabel = canvas.NewText(fmt.Sprintf("http://%s:%d", g.cfg.LocalIP, g.cfg.Port), ColorAccent)
	g.urlLabel.TextSize = 11

	// Controls
	g.btnToggle = widget.NewButton("OFFLINE", func() {
		if g.cap.IsStreaming() {
			g.cap.Stop()
		} else {
			go g.cap.Start(context.Background())
		}
	})
	g.btnToggle.Importance = widget.DangerImportance

	g.btnRestart = widget.NewButton("RESTART", func() {
		g.cap.Stop()
		time.Sleep(500 * time.Millisecond)
		os.Exit(0)
	})
	g.btnRestart.Importance = widget.LowImportance

	controls := container.NewHBox(g.btnToggle, g.btnRestart)

	// Header right side
	headerRight := container.NewVBox(statusBadge, g.urlLabel, g.uptimeLabel, controls)

	// Full header
	headerContent := container.NewHBox(headerLeft, layout.NewSpacer(), headerRight)

	// Header background
	headerBg := canvas.NewRectangle(ColorHeader)
	headerBg.SetMinSize(fyne.NewSize(0, 80))

	return container.NewStack(headerBg, container.NewPadded(headerContent))
}

func (g *GUI) buildStatsSection() fyne.CanvasObject {
	// Section title
	titleText := canvas.NewText("System Monitor", ColorText)
	titleText.TextSize = 14
	titleText.TextStyle = fyne.TextStyle{Bold: true}

	// Create stat cards
	cpuCard := g.createStatCard("CPU", &g.cpuValue, &g.cpuSub)
	ramCard := g.createStatCard("RAM", &g.ramValue, &g.ramSub)
	diskCard := g.createStatCard("DISK", &g.diskValue, &g.diskSub)
	gpuCard := g.createStatCard("GPU", &g.gpuValue, &g.gpuSub)
	swapCard := g.createStatCard("SWAP", &g.swapValue, &g.swapSub)
	nicCard := g.createStatCard("NIC", &g.nicValue, &g.nicSub)
	fanCard := g.createStatCard("FAN", &g.fanValue, nil)
	batCard := g.createStatCard("BATTERY", &g.batValue, nil)

	// Stats grid
	statsGrid := container.NewGridWithColumns(2,
		cpuCard, ramCard,
		diskCard, gpuCard,
		swapCard, nicCard,
		fanCard, batCard,
	)

	// System info section
	infoTitle := canvas.NewText("Stream Config", ColorText2)
	infoTitle.TextSize = 11
	infoTitle.TextStyle = fyne.TextStyle{Bold: true}

	audioStatus := "Off"
	if !g.cfg.NoAudio {
		audioStatus = "On"
	}
	webrtcStatus := "Off"
	if !g.cfg.NoWebRTC {
		webrtcStatus = "On"
	}

	infoGrid := container.NewGridWithColumns(2,
		g.createInfoRow("Resolution", g.cfg.Resolution),
		g.createInfoRow("FPS", fmt.Sprintf("%d", g.cfg.FPS)),
		g.createInfoRow("Quality", fmt.Sprintf("%d", g.cfg.Quality)),
		g.createInfoRow("Port", fmt.Sprintf("%d", g.cfg.Port)),
		g.createInfoRow("Audio", audioStatus),
		g.createInfoRow("WebRTC", webrtcStatus),
	)

	// Hardware info
	hwTitle := canvas.NewText("Hardware", ColorText2)
	hwTitle.TextSize = 11
	hwTitle.TextStyle = fyne.TextStyle{Bold: true}

	g.cpuValue = g.cpuValue // ensure initialized
	cpuModel := canvas.NewText("--", ColorText3)
	cpuModel.TextSize = 10
	g.cpuSub = cpuModel

	diskModel := canvas.NewText("--", ColorText3)
	diskModel.TextSize = 10
	g.diskSub = diskModel

	boardModel := canvas.NewText("--", ColorText3)
	boardModel.TextSize = 10

	hwGrid := container.NewGridWithColumns(1,
		g.createInfoRowCustom("CPU", cpuModel),
		g.createInfoRowCustom("Disk", diskModel),
		g.createInfoRowCustom("Board", boardModel),
	)

	content := container.NewVBox(
		titleText,
		layout.NewSpacer(),
		statsGrid,
		widget.NewSeparator(),
		infoTitle,
		infoGrid,
		widget.NewSeparator(),
		hwTitle,
		hwGrid,
		widget.NewSeparator(),
	)

	// Card background
	cardBg := canvas.NewRectangle(ColorSurface)
	headerBg := canvas.NewRectangle(ColorCardHeader)
	headerBg.SetMinSize(fyne.NewSize(0, 40))

	headerOverlay := container.NewStack(headerBg, container.NewPadded(titleText))
	statsContent := container.NewVBox(
		container.NewStack(cardBg, container.NewPadded(content)),
	)

	return container.NewVBox(headerOverlay, statsContent)
}

func (g *GUI) buildClientsSection() fyne.CanvasObject {
	// Section title
	titleText := canvas.NewText("Connected Clients", ColorText)
	titleText.TextSize = 14
	titleText.TextStyle = fyne.TextStyle{Bold: true}

	g.clientCount = canvas.NewText("0 active", ColorText3)
	g.clientCount.TextSize = 11

	titleRow := container.NewHBox(titleText, layout.NewSpacer(), g.clientCount)

	// Client list
	g.clientList = widget.NewList(
		func() int {
			g.clientsMu.RLock()
			defer g.clientsMu.RUnlock()
			return len(g.clients)
		},
		func() fyne.CanvasObject {
			return g.createClientRow()
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			g.clientsMu.RLock()
			defer g.clientsMu.RUnlock()
			if id < len(g.clients) {
				g.updateClientRow(obj.(*fyne.Container), g.clients[id])
			}
		},
	)

	// Card background
	cardBg := canvas.NewRectangle(ColorSurface)
	headerBg := canvas.NewRectangle(ColorCardHeader)
	headerBg.SetMinSize(fyne.NewSize(0, 40))

	headerOverlay := container.NewStack(headerBg, container.NewPadded(titleRow))
	listContent := container.NewStack(cardBg, container.NewPadded(g.clientList))

	return container.NewVBox(headerOverlay, listContent)
}

func (g *GUI) createStatCard(label string, value **canvas.Text, sub **canvas.Text) *fyne.Container {
	// Label
	titleText := canvas.NewText(label, ColorText3)
	titleText.TextSize = 10

	// Value
	valText := canvas.NewText("--", ColorText)
	valText.TextSize = 18
	valText.TextStyle = fyne.TextStyle{Bold: true}
	*value = valText

	// Sub text
	subText := canvas.NewText("", ColorText3)
	subText.TextSize = 10
	if sub != nil {
		*sub = subText
	}

	content := container.NewVBox(
		titleText,
		valText,
		subText,
	)

	// Card background
	cardBg := canvas.NewRectangle(ColorSurface2)

	return container.NewStack(cardBg, container.NewPadded(content))
}

func (g *GUI) createInfoRow(label, value string) *fyne.Container {
	titleText := canvas.NewText(label, ColorText3)
	titleText.TextSize = 10

	valText := canvas.NewText(value, ColorText2)
	valText.TextSize = 11

	return container.NewHBox(titleText, layout.NewSpacer(), valText)
}

func (g *GUI) createInfoRowCustom(label string, valCanvas *canvas.Text) *fyne.Container {
	titleText := canvas.NewText(label, ColorText3)
	titleText.TextSize = 10

	return container.NewHBox(titleText, layout.NewSpacer(), valCanvas)
}

func (g *GUI) createClientRow() *fyne.Container {
	// Status dot
	dot := canvas.NewCircle(ColorText3)
	dot.Resize(fyne.NewSize(8, 8))

	// Device icon
	deviceIcon := canvas.NewText(" Desktop", ColorText2)
	deviceIcon.TextSize = 12

	// IP
	ipText := canvas.NewText("--", ColorText)
	ipText.TextSize = 12
	ipText.TextStyle = fyne.TextStyle{Monospace: true}

	// OS/Browser
	infoText := canvas.NewText("-- / --", ColorText2)
	infoText.TextSize = 11

	return container.NewHBox(
		container.NewPadded(dot),
		deviceIcon,
		ipText,
		layout.NewSpacer(),
		infoText,
	)
}

func (g *GUI) updateClientRow(row *fyne.Container, c *tracker.Client) {
	if len(row.Objects) < 5 {
		return
	}

	// Status dot
	dot := row.Objects[0].(*fyne.Container).Objects[0].(*canvas.Circle)
	if c.Active {
		dot.FillColor = ColorGreen
	} else {
		dot.FillColor = ColorText3
	}
	dot.Refresh()

	// Device icon
	deviceIcon := row.Objects[1].(*canvas.Text)
	if c.Device == "mobile" {
		deviceIcon.Text = " Mobile"
	} else {
		deviceIcon.Text = " Desktop"
	}
	deviceIcon.Refresh()

	// IP
	ipText := row.Objects[2].(*canvas.Text)
	ipText.Text = c.IP
	ipText.Refresh()

	// Info
	infoText := row.Objects[4].(*canvas.Text)
	infoText.Text = fmt.Sprintf("%s / %s", c.OS, c.Browser)
	infoText.Refresh()
}

func (g *GUI) refreshStats() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		sys := stats.Collect()

		fyne.Do(func() {
			// CPU
			g.cpuValue.Text = fmt.Sprintf("%d%%", sys.CPUUsage)
			g.cpuValue.Color = barColor(sys.CPUUsage)
			g.cpuValue.Refresh()
			if g.cpuSub != nil {
				g.cpuSub.Text = fmt.Sprintf("%d MHz  %d°C", sys.CPUFreqMHz, sys.CPUTempC)
				g.cpuSub.Refresh()
			}

			// RAM
			g.ramValue.Text = fmt.Sprintf("%d%%", sys.MemPct)
			g.ramValue.Color = barColor(sys.MemPct)
			g.ramValue.Refresh()
			if g.ramSub != nil {
				g.ramSub.Text = fmt.Sprintf("%.1f / %.1f GB", sys.MemUsedGB, sys.MemTotalGB)
				g.ramSub.Refresh()
			}

			// Disk
			g.diskValue.Text = fmt.Sprintf("%d%%", sys.DiskPct)
			g.diskValue.Color = barColor(sys.DiskPct)
			g.diskValue.Refresh()
			if g.diskSub != nil {
				g.diskSub.Text = fmt.Sprintf("%s / %s", sys.DiskUsed, sys.DiskTotal)
				g.diskSub.Refresh()
			}

			// GPU
			if sys.GPUFreqMHz > 0 || sys.GPUTempC > 0 {
				g.gpuValue.Text = fmt.Sprintf("%d MHz", sys.GPUFreqMHz)
				g.gpuValue.Color = ColorText
				if g.gpuSub != nil {
					g.gpuSub.Text = fmt.Sprintf("%d°C", sys.GPUTempC)
					g.gpuSub.Refresh()
				}
			} else {
				g.gpuValue.Text = "N/A"
				g.gpuValue.Color = ColorText3
			}
			g.gpuValue.Refresh()

			// Swap
			if sys.SwapTotalGB > 0 {
				g.swapValue.Text = fmt.Sprintf("%d%%", sys.SwapPct)
				g.swapValue.Color = barColor(sys.SwapPct)
				if g.swapSub != nil {
					g.swapSub.Text = fmt.Sprintf("%.1f / %.1f GB", sys.SwapUsedGB, sys.SwapTotalGB)
					g.swapSub.Refresh()
				}
			} else {
				g.swapValue.Text = "N/A"
				g.swapValue.Color = ColorText3
			}
			g.swapValue.Refresh()

			// NIC
			if sys.NICName != "" {
				g.nicValue.Text = sys.NICName
				g.nicValue.Color = ColorText
				if g.nicSub != nil {
					g.nicSub.Text = fmt.Sprintf("%d Mb/s  %s", sys.NICSpeed, sys.NICType)
					g.nicSub.Refresh()
				}
			} else {
				g.nicValue.Text = "N/A"
				g.nicValue.Color = ColorText3
			}
			g.nicValue.Refresh()

			// Fan
			if sys.FanRPM > 0 {
				g.fanValue.Text = fmt.Sprintf("%d RPM", sys.FanRPM)
				g.fanValue.Color = ColorText
			} else {
				g.fanValue.Text = "N/A"
				g.fanValue.Color = ColorText3
			}
			g.fanValue.Refresh()

			// Battery
			if sys.BatPct != "N/A" {
				g.batValue.Text = fmt.Sprintf("%s%%", sys.BatPct)
				g.batValue.Color = ColorText
			} else {
				g.batValue.Text = "N/A"
				g.batValue.Color = ColorText3
			}
			g.batValue.Refresh()

			// Status
			if g.cap.IsStreaming() {
				g.statusDot.FillColor = ColorGreen
				g.statusLabel.Text = "LIVE"
				g.statusLabel.Color = ColorGreen
				g.btnToggle.SetText("STOP")
				g.btnToggle.Importance = widget.DangerImportance
			} else {
				g.statusDot.FillColor = ColorRed
				g.statusLabel.Text = "OFFLINE"
				g.statusLabel.Color = ColorRed
				g.btnToggle.SetText("START")
				g.btnToggle.Importance = widget.HighImportance
			}
			g.statusDot.Refresh()
			g.statusLabel.Refresh()

			// URL
			g.urlLabel.Refresh()
		})
	}
}

func (g *GUI) refreshClients() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		all := g.trk.GetAll()
		g.clientsMu.Lock()
		g.clients = all
		g.clientsMu.Unlock()

		active := 0
		for _, c := range all {
			if c.Active {
				active++
			}
		}

		fyne.Do(func() {
			g.clientCount.Text = fmt.Sprintf("%d active", active)
			g.clientCount.Refresh()
			g.clientList.Refresh()
		})
	}
}

func barColor(pct int) color.Color {
	if pct >= 90 {
		return ColorRed
	}
	if pct >= 70 {
		return ColorYellow
	}
	return ColorGreen
}
