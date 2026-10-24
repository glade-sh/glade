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

func TestStandardParentSOQLAnonymousAndIsTestNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "standard_parent_routes", "StandardParentProbeTest", "62.0", "67.0")
}

func TestStandardParentPartialProjectNamedAnonymousAndTestRunnerGateRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			fixture := filepath.Join("testdata", "conformance", "standard_parent_routes")
			project := filepath.Join(fixture, "project")
			err := filepath.WalkDir(project, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				relative, err := filepath.Rel(project, path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, relative), strings.ReplaceAll(string(data), "62.0", api))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			// Compile the captured observer even when the selected test only uses
			// catalog fields. Runtime field availability is checked separately
			// by the native conformance test with the complete schema.
			gatePath := filepath.Join(root, "force-app/main/default/classes/ParentRouteGateTest.cls")
			writeFile(t, gatePath, "@IsTest private class ParentRouteGateTest { @IsTest static void observe() { if (!Test.isRunningTest()) StandardParentProbe.observe(); System.assertEquals('Parent', new Contact(LastName='Parent').LastName); } }")
			writeFile(t, gatePath+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			index := loadTestIndex(t, root)
			// A local field-only extension need not include fields supplied elsewhere.
			// Keep the lookup before its partial parent to exercise lazy hydration.
			slices.SortStableFunc(index.Objects, func(a, b schema.Object) int {
				if a.Name == "ParentLink__c" && b.Name != "ParentLink__c" {
					return -1
				}
				if b.Name == "ParentLink__c" && a.Name != "ParentLink__c" {
					return 1
				}
				return 0
			})
			for i := range index.Objects {
				if index.Objects[i].Name == "ParentLink__c" {
					index.Objects[i].Partial = false
				}
				if index.Objects[i].Name == "Contact" {
					index.Objects[i].Partial = true
					index.Objects[i].Fields = []schema.Field{{Name: "Marker__c", Type: "Text"}}
				}
			}
			if index.HasErrors() {
				t.Fatal(index.Diagnostics)
			}

			t.Run("named_and_isTest", func(t *testing.T) {
				named := sema.Analyze(index)
				if named.HasErrors() {
					t.Fatalf("named and @IsTest project gate: %v", named.Diagnostics)
				}
			})
			t.Run("anonymous", func(t *testing.T) {
				source := "ParentLink__c row = [SELECT Contact__r.LocalNote__c FROM ParentLink__c LIMIT 1]; Object note = row.Contact__r.LocalNote__c;"
				if anonymous := sema.AnalyzeAnonymous(index, source, api); anonymous.HasErrors() {
					t.Fatalf("anonymous project gate: %v", anonymous.Diagnostics)
				}
			})
			t.Run("test_runner", func(t *testing.T) {
				run := Run(index, Options{SelectedClasses: []string{"ParentRouteGateTest"}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
				summary := run.Summary()
				if summary.Total != 1 || summary.Passed != 1 {
					t.Fatalf("test-runner gate: %+v; suites=%+v", summary, run.Suites)
				}
			})
		})
	}
}

func TestStandardParentSOSLPreservationProjectAnonymousNamedAndTestRunnerRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, leaf := range []string{"LocalNote__c", "LastName", "Marker__c"} {
			t.Run(api+"/"+leaf, func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
				metadata := `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>` + api + `</apiVersion><status>Active</status></ApexClass>`
				body := "List<List<SObject>> rows = [FIND 'Parent' RETURNING ParentLink__c(Contact__r." + leaf + ")];"
				observer := filepath.Join(root, "force-app/main/default/classes/ParentSearchRoute.cls")
				writeFile(t, observer, "public class ParentSearchRoute { public static void observe() { "+body+" } }")
				writeFile(t, observer+"-meta.xml", metadata)
				gate := filepath.Join(root, "force-app/main/default/classes/ParentSearchRouteTest.cls")
				// The runner must check a named dependency even though it is not executed.
				writeFile(t, gate, "@IsTest private class ParentSearchRouteTest { @IsTest static void observe() { if (!Test.isRunningTest()) ParentSearchRoute.observe(); System.assertEquals('Parent', new Contact(LastName='Parent').LastName); } }")
				writeFile(t, gate+"-meta.xml", metadata)
				lookup := filepath.Join(root, "force-app/main/default/objects/ParentLink__c")
				writeFile(t, filepath.Join(lookup, "ParentLink__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Parent Link</label><pluralLabel>Parent Links</pluralLabel><nameField><label>Name</label><type>Text</type></nameField><deploymentStatus>Deployed</deploymentStatus><sharingModel>ReadWrite</sharingModel></CustomObject>`)
				writeFile(t, filepath.Join(lookup, "fields/Contact__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Contact__c</fullName><label>Contact</label><type>Lookup</type><referenceTo>Contact</referenceTo><relationshipName>ParentLinks</relationshipName></CustomField>`)
				writeFile(t, filepath.Join(root, "force-app/main/default/objects/Contact/fields/Marker__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Marker__c</fullName><label>Marker</label><type>Text</type><length>80</length></CustomField>`)
				index := loadTestIndex(t, root)
				if index.HasErrors() {
					t.Fatal(index.Diagnostics)
				}
				// Match the partial-parent hydration order used by the SOQL project twin.
				index.Objects = []schema.Object{
					{Name: "ParentLink__c", Fields: []schema.Field{{Name: "Contact__c", Type: "Lookup", ReferenceTo: []string{"Contact"}, RelationshipName: "Contact__r"}}},
					{Name: "Contact", Partial: true, Fields: []schema.Field{{Name: "Marker__c", Type: "Text"}}},
				}
				reject := leaf == "LocalNote__c"
				check := func(t *testing.T, result sema.Result) {
					t.Helper()
					if reject {
						if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "GLADESEMA_SOSL_FIELD" {
							t.Fatalf("SOSL project gate lost omitted-leaf diagnostic: %v", result.Diagnostics)
						}
					} else if result.HasErrors() {
						t.Fatal(result.Diagnostics)
					}
				}
				t.Run("named_and_isTest", func(t *testing.T) { check(t, sema.Analyze(index)) })
				t.Run("anonymous", func(t *testing.T) { check(t, sema.AnalyzeAnonymous(index, body, api)) })
				t.Run("test_runner", func(t *testing.T) {
					run := Run(index, Options{SelectedClasses: []string{"ParentSearchRouteTest"}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
					summary := run.Summary()
					if reject {
						if summary.Total != 1 || summary.CompileErrors != 1 || summary.Passed != 0 || !strings.Contains(firstRunProblem(run), "No such column 'Contact__r.LocalNote__c' on entity 'ParentLink__c'") {
							t.Fatalf("SOSL dependency must fail compilation before execution: %+v; problem=%s", summary, firstRunProblem(run))
						}
					} else if summary.Total != 1 || summary.Passed != 1 || summary.CompileErrors != 0 {
						t.Fatalf("SOSL known-leaf dependency failed: %+v; problem=%s", summary, firstRunProblem(run))
					}
				})
			})
		}
	}
}
