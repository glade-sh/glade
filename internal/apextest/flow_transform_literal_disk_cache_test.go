package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Capture-backed string literals, including an empty literal, survive a disk runtime round trip.
func TestFlowTransformStringLiteralDiskRuntime(t *testing.T) {
	t.Setenv("GLADE_HOME", t.TempDir())
	t.Cleanup(EnableDiskCacheForTesting())
	t.Cleanup(InvalidateRuntimeCaches)
	t.Cleanup(ResetPerfCounters)
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			files := make(map[string]string)
			for _, name := range []string{"TransformLiteral", "TransformEmpty"} {
				path := filepath.Join("testdata/conformance/flow_transform_literal", "api"+strings.TrimSuffix(api, ".0"), name+".flow-meta.xml")
				flow, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				files["force-app/main/default/flows/"+name+".flow-meta.xml"] = string(flow)
			}
			classes := map[string]string{"TransformLiteralDiskProbeTest": `@IsTest private class TransformLiteralDiskProbeTest {
				@IsTest static void observe() {
					for (String name : new List<String>{'TransformLiteral','TransformEmpty'}) {
						Flow.Interview f = Flow.Interview.createInterview(name,new Map<String,Object>{'InputText'=>'Sample'});
						f.start(); Contact c=(Contact)f.getVariableValue('OutputRecord');
						System.assertEquals('Sample',c.LastName);
						System.assertEquals(name=='TransformLiteral' ? 'Mapped' : '',c.Title);
						System.assertEquals(true,c.getPopulatedFieldsAsMap().containsKey('Title'));
					}
				}
			}`}
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
					run := Run(index, Options{SelectedClasses: []string{"TransformLiteralDiskProbeTest"}, SelectedMethod: "observe", Parallelism: 1, PerfCounters: true})
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
