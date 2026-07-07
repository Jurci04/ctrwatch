package tui

import (
	"fmt"
	"maps"
	"strings"

	"ctrwatch/internal/runtime"

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

type logBatchMsg []runtime.LogLine

type statsMsg struct {
	Stats map[string]*runtime.ContainerStats
}

type Model struct {
	containers []string
	lines      map[string][]runtime.LogLine
	stats      map[string]*runtime.ContainerStats
	linesCh    chan []runtime.LogLine
	statsCh    chan map[string]*runtime.ContainerStats
	width      int
	height     int
	selected   int
	focused    bool
}

func NewModel(containers []string) *Model {
	return &Model{
		containers: containers,
		lines:      make(map[string][]runtime.LogLine),
		stats:      make(map[string]*runtime.ContainerStats),
		linesCh:    make(chan []runtime.LogLine, 64),
		statsCh:    make(chan map[string]*runtime.ContainerStats, 4),
		width:      80,
		height:     24,
	}
}

func (m *Model) LinesCh() chan<- []runtime.LogLine                  { return m.linesCh }
func (m *Model) StatsCh() chan<- map[string]*runtime.ContainerStats { return m.statsCh }

func (m *Model) Init() tea.Cmd {
	return tea.Batch(listenLogs(m.linesCh), listenStats(m.statsCh))
}

func listenLogs(ch <-chan []runtime.LogLine) tea.Cmd {
	return func() tea.Msg {
		batch, ok := <-ch
		if !ok {
			return nil
		}
		return logBatchMsg(batch)
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

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "tab", "right", "down":
			if len(m.containers) > 0 {
				m.selected = (m.selected + 1) % len(m.containers)
			}
		case "shift+tab", "left", "up":
			if len(m.containers) > 0 {
				m.selected = (m.selected + len(m.containers) - 1) % len(m.containers)
			}
		case "enter", " ":
			m.focused = !m.focused
			return m, tea.ClearScreen
		case "a":
			m.focused = false
			return m, tea.ClearScreen
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case logBatchMsg:
		for _, ll := range []runtime.LogLine(msg) {
			buf := m.lines[ll.Container]
			buf = append(buf, ll)
			if len(buf) > maxLines {
				buf = buf[len(buf)-maxLines:]
			}
			m.lines[ll.Container] = buf
		}
		return m, listenLogs(m.linesCh)

	case statsMsg:
		maps.Copy(m.stats, msg.Stats)
		return m, listenStats(m.statsCh)
	}

	return m, nil
}

func (m *Model) View() string {
	if len(m.containers) == 0 {
		return "no containers"
	}

	containers := m.containers
	if m.focused {
		containers = []string{m.containers[m.selected]}
	}
	n := len(containers)
	panelRows := max(m.height-1, 1)
	bodyRows := max(panelRows/n-2, 0)
	panelW := max(m.width-2, 1)
	innerW := max(panelW-2, 1)

	var panels []string

	for i, name := range containers {
		contentHeight := bodyRows
		logSlots := contentHeight
		color := panelColors[i%len(panelColors)]
		selected := name == m.containers[m.selected]
		bStyle := lipgloss.NewStyle().Foreground(color).Bold(selected)
		vl := bStyle.Render("│")

		title := " " + name
		if s, ok := m.stats[name]; ok {
			memMB := float64(s.MemoryUsage) / 1024 / 1024
			status := s.Status
			if status == "" {
				status = "unknown"
			}
			title += fmt.Sprintf(" | %s | CPU %.1f%% | MEM %.0fMB", status, s.CPUPercent, memMB)
		}
		title = truncate(title+" ", max(innerW-2, 1))
		dashes := max(0, innerW-1-lipgloss.Width(title))
		topBorder := bStyle.Render("╭" + "─" + title + strings.Repeat("─", dashes) + "╮")

		var body []string

		// Log lines
		buf := m.lines[name]
		if len(buf) == 0 && len(m.stats) == 0 {
			w := lipgloss.NewStyle().Width(innerW).Render(
				lipgloss.NewStyle().Italic(true).Foreground(lipgloss.Color("8")).Render("waiting..."))
			body = append(body, vl+w+vl)
		} else {
			lines := visibleLogLines(buf, logSlots, innerW)
			for _, line := range lines {
				txt := lipgloss.NewStyle().Width(innerW).Render(colorLogLine(line))
				body = append(body, vl+txt+vl)
			}
		}

		// Pad/truncate body to exact contentHeight
		for len(body) < contentHeight {
			body = append(body, vl+strings.Repeat(" ", innerW)+vl)
		}
		if len(body) > contentHeight {
			body = body[:contentHeight]
		}

		// Bottom border
		bottomBorder := bStyle.Render("╰" + strings.Repeat("─", innerW) + "╯")

		panelLines := append([]string{topBorder}, body...)
		panelLines = append(panelLines, bottomBorder)
		panels = append(panels, strings.Join(panelLines, "\n"))
	}

	footer := fmt.Sprintf("%d/%d  tab switch  enter focus  a all  q quit", m.selected+1, len(m.containers))
	out := strings.Join(panels, "\n") + "\n" + lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Width(panelW).
		Render(truncate(footer, panelW))
	return fitHeight(out, panelW, m.height)
}

func truncate(s string, width int) string {
	if width <= 0 || lipgloss.Width(s) <= width {
		return s
	}
	if width <= 3 {
		return takeWidth(s, width)
	}
	return takeWidth(s, width-3) + "..."
}

func takeWidth(s string, width int) string {
	var b strings.Builder
	used := 0
	for _, r := range s {
		w := lipgloss.Width(string(r))
		if used+w > width {
			break
		}
		b.WriteRune(r)
		used += w
	}
	return b.String()
}

func cleanLine(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\x1b' {
			i = skipANSI(s, i)
			continue
		}
		switch c {
		case '\r', '\n':
			continue
		case '\t':
			b.WriteString("    ")
		default:
			if c >= 0x20 {
				b.WriteByte(c)
			}
		}
	}
	return b.String()
}

