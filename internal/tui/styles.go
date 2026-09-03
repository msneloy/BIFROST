package tui

import "github.com/charmbracelet/lipgloss"

// Color palette — dark terminal, high contrast.
// Greens for nominal, amber for caution, red for critical. Cyan accents.
var (
	ColorPrimary   = lipgloss.Color("#00ff9f") // Green (nominal)
	ColorSecondary = lipgloss.Color("#00d4ff") // Cyan accent
	ColorWarn      = lipgloss.Color("#ffbf00") // Amber (caution)
	ColorCrit      = lipgloss.Color("#ff4757") // Red (critical)
	ColorDim       = lipgloss.Color("#4a4a4a")
	ColorBorder    = lipgloss.Color("#00aa77") // Mil green border
	ColorText      = lipgloss.Color("#e8e8e8")
	ColorTextDim   = lipgloss.Color("#8a8a8a")
	ColorAccent    = lipgloss.Color("#00d4ff")
	ColorBg        = lipgloss.Color("#0a0f0a")
)

// Header / HUD styles.
var (
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(ColorTextDim)

	StatusOnlineStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorPrimary)

	StatusOfflineStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorCrit)

	URLStyle = lipgloss.NewStyle().
			Foreground(ColorAccent).
			Underline(true)

	// Full-width HUD bar (header)
	HUDBarStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true)

	// Section frame — double-line border for panel separation
	PanelBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.DoubleBorder()).
				BorderForeground(ColorBorder).
				Padding(0, 0)

	PanelTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorAccent)
)

// Divider line style for HUD
var (
	DividerStyle = lipgloss.NewStyle().
		Foreground(ColorBorder)
)

// Stats bar styles.
var (
	BarFilledStyle = lipgloss.NewStyle().Foreground(ColorPrimary) // green
	BarEmptyStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#222222"))
	BarWarnStyle   = lipgloss.NewStyle().Foreground(ColorWarn) // amber
	BarCritStyle   = lipgloss.NewStyle().Foreground(ColorCrit) // red

	StatLabelStyle = lipgloss.NewStyle().Foreground(ColorTextDim).Width(6)
	StatValueStyle = lipgloss.NewStyle().Foreground(ColorText).Bold(true)
	StatPctStyle   = lipgloss.NewStyle().Foreground(ColorTextDim).Width(4).Align(lipgloss.Right)
)

// Table styles. No heavy underline to save vertical space (bashtop/htop style).
var (
	TableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorAccent)

	TableCellStyle = lipgloss.NewStyle().
			Foreground(ColorText).
			PaddingRight(1)

	TableActiveDotStyle = lipgloss.NewStyle().Foreground(ColorSecondary)
	TableIdleDotStyle   = lipgloss.NewStyle().Foreground(ColorDim)
)

// Footer / help styles. Minimal for density (no border line to save vertical space).
var (
	HelpKeyStyle  = lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
	HelpDescStyle = lipgloss.NewStyle().Foreground(ColorTextDim)
	FooterStyle   = lipgloss.NewStyle().Foreground(ColorTextDim)
)
