package apextest

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// NATIVE testdata/signed_double_zero_operations/org.tsv holds one row per case,
// API, and route. Each probe class declares its own API version; the anonymous
// route runs the probe directly and the istest route fails an assertion that
// carries the probe result.
func TestSignedDoubleZeroOperationsNative(t *testing.T) {
	dir := filepath.Join("testdata", "signed_double_zero_operations")
	data, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if lines[0] != "case\tapi\troute\tobservation" {
		t.Fatal("invalid native table")
	}
	for _, api := range []string{"62.0", "67.0"} {
		suffix := strings.TrimSuffix(api, ".0")
		classes := map[string]string{}
		entries, err := os.ReadDir(filepath.Join(dir, "classes"))
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			name := strings.TrimSuffix(entry.Name(), ".cls")
			if strings.HasPrefix(name, "ZP"+suffix) || strings.HasPrefix(name, "ZA"+suffix) || strings.HasPrefix(name, "ZT"+suffix) || strings.HasPrefix(name, "ZU"+suffix) {
				b, err := os.ReadFile(filepath.Join(dir, "classes", entry.Name()))
				if err != nil {
					t.Fatal(err)
				}
				classes[name] = string(b)
			}
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
				t.Fatalf("invalid row %q", line)
			}
			if row[1] != api {
				continue
			}
			id, route, want := row[0], row[2], row[3]
			t.Run(api+"/"+id+"/"+route, func(t *testing.T) {
				machine := runner.newMachine()
				call := probeClass(id, suffix) + "." + strings.SplitN(id, "_", 2)[0] + "()"
				var observation string
				switch route {
				case "anonymous":
					p, err := vm.CompileAnonymousWithOptions("String observed="+call+";", vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					_, err = machine.Execute(p)
					if err == nil {
						observation = machine.Globals["observed"].Text
					} else {
						observation = assertionActual(t, err)
					}
				case "istest":
					class := "ZT" + suffix + strings.SplitN(id, "_", 2)[0]
					if strings.HasPrefix(id, "d") {
						class = "ZU" + suffix + strings.SplitN(id, "_", 2)[0]
						want = strings.TrimPrefix(want, "after:")
					}
					cases := Discover(index, Options{SelectedClasses: []string{class}, SelectedMethod: "t"})
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
					observation = strings.TrimPrefix(assertionActual(t, err), "after:")
				default:
					t.Fatalf("invalid route %q", route)
				}
				if observation != strings.TrimPrefix(want, "after:") {
					t.Fatalf("got %s\nwant %s", observation, want)
				}
			})
		}
	}
}

func probeClass(id, suffix string) string {
	if strings.HasPrefix(id, "d") {
		return "ZA" + suffix
	}
	return "ZP" + suffix
}

func assertionActual(t *testing.T, err error) string {
	t.Helper()
	var fault *vm.RuntimeError
	if !errors.As(err, &fault) || !strings.HasSuffix(fault.Type, "AssertException") {
		t.Fatalf("expected assertion: %v", err)
	}
	return strings.TrimPrefix(fault.ExceptionMessage(), "Assertion Failed: ")
}
