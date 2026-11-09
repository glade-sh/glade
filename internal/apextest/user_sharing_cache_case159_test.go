package apextest

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
)

func TestDefaultUserReadCase159DiskCache(t *testing.T) {
	t.Setenv("GLADE_HOME", t.TempDir())
	t.Cleanup(EnableDiskCacheForTesting())
	t.Cleanup(InvalidateRuntimeCaches)
	runCase159ProjectVersions(t, "case159_sharing", func(t *testing.T, index typesys.Index) {
		options := Options{SelectedClasses: []string{"Case159SharingTest"}, SelectedMethod: "userVisibility", Parallelism: 1, PerfCounters: true}
		for _, route := range []string{"cold", "disk"} {
			InvalidateRuntimeCaches()
			ResetPerfCounters()
			run := Run(index, options)
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
				t.Fatalf("%s missing native transport: %+v", route, run.Summary())
			}
			message := run.Suites[0].Cases[0].Problem.Message
			for _, part := range []string{`"readable":true`, `"with":1`, `"without":1`, `"inherited":1`} {
				if !strings.Contains(message, part) {
					t.Errorf("%s missing %s: %s", route, part, message)
				}
			}
			if route == "disk" && SnapshotPerfCounters().Phases.DiskCacheHits == 0 {
				t.Fatal("disk route did not restore startup cache")
			}
		}
	})
}
