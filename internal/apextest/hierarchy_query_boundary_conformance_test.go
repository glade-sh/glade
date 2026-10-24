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

func TestHierarchyQueryBoundaryNativeRouteTwins(t *testing.T) {
	const dir = "testdata/conformance/hierarchy_query_boundary"
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
	if len(rows) != 15 || strings.Join(rows[0], ",") != "source_api,route,observation" {
		t.Fatal("incomplete native matrix")
	}
	for _, api := range []string{"62.0", "63.0", "64.0", "65.0", "66.0", "67.0"} {
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
			seen := map[string]bool{}
			for _, row := range rows[1:] {
				if len(row) != 3 {
					t.Fatal("malformed native row")
				}
				if row[0] != api {
					continue
				}
				if seen[row[1]] || (row[1] != "anonymous" && row[1] != "isTest" && row[1] != "isTestGrant") {
					t.Fatal("duplicate or invalid native route")
				}
				seen[row[1]] = true
				routes := []string{row[1]}
				if row[1] != "anonymous" {
					routes = append(routes, "namedVM")
				}
				for _, route := range routes {
					execution := route
					if row[1] == "isTestGrant" && route == "namedVM" {
						execution = "namedVMGrant"
					}
					t.Run(execution, func(t *testing.T) {
						className := "HierarchyBoundaryTest"
						if row[1] == "isTestGrant" {
							className = "HierarchyBoundaryGrantTest"
						}
						message := ""
						switch route {
						case "anonymous":
							source := "System.debug('OBSERVED|'+JSON.serialize(HierarchyBoundaryProbe.all()));"
							if analysis := sema.AnalyzeAnonymous(index, source, api); analysis.HasErrors() {
								t.Fatal(analysis.Diagnostics)
							}
							program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
							if err != nil {
								t.Fatal(err)
							}
							result, err := request.execute(program)
							if err != nil {
								t.Fatal(err)
							}
							for _, line := range result.Debug {
								if strings.HasPrefix(line, "OBSERVED|") {
									message = line
								}
							}
						case "isTest", "isTestGrant":
							run := Run(index, Options{SelectedClasses: []string{className}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
							if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
								t.Fatalf("discovery: %+v", run.Summary())
							}
							result := run.Suites[0].Cases[0]
							if result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
								t.Fatalf("missing observation: %+v", result.Problem)
							}
							message = result.Problem.Message
						case "namedVM":
							cases := Discover(index, Options{SelectedClasses: []string{className}, SelectedMethod: "observe"})
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
						_, payload, ok := strings.Cut(message, "OBSERVED|")
						var got, want map[string]any
						if !ok || json.Unmarshal([]byte(payload), &got) != nil || json.Unmarshal([]byte(row[2]), &want) != nil {
							t.Fatalf("invalid observation: %s", message)
						}
						for _, shape := range []string{"organization", "override"} {
							expected, actual := want[shape].(map[string]any), got[shape].(map[string]any)
							for context, native := range expected {
								if !reflect.DeepEqual(actual[context], native) {
									t.Errorf("%s/%s native=%v; Glade=%v", shape, context, native, actual[context])
								}
							}
							if len(actual) != len(expected) {
								t.Errorf("%s context count=%d, native=%d", shape, len(actual), len(expected))
							}
						}
					})
				}
			}
			expectedRoutes := 2
			if api == "62.0" || api == "67.0" {
				expectedRoutes = 3
			}
			if len(seen) != expectedRoutes {
				t.Fatal("missing native route")
			}
		})
	}
}
