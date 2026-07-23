package gui

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
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

	// Stats
	lblCPU, lblRAM, lblDisk, lblGPU, lblNIC, lblSwap, lblFan, lblBat, lblUptime *widget.Label

	// Hardware models
	lblCPUModel, lblDiskModel, lblBoardModel *widget.Label

	// Clients
	clientList *widget.List
	clientCard *widget.Card
	clients    []*tracker.Client
	clientsMu  sync.RWMutex

	// Status & controls
	lblStatus  *widget.Label
	lblURL     *widget.Label
	btnToggle  *widget.Button
	btnRestart *widget.Button
}

func Run(cfg *config.Config, cap *capture.Capture, trk *tracker.Tracker) {
	g := &GUI{cfg: cfg, cap: cap, trk: trk}

	theApp = app.NewWithID("com.nelobster.bifrost")
	theApp.Settings().SetTheme(&bifrostTheme{})

	theWindow = theApp.NewWindow("BIFROST")
	theWindow.Resize(fyne.NewSize(1920, 1080))
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
	// ─── Status bar ─────────────────────────────────────────
	g.lblStatus = widget.NewLabel("INITIALIZING")
	g.lblStatus.TextStyle = fyne.TextStyle{Bold: true}
	g.lblStatus.Importance = widget.HighImportance

	g.lblURL = widget.NewLabel(fmt.Sprintf("http://%s:%d  |  http://%s.local:%d", g.cfg.LocalIP, g.cfg.Port, g.cfg.LocalIP, g.cfg.Port))
	g.lblURL.TextStyle = fyne.TextStyle{Monospace: true}

	// ─── Broadcast toggle (button that changes state) ───────
	g.btnToggle = widget.NewButton("OFFLINE", func() {
		if g.cap.IsStreaming() {
			g.cap.Stop()
		} else {
			go g.cap.Start(context.Background())
			g.lblStatus.SetText("STARTING...")
			g.lblStatus.Importance = widget.WarningImportance
		}
	})
	g.btnToggle.Importance = widget.DangerImportance

	g.btnRestart = widget.NewButton("RESTART", func() {
		g.cap.Stop()
		time.Sleep(500 * time.Millisecond)
		os.Exit(0)
	})
	g.btnRestart.Importance = widget.DangerImportance

	controls := container.NewHBox(g.btnToggle, layout.NewSpacer(), g.btnRestart)

	statusBar := container.NewVBox(
		container.NewHBox(
			g.lblStatus,
			layout.NewSpacer(),
			widget.NewLabel("STREAM:"),
			g.lblURL,
		),
		controls,
	)

	// ─── System monitor (compact, with specs inline) ───────
	g.lblCPU = widget.NewLabel("--")
	g.lblRAM = widget.NewLabel("--")
	g.lblDisk = widget.NewLabel("--")
	g.lblGPU = widget.NewLabel("--")
	g.lblNIC = widget.NewLabel("--")
	g.lblSwap = widget.NewLabel("--")
	g.lblFan = widget.NewLabel("--")
	g.lblBat = widget.NewLabel("--")
	g.lblUptime = widget.NewLabel("--")

	g.lblCPUModel = widget.NewLabel("--")
	g.lblDiskModel = widget.NewLabel("--")
	g.lblBoardModel = widget.NewLabel("--")

	audioStatus := "Off"
	if !g.cfg.NoAudio {
		audioStatus = "On"
	}
	webrtcStatus := "Off"
	if !g.cfg.NoWebRTC {
		webrtcStatus = "On"
	}

	specsGrid := container.NewGridWithColumns(2,
		newCompactRow("Resolution:", g.cfg.Resolution),
		newCompactRow("FPS:", fmt.Sprintf("%d", g.cfg.FPS)),
		newCompactRow("Quality:", fmt.Sprintf("%d", g.cfg.Quality)),
		newCompactRow("Port:", fmt.Sprintf("%d", g.cfg.Port)),
		newCompactRow("Audio:", audioStatus),
		newCompactRow("WebRTC:", webrtcStatus),
	)

	statsGrid := container.NewGridWithColumns(3,
		newCompactLabelRow("CPU:", g.lblCPU),
		newCompactLabelRow("RAM:", g.lblRAM),
		newCompactLabelRow("Disk:", g.lblDisk),
		newCompactLabelRow("GPU:", g.lblGPU),
		newCompactLabelRow("Swap:", g.lblSwap),
		newCompactLabelRow("Fan:", g.lblFan),
		newCompactLabelRow("NIC:", g.lblNIC),
		newCompactLabelRow("Board:", g.lblBoardModel),
		newCompactLabelRow("Bat:", g.lblBat),
	)

	modelsGrid := container.NewGridWithColumns(3,
		newCompactItalicRow("CPU:", g.lblCPUModel),
		newCompactItalicRow("Disk:", g.lblDiskModel),
		newCompactItalicRow("Board:", g.lblBoardModel),
	)

	uptimeRow := newCompactLabelRow("Uptime:", g.lblUptime)

	monitorContent := container.NewVBox(
		container.NewPadded(specsGrid),
		widget.NewSeparator(),
		container.NewPadded(statsGrid),
		widget.NewSeparator(),
		container.NewPadded(modelsGrid),
		widget.NewSeparator(),
		container.NewPadded(uptimeRow),
	)
	monitorCard := widget.NewCard("Monitor", "", monitorContent)

	// ─── Client list ────────────────────────────────────────
	g.clientList = widget.NewList(
		func() int {
			g.clientsMu.RLock()
			defer g.clientsMu.RUnlock()
			return len(g.clients)
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("template")
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			g.clientsMu.RLock()
			defer g.clientsMu.RUnlock()
			if id < len(g.clients) {
				c := g.clients[id]
				status := "○"
				if c.Active {
					status = "●"
				}
				obj.(*widget.Label).SetText(fmt.Sprintf("%s  %-15s  %s/%s  %s",
					status, c.IP, c.OS, c.Browser, c.Device))
			}
		},
	)

	g.clientCard = widget.NewCard("Connected Clients", "0 active", g.clientList)

	// ─── Layout: clients left, monitor right ────────────────
	content := container.NewHSplit(g.clientCard, monitorCard)
	content.SetOffset(0.3)

	fullLayout := container.NewVBox(statusBar, content)
	theWindow.SetContent(fullLayout)
}

