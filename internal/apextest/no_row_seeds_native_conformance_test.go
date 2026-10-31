package apextest

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestNoRowSeedsNativeConformance(t *testing.T) {
	t.Cleanup(InvalidateRuntimeCaches)
	const fixture = "testdata/conformance/no_row_seeds"
	f, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 5 || strings.Join(rows[0], ",") != "source_api,route,observation" {
		t.Fatalf("invalid capture: %v", err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			err := filepath.WalkDir(filepath.Join(fixture, "project"), func(path string, entry os.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				b, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				name, err := filepath.Rel(filepath.Join(fixture, "project"), path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, name), strings.ReplaceAll(string(b), "62.0", api))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			if a := sema.Analyze(index); index.HasErrors() || a.HasErrors() {
				t.Fatalf("diagnostics: %v %v", index.Diagnostics, a.Diagnostics)
			}
			seen := map[string]bool{}
			for _, row := range rows[1:] {
				if len(row) != 3 {
					t.Fatal("bad row")
				}
				if row[0] != api {
					continue
				}
				route := row[1]
				if seen[route] {
					t.Fatal("duplicate route")
				}
				seen[route] = true
				t.Run(route, func(t *testing.T) {
					var message string
					if route == "isTest" {
						report := Run(index, Options{SelectedClasses: []string{"NoRowSeedsTest"}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
						if report.Summary().Total != 1 || len(report.Suites) != 1 || len(report.Suites[0].Cases) != 1 {
							t.Fatalf("selection: %+v", report.Summary())
						}
						result := report.Suites[0].Cases[0]
						if result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
							t.Fatalf("missing observation: %+v", result)
						}
						message = result.Problem.Message
					} else if route == "anonymous" {
						source := "System.assert(false, 'OBSERVED|' + JSON.serialize(NoRowSeedsProbe.observe()));"
						if a := sema.AnalyzeAnonymous(index, source, api); a.HasErrors() {
							t.Fatal(a.Diagnostics)
						}
						program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						org := orgFromIndex(index)
						runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
						_, err = runner.newMachine().Execute(program)
						var re *vm.RuntimeError
						if !errors.As(err, &re) || re.Type != "System.AssertException" {
							t.Fatalf("missing observation: %v", err)
						}
						message = re.ExceptionMessage()
					} else {
						t.Fatal("unknown route", route)
					}
					_, payload, ok := strings.Cut(message, "OBSERVED|")
					var got, want map[string]any
					if !ok || json.Unmarshal([]byte(payload), &got) != nil || json.Unmarshal([]byte(row[2]), &want) != nil {
						t.Fatalf("invalid observation: %s", message)
					}
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("native=%s; Glade=%s", row[2], payload)
					}
				})
			}
			if len(seen) != 2 {
				t.Fatal("missing routes")
			}
		})
	}
}
