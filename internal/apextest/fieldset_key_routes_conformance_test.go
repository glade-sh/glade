package apextest

import (
	"path/filepath"
	"testing"
)

// Native named-route observations: pr163_fieldset_keys/org.tsv, IsTest rows.
func TestFieldSetKeyNamedRoutesNativeConformance(t *testing.T) {
	for _, kind := range []string{"managed", "unmanaged"} {
		t.Run(kind, func(t *testing.T) {
			root := filepath.Join("testdata", "conformance", "pr163_fieldset_keys", "istest", kind)
			index := loadTestIndex(t, root)
			opts := Options{NoDiskCache: true, Parallelism: 1, PerfCounters: true}
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
				if got := run.Summary(); got.Total != 2 || got.Passed != 2 {
					t.Fatalf("project run %d: summary=%#v problem=%q", pass, got, firstRunProblem(run))
				}
			}
		})
	}
}
