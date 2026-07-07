// Package tui implements a Bubble Tea TUI for monitoring container logs
// and stats in a split-screen layout.
package tui

import (
	"ctrwatch/internal/runtime"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const maxLines = 200

var panelColors = []lipgloss.Color{
	lipgloss.Color("12"),
	lipgloss.Color("10"),
	lipgloss.Color("11"),
	lipgloss.Color("13"),
}

type logLineMsg runtime.LogLine

type statsMsg struct {
	Stats map[string]*runtime.ContainerStats
}

// Model is the Bubble Tea model for the TUI.
type Model struct {
	containers []string
	lines      map[string][]runtime.LogLine
	stats      map[string]*runtime.ContainerStats
	linesCh    chan runtime.LogLine
	statsCh    chan map[string]*runtime.ContainerStats
	width      int
	height     int
}

// NewModel creates a new TUI model for the given container names.
func NewModel(containers []string) *Model {
	return &Model{
		containers: containers,
		lines:      make(map[string][]runtime.LogLine),
		stats:      make(map[string]*runtime.ContainerStats),
		linesCh:    make(chan runtime.LogLine, 256),
		statsCh:    make(chan map[string]*runtime.ContainerStats, 4),
		width:      80,
		height:     24,
	}
}

// LinesCh returns the send-only channel for incoming log lines.
func (m *Model) LinesCh() chan<- runtime.LogLine { return m.linesCh }

// StatsCh returns the send-only channel for incoming stats snapshots.
func (m *Model) StatsCh() chan<- map[string]*runtime.ContainerStats { return m.statsCh }

// Init starts the channel listeners.
func (m *Model) Init() tea.Cmd {
	return tea.Batch(listenLogs(m.linesCh), listenStats(m.statsCh))
}

func listenLogs(ch <-chan runtime.LogLine) tea.Cmd {
	return func() tea.Msg {
		line, ok := <-ch
		if !ok {
			return nil
		}
		return logLineMsg(line)
	}
}

func listenStats(ch <-chan map[string]*runtime.ContainerStats) tea.Cmd {
	return func() tea.Msg {
		s, ok := <-ch
		if !ok {
			return nil
		}
		return statsMsg{s}
	}
}

// Update handles Bubble Tea messages.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case logLineMsg:
		ll := runtime.LogLine(msg)
		buf := m.lines[ll.Container]
		buf = append(buf, ll)
		if len(buf) > maxLines {
			buf = buf[len(buf)-maxLines:]
		}
		m.lines[ll.Container] = buf
		return m, listenLogs(m.linesCh)

	case statsMsg:
		for name, s := range msg.Stats {
			m.stats[name] = s
		}
		return m, listenStats(m.statsCh)
	}

	return m, nil
}

// View renders the split-screen layout.
func (m *Model) View() string {
	if len(m.containers) == 0 {
		return "no containers"
	}

	n := len(m.containers)
	colWidth := m.width / n

	var cols []string
	for i, name := range m.containers {
		color := panelColors[i%len(panelColors)]
		buf := m.lines[name]

		title := lipgloss.NewStyle().
			Bold(true).
			Foreground(color).
			Render(name)

		statLine := ""
		statusBadge := ""
		if s, ok := m.stats[name]; ok {
			memMB := float64(s.MemoryUsage) / 1024 / 1024
			statLine = fmt.Sprintf("CPU: %.1f%%  MEM: %.0fMB", s.CPUPercent, memMB)
			statusColor := lipgloss.Color("10")
			if s.Status != "running" {
				statusColor = lipgloss.Color("11")
			}
			statusBadge = lipgloss.NewStyle().
				Foreground(statusColor).
				Render(s.Status)
		}

		headerText := title
		if statusBadge != "" {
			headerText += " " + statusBadge
		}
		if statLine != "" {
			headerText += "\n" + statLine
		}

		header := lipgloss.NewStyle().
			Width(colWidth).
			Align(lipgloss.Center).
			Render(headerText)

		border := lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(color).
			Width(colWidth)

		header = border.Render(header)

		borderHeight := 2
		headerLines := 1
		if statLine != "" {
			headerLines = 2
		}
		bodyHeight := m.height - borderHeight - headerLines - 1
		start := 0
		if len(buf) > bodyHeight {
			start = len(buf) - bodyHeight
		}

		stderrStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
		var bodyLines []string
		for _, l := range buf[start:] {
			truncated := l.Text
			if len(truncated) > colWidth-4 {
				truncated = truncated[:colWidth-4]
			}
			line := truncated
			if l.Stream == 2 {
				line = stderrStyle.Render(truncated)
			}
			bodyLines = append(bodyLines, line)
		}
		body := strings.Join(bodyLines, "\n")

		col := lipgloss.JoinVertical(lipgloss.Top, header, body)
		cols = append(cols, col)
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, cols...)
}
