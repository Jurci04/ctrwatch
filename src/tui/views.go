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
	nav := "←→ views  ↑↓ select  enter focus  q quit"
	switch {
	case m.view == viewLogs && m.focused:
		nav = "↑↓ select  esc unfocus  q quit"
	case m.view == viewLogs && m.logSelectorOpen:
		nav = "↑↓ select  d toggle  enter focus  m/esc back  q quit"
	case m.view == viewLogs:
		nav = "←→ views  ↑↓ select  enter focus  m selector  d hide  q quit"
	case m.view == viewServers:
		nav = "←→ views  ↑↓ sel  enter connect  d disconnect  e edit  q quit"
		if len(m.servers) == 0 {
			nav = "e add server  ←→ views  q quit"
		}
	}
	var footer string
	pos := m.selected + 1
	total := len(m.containers)
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

func (m *Model) viewPS(bodyHeight, panelW int) string {
	innerW := max(panelW-2, 1)
	info := m.containersInfo
	if info == nil {
		return lipgloss.NewStyle().Width(innerW).
			Italic(true).Foreground(lipgloss.Color("8")).
			Render("loading...")
	}
	selName := m.containers[m.selected]
	top, bottom := boxBorder(fmt.Sprintf(" PS: %s — %d/%d", selName, m.selected+1, len(m.containers)), innerW)
	color := panelColors[viewPS]
	bStyle := lipgloss.NewStyle().Foreground(color)
	top, bottom = bStyle.Render(top), bStyle.Render(bottom)
	vl := bStyle.Render("│")
	style := lipgloss.NewStyle().Width(innerW)
	infoByClientName := map[*runtime.Client]map[string]runtime.Container{}
	infoByName := map[string]runtime.Container{}
	for _, item := range info {
		cn := runtime.ContainerName(item.Container.Names)
		if _, ok := infoByName[cn]; !ok {
			infoByName[cn] = item.Container
		}
		if item.Client == nil {
			continue
		}
		if infoByClientName[item.Client] == nil {
			infoByClientName[item.Client] = map[string]runtime.Container{}
		}
		if _, ok := infoByClientName[item.Client][cn]; !ok {
			infoByClientName[item.Client][cn] = item.Container
		}
	}

	body := make([]string, 0, bodyHeight)
	body = append(body, vl+style.Render(truncate(psRow("    ", "RT", "NAME", "ID", "IMAGE", "STATE", "STATUS"), innerW))+vl)

	start, end := visibleRange(len(m.containers), max(bodyHeight-3, 1), m.selected)
	for i := start; i < end; i++ {
		key := m.containers[i]
		cn := m.containerName(i)
		var client *runtime.Client
		if i < len(m.clients) {
			client = m.clients[i]
		}
		c, ok := infoByClientName[client][cn]
		if !ok {
			c, ok = infoByName[cn]
		}
		if !ok {
			continue
		}
		id := runtime.ShortID(c.ID)
		marker := "    "
		if key == selName {
			marker = "●   "
		}
		body = append(body, vl+style.Render(truncate(psRow(marker, m.containerRuntime(i), cn, id, c.Image, c.State, c.Status), innerW))+vl)
	}

	body = padBody(body, innerW, bodyHeight, vl)
	body = append([]string{top}, body...)
	body = append(body, bottom)
	return strings.Join(body, "\n")
}

func psRow(marker, runtimeName, name, id, image, state, status string) string {
	return fmt.Sprintf("%s%-8s %-22s %-12s %-24s %-10s %s",
		marker, runtimeName, truncate(name, 22), id, truncate(image, 24), state, status)
}

