package sema

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// incrementalDependencyGenerations models a short type name embedded in many
// unrelated identifiers, with both a real reference and an inherited type.
func incrementalDependencyGenerations(tb testing.TB) [2]incrementalGeneration {
	tb.Helper()
	root := tb.TempDir()
	dir := filepath.Join(root, "force-app", "main", "default", "classes")
	if err := os.MkdirAll(dir, 0755); err != nil {
		tb.Fatal(err)
	}
	write := func(path, data string) {
		tb.Helper()
		if err := os.WriteFile(path, []byte(data), 0644); err != nil {
			tb.Fatal(err)
		}
	}
	write(filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
	linePath := filepath.Join(dir, "Line.cls")
	write(linePath, `public virtual class Line { public Integer value() { return 1; } }`)
	for i := 0; i < 383; i++ {
		name := fmt.Sprintf("Pipeline%03d", i)
		source := fmt.Sprintf("public class %s { public Integer value() { Integer pipeline = 1; return pipeline; } }", name)
		if i == 0 {
			source = fmt.Sprintf("public class %s { public Integer value() { return new lInE().value(); } }", name)
		}
		if i == 1 {
			source = fmt.Sprintf("public class %s extends Line { public Integer other() { return 2; } }", name)
		}
		write(filepath.Join(dir, name+".cls"), source)
	}
	load := func() incrementalGeneration {
		tb.Helper()
		p, err := project.Load(root)
		if err != nil {
			tb.Fatal(err)
		}
		s, err := schema.LoadProject(p)
		if err != nil {
			tb.Fatal(err)
		}
		index, artifacts := typesys.BuildWithArtifacts(p, s)
		return incrementalGeneration{index: index, artifacts: artifacts}
	}
	before := load()
	write(linePath, `public virtual class Line { public Integer value() { return 2; } }`)
	return [2]incrementalGeneration{before, load()}
}

func incrementalDependencyJSON(tb testing.TB, result Result) []byte {
	tb.Helper()
	data, err := json.Marshal(result)
	if err != nil {
		tb.Fatal(err)
	}
	return data
}

func TestIncrementalIdentifierDependencies(t *testing.T) {
	generations := incrementalDependencyGenerations(t)
	state := NewIncrementalState()
	for i, gen := range generations {
		got, stats := state.Analyze(gen.index, incrementalOptions(&gen.artifacts))
		want := AnalyzeWithOptions(gen.index, incrementalOptions(&gen.artifacts))
		if string(incrementalDependencyJSON(t, got)) != string(incrementalDependencyJSON(t, want)) {
			t.Fatalf("generation %d: incremental and full results differ", i)
		}
		if i == 1 && (!stats.Incremental || stats.Rechecked != 3) {
			t.Fatalf("literal edit stats = %+v; want exactly the edited type, real reference and subtype", stats)
		}
	}
}

func BenchmarkIncrementalIdentifierDependencies(b *testing.B) {
	generations := incrementalDependencyGenerations(b)
	var expected [2]string
	for i, gen := range generations {
		expected[i] = string(incrementalDependencyJSON(b, AnalyzeWithOptions(gen.index, incrementalOptions(&gen.artifacts))))
	}
	state := NewIncrementalState()
	state.Analyze(generations[1].index, incrementalOptions(&generations[1].artifacts))
	b.ResetTimer()
	var got Result
	var stats IncrementalStats
	for i := 0; i < b.N; i++ {
		gen := generations[i%2]
		got, stats = state.Analyze(gen.index, incrementalOptions(&gen.artifacts))
	}
	b.StopTimer()
	if string(incrementalDependencyJSON(b, got)) != expected[(b.N-1)%2] {
		b.Fatal("incremental result differs from full analysis")
	}
	b.ReportMetric(float64(stats.Rechecked), "rechecked/op")
}
