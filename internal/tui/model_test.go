package tui

import (
	"strings"
	"testing"

	"ctrwatch/internal/config"
	"ctrwatch/internal/runtime"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func testModel(containers []string) *Model {
	clients := make([]*runtime.Client, len(containers))
	return NewModel(containers, clients, runtime.LogOptions{Tail: "100"}, 0)
}

func TestViewLayoutInvariants(t *testing.T) {
	m := testModel([]string{"api", "worker", "db"})
	m.width = 80
	m.height = 47
	longLine := "\x1b[31m" + strings.Repeat("x", 200) + "\x1b[0m\r"
	for _, name := range m.containers {
		m.lines[name] = []runtime.LogLine{{Container: name, Text: longLine}}
	}
	// Seed stats so log view doesn't show "waiting..."
	for _, name := range m.containers {
		m.stats[name] = &runtime.ContainerStats{Status: "running", CPUPercent: 1.0, MemoryUsage: 1 << 20}
	}

	view := m.View()
	lines := strings.Split(view, "\n")
	if len(lines) != 47 {
		t.Fatalf("view height = %d, want 47", len(lines))
	}
	for _, line := range lines {
		if got := lipgloss.Width(line); got > 78 {
			t.Fatalf("line width = %d, want <= 78: %q", got, line)
		}
	}
}

func TestTUILogHelpers(t *testing.T) {
	if got := cleanLine("\x1b[31mred\x1b[0m\r\tok"); got != "red    ok" {
		t.Fatalf("cleanLine = %q, want %q", got, "red    ok")
	}

	buf := []runtime.LogLine{{Text: "one"}, {Text: "two"}, {Text: ""}, {Text: "\r"}}
	lines := visibleLogLines(buf, 2, 80)
	if len(lines) != 2 || lines[0].text != "one" || lines[1].text != "two" {
		t.Fatalf("visibleLogLines = %#v, want one/two", lines)
	}

	if got := colorLogLine(visibleLogLine{text: "2026 info ok"}); !strings.Contains(got, "\x1b[") {
		t.Fatal("info keyword should be colored")
	}
	if got := colorLogLine(visibleLogLine{text: "plain"}); got != "plain" {
		t.Fatal("plain line should be unchanged")
	}
}

func TestFocusClearsScreenButTabDoesNot(t *testing.T) {
	m := testModel([]string{"api", "worker"})

	model, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = model.(*Model)
	if cmd == nil || cmd() == nil {
		t.Fatal("enter did not request clear screen")
	}

	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if cmd != nil {
		t.Fatal("tab requested clear screen")
	}
}

func TestViewSwitching(t *testing.T) {
	m := testModel([]string{"api", "worker"})
	if m.view != viewLogs {
		t.Fatalf("default view = %d, want viewLogs", m.view)
	}

	views := []viewType{viewPS, viewStats, viewServers, viewLogs}
	for _, want := range views {
		m.Update(tea.KeyMsg{Type: tea.KeyRight})
		if m.view != want {
			t.Fatalf("after right = %d, want %d", m.view, want)
		}
	}

	// Left wraps around: from viewLogs(0), left→viewServers(3)→viewStats(2)→viewPS(1)→viewLogs(0)
	leftViews := []viewType{viewServers, viewStats, viewPS, viewLogs}
	for _, want := range leftViews {
		m.Update(tea.KeyMsg{Type: tea.KeyLeft})
		if m.view != want {
			t.Fatalf("after left = %d, want %d", m.view, want)
		}
	}
}

func TestPSViewRendersTable(t *testing.T) {
	m := testModel([]string{"nginx"})
	m.view = viewPS
	m.width = 80
	m.height = 24
	m.containersInfo = []runtime.Container{
		{ID: "abc123def456", Names: []string{"/nginx"}, Image: "nginx:1.25", State: "running", Status: "Up 2h"},
	}

	view := m.View()
	if !strings.Contains(view, "nginx") {
		t.Fatalf("ps view missing container name:\n%s", view)
	}
	if !strings.Contains(view, "abc123def456") {
		t.Fatalf("ps view missing container id:\n%s", view)
	}
}

func TestInspectViewRendersMetadata(t *testing.T) {
	m := testModel([]string{"nginx"})
	m.view = viewStats
	m.focused = true
	m.width = 80
	m.height = 24
	m.inspect = &runtime.ContainerInspect{
		ID:    "abc123def456",
		Name:  "/nginx",
		State: runtime.ContainerState{Status: "running"},
		Config: runtime.ContainerConfig{
			Image:  "nginx:1.25",
			Env:    []string{"A=B"},
			Labels: map[string]string{"app": "web"},
		},
	}

	view := m.View()
	if !strings.Contains(view, "nginx") {
		t.Fatalf("inspect view missing name:\n%s", view)
	}
	if !strings.Contains(view, "running") {
		t.Fatalf("inspect view missing status:\n%s", view)
	}
	if !strings.Contains(view, "1 variables") {
		t.Fatalf("inspect view missing env count:\n%s", view)
	}
}

func TestStatsViewRendersStats(t *testing.T) {
	m := testModel([]string{"nginx", "redis"})
	m.view = viewStats
	m.width = 80
	m.height = 24
	m.stats[m.containers[0]] = &runtime.ContainerStats{CPUPercent: 12.5, MemoryUsage: 45 << 20, MemoryLimit: 512 << 20, Status: "running"}
	m.stats[m.containers[1]] = &runtime.ContainerStats{CPUPercent: 2.1, MemoryUsage: 32 << 20, MemoryLimit: 512 << 20, Status: "running"}

	view := m.View()
	if !strings.Contains(view, "nginx") || !strings.Contains(view, "redis") {
		t.Fatalf("stats view missing containers:\n%s", view)
	}
	if !strings.Contains(view, "12.5") {
		t.Fatalf("stats view missing cpu:\n%s", view)
	}
}

func TestStatsViewShowsRuntimeLabels(t *testing.T) {
	docker := runtime.NewClientForSocket("/var/run/docker.sock")
	podman := runtime.NewClientForSocket("/run/user/1000/podman/podman.sock")
	m := NewModel(
		[]string{"api", "api"},
		[]*runtime.Client{docker, podman},
		runtime.LogOptions{},
		0,
	)
	m.view = viewStats
	m.width = 100
	m.height = 24

	view := m.View()
	if !strings.Contains(view, "docker") {
		t.Fatalf("stats view missing docker label:\n%s", view)
	}
	if !strings.Contains(view, "podman") {
		t.Fatalf("stats view missing podman label:\n%s", view)
	}
}

func TestEmptyState(t *testing.T) {
	m := testModel(nil)
	m.width = 80
	m.height = 24

	view := m.View()
	if !strings.Contains(view, "no containers") {
		t.Fatalf("empty state missing message:\n%s", view)
	}
}

func TestEscInFocusedGoesBack(t *testing.T) {
	m := testModel([]string{"api", "worker"})
	m.focused = true

	model, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = model.(*Model)
	if m.focused {
		t.Fatal("esc did not unfocus")
	}
	if cmd == nil || cmd() == nil {
		t.Fatal("esc did not request clear screen")
	}
}

func TestConnectLocalServerUsesConfiguredSocket(t *testing.T) {
	servers := []config.Server{{
		Host:       "localhost",
		Socket:     "/run/user/1000/podman/podman.sock",
		Containers: []string{"api"},
	}}
	m := NewModel(nil, nil, runtime.LogOptions{}, 0, servers)

	msg := m.connectToServer(0)()
	got, ok := msg.(serverConnectMsg)
	if !ok {
		t.Fatalf("msg = %#v", msg)
	}
	if got.err != nil {
		t.Fatal(got.err)
	}
	if got.client.SocketPath != "unix:///run/user/1000/podman/podman.sock" {
		t.Fatalf("socket = %q", got.client.SocketPath)
	}
}

func TestViewClampsServerSelectionWhenShowingContainers(t *testing.T) {
	m := testModel([]string{"api"})
	m.view = viewStats
	m.selected = 5
	m.width = 80
	m.height = 24

	view := m.View()
	if !strings.Contains(view, "api") {
		t.Fatalf("view missing container:\n%s", view)
	}
	if m.selected != 0 {
		t.Fatalf("selected = %d, want 0", m.selected)
	}
}

func TestViewClearsFocusWhenServerSelectionExceedsContainers(t *testing.T) {
	servers := []config.Server{
		{Host: "one", Containers: []string{"a"}},
		{Host: "two", Containers: []string{"b"}},
		{Host: "three", Containers: []string{"c"}},
	}
	m := NewModel([]string{"api"}, []*runtime.Client{runtime.NewClientForSocket("/var/run/docker.sock")}, runtime.LogOptions{}, 0, servers)
	m.view = viewServers
	m.focused = true
	m.selected = 2
	m.width = 80
	m.height = 24

	view := m.View()
	if !strings.Contains(view, "servers") {
		t.Fatalf("view missing servers header:\n%s", view)
	}
	if m.focused {
		t.Fatal("focus should be cleared when selected is not a valid container")
	}
}

func TestViewHandlesShortClientSlice(t *testing.T) {
	m := NewModel([]string{"api", "worker"}, nil, runtime.LogOptions{}, 0)
	m.view = viewPS
	m.width = 80
	m.height = 24
	m.containersInfo = []runtime.Container{
		{ID: "abc123def456", Names: []string{"/api"}, Image: "nginx:1.25", State: "running", Status: "Up"},
	}

	view := m.View()
	if !strings.Contains(view, "api") {
		t.Fatalf("view missing container:\n%s", view)
	}
}

func TestDisconnectServerBoundsContainerSlices(t *testing.T) {
	servers := []config.Server{{
		Host:       "localhost",
		Socket:     "/var/run/docker.sock",
		Containers: []string{"api", "worker"},
	}}
	m := NewModel([]string{"api"}, []*runtime.Client{runtime.NewClientForSocket("/var/run/docker.sock")}, runtime.LogOptions{}, 0, servers)
	m.serverStatus[0] = "connected"
	m.serverContainerStart[0] = 0

	m.disconnectServer(0)

	if len(m.containers) != 0 {
		t.Fatalf("containers = %#v, want empty", m.containers)
	}
	if len(m.clients) != 0 || len(m.containerNames) != 0 || len(m.containerRuntimes) != 0 {
		t.Fatalf("parallel container slices not trimmed: clients=%d names=%d runtimes=%d", len(m.clients), len(m.containerNames), len(m.containerRuntimes))
	}
}
