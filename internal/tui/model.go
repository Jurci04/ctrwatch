package tui

import (
	"context"
	"fmt"
	"maps"
	"time"

	"ctrwatch/internal/config"
	"ctrwatch/internal/runtime"
	"ctrwatch/internal/ssh"

	tea "github.com/charmbracelet/bubbletea"
)

const maxLines = 200

// ponytail: cached every 60s to avoid hammering the daemon.
// InspectContainer returns the full container metadata; its
// status and startedAt rarely change between polls.
type containerMeta struct {
	status    string
	startedAt time.Time
	fetchedAt time.Time
}

type Model struct {
	containers []string
	clients    []*runtime.Client
	lines      map[string][]runtime.LogLine
	stats      map[string]*runtime.ContainerStats
	linesCh    chan []runtime.LogLine
	statsCh    chan map[string]*runtime.ContainerStats
	width      int
	height     int
	selected   int
	focused    bool
	ctx        context.Context
	cancel     context.CancelFunc
	logOpts    runtime.LogOptions

	view           viewType
	containersInfo []runtime.Container
	inspect        *runtime.ContainerInspect
	diff           []runtime.Change
	top            *runtime.TopResponse

	pollInterval time.Duration
	disabled     map[string]bool

	servers              []config.Server
	serverStatus         []string
	tunnelCleanups       []func()
	serverContainerStart []int
}

func NewModel(containers []string, clients []*runtime.Client, opts runtime.LogOptions, pollInterval time.Duration, servers ...[]config.Server) *Model {
	ctx, cancel := context.WithCancel(context.Background())
	if pollInterval <= 0 {
		pollInterval = 10 * time.Second
	}
	m := &Model{
		containers:   containers,
		clients:      clients,
		lines:        make(map[string][]runtime.LogLine),
		stats:        make(map[string]*runtime.ContainerStats),
		disabled:     make(map[string]bool),
		linesCh:      make(chan []runtime.LogLine, 64),
		statsCh:      make(chan map[string]*runtime.ContainerStats, 4),
		width:        80,
		height:       24,
		ctx:          ctx,
		cancel:       cancel,
		logOpts:      opts,
		pollInterval: pollInterval,
	}
	if len(servers) > 0 {
		m.servers = servers[0]
		m.serverStatus = make([]string, len(m.servers))
		m.tunnelCleanups = make([]func(), len(m.servers))
		m.serverContainerStart = make([]int, len(m.servers))
		for i := range m.serverContainerStart {
			m.serverContainerStart[i] = -1
		}
	}
	return m
}

func (m *Model) LinesCh() chan<- []runtime.LogLine                  { return m.linesCh }
func (m *Model) StatsCh() chan<- map[string]*runtime.ContainerStats { return m.statsCh }

func (m *Model) Init() tea.Cmd {
	for i, name := range m.containers {
		if i < len(m.clients) && m.clients[i] != nil {
			go m.streamLogs(m.clients[i], name)
		}
	}
	go m.pollStats()
	return tea.Batch(listenLogs(m.linesCh), listenStats(m.statsCh))
}

func (m *Model) streamLogs(client *runtime.Client, name string) {
	lines, errs := client.StreamLogs(m.ctx, name, m.logOpts)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	var batch []runtime.LogLine

	for {
		select {
		case line, ok := <-lines:
			if !ok {
				if len(batch) > 0 {
					m.linesCh <- batch
				}
				goto done
			}
			batch = append(batch, line)
		case <-ticker.C:
			if len(batch) > 0 {
				m.linesCh <- batch
				batch = nil
			}
		case <-m.ctx.Done():
			return
		}
	}

done:
	if err := runtime.ReadStreamError(errs); err != nil {
		m.linesCh <- []runtime.LogLine{
			{Container: name, Text: fmt.Sprintf("error: %v", err)},
		}
	}
}

func (m *Model) pollStats() {
	if len(m.containers) == 0 || len(m.clients) == 0 {
		return
	}
	ticker := time.NewTicker(m.pollInterval)
	defer ticker.Stop()

	const refreshMeta = 60 * time.Second
	metaCache := map[string]*containerMeta{}

	loadMeta := func(i int, c string) {
		info, err := m.clients[i].InspectContainer(m.ctx, c)
		if err != nil {
			return
		}
		meta := &containerMeta{status: info.State.Status, fetchedAt: time.Now()}
		if info.State.Status == "running" && info.State.StartedAt != "" {
			t, err := time.Parse(time.RFC3339Nano, info.State.StartedAt)
			if err == nil {
				meta.startedAt = t
			}
		}
		metaCache[c] = meta
	}

	formatUptime := func(d time.Duration) string {
		h := int(d.Hours())
		if h > 24 {
			return fmt.Sprintf("%dd%dh", h/24, h%24)
		} else if h > 0 {
			return fmt.Sprintf("%dh%dm", h, int(d.Minutes())%60)
		}
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}

	fetch := func() {
		stats := make(map[string]*runtime.ContainerStats, len(m.containers))
		for i, c := range m.containers {
			if i >= len(m.clients) || m.clients[i] == nil {
				continue
			}
			s, err := m.clients[i].StatsContainer(m.ctx, c)
			if err != nil {
				stats[c] = &runtime.ContainerStats{Status: fmt.Sprintf("error: %v", err)}
				continue
			}
			meta := metaCache[c]
			if meta == nil || time.Since(meta.fetchedAt) > refreshMeta {
				loadMeta(i, c)
				meta = metaCache[c]
			}
			if meta != nil {
				s.Status = meta.status
				if !meta.startedAt.IsZero() {
					s.Uptime = formatUptime(time.Since(meta.startedAt))
				}
			}
			stats[c] = s
		}
		if len(stats) > 0 {
			select {
			case m.statsCh <- stats:
			case <-m.ctx.Done():
			}
		}
	}

	fetch()
	for {
		select {
		case <-ticker.C:
			fetch()
		case <-m.ctx.Done():
			return
		}
	}
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

func (m *Model) connectToServer(serverIdx int) tea.Cmd {
	return func() tea.Msg {
		s := m.servers[serverIdx]
		var client *runtime.Client
		var cleanupFn func()

		if config.IsLocalHost(s.Host) {
			client = runtime.NewClient()
			cleanupFn = func() {}
		} else {
			localSock, c, err := ssh.Tunnel(s.Host, s.Socket)
			if err != nil {
				return serverConnectMsg{serverIdx: serverIdx, err: err}
			}
			cleanupFn = c
			client = runtime.NewClientForSocket(localSock)
		}

		return serverConnectMsg{
			serverIdx:  serverIdx,
			client:     client,
			containers: s.Containers,
			cleanup:    cleanupFn,
		}
	}
}

func (m *Model) fetchContainers() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		idx := m.selected
		if idx < 0 || idx >= len(m.clients) || m.clients[idx] == nil {
			return containersListMsg{Err: fmt.Errorf("no client for selected container")}
		}
		containers, err := m.clients[idx].ListContainers(ctx, true)
		return containersListMsg{Containers: containers, Err: err}
	}
}

