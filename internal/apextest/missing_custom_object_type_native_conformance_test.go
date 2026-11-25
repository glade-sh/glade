package apextest

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// Salesforce's direct, generic and named-class results are recorded in
// testdata/conformance/missing_custom_object_type/org.tsv.
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
	if err != nil || len(rows) != 13 {
		t.Fatalf("native rows=%d, error=%v", len(rows), err)
	}
	cases := map[string]string{
		"missing_object":   "missing.apex",
		"missing_generic":  "missing_generic.apex",
		"standard_control": "standard.apex",
		"missing_test":     "force-app/main/default/classes/AbsentObjectTypeTest.cls",
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
				got := namespaceFamilyFileErrors(sema.Analyze(index).Diagnostics, "AbsentObjectTypeTest")
				if !slices.Contains(got, outcome) {
					t.Fatalf("named diagnostic=%q, native=%q", got, outcome)
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
