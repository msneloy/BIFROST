package gui

import (
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
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

	// Preview - use canvas.Raster for efficient frame updates
	preview *canvas.Raster
	lastImg image.Image
	imgMu   sync.RWMutex

	// Stats
	lblCPU, lblRAM, lblDisk, lblGPU, lblNIC, lblSwap, lblFan, lblBat, lblUptime *widget.Label
	barCPU, barRAM, barDisk                                                       *widget.ProgressBar

	// Clients
	clientList *widget.List
	clients    []*tracker.Client
	clientsMu  sync.RWMutex

	// Status
	lblStatus *widget.Label
	lblURL    *widget.Label
}

func Run(cfg *config.Config, cap *capture.Capture, trk *tracker.Tracker) {
	g := &GUI{cfg: cfg, cap: cap, trk: trk}

	theApp = app.New()
	theApp.Settings().SetTheme(&bifrostTheme{})

	theWindow = theApp.NewWindow("BIFROST")
	theWindow.Resize(fyne.NewSize(1100, 700))
	theWindow.SetMaster()

	g.buildUI()

	go g.refreshPreview()
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

	g.lblURL = widget.NewLabel(fmt.Sprintf("http://%s:%d", g.cfg.LocalIP, g.cfg.Port))
	g.lblURL.TextStyle = fyne.TextStyle{Monospace: true}

	statusBar := container.NewHBox(
		g.lblStatus,
		layout.NewSpacer(),
		widget.NewLabel("STREAM:"),
		g.lblURL,
	)

	// ─── Preview using canvas.Raster ────────────────────────
	// canvas.Raster takes a function: func(w, h int) image.Image
	g.preview = canvas.NewRaster(g.drawFrame)
	g.preview.SetMinSize(fyne.NewSize(640, 360))

	previewCard := widget.NewCard("Live Preview", "", g.preview)

	// ─── System stats ───────────────────────────────────────
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

	statsContent := container.NewVBox(
		widget.NewLabel("System Information"),
		newStatRow("CPU:", g.lblCPU, g.barCPU),
		newStatRow("RAM:", g.lblRAM, g.barRAM),
		newStatRow("Disk:", g.lblDisk, g.barDisk),
		widget.NewSeparator(),
		newStatRowLabel("GPU:", g.lblGPU),
		newStatRowLabel("NIC:", g.lblNIC),
		newStatRowLabel("Swap:", g.lblSwap),
		newStatRowLabel("Fan:", g.lblFan),
		newStatRowLabel("Battery:", g.lblBat),
		widget.NewSeparator(),
		newStatRowLabel("Uptime:", g.lblUptime),
	)

	statsCard := widget.NewCard("System", "", statsContent)

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

	clientCard := widget.NewCard("Connected Clients", "0 active", g.clientList)

	// ─── Layout ─────────────────────────────────────────────
	rightPanel := container.NewVBox(statsCard, clientCard)

	content := container.NewHSplit(previewCard, rightPanel)
	content.SetOffset(0.6)

	theWindow.SetContent(container.NewBorder(statusBar, nil, nil, nil, content))
}

// drawFrame is called by canvas.Raster to render the current frame.
// It runs on the Fyne render thread — safe to read lastImg.
func (g *GUI) drawFrame(w, h int) image.Image {
	g.imgMu.RLock()
	img := g.lastImg
	g.imgMu.RUnlock()

	if img == nil {
		// Return a blank dark image
		blank := image.NewRGBA(image.Rect(0, 0, w, h))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				blank.Set(x, y, color.RGBA{R: 20, G: 20, B: 20, A: 255})
			}
		}
		return blank
	}

	// Scale the captured frame to the requested size
	bounds := img.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()
	if srcW == 0 || srcH == 0 {
		return img
	}

	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	sx := float64(srcW) / float64(w)
	sy := float64(srcH) / float64(h)

	for dy := 0; dy < h; dy++ {
		sy0 := int(float64(dy)*sy) + bounds.Min.Y
		if sy0 >= bounds.Max.Y {
			sy0 = bounds.Max.Y - 1
		}
		for dx := 0; dx < w; dx++ {
			sx0 := int(float64(dx)*sx) + bounds.Min.X
			if sx0 >= bounds.Max.X {
				sx0 = bounds.Max.X - 1
			}
			dst.Set(dx, dy, img.At(sx0, sy0))
		}
	}
	return dst
}

func (g *GUI) refreshPreview() {
	ticker := time.NewTicker(100 * time.Millisecond) // ~10 FPS UI refresh
	defer ticker.Stop()

	for range ticker.C {
		if !g.cap.IsStreaming() {
			continue
		}

		frame := g.cap.Broadcaster().RingBuffer().Latest()
		if frame == nil {
			continue
		}

		img, err := decodeJPEG(frame)
		if err != nil {
			continue
		}

		g.imgMu.Lock()
		g.lastImg = img
		g.imgMu.Unlock()

		// Trigger Fyne to re-call drawFrame on the UI thread
		fyne.Do(func() {
			if theApp != nil && g.preview != nil {
				c := theApp.Driver().CanvasForObject(g.preview)
				if c != nil {
					c.Refresh(g.preview)
				}
			}
		})
	}
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

			if g.cap.IsStreaming() {
				g.lblStatus.SetText("LIVE")
				g.lblStatus.Importance = widget.HighImportance
			} else {
				g.lblStatus.SetText("OFFLINE")
				g.lblStatus.Importance = widget.DangerImportance
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
			g.clientList.Refresh()
		})
	}
}

func decodeJPEG(data []byte) (image.Image, error) {
	return jpeg.Decode(&byteReader{data: data})
}

type byteReader struct {
	data []byte
	pos  int
}

func (r *byteReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, fmt.Errorf("EOF")
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

func newStatRow(label string, val *widget.Label, bar *widget.ProgressBar) *fyne.Container {
	return container.NewBorder(nil, nil,
		widget.NewLabelWithStyle(label, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		nil,
		container.NewVBox(val, bar),
	)
}

func newStatRowLabel(label string, val *widget.Label) *fyne.Container {
	return container.NewBorder(nil, nil,
		widget.NewLabelWithStyle(label, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		nil,
		val,
	)
}

func init() {
	_ = color.RGBA{}
}
