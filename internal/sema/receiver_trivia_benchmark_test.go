package sema

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// BenchmarkReceiverTriviaAnalysis measures complete semantic analysis for an
// ordinary unit and one dominated by fluent builder calls.
func BenchmarkReceiverTriviaAnalysis(b *testing.B) {
	root := b.TempDir()
	path := filepath.Join(root, "FluentProbe.cls")
	if err := os.WriteFile(path, []byte("public class FluentProbe { public FluentProbe step() { return this; } }"), 0600); err != nil {
		b.Fatal(err)
	}
	index := typesys.Build(project.Project{Root: root, SourceAPIVersion: "67.0", ApexFiles: []string{path}}, schema.Schema{})
	var plain, chain, freeChain strings.Builder
	chain.WriteString("FluentProbe p = new FluentProbe();\n")
	freeChain.WriteString("FluentProbe p = new FluentProbe();\n")
	for i := 0; i < 30; i++ {
		fmt.Fprintf(&plain, "Integer v%d = %d + 1;\n", i, i)
		chain.WriteString("p.step().step().step().step().step();\n")
	}
	for i := 0; i < 100; i++ {
		freeChain.WriteString("p\n.step().step().step().step().step();\n")
	}
	for _, tc := range []struct{ name, source string }{{"no_breaks", plain.String()}, {"builder_chain", chain.String()}, {"builder_free", freeChain.String()}} {
		b.Run(tc.name, func(b *testing.B) {
			if result := AnalyzeAnonymous(index, tc.source, "67.0"); result.HasErrors() {
				b.Fatal(result.Diagnostics)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				AnalyzeAnonymous(index, tc.source, "67.0")
			}
		})
	}
}
