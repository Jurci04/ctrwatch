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
	// fast path: most log lines have no ANSI or control chars
	if !strings.ContainsAny(s, "\x1b\r\n\t") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
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

// ponytail: lipgloss borders produce double ││ when content already has pipe
// chars, Height(n) controls only the content area (not total), and Height
// doesn't truncate overflow — so we keep the hand-rolled ╭╮╰╯│ box layout.
var keywordReplacer = strings.NewReplacer(
	"CRITICAL", "\x1b[91mCRITICAL\x1b[0m",
	"critical", "\x1b[91mcritical\x1b[0m",
	"ERROR", "\x1b[91mERROR\x1b[0m",
	"error", "\x1b[91merror\x1b[0m",
	"WARNING", "\x1b[93mWARNING\x1b[0m",
	"warning", "\x1b[93mwarning\x1b[0m",
	"WARN", "\x1b[93mWARN\x1b[0m",
	"warn", "\x1b[93mwarn\x1b[0m",
	"INFO", "\x1b[92mINFO\x1b[0m",
	"info", "\x1b[92minfo\x1b[0m",
	"DEBUG", "\x1b[94mDEBUG\x1b[0m",
	"debug", "\x1b[94mdebug\x1b[0m",
)

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

func padBody(body []string, innerW, bodyHeight int, vl string) []string {
	contentHeight := max(bodyHeight-2, 1)
	for len(body) < contentHeight {
		body = append(body, vl+strings.Repeat(" ", innerW)+vl)
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
