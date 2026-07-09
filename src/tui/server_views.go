package tui

import (
	"fmt"
	"strings"

	"ctrwatch/src/config"

	"github.com/charmbracelet/lipgloss"
)

func (m *Model) viewServers(bodyHeight, panelW int) string {
	innerW := max(panelW-2, 1)
	if m.setupActive {
		return m.viewConfigSetup(bodyHeight, innerW)
	}
	if len(m.servers) == 0 {
		return lipgloss.NewStyle().Width(innerW).
			Italic(true).Foreground(lipgloss.Color("8")).
			Render("no servers configured — press e to add one")
	}

	lines := make([]string, 0, bodyHeight)
	lines = append(lines, fmt.Sprintf("%-3s %-20s %-28s %-8s %s", "#", "HOST", "SOCKET", "STATUS", "CONTAINERS"))
	lines = append(lines, strings.Repeat("─", innerW))

	for i, s := range m.servers {
		host := s.Host
		if host == "" {
			host = "localhost"
		}
		status := "○"
		serverState := m.serverStates[i].status
		if len(m.serverStates[i].sessions) > 0 && m.serverStates[i].sessions[0] != nil {
			serverState = m.serverStates[i].sessions[0].State()
		}
		switch serverState {
		case "connected":
			status = "●"
		case "connecting":
			status = "⋯"
		case "reconnecting":
			status = "↻"
		case "error":
			status = "✕"
		case "failed":
			status = "!"
		}
		sock := truncate(s.Socket, 28)
		containers := strings.Join(s.Containers, ", ")
		row := fmt.Sprintf("%-3d %-20s %-28s  %-6s %s", i+1, host, sock, status, containers)
		if i == m.selected {
			row = "● " + row[2:]
		}
		lines = append(lines, truncate(row, innerW))
	}
	lines = append(lines, "")
	lines = append(lines, lipgloss.NewStyle().Italic(true).Foreground(lipgloss.Color("8")).
		Render("enter to connect"))
	if m.selected < len(m.serverStates) && m.serverStates[m.selected].err != "" {
		lines = append(lines, "")
		lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("9")).
			Render(truncate("last error: "+m.serverStates[m.selected].err, innerW)))
	}

	for len(lines) < bodyHeight {
		lines = append(lines, "")
	}
	if len(lines) > bodyHeight {
		lines = lines[:bodyHeight]
	}
	return strings.Join(lines, "\n")
}

func (m *Model) viewConfigSetup(bodyHeight, innerW int) string {
	labels := []string{"Host", "Socket", "Containers", "Tags"}
	lines := make([]string, 0, bodyHeight)
	var title string
	if m.setupEditIdx >= 0 {
		s := m.servers[m.setupEditIdx]
		host := s.Host
		if host == "" {
			host = "localhost"
		}
		title = lipgloss.NewStyle().Bold(true).Render("✎ Edit server") + fmt.Sprintf("  %s (%d containers)", host, len(s.Containers))
	} else {
		title = lipgloss.NewStyle().Bold(true).Render("＋ Add server")
	}
	lines = append(lines, title)
	lines = append(lines, strings.Repeat("─", innerW))
	for i, label := range labels {
		marker := "  "
		if i == m.setupField {
			marker = "● "
		}
		field := lipgloss.NewStyle().Width(max(innerW-2, 1)).Render(
			marker + label + ": " + m.setupInputs[i].View())
		lines = append(lines, field)
	}

	if m.setupField == 1 && len(m.detectedSocks) > 0 && config.IsLocalHost(m.setupInputs[0].Value()) {
		lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(
			"   detected: "+strings.Join(m.detectedSocks, ", ")))
	}
	if m.setupField == 0 && len(m.setupHosts) > 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(
			fmt.Sprintf("   SSH aliases: %d available (ctrl+a to cycle)", len(m.setupHosts))))
	}
	if m.setupMessage != "" {
		lines = append(lines, "")
		lines = append(lines, m.setupMessage)
	}
	lines = append(lines, "")
	lines = append(lines, lipgloss.NewStyle().Italic(true).Foreground(lipgloss.Color("8")).
		Render("tab next · ctrl+a aliases · ctrl+p discover · enter save · esc cancel"))
	lipW := lipgloss.NewStyle().Width(innerW)
	for len(lines) < bodyHeight {
		lines = append(lines, lipW.Render(""))
	}
	if len(lines) > bodyHeight {
		lines = lines[:bodyHeight]
	}
	return strings.Join(lines, "\n")
}
