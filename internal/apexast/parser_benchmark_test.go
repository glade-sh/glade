package apexast

import (
	"fmt"
	"strings"
	"testing"
)

func BenchmarkParseClass(b *testing.B) {
	src := benchmarkApexClass(40)
	parser := NewParser()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		file := parser.ParseSource("Benchmark.cls", src)
		if len(file.Diagnostics) != 0 {
			b.Fatalf("unexpected diagnostics: %#v", file.Diagnostics)
		}
	}
}

func benchmarkApexClass(methods int) string {
	var src strings.Builder
	src.WriteString("public with sharing class Benchmark {\n")
	src.WriteString("  public Integer seed { get; set; }\n")
	for i := 0; i < methods; i++ {
		fmt.Fprintf(&src, "  public static Integer method%d(Integer value) {\n", i)
		src.WriteString("    Integer total = 0;\n")
		src.WriteString("    for (Integer i = 0; i < value; i++) {\n")
		src.WriteString("      total = total + i;\n")
		src.WriteString("    }\n")
		src.WriteString("    return total;\n")
		src.WriteString("  }\n")
	}
	src.WriteString("}\n")
	return src.String()
}
