package apextest

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// NATIVE: testdata/conformance/absent_org_wide_email/org.tsv.
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
	header := []string{"id", "api", "route", "compiled", "success", "outcome", "class_api", "trigger_api", "residue"}
	if len(rows) != 9 || !reflect.DeepEqual(rows[0], header) {
		t.Fatalf("invalid table: %q", rows)
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		id, api, route := row[0], row[1], row[2]
		key := id + "/" + api + "/" + route
		if seen[key] {
			t.Fatalf("duplicate row %s", key)
		}
		seen[key] = true
		if (id != "direct" && id != "lookup") || (api != "62.0" && api != "67.0") || (route != "anonymous" && route != "istest") {
			t.Fatalf("unexpected row: %q", row)
		}
		t.Run(key, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			actual := []string{id, api, route, "true", "", "", "n/a", "n/a", "n/a"}
			if route == "anonymous" {
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
				result, err := runner.newMachine().Execute(program)
				if err != nil {
					t.Fatal(err)
				}
				actual[4] = "true"
				if len(result.Debug) != 2 {
					t.Fatalf("debug = %q", result.Debug)
				}
				pos := 0
				if id == "lookup" {
					pos = 1
				}
				prefix := fmt.Sprintf("ROAD548_CASE_%d=", pos)
				var ok bool
				actual[5], ok = strings.CutPrefix(result.Debug[pos], prefix)
				if !ok {
					t.Fatalf("debug = %q", result.Debug)
				}
			} else {
				source, err := os.ReadFile(filepath.Join(fixture, "force-app/main/default/classes/Road548Probe.cls"))
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(root, "force-app/main/default/classes/Road548Probe.cls"), string(source))
				writeFile(t, filepath.Join(root, "force-app/main/default/classes/Road548Probe.cls-meta.xml"), fmt.Sprintf(`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`, api))
				run := Run(loadTestIndex(t, root), Options{SelectedClasses: []string{"Road548Probe"}, NoDiskCache: true, Parallelism: 1})
				count := 0
				for _, suite := range run.Suites {
					for _, c := range suite.Cases {
						if strings.EqualFold(c.MethodName, id) {
							count++
							actual[4] = string(c.Status)
							if actual[4] == "fail" {
								actual[4] = "Failed"
							}
							if c.Problem == nil {
								t.Fatalf("missing assertion: %+v", c)
							}
							if c.Problem.Type != "System.AssertException" {
								t.Fatalf("unexpected assertion type: %q", c.Problem.Type)
							}
							const prefix = "Assertion Failed: ROAD548="
							var ok bool
							actual[5], ok = strings.CutPrefix(c.Problem.Message, prefix)
							if !ok {
								t.Fatalf("unexpected assertion message: %q", c.Problem.Message)
							}
						}
					}
				}
				if count != 1 {
					t.Fatalf("found %d cases for %s: %+v", count, id, run)
				}
				actual[6] = api
			}
			if !reflect.DeepEqual(actual, row) {
				t.Fatalf("got  %q\nwant %q", actual, row)
			}
		})
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "istest"} {
			for _, id := range []string{"direct", "lookup"} {
				key := id + "/" + api + "/" + route
				if !seen[key] {
					t.Fatalf("missing row %s", key)
				}
			}
		}
	}
}
