package apextest

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// Owned native rows exercise Integer.valueOf through both split octets and
// Matcher.group octets, plus the rewritten grapheme delimiter regression.
func TestNumericAddressAndGraphemeNativeProjectAnonymousAndIsTestRoutes(t *testing.T) {
	fixture := filepath.Join("testdata", "ipv4_validation", "numeric_routes")
	raw, err := os.ReadFile(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(strings.NewReader(string(raw)))
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 9 {
		t.Fatalf("native row count=%d want 8 plus header", len(rows)-1)
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		if len(row) != 4 {
			t.Fatal("invalid native row")
		}
		api, context, id := row[0], row[1], row[2]
		method, marker, count := "observe", "NUMERIC_ROWS", 20
		if id == "grapheme-boundary" {
			method, marker, count = "grapheme", "GRAPHEME_ROWS", 2
		} else if id != "numeric-address-routes" {
			t.Fatal("unexpected native row id")
		}
		if (api != "62.0" && api != "67.0") || (context != "anonymous" && context != "isTest") {
			t.Fatal("unexpected native cell")
		}
		label := api + "/" + context + "/" + id
		if seen[label] {
			t.Fatal("duplicate native cell")
		}
		seen[label] = true
		t.Run(label, func(t *testing.T) {
			var want map[string]any
			if err := json.Unmarshal([]byte(row[3]), &want); err != nil {
				t.Fatal(err)
			}
			if len(want) != count {
				t.Fatalf("native observation count=%d want %d", len(want), count)
			}
			root := t.TempDir()
			if err := os.CopyFS(root, os.DirFS(filepath.Join(fixture, "project"))); err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			classes := filepath.Join(root, "force-app/main/default/classes")
			for _, class := range []string{"IntegerSplitAddressProbe", "GroupedAddressProbe", "NumericRouteObservation", "NumericRouteObservationTest"} {
				writeFile(t, filepath.Join(classes, class+".cls-meta.xml"), `<ApexClass><apiVersion>`+api+`</apiVersion></ApexClass>`)
			}
			index := loadTestIndex(t, root)
			var observed string
			if context == "anonymous" {
				program, err := vm.CompileAnonymousWithOptions(`String observation=JSON.serialize(NumericRouteObservation.`+method+`());`, vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				org := orgFromIndex(index)
				org.APIVersion = api
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				machine := runner.newMachine()
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
				observed = machine.Globals["observation"].Text
			} else {
				result := Run(index, Options{SelectedClasses: []string{"NumericRouteObservationTest"}, SelectedMethod: method, NoDiskCache: true, Parallelism: 1})
				summary := result.Summary()
				if summary.Total != 1 || summary.Failed != 1 || len(result.Suites) != 1 || len(result.Suites[0].Cases) != 1 || result.Suites[0].Cases[0].Problem == nil {
					t.Fatalf("missing observation transport: %+v %s", summary, firstRunProblem(result))
				}
				message := result.Suites[0].Cases[0].Problem.Message
				var found bool
				_, observed, found = strings.Cut(message, ", Actual: ")
				if !found || !strings.Contains(message, marker) {
					t.Fatal("unexpected observation transport: ", message)
				}
			}
			var got map[string]any
			if err := json.Unmarshal([]byte(observed), &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("native row differs: got %s want %s", observed, row[3])
			}
		})
	}
	if len(seen) != 8 {
		t.Fatalf("native matrix cells=%d", len(seen))
	}
}
