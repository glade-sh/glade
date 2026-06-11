package cliui

import (
	"io"
	"os"
	"strings"
)

var brailleSpinner = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

const (
	ansiLichen   = "38;2;183;198;143"
	ansiRed      = "38;2;214;116;103"
	ansiAmber    = "38;2;240;198;116"
	ansiTarn     = "38;2;120;151;184"
	ansiPaleTarn = "38;2;182;202;223"
	ansiDim      = "2;38;2;143;162;162"
)

type Theme struct {
	Color     bool
	PlainBox  bool
	GlyphPass string
	GlyphFail string
	GlyphWarn string
	GlyphSpin []string
}

func NewTheme(w io.Writer) Theme {
	color := ColorEnabled(IsTerminalWriter(w), os.Getenv("NO_COLOR"))
	return Theme{
		Color:     color,
		PlainBox:  !color,
		GlyphPass: "✓",
		GlyphFail: "✗",
		GlyphWarn: "⚠",
		GlyphSpin: append([]string(nil), brailleSpinner...),
	}
}

func NewPlainTheme() Theme {
	return Theme{
		Color:     false,
		PlainBox:  true,
		GlyphPass: "✓",
		GlyphFail: "✗",
		GlyphWarn: "⚠",
		GlyphSpin: []string{"…"},
	}
}

func (t Theme) Green(s string) string {
	return t.colorize(s, ansiLichen)
}

func (t Theme) Red(s string) string {
	return t.colorize(s, ansiRed)
}

func (t Theme) Yellow(s string) string {
	return t.colorize(s, ansiAmber)
}

func (t Theme) Cyan(s string) string {
	return t.colorize(s, ansiTarn)
}

func (t Theme) Magenta(s string) string {
	return t.colorize(s, ansiPaleTarn)
}

func (t Theme) Dim(s string) string {
	return t.colorize(s, ansiDim)
}

func (t Theme) Bold(s string) string {
	return t.colorize(s, "1")
}

func (t Theme) colorize(s, code string) string {
	if !t.Color || strings.TrimSpace(s) == "" {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

func (t Theme) SpinnerFrame(frame int) string {
	if len(t.GlyphSpin) == 0 {
		return "…"
	}
	return t.GlyphSpin[frame%len(t.GlyphSpin)]
}
