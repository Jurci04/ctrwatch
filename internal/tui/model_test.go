package tui

import (
	"strings"
	"testing"

	"ctrwatch/internal/runtime"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestViewLayoutInvariants(t *testing.T) {
	m := NewModel([]string{"api", "worker", "db"})
	m.width = 80
	m.height = 47
	longLine := "\x1b[31m" + strings.Repeat("x", 200) + "\x1b[0m\r"
	for _, name := range m.containers {
		m.lines[name] = []runtime.LogLine{{Container: name, Text: longLine}}
	}

	view := m.View()
	lines := strings.Split(view, "\n")
	if len(lines) != 47 {
		t.Fatalf("view height = %d, want 47", len(lines))
	}
	if got := strings.Count(view, "╰"); got != 3 {
		t.Fatalf("bottom borders = %d, want 3\n%s", got, view)
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

	if _, ok := logLineColor(visibleLogLine{text: "2026 info ok"}); !ok {
		t.Fatal("lowercase info was not classified")
	}
	if _, ok := logLineColor(visibleLogLine{text: "plain"}); ok {
		t.Fatal("plain line was classified")
	}
}

func TestFocusClearsScreenButTabDoesNot(t *testing.T) {
	m := NewModel([]string{"api", "worker"})

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
