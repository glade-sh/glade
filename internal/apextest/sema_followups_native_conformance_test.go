package apextest

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func semaFollowupRows(t *testing.T, path string, columns int) [][]string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma, r.FieldsPerRecord = '\t', columns
	rows, err := r.ReadAll()
	if err != nil || len(rows) < 2 {
		t.Fatalf("%s: rows=%d err=%v", path, len(rows), err)
	}
	return rows[1:]
}

func TestExceptionConstructorVisibilityNativeConformance(t *testing.T) {
	const dir = "../../testdata/conformance/exception_constructor_visibility"
	cases := map[string][]string{}
	for _, row := range semaFollowupRows(t, filepath.Join(dir, "cases.tsv"), 3) {
		cases[row[0]] = row[1:]
	}
	checked, anonymous, accepted, testMethods := 0, 0, 0, 0
	rows := semaFollowupRows(t, filepath.Join(dir, "org.tsv"), 5)
	for _, row := range rows {
		if row[2] == "isTest" {
			testMethods++
			t.Run(row[0]+"/isTest", func(t *testing.T) {
				classes := map[string]string{}
				for _, named := range rows {
					if named[0] != row[0] || named[2] != "named" || named[3] != "accepted" {
						continue
					}
					for _, name := range cases[named[1]] {
						source, err := os.ReadFile(filepath.Join(dir, "sources", name))
						if err != nil {
							t.Fatal(err)
						}
						classes[strings.TrimSuffix(name, ".cls")] = string(source)
					}
				}
				source, err := os.ReadFile(filepath.Join(dir, "VisibilityNativeTest.cls"))
				if err != nil {
					t.Fatal(err)
				}
				classes["VisibilityNativeTest"] = string(source)
				index := nativeBoundaryIndex(t, row[0], classes, nil)
				for _, warm := range []bool{false, true} {
					run := Run(index, Options{Parallelism: 1, SelectedClasses: []string{"VisibilityNativeTest"}})
					if summary := run.Summary(); summary.Passed != 1 || summary.CompileErrors != 0 || len(run.ExcludedClasses) != 0 {
						t.Fatalf("warm=%v %+v %+v %s", warm, summary, run.ExcludedClasses, firstRunProblem(run))
					}
				}
			})
			continue
		}
		if row[2] == "named" {
			checked++
			if row[3] == "accepted" {
				accepted++
			}
		} else {
			anonymous++
		}
		t.Run(strings.Join(row[:3], "/"), func(t *testing.T) {
			files, ok := cases[row[1]]
			if !ok {
				t.Fatalf("missing case %s", row[1])
			}
			classes := map[string]string{}
			for _, name := range files {
				source, err := os.ReadFile(filepath.Join(dir, "sources", name))
				if err != nil {
					t.Fatal(err)
				}
				classes[strings.TrimSuffix(name, ".cls")] = string(source)
			}
			child := strings.TrimSuffix(files[1], ".cls")
			args := ""
			if strings.Contains(row[1], "_custom") {
				args = "1"
			}
			if row[2] == "anonymous" {
				source := "Object value = new " + child + "(" + args + ");"
				index := nativeBoundaryIndex(t, row[0], classes, nil)
				if result := sema.AnalyzeAnonymous(index, source, row[0]); result.HasErrors() {
					t.Fatal(result.Diagnostics)
				}
				program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: row[0]})
				if err != nil {
					t.Fatal(err)
				}
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true})
				if _, err := runner.execute(program); err != nil {
					t.Fatal(err)
				}
				return
			}
			classes["VisibilityCaseTest"] = "@IsTest private class VisibilityCaseTest { @IsTest static void observe() { System.assertNotEquals(null, new " + child + "(" + args + ")); } }"
			index := nativeBoundaryIndex(t, row[0], classes, nil)
			for _, warm := range []bool{false, true} {
				run := Run(index, Options{Parallelism: 1, SelectedClasses: []string{"VisibilityCaseTest"}})
				summary := run.Summary()
				if row[3] == "accepted" {
					if summary.Passed != 1 || summary.CompileErrors != 0 || len(run.ExcludedClasses) != 0 {
						t.Fatalf("warm=%v native accepted: %+v %+v %s", warm, summary, run.ExcludedClasses, firstRunProblem(run))
					}
				} else if summary.CompileErrors != 1 || summary.RuntimeErrors != 0 || summary.Passed != 0 {
					t.Fatalf("warm=%v native rejected %s: %+v %+v %s", warm, row[4], summary, run.ExcludedClasses, firstRunProblem(run))
				}
			}
		})
	}
	if testMethods != 2 {
		t.Fatalf("native test-method coverage=%d want=2", testMethods)
	}
	if anonymous != accepted {
		t.Fatalf("anonymous coverage=%d want=%d", anonymous, accepted)
	}
	if checked != 2*len(cases) {
		t.Fatalf("coverage=%d want=%d", checked, 2*len(cases))
	}
}

func TestNamespaceStaticCallNativeConformance(t *testing.T) {
	const dir = "../../testdata/conformance/namespace_absence"
	cases := map[string]string{}
	for _, row := range semaFollowupRows(t, filepath.Join(dir, "native-cases.tsv"), 2) {
		source, err := os.ReadFile(filepath.Join(dir, row[1]))
		if err != nil {
			t.Fatal(err)
		}
		cases[row[0]] = string(source)
	}
	checked := 0
	for _, row := range semaFollowupRows(t, filepath.Join(dir, "org.tsv"), 5) {
		if row[2] == "anonymous" {
			continue
		} // The semantic conformance test checks this route.
		checked++
		t.Run(strings.Join(row[:3], "/"), func(t *testing.T) {
			source, ok := cases[row[1]]
			if !ok {
				t.Fatalf("missing case %s", row[1])
			}
			classes := map[string]string{}
			if row[2] == "named" {
				classes["QualifiedNativeProbe"] = "public class QualifiedNativeProbe { public static void observe() { " + source + " } }"
				source = "QualifiedNativeProbe.observe();"
			}
			classes["QualifiedNativeProbeTest"] = "@IsTest private class QualifiedNativeProbeTest { @IsTest static void observe() { " + source + " } }"
			classes["QualifiedBoundaryControlTest"] = "@IsTest private class QualifiedBoundaryControlTest { @IsTest static void observe() { System.assert(true); } }"
			index := nativeBoundaryIndex(t, row[0], classes, nil)
			for _, warm := range []bool{false, true} {
				run := Run(index, Options{Parallelism: 1})
				summary := run.Summary()
				wantPassed, wantCompile := 2, 0
				if row[3] == "rejected" {
					wantPassed, wantCompile = 1, 1
				}
				if summary.Passed != wantPassed || summary.CompileErrors != wantCompile || summary.RuntimeErrors != 0 || summary.Unsupported != 0 {
					t.Fatalf("warm=%v native=%s: %+v %+v %s", warm, row[3], summary, run.ExcludedClasses, firstRunProblem(run))
				}
			}
		})
	}
	if checked != 2*2*len(cases) {
		t.Fatalf("coverage=%d want=%d", checked, 2*2*len(cases))
	}
}
