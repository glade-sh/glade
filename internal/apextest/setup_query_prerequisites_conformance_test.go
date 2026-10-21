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

func TestSetupQueryPrerequisitesNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/setup_query_prerequisites"
	file, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma, reader.LazyQuotes = '\t', true
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 13 || strings.Join(rows[0], ",") != "source_api,route,method,observation" {
		t.Fatal("incomplete native observations")
	}
	seen := map[string]bool{}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			project := filepath.Join(fixture, "project")
			err := filepath.WalkDir(project, func(path string, entry os.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				relative, err := filepath.Rel(project, path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, relative), strings.ReplaceAll(string(body), "62.0", api))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native source compiled; local diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			for _, row := range rows[1:] {
				if len(row) != 4 || row[0] != api {
					continue
				}
				key := strings.Join(row[:3], "/")
				if seen[key] || (row[2] != "metadata" && row[2] != "community" && row[2] != "packageLicenses") {
					t.Fatal("invalid native observation", key)
				}
				seen[key] = true
				t.Run(row[1]+"/"+row[2], func(t *testing.T) {
					var message string
					if row[1] == "isTest" {
						run := Run(index, Options{SelectedClasses: []string{"SetupQueryPrerequisitesTest"}, SelectedMethod: row[2], NoDiskCache: true, Parallelism: 1})
						if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
							t.Fatalf("selection: %+v", run.Summary())
						}
						result := run.Suites[0].Cases[0]
						if result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
							t.Fatalf("missing observation: %+v", result)
						}
						message = result.Problem.Message
					} else if row[1] == "anonymous" {
						body := "System.assert(false,'OBSERVED|" + row[2] + "|'+JSON.serialize(SetupQueryPrerequisiteProbe." + row[2] + "()));"
						if analysis := sema.AnalyzeAnonymous(index, body, api); analysis.HasErrors() {
							t.Fatal(analysis.Diagnostics)
						}
						org := orgFromIndex(index)
						runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
						program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						_, err = runner.newMachine().Execute(program)
						var runtimeErr *vm.RuntimeError
						if !errors.As(err, &runtimeErr) || runtimeErr.Type != "System.AssertException" {
							t.Fatalf("missing observation: %v", err)
						}
						message = runtimeErr.ExceptionMessage()
					} else {
						t.Fatal("invalid native route", row[1])
					}
					_, payload, found := strings.Cut(message, "OBSERVED|"+row[2]+"|")
					var got, want map[string]any
					if !found || json.Unmarshal([]byte(payload), &got) != nil || json.Unmarshal([]byte(row[3]), &want) != nil {
						t.Fatalf("invalid observation: %s", message)
					}
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("native=%s; Glade=%s", row[3], payload)
					}
				})
			}
		})
	}
	if len(seen) != 12 {
		t.Fatalf("observations=%d; want 12", len(seen))
	}
}
