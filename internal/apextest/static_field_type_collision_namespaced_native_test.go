package apextest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

func TestCapturedNamespacedStaticFieldTypeCollision(t *testing.T) {
	const fixture = "testdata/conformance/static_field_type_collision_namespaced"
	project, err := os.ReadFile(filepath.Join(fixture, "project.json"))
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		Namespace string `json:"namespace"`
	}
	if err := json.Unmarshal(project, &config); err != nil || config.Namespace == "" {
		t.Fatalf("invalid namespace config: %v", err)
	}
	rows := readScopeRunnerTSV(t, filepath.Join(fixture, "org.tsv"), []string{"case", "api", "route", "outcome"})
	if len(rows) != 4 {
		t.Fatalf("expected four native rows, got %d", len(rows))
	}
	for _, row := range rows {
		if len(row) != 4 || (row[1] != "62.0" && row[1] != "67.0") {
			t.Fatalf("invalid native row: %v", row)
		}
		t.Run(row[0]+"/"+row[1], func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(fixture, "cases", row[0]+".json"))
			if err != nil {
				t.Fatal(err)
			}
			var classes map[string]string
			if err := json.Unmarshal(data, &classes); err != nil {
				t.Fatal(err)
			}
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace":"`+config.Namespace+`","sourceApiVersion":"`+row[1]+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			testClass := ""
			for name, source := range classes {
				path := filepath.Join(root, "force-app/main/default/classes", name)
				writeFile(t, path, source)
				writeFile(t, path+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+row[1]+`</apiVersion><status>Active</status></ApexClass>`)
				if strings.Contains(source, "@IsTest") {
					testClass = strings.TrimSuffix(name, ".cls")
				}
			}
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			rejected := index.HasErrors() || analysis.HasErrors()
			if row[0] == "missing_method" {
				if row[2] != "metadata-validation" || !strings.Contains(row[3], "Method does not exist") || !rejected {
					t.Fatalf("native rejection %q, local diagnostics %v %v", row[3], index.Diagnostics, analysis.Diagnostics)
				}
				return
			}
			if row[0] != "field_dispatch" || row[2] != "metadata-validation+test" || row[3] != "passed" || rejected || testClass == "" {
				t.Fatalf("native acceptance %v, local diagnostics %v %v", row, index.Diagnostics, analysis.Diagnostics)
			}
			run := Run(index, Options{SelectedClasses: []string{testClass}, SelectedMethod: "verify", NoDiskCache: true, Parallelism: 1})
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem != nil {
				t.Fatalf("native test passed; local runner: %+v", run.Summary())
			}
		})
	}
}
