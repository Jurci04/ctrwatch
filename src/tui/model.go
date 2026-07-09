package tui

import (
	"context"
	"maps"
	"time"

	"ctrwatch/src/config"
	"ctrwatch/src/runtime"
	"ctrwatch/src/ssh"

	tea "github.com/charmbracelet/bubbletea"
)

const maxLines = 200

type Model struct {
	containers        []string
	containerNames    []string
	containerRuntimes []string
	clients           []*runtime.Client
	lines             map[string][]runtime.LogLine
	stats             map[string]*runtime.ContainerStats
	linesCh           chan []runtime.LogLine
	statsCh           chan map[string]*runtime.ContainerStats
	width             int
	height            int
	selected          int
	focused           bool
	ctx               context.Context
	cancel            context.CancelFunc
	logOpts           runtime.LogOptions

	view           viewType
	containersInfo []containerListItem
	inspect        *runtime.ContainerInspect
	diff           []runtime.Change
	top            *runtime.TopResponse

	pollInterval time.Duration
	disabled     map[string]bool

	servers      []config.Server
	serverStates []serverState

	setupActive  bool
	setupField   int
	setupValues  [4]string
	setupMessage string
	setupHosts   []string
}

type serverState struct {
	status         string
	sessions       []*ssh.ServerSession
	containerStart int
	containerCount int
	err            string
}

func NewModel(containers []string, clients []*runtime.Client, opts runtime.LogOptions, pollInterval time.Duration, servers ...[]config.Server) *Model {
	ctx, cancel := context.WithCancel(context.Background())
	if pollInterval <= 0 {
		pollInterval = 10 * time.Second
	}
	names := append([]string(nil), containers...)
	normalizedClients := make([]*runtime.Client, len(containers))
	copy(normalizedClients, clients)
	runtimes := make([]string, len(containers))
	for i := range containers {
		if normalizedClients[i] != nil {
			runtimes[i] = normalizedClients[i].Runtime
		}
		if runtimes[i] == "" {
			runtimes[i] = "runtime"
		}
		containers[i] = containerKey(runtimes[i], names[i])
	}
	m := &Model{
		containers:        containers,
		containerNames:    names,
		containerRuntimes: runtimes,
		clients:           normalizedClients,
		lines:             make(map[string][]runtime.LogLine),
		stats:             make(map[string]*runtime.ContainerStats),
		disabled:          make(map[string]bool),
		linesCh:           make(chan []runtime.LogLine, 64),
		statsCh:           make(chan map[string]*runtime.ContainerStats, 4),
		width:             80,
		height:            24,
		ctx:               ctx,
		cancel:            cancel,
		logOpts:           opts,
		pollInterval:      pollInterval,
	}
	if len(servers) > 0 {
		m.servers = servers[0]
		m.serverStates = make([]serverState, len(m.servers))
		for i := range m.serverStates {
			m.serverStates[i].containerStart = -1
		}
	}
	return m
}

func (m *Model) clampSelected() {
	if m.focused && (m.selected < 0 || m.selected >= len(m.containers)) {
		m.focused = false
	}
	if m.view == viewServers {
		if len(m.servers) == 0 {
			m.selected = 0
		} else if m.selected >= len(m.servers) {
			m.selected = len(m.servers) - 1
		}
		return
	}
	if len(m.containers) == 0 {
		m.selected = 0
		m.focused = false
	} else if m.selected >= len(m.containers) {
		m.selected = len(m.containers) - 1
	}
}

func (m *Model) Init() tea.Cmd {
	var cmds []tea.Cmd
	for i, name := range m.containers {
		if i < len(m.clients) && m.clients[i] != nil {
			go m.streamLogs(m.clients[i], name, m.containerName(i))
		}
	}
	for i, s := range m.servers {
		if config.IsLocalHost(s.Host) {
			m.serverStates[i].status = "connecting"
			cmds = append(cmds, m.connectToServer(i))
		}
	}
	go m.pollStats()
	if len(m.servers) > 0 {
		cmds = append(cmds, m.serverStateTick())
	}
	cmds = append(cmds, listenLogs(m.linesCh), listenStats(m.statsCh))
	return tea.Batch(cmds...)
}

func (m *Model) containerName(i int) string {
	if i >= 0 && i < len(m.containerNames) {
		return m.containerNames[i]
	}
	if i >= 0 && i < len(m.containers) {
		return m.containers[i]
	}
	return ""
}

func (m *Model) containerRuntime(i int) string {
	if i >= 0 && i < len(m.containerRuntimes) && m.containerRuntimes[i] != "" {
		return m.containerRuntimes[i]
	}
	return "runtime"
}