func (g *GUI) refreshStats() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		sys := stats.Collect()

		fyne.Do(func() {
			g.lblCPU.SetText(fmt.Sprintf("%d%%  %dMHz  %d°C", sys.CPUUsage, sys.CPUFreqMHz, sys.CPUTempC))
			g.lblRAM.SetText(fmt.Sprintf("%d%%  %.1f/%.1fG", sys.MemPct, sys.MemUsedGB, sys.MemTotalGB))
			g.lblDisk.SetText(fmt.Sprintf("%d%%  %s/%s", sys.DiskPct, sys.DiskUsed, sys.DiskTotal))

			g.lblCPUModel.SetText(fmt.Sprintf("%s (%d cores)", sys.CPUModel, sys.CPUCores))
			g.lblDiskModel.SetText(sys.DiskModel)
			g.lblBoardModel.SetText(fmt.Sprintf("%s %s", sys.BoardVendor, sys.BoardModel))

			if sys.GPUFreqMHz > 0 || sys.GPUTempC > 0 {
				g.lblGPU.SetText(fmt.Sprintf("%dMHz  %d°C", sys.GPUFreqMHz, sys.GPUTempC))
			} else {
				g.lblGPU.SetText("N/A")
			}

			if sys.NICName != "" {
				g.lblNIC.SetText(fmt.Sprintf("%s  %dMb/s  %s", sys.NICName, sys.NICSpeed, sys.NICType))
			} else {
				g.lblNIC.SetText("N/A")
			}

			if sys.SwapTotalGB > 0 {
				g.lblSwap.SetText(fmt.Sprintf("%d%%  %.1f/%.1fG", sys.SwapPct, sys.SwapUsedGB, sys.SwapTotalGB))
			} else {
				g.lblSwap.SetText("N/A")
			}

			if sys.FanRPM > 0 {
				g.lblFan.SetText(fmt.Sprintf("%d RPM", sys.FanRPM))
			} else {
				g.lblFan.SetText("N/A")
			}

			if sys.BatPct != "N/A" {
				g.lblBat.SetText(fmt.Sprintf("%s%%  %s", sys.BatPct, sys.BatStatus))
			} else {
				g.lblBat.SetText("N/A")
			}

			g.lblUptime.SetText(sys.Uptime)

			// Sync toggle button with streaming state
			if g.cap.IsStreaming() {
				g.lblStatus.SetText("LIVE")
				g.lblStatus.Importance = widget.HighImportance
				g.btnToggle.SetText("LIVE")
				g.btnToggle.Importance = widget.HighImportance
			} else {
				g.lblStatus.SetText("OFFLINE")
				g.lblStatus.Importance = widget.DangerImportance
				g.btnToggle.SetText("OFFLINE")
				g.btnToggle.Importance = widget.DangerImportance
			}
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
			g.clientCard.SetSubTitle(fmt.Sprintf("%d active", active))
			g.clientList.Refresh()
		})
	}
}

func newCompactRow(label string, val string) *fyne.Container {
	return container.NewHBox(
		widget.NewLabelWithStyle(label, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel(val),
	)
}

func newCompactLabelRow(label string, val *widget.Label) *fyne.Container {
	return container.NewHBox(
		widget.NewLabelWithStyle(label, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		val,
	)
}

func newCompactItalicRow(label string, val *widget.Label) *fyne.Container {
	val.TextStyle = fyne.TextStyle{Italic: true}
	return container.NewHBox(
		widget.NewLabelWithStyle(label, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		val,
	)
}
