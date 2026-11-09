package apextest

import (
	"path/filepath"
	"testing"
)

// The native @IsTest controls are recorded in location_describe/org.tsv:14-15.
func TestLocationDescribeNativeProject(t *testing.T) {
	index := loadTestIndex(t, filepath.Join("testdata", "conformance", "location_describe", "project"))
	for _, class := range []string{"LocationProbe62", "LocationProbe67"} {
		t.Run(class, func(t *testing.T) {
			opts := Options{SelectedClasses: []string{class}, SelectedMethod: "capture", NoDiskCache: true, Parallelism: 1, PerfCounters: true}
			Run(index, opts)
			ResetPerfCounters()
			run := Run(index, opts)
			if phases := SnapshotPerfCounters().Phases; phases.MemoryCacheHits != 1 || phases.CacheMisses != 0 {
				t.Fatalf("warm project cache: %+v", phases)
			}
			if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 || summary.Failed != 0 || summary.Errors != 0 {
				for _, suite := range run.Suites {
					for _, c := range suite.Cases {
						if c.Problem != nil {
							t.Logf("%s: %+v", c.MethodName, *c.Problem)
						}
					}
				}
				t.Fatalf("native test passed; Glade: %+v", summary)
			}
		})
	}
}
