package apextest

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// NATIVE: testdata/conformance/child_semijoin_siblings/org.tsv:2-5.
func TestChildSemiJoinSiblingRoutesNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/child_semijoin_siblings"
	f, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]bool)
	for _, row := range rows[1:] {
		id, api, route := row[0], row[1], row[2]
		key := api + "/" + route
		if seen[key] {
			t.Fatalf("duplicate native row %s", key)
		}
		seen[key] = true
		if id == "seeded" {
			if route != "anonymous" || row[3] != "True" || row[4] != "True" || row[5] != "SEEDED=pass" {
				t.Fatalf("unexpected row: %q", row)
			}
		} else if id == "seeded_test" {
			if route != "istest" || row[3] != "true" || row[4] != "Passed" || row[5] != "run=Pass" {
				t.Fatalf("unexpected row: %q", row)
			}
		} else {
			t.Fatalf("unexpected case %q", id)
		}
		t.Run(key, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			source, err := os.ReadFile(filepath.Join(fixture, "seeded.apex"))
			if err != nil {
				t.Fatal(err)
			}
			if route == "istest" {
				writeFile(t, filepath.Join(root, "force-app/main/default/classes/SeededQueryTest.cls"), "@IsTest private class SeededQueryTest { @IsTest static void run() {\n"+string(source)+"\n} }")
				writeFile(t, filepath.Join(root, "force-app/main/default/classes/SeededQueryTest.cls-meta.xml"), fmt.Sprintf(`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`, api))
			}
			index := loadTestIndex(t, root)
			if route == "istest" {
				run := Run(index, Options{SelectedClasses: []string{"SeededQueryTest"}, NoDiskCache: true, Parallelism: 1})
				if s := run.Summary(); s.Total != 1 || s.Passed != 1 {
					t.Fatalf("result: %+v; %s", s, firstRunProblem(run))
				}
				return
			}
			program, err := vm.CompileAnonymousWithOptions(string(source), vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			if _, err := runner.newMachine().Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
	if len(seen) != 4 {
		t.Fatalf("missing native rows: %v", seen)
	}
}
