package apextest

import (
	"encoding/csv"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

const loopScopeFixture = "../../testdata/conformance/loop_scope_dynamic_binds"

type loopScopeRow struct {
	id, api, route, observation string
	compiled                    bool
}

// TestLoopScopeDynamicBindsNativeConformance replays native rows for locals
// declared in while and do-while bodies and for the locals a dynamic SOQL
// string binds (anonymous and named-class routes, API 62.0 and 67.0).
// Rows named in parked.tsv are native observations Glade does not match yet;
// they are listed with a reason and skipped by id, never edited.
func TestLoopScopeDynamicBindsNativeConformance(t *testing.T) {
	rows := readLoopScopeTSV(t, "org.tsv")
	cases := map[string]string{}
	for _, row := range readLoopScopeTSV(t, "cases.tsv") {
		cases[row[0]+"/"+row[1]] = row[2]
	}
	parked := map[string]bool{}
	if _, err := os.Stat(filepath.Join(loopScopeFixture, "parked.tsv")); err == nil {
		for _, row := range readLoopScopeTSV(t, "parked.tsv") {
			parked[row[0]] = true
		}
	}
	var native []loopScopeRow
	for _, row := range rows {
		if len(row) != 5 || (row[3] != "true" && row[3] != "false") {
			t.Fatalf("invalid native row: %v", row)
		}
		if _, ok := cases[row[0]+"/"+row[2]]; !ok {
			t.Fatalf("native row names unknown case %q", row[0])
		}
		native = append(native, loopScopeRow{id: row[0], api: row[1], route: row[2], compiled: row[3] == "true", observation: row[4]})
	}
	if len(native) == 0 {
		t.Fatal("no native rows")
	}
	for _, api := range []string{"62.0", "67.0"} {
		api := api
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			runLoopScopeAPI(t, api, native, cases, parked)
		})
	}
}

func readLoopScopeTSV(t *testing.T, name string) [][]string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(loopScopeFixture, name))
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(strings.NewReader(string(raw)))
	reader.Comma = '\t'
	reader.LazyQuotes = true
	all, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	return all[1:]
}

func loopScopeSource(t *testing.T, relative string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(loopScopeFixture, relative))
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func runLoopScopeAPI(t *testing.T, api string, native []loopScopeRow, cases map[string]string, parked map[string]bool) {
	classes := map[string]string{}
	classOf := map[string]string{}
	for _, row := range native {
		if row.api != api || row.route != "named_class" || !row.compiled {
			continue
		}
		name := strings.TrimSuffix(filepath.Base(cases[row.id+"/"+row.route]), ".cls")
		classes[name] = loopScopeSource(t, cases[row.id+"/"+row.route])
		classOf[row.id] = name
	}
	index := nativeBoundaryIndex(t, api, classes, nil)
	if index.HasErrors() {
		t.Fatalf("native named classes did not index at API %s: %+v", api, index.Diagnostics)
	}
	testCases := Discover(index, Options{})
	methods, methodErrors := compileTestMethodsForIndex(&index, testCases)
	programs, programErrors := compileTestInvokePrograms(testCases)
	org := orgFromIndex(index)
	org.APIVersion = api
	base := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
	anonymousIndex := nativeBoundaryIndex(t, api, map[string]string{}, nil)

	for _, row := range native {
		if row.api != api || parked[row.id] {
			continue
		}
		row := row
		t.Run(row.id+"/"+row.route, func(t *testing.T) {
			switch row.route {
			case "anonymous":
				source := loopScopeSource(t, cases[row.id+"/"+row.route])
				analysis := sema.AnalyzeAnonymous(anonymousIndex, source, api)
				if !row.compiled {
					requireLoopScopeCompileError(t, row, analysis)
					return
				}
				if analysis.HasErrors() {
					t.Fatalf("native compiled; Glade diagnostics=%+v", analysis.Diagnostics)
				}
				program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatalf("native compiled; Glade compile error=%v", err)
				}
				machine := base.newMachine()
				result, runErr := machine.Execute(program)
				got := loopScopeObservation(result.Debug, runErr)
				if got != row.observation {
					t.Fatalf("native=%q; Glade=%q", row.observation, got)
				}
			case "named_class":
				if !row.compiled {
					name := strings.TrimSuffix(filepath.Base(cases[row.id+"/"+row.route]), ".cls")
					single := nativeBoundaryIndex(t, api, map[string]string{name: loopScopeSource(t, cases[row.id+"/"+row.route])}, nil)
					requireLoopScopeCompileError(t, row, sema.Analyze(single))
					return
				}
				name := classOf[row.id]
				machine := base.newMachine()
				machine.EnableTestContext()
				var result vm.Result
				var runErr error
				found := false
				for _, test := range testCases {
					if test.ClassName != name {
						continue
					}
					found = true
					key := testCaseKey(test)
					if methodErrors[key] != nil || programErrors[key] != nil {
						t.Fatalf("native executes: method=%v program=%v", methodErrors[key], programErrors[key])
					}
					if err := machine.RegisterMethod(methods[key]); err != nil {
						t.Fatal(err)
					}
					result, runErr = machine.ExecuteInClass(programs[key], name)
				}
				if !found {
					t.Fatalf("test class %s was not discovered", name)
				}
				got := loopScopeObservation(result.Debug, runErr)
				if got != row.observation {
					t.Fatalf("native=%q; Glade=%q", row.observation, got)
				}
			default:
				t.Fatalf("unknown route %q", row.route)
			}
		})
	}
}

func requireLoopScopeCompileError(t *testing.T, row loopScopeRow, analysis sema.Result) {
	t.Helper()
	if !analysis.HasErrors() {
		t.Fatalf("native rejects with %q; Glade compiled", row.observation)
	}
	// Glade words these two compile errors itself; the native text maps to its
	// diagnostic code by prefix.
	code := ""
	switch {
	case strings.HasPrefix(row.observation, "Variable does not exist:"):
		code = "GLADESEMA013"
	case strings.HasPrefix(row.observation, "Duplicate variable:"):
		code = "GLADESEMA014"
	}
	for _, diagnostic := range analysis.Diagnostics {
		message := diagnostic.NativeMessage
		if message == "" {
			message = diagnostic.Message
		}
		if strings.Contains(message, row.observation) || (code != "" && diagnostic.Code == code) {
			return
		}
	}
	t.Fatalf("native rejects with %q; Glade diagnostics=%+v", row.observation, analysis.Diagnostics)
}

// loopScopeObservation renders one run as the org.tsv observation: OUT lines
// joined by "|", an assertion message carrying the named-class buffer, or the
// uncaught exception as EXC:type: message.
func loopScopeObservation(debug []string, runErr error) string {
	if runErr != nil {
		var runtimeErr *vm.RuntimeError
		if errors.As(runErr, &runtimeErr) {
			if runtimeErr.Type == "System.AssertException" && strings.HasPrefix(runtimeErr.Message, "Assertion Failed: OUT:") {
				buffer := strings.TrimPrefix(runtimeErr.Message, "Assertion Failed: ")
				return strings.ReplaceAll(strings.TrimSuffix(buffer, ";"), ";OUT:", "|OUT:")
			}
			return "EXC:System." + strings.TrimPrefix(runtimeErr.Type, "System.") + ": " + runtimeErr.Message
		}
		return "ERR:" + runErr.Error()
	}
	var lines []string
	for _, line := range debug {
		if strings.HasPrefix(line, "OUT:") {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "|")
}
