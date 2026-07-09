package tui

import (
	"fmt"
	"strings"

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
			Render("no servers configured — press i to create ctrwatch.yaml")
	}

	var lines []string
	header := fmt.Sprintf("%-3s %-20s %-28s %-8s %s", "#", "HOST", "SOCKET", "STATUS", "CONTAINERS")
	lines = append(lines, header)
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
	hints := []string{"localhost or SSH alias", "blank = default runtime socket", "comma-separated", "comma-separated"}
	var lines []string
	lines = append(lines, "Create ctrwatch.yaml")
	lines = append(lines, strings.Repeat("─", innerW))
	for i, label := range labels {
		marker := "  "
		if i == m.setupField {
			marker = "● "
		}
		value := m.setupValues[i]
		if value == "" {
			value = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(hints[i])
		}
		lines = append(lines, truncate(fmt.Sprintf("%s%-11s %s", marker, label+":", value), innerW))
	}
	if len(m.setupHosts) > 0 {
		lines = append(lines, "")
		lines = append(lines, truncate("SSH hosts: "+strings.Join(m.setupHosts, ", "), innerW))
	}
	if m.setupMessage != "" {
		lines = append(lines, "")
		lines = append(lines, truncate(m.setupMessage, innerW))
	}
	lines = append(lines, "")
	lines = append(lines, lipgloss.NewStyle().Italic(true).Foreground(lipgloss.Color("8")).
		Render("tab next, ctrl+a aliases, enter save, esc cancel"))
	for len(lines) < bodyHeight {
		lines = append(lines, "")
	}
	if len(lines) > bodyHeight {
		lines = lines[:bodyHeight]
	}
	return lipgloss.NewStyle().Width(innerW).Render(strings.Join(lines, "\n"))
}
