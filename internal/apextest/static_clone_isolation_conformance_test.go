package apextest

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/testreport"
)

func TestStaticCloneIsolationNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/static_clone_isolation"
	file, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	file.Close()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 9 {
		t.Fatalf("native rows = %d, want header and eight observations", len(rows))
	}
	var want []diskRuntimeIsolationResult
	for _, row := range rows[1:] {
		if len(row) != 5 || (row[0] != "62.0" && row[0] != "67.0") || row[3] != "pass" || row[4] == "" {
			t.Fatalf("invalid native row: %v", row)
		}
		want = append(want, diskRuntimeIsolationResult{Class: row[1], Method: row[2], Status: testreport.Status(row[3])})
	}
	capture, err := os.ReadFile(filepath.Join(fixture, "RUN-FACTS.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range want {
		if !strings.Contains(string(capture), row.Class+"."+row.Method+": Pass") {
			t.Fatalf("missing native result for %s.%s", row.Class, row.Method)
		}
	}
	root := filepath.Join(t.TempDir(), "project")
	copyDiskRuntimeScopeProject(t, filepath.Join(fixture, "project"), root)
	index := loadTestIndex(t, root)
	restore := EnableDiskCacheForTesting()
	t.Cleanup(restore)
	t.Cleanup(InvalidateRuntimeCaches)
	InvalidateRuntimeCaches()
	cases := Discover(index, Options{})
	if len(cases) != len(want) {
		t.Fatalf("discovered %d tests, want %d", len(cases), len(want))
	}
	for _, workers := range []int{1, 2, 4} {
		for _, mode := range []string{"built", "memory", "disk"} {
			t.Run(fmt.Sprintf("%s/workers-%d", mode, workers), func(t *testing.T) {
				if mode != "memory" {
					InvalidateRuntimeCaches()
				}
				ResetPerfCounters()
				run := RunCasesContext(context.Background(), index, Options{
					ParallelMethods: true, Parallelism: workers,
					RestoredRuntimeMultiWorker: true,
					NoDiskCache:                mode != "disk", PerfCounters: true,
				}, cases)
				got := canonicalDiskRuntimeIsolationResult(run)
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("static isolation differs from native:\n got %#v\nwant %#v; %s", got, want, firstRunProblem(run))
				}
				phases := SnapshotPerfCounters().Phases
				if mode == "disk" && phases.DiskCacheHits != 1 {
					t.Fatalf("disk phase = %#v, want one restored snapshot", phases)
				}
				if mode == "memory" && phases.MemoryCacheHits != 1 {
					t.Fatalf("memory phase = %#v, want one cache hit", phases)
				}
				if mode == "built" {
					key, entry, err := runtimeFromIndexWithSourceDigests(index, nil, newSourceCache(), false)
					if err != nil {
						t.Fatal(err)
					}
					persistDiskRuntimeWithPerf(index, nil, key, entry.restored.CloneOrg(), entry, newRunPerfCounters(true))
				}
			})
		}
	}
	raw, _ := json.Marshal(want)
	t.Logf("duration-free report: %s", raw)
}
