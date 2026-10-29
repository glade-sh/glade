package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The same eleven native subflow methods must survive runtime persistence and
// repeated use of the cached metadata, without changing their observations.
func TestFlowSubflowCachedProjectRoutes(t *testing.T) {
	t.Cleanup(EnableDiskCacheForTesting())
	t.Cleanup(InvalidateRuntimeCaches)
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			fixture := filepath.Join("testdata/conformance/flow_subflow_route_twins", "api"+api[:2])
			root := t.TempDir()
			copyDiskRuntimeScopeProject(t, fixture, root)
			index := loadTestIndex(t, root)
			raw, err := os.ReadFile(filepath.Join(fixture, "native/org.tsv"))
			if err != nil {
				t.Fatal(err)
			}
			rows := strings.Split(strings.TrimSpace(string(raw)), "\n")[1:]
			if len(rows) != 11 {
				t.Fatalf("native methods=%d; want 11", len(rows))
			}
			for _, route := range []string{"cold", "disk_restore", "memory_warm"} {
				t.Run(route, func(t *testing.T) {
					if route != "memory_warm" {
						InvalidateRuntimeCaches()
					}
					counters := newRunPerfCounters(true)
					if _, _, err := runtimeFromIndexWithSourceDigestsAndPerf(index, nil, newSourceCache(), true, counters); err != nil {
						t.Fatal(err)
					}
					phases := snapshotPerfCounters(counters).Phases
					if (phases.DiskCacheHits == 1) != (route == "disk_restore") || (phases.MemoryCacheHits == 1) != (route == "memory_warm") {
						t.Fatalf("cache route %s was not exercised: %+v", route, phases)
					}
					run := Run(index, Options{SelectedClasses: []string{"CedarRouteTest"}, Parallelism: 1})
					if run.Summary().Total != len(rows) || run.Summary().Passed != len(rows) || len(run.Suites) != 1 {
						t.Fatalf("native subflow runner: %+v %+v", run.Summary(), run.Suites)
					}
					for _, row := range rows {
						method, outcome, ok := strings.Cut(row, "\t")
						found := false
						for _, result := range run.Suites[0].Cases {
							found = found || result.MethodName == method && result.Status == "pass"
						}
						if !ok || outcome != "Pass" || !found {
							t.Fatalf("native row did not pass: %q", row)
						}
					}
				})
			}
		})
	}
}
