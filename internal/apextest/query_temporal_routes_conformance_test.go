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

func TestQueryTemporalRouteNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/query_temporal_routes"
	read := func(path string) string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(fixture, path))
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	if !strings.Contains(read("RUN-FACTS.txt"), "native_complete=true\n") {
		t.Fatal("native capture is incomplete")
	}
	table := csv.NewReader(strings.NewReader(read("org.tsv")))
	table.Comma, table.LazyQuotes = '\t', true
	rows, err := table.ReadAll()
	if err != nil || len(rows) != 17 || strings.Join(rows[0], "|") != "source_api|entry_route|execution_route|observation" {
		t.Fatalf("incomplete native matrix: %v", err)
	}
	cases := csv.NewReader(strings.NewReader(read("cases.tsv")))
	cases.Comma = '\t'
	caseRows, err := cases.ReadAll()
	if err != nil || len(caseRows) != 77 || strings.Join(caseRows[0], "|") != "query_route|case_id|where_clause" {
		t.Fatalf("incomplete query route matrix: %v", err)
	}
	cells := map[string]bool{}
	for _, row := range caseRows[1:] {
		key := row[0] + "/" + row[1]
		if cells[key] || row[2] == "" {
			t.Fatal("invalid query case: " + key)
		}
		cells[key] = true
	}
	controls := csv.NewReader(strings.NewReader(read("controls.tsv")))
	controls.Comma, controls.LazyQuotes = '\t', true
	controlRows, err := controls.ReadAll()
	if err != nil || len(controlRows) != 5 || strings.Join(controlRows[0], "|") != "source_api|execution_route|rejection" {
		t.Fatal("missing native outside-query rejection controls")
	}
	seen := map[string]bool{}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			project := filepath.Join(fixture, "project")
			if err := filepath.WalkDir(project, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				name, err := filepath.Rel(project, path)
				if err == nil {
					writeFile(t, filepath.Join(root, name), strings.ReplaceAll(string(body), "62.0", api))
				}
				return err
			}); err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native project compiles; Glade diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			for _, control := range controlRows[1:] {
				if control[0] != api {
					continue
				}
				t.Run("outside/"+control[1], func(t *testing.T) {
					if control[2] == "" {
						t.Fatal("missing native compiler rejection")
					}
					if control[1] == "anonymous" {
						if analysis := sema.AnalyzeAnonymous(index, read("controls/outside.apex"), api); !analysis.HasErrors() {
							t.Fatal("native rejected outside-query DateTime; Glade accepted it")
						}
					} else if control[1] == "class" {
						negativeRoot := t.TempDir()
						writeFile(t, filepath.Join(negativeRoot, "sfdx-project.json"), "{\"sourceApiVersion\":\""+api+"\",\"packageDirectories\":[{\"path\":\"force-app\",\"default\":true}]}")
						writeFile(t, filepath.Join(negativeRoot, "force-app/TemporalLiteralOutside.cls"), read("controls/TemporalLiteralOutside.cls"))
						negativeIndex := loadTestIndex(t, negativeRoot)
						if analysis := sema.Analyze(negativeIndex); !negativeIndex.HasErrors() && !analysis.HasErrors() {
							t.Fatal("native rejected outside-query class DateTime; Glade accepted it")
						}
					} else {
						t.Fatal("unknown native rejection route")
					}
				})
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			requestRuntime, err := CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
			if err != nil {
				t.Fatal(err)
			}
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{RequestRuntime: &requestRuntime, Org: &org})
			for _, row := range rows[1:] {
				if row[0] != api {
					continue
				}
				entry, route := row[1], row[2]
				key := api + "/" + entry + "/" + route
				if seen[key] || (entry != "direct" && entry != "trigger" && entry != "flow" && entry != "triggerFlow") || (route != "anonymous" && route != "isTest") {
					t.Fatal("invalid or repeated native route: " + key)
				}
				seen[key] = true
				t.Run(entry+"/"+route, func(t *testing.T) {
					var want map[string]any
					expectedCells := len(cells)
					if entry == "trigger" {
						expectedCells++
					}
					if json.Unmarshal([]byte(row[3]), &want) != nil || len(want) != expectedCells {
						t.Fatal("incomplete captured query cells")
					}
					var message string
					if route == "anonymous" {
						source := strings.ReplaceAll(read("anonymous.apex"), "ENTRY", entry)
						if analysis := sema.AnalyzeAnonymous(index, source, api); analysis.HasErrors() {
							t.Fatalf("native anonymous compiles: %v", analysis.Diagnostics)
						}
						program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						_, execErr := runner.execute(program)
						var runtimeErr *vm.RuntimeError
						if !errors.As(execErr, &runtimeErr) || !strings.HasSuffix(runtimeErr.Type, "AssertException") {
							t.Fatalf("missing anonymous observation: %v", execErr)
						}
						message = runtimeErr.ExceptionMessage()
					} else {
						method := "observe" + strings.ToUpper(entry[:1]) + entry[1:]
						run := Run(index, Options{SelectedClasses: []string{"TemporalQuerySpec"}, SelectedMethod: method, NoDiskCache: true, Parallelism: 1})
						if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
							t.Fatalf("expected one named observation: %+v", run.Summary())
						}
						problem := run.Suites[0].Cases[0].Problem
						if problem == nil || !strings.HasSuffix(problem.Type, "AssertException") {
							t.Fatalf("missing named observation: %+v", problem)
						}
						message = problem.Message
					}
					_, payload, found := strings.Cut(message, "TEMPORAL|"+entry+"|")
					var got map[string]any
					if !found || json.Unmarshal([]byte(payload), &got) != nil || len(got) != len(want) {
						t.Fatalf("invalid query observation: %s", message)
					}
					for cell, expected := range want {
						if !cells[cell] && !(entry == "trigger" && cell == "trigger/literal") {
							t.Fatal("unexpected native query cell: " + cell)
						}
						t.Run(cell, func(t *testing.T) {
							if !reflect.DeepEqual(got[cell], expected) {
								t.Fatalf("native=%v; Glade=%v", expected, got[cell])
							}
						})
					}
				})
			}
		})
	}
	if len(seen) != 16 {
		t.Fatalf("missing entry/execution/API route: %d", len(seen))
	}
}
