package apextest

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestHierarchyQueryOverloadsProjectNativeRouteTwins(t *testing.T) {
	const dir = "testdata/conformance/hierarchy_query_boundary/overloads"
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
	if len(rows) != 7 {
		t.Fatal("incomplete native overload matrix")
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			copyDiskRuntimeScopeProject(t, filepath.Join(dir, "project"), root)
			err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				if !strings.HasSuffix(path, ".xml") && !strings.HasSuffix(path, ".json") {
					return nil
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				return os.WriteFile(path, []byte(strings.ReplaceAll(string(body), "62.0", api)), 0600)
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
			for _, row := range rows[1:] {
				if row[0] != api {
					continue
				}
				check := func(t *testing.T, message string) {
					t.Helper()
					_, payload, ok := strings.Cut(message, "OBSERVED|")
					if !ok {
						t.Fatalf("missing native observation: %s", message)
					}
					var got, want map[string]json.RawMessage
					if json.Unmarshal([]byte(payload), &got) != nil || json.Unmarshal([]byte(row[2]), &want) != nil {
						t.Fatalf("invalid observation: %s", message)
					}
					if len(got) != len(want) {
						t.Errorf("context count=%d; native=%d", len(got), len(want))
					}
					for context, expected := range want {
						if context == "boundary" {
							if string(got[context]) != string(expected) {
								t.Errorf("anonymous boundary=%s; native=%s", got[context], expected)
							}
							continue
						}
						var observed, native map[string]any
						if json.Unmarshal(got[context], &observed) != nil || json.Unmarshal(expected, &native) != nil {
							t.Fatalf("missing context %s: %s", context, payload)
						}
						if len(observed) != len(native) {
							t.Errorf("%s route count=%d; native=%d", context, len(observed), len(native))
						}
						for route, value := range native {
							t.Run(context+"/"+route, func(t *testing.T) {
								if observed[route] != value {
									t.Errorf("native=%v; Glade=%v", value, observed[route])
								}
							})
						}
					}
				}
				if row[1] == "anonymous" {
					t.Run("anonymousRequestRuntime", func(t *testing.T) {
						source := "System.debug('OBSERVED|'+JSON.serialize(HierarchyOverloadProbe.observe(false)));"
						if analysis := sema.AnalyzeAnonymous(index, source, api); analysis.HasErrors() {
							t.Fatal(analysis.Diagnostics)
						}
						program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						runner := newConformanceRunner(t, index, conformanceRunnerOptions{RequestRuntime: &runtime, Org: &org})
						result, err := runner.execute(program)
						if err != nil {
							t.Fatal(err)
						}
						message := ""
						for _, line := range result.Debug {
							if strings.HasPrefix(line, "OBSERVED|") {
								message = line
							}
						}
						check(t, message)
					})
					continue
				}
				method := "observe"
				if row[1] == "isTestGrant" {
					method = "observeGrant"
				}
				cases := Discover(index, Options{SelectedClasses: []string{"HierarchyOverloadTest"}, SelectedMethod: method})
				if len(cases) != 1 {
					t.Fatal("missing overload observer")
				}
				methods, methodErrors := compileTestMethods(cases)
				programs, programErrors := compileTestInvokePrograms(cases)
				key := testCaseKey(cases[0])
				if methodErrors[key] != nil || programErrors[key] != nil {
					t.Fatalf("compile=%v %v", methodErrors[key], programErrors[key])
				}
				for _, entry := range []string{"namedVM", "anonymousRequestRuntimeTest"} {
					t.Run(row[1]+"/"+entry, func(t *testing.T) {
						options := conformanceRunnerOptions{LinkProject: true, Org: &org}
						if entry == "anonymousRequestRuntimeTest" {
							options = conformanceRunnerOptions{RequestRuntime: &runtime, Org: &org}
						}
						runner := newConformanceRunner(t, index, options)
						machine := runner.newMachine()
						machine.EnableTestContext()
						if err := machine.RegisterMethod(methods[key]); err != nil {
							t.Fatal(err)
						}
						_, executionError := machine.ExecuteInClass(programs[key], cases[0].ClassName)
						var issue *vm.RuntimeError
						if !errors.As(executionError, &issue) || issue.Type != "System.AssertException" {
							t.Fatalf("missing observation: %v", executionError)
						}
						check(t, issue.ExceptionMessage())
					})
				}
				InvalidateRuntimeCaches()
				t.Cleanup(InvalidateRuntimeCaches)
				for _, entry := range []string{"isTestCold", "isTestCachedParallel"} {
					t.Run(row[1]+"/"+entry, func(t *testing.T) {
						workers := 1
						if entry == "isTestCachedParallel" {
							workers = 2
						}
						selectedMethod, wantCases := method, 1
						if entry == "isTestCachedParallel" {
							selectedMethod, wantCases = "", 2
						}
						ResetPerfCounters()
						run := Run(index, Options{SelectedClasses: []string{"HierarchyOverloadTest"}, SelectedMethod: selectedMethod, NoDiskCache: true, ParallelMethods: true, Parallelism: workers, RestoredRuntimeMultiWorker: true, PerfCounters: true})
						if len(run.Suites) != 1 || len(run.Suites[0].Cases) != wantCases {
							t.Fatalf("discovery: %+v", run.Summary())
						}
						if entry == "isTestCachedParallel" && SnapshotPerfCounters().Phases.MemoryCacheHits != 1 {
							t.Fatalf("expected cached parallel runtime: %+v", SnapshotPerfCounters().Phases)
						}
						found := false
						for _, result := range run.Suites[0].Cases {
							if result.MethodName != method {
								continue
							}
							found = true
							problem := result.Problem
							if problem == nil || !strings.HasSuffix(problem.Type, "AssertException") {
								t.Fatalf("missing observation: %+v", problem)
							}
							check(t, problem.Message)
						}
						if !found {
							t.Fatal("missing selected observer")
						}
					})
				}
			}
		})
	}
}
