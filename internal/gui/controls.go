package gui

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"bifrost/internal/capture"
)

// Controls provides broadcast start/stop and settings.
type Controls struct {
	content      fyne.CanvasObject
	capturer     *capture.Capturer
	startBtn     *widget.Button
	urlText      *canvas.Text
	statusText   *canvas.Text
	isBroadcasting bool
}

// NewControls creates a new control bar.
func NewControls(capturer *capture.Capturer, ip string) *Controls {
	c := &Controls{
		capturer: capturer,
	}

	c.startBtn = widget.NewButton("  START  ", func() {
		if c.isBroadcasting {
			c.capturer.StopCapture()
			c.startBtn.SetText("  START  ")
			c.startBtn.Importance = widget.HighImportance
			c.statusText.Text = "OFFLINE"
			c.statusText.Refresh()
			c.isBroadcasting = false
		} else {
			if err := c.capturer.Start(nil); err != nil {
				dialog.ShowError(err, nil)
				return
			}
			c.startBtn.SetText("  STOP  ")
			c.startBtn.Importance = widget.DangerImportance
			c.statusText.Text = "BROADCASTING"
			c.statusText.Refresh()
			c.isBroadcasting = true
		}
	})
	c.startBtn.Importance = widget.HighImportance

	c.urlText = canvas.NewText(fmt.Sprintf("http://%s:8080", ip), textSecondary)
	c.urlText.TextSize = 11

	c.statusText = canvas.NewText("OFFLINE", textDim)
	c.statusText.TextStyle = fyne.TextStyle{Bold: true}
	c.statusText.TextSize = 12

	header := canvas.NewText("CONTROLS", accentRed)
	header.TextStyle = fyne.TextStyle{Bold: true}
	header.TextSize = 11

	c.content = container.NewBorder(
		nil, nil, nil, nil,
		container.NewVBox(
			header,
			container.NewHBox(c.startBtn, c.statusText),
			c.urlText,
		),
	)

	return c
}

func (c *Controls) Content() fyne.CanvasObject {
	return c.content
}

// StartBroadcasting triggers the capture start.
func (c *Controls) StartBroadcasting() {
	if !c.isBroadcasting {
		c.startBtn.OnTapped()
	}
}

// StatusWidget returns the uptime/status display.
func StatusWidget() fyne.CanvasObject {
	uptimeText := canvas.NewText("UPTIME 0s", textDim)
	uptimeText.TextSize = 11

	go func() {
		start := time.Now()
		ticker := time.NewTicker(1 * time.Second)
		for range ticker.C {
			d := time.Since(start).Truncate(time.Second)
			fyne.Do(func() {
				uptimeText.Text = fmt.Sprintf("UPTIME %s", d)
				uptimeText.Refresh()
			})
		}
	}()

	title := canvas.NewText("BIFROST", accentRed)
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.TextSize = 18

	version := canvas.NewText("v0.1.0", textDim)
	version.TextSize = 10

	return container.NewHBox(title, version, layout.NewSpacer(), uptimeText)
}
