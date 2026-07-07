package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestVisibleContainersFocus(t *testing.T) {
	m := NewModel([]string{"api", "worker", "db"})

	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = model.(*Model)
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = model.(*Model)

	view := m.View()
	if !strings.Contains(view, "worker") || strings.Contains(view, "api") || strings.Contains(view, "db") {
		t.Fatalf("focused view = %q, want only worker", view)
	}

	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m = model.(*Model)
	view = m.View()
	if !strings.Contains(view, "api") || strings.Contains(view, "worker") || strings.Contains(view, "db") {
		t.Fatalf("focused view = %q, want only api", view)
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("abcdef", 4); got != "a..." {
		t.Fatalf("truncate = %q, want a...", got)
	}
	if got := truncate("abcdef", 2); got != "ab" {
		t.Fatalf("truncate = %q, want ab", got)
	}
}

func TestViewFillsTerminalHeight(t *testing.T) {
	m := NewModel([]string{"api"})
	m.width = 40
	m.height = 10

	lines := strings.Split(m.View(), "\n")
	if len(lines) != 10 {
		t.Fatalf("view height = %d, want 10", len(lines))
	}
}

func TestViewKeepsPanelBottomBorders(t *testing.T) {
	m := NewModel([]string{"api", "worker", "db"})
	m.width = 80
	m.height = 18

	view := m.View()
	if got := strings.Count(view, "╰"); got != 3 {
		t.Fatalf("bottom borders = %d, want 3\n%s", got, view)
	}
}
