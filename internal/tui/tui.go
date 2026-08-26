// Package tui provides a terminal user interface for BIFROST using bubbletea.
// Inspired by bashtop, btop, glances, and htop.
package tui

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/nelobster/bifrost/internal/config"
	"github.com/nelobster/bifrost/internal/stats"
	"github.com/nelobster/bifrost/internal/tracker"
)

const maxLogLines = 5

// --- Log writer that feeds into bubbletea ---

// LogWriter is an io.Writer that sends log lines as tea.Msg to a bubbletea program.
type LogWriter struct {
	mu  sync.Mutex
	p   *tea.Program
	buf []byte
}

// SetProgram attaches the bubbletea program after it's created.
func (lw *LogWriter) SetProgram(p *tea.Program) {
	lw.mu.Lock()
	defer lw.mu.Unlock()
	lw.p = p
}

func (lw *LogWriter) Write(p []byte) (n int, err error) {
	lw.mu.Lock()
	defer lw.mu.Unlock()
	// Accumulate until we have a full line
	lw.buf = append(lw.buf, p...)
	for {
		idx := indexOf(lw.buf, '\n')
		if idx < 0 {
			break
		}
		line := string(lw.buf[:idx])
		lw.buf = lw.buf[idx+1:]
		if lw.p != nil {
			lw.p.Send(logMsg(line))
		}
	}
	return len(p), nil
}

func indexOf(b []byte, c byte) int {
	for i, v := range b {
		if v == c {
			return i
		}
	}
	return -1
}

// NewLogWriter creates a LogWriter and sets it as the global log output.
func NewLogWriter() *LogWriter {
	lw := &LogWriter{}
	log.SetOutput(lw)
	log.SetFlags(log.Ldate | log.Ltime)
	return lw
}

// ClientProvider is the interface the TUI needs to query connected clients.
type ClientProvider interface {
	GetAll() []*tracker.Client
	CountActive() int
}

// CaptureController is the interface the TUI needs to control the stream.
type CaptureController interface {
	IsStreaming() bool
	Start() error
	Stop()
}

// --- Messages ---

type tickMsg time.Time
type logMsg string

// --- Key bindings ---

type keyMap struct {
	Quit    key.Binding
	Toggle  key.Binding
	Help    key.Binding
	Refresh key.Binding
}

var keys = keyMap{
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	Toggle: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "start/stop stream"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	),
	Refresh: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "refresh"),
	),
}

// --- Model ---

type Model struct {
	cfg      *config.Config
	capture  CaptureController
	tracker  ClientProvider
	viewport viewport.Model
	width    int
	height   int
	quitting bool
	showHelp bool

	// Cached stats (updated on tick)
	sysStats  *stats.SystemStats
	clients   []*tracker.Client
	streaming bool
	logLines  []string
}

// New creates a new TUI model.
func New(cfg *config.Config, cap CaptureController, trk ClientProvider) Model {
	return Model{
		cfg:      cfg,
		capture:  cap,
		tracker:  trk,
		viewport: viewport.New(0, 0),
	}
}

// Init implements tea.Model. Starts the tick timer.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(),
		tea.EnterAltScreen,
	)
}

