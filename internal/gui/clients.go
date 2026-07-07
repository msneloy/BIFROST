package gui

import (
	"fmt"
	"image/color"
	"sort"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"

	"bifrost/internal/stream"
	"bifrost/internal/tracker"
)

var startTime = time.Now()

// Clients displays the list of connected students.
type Clients struct {
	content     fyne.CanvasObject
	tr          *tracker.Tracker
	bc          *stream.Broadcaster
	countText   *canvas.Text
	table       *fyne.Container
	footerText  *canvas.Text
	summaryText *canvas.Text
}

// NewClients creates a new client list panel.
func NewClients(tr *tracker.Tracker, bc *stream.Broadcaster) *Clients {
	c := &Clients{
		tr:          tr,
		bc:          bc,
		countText:   canvas.NewText("STUDENTS", accentRed),
		table:       container.NewVBox(),
		footerText:  canvas.NewText("--", textDim),
		summaryText: canvas.NewText("--", textSecondary),
	}

	c.countText.TextStyle = fyne.TextStyle{Bold: true}
	c.countText.TextSize = 12
	c.footerText.TextSize = 10
	c.summaryText.TextSize = 10

	underline := canvas.NewRectangle(accentRed)
	underline.Resize(fyne.NewSize(9999, 1))

	header := container.NewVBox(
		container.NewHBox(c.countText, layout.NewSpacer()),
		underline,
		container.NewHBox(
			clientHeaderText("S"),
			clientHeaderText("#"),
			clientHeaderText("DEV"),
			clientHeaderText("IP"),
			clientHeaderText("OS"),
			clientHeaderText("BROWSER"),
			clientHeaderText("RES"),
			clientHeaderText("GPU"),
			clientHeaderText("BAT"),
			clientHeaderText("BW"),
			clientHeaderText("TOTAL"),
			layout.NewSpacer(),
		),
	)

	c.table = container.NewVBox(header)
	scrollable := container.NewVScroll(c.table)

	c.content = container.NewBorder(
		nil,
		container.NewVBox(c.summaryText, c.footerText),
		nil, nil,
		scrollable,
	)

	go c.refreshLoop()

	return c
}

func clientHeaderText(label string) *canvas.Text {
	t := canvas.NewText(label, textDim)
	t.TextSize = 9
	t.TextStyle = fyne.TextStyle{Bold: true}
	return t
}

func (c *Clients) refreshLoop() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		fyne.Do(func() {
			c.refresh()
		})
	}
}

func (c *Clients) refresh() {
	clients := c.tr.GetAllClients()

	sort.Slice(clients, func(i, j int) bool {
		return clients[i].LastSeen.After(clients[j].LastSeen)
	})

	active := 0
	var totalBW float64
	for _, cl := range clients {
		if cl.Active {
			active++
			bw := float64(cl.Bytes-cl.PrevBytes) / 1024.0 / 1024.0
			totalBW += bw
		}
	}

	c.countText.Text = fmt.Sprintf("STUDENTS   %d active", active)
	c.countText.Refresh()

	// Clear old rows (keep header)
	for len(c.table.Objects) > 1 {
		c.table.Objects = c.table.Objects[:len(c.table.Objects)-1]
	}

	i := 0
	for _, cl := range clients {
		if !cl.Active {
			continue
		}
		i++

		statusColor := statusGreen
		status := "*"

		dev := "PC"
		if cl.DevType == "mobile" {
			dev = "MB"
		}

		osName := cl.OS
		if osName == "" {
			osName = "--"
		}

		browser := cl.Browser
		if browser == "" {
			browser = "--"
		}

		res := cl.Resolution
		if res == "" {
			res = "--"
		}

		gpu := cl.GPU
		if gpu == "" {
			gpu = "--"
		}

		bat := "--"
		if cl.BatPct > 0 {
			bat = fmt.Sprintf("%d%%", cl.BatPct)
			if cl.Charging {
				bat += "+"
			}
		}

		bw := float64(cl.Bytes-cl.PrevBytes) / 1024.0 / 1024.0
		uplink := float64(cl.Bytes) / 1024.0 / 1024.0

		// Alternate row backgrounds
		rowBg := bgCard
		if i%2 == 0 {
			rowBg = bgCardAlt
		}

		rowContent := container.NewHBox(
			canvas.NewText(status, statusColor),
			canvas.NewText(fmt.Sprintf("%d", i), textSecondary),
			canvas.NewText(dev, textSecondary),
			canvas.NewText(cl.IP, textPrimary),
			canvas.NewText(osName, textSecondary),
			canvas.NewText(browser, textSecondary),
			canvas.NewText(res, textDim),
			canvas.NewText(gpu, textDim),
			canvas.NewText(bat, textDim),
			canvas.NewText(fmt.Sprintf("%.1fMB/s", bw), accentOrange),
			canvas.NewText(fmt.Sprintf("%.1fMB", uplink), textSecondary),
			layout.NewSpacer(),
		)

		// Set text sizes
		for _, obj := range rowContent.Objects {
			if txt, ok := obj.(*canvas.Text); ok {
				txt.TextSize = 10
			}
		}

		bg := canvas.NewRectangle(rowBg)
		row := container.NewStack(bg, container.NewPadded(rowContent))
		c.table.Add(row)

		if i >= 20 {
			break
		}
	}

	c.table.Refresh()

	// Summary
	totalPubMB := float64(c.bc.Total) / 1024.0 / 1024.0
	pubRate := float64(c.bc.GetPubRate()) / 1024.0 / 1024.0
	c.summaryText.Text = fmt.Sprintf("Published: %.1f MB   Rate: %.1f MB/s", totalPubMB, pubRate)
	c.summaryText.Refresh()

	uptime := time.Since(startTime).Truncate(time.Second)
	c.footerText.Text = fmt.Sprintf("Session: %s", uptime.String())
	c.footerText.Refresh()
}

func (c *Clients) Content() fyne.CanvasObject {
	return c.content
}

// Keep for backward compat
func canvasNewSmallText(label string, gray uint8) *canvas.Text {
	t := canvas.NewText(label, color.RGBA{R: gray, G: gray, B: gray, A: 255})
	t.TextSize = 9
	return t
}
