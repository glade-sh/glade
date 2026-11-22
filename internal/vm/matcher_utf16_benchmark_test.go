package vm

import (
	"fmt"
	"strings"
	"testing"
)

func BenchmarkMatcherFindSupplementaryPositions(b *testing.B) {
	const pairs = 1024
	subject := strings.Repeat("😀a", pairs)
	source := fmt.Sprintf("Matcher m = Pattern.compile('.').matcher('%s'); Integer hits=0; Integer positions=0; while(m.find()) { hits++; positions += m.start(); }", subject)
	program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: "67.0"})
	if err != nil {
		b.Fatal(err)
	}
	machine := New(nil)
	machine.SetTraceEnabled(false)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		clear(machine.Globals)
		clear(machine.VarTypes)
		machine.ResetLimits()
		result, err := machine.Execute(program)
		if err != nil {
			b.Fatal(err)
		}
		if got := result.Vars["hits"]; got.Kind != ValueInt || got.Int != 2*pairs {
			b.Fatalf("unexpected count: %v", got)
		}
		if got := result.Vars["positions"]; got.Kind != ValueInt || got.Int != 3*pairs*pairs-pairs {
			b.Fatalf("unexpected position sum: %v", got)
		}
	}
}

func BenchmarkMatcherFindSupplementaryEmpty(b *testing.B) {
	const pairs = 256
	source := fmt.Sprintf("Matcher m = Pattern.compile('').matcher('%s'); Integer hits=0; while(m.find()) { hits++; }", strings.Repeat("😀a", pairs))
	program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: "67.0"})
	if err != nil {
		b.Fatal(err)
	}
	machine := New(nil)
	machine.SetTraceEnabled(false)
	var hits int64
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		clear(machine.Globals)
		clear(machine.VarTypes)
		machine.ResetLimits()
		result, err := machine.Execute(program)
		if err != nil {
			b.Fatal(err)
		}
		value := result.Vars["hits"]
		if value.Kind != ValueInt || (value.Int != 2*pairs+1 && value.Int != 3*pairs+1) {
			b.Fatalf("unexpected count: %v", value)
		}
		hits = value.Int
	}
	b.ReportMetric(float64(hits), "matches/op")
}

func BenchmarkMatcherFindASCIIEmpty(b *testing.B) {
	const size = 1024
	source := fmt.Sprintf("Matcher m = Pattern.compile('').matcher('%s'); Integer hits=0; while(m.find()) { hits++; }", strings.Repeat("a", size))
	program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: "67.0"})
	if err != nil {
		b.Fatal(err)
	}
	machine := New(nil)
	machine.SetTraceEnabled(false)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		clear(machine.Globals)
		clear(machine.VarTypes)
		machine.ResetLimits()
		result, err := machine.Execute(program)
		if err != nil {
			b.Fatal(err)
		}
		if got := result.Vars["hits"]; got.Kind != ValueInt || got.Int != size+1 {
			b.Fatalf("unexpected count: %v", got)
		}
	}
}
