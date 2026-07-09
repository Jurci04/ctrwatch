package tui

import (
	"strings"

	"ctrwatch/src/runtime"

	"github.com/charmbracelet/lipgloss"
)

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
