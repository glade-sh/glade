package apextest

import (
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

type sourceBoundaryRow struct {
	API, ID, Route, Source, ClassName string
	Classes, Objects, Triggers        map[string]string
	Native                            json.RawMessage
}

func sourceBoundaryRows(t *testing.T, group string) []sourceBoundaryRow {
	t.Helper()
	dir := "testdata/conformance/source_boundary/" + group + "/"
	raw, err := os.ReadFile(dir + "cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []sourceBoundaryRow
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	native, err := os.ReadFile(dir + "org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, row := range rows {
		key := row.API + "\t" + row.Route + "\t" + row.ID
		if seen[key] || (row.API != "62.0" && row.API != "67.0") {
			t.Fatal("duplicate or unsupported native row", key)
		}
		seen[key] = true
		bound := false
		for _, line := range strings.Split(string(native), "\n") {
			fields := strings.SplitN(line, "\t", 4)
			if len(fields) != 4 || strings.Join(fields[:3], "\t") != key {
				continue
			}
			var want, got any
			bound = json.Unmarshal([]byte(fields[3]), &want) == nil && json.Unmarshal(row.Native, &got) == nil && reflect.DeepEqual(want, got)
		}
		if !bound {
			t.Fatal("missing native answer", key)
		}
	}
	return rows
}

func TestMissingBindSourceNativeConformance(t *testing.T) {
	runSourceBoundaryNativeConformance(t, "bind", 24)
}
func TestSparseObjectSourceNativeConformance(t *testing.T) {
	runSourceBoundaryNativeConformance(t, "observations", 14)
}
func TestProjectionSourceNativeConformance(t *testing.T) {
	runSourceBoundaryNativeConformance(t, "projection", 4)
}

func TestSourceDefaultsNativeConformance(t *testing.T) {
	runSourceBoundaryNativeConformance(t, "defaults", 4)
}

func TestHeapGrowthSourceNativeConformance(t *testing.T) {
	runSourceBoundaryNativeConformance(t, "heap", 4)
}

func runSourceBoundaryNativeConformance(t *testing.T, group string, count int) {
	t.Helper()
	rows := sourceBoundaryRows(t, group)
	if len(rows) != count {
		t.Fatalf("native rows=%d, want %d", len(rows), count)
	}
	for _, row := range rows {
		t.Run(row.API+"/"+row.ID+"/"+row.Route, func(t *testing.T) {
			classes := map[string]string{}
			for name, source := range row.Classes {
				classes[name] = source
			}
			if row.Route != "anonymous" {
				classes[row.ClassName] = row.Source
			}
			index := nativeBoundaryIndex(t, row.API, classes, row.Objects)
			analysis := sema.Analyze(index)
			if row.Route == "anonymous" {
				analysis = sema.AnalyzeAnonymous(index, row.Source, row.API)
			}
			var native struct {
				Compiled                         bool
				CompileProblem, ExceptionMessage string
				Line, Column                     string
				Failures                         []struct {
					Problem                  string
					LineNumber, ColumnNumber int
				}
				Tests []struct{ MethodName, Outcome, Message string }
			}
			if row.Route == "isTest" && group == "observations" || row.Route == "queueable" {
				if err := json.Unmarshal(row.Native, &native.Tests); err != nil {
					t.Fatal(err)
				}
			} else if err := json.Unmarshal(row.Native, &native); err != nil {
				t.Fatal(err)
			}
			wantCompile := native.CompileProblem
			if len(native.Failures) > 0 {
				wantCompile = native.Failures[0].Problem
			}
			if wantCompile != "" {
				for _, d := range analysis.Diagnostics {
					if d.Severity == diagnostic.Error && (d.Message == wantCompile || d.NativeMessage == wantCompile) {
						if row.ID == "dynamic-map" {
							line, _ := strconv.Atoi(native.Line)
							column, _ := strconv.Atoi(native.Column)
							if len(native.Failures) > 0 {
								line, column = native.Failures[0].LineNumber, native.Failures[0].ColumnNumber
							}
							if d.Range == nil || d.Range.Start.Line != line || d.Range.Start.Column != column {
								t.Fatalf("native qualifier at %d:%d; Glade range=%+v", line, column, d.Range)
							}
						}
						return
					}
				}
				t.Fatalf("native compile rejection <%s>; Glade diagnostics=%v", wantCompile, analysis.Diagnostics)
			}
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native compiled; Glade diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
			}
			want := native.ExceptionMessage
			if len(native.Tests) > 0 {
				method := "observe"
				if group == "observations" {
					method = row.ID
				}
				for _, test := range native.Tests {
					if test.MethodName == method {
						want = test.Message
					}
				}
			}
			if want == "" {
				t.Fatal("missing native execution answer")
			}
			var got string
			if row.Route == "anonymous" {
				program, err := vm.CompileAnonymousWithOptions(row.Source, vm.CompileOptions{APIVersion: row.API})
				if err != nil {
					t.Fatal(err)
				}
				org := orgFromIndex(index)
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				_, err = runner.execute(program, func(machine *vm.VM) {
					if group == "heap" {
						machine.SetLimitMode(vm.LimitModeStrict)
					}
				})
				var runtimeErr *vm.RuntimeError
				if !errors.As(err, &runtimeErr) {
					t.Fatalf("native execution failed <%s>; Glade=%v", want, err)
				}
				got = runtimeErr.Type + ": " + runtimeErr.ExceptionMessage()
			} else {
				method := "observe"
				if group == "observations" {
					method = row.ID
				}
				mode := vm.LimitModePermissive
				if group == "heap" {
					mode = vm.LimitModeStrict
				}
				run := Run(index, Options{SelectedClasses: []string{row.ClassName}, SelectedMethod: method, LimitMode: mode, NoDiskCache: true, Parallelism: 1})
				if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
					t.Fatalf("native execution failed <%s>; Glade=%+v", want, run.Summary())
				}
				problem := run.Suites[0].Cases[0].Problem
				got = problem.Type + ": " + problem.Message
			}
			normalize := func(s string) string {
				if _, payload, found := strings.Cut(s, "OBS|"); found {
					return payload
				}
				if group == "heap" {
					return regexp.MustCompile(`Apex heap size too large: [0-9]+`).ReplaceAllString(s, "Apex heap size too large: <bytes>")
				}
				return s
			}
			// The VM exposes qualified exception types; the test report may omit System.
			got = strings.TrimPrefix(got, "System.")
			want = strings.TrimPrefix(want, "System.")
			observed, expected := normalize(got), normalize(want)
			var observedJSON, expectedJSON any
			if json.Unmarshal([]byte(observed), &observedJSON) == nil && json.Unmarshal([]byte(expected), &expectedJSON) == nil {
				if reflect.DeepEqual(observedJSON, expectedJSON) {
					return
				}
			} else if observed == expected {
				return
			}
			{
				t.Fatalf("native <%s>; Glade <%s>", want, got)
			}
		})
	}
}

