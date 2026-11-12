package apextest

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// The automation_residual_paths family holds native rows for flow, duplicate
// rule, assignment rule and Apex-action paths that earlier families did not
// cover. org.tsv columns: source_api, route (anonymous or istest), method,
// observation (JSON). Anonymous rows run ResidualRun.run(method); istest rows
// run the same method of ResidualProbe. A method name ending in ".repeat" is
// a second native observation of the same method that differed from the first.
const automationResidualFixture = "testdata/conformance/automation_residual_paths"

type automationResidualRow struct {
	API, Route, Method, Observation string
}

func automationResidualRows(t *testing.T) []automationResidualRow {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(automationResidualFixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	if len(lines) < 2 || lines[0] != "source_api\troute\tmethod\tobservation" {
		t.Fatal("invalid native rows header")
	}
	rows := make([]automationResidualRow, 0, len(lines)-1)
	for _, line := range lines[1:] {
		cells := strings.SplitN(line, "\t", 4)
		if len(cells) != 4 || !json.Valid([]byte(cells[3])) {
			t.Fatalf("invalid native row: %s", line)
		}
		rows = append(rows, automationResidualRow{cells[0], cells[1], cells[2], cells[3]})
	}
	return rows
}

// automationResidualRowLine is the 1-based line of a row in org.tsv.
func automationResidualRowLine(t *testing.T, api, route, method string) int {
	t.Helper()
	for i, row := range automationResidualRows(t) {
		if row.API == api && row.Route == route && row.Method == method {
			return i + 2
		}
	}
	t.Fatalf("no native row %s %s %s", api, route, method)
	return 0
}

var (
	automationResidualID      = regexp.MustCompile(`00Q[A-Za-z0-9]{12}(?:[A-Za-z0-9]{3})?`)
	automationResidualErrorID = regexp.MustCompile(`Error ID: \d+-\d+ \(-?\d+\)`)
)

func automationResidualNormalize(raw string) any {
	raw = automationResidualID.ReplaceAllString(raw, "00QREDACTED")
	raw = automationResidualErrorID.ReplaceAllString(raw, "Error ID: N")
	var value any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return raw
	}
	return value
}

func newAutomationResidualFixtureRun(t *testing.T, api string) (anonymous, named func(method string) string) {
	t.Helper()
	root := t.TempDir()
	source := filepath.Join(automationResidualFixture, "project")
	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		name, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		writeFile(t, filepath.Join(root, name), strings.ReplaceAll(string(body), "62.0", api))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// The capture assigned ResidualFields to the running user (RUN-FACTS.txt).
	writeFile(t, filepath.Join(root, "glade.yml"), "test:\n  permissionSets: [ResidualFields]\n")
	index := loadTestIndex(t, root)
	analysis := sema.Analyze(index)
	if index.HasErrors() || analysis.HasErrors() {
		t.Fatalf("native project accepted; diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
	}
	selected := Discover(index, Options{SelectedClasses: []string{"ResidualProbe"}})
	org := orgFromIndex(index)
	org.APIVersion = api
	if err := applyConfiguredTestPermissions(&org, loadProjectForTest(t, root)); err != nil {
		t.Fatal(err)
	}
	runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
	methods, methodErrors := compileTestMethodsForIndex(&index, selected)
	programs, programErrors := compileTestInvokePrograms(selected)
	anonymous = func(method string) string {
		t.Helper()
		program, err := vm.CompileAnonymousWithOptions("System.assert(false, 'OBS|' + ResidualRun.run('"+method+"'));", vm.CompileOptions{APIVersion: api})
		if err != nil {
			t.Fatal(err)
		}
		_, err = runner.newMachine().Execute(program)
		return automationResidualAssertion(t, err)
	}
	named = func(method string) string {
		t.Helper()
		for _, row := range selected {
			if row.MethodName != method {
				continue
			}
			key := testCaseKey(row)
			if methodErrors[key] != nil || programErrors[key] != nil {
				t.Fatalf("compile: %v %v", methodErrors[key], programErrors[key])
			}
			machine := runner.newMachine()
			machine.EnableTestContext()
			if err := machine.RegisterMethod(methods[key]); err != nil {
				t.Fatal(err)
			}
			_, err := machine.ExecuteTestInClass(programs[key], row.ClassName)
			return automationResidualAssertion(t, err)
		}
		t.Fatalf("no test method %s", method)
		return ""
	}
	return anonymous, named
}

// automationResidualAssertion returns the observation transported by the
// assertion message, or the uncaught error text when the run failed otherwise.
func automationResidualAssertion(t *testing.T, err error) string {
	t.Helper()
	var runtimeErr *vm.RuntimeError
	if !errors.As(err, &runtimeErr) {
		t.Fatalf("missing assertion transport: %v", err)
	}
	message := runtimeErr.ExceptionMessage()
	if _, observed, ok := strings.Cut(message, "OBS|"); ok {
		return observed
	}
	if _, observed, ok := strings.Cut(runtimeErr.Message, "OBSERVED|"); ok {
		if _, payload, ok := strings.Cut(observed, "|"); ok {
			return payload
		}
	}
	return "UNCAUGHT " + runtimeErr.Type + ": " + message
}

// runAutomationResidualRows replays every native row of the listed methods
// that matches an API, on both routes. A row with a ".repeat" twin passes
// when Glade matches either observation.
func runAutomationResidualRows(t *testing.T, methods ...string) {
	t.Helper()
	want := map[string]bool{}
	for _, method := range methods {
		want[method] = true
	}
	rows := automationResidualRows(t)
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			anonymous, named := newAutomationResidualFixtureRun(t, api)
			seen := map[string]bool{}
			for _, row := range rows {
				base := strings.TrimSuffix(row.Method, ".repeat")
				if row.API != api || !want[base] || row.Method != base {
					continue
				}
				seen[base] = true
				t.Run(row.Route+"/"+base, func(t *testing.T) {
					var got string
					if row.Route == "anonymous" {
						got = anonymous(base)
					} else {
						got = named(base)
					}
					accepted := []string{row.Observation}
					for _, other := range rows {
						if other.API == api && other.Route == row.Route && other.Method == base+".repeat" {
							accepted = append(accepted, other.Observation)
						}
					}
					for _, observation := range accepted {
						if reflect.DeepEqual(automationResidualNormalize(got), automationResidualNormalize(observation)) {
							return
						}
					}
					t.Fatalf("NATIVE %s %s:%d %s %s\nnative=%s\nGlade =%s", base, automationResidualFixture+"/org.tsv", automationResidualRowLine(t, api, row.Route, row.Method), row.Route, api, row.Observation, got)
				})
			}
			for method := range want {
				if !seen[method] {
					t.Errorf("no native row for %s at %s", method, api)
				}
			}
		})
	}
}