func tickCmd() tea.Cmd {
	return tea.Tick(1500*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Quit):
			m.quitting = true
			return m, tea.Quit

		case key.Matches(msg, keys.Toggle):
			if m.capture.IsStreaming() {
				m.capture.Stop()
			} else {
				m.capture.Start()
			}
			return m, nil

		case key.Matches(msg, keys.Help):
			m.showHelp = !m.showHelp
			return m, nil

		case key.Matches(msg, keys.Refresh):
			return m, nil
		}

	case tickMsg:
		m.sysStats = stats.Collect()
		m.clients = m.tracker.GetAll()
		m.streaming = m.capture.IsStreaming()
		return m, tickCmd()

	case logMsg:
		m.logLines = append(m.logLines, string(msg))
		if len(m.logLines) > maxLogLines {
			m.logLines = m.logLines[len(m.logLines)-maxLogLines:]
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// View implements tea.Model.
func (m Model) View() string {
	if m.quitting {
		return "\n  BIFROST stopped. Goodbye!\n\n"
	}

	var b strings.Builder

	// Header
	b.WriteString(m.renderHeader())
	b.WriteString("\n")

	// Stats panels (side by side)
	leftPanel := m.renderSystemStats()
	rightPanel := m.renderStreamInfo()

	leftW := lipgloss.Width(leftPanel)
	rightW := m.width - leftW - 2 // 2 for gap
	if rightW < 30 {
		rightW = m.width - 2
	}

	if rightW > 30 {
		row := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
		b.WriteString(row)
	} else {
		b.WriteString(leftPanel)
		b.WriteString("\n")
		b.WriteString(rightPanel)
	}
	b.WriteString("\n")

	// Clients table
	b.WriteString(m.renderClients())
	b.WriteString("\n")

	// Log panel
	b.WriteString(m.renderLogs())
	b.WriteString("\n")

	// Footer
	b.WriteString(m.renderFooter())

	return b.String()
}

// --- Header ---

func (m Model) renderHeader() string {
	title := TitleStyle.Render("██████╗ ██╗██████╗ ██████╗  ██████╗ ███████╗████████╗")
	title2 := TitleStyle.Render("██╔══██╗██║██╔══██╗██╔══██╗██╔═══██╗██╔════╝╚══██╔══╝")
	title3 := TitleStyle.Render("██████╔╝██║██████╔╝██║  ██║██║   ██║███████╗   ██║")
	title4 := TitleStyle.Render("██╔══██╗██║██╔══██╗██║  ██║██║   ██║╚════██║   ██║")
	title5 := TitleStyle.Render("██████╔╝██║██║  ██║██████╔╝╚██████╔╝███████║   ██║")
	title6 := TitleStyle.Render("╚═════╝ ╚═╝╚═╝  ╚═╝╚═════╝  ╚═════╝ ╚══════╝   ╚═╝")
	banner := lipgloss.JoinVertical(lipgloss.Left, title, title2, title3, title4, title5, title6)

	var statusLine string
	if m.streaming {
		statusLine = StatusOnlineStyle.Render("● STREAMING")
	} else {
		statusLine = StatusOfflineStyle.Render("● STOPPED")
	}

	url := fmt.Sprintf("http://bifrost.local:%d", m.cfg.Port)
	urlLine := URLStyle.Render(url)

	info := lipgloss.JoinVertical(lipgloss.Left,
		statusLine,
		lipgloss.NewStyle().Foreground(ColorTextDim).Render("Students connect to: "+urlLine),
		lipgloss.NewStyle().Foreground(ColorTextDim).Render(fmt.Sprintf("Resolution: %s | FPS: %d", m.cfg.Resolution, m.cfg.FPS)),
	)

	return lipgloss.JoinHorizontal(lipgloss.Center, banner, "    ", info)
}

// --- System stats panel ---

func (m Model) renderSystemStats() string {
	s := m.sysStats
	if s == nil {
		return PanelBorderStyle.Width(50).Render(PanelTitleStyle.Render("SYSTEM") + "\n  Collecting...")
	}

	barWidth := 20

	cpuBar := renderBar(s.CPUUsage, barWidth, "auto")
	memBar := renderBar(s.MemPct, barWidth, "auto")
	diskBar := renderBar(s.DiskPct, barWidth, "auto")

	var content strings.Builder
	content.WriteString(PanelTitleStyle.Render("  SYSTEM") + "\n\n")
	content.WriteString(fmt.Sprintf("  CPU   %s %3d%%  %d MHz  %.0f°C\n", cpuBar, s.CPUUsage, s.CPUFreqMHz, float64(s.CPUTempC)))
	content.WriteString(fmt.Sprintf("  MEM   %s %3d%%  %.1fG / %.1fG\n", memBar, s.MemPct, s.MemUsedGB, s.MemTotalGB))
	content.WriteString(fmt.Sprintf("  DISK  %s %3d%%  %s / %s\n", diskBar, s.DiskPct, s.DiskUsed, s.DiskTotal))

	if s.SwapTotalGB > 0 {
		swapBar := renderBar(s.SwapPct, barWidth, "auto")
		content.WriteString(fmt.Sprintf("  SWAP  %s %3d%%  %.1fG / %.1fG\n", swapBar, s.SwapPct, s.SwapUsedGB, s.SwapTotalGB))
	}

	content.WriteString(fmt.Sprintf("\n  NET   %s  %s  %d Mbps\n", s.NICName, s.NICType, s.NICSpeed))
	content.WriteString(fmt.Sprintf("  LOAD  %.2f\n", s.LoadAvg))
	content.WriteString(fmt.Sprintf("  UP    %s", s.Uptime))

	if s.CPUModel != "" {
		content.WriteString(fmt.Sprintf("\n  CPU   %s", truncate(s.CPUModel, 40)))
	}

	return PanelBorderStyle.Width(50).Render(content.String())
}

// --- Stream info panel ---

func (m Model) renderStreamInfo() string {
	var content strings.Builder
	content.WriteString(PanelTitleStyle.Render("  STREAM") + "\n\n")

	if m.streaming {
		content.WriteString(fmt.Sprintf("  Status    %s\n", StatusOnlineStyle.Render("LIVE")))
	} else {
		content.WriteString(fmt.Sprintf("  Status    %s\n", StatusOfflineStyle.Render("STOPPED")))
	}

	clientCount := m.tracker.CountActive()
	content.WriteString(fmt.Sprintf("  Clients   %d connected\n", clientCount))
	content.WriteString("  Protocol  WebRTC (VP8 + Opus)\n")

	if !m.cfg.NoAudio {
		content.WriteString("  Audio     Enabled (Opus)\n")
	} else {
		content.WriteString("  Audio     Disabled\n")
	}

	content.WriteString(fmt.Sprintf("\n  Port      %d\n", m.cfg.Port))
	content.WriteString(fmt.Sprintf("  Address   bifrost.local\n"))

	return PanelBorderStyle.Width(44).Render(content.String())
}

// --- Clients table ---

func (m Model) renderClients() string {
	var content strings.Builder
	content.WriteString(PanelTitleStyle.Render("  CONNECTED CLIENTS") + "\n\n")

	if len(m.clients) == 0 {
		content.WriteString("  " + lipgloss.NewStyle().Foreground(ColorTextDim).Render("No clients connected"))
		return PanelBorderStyle.Width(m.width - 4).Render(content.String())
	}

	// Table header
	header := fmt.Sprintf("  %-6s %-18s %-12s %-10s %-8s %-10s",
		"STATUS", "IP", "HOSTNAME", "OS", "BROWSER", "TRANSFERRED")
	content.WriteString(TableHeaderStyle.Width(m.width-8).Render(header) + "\n")

	// Table rows
	for _, c := range m.clients {
		var statusDot string
		if c.Active {
			statusDot = TableActiveDotStyle.Render("●")
		} else {
			statusDot = TableIdleDotStyle.Render("○")
		}

		hostname := c.Host
		if hostname == "" {
			hostname = "-"
		}

		row := fmt.Sprintf("  %-6s %-18s %-12s %-10s %-8s %-10s",
			statusDot,
			truncate(c.IP, 18),
			truncate(hostname, 12),
			truncate(c.OS, 10),
			truncate(c.Browser, 8),
			formatBytes(c.Bytes),
		)
		content.WriteString(TableCellStyle.Width(m.width-8).Render(row) + "\n")
	}

	return PanelBorderStyle.Width(m.width - 4).Render(content.String())
}

// --- Log panel ---

func (m Model) renderLogs() string {
	var content strings.Builder
	content.WriteString(PanelTitleStyle.Render("  LOG") + "\n")

	if len(m.logLines) == 0 {
		content.WriteString("  " + lipgloss.NewStyle().Foreground(ColorTextDim).Render("No log messages"))
	} else {
		for _, line := range m.logLines {
			content.WriteString("  " + lipgloss.NewStyle().Foreground(ColorTextDim).Render(truncate(line, m.width-8)) + "\n")
		}
	}

	return PanelBorderStyle.Width(m.width - 4).Render(content.String())
}

// --- Footer / help ---

func (m Model) renderFooter() string {
	if m.showHelp {
		help := lipgloss.JoinVertical(lipgloss.Left,
			HelpKeyStyle.Render("  s")+"  "+HelpDescStyle.Render("Start/stop the screen capture stream"),
			HelpKeyStyle.Render("  r")+"  "+HelpDescStyle.Render("Refresh the display"),
			HelpKeyStyle.Render("  ?")+"  "+HelpDescStyle.Render("Toggle this help"),
			HelpKeyStyle.Render("  q")+"  "+HelpDescStyle.Render("Quit BIFROST"),
		)
		return FooterStyle.Width(m.width).Render(help)
	}

	footer := fmt.Sprintf("  %s  %s  %s  %s",
		HelpKeyStyle.Render("s")+HelpDescStyle.Render(" stream"),
		HelpKeyStyle.Render("r")+HelpDescStyle.Render(" refresh"),
		HelpKeyStyle.Render("?")+HelpDescStyle.Render(" help"),
		HelpKeyStyle.Render("q")+HelpDescStyle.Render(" quit"),
	)
	return FooterStyle.Width(m.width).Render(footer)
}

// --- Helpers ---

func renderBar(pct, width int, mode string) string {
	if width < 4 {
		width = 4
	}
	filled := pct * (width - 2) / 100
	if filled > width-2 {
		filled = width - 2
	}
	empty := width - 2 - filled

	var style lipgloss.Style
	switch mode {
	case "auto":
		if pct >= 90 {
			style = BarCritStyle
		} else if pct >= 70 {
			style = BarWarnStyle
		} else {
			style = BarFilledStyle
		}
	default:
		style = BarFilledStyle
	}

	return "[" + style.Render(strings.Repeat("█", filled)) + BarEmptyStyle.Render(strings.Repeat("░", empty)) + "]"
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

func formatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%dB", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%cB", float64(b)/float64(div), "KMGTPE"[exp])
}
