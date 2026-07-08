package tui

import (
	"fmt"
	"strings"
	"time"

	"ctrwatch/src/runtime"

	"github.com/charmbracelet/lipgloss"
)

var panelColors = []lipgloss.Color{
	lipgloss.Color("12"),
	lipgloss.Color("10"),
	lipgloss.Color("11"),
	lipgloss.Color("13"),
}

var viewNames = []string{" logs ", "  ps  ", " stats", "servers"}

var changeKinds = map[int]string{0: "M", 1: "A", 2: "D"}

func viewHeader(v viewType, w int) string {
	tabs := make([]string, len(viewNames))
	for i, n := range viewNames {
		if i == int(v) {
			tabs[i] = lipgloss.NewStyle().Bold(true).Reverse(true).Render(n)
		} else {
			tabs[i] = n
		}
	}
	line := strings.Join(tabs, " │ ")
	pad := max(0, w-lipgloss.Width(line))
	return line + strings.Repeat(" ", pad)
}

func (m *Model) View() string {
	m.clampSelected()
	if len(m.containers) == 0 {
		return m.emptyView()
	}

	mode := "[all]"
	if m.focused {
		mode = m.containers[m.selected]
	}
	head := fmt.Sprintf(" ctrwatch  %s  ", mode)
	nav := "←→ views  ↑↓ sel  enter focus  s servers  d hide  esc unfocus  q quit"
	if m.view == viewServers {
		nav = "←→ views  ↑↓ sel  enter connect  d disconnect  q quit"
	}
	var footer string
	pos := m.selected + 1
	total := len(m.containers)
	if m.view != viewLogs && m.view != viewServers {
		sc := m.sourceContainers()
		pos = 1
		for i, name := range sc {
			if name == m.containers[m.selected] {
				pos = i + 1
				break
			}
		}
		total = len(sc)
	}
	footer = fmt.Sprintf(" %d/%d  %s  ", pos, total, nav)
	panelW := max(m.width-2, 1)
	head = lipgloss.NewStyle().Bold(true).Width(panelW).Render(truncate(head, panelW))
	footer = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Width(panelW).Render(truncate(footer, panelW))
	vh := viewHeader(m.view, panelW)

	bodyHeight := max(m.height-4, 1)
	var body string
	switch m.view {
	case viewLogs:
		body = m.viewLogs(bodyHeight, panelW)
	case viewPS:
		body = m.viewPS(bodyHeight, panelW)
	case viewStats:
		body = m.viewStats(bodyHeight, panelW)
	case viewServers:
		body = m.viewServers(bodyHeight, panelW)
	}

	out := head + "\n" + vh + "\n" + body + "\n" + footer
	return fitHeight(out, panelW, m.height)
}

func (m *Model) emptyView() string {
	w := max(m.width-2, 1)
	if m.view == viewServers {
		return m.viewServers(max(m.height-4, 1), w)
	}
	msg := "no containers — connect to a server via the servers view (←→ to navigate)"
	return lipgloss.NewStyle().Width(w).Render(msg)
}

