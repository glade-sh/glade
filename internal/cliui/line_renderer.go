package cliui

import (
	"fmt"
	"io"
	"strings"
	"time"
)

type LineRenderer struct {
	w       io.Writer
	clock   Clock
	started time.Time
}

func NewLineRenderer(w io.Writer, clock Clock) *LineRenderer {
	if clock == nil {
		clock = systemClock{}
	}
	return &LineRenderer{w: w, clock: clock}
}

func (r *LineRenderer) Render(ev Event) {
	if r == nil || r.w == nil {
		return
	}
	if r.started.IsZero() {
		r.started = r.clock.Now()
	}
	now := r.clock.Now()
	elapsed := FormatDuration(now.Sub(r.started))
	phase := strings.TrimSpace(ev.Phase)
	label := strings.TrimSpace(ev.Label)
	detail := strings.TrimSpace(ev.Detail)
	switch ev.Kind {
	case EventFail:
		if detail != "" {
			fmt.Fprintf(r.w, "✗ %s: %s - %s · %s\n", phase, label, detail, elapsed)
			return
		}
		fmt.Fprintf(r.w, "✗ %s: %s · %s\n", phase, label, elapsed)
	case EventWarn:
		if ev.Total > 0 {
			fmt.Fprintf(r.w, "⚠ %s: %d/%d %s · %s\n", phase, ev.Current, ev.Total, label, elapsed)
			return
		}
		fmt.Fprintf(r.w, "⚠ %s: %s · %s\n", phase, label, elapsed)
	case EventInfo:
		if ev.Total > 0 {
			fmt.Fprintf(r.w, "%s: %d/%d %s elapsed=%s\n", phase, ev.Current, ev.Total, label, elapsed)
			return
		}
		fmt.Fprintf(r.w, "%s: %s\n", phase, label)
	case EventPhaseTick:
		if ev.Total > 0 {
			fmt.Fprintf(r.w, "%s · %d/%d %s · %s\n", phase, ev.Current, ev.Total, label, elapsed)
			return
		}
		fmt.Fprintf(r.w, "%s · %s · %s\n", phase, label, elapsed)
	default:
		if label == "" {
			return
		}
		fmt.Fprintf(r.w, "%s · %s · %s\n", phase, label, elapsed)
	}
}

func (r *LineRenderer) Finish(result Result) {
	if r == nil || r.w == nil || strings.TrimSpace(result.Label) == "" {
		return
	}
	now := r.clock.Now()
	if r.started.IsZero() {
		r.started = now
	}
	fmt.Fprintf(r.w, "done · %s · %s\n", result.Label, FormatDuration(now.Sub(r.started)))
}
