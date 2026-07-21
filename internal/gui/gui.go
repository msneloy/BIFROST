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
	barCPU, barRAM, barDisk                                                       *widget.ProgressBar

	// Hardware models
	lblCPUModel, lblDiskModel, lblBoardModel *widget.Label

	// Clients
	clientList *widget.List
	clientCard *widget.Card
	clients    []*tracker.Client
	clientsMu  sync.RWMutex

	// Status & controls
	lblStatus *widget.Label
	lblURL    *widget.Label
	btnToggle *widget.Button
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

	// ─── Stream specs ──────────────────────────────────────
	audioStatus := "Off"
	if !g.cfg.NoAudio {
		audioStatus = "On"
	}
	webrtcStatus := "Off"
	if !g.cfg.NoWebRTC {
		webrtcStatus = "On"
	}
	specsContent := container.NewVBox(
		widget.NewLabel("Stream Configuration"),
		newStaticRow("Resolution:", g.cfg.Resolution),
		newStaticRow("FPS:", fmt.Sprintf("%d", g.cfg.FPS)),
		newStaticRow("Quality:", fmt.Sprintf("%d", g.cfg.Quality)),
		newStaticRow("Port:", fmt.Sprintf("%d", g.cfg.Port)),
		newStaticRow("Audio:", audioStatus),
		newStaticRow("WebRTC:", webrtcStatus),
	)
	specsCard := widget.NewCard("Specs", "", specsContent)

	// ─── System monitor (with hardware models merged in) ────
	g.lblCPU = widget.NewLabel("--")
	g.lblRAM = widget.NewLabel("--")
	g.lblDisk = widget.NewLabel("--")
	g.lblGPU = widget.NewLabel("--")
	g.lblNIC = widget.NewLabel("--")
	g.lblSwap = widget.NewLabel("--")
	g.lblFan = widget.NewLabel("--")
	g.lblBat = widget.NewLabel("--")
	g.lblUptime = widget.NewLabel("--")

	g.barCPU = widget.NewProgressBar()
	g.barRAM = widget.NewProgressBar()
	g.barDisk = widget.NewProgressBar()

	g.lblCPUModel = widget.NewLabel("--")
	g.lblDiskModel = widget.NewLabel("--")
	g.lblBoardModel = widget.NewLabel("--")

	statsContent := container.NewVBox(
		widget.NewLabel("System Monitor"),
		newStatRowWithSub("CPU:", g.lblCPU, g.barCPU, g.lblCPUModel),
		newStatRowWithSub("RAM:", g.lblRAM, g.barRAM, nil),
		newStatRowWithSub("Disk:", g.lblDisk, g.barDisk, g.lblDiskModel),
		widget.NewSeparator(),
		newStatRowLabel("GPU:", g.lblGPU),
		newStatRowLabel("NIC:", g.lblNIC),
		newStatRowLabel("Board:", g.lblBoardModel),
		newStatRowLabel("Swap:", g.lblSwap),
		newStatRowLabel("Fan:", g.lblFan),
		newStatRowLabel("Battery:", g.lblBat),
		widget.NewSeparator(),
		newStatRowLabel("Uptime:", g.lblUptime),
	)

	statsCard := widget.NewCard("Monitor", "", statsContent)

	topRow := container.NewHSplit(specsCard, statsCard)
	topRow.SetOffset(0.3)

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

	// ─── Layout ─────────────────────────────────────────────
	content := container.NewVSplit(topRow, g.clientCard)
	content.SetOffset(0.55)

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
			g.barCPU.SetValue(float64(sys.CPUUsage) / 100.0)

			g.lblRAM.SetText(fmt.Sprintf("%d%%  %.1f/%.1fG", sys.MemPct, sys.MemUsedGB, sys.MemTotalGB))
			g.barRAM.SetValue(float64(sys.MemPct) / 100.0)

			g.lblDisk.SetText(fmt.Sprintf("%d%%  %s/%s", sys.DiskPct, sys.DiskUsed, sys.DiskTotal))
			g.barDisk.SetValue(float64(sys.DiskPct) / 100.0)

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

func newStatRow(label string, val *widget.Label, bar *widget.ProgressBar) *fyne.Container {
	return container.NewBorder(nil, nil,
		widget.NewLabelWithStyle(label, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		nil,
		container.NewVBox(val, bar),
	)
}

func newStatRowWithSub(label string, val *widget.Label, bar *widget.ProgressBar, sub *widget.Label) *fyne.Container {
	items := []fyne.CanvasObject{val, bar}
	if sub != nil {
		sub.TextStyle = fyne.TextStyle{Italic: true}
		items = append(items, sub)
	}
	return container.NewBorder(nil, nil,
		widget.NewLabelWithStyle(label, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		nil,
		container.NewVBox(items...),
	)
}

func newStatRowLabel(label string, val *widget.Label) *fyne.Container {
	return container.NewBorder(nil, nil,
		widget.NewLabelWithStyle(label, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		nil,
		val,
	)
}

func newStaticRow(label, value string) *fyne.Container {
	return container.NewBorder(nil, nil,
		widget.NewLabelWithStyle(label, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		nil,
		widget.NewLabel(value),
	)
}
