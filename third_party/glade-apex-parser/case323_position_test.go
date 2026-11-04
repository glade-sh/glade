package apexast

import "testing"

func TestCase323LineMapPositions(t *testing.T) {
	for _, source := range []string{"", "x", "\n", "a\nb\n", "a\r\nb\r\n", "a\rb", "é\nİKſΣς\r\n", "a\xff\xc0\xaf\n\x00b"} {
		m := NewLineMap(source)
		for offset := -1; offset <= len(source)+2; offset++ {
			o := offset
			if o < 0 {
				o = 0
			}
			line := 0
			for line+1 < len(m.starts) && m.starts[line+1] <= o {
				line++
			}
			want := Position{Line: line + 1, Column: o - m.starts[line] + 1, Offset: o}
			if got := m.Position(offset); got != want {
				t.Fatalf("%q offset %d: got %+v want %+v", source, offset, got, want)
			}
		}
	}
}
