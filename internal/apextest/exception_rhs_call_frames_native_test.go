package apextest

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// NATIVE testdata/exception_rhs_call_frames/org.tsv:2-17 captures direct,
// one-line, split, and blank-line nested calls on both routes and APIs.
func TestExceptionRHSCallFramesNativeConformance(t *testing.T) {
	dir := filepath.Join("testdata", "exception_rhs_call_frames")
	data, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 17 || lines[0] != "id\tapi\troute\tstack_decoded" {
		t.Fatal("invalid native table")
	}
	for _, api := range []string{"62.0", "67.0"} {
		suffix := strings.TrimSuffix(api, ".0")
		entries, err := os.ReadDir(filepath.Join(dir, "classes"))
		if err != nil {
			t.Fatal(err)
		}
		classes := map[string]string{}
		for _, entry := range entries {
			name := strings.TrimSuffix(entry.Name(), ".cls")
			if !strings.HasSuffix(name, "_"+suffix) {
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
		for _, line := range lines[1:] {
			row := strings.Split(line, "\t")
			if len(row) != 4 {
				t.Fatal("invalid row")
			}
			if row[1] != api {
				continue
			}
			t.Run(api+"/"+row[0], func(t *testing.T) {
				want := strings.ReplaceAll(row[3], `\n`, "\n")
				var got []string
				if row[2] == "anonymous" {
					name := strings.TrimSuffix(row[0], "_anonymous")
					source, err := os.ReadFile(filepath.Join(dir, "anonymous", name+".apex"))
					if err != nil {
						t.Fatal(err)
					}
					code := strings.ReplaceAll(string(source), "FrameProbe", "FrameProbe_"+suffix)
					program, err := vm.CompileAnonymousWithOptions(code, vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					_, err = runner.newMachine().Execute(program)
					var fault *vm.RuntimeError
					if !errors.As(err, &fault) {
						t.Fatalf("expected exception: %v", err)
					}
					for _, frame := range fault.Stack {
						got = append(got, fmt.Sprintf("%s: line %d, column 1", frame.Symbol, frame.Line))
					}
				} else {
					id := strings.TrimSuffix(row[0], "_test")
					names := map[string]string{"direct": "Direct", "one_line": "OneLine", "split": "Split", "blank_nested": "BlankNested"}
					result := Run(index, Options{SelectedClasses: []string{"Frame" + names[id] + "Test_" + suffix}, NoDiskCache: true, Parallelism: 1})
					if len(result.Suites) != 1 || len(result.Suites[0].Cases) != 1 || result.Suites[0].Cases[0].Problem == nil {
						t.Fatalf("missing failure: %#v", result)
					}
					for _, frame := range result.Suites[0].Cases[0].Problem.Stack {
						got = append(got, fmt.Sprintf("%s: line %d, column 1", frame.Symbol, frame.Line))
					}
				}
				if strings.Join(got, "\n") != want {
					t.Errorf("stack:\n%s\nwant:\n%s", strings.Join(got, "\n"), want)
				}
			})
		}
	}
}
