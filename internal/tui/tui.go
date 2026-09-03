// Package tui provides a terminal user interface for BIFROST using bubbletea.
// Terminal dashboard for BIFROST using bubbletea.
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
		if m.tracker != nil {
			m.clients = m.tracker.GetAll()
		}
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
func (m Model) View() string {
	if m.quitting {
		return "\n  BIFROST stopped. Goodbye!\n\n"
	}

	hud := m.renderHUD()
	sys := m.renderSystemPanel()
	nodes := m.renderClientsPanel()
	logs := m.renderLogPanel()
	foot := m.renderFooter()

	return lipgloss.JoinVertical(lipgloss.Left, hud, sys, nodes, logs, foot)
}

// --- HUD (Status Bar) ---

func (m Model) renderHUD() string {
	var status string
	if m.streaming {
		status = StatusOnlineStyle.Render("◉LIVE")
	} else {
		status = StatusOfflineStyle.Render("◎STBY")
	}

	clientCount := 0
	if m.tracker != nil {
		clientCount = m.tracker.CountActive()
	}
	audio := "A:OFF"
	if !m.cfg.NoAudio {
		audio = "A:ON"
	}

	// Compact status bar
	// Example: [BIFROST 0.3] ◉LIVE  1920x1080@30  N:1  A:ON  bifrost.local:8080
	hud := fmt.Sprintf("[BIFROST %s] %s  %s@%d  N:%-2d  %s  bifrost.local:%d",
		"0.3",
		status,
		m.cfg.Resolution,
		m.cfg.FPS,
		clientCount,
		audio,
		m.cfg.Port,
	)
	// Constrain HUD to the same bordered panel width and truncate inner content
	maxInner := m.panelWidth() - 4
	if maxInner <= 0 {
		maxInner = 80
	}
	if len(hud) > maxInner {
		hud = truncate(hud, maxInner)
	}
	// Render HUD text then place inside the same PanelBorderStyle so it lines up
	inner := HUDBarStyle.Render(hud)
	return PanelBorderStyle.Width(m.panelWidth()).Render(inner)
}

// panelWidth returns the width for all bordered panels.
// Matches the terminal width exactly so all boxes + footer stay flush.
func (m Model) panelWidth() int {
	w := m.width
	if w <= 0 {
		w = 80
	}
	return w
}

// --- System Telemetry Panel ---

func (m Model) renderSystemPanel() string {
	s := m.sysStats
	if s == nil {
		return PanelBorderStyle.Width(m.panelWidth()).Render(PanelTitleStyle.Render(" SYS TELEMETRY ") + "\n  ACQUIRING...\n")
	}

	// innerW = panel width minus border (2) minus lipgloss default padding (2).
	innerW := m.panelWidth() - 4
	if innerW < 20 {
		innerW = 20
	}

	// Bar width scales with available space.
	barW := 10
	switch {
	case innerW >= 100:
		barW = 20
	case innerW >= 80:
		barW = 16
	case innerW >= 60:
		barW = 12
	}

	// Visual columns before the detail field:
	// label(3) + sp(1) + bar(barW+2) + sp(1) + pct(3) + sp(1) = barW+11
	// Truncate detail as PLAIN TEXT before passing to statRow so that
	// truncate() never sees ANSI escape bytes (which inflate len()).
	detailW := innerW - barW - 11
	if detailW < 0 {
		detailW = 0
	}

	var c strings.Builder
	c.WriteString(PanelTitleStyle.Render(" SYS TELEMETRY ") + "\n")

	cpuDet := fmt.Sprintf("%dMHz", s.CPUFreqMHz)
	if s.CPUTempC > 0 {
		cpuDet = fmt.Sprintf("%dMHz %d°C", s.CPUFreqMHz, s.CPUTempC)
	}
	c.WriteString(statRow("CPU", s.CPUUsage, barW, truncate(cpuDet, detailW)))
	c.WriteString(statRow("MEM", s.MemPct, barW, truncate(fmt.Sprintf("%.1f/%.1fG", s.MemUsedGB, s.MemTotalGB), detailW)))
	c.WriteString(statRow("DSK", s.DiskPct, barW, truncate(fmt.Sprintf("%s/%s", s.DiskUsed, s.DiskTotal), detailW)))
	if s.SwapTotalGB > 0 {
		c.WriteString(statRow("SWP", s.SwapPct, barW, truncate(fmt.Sprintf("%.1f/%.1fG", s.SwapUsedGB, s.SwapTotalGB), detailW)))
	}
	if s.GPUTempC > 0 {
		c.WriteString(tempRow("GPU", s.GPUTempC, barW))
	}

	// Aux line is plain text — safe to truncate normally.
	aux := ""
	if s.FanRPM > 0 {
		aux += fmt.Sprintf("FAN:%-4d ", s.FanRPM)
	}
	nicSpd := s.NICSpeed
	if nicSpd > 1000 {
		nicSpd = nicSpd / 1000
	}
	aux += fmt.Sprintf("NET:%s@%dM  LD:%.1f  UP:%s",
		s.NICName, nicSpd, s.LoadAvg, s.Uptime)
	c.WriteString(truncate(aux, innerW) + "\n")

	inner := strings.TrimRight(c.String(), "\n")
	return PanelBorderStyle.Width(m.panelWidth()).Render(inner)
}