func (m *Model) viewLogs(bodyHeight, panelW int) string {
	containers := m.containers
	if m.focused {
		containers = []string{m.containers[m.selected]}
	}
	activeCount := len(containers)
	hiddenCount := 0
	if !m.focused && len(m.disabled) > 0 {
		var active, hidden []string
		for _, name := range containers {
			if m.disabled[name] {
				hidden = append(hidden, name)
			} else {
				active = append(active, name)
			}
		}
		hiddenCount = len(hidden)
		activeCount = len(active)
		containers = append(active, hidden...)
	}
	bodyRows := 0
	if activeCount > 0 {
		bodyRows = max((bodyHeight-hiddenCount)/activeCount-2, 0)
	}
	if activeCount == 0 {
		return lipgloss.NewStyle().Width(max(panelW-2, 1)).Render("all hidden — press d to show")
	}
	innerW := max(panelW-2, 1)

	var panels []string
	for i, name := range containers {
		idx := m.indexOfContainer(name)
		contentHeight := bodyRows
		color := panelColors[i%len(panelColors)]
		selected := name == m.containers[m.selected]
		hidden := m.disabled[name]
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
		if hidden {
			title = "[hidden] " + title
		}
		topBorder := bStyle.Render("╭" + "─" + title + strings.Repeat("─", dashes) + "╮")

		if hidden {
			panels = append(panels, topBorder)
			continue
		}

		var body []string
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

func (m *Model) sourceContainers() []string {
	if len(m.clients) <= m.selected || m.clients[m.selected] == nil {
		return m.containers
	}
	client := m.clients[m.selected]
	var out []string
	for i, name := range m.containers {
		if i < len(m.clients) && m.clients[i] == client {
			out = append(out, name)
		}
	}
	return out
}

func (m *Model) sourceContainerNames() []string {
	if len(m.clients) <= m.selected || m.clients[m.selected] == nil {
		return m.containerNames
	}
	client := m.clients[m.selected]
	var out []string
	for i, name := range m.containerNames {
		if i < len(m.clients) && m.clients[i] == client {
			out = append(out, name)
		}
	}
	return out
}

func (m *Model) indexOfContainer(key string) int {
	for i, name := range m.containers {
		if name == key {
			return i
		}
	}
	return -1
}

func (m *Model) keyForContainer(client *runtime.Client, rawName string) (string, int) {
	for i, name := range m.containerNames {
		if name == rawName && i < len(m.clients) && m.clients[i] == client {
			return m.containers[i], i
		}
	}
	return rawName, -1
}

func (m *Model) viewPS(bodyHeight, panelW int) string {
	innerW := max(panelW-2, 1)
	info := m.containersInfo
	if info == nil {
		return lipgloss.NewStyle().Width(innerW).
			Italic(true).Foreground(lipgloss.Color("8")).
			Render("loading...")
	}
	selName := m.containers[m.selected]
	sc := m.sourceContainers()
	top, bottom := boxBorder(fmt.Sprintf(" PS: %s — %d/%d", selName, len(sc), len(info)), innerW)
	style := lipgloss.NewStyle().Width(innerW)
	scNames := m.sourceContainerNames()
	scSet := make(map[string]bool, len(scNames))
	for _, n := range scNames {
		scSet[n] = true
	}
	selectedClient := m.clients[m.selected]

	var body []string
	body = append(body, "│"+style.Render(truncate(fmt.Sprintf("    %-8s %-24s %-12s %-24s %-10s %s", "RT", "NAME", "ID", "IMAGE", "STATE", "STATUS"), innerW))+"│")

	// connected section
	connectedHeader := false
	for _, c := range info {
		cn := runtime.ContainerName(c.Names)
		if !scSet[cn] {
			continue
		}
		if !connectedHeader {
			body = append(body, "│"+style.Render(truncate(lipgloss.NewStyle().Bold(true).Render("  Connected"), innerW))+"│")
			connectedHeader = true
		}
		key, idx := m.keyForContainer(selectedClient, cn)
		id := runtime.ShortID(c.ID)
		row := fmt.Sprintf("%-8s %-24s %-12s %-24s %-10s %s", m.containerRuntime(idx), truncate(cn, 24), id, truncate(c.Image, 24), c.State, c.Status)
		marker := "    "
		if key == selName {
			marker = "●   "
		}
		body = append(body, "│"+style.Render(truncate(marker+row, innerW))+"│")
	}

	// all section
	allHeader := false
	for _, c := range info {
		cn := runtime.ContainerName(c.Names)
		if scSet[cn] {
			continue
		}
		if !allHeader {
			body = append(body, "│"+style.Render("")+"│")
			body = append(body, "│"+style.Render(truncate(lipgloss.NewStyle().Bold(true).Render("  All on socket"), innerW))+"│")
			allHeader = true
		}
		id := runtime.ShortID(c.ID)
		row := fmt.Sprintf("%-8s %-24s %-12s %-24s %-10s %s", m.containerRuntime(m.selected), truncate(cn, 24), id, truncate(c.Image, 24), c.State, c.Status)
		body = append(body, "│"+style.Render(truncate("    "+row, innerW))+"│")
	}

	body = padBody(body, innerW, bodyHeight)
	body = append([]string{top}, body...)
	body = append(body, bottom)
	return strings.Join(body, "\n")
}

func formatBytes(b uint64) string {
	if b < 1024 {
		return fmt.Sprintf("%dB", b)
	}
	if b < 1024*1024 {
		return fmt.Sprintf("%.1fK", float64(b)/1024)
	}
	if b < 1024*1024*1024 {
		return fmt.Sprintf("%.1fM", float64(b)/1024/1024)
	}
	return fmt.Sprintf("%.1fG", float64(b)/1024/1024/1024)
}

func shortenPath(p string) string {
	if len(p) > 40 {
		return "..." + p[len(p)-37:]
	}
	return p
}

func (m *Model) viewStats(bodyHeight, panelW int) string {
	innerW := max(panelW-2, 1)

	if m.focused {
		name := m.containers[m.selected]
		s := m.stats[name]
		title := fmt.Sprintf("%s/%s", m.containerRuntime(m.selected), m.containerName(m.selected))
		if s != nil && s.Uptime != "" {
			title = fmt.Sprintf("%s — %s", name, s.Uptime)
		}
		top, bottom := boxBorder(" Detail: "+title, innerW)
		style := lipgloss.NewStyle().Width(innerW)
		var body []string

		detail := m.inspect
		if detail != nil {
			body = append(body, "│"+style.Render(lipgloss.NewStyle().Bold(true).Render("Overview"))+"│")
			body = append(body, "│"+style.Render(fmt.Sprintf("  ID:       %s", runtime.ShortID(detail.ID)))+"│")
			body = append(body, "│"+style.Render(fmt.Sprintf("  Image:    %s", detail.Config.Image))+"│")
			body = append(body, "│"+style.Render(fmt.Sprintf("  Status:   %s", detail.State.Status))+"│")
			if !detail.Created.IsZero() {
				body = append(body, "│"+style.Render(fmt.Sprintf("  Created:  %s", detail.Created.Format(time.RFC1123)))+"│")
			}
			body = append(body, "│"+style.Render(fmt.Sprintf("  Restarts: %d", detail.RestartCount))+"│")
			body = append(body, "│"+style.Render("")+"│")

			if len(detail.Mounts) > 0 {
				body = append(body, "│"+style.Render(lipgloss.NewStyle().Bold(true).Render("Mounts"))+"│")
				for _, mnt := range detail.Mounts {
					rw := "ro"
					if mnt.RW {
						rw = "rw"
					}
					body = append(body, "│"+style.Render(fmt.Sprintf("  %s %s → %s (%s)", mnt.Type, shortenPath(mnt.Source), mnt.Destination, rw))+"│")
				}
				body = append(body, "│"+style.Render("")+"│")
			}

			if len(detail.Config.Env) > 0 || len(detail.Config.Labels) > 0 {
				body = append(body, "│"+style.Render(lipgloss.NewStyle().Bold(true).Render("Configuration"))+"│")
				if len(detail.Config.Env) > 0 {
					body = append(body, "│"+style.Render(fmt.Sprintf("  Env:     %d variables", len(detail.Config.Env)))+"│")
				}
				if len(detail.Config.Labels) > 0 {
					body = append(body, "│"+style.Render(fmt.Sprintf("  Labels:  %d", len(detail.Config.Labels)))+"│")
				}
				body = append(body, "│"+style.Render("")+"│")
			}

			if len(detail.NetworkSettings.Ports) > 0 {
				body = append(body, "│"+style.Render(lipgloss.NewStyle().Bold(true).Render("Ports"))+"│")
				for proto, bindings := range detail.NetworkSettings.Ports {
					for _, b := range bindings {
						body = append(body, "│"+style.Render(fmt.Sprintf("  %s:%s → %s", b.HostIP, b.HostPort, proto))+"│")
					}
				}
				body = append(body, "│"+style.Render("")+"│")
			}
		}

		changes := m.diff
		if len(changes) > 0 {
			body = append(body, "│"+style.Render(lipgloss.NewStyle().Bold(true).Render("Filesystem Changes"))+"│")
			for _, ch := range changes {
				k := changeKinds[ch.Kind]
				if k == "" {
					k = "?"
				}
				body = append(body, "│"+style.Render(fmt.Sprintf("  %s  %s", k, ch.Path))+"│")
			}
			body = append(body, "│"+style.Render("")+"│")
		}

		proc := m.top
		if proc != nil && len(proc.Processes) > 0 {
			colWidths := make([]int, len(proc.Titles))
			for i, t := range proc.Titles {
				colWidths[i] = len(t) + 2
			}
			formatRow := func(cells []string) string {
				var parts []string
				for i, cell := range cells {
					w := colWidths[i]
					if i == len(cells)-1 {
						parts = append(parts, cell)
					} else {
						padded := cell
						if len(padded) < w {
							padded += strings.Repeat(" ", w-len(padded))
						}
						parts = append(parts, padded)
					}
				}
				return strings.Join(parts, "")
			}
			body = append(body, "│"+style.Render(lipgloss.NewStyle().Bold(true).Render("Processes"))+"│")
			total := 0
			for _, w := range colWidths {
				total += w
			}
			body = append(body, "│"+style.Render(formatRow(proc.Titles))+"│")
			body = append(body, "│"+style.Render(strings.Repeat("─", min(innerW, total)))+"│")
			for _, p := range proc.Processes {
				body = append(body, "│"+style.Render(formatRow(p))+"│")
			}
			body = append(body, "│"+style.Render("")+"│")
		}

		body = padBody(body, innerW, bodyHeight)
		body = append([]string{top}, body...)
		body = append(body, bottom)
		return strings.Join(body, "\n")
	}

	top, bottom := boxBorder(fmt.Sprintf(" Stats: %d container(s)", len(m.containers)), innerW)

	selName := ""
	if m.selected < len(m.containers) {
		selName = m.containers[m.selected]
	}
	style := lipgloss.NewStyle().Width(innerW)
	body := []string{
		"│" + style.Render(truncate(fmt.Sprintf("    %-8s %-22s %6s %6s %-16s %-20s %-10s %5s %10s %10s", "RT", "NAME", "CPU", "MEM%", "MEM", "STATUS", "UPTIME", "PIDS", "NET", "BLK"), innerW)) + "│",
	}
	for i, name := range m.containers {
		if m.focused && name != selName {
			continue
		}
		s := m.stats[name]
		marker := "    "
		if name == selName {
			marker = "●   "
		}
		rawName := m.containerName(i)
		runtimeName := m.containerRuntime(i)
		if s == nil {
			row := fmt.Sprintf("%-8s %-22s %5s %5s %-16s %-20s %-10s %5s %10s %10s", runtimeName, truncate(rawName, 22), "-", "-", "-", "-", "-", "-", "-", "-")
			body = append(body, "│"+style.Render(truncate(marker+row, innerW))+"│")
			continue
		}
		memMB := float64(s.MemoryUsage) / 1024 / 1024
		limitMB := float64(s.MemoryLimit) / 1024 / 1024
		memPct := float64(s.MemoryUsage) / float64(s.MemoryLimit) * 100
		memStr := fmt.Sprintf("%.0f / %.0f MB", memMB, limitMB)
		uptime := s.Uptime
		if uptime == "" {
			uptime = "-"
		}
		pids := fmt.Sprintf("%d", s.PIDsCurrent)
		if s.PIDsCurrent == 0 {
			pids = "-"
		}
		net := formatBytes(s.NetRxBytes + s.NetTxBytes)
		if net == "0B" {
			net = "-"
		}
		blk := formatBytes(s.BlkReadBytes + s.BlkWriteBytes)
		if blk == "0B" {
			blk = "-"
		}
		row := fmt.Sprintf("%-8s %-22s %5.1f%% %5.1f%% %-16s %-20s %-10s %5s %10s %10s", runtimeName, truncate(rawName, 22), s.CPUPercent, memPct, memStr, s.Status, uptime, pids, net, blk)
		body = append(body, "│"+style.Render(truncate(marker+row, innerW))+"│")
	}
	body = padBody(body, innerW, bodyHeight)
	body = append([]string{top}, body...)
	body = append(body, bottom)
	return strings.Join(body, "\n")
}

func (m *Model) viewServers(bodyHeight, panelW int) string {
	innerW := max(panelW-2, 1)
	if len(m.servers) == 0 {
		return lipgloss.NewStyle().Width(innerW).
			Italic(true).Foreground(lipgloss.Color("8")).
			Render("no servers configured — create a ctrwatch.yaml")
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
		serverState := m.serverStatus[i]
		if i < len(m.serverSessions) && m.serverSessions[i] != nil {
			serverState = m.serverSessions[i].State()
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

	for len(lines) < bodyHeight {
		lines = append(lines, "")
	}
	if len(lines) > bodyHeight {
		lines = lines[:bodyHeight]
	}
	return strings.Join(lines, "\n")
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

func colorLogLine(line visibleLogLine) string {
	return keywordReplacer.Replace(line.text)
}

var keywordReplacer = strings.NewReplacer(
	"CRITICAL", ansiColor("9")+"CRITICAL"+ansiReset,
	"critical", ansiColor("9")+"critical"+ansiReset,
	"ERROR", ansiColor("9")+"ERROR"+ansiReset,
	"error", ansiColor("9")+"error"+ansiReset,
	"WARNING", ansiColor("11")+"WARNING"+ansiReset,
	"warning", ansiColor("11")+"warning"+ansiReset,
	"WARN", ansiColor("11")+"WARN"+ansiReset,
	"warn", ansiColor("11")+"warn"+ansiReset,
	"INFO", ansiColor("10")+"INFO"+ansiReset,
	"info", ansiColor("10")+"info"+ansiReset,
	"DEBUG", ansiColor("12")+"DEBUG"+ansiReset,
	"debug", ansiColor("12")+"debug"+ansiReset,
)

func ansiColor(n string) string { return "\x1b[38;5;" + n + "m" }

const ansiReset = "\x1b[0m"

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

func boxBorder(title string, innerW int) (top, bottom string) {
	title = truncate(title, max(innerW-2, 1))
	dashes := max(0, innerW-1-lipgloss.Width(title))
	top = "╭" + "─" + title + strings.Repeat("─", dashes) + "╮"
	bottom = "╰" + strings.Repeat("─", innerW) + "╯"
	return
}

func padBody(body []string, innerW, bodyHeight int) []string {
	contentHeight := max(bodyHeight-2, 1)
	for len(body) < contentHeight {
		body = append(body, "│"+strings.Repeat(" ", innerW)+"│")
	}
	if len(body) > contentHeight {
		body = body[:contentHeight]
	}
	return body
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
