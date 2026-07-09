package tui

import (
	"context"
	"fmt"
	"time"

	"ctrwatch/src/runtime"
	"ctrwatch/src/ssh"

	tea "github.com/charmbracelet/bubbletea"
)

// ponytail: cached every 60s to avoid hammering the daemon.
// InspectContainer returns the full container metadata; its
// status and startedAt rarely change between polls.
type containerMeta struct {
	status    string
	startedAt time.Time
	fetchedAt time.Time
}

func (m *Model) serverStateTick() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		return serverStateTickMsg{}
	})
}

func (m *Model) streamLogs(client *runtime.Client, key, name string) {
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
			line.Container = key
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
			{Container: key, Text: fmt.Sprintf("error: %v", err)},
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

	loadMeta := func(i int, key, name string) {
		info, err := m.clients[i].InspectContainer(m.ctx, name)
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
		metaCache[key] = meta
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
		for i, key := range m.containers {
			if i >= len(m.clients) || m.clients[i] == nil {
				continue
			}
			name := m.containerName(i)
			s, err := m.clients[i].StatsContainer(m.ctx, name)
			if err != nil {
				stats[key] = &runtime.ContainerStats{Status: fmt.Sprintf("error: %v", err)}
				continue
			}
			meta := metaCache[key]
			if meta == nil || time.Since(meta.fetchedAt) > refreshMeta {
				loadMeta(i, key, name)
				meta = metaCache[key]
			}
			if meta != nil {
				s.Status = meta.status
				if !meta.startedAt.IsZero() {
					s.Uptime = formatUptime(time.Since(meta.startedAt))
				}
			}
			stats[key] = s
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
		endpoints, err := ssh.ResolveServer(s)

		if err != nil {
			return serverConnectMsg{serverIdx: serverIdx, err: err}
		}

		return serverConnectMsg{
			serverIdx: serverIdx,
			endpoints: endpoints,
		}
	}
}

func (m *Model) fetchContainers() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		seen := map[*runtime.Client]bool{}
		var containers []containerListItem
		for _, client := range m.clients {
			if client == nil || seen[client] {
				continue
			}
			seen[client] = true
			list, err := client.ListContainers(ctx, true)
			if err != nil {
				return containersListMsg{Err: fmt.Errorf("%s: %w", client.SocketPath, err)}
			}
			for _, c := range list {
				containers = append(containers, containerListItem{Client: client, Container: c})
			}
		}
		return containersListMsg{Containers: containers}
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
		info, err := m.clients[idx].InspectContainer(ctx, m.containerName(idx))
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
		changes, err := m.clients[idx].DiffContainer(ctx, m.containerName(idx))
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
		top, err := m.clients[idx].TopContainer(ctx, m.containerName(idx))
		return topMsg{Top: top, Err: err}
	}
}

func (m *Model) disconnectServer(srvIdx int) {
	// TODO(tui): only prune container slices on explicit user disconnect; a
	// reconnecting session should preserve the old rows as stale data.
	state := &m.serverStates[srvIdx]
	if state.status != "connected" && state.status != "reconnecting" && state.status != "connecting" {
		return
	}
	state.status = ""
	state.err = ""
	for _, session := range state.sessions {
		if session != nil {
			_ = session.Disconnect()
		}
	}
	state.sessions = nil
	start := state.containerStart
	state.containerStart = -1
	if start < 0 {
		return
	}
	count := len(m.servers[srvIdx].Containers)
	if state.containerCount > 0 {
		count = state.containerCount
		state.containerCount = 0
	}
	end := start + count
	if start > len(m.containers) {
		return
	}
	if end > len(m.containers) {
		end = len(m.containers)
	}

	for _, name := range m.servers[srvIdx].Containers {
		key := containerKey(runtime.RuntimeKind(m.servers[srvIdx].Socket), name)
		delete(m.lines, key)
		delete(m.stats, key)
		delete(m.disabled, key)
	}

	m.containers = append(m.containers[:start], m.containers[end:]...)
	m.containerNames = append(m.containerNames[:start], m.containerNames[end:]...)
	m.containerRuntimes = append(m.containerRuntimes[:start], m.containerRuntimes[end:]...)
	m.clients = append(m.clients[:start], m.clients[end:]...)

	for i := range m.serverStates {
		if m.serverStates[i].status == "connected" && m.serverStates[i].containerStart > start {
			m.serverStates[i].containerStart -= count
		}
	}

	if m.selected >= len(m.containers) {
		m.selected = max(0, len(m.containers)-1)
	}
}

func (m *Model) disconnectAll() {
	for i := range m.serverStates {
		m.disconnectServer(i)
	}
}