type visibleLogLine struct {
	text   string
	stderr bool
}

func colorLogLine(line visibleLogLine) string {
	color, ok := logLineColor(line)
	if !ok {
		return line.text
	}
	return lipgloss.NewStyle().Foreground(color).Render(line.text)
}

func logLineColor(line visibleLogLine) (lipgloss.Color, bool) {
	if line.stderr {
		return lipgloss.Color("9"), true
	}
	text := strings.ToUpper(line.text)
	switch {
	case strings.Contains(text, "ERROR") || strings.Contains(text, "CRITICAL"):
		return lipgloss.Color("9"), true
	case strings.Contains(text, "WARNING") || strings.Contains(text, "WARN"):
		return lipgloss.Color("11"), true
	case strings.Contains(text, "INFO"):
		return lipgloss.Color("10"), true
	case strings.Contains(text, "DEBUG"):
		return lipgloss.Color("12"), true
	default:
		return "", false
	}
}

func visibleLogLines(buf []runtime.LogLine, limit, width int) []visibleLogLine {
	if limit <= 0 {
		return nil
	}
	lines := make([]visibleLogLine, 0, limit)
	for i := len(buf) - 1; i >= 0 && len(lines) < limit; i-- {
		txt := truncate(cleanLine(buf[i].Text), width)
		if txt == "" {
			continue
		}
		lines = append(lines, visibleLogLine{text: txt, stderr: buf[i].Stream == 2})
	}
	for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
		lines[i], lines[j] = lines[j], lines[i]
	}
	return lines
}

func skipANSI(s string, i int) int {
	if i+1 >= len(s) || s[i+1] != '[' {
		return i
	}
	i += 2
	for i < len(s) {
		if s[i] >= '@' && s[i] <= '~' {
			return i
		}
		i++
	}
	return len(s) - 1
}

func fitHeight(s string, width, height int) string {
	if height <= 0 {
		return s
	}
	lines := strings.Split(s, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", max(width, 0)))
	}
	return strings.Join(lines, "\n")
}
