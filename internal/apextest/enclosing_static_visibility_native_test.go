package apextest

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestCapturedEnclosingStaticVisibilityAnonymous(t *testing.T) {
	const fixture = "testdata/native_conformance/enclosing_static_visibility"
	file, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 31 || strings.Join(rows[0], ",") != "case,api,route,outcome" {
		t.Fatalf("incomplete native matrix: %d rows", len(rows))
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		if len(row) != 4 {
			t.Fatalf("invalid native row: %v", row)
		}
		if row[2] != "anonymous" {
			continue
		}
		name, api, outcome := row[0], row[1], row[3]
		key := name + "/" + api
		if seen[key] || (api != "62.0" && api != "67.0") || !strings.HasPrefix(outcome, "value=") {
			t.Fatalf("invalid native observation: %v", row)
		}
		seen[key] = true
		t.Run(key, func(t *testing.T) {
			dir := filepath.Join(fixture, "cases", name, api)
			sources, err := filepath.Glob(filepath.Join(dir, "*.cls"))
			if err != nil || len(sources) == 0 {
				t.Fatalf("missing sources: %v", err)
			}
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			for _, source := range sources {
				body, err := os.ReadFile(source)
				if err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(root, "force-app/main/default/classes", filepath.Base(source))
				writeFile(t, path, string(body))
				writeFile(t, path+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			}
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native accepted project: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			body, err := os.ReadFile(filepath.Join(dir, "anonymous.apex"))
			if err != nil {
				t.Fatal(err)
			}
			program, err := vm.CompileAnonymousWithOptions(string(body), vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			result, err := newConformanceRunner(t, index, conformanceRunnerOptions{Org: &org, LinkProject: true}).execute(program)
			if err != nil {
				t.Fatal(err)
			}
			want := "CAPTURE:" + strings.TrimPrefix(outcome, "value=")
			found := false
			for _, line := range result.Debug {
				if strings.Contains(line, want) {
					found = true
				}
			}
			if !found {
				t.Fatalf("native=%q; local debug=%v", want, result.Debug)
			}
		})
	}
	if len(seen) != 6 {
		t.Fatalf("native executions covered %d cells, want 6", len(seen))
	}
}
