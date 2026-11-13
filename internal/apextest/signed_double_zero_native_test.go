package apextest

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// NATIVE testdata/signed_double_zero/org.tsv rows 2–5.
func TestSignedDoubleZeroNative(t *testing.T) {
	dir := filepath.Join("testdata", "signed_double_zero")
	data, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 5 || lines[0] != "api\troute\tobservation" {
		t.Fatal("invalid native table")
	}
	for _, api := range []string{"62.0", "67.0"} {
		suffix := strings.TrimSuffix(api, ".0")
		classes := map[string]string{}
		for _, prefix := range []string{"ZeroProbe", "ZeroTest"} {
			name := prefix + suffix
			b, err := os.ReadFile(filepath.Join(dir, "classes", name+".cls"))
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
		for _, line := range lines[1:] {
			row := strings.Split(line, "\t")
			if len(row) != 3 {
				t.Fatalf("invalid row %q", line)
			}
			if row[0] != api {
				continue
			}
			t.Run(api+"/"+row[1], func(t *testing.T) {
				machine := runner.newMachine()
				var observation string
				switch row[1] {
				case "anonymous":
					p, err := vm.CompileAnonymousWithOptions("String observed=ZeroProbe"+suffix+".probe();", vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if _, err := machine.Execute(p); err != nil {
						t.Fatal(err)
					}
					observation = machine.Globals["observed"].Text
				case "istest":
					cases := Discover(index, Options{SelectedClasses: []string{"ZeroTest" + suffix}, SelectedMethod: "zero"})
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
					t.Fatalf("invalid route %q", row[1])
				}
				if observation != row[2] {
					t.Fatalf("got %s\nwant %s", observation, row[2])
				}
			})
		}
	}
}
