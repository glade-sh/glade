package apexversion

import (
	"math"
	"strconv"
	"strings"
	"testing"
)

var versionResult int
var versionOK bool

func TestVersionSingleCutMatchesOriginal(t *testing.T) {
	compare := func(s string) {
		a, okA := Major(s)
		b, okB := originalMajor(s)
		if a != b || okA != okB {
			t.Fatalf("input %q: %d/%v, want %d/%v", s, a, okA, b, okB)
		}
	}
	var walk func(string, int)
	walk = func(s string, left int) {
		compare(s)
		if left == 0 {
			return
		}
		for _, r := range []rune{'0', '1', '.', '9', '+', '-', ' ', 'x', '\u00a0'} {
			walk(s+string(r), left-1)
		}
	}
	walk("", 5)
	for _, s := range []string{strconv.Itoa(math.MaxInt), strconv.Itoa(math.MaxInt) + "0", "9223372036854775808.0", "67.999999999999999999999", "\u200367.0\u2002", "1." + strings.Repeat("0", 300), "67.0.0", "1..", "01.00", "\xff.0"} {
		compare(s)
	}
}

func BenchmarkVersionParse(b *testing.B) {
	inputs := []string{"67.0", "62", "  63.15  ", "67.0.0"}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		versionResult, versionOK = Major(inputs[i%len(inputs)])
	}
}

func originalMajor(raw string) (int, bool) {
	parts := strings.Split(strings.TrimSpace(raw), ".")
	if len(parts) == 0 || parts[0] == "" {
		return 0, false
	}
	major, ok := unsignedComponent(parts[0])
	if !ok || major < 1 {
		return 0, false
	}
	if len(parts) > 2 {
		return 0, false
	}
	if len(parts) == 2 {
		if parts[1] == "" {
			return 0, false
		}
		if _, ok := unsignedComponent(parts[1]); !ok {
			return 0, false
		}
	}
	return major, true
}
