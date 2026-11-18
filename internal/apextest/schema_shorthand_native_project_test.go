package apextest

import (
	"path/filepath"
	"testing"
)

// Native @IsTest outcomes are in ../sema/testdata/schema_shorthand/org.tsv:24-25.
func TestSchemaShorthandNativeProject(t *testing.T) {
	index := loadTestIndex(t, filepath.Join("testdata", "conformance", "schema_shorthand_project", "project"))
	for _, class := range []string{"ShorthandProbe62", "ShorthandProbe67"} {
		t.Run(class, func(t *testing.T) {
			opts := Options{SelectedClasses: []string{class}, SelectedMethod: "run", NoDiskCache: true, Parallelism: 1, PerfCounters: true}
			for _, runName := range []string{"initial", "warm"} {
				ResetPerfCounters()
				run := Run(index, opts)
				if runName == "warm" {
					if phases := SnapshotPerfCounters().Phases; phases.MemoryCacheHits != 1 || phases.CacheMisses != 0 {
						t.Fatalf("warm project cache: %+v", phases)
					}
				}
				if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 || summary.Failed != 0 || summary.Errors != 0 {
					for _, suite := range run.Suites {
						for _, c := range suite.Cases {
							if c.Problem != nil {
								t.Logf("%s: %+v", c.MethodName, *c.Problem)
							}
						}
					}
					t.Fatalf("%s: native test passed; Glade: %+v", runName, summary)
				}
			}
		})
	}
}
