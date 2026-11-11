package apextest

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
)

// replayCapturedAfterSaveWarm checks both in-memory and persisted runtime
// restores against a native row after the captured compiler gates run.
func replayCapturedAfterSaveWarm(t *testing.T, index typesys.Index, class, name, want string) {
	t.Helper()
	restore := EnableDiskCacheForTesting()
	defer restore()
	defer InvalidateRuntimeCaches()
	defer ResetPerfCounters()
	InvalidateRuntimeCaches()
	opts := Options{SelectedClasses: []string{class}, SelectedMethod: name, Parallelism: 1, PerfCounters: true}
	check := func(label string, run testreport.Run) {
		t.Helper()
		if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
			t.Fatalf("%s case selection: %v", label, run.Summary())
		}
		problem := run.Suites[0].Cases[0].Problem
		if problem == nil || !strings.Contains(problem.Message, "Expected: SENTINEL, Actual: "+want) {
			t.Fatalf("%s result differs from native: %+v", label, problem)
		}
	}
	check("cold", Run(index, opts))
	ResetPerfCounters()
	check("memory", Run(index, opts))
	if SnapshotPerfCounters().Phases.MemoryCacheHits == 0 {
		t.Fatal("second run did not restore the memory cache")
	}
	InvalidateRuntimeCaches()
	ResetPerfCounters()
	check("disk", Run(index, opts))
	if SnapshotPerfCounters().Phases.DiskCacheHits == 0 {
		t.Fatal("third run did not restore the disk cache")
	}
}
