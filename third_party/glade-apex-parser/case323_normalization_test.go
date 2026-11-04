//go:build cgo

package apexast

import (
	"strings"
	"testing"
)

func TestCase323NormalizationPreservesBytes(t *testing.T) {
	for _, source := range []string{"", "class A {}", "// void() Trigger.x Package.X\n'void()'", "é\xff\n/* text */", "class B { Integer x; }"} {
		if got := normalizeApexSource(source); got != source {
			t.Fatalf("unchanged source %q became %q", source, got)
		}
	}
	for _, tc := range []struct{ source, from, to string }{
		{"void();", "void", voidIdentifierSentinel},
		{"Trigger.new", "Trigger", triggerContextSentinel},
		{"Package.Version", "Package", packageQualifiedTypeSentinel},
		{"this();", "this", "thiz"},
		{"super();", "super", "soper"},
	} {
		want := strings.Replace(tc.source, tc.from, tc.to, 1)
		if got := normalizeApexSource(tc.source); got != want {
			t.Fatalf("%q: got %q want %q", tc.source, got, want)
		}
	}
}
