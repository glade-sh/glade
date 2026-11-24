package apextest

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// NATIVE: testdata/conformance/absent_org_wide_email/org.tsv:2-7.
func TestAbsentOrgWideEmailNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/absent_org_wide_email"
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
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		id, api, route := row[0], row[1], row[2]
		key := id + "/" + api
		if seen[key] {
			t.Fatalf("duplicate native row %s", key)
		}
		seen[key] = true
		t.Run(key, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			if route == "anonymous" {
				if id != "missing" || row[3] != "True" || row[4] != "True" || row[5] != "ok" || row[8] != "0" {
					t.Fatalf("unexpected native row: %q", row)
				}
				source, err := os.ReadFile(filepath.Join(fixture, "missing.apex"))
				if err != nil {
					t.Fatal(err)
				}
				index := loadTestIndex(t, root)
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
				return
			}
			if route != "istest" || row[3] != "true" {
				t.Fatalf("unexpected native row: %q", row)
			}
			class := "AbsentAddressProbe"
			if id == "assertion_test" {
				class = "AbsentAddressAssertion"
				if row[4] != "Failed" || !strings.Contains(row[5], "missing address ID: Same value: null") {
					t.Fatalf("unexpected native row: %q", row)
				}
			} else if id != "missing_test" || row[4] != "Passed" || row[5] != "probe=Pass" {
				t.Fatalf("unexpected native row: %q", row)
			}
			source, err := os.ReadFile(filepath.Join(fixture, "force-app/main/default/classes", class+".cls"))
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, "force-app/main/default/classes", class+".cls"), string(source))
			writeFile(t, filepath.Join(root, "force-app/main/default/classes", class+".cls-meta.xml"), fmt.Sprintf(`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`, api))
			run := Run(loadTestIndex(t, root), Options{SelectedClasses: []string{class}, NoDiskCache: true, Parallelism: 1})
			summary := run.Summary()
			if id == "assertion_test" {
				if summary.Total != 1 || summary.Failed != 1 || !strings.Contains(firstRunProblem(run), "missing address ID: Same value: null") {
					t.Fatalf("result: %+v; %s", summary, firstRunProblem(run))
				}
			} else if summary.Total != 1 || summary.Passed != 1 {
				t.Fatalf("result: %+v; %s", summary, firstRunProblem(run))
			}
		})
	}
	for _, id := range []string{"missing", "missing_test", "assertion_test"} {
		for _, api := range []string{"62.0", "65.0"} {
			if !seen[id+"/"+api] {
				t.Fatalf("missing native row %s/%s", id, api)
			}
		}
	}
}
