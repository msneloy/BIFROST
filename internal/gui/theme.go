package gui

import "image/color"

// BIFROST Red theme
var (
	// Backgrounds
	bgDeep    = color.RGBA{R: 14, G: 8, B: 8, A: 255}     // #0e0808 main bg
	bgCard    = color.RGBA{R: 24, G: 14, B: 14, A: 255}    // #180e0e card bg
	bgCardAlt = color.RGBA{R: 32, G: 18, B: 18, A: 255}    // #201212 hover/alt
	bgBar     = color.RGBA{R: 36, G: 20, B: 20, A: 255}    // #241414 bar track

	// Accents
	accentRed    = color.RGBA{R: 255, G: 51, B: 51, A: 255}   // #ff3333
	accentCrimson = color.RGBA{R: 220, G: 40, B: 60, A: 255}  // #dc283c
	accentOrange = color.RGBA{R: 255, G: 100, B: 60, A: 255}  // #ff643c

	// Status
	statusGreen  = color.RGBA{R: 102, G: 187, B: 106, A: 255} // #66bb6a
	statusOrange = color.RGBA{R: 255, G: 167, B: 38, A: 255}  // #ffa726
	statusRed    = color.RGBA{R: 255, G: 51, B: 51, A: 255}   // #ff3333
	statusYellow = color.RGBA{R: 255, G: 238, B: 88, A: 255}  // #ffee58

	// Text
	textPrimary   = color.RGBA{R: 240, G: 230, B: 230, A: 255} // #f0e6e6
	textSecondary = color.RGBA{R: 170, G: 140, B: 140, A: 255} // #aa8c8c
	textDim       = color.RGBA{R: 110, G: 80, B: 80, A: 255}   // #6e5050
	textMuted     = color.RGBA{R: 70, G: 50, B: 50, A: 255}    // #463232

	// Bar gradient endpoints
	barLow  = color.RGBA{R: 255, G: 160, B: 60, A: 255}  // orange at low
	barMid  = color.RGBA{R: 255, G: 80, B: 60, A: 255}   // red-orange at mid
	barHigh = color.RGBA{R: 220, G: 30, B: 50, A: 255}   // deep red at high
)

// gradientColor returns a 3-stop gradient: orange -> red -> deep red
// based on a 0.0-1.0 fill percentage.
func gradientColor(t float32) color.RGBA {
	if t < 0.5 {
		return lerpColor(barLow, barMid, t*2)
	}
	return lerpColor(barMid, barHigh, (t-0.5)*2)
}

func lerpColor(a, b color.RGBA, t float32) color.RGBA {
	return color.RGBA{
		R: uint8(float32(a.R) + (float32(b.R)-float32(a.R))*t),
		G: uint8(float32(a.G) + (float32(b.G)-float32(a.G))*t),
		B: uint8(float32(a.B) + (float32(b.B)-float32(a.B))*t),
		A: 255,
	}
}
