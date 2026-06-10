package cliui

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

type TTYRenderer struct {
	w          io.Writer
	clock      Clock
	theme      Theme
	feed       *ActivityFeed
	started    time.Time
	last       Event
	frame      int
	lines      int
	width      int
	isTTY      bool
	mu         sync.Mutex
	ticker     *time.Ticker
	done       chan struct{}
	doneOnce   sync.Once
	phaseStart map[string]time.Time
}

func NewTTYRenderer(w io.Writer, clock Clock) *TTYRenderer {
	if clock == nil {
		clock = systemClock{}
	}
	r := &TTYRenderer{
		w:          w,
		clock:      clock,
		theme:      NewTheme(w),
		feed:       NewActivityFeed(5),
		started:    clock.Now(),
		phaseStart: make(map[string]time.Time),
	}
	r.done = make(chan struct{})
	r.isTTY = IsTerminalWriter(w)
	r.startTicker()
	return r
}

func (r *TTYRenderer) startTicker() {
	if r == nil || !r.isTTY || r.w == nil {
		return
	}
	r.ticker = time.NewTicker(150 * time.Millisecond)
	go func() {
		for {
			select {
			case <-r.ticker.C:
				r.tick()
			case <-r.done:
				return
			}
		}
	}()
}

func (r *TTYRenderer) Render(ev Event) {
	if r == nil || r.w == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if ev.At.IsZero() {
		ev.At = r.clock.Now()
	}
	switch ev.Kind {
	case EventPhaseStart:
		r.phaseStart[ev.Phase] = ev.At
	case EventPhaseEnd:
		if start, ok := r.phaseStart[ev.Phase]; ok {
			duration := FormatDuration(ev.At.Sub(start))
			resolved := fmt.Sprintf("%s %s (%s)", r.theme.GlyphPass, strings.TrimSpace(ev.Label), duration)
			if !r.theme.Color {
				resolved = fmt.Sprintf("%s %s (%s)", r.theme.GlyphPass, strings.TrimSpace(ev.Label), duration)
			}
			r.feed.Add(Event{Kind: EventInfo, Label: resolved})
		}
	case EventInfo, EventWarn, EventFail, EventPhaseTick:
		r.feed.Add(ev)
	}
	r.last = ev
	r.frame++
	r.draw(false, true, "")
}

func (r *TTYRenderer) Finish(result Result) {
	if r == nil || r.w == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stop()
	r.draw(true, result.OK, result.Label)
	fmt.Fprint(r.w, "\n")
	r.lines = 0
}

func (r *TTYRenderer) tick() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r == nil || r.w == nil {
		return
	}
	r.frame++
	r.draw(false, true, "")
}

func (r *TTYRenderer) stop() {
	if r == nil {
		return
	}
	r.doneOnce.Do(func() {
		if r.ticker != nil {
			r.ticker.Stop()
		}
		close(r.done)
	})
}

func (r *TTYRenderer) draw(done bool, ok bool, doneLabel string) {
	if r.lines > 0 {
		for i := 0; i < r.lines; i++ {
			fmt.Fprint(r.w, "\x1b[1A\r\x1b[K")
		}
	} else {
		fmt.Fprint(r.w, "\r\x1b[K")
	}
	lines := r.renderLines(done, ok, doneLabel)
	width := r.terminalWidth()
	for _, line := range lines {
		fmt.Fprintln(r.w, truncateCell(line, width))
	}
	r.lines = len(lines)
}

func (r *TTYRenderer) renderLines(done bool, ok bool, doneLabel string) []string {
	ev := r.last
	elapsed := r.theme.Dim(FormatDuration(r.clock.Now().Sub(r.started)))
	if !r.theme.Color {
		elapsed = FormatDuration(r.clock.Now().Sub(r.started))
	}
	bar := RenderProgressBarStyled(r.theme, ev.Current, ev.Total, 22)
	icon := r.theme.SpinnerFrame(r.frame)
	label := strings.TrimSpace(ev.Label)
	if done {
		icon = r.theme.GlyphFail
		if ok {
			icon = r.theme.GlyphPass
		}
		if r.theme.Color {
			if ok {
				icon = r.theme.Green(icon)
			} else {
				icon = r.theme.Red(icon)
			}
		}
		if doneLabel != "" {
			label = doneLabel
		}
	}
	count := ""
	if ev.Total > 0 {
		count = fmt.Sprintf(" · %d/%d", ev.Current, ev.Total)
	}
	status := fmt.Sprintf("%s %s%s %s · %s", icon, bar, count, label, elapsed)
	lines := []string{status}
	for _, line := range r.feed.Lines() {
		lines = append(lines, "  "+r.colorFeedLine(line))
	}
	return lines
}

func (r *TTYRenderer) colorFeedLine(line string) string {
	trimmed := strings.TrimSpace(line)
	switch {
	case strings.HasPrefix(trimmed, r.theme.GlyphPass),
		strings.HasPrefix(trimmed, "PASS"),
		strings.Contains(trimmed, " PASS "):
		if r.theme.Color {
			return r.theme.Green(line)
		}
	case strings.HasPrefix(trimmed, r.theme.GlyphFail),
		strings.HasPrefix(trimmed, "FAIL"),
		strings.HasPrefix(trimmed, "setup failed"),
		strings.Contains(trimmed, " FAIL "):
		if r.theme.Color {
			return r.theme.Red(line)
		}
	case strings.HasPrefix(trimmed, "WARN"),
		strings.HasPrefix(trimmed, r.theme.GlyphWarn):
		if r.theme.Color {
			return r.theme.Yellow(line)
		}
	}
	return line
}

func (r *TTYRenderer) SetWidthForTest(width int) {
	r.width = width
}

func (r *TTYRenderer) terminalWidth() int {
	if r.width > 0 {
		return r.width
	}
	return defaultBoxWidth
}

func truncateCell(s string, width int) string {
	if width <= 0 || VisibleWidth(s) <= width {
		return s
	}
	return TruncateVisible(s, width)
}
