package apextest

import (
	"path/filepath"
	"testing"
)

func TestDescribeDefaultValueNativeConformance(t *testing.T) {
	index := loadTestIndex(t, filepath.Join("testdata", "conformance", "describe_default_value", "project"))
	for _, class := range []string{"DefaultProbeTest62", "DefaultProbeTest67"} {
		t.Run(class, func(t *testing.T) {
			opts := Options{SelectedClasses: []string{class}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1, PerfCounters: true}
			for pass := 0; pass < 2; pass++ {
				if pass == 1 {
					ResetPerfCounters()
				}
				run := Run(index, opts)
				if pass == 1 {
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
					t.Fatalf("native test passed; Glade: %+v", summary)
				}
			}
		})
	}
}
