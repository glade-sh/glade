package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunHttpHeaderIdNativeProject(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	for _, name := range []string{"Pr163ApexCapture62", "Pr163ApexCapture67"} {
		for _, suffix := range []string{".cls", ".cls-meta.xml"} {
			data, err := os.ReadFile(filepath.Join("testdata/conformance/pr163_apex_basics/project/classes", name+suffix))
			if err != nil {
				t.Fatal(err)
			}
			if suffix == ".cls" {
				var method string
				for _, line := range strings.Split(string(data), "\n") {
					if strings.Contains(line, "static void idHeader()") {
						method = line
					}
				}
				if method == "" {
					t.Fatal("captured header method missing")
				}
				data = []byte("@IsTest private class " + name + " {\n" + method + "\n}\n")
			}
			writeFile(t, filepath.Join(root, "force-app/main/default/classes", name+suffix), string(data))
		}
	}
	run := Run(loadTestIndex(t, root), Options{SelectedClasses: []string{"Pr163ApexCapture62", "Pr163ApexCapture67"}, SelectedMethod: "idHeader", NoDiskCache: true, Parallelism: 1})
	if summary := run.Summary(); summary.Total != 2 || summary.Passed != 2 {
		t.Fatalf("summary=%#v, problem=%s", summary, firstRunProblem(run))
	}
	index := loadTestIndex(t, root)
	restore := EnableDiskCacheForTesting()
	defer restore()
	InvalidateRuntimeCaches()
	for pass, route := range []string{"cold", "memory", "disk"} {
		t.Run(route, func(t *testing.T) {
			if pass == 2 {
				InvalidateRuntimeCaches()
			}
			ResetPerfCounters()
			result := Run(index, Options{SelectedClasses: []string{"Pr163ApexCapture62", "Pr163ApexCapture67"}, SelectedMethod: "idHeader", Parallelism: 1, PerfCounters: true})
			if summary := result.Summary(); summary.Total != 2 || summary.Passed != 2 {
				t.Fatalf("summary=%#v, problem=%s", summary, firstRunProblem(result))
			}
			phases := SnapshotPerfCounters().Phases
			if pass == 1 && phases.MemoryCacheHits != 1 {
				t.Fatal("warm runner missed memory cache")
			}
			if pass == 2 && phases.DiskCacheHits != 1 {
				t.Fatal("restored runner missed disk cache")
			}
		})
	}
}