// statRow renders a compact stat line: label [████░░░░] 42% detail
// No leading spaces inside panel (panel padding is 0).
func statRow(label string, pct, barW int, detail string) string {
	lbl := lipgloss.NewStyle().Foreground(ColorTextDim).Width(3).Render(label)
	bar := renderBar(pct, barW, "auto")
	pctStr := lipgloss.NewStyle().Foreground(ColorText).Width(3).Align(lipgloss.Right).Render(fmt.Sprintf("%d%%", pct))
	det := lipgloss.NewStyle().Foreground(ColorTextDim).Render(detail)
	return fmt.Sprintf("%s %s %s %s\n", lbl, bar, pctStr, det)
}

// tempRow renders a compact temperature line: label [████░░░░] 65°C
func tempRow(label string, tempC, barW int) string {
	lbl := lipgloss.NewStyle().Foreground(ColorTextDim).Width(3).Render(label)
	bar := renderTempBar(tempC, barW)
	tempStr := lipgloss.NewStyle().Foreground(tempColor(tempC)).Width(4).Align(lipgloss.Right).Render(fmt.Sprintf("%d°C", tempC))
	return fmt.Sprintf("%s %s %s\n", lbl, bar, tempStr)
}

// renderTempBar renders a compact temperature bar (thermal colors).
func renderTempBar(tempC, width int) string {
	pct := tempC
	if pct > 100 {
		pct = 100
	}
	filled := pct * (width - 2) / 100
	if filled > width-2 {
		filled = width - 2
	}
	empty := width - 2 - filled
	style := tempColor(tempC)
	return "[" + lipgloss.NewStyle().Foreground(style).Render(strings.Repeat("█", filled)) +
		BarEmptyStyle.Render(strings.Repeat("░", empty)) + "]"
}

func tempColor(tempC int) lipgloss.Color {
	switch {
	case tempC >= 85:
		return ColorCrit // red - critical overheat
	case tempC >= 70:
		return ColorWarn // amber - caution
	default:
		return ColorSecondary // cyan - nominal
	}
}

// --- Connected Clients Panel ---

