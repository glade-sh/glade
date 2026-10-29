package apextest

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// Missing source metadata does not describe an empty org. These are synthetic
// compatibility cases, not additional Salesforce observations. The declared
// shape control is standard_recordtype_field_policy/org.tsv:2-15, with its
// ORG-RUN.txt; metadata-enabled controls are :16-25. Contact and Case absence
// is captured in standard_describe_profiles/P0/historical-extra-fields.tsv.
func TestStandardFieldIncompleteProjectRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, shape := range []struct {
			name, path, content string
			present             bool
		}{
			{name: "omitted", present: true},
			{name: "unrelated-config", path: "glade.yml", content: "test:\n  permissionSets: []\n", present: true},
			{name: "malformed-scratch", path: "config/project-scratch-def.json", content: "{", present: true},
			{name: "declared-scratch", path: "config/project-scratch-def.json", content: `{"edition":"Developer"}`},
			{name: "declared-empty-features", path: "glade.yml", content: "org:\n  features: []\n"},
		} {
			t.Run(api+"/"+shape.name, func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
				if shape.path != "" {
					writeFile(t, filepath.Join(root, shape.path), shape.content)
				}
				var body string
				for i, object := range []string{"Account", "Contact", "Case"} {
					body += fmt.Sprintf("%s row%d = new %s(RecordTypeId=null); row%d.RecordTypeId='012000000000001AAA'; System.assertEquals('012000000000001AAA', String.valueOf(row%d.RecordTypeId));", object, i, object, i, i)
				}
				class := "@IsTest private class IncompleteShapeTest { @IsTest static void sample(){" + body + "} }"
				path := filepath.Join(root, "force-app/main/default/classes/IncompleteShapeTest.cls")
				writeFile(t, path, class)
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
				index := loadTestIndex(t, root)
				for _, route := range []string{"named-is-test", "anonymous-cold", "anonymous-warm"} {
					result := sema.Analyze(index)
					if route != "named-is-test" {
						result = sema.AnalyzeAnonymous(index, body, api)
					}
					if got := !index.HasErrors() && !result.HasErrors(); got != shape.present {
						t.Errorf("%s: compiled=%t, want %t: %v %v", route, got, shape.present, index.Diagnostics, result.Diagnostics)
					}
				}
				if t.Failed() {
					return
				}
				if !shape.present {
					return
				}
				org := orgFromIndex(index)
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				for _, route := range []string{"anonymous-cold", "anonymous-warm"} {
					if _, err := runner.execute(program); err != nil {
						t.Fatalf("%s: %v", route, err)
					}
				}
				run := Run(index, Options{SelectedClasses: []string{"IncompleteShapeTest"}, SelectedMethod: "sample", NoDiskCache: true, Parallelism: 1})
				if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem != nil {
					t.Fatalf("project runner: %+v", run)
				}
			})
		}
	}
}
