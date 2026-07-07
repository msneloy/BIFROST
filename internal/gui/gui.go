package gui

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"

	"bifrost/internal/capture"
	"bifrost/internal/dashboard"
	"bifrost/internal/stream"
	"bifrost/internal/tracker"
)

// Run starts the Fyne GUI application. Must be called from the main goroutine.
func Run(tr *tracker.Tracker, bc *stream.Broadcaster, capturer *capture.Capturer, ip, version string, headless bool) {
	if headless {
		dashboard.Start(tr, bc, ip, version)
		return
	}

	myApp := app.New()

	window := myApp.NewWindow(fmt.Sprintf("BIFROST v%s", version))
	window.Resize(fyne.NewSize(1280, 720))

	// Left panel: Preview on top, Stats on bottom
	preview := NewPreviewContainer(bc)
	stats := NewStats()
	leftPanel := container.NewVSplit(
		preview,
		stats.Content(),
	)

	// Right panel: Clients (full height)
	clients := NewClients(tr, bc)

	// Bottom controls
	controls := NewControls(capturer, ip)

	// Top bar with status
	topBar := container.NewHBox(
		StatusWidget(),
		layout.NewSpacer(),
	)

	// Main layout
	mainContent := container.NewBorder(
		topBar,
		controls.Content(),
		nil, nil,
		container.NewHSplit(leftPanel, clients.Content()),
	)

	// Set background
	bg := canvas.NewRectangle(bgDeep)
	window.SetContent(container.NewStack(bg, mainContent))

	// System tray: hide on close instead of quitting
	window.SetCloseIntercept(func() {
		window.Hide()
	})

	// Handle SIGINT/SIGTERM to quit the app
	go func() {
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
		<-sigs
		myApp.Quit()
	}()

	window.ShowAndRun()
}