func (m Model) renderClientsPanel() string {
	var content strings.Builder
	content.WriteString(PanelTitleStyle.Render(" NODES ") + "\n")

	if len(m.clients) == 0 {
		content.WriteString(lipgloss.NewStyle().Foreground(ColorTextDim).Render("  NO REMOTE NODES LINKED\n"))
		return PanelBorderStyle.Width(m.panelWidth()).Render(content.String())
	}

	// Use deterministic fixed column widths and truncate long fields to ensure stable alignment
	statW := 6
	ipW := 15
	hostW := 20
	devW := 10
	osW := 10
	brW := 10
	resW := 11
	txW := 6

	// Header
	statHdr := fmt.Sprintf("%-*s", statW, "STAT")
	ipHdr := fmt.Sprintf("%-*s", ipW, "IP")
	hostHdr := fmt.Sprintf("%-*s", hostW, "HOST")
	devHdr := fmt.Sprintf("%-*s", devW, "DEVICE")
	osHdr := fmt.Sprintf("%-*s", osW, "OS")
	brHdr := fmt.Sprintf("%-*s", brW, "BROWSER")
	resHdr := fmt.Sprintf("%-*s", resW, "RES")
	txHdr := fmt.Sprintf("%-*s", txW, "TX")
	hdrLine := fmt.Sprintf("  %s  %s  %s  %s  %s  %s  %s  %s", statHdr, ipHdr, hostHdr, devHdr, osHdr, brHdr, resHdr, txHdr)
	content.WriteString(TableCellStyle.Render(hdrLine) + "\n")

	for _, c := range m.clients {
		statusPlain := "○ IDLE"
		if c.Active {
			statusPlain = "▶ LINK"
		}

		ip := c.IP
		if ip == "" {
			ip = "-"
		}
		host := c.Host
		if host == "" {
			host = "-"
		}
		device := c.Device
		if device == "" {
			device = "-"
		}
		os := c.OS
		if os == "" {
			os = "-"
		}
		browser := c.Browser
		if browser == "" {
			browser = "-"
		}
		res := c.Resolution
		if res == "" {
			res = "-"
		}
		tx := formatBytes(c.Bytes)

		// Truncate long values so they fit their columns
		ipT := truncate(ip, ipW)
		hostT := truncate(host, hostW)
		devT := truncate(device, devW)
		osT := truncate(os, osW)
		brT := truncate(browser, brW)
		resT := truncate(res, resW)
		txT := truncate(tx, txW)

		statField := fmt.Sprintf("%-*s", statW, statusPlain)
		ipField := fmt.Sprintf("%-*s", ipW, ipT)
		hostField := fmt.Sprintf("%-*s", hostW, hostT)
		devField := fmt.Sprintf("%-*s", devW, devT)
		osField := fmt.Sprintf("%-*s", osW, osT)
		brField := fmt.Sprintf("%-*s", brW, brT)
		resField := fmt.Sprintf("%-*s", resW, resT)
		txField := fmt.Sprintf("%-*s", txW, txT)

		styledStat := TableIdleDotStyle.Render(statField)
		if c.Active {
			styledStat = TableActiveDotStyle.Render(statField)
		}

		line := fmt.Sprintf("  %s  %s  %s  %s  %s  %s  %s  %s", styledStat, ipField, hostField, devField, osField, brField, resField, txField)
		content.WriteString(TableCellStyle.Render(line) + "\n")
	}

	return PanelBorderStyle.Width(m.panelWidth()).Render(strings.TrimRight(content.String(), "\n"))
}

// --- Log Feed Panel ---

func (m Model) renderLogPanel() string {
	var content strings.Builder
	content.WriteString(PanelTitleStyle.Render(" LOG FEED ") + "\n")

	if len(m.logLines) == 0 {
		content.WriteString(lipgloss.NewStyle().Foreground(ColorTextDim).Render("  NO EVENTS\n"))
		return PanelBorderStyle.Width(m.panelWidth()).Render(content.String())
	}

	for _, line := range m.logLines {
		// truncate each log line to panel inner width to avoid expanding the border
		maxInner := m.panelWidth() - 4
		content.WriteString("  " + lipgloss.NewStyle().Foreground(ColorTextDim).Render(truncate(line, maxInner-2)) + "\n")
	}

	inner := strings.TrimRight(content.String(), "\n")
	return PanelBorderStyle.Width(m.panelWidth()).Render(inner)
}

// --- Footer / help ---
// Footer command legend.

func (m Model) renderFooter() string {
	pw := m.panelWidth()
	if m.showHelp {
		help := lipgloss.JoinVertical(lipgloss.Left,
			HelpKeyStyle.Render("[s]")+HelpDescStyle.Render(" STREAM"),
			HelpKeyStyle.Render("[r]")+HelpDescStyle.Render(" REFRESH"),
			HelpKeyStyle.Render("[?]")+HelpDescStyle.Render(" HELP"),
			HelpKeyStyle.Render("[q]")+HelpDescStyle.Render(" QUIT"),
		)
		return FooterStyle.Width(pw).Render(help)
	}
	footer := fmt.Sprintf("%s STREAM   %s REFRESH   %s HELP   %s QUIT",
		HelpKeyStyle.Render("[s]"),
		HelpKeyStyle.Render("[r]"),
		HelpKeyStyle.Render("[?]"),
		HelpKeyStyle.Render("[q]"),
	)
	return FooterStyle.Width(pw).Render(footer)
}

// --- Helpers ---

// renderBar renders a compact usage bar (bashtop style).
// mode "auto" applies warning/critical colors.
func renderBar(pct, width int, mode string) string {
	if width < 6 {
		width = 6
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
	if max <= 0 {
		max = 80 // safe default for early renders before WindowSize
	}
	if len(s) <= max {
		return s
	}
	if max <= 1 {
		return "…"
	}
	return s[:max-1] + "…"
}

// safeWidth returns a usable width for truncation, defaulting before first resize.
func (m Model) safeWidth(delta int) int {
	w := m.width
	if w <= 0 {
		w = 100
	}
	w += delta
	if w < 10 {
		w = 10
	}
	return w
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
