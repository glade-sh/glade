package apextest

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// NATIVE testdata/exception_anonymous_frames/org.tsv rows 2–29.
func TestExceptionAnonymousFramesNative(t *testing.T) {
	dir := filepath.Join("testdata", "exception_anonymous_frames")
	data, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 29 || lines[0] != "api\troute\tcase\ttype\tmessage64\tcauseType\tcauseMessage64\tline\tstack64" {
		t.Fatal("invalid native table")
	}
	for _, api := range []string{"62.0", "67.0"} {
		suffix := strings.TrimSuffix(api, ".0")
		classes := map[string]string{}
		for _, prefix := range []string{"TraceLeaf", "TraceProbe", "TraceTest"} {
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
			if len(row) != 9 {
				t.Fatalf("invalid row %q", line)
			}
			if row[0] != api {
				continue
			}
			if row[1] != "anonymous" && row[1] != "istest" {
				t.Fatalf("invalid route %q", row[1])
			}
			t.Run(api+"/"+row[1]+"/"+row[2], func(t *testing.T) {
				machine := runner.newMachine()
				var observation string
				if row[1] == "anonymous" {
					source := fmt.Sprintf("String observed=TraceProbe%s.capture('%s');", suffix, row[2])
					program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if _, err := machine.Execute(program); err != nil {
						t.Fatal(err)
					}
					observation = machine.Globals["observed"].Text
				} else {
					cases := Discover(index, Options{SelectedClasses: []string{"TraceTest" + suffix}, SelectedMethod: "case_" + row[2]})
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
				}
				parts := strings.Split(observation, "|")
				got := map[string]string{}
				for _, part := range parts {
					k, v, ok := strings.Cut(part, ":")
					if ok {
						got[k] = v
					}
				}
				for _, tc := range []struct{ key, want string }{{"type", row[3]}, {"message", row[4]}, {"causeType", row[5]}, {"causeMessage", row[6]}, {"line", row[7]}, {"stack", row[8]}} {
					value := got[tc.key]
					if tc.key == "message" || tc.key == "causeMessage" || tc.key == "stack" {
						if value != "N" {
							value = strings.TrimPrefix(value, "B:")
						}
						if tc.want != "N" {
							decoded, err := base64.StdEncoding.DecodeString(value)
							if err != nil {
								t.Fatalf("%s base64: %v", tc.key, err)
							}
							value = string(decoded)
							b, err := base64.StdEncoding.DecodeString(tc.want)
							if err != nil {
								t.Fatal(err)
							}
							tc.want = string(b)
						}
					}
					if value != tc.want {
						t.Errorf("%s=%q, want %q", tc.key, value, tc.want)
					}
				}
			})
		}
	}
}
