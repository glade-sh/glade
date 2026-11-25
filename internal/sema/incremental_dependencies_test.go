package sema

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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
		if !reflect.DeepEqual(got, want) || string(incrementalDependencyJSON(t, got)) != string(incrementalDependencyJSON(t, want)) {
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

func TestIncrementalIdentifierMentionBoundaries(t *testing.T) {
	for _, tc := range []struct {
		source string
		want   bool
	}{
		{"line", true},
		{"new line()", true},
		{"namespace.line.inner", true},
		{"list<line>", true},
		{"pipeline", false},
		{"lineitem", false},
		{"line_1", false},
		{"line1", false},
		{"pipeline line", true},
		{"line$inner", true},
		{"éline", true},
		{"pipeline café", true},
		{"Kpipeline", true},
		{"1line", true},
		{`pipeline '\\escaped'`, true},
	} {
		if got := containsIdentifierMention(tc.source, strings.ToLower(tc.source), "line"); got != tc.want {
			t.Errorf("containsIdentifierMention(%q, line) = %v, want %v", tc.source, got, tc.want)
		}
	}
}

func TestIncrementalDependencyInvalidation(t *testing.T) {
	root := t.TempDir()
	incrementalSyntheticProject(t, root)
	state := NewIncrementalState()
	analyze := func(export bool) IncrementalStats {
		t.Helper()
		gen := incrementalLoad(t, root)
		opts := incrementalOptions(&gen.artifacts)
		opts.ExportTypes = export
		got, stats := state.Analyze(gen.index, opts)
		want := AnalyzeWithOptions(gen.index, opts)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("incremental result differs from full analysis: %+v", stats)
		}
		return stats
	}
	analyze(true)
	if stats := analyze(false); stats.Incremental || stats.Reason != "options changed" {
		t.Fatalf("export option change: %+v", stats)
	}
	analyze(true)
	generation := incrementalLoad(t, root)
	opts := incrementalOptions(&generation.artifacts)
	opts.SuppressPerformanceDiagnostics = false
	got, stats := state.Analyze(generation.index, opts)
	if stats.Incremental || stats.Reason != "options changed" || !reflect.DeepEqual(got, AnalyzeWithOptions(generation.index, opts)) {
		t.Fatalf("performance diagnostics option change: %+v", stats)
	}
	opts.Diagnostics = false
	if _, stats := state.Analyze(generation.index, opts); stats.Incremental || stats.Reason != "no captured build artifacts" {
		t.Fatalf("disabled diagnostics: %+v", stats)
	}
	opts.Diagnostics = true
	opts.CapturedSource = func(string) (string, bool) { return "", false }
	if _, stats := state.Analyze(generation.index, opts); stats.Incremental || stats.Reason != "no captured build artifacts" {
		t.Fatalf("captured source override: %+v", stats)
	}
	opts.CapturedSource = nil
	opts.BuildArtifacts = nil
	if _, stats := state.Analyze(generation.index, opts); stats.Incremental || stats.Reason != "no captured build artifacts" {
		t.Fatalf("missing build artifacts: %+v", stats)
	}
	analyze(true)
	path := filepath.Join(root, "force-app/main/default/classes/AccountWorker.cls-meta.xml")
	metadata, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(metadata, []byte("\n<!-- identity change -->")...), 0644); err != nil {
		t.Fatal(err)
	}
	if stats := analyze(true); !stats.Incremental || stats.Rechecked == 0 {
		t.Fatalf("metadata change: %+v", stats)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if stats := analyze(true); stats.Incremental && stats.Rechecked == 0 {
		t.Fatalf("removed metadata reused every type: %+v", stats)
	}
	marker := filepath.Join(root, "force-app/main/default/classes/sfdx-project.json")
	if err := os.WriteFile(marker, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	if stats := analyze(true); stats.Incremental || stats.Reason != "signature, member or metadata change" {
		t.Fatalf("marker change: %+v", stats)
	}
	gen := incrementalLoad(t, root)
	run := incrementalRun{artifacts: &typesys.BuildArtifacts{}}
	run.begin(gen.index, gen.index.Objects)
	if run.digestsOK || run.reason != "missing captured source" {
		t.Fatalf("missing source: digestsOK=%v reason=%q", run.digestsOK, run.reason)
	}
	run = incrementalRun{later: &semaLaterHistory{}, ever: map[string]struct{}{"installdate": {}}}
	if run.later.answer("installdate") {
		t.Fatal("later history answered known without a visible source")
	}
	run.finish()
	if !run.fallback {
		t.Fatal("a later unknown answer for a field a type can add did not force a full analysis")
	}
	run = incrementalRun{later: &semaLaterHistory{visible: map[string]int{"installdate": 1}}, ever: map[string]struct{}{"installdate": {}}}
	if !run.later.answer("installdate") || run.later.answer("nosuchfield") {
		t.Fatal("later history answers differ from the recorded sources")
	}
	run.finish()
	if run.fallback {
		t.Fatal("a later unknown answer for a field no type adds forced a full analysis")
	}
}

func TestIncrementalNestedIdentifierDependencies(t *testing.T) {
	root := t.TempDir()
	incrementalWrite(t, root, "sfdx-project.json", `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
	const dir = "force-app/main/default/classes/"
	incrementalWrite(t, root, dir+"Line.cls", `public class Line { public class Nested {} public Integer value() { return 1; } }`)
	incrementalWrite(t, root, dir+"UsesNested.cls", `public class UsesNested { public Object value() { return new lInE.Nested(); } }`)
	incrementalWrite(t, root, dir+"Pipeline.cls", `public class Pipeline { public Integer value() { return 1; } }`)
	state := NewIncrementalState()
	first := incrementalLoad(t, root)
	state.Analyze(first.index, incrementalOptions(&first.artifacts))
	incrementalWrite(t, root, dir+"Line.cls", `public class Line { public class Nested {} public Integer value() { return 2; } }`)
	second := incrementalLoad(t, root)
	got, stats := state.Analyze(second.index, incrementalOptions(&second.artifacts))
	want := AnalyzeWithOptions(second.index, incrementalOptions(&second.artifacts))
	if !reflect.DeepEqual(got, want) || string(incrementalDependencyJSON(t, got)) != string(incrementalDependencyJSON(t, want)) {
		t.Fatalf("incremental result differs from full analysis: %+v", stats)
	}
	if !stats.Incremental || stats.Rechecked != 2 || stats.Reused == 0 {
		t.Fatalf("nested reference edit stats = %+v, want edited type and nested reference rechecked, with lookalike reused", stats)
	}
}

// TestIncrementalSkipMentionsKeepsSubtypes pins the narrowed rule: a body-only
// edit rechecks the edited type and its subtype, reuses the type that only
// names it, and still equals a full analysis.
func TestIncrementalSkipMentionsKeepsSubtypes(t *testing.T) {
	generations := incrementalDependencyGenerations(t)
	state := NewIncrementalState()
	state.SkipMentionRechecks()
	for i, gen := range generations {
		got, stats := state.Analyze(gen.index, incrementalOptions(&gen.artifacts))
		want := AnalyzeWithOptions(gen.index, incrementalOptions(&gen.artifacts))
		if !reflect.DeepEqual(got, want) || string(incrementalDependencyJSON(t, got)) != string(incrementalDependencyJSON(t, want)) {
			t.Fatalf("generation %d: incremental and full results differ", i)
		}
		if i == 1 && (!stats.Incremental || stats.Rechecked != 2) {
			t.Fatalf("literal edit stats = %+v; want the edited type and its subtype", stats)
		}
	}
}
