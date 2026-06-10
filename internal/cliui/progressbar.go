package cliui

import (
	"fmt"
	"strings"
	"time"
)

func RenderProgressBar(current, total, width int) string {
	return renderProgressBar(current, total, width, '=', '>', ' ')
}

func RenderProgressBarStyled(t Theme, current, total, width int) string {
	if t.Color && total > 0 {
		bar := renderProgressBar(current, total, width, '█', '█', '░')
		return t.Cyan(bar)
	}
	return RenderProgressBar(current, total, width)
}

func renderProgressBar(current, total, width int, fill, head, empty rune) string {
	if width < 4 {
		width = 4
	}
	inner := width - 2
	pos := 0
	if total > 0 {
		if current < 0 {
			current = 0
		}
		if current > total {
			current = total
		}
		if inner > 1 {
			pos = current * (inner - 1) / total
		}
	} else {
		if inner > 0 {
			pos = current % inner
		}
	}
	var b strings.Builder
	b.WriteRune('[')
	for i := 0; i < inner; i++ {
		switch {
		case i < pos && total > 0:
			b.WriteRune(fill)
		case i == pos:
			b.WriteRune(head)
		default:
			b.WriteRune(empty)
		}
	}
	b.WriteRune(']')
	return b.String()
}

func FormatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	d = d.Round(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d/time.Second))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%02ds", int(d/time.Minute), int((d%time.Minute)/time.Second))
	}
	return fmt.Sprintf("%dh%02dm", int(d/time.Hour), int((d%time.Hour)/time.Minute))
}
