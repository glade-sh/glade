package apextest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestUpsertDuplicateUncapturedProjectRoutesRetainReleaseBehavior(t *testing.T) {
	t.Cleanup(EnableDiskCacheForTesting())
	t.Cleanup(InvalidateRuntimeCaches)
	const fixture = "testdata/conformance/dml_policies"
	source, err := os.ReadFile(filepath.Join(fixture, "upsert_release_controls.apex"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := copyLookupFilterValidationProject(t, filepath.Join(fixture, "project"), api)
			writeFile(t, filepath.Join(root, "force-app/main/default/objects/Contact/fields/RouteKey__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>RouteKey__c</fullName><externalId>true</externalId><label>Route Key</label><length>100</length><type>Text</type></CustomField>`)
			class := filepath.Join(root, "force-app/main/default/classes/UpsertReleaseControlTest.cls")
			var methods strings.Builder
			for _, shape := range []string{"custom_insert", "implicit_id_update", "explicit_id_update", "custom_update"} {
				for _, route := range []string{"keyword", "database_partial", "database_atomic"} {
					body := strings.ReplaceAll(string(source), "new List<String>{'custom_insert','implicit_id_update','explicit_id_update','custom_update'}", "new List<String>{'"+shape+"'}")
					body = strings.ReplaceAll(body, "new List<String>{'keyword','database_partial','database_atomic'}", "new List<String>{'"+route+"'}")
					fmt.Fprintf(&methods, "@IsTest static void %s_%s() {\n%s\n}\n", shape, route, body)
				}
			}
			writeFile(t, class, "@IsTest private class UpsertReleaseControlTest {\n"+methods.String()+"\n}")
			writeFile(t, class+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("compile: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			t.Run("anonymous", func(t *testing.T) {
				program, err := vm.CompileAnonymousWithOptions(string(source), vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				org := orgFromIndex(index)
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				if _, err := runner.execute(program); err != nil {
					t.Fatal(err)
				}
			})
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
					run := Run(index, Options{SelectedClasses: []string{"UpsertReleaseControlTest"}, Parallelism: 1})
					if summary := run.Summary(); summary.Total != 12 || summary.Passed != 12 {
						for _, suite := range run.Suites {
							for _, result := range suite.Cases {
								if result.Status != "pass" {
									t.Errorf("%s: %+v", result.MethodName, result.Problem)
								}
							}
						}
						t.Fatalf("release upsert controls: %+v", summary)
					}
				})
			}
		})
	}
}
