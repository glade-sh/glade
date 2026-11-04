package apexast

import (
	"github.com/glade-sh/glade/internal/diagnostic"
	"testing"
)

func TestCase323AdapterLineMapPositions(t *testing.T) {
	for _, source := range []string{"", "a\nb\n", "a\r\nb", "é\n\xff"} {
		m := NewLineMap(source)
		for offset := -1; offset <= len(source)+2; offset++ {
			o := offset
			if o < 0 {
				o = 0
			}
			if o > len(source) {
				o = len(source)
			}
			line := 0
			for line+1 < len(m.starts) && m.starts[line+1] <= o {
				line++
			}
			want := diagnostic.Position{Line: line + 1, Column: o - m.starts[line] + 1, Offset: o}
			if got := m.Position(offset); got != want {
				t.Fatalf("%q offset %d: got %+v want %+v", source, offset, got, want)
			}
		}
	}
}
