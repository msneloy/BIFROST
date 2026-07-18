package gui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

type bifrostTheme struct{}

var _ fyne.Theme = (*bifrostTheme)(nil)

func (b *bifrostTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return color.NRGBA{R: 12, G: 12, B: 12, A: 255}
	case theme.ColorNameButton:
		return color.NRGBA{R: 30, G: 15, B: 15, A: 255}
	case theme.ColorNameDisabledButton:
		return color.NRGBA{R: 25, G: 25, B: 25, A: 255}
	case theme.ColorNameForeground:
		return color.NRGBA{R: 230, G: 230, B: 230, A: 255}
	case theme.ColorNamePlaceHolder:
		return color.NRGBA{R: 100, G: 100, B: 100, A: 255}
	case theme.ColorNameHover:
		return color.NRGBA{R: 229, G: 57, B: 53, A: 40}
	case theme.ColorNameInputBackground:
		return color.NRGBA{R: 20, G: 20, B: 20, A: 255}
	case theme.ColorNameInputBorder:
		return color.NRGBA{R: 50, G: 50, B: 50, A: 255}
	case theme.ColorNameSeparator:
		return color.NRGBA{R: 40, G: 40, B: 40, A: 255}
	case theme.ColorNameScrollBar:
		return color.NRGBA{R: 60, G: 60, B: 60, A: 255}
	case theme.ColorNamePrimary:
		return color.NRGBA{R: 229, G: 57, B: 53, A: 255}
	case theme.ColorNameOverlayBackground:
		return color.NRGBA{R: 18, G: 18, B: 18, A: 255}
	case theme.ColorNameHeaderBackground:
		return color.NRGBA{R: 16, G: 10, B: 10, A: 255}
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
		return 14
	case theme.SizeNameHeadingText:
		return 20
	case theme.SizeNameSubHeadingText:
		return 16
	case theme.SizeNameCaptionText:
		return 11
	case theme.SizeNamePadding:
		return 8
	case theme.SizeNameInnerPadding:
		return 12
	case theme.SizeNameScrollBar:
		return 8
	}
	return theme.DefaultTheme().Size(name)
}
