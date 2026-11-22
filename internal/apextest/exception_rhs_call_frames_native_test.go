package apextest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/vm"
)

// NATIVE testdata/exception_rhs_call_frames/org.tsv:2-25 captures direct,
// one-line, split, and blank-line nested calls on both routes and APIs.
func TestExceptionRHSCallFramesNativeConformance(t *testing.T) {
	t.Setenv("GLADE_HOME", t.TempDir())
	t.Cleanup(EnableDiskCacheForTesting())
	t.Cleanup(InvalidateRuntimeCaches)
	t.Cleanup(ResetPerfCounters)
	dir := filepath.Join("testdata", "exception_rhs_call_frames")
	caseData, err := os.ReadFile(filepath.Join(dir, "cases.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	caseFiles := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(caseData)), "\n")[1:] {
		parts := strings.Split(line, "\t")
		if len(parts) != 3 {
			t.Fatal("invalid case")
		}
		caseFiles[parts[0]] = parts[2]
	}
	data, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 25 || lines[0] != "id\tapi\troute\tstack_decoded" {
		t.Fatal("invalid native table")
	}
	for _, api := range []string{"62.0", "67.0", "68.0"} {
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
					if trace, ok := fault.CapturedAnonymousStackTrace(); !ok || trace != want {
						t.Errorf("production stack = %q (captured %t), want %q", trace, ok, want)
					}
					for _, frame := range fault.Stack {
						symbol := frame.Symbol
						if strings.Contains(symbol, ".") && !strings.HasPrefix(symbol, "Class.") {
							symbol = "Class." + symbol
						}
						got = append(got, fmt.Sprintf("%s: line %d, column 1", symbol, frame.Line))
					}
				} else {
					className := caseFiles[row[0]] + "_" + suffix
					InvalidateRuntimeCaches()
					result := Run(index, Options{SelectedClasses: []string{className}, Parallelism: 1})
					if len(result.Suites) != 1 || len(result.Suites[0].Cases) != 1 || result.Suites[0].Cases[0].Problem == nil {
						t.Fatalf("missing failure: %#v", result)
					}
					var report bytes.Buffer
					if err := testreport.WriteJSON(&report, result); err != nil {
						t.Fatal(err)
					}
					var decoded testreport.Run
					if err := json.Unmarshal(report.Bytes(), &decoded); err != nil {
						t.Fatal(err)
					}
					if len(decoded.Suites) != 1 || len(decoded.Suites[0].Cases) != 1 || decoded.Suites[0].Cases[0].Problem == nil {
						t.Fatalf("missing serialized problem: %s", report.String())
					}
					last := strings.Split(want, "\n")[1]
					_, location, ok := strings.Cut(last, ": line ")
					if !ok {
						t.Fatalf("invalid native stack %q", want)
					}
					lineText, _, ok := strings.Cut(location, ", column ")
					if !ok {
						t.Fatalf("invalid native location %q", location)
					}
					lineNumber, err := strconv.Atoi(lineText)
					if err != nil {
						t.Fatal(err)
					}
					probe := "FrameProbe_" + suffix
					classDir := filepath.Join(index.Project.Root, "force-app/main/default/classes")
					wantProblem := &testreport.Problem{
						Type:    probe + ".FrameException",
						Message: probe + ".FrameException:[__lineNumber=2, __stackTrace=" + want + ", __thrown=true]",
						Stack: []testreport.StackFrame{
							{Symbol: probe + ".fail", File: filepath.Join(classDir, probe+".cls"), Line: 2, Column: 36},
							{Symbol: className + ".runCase", File: filepath.Join(classDir, className+".cls"), Line: lineNumber, Column: 9},
						},
					}
					if gotProblem := decoded.Suites[0].Cases[0].Problem; !reflect.DeepEqual(gotProblem, wantProblem) {
						t.Errorf("serialized problem = %#v, want %#v", gotProblem, wantProblem)
					}
					InvalidateRuntimeCaches()
					if _, ok := tryLoadDiskRuntime(index); !ok {
						t.Fatal("runtime was not restored from disk")
					}
					ResetPerfCounters()
					cached := Run(index, Options{SelectedClasses: []string{className}, Parallelism: 1, PerfCounters: true})
					if SnapshotPerfCounters().Phases.DiskCacheHits != 1 || len(cached.Suites) != 1 || len(cached.Suites[0].Cases) != 1 || cached.Suites[0].Cases[0].Problem == nil {
						t.Fatal("cached execution did not restore the failure")
					}
					if fmt.Sprint(cached.Suites[0].Cases[0].Problem.Stack) != fmt.Sprint(result.Suites[0].Cases[0].Problem.Stack) {
						t.Fatal("cached stack differs")
					}
					if !reflect.DeepEqual(cached.Suites[0].Cases[0].Problem, result.Suites[0].Cases[0].Problem) {
						t.Fatal("cached diagnostic differs")
					}
					for _, frame := range result.Suites[0].Cases[0].Problem.Stack {
						symbol := frame.Symbol
						if strings.Contains(symbol, ".") && !strings.HasPrefix(symbol, "Class.") {
							symbol = "Class." + symbol
						}
						got = append(got, fmt.Sprintf("%s: line %d, column 1", symbol, frame.Line))
					}
				}
				if strings.Join(got, "\n") != want {
					t.Errorf("stack:\n%s\nwant:\n%s", strings.Join(got, "\n"), want)
				}
			})
		}
	}
}
