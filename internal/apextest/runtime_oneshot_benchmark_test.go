package apextest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func BenchmarkOneShotRuntimeStartup(b *testing.B) {
	index := standardRuntimeBaseIndex(b, "67.0", "StartupRow", 128)
	path := filepath.Join(index.Project.Root, "force-app/main/default/classes/StartupProbeTest.cls")
	if err := os.WriteFile(path, []byte(`@IsTest private class StartupProbeTest { @IsTest static void observe() { System.assertEquals(0, StartupRow.calls); } }`), 0600); err != nil {
		b.Fatal(err)
	}
	p, err := project.Load(index.Project.Root)
	if err != nil {
		b.Fatal(err)
	}
	sch, err := schema.LoadProject(p)
	if err != nil {
		b.Fatal(err)
	}
	index, artifacts := typesys.BuildWithArtifacts(p, sch)
	options := Options{NoDiskCache: true, Parallelism: 1, OneShotProcess: true, SourceDigests: artifacts.SourceDigests}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		InvalidateRuntimeCaches()
		summary := Run(index, options).Summary()
		if summary.Total != 1 || summary.Passed != 1 {
			b.Fatalf("startup workload: %#v", summary)
		}
	}
}
