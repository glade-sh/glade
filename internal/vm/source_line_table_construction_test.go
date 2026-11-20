package vm

import (
	"reflect"
	"strings"
	"testing"
)

func TestSourceLineTableConstruction(t *testing.T) {
	for _, source := range []string{"", "single line", "\n", "\n\n", "α\r\nβ\n", strings.Repeat("System.debug('value');\n", 1000)} {
		var offsets []int
		for i := range len(source) {
			if source[i] == '\n' {
				offsets = append(offsets, i)
			}
		}
		table := newSourceLineTable(source)
		if table.source != source || !reflect.DeepEqual(table.newlines, offsets) {
			t.Fatalf("source length %d: newline offsets differ", len(source))
		}
		for offset := -1; offset <= len(source)+1; offset++ {
			line, column := table.lineColumn(offset)
			wantLine, wantColumn := sourceLineColumn(source, offset)
			if line != wantLine || column != wantColumn {
				t.Fatalf("source length %d offset %d: got %d:%d, want %d:%d", len(source), offset, line, column, wantLine, wantColumn)
			}
		}
	}
}

func BenchmarkSourceLineTableConstruction(b *testing.B) {
	source := strings.Repeat("System.debug('value');\n", 1000)
	b.ReportAllocs()
	b.SetBytes(int64(len(source)))
	for b.Loop() {
		_ = newSourceLineTable(source)
	}
}
