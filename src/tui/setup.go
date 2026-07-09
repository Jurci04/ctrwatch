package tui

import (
	"fmt"
	"strings"

	"ctrwatch/src/config"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *Model) startSetup() {
	m.setupActive = true
	m.setupField = 0
	m.setupValues = [4]string{"localhost", "", "", "dev"}
	m.setupMessage = ""
	m.setupHosts = nil
}

func (m *Model) updateSetup(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.disconnectAll()
		m.cancel()
		return m, tea.Quit
	case "esc":
		m.setupActive = false
		return m, nil
	case "tab", "down":
		m.setupField = (m.setupField + 1) % len(m.setupValues)
	case "shift+tab", "up":
		m.setupField = (m.setupField + len(m.setupValues) - 1) % len(m.setupValues)
	case "backspace":
		value := m.setupValues[m.setupField]
		if value != "" {
			m.setupValues[m.setupField] = value[:len(value)-1]
		}
	case "ctrl+u":
		m.setupValues[m.setupField] = ""
	case "ctrl+a":
		hosts, err := config.SSHConfigHosts()
		if err != nil {
			m.setupMessage = fmt.Sprintf("~/.ssh/config: %v", err)
			return m, nil
		}
		m.setupHosts = hosts
		m.setupMessage = fmt.Sprintf("loaded %d SSH hosts", len(hosts))
	case "enter":
		if m.setupField < len(m.setupValues)-1 {
			m.setupField++
			return m, nil
		}
		return m, m.saveSetup()
	default:
		if msg.Type == tea.KeyRunes {
			m.setupValues[m.setupField] += msg.String()
		}
	}
	return m, nil
}

func (m *Model) saveSetup() tea.Cmd {
	server := config.Server{
		Host:       strings.TrimSpace(m.setupValues[0]),
		Socket:     strings.TrimSpace(m.setupValues[1]),
		Containers: config.SplitList(m.setupValues[2]),
		Tags:       config.SplitList(m.setupValues[3]),
	}
	if server.Host == "" {
		server.Host = "localhost"
	}
	if len(server.Containers) == 0 {
		m.setupMessage = "containers are required"
		m.setupField = 2
		return nil
	}
	if len(server.Tags) == 0 {
		server.Tags = []string{"dev"}
	}
	if err := config.Save(config.ConfigPath(), &config.Config{Servers: []config.Server{server}}); err != nil {
		m.setupMessage = err.Error()
		return nil
	}
	m.setupActive = false
	m.servers = []config.Server{server}
	m.serverStates = []serverState{{containerStart: -1}}
	m.selected = 0
	m.setupMessage = fmt.Sprintf("wrote %s", config.ConfigPath())
	if config.IsLocalHost(server.Host) {
		m.serverStates[0].status = "connecting"
		m.serverStates[0].err = ""
		return m.connectToServer(0)
	}
	return nil
}
