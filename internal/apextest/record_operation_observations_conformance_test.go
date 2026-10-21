package apextest

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestInsertedIdentifierBodyNativeConformance(t *testing.T) {
	runRecordOperationObservations(t, []string{"identifierAccount", "identifierCustom"}, 12)
}

func TestGroupedMergeResultNativeConformance(t *testing.T) {
	runRecordOperationObservations(t, []string{"mergeAll", "mergePartial", "mergeEmpty"}, 18)
}

func TestSetupPermissionSetVisibilityNativeConformance(t *testing.T) {
	runRecordOperationObservations(t, []string{"setupVisibility"}, 3)
}

func runRecordOperationObservations(t *testing.T, methods []string, expectedRows int) {
	t.Helper()
	dir := filepath.Join("testdata", "conformance", "record_operation_observations")
	file, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 34 || strings.Join(rows[0], ",") != "source_api,route,method,observation" {
		t.Fatal("incomplete native observations")
	}
	selected := map[string]bool{}
	for _, method := range methods {
		selected[method] = true
	}
	observed := 0
	seen := map[string]bool{}
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			project := filepath.Join(dir, "project")
			err := filepath.WalkDir(project, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				name, err := filepath.Rel(project, path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, name), strings.ReplaceAll(string(body), "62.0", api))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native source compiled; Glade diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			for _, row := range rows[1:] {
				if row[0] != api || !selected[row[2]] {
					continue
				}
				key := strings.Join(row[:3], "/")
				if seen[key] {
					t.Fatal("duplicate native observation", key)
				}
				seen[key] = true
				observed++
				t.Run(row[1]+"/"+row[2], func(t *testing.T) {
					var message string
					switch row[1] {
					case "isTest":
						run := Run(index, Options{SelectedClasses: []string{"RecordOperationProbeTest"}, SelectedMethod: row[2], NoDiskCache: true, Parallelism: 1})
						if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
							t.Fatalf("expected one observation: %+v", run.Summary())
						}
						result := run.Suites[0].Cases[0]
						if result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
							t.Fatalf("missing assertion transport: %+v", result)
						}
						message = result.Problem.Message
					case "anonymous":
						runtime, err := CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
						if err != nil {
							t.Fatal(err)
						}
						org := orgFromIndex(index)
						org.APIVersion = api
						runner := newConformanceRunner(t, index, conformanceRunnerOptions{RequestRuntime: &runtime, Org: &org})
						program, err := vm.CompileAnonymousWithOptions("System.assert(false,'OBSERVED|"+row[2]+"|'+JSON.serialize(RecordOperationProbe.observe('"+row[2]+"')));", vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						_, err = runner.newMachine().Execute(program)
						var runtimeErr *vm.RuntimeError
						if !errors.As(err, &runtimeErr) || runtimeErr.Type != "System.AssertException" {
							t.Fatalf("missing assertion transport: %v", err)
						}
						message = runtimeErr.ExceptionMessage()
					default:
						t.Fatal("unknown native route", row[1])
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
	if observed != expectedRows {
		t.Fatalf("native rows=%d; want %d", observed, expectedRows)
	}
}
