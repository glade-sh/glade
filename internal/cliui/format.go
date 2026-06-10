package cliui

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

const defaultBoxWidth = 80

func FormatDurationMS(ms int64) string {
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	return FormatDuration(time.Duration(ms) * time.Millisecond)
}

func StripANSI(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' {
			j := i + 1
			if j < len(s) && s[j] == '[' {
				for j < len(s) && s[j] != 'm' {
					j++
				}
				if j < len(s) {
					i = j + 1
					continue
				}
			}
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

func VisibleWidth(s string) int {
	return utf8.RuneCountInString(StripANSI(s))
}

func TruncateVisible(s string, max int) string {
	if max <= 0 || VisibleWidth(s) <= max {
		return s
	}
	if max <= 3 {
		return truncateRunes(s, max)
	}
	return truncateRunes(s, max-3) + "..."
}

func truncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	n := 0
	for i := range s {
		if n == max {
			return s[:i]
		}
		n++
	}
	return s
}

func PadVisible(s string, width int) string {
	gap := width - VisibleWidth(s)
	if gap <= 0 {
		return s
	}
	return s + strings.Repeat(" ", gap)
}

func FormatBox(t Theme, title, content string, width int) string {
	if width <= 0 {
		width = defaultBoxWidth
	}
	lines := wrapVisible(content, width-4)
	if t.PlainBox {
		return formatBoxASCII(title, lines, width)
	}
	return formatBoxUnicode(t, title, lines, width)
}

func formatBoxUnicode(t Theme, title string, lines []string, width int) string {
	innerWidth := width - 2 // between corner glyphs
	var out strings.Builder
	topFill := "─ " + title + " "
	if VisibleWidth(topFill) > innerWidth {
		topFill = TruncateVisible(topFill, innerWidth)
	}
	topPad := innerWidth - VisibleWidth(topFill)
	out.WriteString("╭" + topFill + strings.Repeat("─", topPad) + "╮\n")
	for _, line := range lines {
		inner := "  " + line
		if VisibleWidth(inner) > innerWidth {
			inner = TruncateVisible(inner, innerWidth)
		}
		padding := innerWidth - VisibleWidth(inner)
		out.WriteString("│" + inner + strings.Repeat(" ", padding) + "│\n")
	}
	out.WriteString("╰" + strings.Repeat("─", innerWidth) + "╯")
	return out.String()
}

func formatBoxASCII(title string, lines []string, width int) string {
	innerWidth := width - 2
	var out strings.Builder
	topFill := "- " + title + " "
	if VisibleWidth(topFill) > innerWidth {
		topFill = TruncateVisible(topFill, innerWidth)
	}
	topPad := innerWidth - VisibleWidth(topFill)
	out.WriteString("+" + topFill + strings.Repeat("-", topPad) + "+\n")
	for _, line := range lines {
		inner := "  " + line
		if VisibleWidth(inner) > innerWidth {
			inner = TruncateVisible(inner, innerWidth)
		}
		padding := innerWidth - VisibleWidth(inner)
		out.WriteString("|" + inner + strings.Repeat(" ", padding) + "|\n")
	}
	out.WriteString("+" + strings.Repeat("-", innerWidth) + "+")
	return out.String()
}

func FormatRow(t Theme, icon, label, value string) string {
	label = strings.TrimSpace(label)
	value = strings.TrimSpace(value)
	if value == "" {
		return "  " + icon + "  " + label
	}
	return fmt.Sprintf("  %s  %-24s  %s", icon, label, value)
}

func FormatSeparator(width int) string {
	if width <= 0 {
		width = defaultBoxWidth
	}
	return strings.Repeat("─", width)
}

func wrapVisible(text string, width int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if width < 8 {
		width = 8
	}
	var lines []string
	for _, part := range strings.FieldsFunc(text, func(r rune) bool { return r == '\n' }) {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		for len(part) > 0 {
			if VisibleWidth(part) <= width {
				lines = append(lines, part)
				break
			}
			cut := width
			chunk := truncateRunes(part, cut)
			if idx := strings.LastIndex(chunk, " "); idx > 0 && VisibleWidth(part) > width {
				chunk = part[:idx]
			}
			lines = append(lines, strings.TrimSpace(chunk))
			part = strings.TrimSpace(part[len(chunk):])
		}
	}
	return lines
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
