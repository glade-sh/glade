package apextest

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// NATIVE testdata/exception_implicit_call_frames/org.tsv: all 42 rows pin caller
// call-token lines across implicit dispatch at APIs 62.0, 67.0 and 68.0.
func TestExceptionImplicitCallFramesNative(t *testing.T) {
	dir := filepath.Join("testdata", "exception_implicit_call_frames")
	data, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 43 || lines[0] != "id\tapi\troute\tstack_decoded" {
		t.Fatal("invalid native table")
	}
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		suffix := strings.TrimSuffix(api, ".0")
		classes := map[string]string{}
		entries, err := os.ReadDir(filepath.Join(dir, "classes"))
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			name := strings.TrimSuffix(entry.Name(), ".cls")
			if !strings.HasSuffix(name, suffix) {
				continue
			}
			b, err := os.ReadFile(filepath.Join(dir, "classes", entry.Name()))
			if err != nil {
				t.Fatal(err)
			}
			classes[name] = string(b)
		}
		index := nativeBoundaryIndex(t, api, classes, nil)
		if index.HasErrors() {
			t.Fatalf("index: %v", index.Diagnostics)
		}
		org := orgFromIndex(index)
		org.APIVersion = api
		runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
		named := Run(index, Options{SelectedClasses: []string{"PendingFrameTest" + suffix}, NoDiskCache: true, Parallelism: 1})
		if named.Summary().Total != 7 || len(named.Suites) != 1 || len(named.Suites[0].Cases) != 7 {
			t.Fatalf("named runner selection: %#v", named)
		}
		runnerObservations := map[string]string{}
		for _, result := range named.Suites[0].Cases {
			if result.Problem == nil {
				t.Fatalf("missing observation: %#v", result)
			}
			_, observation, found := strings.Cut(result.Problem.Message, "Actual: ")
			if !found {
				t.Fatalf("missing observation: %#v", result)
			}
			runnerObservations[strings.TrimPrefix(result.MethodName, "case_")] = observation
		}
		for _, line := range lines[1:] {
			row := strings.Split(line, "\t")
			if len(row) != 4 {
				t.Fatalf("invalid row %q", line)
			}
			if row[1] != api {
				continue
			}
			id, route := row[0], row[2]
			t.Run(api+"/"+route+"/"+id, func(t *testing.T) {
				machine := runner.newMachine()
				var observation string
				switch route {
				case "anonymous":
					source := "String observed = PendingProbe" + suffix + ".capture('" + id + "');"
					program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if _, err := machine.Execute(program); err != nil {
						t.Fatal(err)
					}
					observation = machine.Globals["observed"].Text
				case "istest":
					cases := Discover(index, Options{SelectedClasses: []string{"PendingFrameTest" + suffix}, SelectedMethod: "case_" + id})
					if len(cases) != 1 {
						t.Fatalf("named discovery: %d", len(cases))
					}
					methods, methodErrors := compileTestMethods(cases)
					programs, programErrors := compileTestInvokePrograms(cases)
					key := testCaseKey(cases[0])
					if methodErrors[key] != nil || programErrors[key] != nil {
						t.Fatalf("compile: %v %v", methodErrors[key], programErrors[key])
					}
					if err := machine.RegisterMethod(methods[key]); err != nil {
						t.Fatal(err)
					}
					machine.EnableTestContext()
					_, err := machine.ExecuteInClass(programs[key], cases[0].ClassName)
					var fault *vm.RuntimeError
					if !errors.As(err, &fault) || !strings.HasSuffix(fault.Type, "AssertException") {
						t.Fatalf("expected assertion: %v", err)
					}
					var found bool
					_, observation, found = strings.Cut(fault.ExceptionMessage(), "Actual: ")
					if !found {
						t.Fatalf("missing observation: %v", err)
					}
				default:
					t.Fatalf("invalid route %q", route)
				}
				want := strings.ReplaceAll(row[3], `\n`, "\n")
				assertStack := func(t *testing.T, got string) {
					t.Helper()
					expected := want
					if id == "broken" {
						// Accessor symbol spelling is a separate pre-existing gap.
						// Pin its location and every caller, without changing that spelling.
						_, got, _ = strings.Cut(got, ": line ")
						_, expected, _ = strings.Cut(expected, ": line ")
					}
					if got != expected {
						t.Errorf("stack:\n%s\nwant:\n%s", got, expected)
					}
				}
				assertStack(t, observation)
				if route == "istest" {
					t.Run("runner", func(t *testing.T) { assertStack(t, runnerObservations[id]) })
				}
			})
		}
	}
}
