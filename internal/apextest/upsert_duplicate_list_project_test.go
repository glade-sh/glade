package apextest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

// Project and cache twins for the local, uncaptured singleton-list control.
func TestUpsertDuplicateSingletonListShippingProjectRoutes(t *testing.T) {
	t.Cleanup(EnableDiskCacheForTesting())
	t.Cleanup(InvalidateRuntimeCaches)
	const fixture = "testdata/conformance/dml_policies"
	source, err := os.ReadFile(filepath.Join(fixture, "upsert_list_preservation.apex"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := copyLookupFilterValidationProject(t, filepath.Join(fixture, "project"), api)
			writeFile(t, filepath.Join(root, "force-app/main/default/objects/Contact/fields/RouteKey__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>RouteKey__c</fullName><externalId>true</externalId><label>Route Key</label><length>100</length><type>Text</type></CustomField>`)
			class := filepath.Join(root, "force-app/main/default/classes/ListPreservationTest.cls")
			writeFile(t, class, "@IsTest private class ListPreservationTest { @IsTest static void observe() {\n"+string(source)+"\n} }")
			writeFile(t, class+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("compile: %v %v", index.Diagnostics, analysis.Diagnostics)
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
						t.Fatalf("cache route %s: %+v", route, phases)
					}
					run := Run(index, Options{SelectedClasses: []string{"ListPreservationTest"}, Parallelism: 1})
					if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
						t.Fatalf("list preservation: %+v; suites=%+v", summary, run.Suites)
					}
				})
			}
		})
	}
}
