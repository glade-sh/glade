package apextest

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// Salesforce's results are in org.tsv. The named missing-type case also pins
// the current open-schema behavior, which differs from the native rejection.
func TestMissingCustomObjectTypeNativeConformance(t *testing.T) {
	const dir = "testdata/conformance/missing_custom_object_type"
	file, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 23 {
		t.Fatalf("native rows=%d, error=%v", len(rows), err)
	}
	cases := map[string]string{
		"missing_object":   "missing.apex",
		"missing_generic":  "missing_generic.apex",
		"standard_control": "standard.apex",
		"missing_test":     "force-app/main/default/classes/AbsentObjectTypeTest.cls",
		"present_custom":   "present.apex",
		"present_test":     "force-app/main/default/classes/PresentObjectControlTest.cls",
	}
	for _, row := range rows[1:] {
		if len(row) != 9 {
			t.Fatalf("invalid native row: %v", row)
		}
		id, api, route, compiled, outcome := row[0], row[1], row[2], row[3], row[5]
		path, ok := cases[id]
		if !ok {
			t.Fatalf("unknown native case %q", id)
		}
		t.Run(api+"/"+id, func(t *testing.T) {
			source, err := os.ReadFile(filepath.Join(dir, path))
			if err != nil {
				t.Fatal(err)
			}
			if route == "istest-checkonly" {
				if compiled != "False" {
					t.Fatalf("named row is not a compile rejection: %v", row)
				}
				index := namespaceFamilyProject(t, "", api, map[string]string{"AbsentObjectTypeTest": string(source)})
				analysis := sema.Analyze(index)
				got := namespaceFamilyFileErrors(analysis.Diagnostics, "AbsentObjectTypeTest")
				// An empty source schema is not proof that the org lacks this
				// object. Pin the existing open-schema behavior separately from
				// the native row, pending a way to establish schema completeness.
				if outcome != "Invalid type: AbsentBuoy__c" || len(got) != 0 {
					t.Errorf("named diagnostics=%q, native=%q", got, outcome)
				}
				run := Run(index, Options{SelectedClasses: []string{"AbsentObjectTypeTest"}, SelectedMethod: "typeIsMissing", NoDiskCache: true, Parallelism: 1})
				if run.Summary().Passed != 1 || run.Summary().CompileErrors != 0 {
					t.Errorf("open-schema runner summary=%+v, problem=%s", run.Summary(), firstRunProblem(run))
				}
				return
			}
			if id == "present_custom" || id == "present_test" {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
				objectPath := "force-app/main/default/objects/PresentBuoy__c/PresentBuoy__c.object-meta.xml"
				object, err := os.ReadFile(filepath.Join(dir, objectPath))
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(root, objectPath), string(object))
				if id == "present_test" {
					writeFile(t, filepath.Join(root, path), string(source))
					meta := `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>` + api + `</apiVersion><status>Active</status></ApexClass>`
					writeFile(t, filepath.Join(root, path+"-meta.xml"), meta)
					index := loadTestIndex(t, root)
					run := Run(index, Options{SelectedClasses: []string{"PresentObjectControlTest"}, SelectedMethod: "presentTypesCompile", NoDiskCache: true, Parallelism: 1})
					if compiled != "true" || outcome != "presentTypesCompile=Pass" || run.Summary().Passed != 1 {
						t.Fatalf("native=%v, runner summary=%+v, problem=%s", row, run.Summary(), firstRunProblem(run))
					}
					return
				}
				index := loadTestIndex(t, root)
				diagnostics := subscriberErrors(sema.AnalyzeAnonymous(index, string(source), api).Diagnostics)
				if compiled != "True" || outcome != "ok" || len(diagnostics) != 0 {
					t.Fatalf("native=%v, anonymous diagnostics=%q", row, diagnostics)
				}
				program, err := vm.CompileAnonymousWithOptions(string(source), vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				org := orgFromIndex(index)
				org.APIVersion = api
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				if _, err := runner.execute(program); err != nil {
					t.Fatal(err)
				}
				return
			}
			if route != "anonymous" {
				t.Fatalf("unknown native route %q", route)
			}
			index := namespaceFamilyProject(t, "", api, nil)
			diagnostics := subscriberErrors(sema.AnalyzeAnonymous(index, string(source), api).Diagnostics)
			if compiled == "False" {
				if !slices.Contains(diagnostics, outcome) {
					t.Fatalf("anonymous diagnostics=%q, native=%q", diagnostics, outcome)
				}
				return
			}
			if compiled != "True" || outcome != "ok" || len(diagnostics) != 0 {
				t.Fatalf("native=%v, anonymous diagnostics=%q", row, diagnostics)
			}
			program, err := vm.CompileAnonymousWithOptions(string(source), vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			if _, err := runner.execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
