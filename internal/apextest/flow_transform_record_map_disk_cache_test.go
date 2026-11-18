package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The captured Transform result must survive a disk-runtime round trip.
func TestFlowTransformRecordMapDiskRuntime(t *testing.T) {
	t.Setenv("GLADE_HOME", t.TempDir())
	t.Cleanup(EnableDiskCacheForTesting())
	t.Cleanup(InvalidateRuntimeCaches)
	t.Cleanup(ResetPerfCounters)
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			path := filepath.Join("testdata/native-capture/flow_transform_record_map", "api"+strings.TrimSuffix(api, ".0"), "TransformProbe.flow-meta.xml")
			flow, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			classes := map[string]string{"TransformDiskProbeTest": `@IsTest private class TransformDiskProbeTest {
				@IsTest static void observe() {
					Flow.Interview f = Flow.Interview.createInterview('TransformProbe', new Map<String,Object>{'InputText' => 'Map sample'});
					f.start(); Contact c = (Contact)f.getVariableValue('OutputRecord');
					System.assertEquals('Map sample', c.LastName);
					System.assertEquals(null, c.Title);
					System.assertEquals(true, c.getPopulatedFieldsAsMap().containsKey('Title'));
				}
			}`}
			files := map[string]string{"force-app/main/default/flows/TransformProbe.flow-meta.xml": string(flow)}
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
					run := Run(index, Options{SelectedClasses: []string{"TransformDiskProbeTest"}, SelectedMethod: "observe", Parallelism: 1, PerfCounters: true})
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
