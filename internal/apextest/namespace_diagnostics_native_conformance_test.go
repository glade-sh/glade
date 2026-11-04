package apextest

import (
	"encoding/csv"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestNamespaceDiagnosticsNativeConformance(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "namespace_diagnostics")
	read := func(name string) [][]string {
		f, err := os.Open(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		r := csv.NewReader(f)
		r.Comma = '\t'
		rows, err := r.ReadAll()
		if err != nil {
			t.Fatal(err)
		}
		return rows
	}
	sources := map[string]string{}
	for _, row := range read("cases.tsv")[1:] {
		sources[row[0]] = row[2]
	}
	rows := read("org.tsv")
	if len(rows) != 17 {
		t.Fatalf("incomplete native matrix: %d", len(rows)-1)
	}
	for _, row := range rows[1:] {
		row := row
		t.Run(strings.Join(row[:3], "/"), func(t *testing.T) {
			source, ok := sources[row[0]]
			if !ok {
				t.Fatal("missing case")
			}
			api, route, want := row[1], row[2], row[3]
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			if route == "istest" {
				path := filepath.Join(root, "force-app/classes/RoadNamespaceProbe.cls")
				writeFile(t, path, "@IsTest private class RoadNamespaceProbe { @IsTest static void observe() { "+source+" } }")
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
			}
			index := loadTestIndex(t, root)
			if route == "istest" {
				for _, warm := range []bool{false, true} {
					run := Run(index, Options{Parallelism: 1, SelectedClasses: []string{"RoadNamespaceProbe"}})
					summary := run.Summary()
					switch want {
					case "accepted":
						if summary.Passed != 1 || !summary.OK() {
							t.Fatalf("warm=%v native accepted, local=%+v", warm, summary)
						}
					case "compile-rejected":
						if summary.CompileErrors == 0 && summary.ExcludedClasses == 0 {
							t.Fatalf("warm=%v native compile rejection, local=%+v", warm, summary)
						}
					case "runtime-rejected":
						if summary.OK() || summary.Passed != 0 {
							t.Fatalf("warm=%v native assertion rejection, local=%+v", warm, summary)
						}
					default:
						t.Fatalf("unknown native outcome %q", want)
					}
				}
			}
			analysis := sema.Analyze(index)
			if route == "anonymous" {
				analysis = sema.AnalyzeAnonymous(index, source, api)
			}
			if want == "compile-rejected" {
				for _, d := range analysis.Diagnostics {
					if d.NativeMessage != "" && strings.HasSuffix(strings.TrimSpace(row[4]), d.NativeMessage) {
						return
					}
				}
				t.Fatalf("native compile diagnostic %q; local diagnostics=%v", row[4], analysis.Diagnostics)
			}
			if analysis.HasErrors() {
				t.Fatalf("native %s; local compile errors: %v", want, analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			var execErr error
			if route == "anonymous" {
				program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				_, execErr = runner.execute(program)
			} else {
				cases := Discover(index, Options{SelectedClasses: []string{"RoadNamespaceProbe"}, SelectedMethod: "observe"})
				if len(cases) != 1 {
					t.Fatalf("discover: %v", cases)
				}
				methods, methodErrors := compileTestMethods(cases)
				programs, programErrors := compileTestInvokePrograms(cases)
				key := testCaseKey(cases[0])
				if methodErrors[key] != nil || programErrors[key] != nil {
					t.Fatalf("compile: %v %v", methodErrors[key], programErrors[key])
				}
				machine := runner.newMachine()
				if err := machine.RegisterMethod(methods[key]); err != nil {
					t.Fatal(err)
				}
				machine.EnableTestContext()
				_, execErr = machine.ExecuteInClass(programs[key], cases[0].ClassName)
			}
			if want == "accepted" && execErr != nil {
				t.Fatalf("native passed; local error: %v", execErr)
			}
			if want == "runtime-rejected" {
				var fault *vm.RuntimeError
				_, nativeMessage, found := strings.Cut(row[4], "System.AssertException: ")
				if !found || !errors.As(execErr, &fault) || fault.Type != "System.AssertException" || fault.ExceptionMessage() != strings.TrimSpace(nativeMessage) {
					t.Fatalf("native assertion diagnostic %q; local error: %v", row[4], execErr)
				}
			}
		})
	}
}
