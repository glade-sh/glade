package apextest

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/vm"
)

// The anonymous and deploy rows pin diagnostics for an unavailable receiver.
// The named rows pin three ways a missing value can reach a null receiver.
func TestUnresolvedReceiverNativeConformance(t *testing.T) {
	const dir = "testdata/conformance/unresolved_receiver"
	file, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 46 {
		t.Fatalf("native rows=%d: %v", len(rows), err)
	}

	classes := namespaceFamilyClasses(t, dir, "classes")
	deployClasses := namespaceFamilyClasses(t, dir, "deploy-checks")
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			index := namespaceFamilyProject(t, "", api, classes)
			deployRows := map[string]map[string]string{}
			for _, row := range rows[1:] {
				id, sourceAPI, route, name, status, expected := row[0], row[1], row[2], row[3], row[4], row[5]
				if sourceAPI != api {
					continue
				}
				switch route {
				case "anonymous":
					t.Run(id, func(t *testing.T) {
						body := namespaceFamilyFixture(t, dir, id+".apex")
						errors := subscriberErrors(sema.AnalyzeAnonymous(index, body, api).Diagnostics)
						if status == "compile_error" {
							if len(errors) == 0 || errors[0] != expected {
								t.Fatalf("native first diagnostic %q, Glade %q", expected, errors)
							}
							return
						}
						if status != "pass" || expected != "ok" || len(errors) != 0 {
							t.Fatalf("native row %q, Glade %q", row, errors)
						}
						org := orgFromIndex(index)
						org.APIVersion = api
						program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
						if _, err := runner.execute(program); err != nil {
							t.Fatal(err)
						}
					})
				case "istest":
					class, method, ok := strings.Cut(name, ".")
					if !ok {
						t.Fatalf("invalid named test: %v", row)
					}
					t.Run(method, func(t *testing.T) {
						opts := Options{SelectedClasses: []string{class}, SelectedMethod: method, NoDiskCache: true, Parallelism: 1}
						run := Run(index, opts)
						if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
							t.Fatalf("named test summary %+v", run.Summary())
						}
						result := run.Suites[0].Cases[0]
						if string(result.Status) != status {
							t.Fatalf("native status %q, Glade %q: %+v", status, result.Status, result.Problem)
						}
						if status == "pass" {
							return
						}
						if result.Problem == nil {
							t.Fatal("missing Glade test problem")
						}
						gotType := strings.TrimPrefix(result.Problem.Type, "System.")
						gotMessage := trimContext218(result.Problem.Message)
						if gotType+": "+gotMessage != expected {
							t.Fatalf("native %q, Glade %q: %q", expected, gotType, gotMessage)
						}
					})
				case "deploy-checkonly":
					if status != "compile_error" {
						t.Fatalf("invalid deploy status %q", status)
					}
					if deployRows[id] == nil {
						deployRows[id] = map[string]string{}
					}
					deployRows[id][name] = strings.ReplaceAll(expected, `\n`, "\n")
				default:
					t.Fatalf("unknown route %q", route)
				}
			}
			for _, id := range []string{"direct", "call-root", "type-root"} {
				t.Run(id, func(t *testing.T) {
					expected := deployRows[id]
					if len(expected) != 2 {
						t.Fatalf("native deploy components %v", expected)
					}
					sources := map[string]string{}
					for class := range expected {
						sources[class] = deployClasses[class]
						if sources[class] == "" {
							t.Fatalf("missing source for %s", class)
						}
					}
					run := runCompileGateRow(t, api, sources, nil)
					if len(run.ExcludedClasses) != len(expected) {
						t.Fatalf("native exclusions %v, Glade %+v", expected, run.ExcludedClasses)
					}
					for _, excluded := range run.ExcludedClasses {
						if want := expected[excluded.ClassName]; excluded.Message != want {
							t.Errorf("%s: native %q, Glade %q", excluded.ClassName, want, excluded.Message)
						}
					}
					wantErrors := 1
					if id == "direct" {
						wantErrors = 2
					}
					if run.Summary().CompileErrors != wantErrors {
						t.Errorf("expected %d compile errors, Glade summary %+v", wantErrors, run.Summary())
					}
					for _, suite := range run.Suites {
						for _, result := range suite.Cases {
							want := expected[suite.Name]
							if result.Status != testreport.StatusCompileError || result.Problem == nil || result.Problem.Message != want {
								t.Errorf("%s.%s: native %q, Glade %s: %+v", suite.Name, result.MethodName, want, result.Status, result.Problem)
							}
						}
					}
				})
			}
		})
	}
}
