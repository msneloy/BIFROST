package gui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

type bifrostTheme struct{}

var _ fyne.Theme = (*bifrostTheme)(nil)

// Web admin color palette
var (
	ColorBG         = color.NRGBA{R: 12, G: 12, B: 12, A: 255}       // #0c0c0c
	ColorSurface    = color.NRGBA{R: 20, G: 20, B: 20, A: 255}       // #141414
	ColorSurface2   = color.NRGBA{R: 26, G: 26, B: 26, A: 255}       // #1a1a1a
	ColorBorder     = color.NRGBA{R: 37, G: 37, B: 37, A: 255}       // #252525
	ColorBorder2    = color.NRGBA{R: 51, G: 51, B: 51, A: 255}       // #333
	ColorText       = color.NRGBA{R: 232, G: 232, B: 232, A: 255}    // #e8e8e8
	ColorText2      = color.NRGBA{R: 153, G: 153, B: 153, A: 255}    // #999
	ColorText3      = color.NRGBA{R: 102, G: 102, B: 102, A: 255}    // #666
	ColorAccent     = color.NRGBA{R: 229, G: 57, B: 53, A: 255}      // #e53935
	ColorAccentDim  = color.NRGBA{R: 183, G: 28, B: 28, A: 255}      // #b71c1c
	ColorGreen      = color.NRGBA{R: 76, G: 175, B: 80, A: 255}      // #4caf50
	ColorYellow     = color.NRGBA{R: 255, G: 193, B: 7, A: 255}      // #ffc107
	ColorOrange     = color.NRGBA{R: 255, G: 152, B: 0, A: 255}      // #ff9800
	ColorRed        = color.NRGBA{R: 229, G: 57, B: 53, A: 255}      // #e53935
	ColorHeader     = color.NRGBA{R: 16, G: 10, B: 10, A: 255}       // #160a0a
	ColorCardHeader = color.NRGBA{R: 22, G: 22, B: 22, A: 255}       // #161616
	ColorWhite      = color.NRGBA{R: 255, G: 255, B: 255, A: 255}    // #fff
)

func (b *bifrostTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return ColorBG
	case theme.ColorNameButton:
		return ColorSurface2
	case theme.ColorNameDisabledButton:
		return ColorSurface
	case theme.ColorNameForeground:
		return ColorText
	case theme.ColorNamePlaceHolder:
		return ColorText3
	case theme.ColorNameHover:
		return color.NRGBA{R: 229, G: 57, B: 53, A: 30}
	case theme.ColorNameInputBackground:
		return ColorSurface
	case theme.ColorNameInputBorder:
		return ColorBorder
	case theme.ColorNameSeparator:
		return ColorBorder
	case theme.ColorNameScrollBar:
		return ColorBorder2
	case theme.ColorNamePrimary:
		return ColorAccent
	case theme.ColorNameOverlayBackground:
		return ColorSurface
	case theme.ColorNameHeaderBackground:
		return ColorHeader
	case theme.ColorNameSuccess:
		return ColorGreen
	case theme.ColorNameWarning:
		return ColorYellow
	case theme.ColorNameError:
		return ColorRed
	}
	return theme.DefaultTheme().Color(name, variant)
}

func (b *bifrostTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

func (b *bifrostTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (b *bifrostTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNameText:
		return 13
	case theme.SizeNameHeadingText:
		return 20
	case theme.SizeNameSubHeadingText:
		return 15
	case theme.SizeNameCaptionText:
		return 11
	case theme.SizeNamePadding:
		return 12
	case theme.SizeNameInnerPadding:
		return 10
	case theme.SizeNameScrollBar:
		return 6
	case theme.SizeNameScrollBarSmall:
		return 4
	}
	return theme.DefaultTheme().Size(name)
}
