package apextest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestCapturedStaticFieldTypeCollision(t *testing.T) {
	const fixture = "testdata/conformance/static_field_type_collision"
	rows := readScopeRunnerTSV(t, filepath.Join(fixture, "org.tsv"), []string{"case", "api", "route", "outcome"})
	captured := map[string]string{}
	for _, row := range rows {
		if len(row) != 4 {
			t.Fatalf("invalid native row: %v", row)
		}
		key := row[0] + "/" + row[1] + "/" + row[2]
		if _, ok := captured[key]; ok {
			t.Fatalf("duplicate native row: %s", key)
		}
		captured[key] = row[3]
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, key := range []string{"project/" + api + "/metadata-deploy", "project/" + api + "/istest", "anonymous/" + api + "/anonymous"} {
			if value := captured[key]; value != "passed" && value != "accepted" {
				t.Fatalf("native row %s: %q", key, value)
			}
		}
		if value := captured["reject/"+api+"/anonymous"]; value == "" || value == "passed" {
			t.Fatalf("missing rejection at %s: %q", api, value)
		}
		for _, namespace := range []string{"", "sample"} {
			t.Run(api+"/"+namespace, func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace":"`+namespace+`","sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
				for _, name := range []string{"Parcel", "ParcelCheck"} {
					body, err := os.ReadFile(filepath.Join(fixture, name+".apex"))
					if err != nil {
						t.Fatal(err)
					}
					path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
					writeFile(t, path, string(body))
					writeFile(t, path+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
				}
				index := loadTestIndex(t, root)
				if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
					t.Fatalf("captured project rejected: %v %v", index.Diagnostics, analysis.Diagnostics)
				}
				opts := Options{SelectedClasses: []string{"ParcelCheck"}, SelectedMethod: "verify", NoDiskCache: true, Parallelism: 1}
				run := Run(index, opts)
				if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem != nil {
					t.Fatalf("native test passed; local runner: %+v", run.Summary())
				}
				anon, err := os.ReadFile(filepath.Join(fixture, "anonymous.apex"))
				if err != nil {
					t.Fatal(err)
				}
				program, err := vm.CompileAnonymousWithOptions(string(anon), vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				org := orgFromIndex(index)
				org.APIVersion = api
				if _, err := newConformanceRunner(t, index, conformanceRunnerOptions{Org: &org, LinkProject: true}).execute(program); err != nil {
					t.Fatalf("native anonymous passed: %v", err)
				}
			})
		}
	}
}
