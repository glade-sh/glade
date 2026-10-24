package apextest

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
)

func TestStandardParentManagedSOQLAnonymousAndIsTestNativeConformance(t *testing.T) {
	requireStandardParentInstalledEvidence(t)
	runInnerTypeNativeConformance(t, "standard_parent_routes/managed", "ManagedParentRouteProbeTest", "62.0", "67.0")
}

func TestStandardParentManagedPartialProjectAnonymousNamedAndTestRunnerRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			fixture := filepath.Join("testdata", "conformance", "standard_parent_routes", "managed", "project")
			err := filepath.WalkDir(fixture, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				// The project has only its local extension, not the provider field.
				if filepath.Base(path) == "gladens001__ParentRouteValue__c.field-meta.xml" {
					return nil
				}
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				relative, err := filepath.Rel(fixture, path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, relative), strings.ReplaceAll(string(data), "62.0", api))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			gate := filepath.Join(root, "force-app/main/default/classes/ManagedParentRouteGateTest.cls")
			writeFile(t, gate, "@IsTest private class ManagedParentRouteGateTest { @IsTest static void observe() { if (!Test.isRunningTest()) ManagedParentRouteProbe.observe(); System.assertEquals('Parent', new Contact(LastName='Parent').LastName); } }")
			writeFile(t, gate+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			writeFile(t, filepath.Join(root, "force-app/main/default/objects/Contact/fields/Marker__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Marker__c</fullName><label>Marker</label><type>Text</type><length>80</length></CustomField>`)
			index := loadTestIndex(t, root)
			if index.HasErrors() {
				t.Fatal(index.Diagnostics)
			}
			// Omit the namespaced provider field from the local Contact extension.
			// Put the complete project lookup first to pin lazy parent hydration.
			slices.SortStableFunc(index.Objects, func(a, b schema.Object) int {
				if a.Name == "gladens003__ParentRouteTwin__c" && b.Name != a.Name {
					return -1
				}
				if b.Name == "gladens003__ParentRouteTwin__c" && a.Name != b.Name {
					return 1
				}
				return 0
			})
			for i := range index.Objects {
				if index.Objects[i].Name == "gladens003__ParentRouteTwin__c" {
					index.Objects[i].Partial = false
				}
				if index.Objects[i].Name == "Contact" {
					index.Objects[i].Partial = true
					index.Objects[i].Fields = []schema.Field{{Name: "Marker__c", Type: "Text"}}
				}
			}
			t.Run("named_and_isTest", func(t *testing.T) {
				if result := sema.Analyze(index); result.HasErrors() {
					t.Fatalf("managed named and @IsTest project gate: %v", result.Diagnostics)
				}
			})
			t.Run("anonymous", func(t *testing.T) {
				body := "gladens003__ParentRouteTwin__c row = [SELECT gladens003__Contact__r.gladens001__ParentRouteValue__c FROM gladens003__ParentRouteTwin__c LIMIT 1]; Object note = row.gladens003__Contact__r.gladens001__ParentRouteValue__c;"
				if result := sema.AnalyzeAnonymous(index, body, api); result.HasErrors() {
					t.Fatalf("managed anonymous project gate: %v", result.Diagnostics)
				}
			})
			t.Run("test_runner", func(t *testing.T) {
				run := Run(index, Options{SelectedClasses: []string{"ManagedParentRouteGateTest"}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
				if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 || summary.CompileErrors != 0 {
					t.Fatalf("managed dependency-scoped test runner: %+v; problem=%s", summary, firstRunProblem(run))
				}
			})
		})
	}
}
