package cliui

import (
	"strings"
	"testing"
)

func TestThemePlainWhenNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	th := NewTheme(nil)
	if th.Color {
		t.Fatal("expected plain theme")
	}
	if th.GlyphPass != "✓" {
		t.Fatalf("pass glyph = %q", th.GlyphPass)
	}
}

func TestThemeANSIWhenColorEnabled(t *testing.T) {
	th := Theme{Color: true}
	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "green", got: th.Green("ok"), want: "\x1b[38;2;183;198;143m"},
		{name: "cyan", got: th.Cyan("ok"), want: "\x1b[38;2;120;151;184m"},
		{name: "magenta", got: th.Magenta("ok"), want: "\x1b[38;2;182;202;223m"},
		{name: "dim", got: th.Dim("ok"), want: "\x1b[2;38;2;143;162;162m"},
	}
	for _, tt := range tests {
		if !strings.Contains(tt.got, tt.want) {
			t.Fatalf("%s = %q, want ANSI code %q", tt.name, tt.got, tt.want)
		}
	}
}

func TestNewPlainTheme(t *testing.T) {
	th := NewPlainTheme()
	if th.Color {
		t.Fatal("expected plain")
	}
	if !th.PlainBox {
		t.Fatal("expected plain boxes")
	}
}
