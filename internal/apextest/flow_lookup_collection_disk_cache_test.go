package apextest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The captured collection filters retain their behavior after a disk-runtime restore.
func TestFlowLookupCollectionDiskRuntime(t *testing.T) {
	t.Setenv("GLADE_HOME", t.TempDir())
	t.Cleanup(EnableDiskCacheForTesting())
	t.Cleanup(InvalidateRuntimeCaches)
	t.Cleanup(ResetPerfCounters)
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			files := make(map[string]string)
			var body strings.Builder
			body.WriteString(`Account a=new Account(Name='Capture Collection Seed A'); Account b=new Account(Name='Capture Collection Seed B'); insert new List<Account>{a,b};`)
			for _, op := range []string{"In", "NotIn"} {
				name := "Capture_Filter_" + op
				path := filepath.Join("testdata/conformance/pr163_flow_lookup_collection", "api"+strings.TrimSuffix(api, ".0"), name+".flow-meta.xml")
				flow, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				files["force-app/main/default/flows/"+name+".flow-meta.xml"] = string(flow)
				for _, mode := range []string{"populated", "empty", "unassigned"} {
					input := "new Map<String,Object>()"
					if mode == "populated" {
						input = "new Map<String,Object>{'ids'=>new List<String>{String.valueOf(b.Id)}}"
					} else if mode == "empty" {
						input = "new Map<String,Object>{'ids'=>new List<String>()}"
					}
					want := "null"
					if op == "In" && mode == "populated" {
						want = "'Capture Collection Seed B'"
					} else if op == "NotIn" {
						want = "'Capture Collection Seed A'"
					}
					variable := "flow" + op + mode
					fmt.Fprintf(&body, "Flow.Interview %s=Flow.Interview.createInterview('%s',%s); %s.start(); System.assertEquals(%s,%s.getVariableValue('outcome'));", variable, name, input, variable, want, variable)
				}
			}
			classes := map[string]string{"LookupCollectionDiskProbeTest": "@IsTest private class LookupCollectionDiskProbeTest { @IsTest static void observe(){" + body.String() + "} }"}
			index := flowCoreIndex(t, api, classes, files)
			for _, route := range []string{"cold", "disk"} {
				t.Run(route, func(t *testing.T) {
					InvalidateRuntimeCaches()
					ResetPerfCounters()
					if route == "disk" {
						if _, ok := tryLoadDiskRuntime(index); !ok {
							t.Fatal("runtime was not restored from disk")
						}
					}
					run := Run(index, Options{SelectedClasses: []string{"LookupCollectionDiskProbeTest"}, SelectedMethod: "observe", Parallelism: 1, PerfCounters: true})
					if s := run.Summary(); s.Total != 1 || s.Passed != 1 {
						t.Fatalf("summary=%+v problem=%s", s, firstRunProblem(run))
					}
					if route == "disk" {
						if phases := SnapshotPerfCounters().Phases; phases.DiskCacheHits != 1 || phases.CacheMisses != 0 {
							t.Fatalf("warm runtime phases=%#v", phases)
						}
					}
				})
			}
		})
	}
}
