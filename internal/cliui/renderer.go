package cliui

import (
	"io"
	"os"
	"time"
)

type ProgressMode string

const (
	ProgressAuto    ProgressMode = "auto"
	ProgressTTY     ProgressMode = "tty"
	ProgressVisible ProgressMode = "visible"
	ProgressLine    ProgressMode = "line"
	ProgressJSON    ProgressMode = "json"
	ProgressOff     ProgressMode = "off"
)

type Clock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time {
	return time.Now()
}

type Renderer interface {
	Render(Event)
	Finish(Result)
}

type RendererOptions struct {
	Stderr io.Writer
	Mode   ProgressMode
	Clock  Clock
}

func NewRenderer(opts RendererOptions) Renderer {
	if opts.Stderr == nil || opts.Mode == ProgressOff {
		return NullRenderer{}
	}
	if opts.Clock == nil {
		opts.Clock = systemClock{}
	}
	switch opts.Mode {
	case ProgressJSON:
		return NewNDJSONRenderer(opts.Stderr)
	case ProgressTTY:
		return NewTTYRenderer(opts.Stderr, opts.Clock)
	case ProgressVisible:
		if IsTerminalWriter(opts.Stderr) {
			return NewTTYRenderer(opts.Stderr, opts.Clock)
		}
		return NewLineRenderer(opts.Stderr, opts.Clock)
	case ProgressLine:
		return NewLineRenderer(opts.Stderr, opts.Clock)
	case ProgressAuto, "":
		if IsTerminalWriter(opts.Stderr) {
			return NewTTYRenderer(opts.Stderr, opts.Clock)
		}
		return NullRenderer{}
	default:
		return NewLineRenderer(opts.Stderr, opts.Clock)
	}
}

type NullRenderer struct{}

func (NullRenderer) Render(Event)  {}
func (NullRenderer) Finish(Result) {}

func IsTerminalWriter(w io.Writer) bool {
	file, ok := w.(*os.File)
	if !ok || file == nil {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
