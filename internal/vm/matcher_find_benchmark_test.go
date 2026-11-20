package vm

import (
	"strings"
	"testing"
)

func BenchmarkMatcherFind4096(b *testing.B) {
	source := "Matcher m = Pattern.compile('a').matcher('" + strings.Repeat("a", 4096) + "'); Integer hits=0; while (m.find()) { hits++; } System.assertEquals(4096, hits);"
	program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: "67.0"})
	if err != nil {
		b.Fatal(err)
	}
	m := New(nil)
	m.SetTraceEnabled(false)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		clear(m.Globals)
		clear(m.VarTypes)
		m.ResetLimits()
		result, err := m.Execute(program)
		if err != nil {
			b.Fatal(err)
		}
		if got := result.Vars["hits"]; got.Kind != ValueInt || got.Int != 4096 {
			b.Fatalf("unexpected count: %v", got)
		}
	}
}