func containerKey(runtimeName, name string) string {
	if runtimeName == "" {
		runtimeName = "runtime"
	}
	return runtimeName + "/" + name
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.setupActive {
			return m.updateSetup(msg)
		}
		switch msg.String() {
		case "q", "ctrl+c":
			m.disconnectAll()
			m.cancel()
			return m, tea.Quit
		case "esc":
			m.focused = false
			return m, tea.ClearScreen
		case "down":
			if m.view == viewServers && len(m.servers) > 0 {
				m.selected = (m.selected + 1) % len(m.servers)
			} else if len(m.containers) > 0 {
				if m.view == viewLogs {
					m.selected = m.nextEnabled(1)
				} else {
					m.selected = (m.selected + 1) % len(m.containers)
				}
			}
			return m, m.onContainerChanged()
		case "up":
			if m.view == viewServers && len(m.servers) > 0 {
				m.selected = (m.selected + len(m.servers) - 1) % len(m.servers)
			} else if len(m.containers) > 0 {
				if m.view == viewLogs {
					m.selected = m.nextEnabled(-1)
				} else {
					m.selected = (m.selected + len(m.containers) - 1) % len(m.containers)
				}
			}
			return m, m.onContainerChanged()
		case "right":
			m.view = (m.view + 1) % 4
			return m, m.onViewChanged()
		case "left":
			m.view = (m.view + 3) % 4
			return m, m.onViewChanged()
		case "enter":
			if m.view == viewServers && len(m.servers) > 0 {
				idx := m.selected
				if idx < len(m.servers) && (m.serverStates[idx].status == "" || m.serverStates[idx].status == "error" || m.serverStates[idx].status == "failed") {
					m.removeServerContainers(idx)
					m.serverStates[idx].status = "connecting"
					m.serverStates[idx].err = ""
					m.serverStates[idx].containerStart = -1
					m.serverStates[idx].containerCount = 0
					return m, m.connectToServer(idx)
				}
			}
			m.focused = !m.focused
			return m, tea.ClearScreen
		case "s":
			if len(m.servers) > 0 && m.view != viewServers {
				m.view = viewServers
				return m, m.onViewChanged()
			}
		case "i":
			if m.view == viewServers && len(m.servers) == 0 {
				m.startSetup()
			}
		case "d":
			if m.view == viewServers {
				if len(m.servers) > 0 && m.selected < len(m.servers) && m.serverStates[m.selected].status == "connected" {
					m.disconnectServer(m.selected)
				}
			} else if len(m.containers) > 0 {
				name := m.containers[m.selected]
				m.disabled[name] = !m.disabled[name]
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case logBatchMsg:
		for _, ll := range msg {
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

	case containersListMsg:
		if msg.Err != nil {
			m.containersInfo = nil
		} else {
			m.containersInfo = msg.Containers
		}

	case inspectMsg:
		if msg.Err != nil {
			m.inspect = nil
		} else {
			m.inspect = msg.Inspect
		}

	case diffMsg:
		if msg.Err != nil {
			m.diff = nil
		} else {
			m.diff = msg.Changes
		}

	case topMsg:
		if msg.Err != nil {
			m.top = nil
		} else {
			m.top = msg.Top
		}

	case serverStateTickMsg:
		// TODO(wiring): feed the live session state and last error into the view
		// so reconnecting/failed rows stay visible without user interaction.
		if m.view == viewServers && len(m.servers) > 0 {
			return m, m.serverStateTick()
		}

	case serverConnectMsg:
		if msg.err != nil {
			// TODO(tui): keep the previous server containers visible and label the
			// row stale instead of clearing everything on a transient SSH failure.
			m.serverStates[msg.serverIdx].status = "error"
			m.serverStates[msg.serverIdx].err = msg.err.Error()
			m.serverStates[msg.serverIdx].sessions = nil
			return m, nil
		}
		state := &m.serverStates[msg.serverIdx]
		state.sessions = nil
		for _, endpoint := range msg.endpoints {
			if endpoint.Session != nil {
				state.sessions = append(state.sessions, endpoint.Session)
			}
		}
		state.status = "connected"
		if len(state.sessions) > 0 {
			state.status = state.sessions[0].State()
		}
		state.err = ""
		state.containerStart = len(m.containers)
		added := 0
		for _, endpoint := range msg.endpoints {
			for _, name := range endpoint.Containers {
				key := containerKey(endpoint.Runtime, name)
				m.containers = append(m.containers, key)
				m.containerNames = append(m.containerNames, name)
				m.containerRuntimes = append(m.containerRuntimes, endpoint.Runtime)
				m.clients = append(m.clients, endpoint.Client)
				m.lines[key] = nil
				go m.streamLogs(endpoint.Client, key, name)
				added++
			}
		}
		state.containerCount = added
		if len(m.containers) > 0 && m.view == viewServers && m.selected >= len(m.servers) {
			m.selected = 0
		}
	}

	return m, nil
}

func (m *Model) nextEnabled(dir int) int {
	n := len(m.containers)
	if n == 0 {
		return 0
	}
	for i := 1; i < n; i++ {
		idx := (m.selected + dir*i + n) % n
		if !m.disabled[m.containers[idx]] {
			return idx
		}
	}
	return m.selected
}

func (m *Model) onContainerChanged() tea.Cmd {
	switch m.view {
	case viewPS:
		return m.fetchContainers()
	case viewStats:
		return tea.Batch(m.fetchInspect(), m.fetchDiff(), m.fetchTop())
	}
	return nil
}

func (m *Model) onViewChanged() tea.Cmd {
	switch m.view {
	case viewPS:
		m.containersInfo = nil
		return m.fetchContainers()
	case viewStats:
		m.inspect = nil
		m.diff = nil
		m.top = nil
		return tea.Batch(m.fetchInspect(), m.fetchDiff(), m.fetchTop())
	}
	return nil
}
