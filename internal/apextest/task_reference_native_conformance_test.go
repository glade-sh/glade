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

func TestTaskReferenceNativeConformance(t *testing.T) {
	const dir = "testdata/conformance/task_reference"
	file, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma, reader.LazyQuotes = '\t', true
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 5 || strings.Join(rows[0], ",") != "source_api,route,observation" {
		t.Fatalf("incomplete native matrix: %v", err)
	}
	seen := map[string]bool{}
	for _, api := range []string{"62.0", "67.0"} {
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
				rel, err := filepath.Rel(project, path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, rel), strings.ReplaceAll(string(body), "62.0", api))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native deployed; diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runtime, err := CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
			if err != nil {
				t.Fatal(err)
			}
			request := newConformanceRunner(t, index, conformanceRunnerOptions{RequestRuntime: &runtime, Org: &org})
			named := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			for _, row := range rows[1:] {
				if len(row) != 3 || (row[1] != "anonymous" && row[1] != "isTest") {
					t.Fatal("malformed native row")
				}
				if row[0] != api {
					continue
				}
				key := api + "/" + row[1]
				if seen[key] {
					t.Fatal("duplicate native row")
				}
				seen[key] = true
				routes := []string{row[1]}
				if row[1] == "isTest" {
					routes = append(routes, "namedVM")
				}
				for _, route := range routes {
					t.Run(route, func(t *testing.T) {
						message := ""
						switch route {
						case "anonymous":
							source, err := os.ReadFile(filepath.Join(dir, "anonymous.apex"))
							if err != nil {
								t.Fatal(err)
							}
							if analysis := sema.AnalyzeAnonymous(index, string(source), api); analysis.HasErrors() {
								t.Fatal(analysis.Diagnostics)
							}
							program, err := vm.CompileAnonymousWithOptions(string(source), vm.CompileOptions{APIVersion: api})
							if err != nil {
								t.Fatal(err)
							}
							result, err := request.execute(program)
							if err != nil {
								t.Fatal(err)
							}
							for _, line := range result.Debug {
								if strings.HasPrefix(line, "OBS|") {
									if message != "" {
										t.Fatal("duplicate observation")
									}
									message = line
								}
							}
						case "isTest":
							run := Run(index, Options{SelectedClasses: []string{"TaskReferenceProbeTest"}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
							if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
								t.Fatalf("observation discovery: %+v", run.Summary())
							}
							result := run.Suites[0].Cases[0]
							if result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
								t.Fatalf("missing assertion transport: %+v", result.Problem)
							}
							message = result.Problem.Message
						case "namedVM":
							cases := Discover(index, Options{SelectedClasses: []string{"TaskReferenceProbeTest"}, SelectedMethod: "observe"})
							if len(cases) != 1 {
								t.Fatal("missing direct observer")
							}
							methods, methodErrors := compileTestMethods(cases)
							programs, programErrors := compileTestInvokePrograms(cases)
							key := testCaseKey(cases[0])
							if methodErrors[key] != nil || programErrors[key] != nil {
								t.Fatalf("compile: %v %v", methodErrors[key], programErrors[key])
							}
							machine := named.newMachine()
							if err := machine.RegisterMethod(methods[key]); err != nil {
								t.Fatal(err)
							}
							machine.EnableTestContext()
							_, err := machine.ExecuteInClass(programs[key], cases[0].ClassName)
							var issue *vm.RuntimeError
							if !errors.As(err, &issue) || issue.Type != "System.AssertException" {
								t.Fatalf("missing direct observation: %v", err)
							}
							message = issue.ExceptionMessage()
						}
						_, payload, ok := strings.Cut(message, "OBS|")
						var got, want map[string]any
						if !ok || json.Unmarshal([]byte(payload), &got) != nil || json.Unmarshal([]byte(row[2]), &want) != nil || len(want) != 48 || len(got) != len(want) {
							t.Fatalf("invalid observation: %s", message)
						}
						for cell, expected := range want {
							t.Run(cell, func(t *testing.T) {
								if !reflect.DeepEqual(got[cell], expected) {
									t.Fatalf("native=%v; Glade=%v", expected, got[cell])
								}
							})
						}
					})
				}
			}
		})
	}
	if len(seen) != 4 {
		t.Fatalf("native row count=%d", len(seen))
	}
}
