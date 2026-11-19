package vm

import (
	"math/rand"
	"strings"
	"testing"
)

// The cached line table must answer exactly as the scanning reference for
// every offset, including out-of-range offsets and sources without newlines.
func TestSourceLineTableMatchesScan(t *testing.T) {
	sources := []string{
		"",
		"Integer i = 1;",
		"\n",
		"\n\n\n",
		"a\nbb\nccc\n",
		"no trailing newline\nsecond line",
		strings.Repeat("System.debug('x');\n", 300) + "Integer tail = 0;",
	}
	rng := rand.New(rand.NewSource(1))
	for i := 0; i < 20; i++ {
		var b strings.Builder
		for j := 0; j < rng.Intn(4000); j++ {
			if rng.Intn(17) == 0 {
				b.WriteByte('\n')
			} else {
				b.WriteByte(byte('a' + rng.Intn(26)))
			}
		}
		sources = append(sources, b.String())
	}
	machine := New(nil)
	for _, source := range sources {
		for offset := -2; offset <= len(source)+2; offset++ {
			wantLine, wantColumn := sourceLineColumn(source, offset)
			gotLine, gotColumn := machine.sourceLineColumnCached(source, offset)
			if gotLine != wantLine || gotColumn != wantColumn {
				t.Fatalf("source %q offset %d: cached %d:%d, scan %d:%d", source, offset, gotLine, gotColumn, wantLine, wantColumn)
			}
		}
	}
	if tables := len(machine.sourceLineTables); tables == 0 {
		t.Fatal("no line tables were cached")
	}
}

// Two distinct sources of equal length share a cache bucket and keep their own
// tables.
func TestSourceLineTableDistinguishesEqualLengthSources(t *testing.T) {
	machine := New(nil)
	a, b := "ab\ncd\nef", "abcde\nfg"
	for offset := 0; offset <= len(a); offset++ {
		if l, c := machine.sourceLineColumnCached(a, offset); func() bool { wl, wc := sourceLineColumn(a, offset); return wl != l || wc != c }() {
			t.Fatalf("source a offset %d mismatch", offset)
		}
		if l, c := machine.sourceLineColumnCached(b, offset); func() bool { wl, wc := sourceLineColumn(b, offset); return wl != l || wc != c }() {
			t.Fatalf("source b offset %d mismatch", offset)
		}
	}
	if got := len(machine.sourceLineTables[len(a)]); got != 2 {
		t.Fatalf("equal-length sources cached %d tables, want 2", got)
	}
}
