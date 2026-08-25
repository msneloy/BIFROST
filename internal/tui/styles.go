package tui

import "github.com/charmbracelet/lipgloss"

// Color palette — dark theme inspired by bashtop/btop.
var (
	ColorPrimary   = lipgloss.Color("#e53935") // Red accent
	ColorSecondary = lipgloss.Color("#4caf50") // Green
	ColorWarn      = lipgloss.Color("#ffc107") // Yellow
	ColorDim       = lipgloss.Color("#666666")
	ColorBg        = lipgloss.Color("#1a1a2e")
	ColorSurface   = lipgloss.Color("#16213e")
	ColorBorder    = lipgloss.Color("#0f3460")
	ColorText      = lipgloss.Color("#e0e0e0")
	ColorTextDim   = lipgloss.Color("#888888")
	ColorAccent    = lipgloss.Color("#53a8b6")
)

// Header styles.
var (
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			MarginBottom(1)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(ColorTextDim).
			MarginBottom(1)

	StatusOnlineStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorSecondary)

	StatusOfflineStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorPrimary)

	URLStyle = lipgloss.NewStyle().
			Foreground(ColorAccent).
			Underline(true)
)

// Panel styles.
var (
	PanelBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorBorder).
				Padding(0, 1)

	PanelTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorAccent).
			MarginBottom(0)
)

// Stats bar styles.
var (
	BarFilledStyle = lipgloss.NewStyle().Foreground(ColorSecondary)
	BarEmptyStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#333333"))
	BarWarnStyle   = lipgloss.NewStyle().Foreground(ColorWarn)
	BarCritStyle   = lipgloss.NewStyle().Foreground(ColorPrimary)

	StatLabelStyle = lipgloss.NewStyle().Foreground(ColorTextDim).Width(6)
	StatValueStyle = lipgloss.NewStyle().Foreground(ColorText).Bold(true)
	StatPctStyle   = lipgloss.NewStyle().Foreground(ColorTextDim).Width(4).Align(lipgloss.Right)
)

// Table styles.
var (
	TableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorAccent).
				BorderBottom(true).
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(ColorBorder)

	TableCellStyle = lipgloss.NewStyle().
			Foreground(ColorText).
			PaddingRight(2)

	TableActiveDotStyle = lipgloss.NewStyle().Foreground(ColorSecondary)
	TableIdleDotStyle   = lipgloss.NewStyle().Foreground(ColorDim)
)

// Footer / help styles.
var (
	HelpKeyStyle  = lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
	HelpDescStyle = lipgloss.NewStyle().Foreground(ColorTextDim)
	FooterStyle   = lipgloss.NewStyle().
			Foreground(ColorTextDim).
			BorderTop(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(ColorBorder).
			PaddingTop(1)
)