func TestTriggerSourceNativeConformance(t *testing.T) {
	rows := sourceBoundaryRows(t, "trigger")
	if len(rows) != 8 {
		t.Fatalf("native trigger rows=%d, want 8", len(rows))
	}
	for _, row := range rows {
		t.Run(row.API+"/"+row.ID, func(t *testing.T) {
			var cell dependencyGateCell
			cell.API, cell.Classes, cell.Triggers = row.API, row.Classes, row.Triggers
			index := dependencyGateIndex(t, cell)
			gate := dependencyGateExclusions(t, index)
			var native struct {
				Failures []struct {
					FullName, ComponentType, Problem string
					LineNumber                       int
				}
			}
			if err := json.Unmarshal(row.Native, &native); err != nil {
				t.Fatal(err)
			}
			for name := range row.Triggers {
				want := ""
				for _, failure := range native.Failures {
					if failure.ComponentType == "ApexTrigger" && failure.FullName == name {
						want = failure.Problem
					}
				}
				entry, excluded := gate.classEntry[strings.ToLower(name)]
				if (want != "") != excluded {
					t.Fatalf("native trigger rejection <%s>; excluded=%t", want, excluded)
				}
				if excluded && gate.excluded[entry].Message != want {
					t.Fatalf("native trigger <%s>; Glade <%s>", want, gate.excluded[entry].Message)
				}
			}
			// The owned gate must keep errors local to source units.
			if gate.fellBack {
				t.Fatal("source error fell back to project-wide gate")
			}
		})
	}
}
