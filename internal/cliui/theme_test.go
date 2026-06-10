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
	got := th.Green("ok")
	if !strings.Contains(got, "\x1b[32m") {
		t.Fatalf("green = %q", got)
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