func (m *Model) fetchInspect() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		idx := m.selected
		if idx < 0 || idx >= len(m.clients) || m.clients[idx] == nil {
			return inspectMsg{Err: fmt.Errorf("no client for selected container")}
		}
		info, err := m.clients[idx].InspectContainer(ctx, m.containers[idx])
		return inspectMsg{Inspect: info, Err: err}
	}
}

func (m *Model) fetchDiff() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		idx := m.selected
		if idx < 0 || idx >= len(m.clients) || m.clients[idx] == nil {
			return diffMsg{Err: fmt.Errorf("no client for selected container")}
		}
		changes, err := m.clients[idx].DiffContainer(ctx, m.containers[idx])
		return diffMsg{Changes: changes, Err: err}
	}
}

func (m *Model) fetchTop() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		idx := m.selected
		if idx < 0 || idx >= len(m.clients) || m.clients[idx] == nil {
			return topMsg{Err: fmt.Errorf("no client for selected container")}
		}
		top, err := m.clients[idx].TopContainer(ctx, m.containers[idx])
		return topMsg{Top: top, Err: err}
	}
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
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
					m.selected = (m.selected + 1) % len(m.containers)
				} else {
					m.selected = m.nextEnabled(1)
				}
			}
			return m, m.onContainerChanged()
		case "up":
			if m.view == viewServers && len(m.servers) > 0 {
				m.selected = (m.selected + len(m.servers) - 1) % len(m.servers)
			} else if len(m.containers) > 0 {
				if m.view == viewLogs {
					m.selected = (m.selected + len(m.containers) - 1) % len(m.containers)
				} else {
					m.selected = m.nextEnabled(-1)
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
				if idx < len(m.servers) && m.serverStatus[idx] != "connected" {
					m.serverStatus[idx] = "connecting"
					if idx < len(m.serverContainerStart) {
						m.serverContainerStart[idx] = -1
					}
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
		case "d":
			if m.view == viewServers {
				if len(m.servers) > 0 && m.selected < len(m.servers) && m.serverStatus[m.selected] == "connected" {
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

	case serverConnectMsg:
		if msg.err != nil {
			m.serverStatus[msg.serverIdx] = "error"
			return m, nil
		}
		m.serverStatus[msg.serverIdx] = "connected"
		m.tunnelCleanups[msg.serverIdx] = msg.cleanup
		m.serverContainerStart[msg.serverIdx] = len(m.containers)
		for _, name := range msg.containers {
			m.containers = append(m.containers, name)
			m.clients = append(m.clients, msg.client)
			m.lines[name] = nil
			go m.streamLogs(msg.client, name)
		}
		if len(m.containers) > 0 && m.view == viewServers && m.selected >= len(m.servers) {
			m.selected = 0
		}
	}

	return m, nil
}

func (m *Model) disconnectServer(srvIdx int) {
	if m.serverStatus[srvIdx] != "connected" {
		return
	}
	m.serverStatus[srvIdx] = ""
	if m.tunnelCleanups[srvIdx] != nil {
		m.tunnelCleanups[srvIdx]()
		m.tunnelCleanups[srvIdx] = nil
	}
	start := m.serverContainerStart[srvIdx]
	m.serverContainerStart[srvIdx] = -1
	if start < 0 {
		return
	}
	count := len(m.servers[srvIdx].Containers)
	end := start + count

	for _, name := range m.servers[srvIdx].Containers {
		delete(m.lines, name)
		delete(m.stats, name)
		delete(m.disabled, name)
	}

	m.containers = append(m.containers[:start], m.containers[end:]...)
	m.clients = append(m.clients[:start], m.clients[end:]...)

	for i := range m.serverContainerStart {
		if m.serverStatus[i] == "connected" && m.serverContainerStart[i] > start {
			m.serverContainerStart[i] -= count
		}
	}

	if m.selected >= len(m.containers) {
		m.selected = max(0, len(m.containers)-1)
	}
}

func (m *Model) disconnectAll() {
	for i := range m.serverStatus {
		m.disconnectServer(i)
	}
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
