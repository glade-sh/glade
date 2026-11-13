package apextest

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// NATIVE testdata/signed_double_zero_object_sort/org.tsv records Object-list
// sorting at APIs 62, 67, and 68 on anonymous and named test routes.
func TestSignedDoubleZeroObjectSortNative(t *testing.T) {
	dir := filepath.Join("testdata", "signed_double_zero_object_sort")
	data, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 19 || lines[0] != "case\tapi\troute\tobservation" {
		t.Fatal("invalid native table")
	}
	rowsRun := 0
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		suffix := strings.TrimSuffix(api, ".0")
		probe := "ZeroObjectSort" + suffix
		classes := map[string]string{}
		entries, err := os.ReadDir(filepath.Join(dir, "classes"))
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			if !strings.HasPrefix(entry.Name(), probe) || !strings.HasSuffix(entry.Name(), ".cls") {
				continue
			}
			b, err := os.ReadFile(filepath.Join(dir, "classes", entry.Name()))
			if err != nil {
				t.Fatal(err)
			}
			classes[strings.TrimSuffix(entry.Name(), ".cls")] = string(b)
		}
		index := nativeBoundaryIndex(t, api, classes, nil)
		if index.HasErrors() {
			t.Fatalf("index: %v", index.Diagnostics)
		}
		org := orgFromIndex(index)
		org.APIVersion = api
		runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
		for _, line := range lines[1:] {
			row := strings.Split(line, "\t")
			if len(row) != 4 {
				t.Fatalf("invalid native row %q", line)
			}
			if row[1] != api {
				continue
			}
			rowsRun++
			id, route, want := row[0], row[2], row[3]
			t.Run(api+"/"+id+"/"+route, func(t *testing.T) {
				machine := runner.newMachine()
				var executionError error
				switch route {
				case "anonymous":
					b, err := os.ReadFile(filepath.Join(dir, strings.ReplaceAll(id, "_", "")+".apex"))
					if err != nil {
						t.Fatal(err)
					}
					program, err := vm.CompileAnonymousWithOptions(string(b), vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					_, executionError = machine.Execute(program)
				case "istest":
					method := strings.SplitN(id, "_", 2)[0]
					class := probe + strings.ToUpper(method[:1]) + method[1:] + "Test"
					cases := Discover(index, Options{SelectedClasses: []string{class}, SelectedMethod: "observe"})
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
					_, executionError = machine.ExecuteInClass(programs[key], cases[0].ClassName)
				default:
					t.Fatalf("invalid route %q", route)
				}
				var fault *vm.RuntimeError
				if !errors.As(executionError, &fault) || !strings.HasSuffix(fault.Type, "AssertException") {
					t.Fatalf("expected observation assertion: %v", executionError)
				}
				observation := strings.TrimPrefix(fault.ExceptionMessage(), "Assertion Failed: ")
				if observation != want {
					t.Fatalf("got %s\nwant %s", observation, want)
				}
			})
		}
	}
	if rowsRun != 18 {
		t.Fatalf("replayed %d rows, want 18", rowsRun)
	}
}
