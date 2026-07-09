package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"ctrwatch/src/config"
	"ctrwatch/src/runtime"
	"ctrwatch/src/ssh"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

func (m *Model) initInputs() {
	placeholders := []string{"localhost or SSH alias", "blank = default runtime socket", "comma-separated", "comma-separated"}
	values := []string{"localhost", "", "", "dev"}
	if m.setupEditIdx >= 0 {
		s := m.servers[m.setupEditIdx]
		values = []string{s.Host, s.Socket, strings.Join(s.Containers, ", "), strings.Join(s.Tags, ", ")}
	}
	for i := range m.setupInputs {
		ti := textinput.New()
		ti.Placeholder = placeholders[i]
		ti.SetValue(values[i])
		m.setupInputs[i] = ti
	}
	m.setupInputs[0].Focus()
}

func (m *Model) startSetup() {
	m.setupActive = true
	m.setupField = 0
	m.setupMessage = ""
	m.setupHosts = nil
	m.setupEditIdx = -1
	m.setupHostIdx = 0
	m.detectedSocks = runtime.ExistingDefaultSockets()
	m.initInputs()
}

func (m *Model) startEditSetup(idx int) {
	m.setupActive = true
	m.setupField = 0
	m.setupMessage = ""
	m.setupHosts = nil
	m.setupEditIdx = idx
	m.setupHostIdx = 0
	m.detectedSocks = runtime.ExistingDefaultSockets()
	m.initInputs()
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
		m.setupInputs[m.setupField].Blur()
		m.setupField = (m.setupField + 1) % len(m.setupInputs)
		m.setupInputs[m.setupField].Focus()
		return m, nil
	case "shift+tab", "up":
		m.setupInputs[m.setupField].Blur()
		m.setupField = (m.setupField + len(m.setupInputs) - 1) % len(m.setupInputs)
		m.setupInputs[m.setupField].Focus()
		return m, nil
	case "ctrl+a":
		if m.setupField == 0 {
			if m.setupHosts == nil {
				hosts, err := config.SSHConfigHosts()
				if err != nil {
					m.setupMessage = fmt.Sprintf("~/.ssh/config: %v", err)
					return m, nil
				}
				m.setupHosts = hosts
				m.setupHostIdx = 0
			}
			if len(m.setupHosts) > 0 {
				m.setupInputs[0].SetValue(m.setupHosts[m.setupHostIdx])
				m.setupHostIdx = (m.setupHostIdx + 1) % len(m.setupHosts)
				m.setupMessage = fmt.Sprintf("SSH alias %d/%d", m.setupHostIdx, len(m.setupHosts))
			}
		}
		return m, nil
	case "ctrl+p":
		if m.setupField == 2 {
			m.setupMessage = "discovering containers..."
			return m, m.discoverContainers(strings.TrimSpace(m.setupInputs[1].Value()))
		}
		return m, nil
	case "enter":
		if m.setupField < len(m.setupInputs)-1 {
			m.setupInputs[m.setupField].Blur()
			m.setupField++
			m.setupInputs[m.setupField].Focus()
			return m, nil
		}
		return m, m.saveSetup()
	default:
		var cmd tea.Cmd
		m.setupInputs[m.setupField], cmd = m.setupInputs[m.setupField].Update(msg)
		return m, cmd
	}
}

func (m *Model) saveSetup() tea.Cmd {
	server := config.Server{
		Host:       strings.TrimSpace(m.setupInputs[0].Value()),
		Socket:     strings.TrimSpace(m.setupInputs[1].Value()),
		Containers: config.SplitList(m.setupInputs[2].Value()),
		Tags:       config.SplitList(m.setupInputs[3].Value()),
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

	var cfg *config.Config
	loadedCfg, err := config.Load(config.ConfigPath())
	if err != nil {
		cfg = &config.Config{}
	} else {
		cfg = loadedCfg
	}

	if m.setupEditIdx >= 0 && m.setupEditIdx < len(cfg.Servers) {
		cfg.Servers[m.setupEditIdx] = server
	} else {
		cfg.Servers = append(cfg.Servers, server)
	}

	if err := config.Save(config.ConfigPath(), cfg); err != nil {
		m.setupMessage = err.Error()
		return nil
	}

	m.setupActive = false
	m.servers = cfg.Servers
	newIdx := len(m.servers) - 1

	if m.setupEditIdx >= 0 {
		newIdx = m.setupEditIdx
		m.disconnectServer(newIdx)
		m.removeServerContainers(newIdx)
		m.serverStates[newIdx] = serverState{containerStart: -1}
	} else {
		m.serverStates = append(m.serverStates, serverState{containerStart: -1})
		m.selected = newIdx
	}

	m.setupMessage = fmt.Sprintf("wrote %s", config.ConfigPath())
	if config.IsLocalHost(server.Host) {
		m.serverStates[newIdx].status = "connecting"
		m.serverStates[newIdx].err = ""
		return m.connectToServer(newIdx)
	}
	return nil
}

func (m *Model) discoverContainers(socket string) tea.Cmd {
	host := strings.TrimSpace(m.setupInputs[0].Value())
	return func() tea.Msg {
		socketPath := socket
		if socketPath == "" {
			socks := runtime.ExistingDefaultSockets()
			if len(socks) > 0 {
				socketPath = socks[0]
			}
		}
		if socketPath == "" {
			return discoveredContainersMsg{err: fmt.Errorf("no socket found")}
		}

		var client *runtime.Client
		var cleanup func()

		if !config.IsLocalHost(host) {
			tunnel := ssh.NewServerTunnel(host, socketPath)
			if err := tunnel.Start(); err != nil {
				return discoveredContainersMsg{err: fmt.Errorf("tunnel to %s: %w", host, err)}
			}
			client = runtime.NewClientForSocket(tunnel.Socket())
			cleanup = func() { _ = tunnel.Stop() }
		} else {
			client = runtime.NewClientForSocket(socketPath)
		}
		if client == nil {
			return discoveredContainersMsg{err: fmt.Errorf("failed to connect to %s", socketPath)}
		}
		if cleanup != nil {
			defer cleanup()
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		containers, err := client.ListContainers(ctx, true)
		if err != nil {
			return discoveredContainersMsg{err: fmt.Errorf("%s: %w", socketPath, err)}
		}
		var names []string
		for _, c := range containers {
			names = append(names, runtime.ContainerName(c.Names))
		}
		return discoveredContainersMsg{names: names, socket: socketPath}
	}
}
