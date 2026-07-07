package gui

import (
	"image"
	"image/color"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"bifrost/internal/stream"
)

// Preview displays live MJPEG frames from the broadcaster.
type Preview struct {
	widget.BaseWidget
	raster  *canvas.Raster
	bc      *stream.Broadcaster
	img     image.Image
	mu      sync.RWMutex
	running bool
}

// Ensure Preview implements fyne.Widget
var _ fyne.Widget = (*Preview)(nil)

func (p *Preview) CreateRenderer() fyne.WidgetRenderer {
	return &previewRenderer{preview: p}
}

type previewRenderer struct {
	preview *Preview
}

func (r *previewRenderer) Destroy()                     { r.preview.running = false }
func (r *previewRenderer) Layout(size fyne.Size)         {}
func (r *previewRenderer) MinSize() fyne.Size            { return fyne.NewSize(320, 240) }
func (r *previewRenderer) Objects() []fyne.CanvasObject  { return nil }
func (r *previewRenderer) Refresh()                      { r.preview.raster.Refresh() }

// NewPreview creates a new live preview widget.
func NewPreview(bc *stream.Broadcaster) *Preview {
	p := &Preview{
		bc:      bc,
		running: true,
	}

	p.raster = canvas.NewRaster(func(w, h int) image.Image {
		p.mu.RLock()
		defer p.mu.RUnlock()
		if p.img == nil {
			return placeholderImage(w, h)
		}
		return scaleImage(p.img, w, h)
	})

	go p.refreshLoop()

	return p
}

func (p *Preview) refreshLoop() {
	ch := p.bc.Subscribe(1)
	defer p.bc.Unsubscribe(ch)

	ticker := time.NewTicker(66 * time.Millisecond) // ~15fps cap
	defer ticker.Stop()

	for p.running {
		select {
		case frame, ok := <-ch:
			if !ok {
				return
			}
			img, err := decodeJPEG(frame)
			if err != nil {
				continue
			}
			p.mu.Lock()
			p.img = img
			p.mu.Unlock()
			fyne.Do(func() {
				p.raster.Refresh()
			})
		case <-ticker.C:
		}
	}
}

// Content returns the Fyne canvas object for this preview.
func (p *Preview) Content() fyne.CanvasObject {
	return p.raster
}

func placeholderImage(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	// Dark red-tinted background
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 14, G: 8, B: 8, A: 255})
		}
	}
	return img
}

func scaleImage(src image.Image, w, h int) image.Image {
	bounds := src.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()

	if srcW == w && srcH == h {
		return src
	}

	dst := image.NewRGBA(image.Rect(0, 0, w, h))

	// Fast path for RGBA: direct pixel copy
	if rgba, ok := src.(*image.RGBA); ok {
		for y := 0; y < h; y++ {
			srcY := bounds.Min.Y + y*srcH/h
			srcRow := rgba.PixOffset(bounds.Min.X, srcY)
			for x := 0; x < w; x++ {
				srcX := bounds.Min.X + x*srcW/w
				srcOff := srcRow + srcX*4
				dstOff := dst.PixOffset(x, y)
				dst.Pix[dstOff] = rgba.Pix[srcOff]
				dst.Pix[dstOff+1] = rgba.Pix[srcOff+1]
				dst.Pix[dstOff+2] = rgba.Pix[srcOff+2]
				dst.Pix[dstOff+3] = rgba.Pix[srcOff+3]
			}
		}
		return dst
	}

	// Slow path: sample only the pixels we need (not full-res conversion)
	for y := 0; y < h; y++ {
		srcY := bounds.Min.Y + y*srcH/h
		for x := 0; x < w; x++ {
			srcX := bounds.Min.X + x*srcW/w
			r, g, b, _ := src.At(srcX, srcY).RGBA()
			dstOff := dst.PixOffset(x, y)
			dst.Pix[dstOff] = uint8(r >> 8)
			dst.Pix[dstOff+1] = uint8(g >> 8)
			dst.Pix[dstOff+2] = uint8(b >> 8)
			dst.Pix[dstOff+3] = 0xff
		}
	}
	return dst
}

// NewPreviewContainer creates a preview with a border and label.
func NewPreviewContainer(bc *stream.Broadcaster) fyne.CanvasObject {
	preview := NewPreview(bc)
	title := canvas.NewText("LIVE PREVIEW", accentRed)
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.TextSize = 11

	underline := canvas.NewRectangle(accentRed)
	underline.Resize(fyne.NewSize(9999, 1))

	return container.NewBorder(
		container.NewVBox(title, underline),
		nil, nil, nil,
		preview,
	)
}
