package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m *Model) viewLogs(bodyHeight, panelW int) string {
	if m.focused {
		return m.viewLogFocused(bodyHeight, panelW)
	}
	if m.logSelectorOpen {
		return m.viewLogSelector(bodyHeight, panelW)
	}
	return m.viewLogPanels(bodyHeight, panelW)
}

func (m *Model) viewLogFocused(bodyHeight, panelW int) string {
	innerW := max(panelW-2, 1)
	contentHeight := bodyHeight - 2
	if contentHeight <= 0 {
		return ""
	}
	name := m.containers[m.selected]
	idx := m.indexOfContainer(name)
	bStyle := lipgloss.NewStyle()
	vl := bStyle.Render("│")

	title := fmt.Sprintf(" [%s] %s", m.containerRuntime(idx), m.containerName(idx))
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

	body := make([]string, 0, contentHeight)
	buf := m.lines[name]
	if len(buf) == 0 && len(m.stats) == 0 {
		w := lipgloss.NewStyle().Width(innerW).Render(
			lipgloss.NewStyle().Italic(true).Foreground(lipgloss.Color("8")).Render("waiting..."))
		body = append(body, vl+w+vl)
	} else {
		lines := visibleLogLines(buf, contentHeight, innerW)
		for _, line := range lines {
			txt := lipgloss.NewStyle().Width(innerW).Render(colorLogLine(line))
			body = append(body, vl+txt+vl)
		}
	}

	for len(body) < contentHeight {
		body = append(body, vl+strings.Repeat(" ", innerW)+vl)
	}
	if len(body) > contentHeight {
		body = body[:contentHeight]
	}

	bottomBorder := bStyle.Render("╰" + strings.Repeat("─", innerW) + "╯")
	panelLines := append([]string{topBorder}, body...)
	panelLines = append(panelLines, bottomBorder)
	return strings.Join(panelLines, "\n")
}

func (m *Model) viewLogPanels(bodyHeight, panelW int) string {
	active := make([]string, 0, len(m.containers))
	for _, name := range m.containers {
		if !m.disabled[name] {
			active = append(active, name)
		}
	}
	if len(active) == 0 {
		return lipgloss.NewStyle().Width(max(panelW-2, 1)).Render("all hidden — press d to show")
	}
	innerW := max(panelW-2, 1)
	bodyRows := max(bodyHeight/len(active)-2, 0)

	var panels []string
	for i, name := range active {
		idx := m.indexOfContainer(name)
		contentHeight := bodyRows
		color := panelColors[i%len(panelColors)]
		selected := name == m.containers[m.selected]
		bStyle := lipgloss.NewStyle().Foreground(color).Bold(selected)
		vl := bStyle.Render("│")

		title := fmt.Sprintf(" [%s] %s", m.containerRuntime(idx), m.containerName(idx))
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

		body := make([]string, 0, contentHeight)
		buf := m.lines[name]
		if len(buf) == 0 && len(m.stats) == 0 {
			w := lipgloss.NewStyle().Width(innerW).Render(
				lipgloss.NewStyle().Italic(true).Foreground(lipgloss.Color("8")).Render("waiting..."))
			body = append(body, vl+w+vl)
		} else {
			lines := visibleLogLines(buf, contentHeight, innerW)
			for _, line := range lines {
				txt := lipgloss.NewStyle().Width(innerW).Render(colorLogLine(line))
				body = append(body, vl+txt+vl)
			}
		}

		for len(body) < contentHeight {
			body = append(body, vl+strings.Repeat(" ", innerW)+vl)
		}
		if len(body) > contentHeight {
			body = body[:contentHeight]
		}

		bottomBorder := bStyle.Render("╰" + strings.Repeat("─", innerW) + "╯")
		panelLines := append([]string{topBorder}, body...)
		panelLines = append(panelLines, bottomBorder)
		panels = append(panels, strings.Join(panelLines, "\n"))
	}

	return strings.Join(panels, "\n")
}

func (m *Model) viewLogSelector(bodyHeight, panelW int) string {
	innerW := max(panelW-2, 1)
	n := len(m.containers)
	contentHeight := max(bodyHeight-1, 0)
	cap := max(contentHeight-2, 1)
	start, end := visibleRange(n, cap, m.selected)
	lines := make([]string, 0, end-start+2)
	lines = append(lines, " Containers (select which to show)")
	lines = append(lines, strings.Repeat("─", innerW))
	for i := start; i < end; i++ {
		name := m.containers[i]
		marker := "  "
		if i == m.selected {
			marker = "● "
		}
		check := " "
		if !m.disabled[name] {
			check = "x"
		}
		rt := m.containerRuntime(i)
		cn := m.containerName(i)
		sty := lipgloss.NewStyle()
		if i == m.selected {
			sty = sty.Bold(true)
		}
		if m.disabled[name] {
			sty = sty.Foreground(lipgloss.Color("8"))
		}
		lines = append(lines, sty.Render(fmt.Sprintf("%s[%s] %s (%s)", marker, check, cn, rt)))
	}
	if len(lines) > contentHeight {
		lines = lines[:contentHeight]
	}
	for len(lines) < contentHeight {
		lines = append(lines, "")
	}
	if bodyHeight > 0 {
		help := lipgloss.NewStyle().Italic(true).Foreground(lipgloss.Color("8")).
			Render(truncate("d toggle · enter focus · m/esc back", innerW))
		lines = append(lines, help)
	}
	return strings.Join(lines, "\n")
}

func (m *Model) indexOfContainer(key string) int {
	for i, name := range m.containers {
		if name == key {
			return i
		}
	}
	return -1
}