func visibleRange(total, capacity, selected int) (int, int) {
	if total <= 0 || capacity <= 0 {
		return 0, 0
	}
	if capacity >= total {
		return 0, total
	}
	start := selected - capacity/2
	if start < 0 {
		start = 0
	}
	if start+capacity > total {
		start = total - capacity
	}
	return start, start + capacity
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
	color := panelColors[viewStats]
	bStyle := lipgloss.NewStyle().Foreground(color)
	vl := bStyle.Render("│")

	if m.focused {
		name := m.containers[m.selected]
		s := m.stats[name]
		title := fmt.Sprintf("%s/%s", m.containerRuntime(m.selected), m.containerName(m.selected))
		if s != nil && s.Uptime != "" {
			title = fmt.Sprintf("%s — %s", name, s.Uptime)
		}
		top, bottom := boxBorder(" Detail: "+title, innerW)
		top, bottom = bStyle.Render(top), bStyle.Render(bottom)
		style := lipgloss.NewStyle().Width(innerW)
		body := make([]string, 0, bodyHeight)

		detail := m.inspect
		if detail != nil {
			body = append(body, vl+style.Render(lipgloss.NewStyle().Bold(true).Render("Overview"))+vl)
			body = append(body, vl+style.Render(fmt.Sprintf("  ID:       %s", runtime.ShortID(detail.ID)))+vl)
			body = append(body, vl+style.Render(fmt.Sprintf("  Image:    %s", detail.Config.Image))+vl)
			body = append(body, vl+style.Render(fmt.Sprintf("  Status:   %s", detail.State.Status))+vl)
			if !detail.Created.IsZero() {
				body = append(body, vl+style.Render(fmt.Sprintf("  Created:  %s", detail.Created.Format(time.RFC1123)))+vl)
			}
			body = append(body, vl+style.Render(fmt.Sprintf("  Restarts: %d", detail.RestartCount))+vl)
			body = append(body, vl+style.Render("")+vl)

			if len(detail.Mounts) > 0 {
				body = append(body, vl+style.Render(lipgloss.NewStyle().Bold(true).Render("Mounts"))+vl)
				for _, mnt := range detail.Mounts {
					rw := "ro"
					if mnt.RW {
						rw = "rw"
					}
					body = append(body, vl+style.Render(fmt.Sprintf("  %s %s → %s (%s)", mnt.Type, shortenPath(mnt.Source), mnt.Destination, rw))+vl)
				}
				body = append(body, vl+style.Render("")+vl)
			}

			if len(detail.Config.Env) > 0 || len(detail.Config.Labels) > 0 {
				body = append(body, vl+style.Render(lipgloss.NewStyle().Bold(true).Render("Configuration"))+vl)
				if len(detail.Config.Env) > 0 {
					body = append(body, vl+style.Render(fmt.Sprintf("  Env:     %d variables", len(detail.Config.Env)))+vl)
				}
				if len(detail.Config.Labels) > 0 {
					body = append(body, vl+style.Render(fmt.Sprintf("  Labels:  %d", len(detail.Config.Labels)))+vl)
				}
				body = append(body, vl+style.Render("")+vl)
			}

			if len(detail.NetworkSettings.Ports) > 0 {
				body = append(body, vl+style.Render(lipgloss.NewStyle().Bold(true).Render("Ports"))+vl)
				for proto, bindings := range detail.NetworkSettings.Ports {
					for _, b := range bindings {
						body = append(body, vl+style.Render(fmt.Sprintf("  %s:%s → %s", b.HostIP, b.HostPort, proto))+vl)
					}
				}
				body = append(body, vl+style.Render("")+vl)
			}
		}

		changes := m.diff
		if len(changes) > 0 {
			body = append(body, vl+style.Render(lipgloss.NewStyle().Bold(true).Render("Filesystem Changes"))+vl)
			for _, ch := range changes {
				k := changeKinds[ch.Kind]
				if k == "" {
					k = "?"
				}
				body = append(body, vl+style.Render(fmt.Sprintf("  %s  %s", k, ch.Path))+vl)
			}
			body = append(body, vl+style.Render("")+vl)
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
			body = append(body, vl+style.Render(lipgloss.NewStyle().Bold(true).Render("Processes"))+vl)
			total := 0
			for _, w := range colWidths {
				total += w
			}
			body = append(body, vl+style.Render(formatRow(proc.Titles))+vl)
			body = append(body, vl+style.Render(strings.Repeat("─", min(innerW, total)))+vl)
			for _, p := range proc.Processes {
				body = append(body, vl+style.Render(formatRow(p))+vl)
			}
			body = append(body, vl+style.Render("")+vl)
		}

		body = padBody(body, innerW, bodyHeight, vl)
		body = append([]string{top}, body...)
		body = append(body, bottom)
		return strings.Join(body, "\n")
	}

	top, bottom := boxBorder(fmt.Sprintf(" Stats: %d container(s)", len(m.containers)), innerW)
	top, bottom = bStyle.Render(top), bStyle.Render(bottom)
	selName := ""
	if m.selected < len(m.containers) {
		selName = m.containers[m.selected]
	}
	style := lipgloss.NewStyle().Width(innerW)
	body := []string{
		vl + style.Render(truncate(fmt.Sprintf("    %-8s %-22s %6s %6s %-16s %-20s %-10s %5s %10s %10s", "RT", "NAME", "CPU", "MEM%", "MEM", "STATUS", "UPTIME", "PIDS", "NET", "BLK"), innerW)) + vl,
	}
	start, end := visibleRange(len(m.containers), max(bodyHeight-3, 1), m.selected)
	for i := start; i < end; i++ {
		name := m.containers[i]
		s := m.stats[name]
		marker := "    "
		if name == selName {
			marker = "●   "
		}
		rawName := m.containerName(i)
		runtimeName := m.containerRuntime(i)
		if s == nil {
			row := fmt.Sprintf("%-8s %-22s %5s %5s %-16s %-20s %-10s %5s %10s %10s", runtimeName, truncate(rawName, 22), "-", "-", "-", "-", "-", "-", "-", "-")
			body = append(body, vl+style.Render(truncate(marker+row, innerW))+vl)
			continue
		}
		memMB := float64(s.MemoryUsage) / 1024 / 1024
		limitMB := float64(s.MemoryLimit) / 1024 / 1024
		memPct := "-"
		if s.MemoryLimit > 0 {
			memPct = fmt.Sprintf("%.1f%%", float64(s.MemoryUsage)/float64(s.MemoryLimit)*100)
		}
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
		row := fmt.Sprintf("%-8s %-22s %5.1f%% %6s %-16s %-20s %-10s %5s %10s %10s", runtimeName, truncate(rawName, 22), s.CPUPercent, memPct, memStr, s.Status, uptime, pids, net, blk)
		body = append(body, vl+style.Render(truncate(marker+row, innerW))+vl)
	}
	body = padBody(body, innerW, bodyHeight, vl)
	body = append([]string{top}, body...)
	body = append(body, bottom)
	return strings.Join(body, "\n")
}
