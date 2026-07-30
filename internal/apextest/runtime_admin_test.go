package apextest

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestWarmRuntimeWithBuildArtifactsWarmsSemanticResult(t *testing.T) {
	restoreDisk := EnableDiskCacheForTesting()
	t.Cleanup(restoreDisk)
	t.Cleanup(InvalidateRuntimeCaches)
	InvalidateRuntimeCaches()

	root := t.TempDir()
	path := filepath.Join(root, "SemanticWarmTest.cls")
	writeFile(t, path, `@isTest private class SemanticWarmTest { @isTest static void passes() { System.assertEquals(1, 1); } }`)
	index, artifacts := typesys.BuildWithArtifacts(project.Project{Root: root, ApexFiles: []string{path}}, gladeschema.Schema{})
	if err := WarmRuntimeWithBuildArtifacts(context.Background(), index, &artifacts); err != nil {
		t.Fatal(err)
	}

	run := RunCasesContext(context.Background(), index, Options{BuildArtifacts: &artifacts, PerfCounters: true}, Discover(index, Options{}))
	if summary := run.Summary(); summary.Passed != 1 {
		t.Fatalf("summary = %#v problem=%q", summary, firstRunProblem(run))
	}
	phases := SnapshotPerfCounters().Phases
	if phases.SemanticMemoryCacheHits != 1 || phases.SemanticDiskCacheHits != 0 || phases.SemanticCacheMisses != 0 {
		t.Fatalf("semantic warm counters = %#v", phases)
	}
}

func TestWarmRuntimeWithBuildArtifactsRejectsChangedGeneration(t *testing.T) {
	t.Cleanup(InvalidateRuntimeCaches)
	InvalidateRuntimeCaches()
	root := t.TempDir()
	path := filepath.Join(root, "SemanticWarmMutationTest.cls")
	writeFile(t, path, `public class SemanticWarmMutationTest { public static Integer value() { return 1; } }`)
	index, artifacts := typesys.BuildWithArtifacts(project.Project{Root: root, ApexFiles: []string{path}}, gladeschema.Schema{})
	writeFile(t, path, `public class SemanticWarmMutationTest { public static Integer value() { return 2; } }`)

	err := WarmRuntimeWithBuildArtifacts(context.Background(), index, &artifacts)
	if err == nil || !strings.Contains(err.Error(), "source snapshot mismatch") {
		t.Fatalf("WarmRuntimeWithBuildArtifacts() = %v", err)
	}
	if stats := semanticResults.Stats(); stats.Entries != 0 {
		t.Fatalf("changed generation retained semantic result: %#v", stats)
	}
}
