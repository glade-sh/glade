package apextest

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestCollectionCloneWritableNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/collection_clone_writable"
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			project := filepath.Join(fixture, "project")
			if err := filepath.WalkDir(project, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				relative, err := filepath.Rel(project, path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, relative), strings.ReplaceAll(string(body), "67.0", api))
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("project compile: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{Org: &org, LinkProject: true})
			triggers, failures := compileProjectTriggers(index)
			if len(failures) != 0 {
				t.Fatal(failures)
			}
			for _, trigger := range triggers {
				if err := runner.base.RegisterTrigger(trigger); err != nil {
					t.Fatal(err)
				}
			}
			cases := Discover(index, Options{SelectedClasses: []string{"CloneWriteProbeTest"}})
			methods, methodErrors := compileTestMethods(cases)
			programs, programErrors := compileTestInvokePrograms(cases)
			if len(cases) != 39 || len(methodErrors) != 0 || len(programErrors) != 0 {
				t.Fatalf("named compilation: cases=%d, methods=%v, programs=%v", len(cases), methodErrors, programErrors)
			}
			native := filepath.Join(fixture, "native", "api"+api[:2])
			run, err := os.ReadFile(filepath.Join(native, "RUN-FACTS.txt"))
			if err != nil || !strings.Contains(string(run), "source_api="+api+"\n") || !strings.Contains(string(run), "native_complete=true\n") {
				t.Fatalf("incomplete native capture: %v", err)
			}
			table, err := os.ReadFile(filepath.Join(native, "org.tsv"))
			if err != nil {
				t.Fatal(err)
			}
			rows := strings.Split(strings.TrimSpace(string(table)), "\n")
			if len(rows) != 69 || rows[0] != "source_api\troute\tmethod\tobservation" {
				t.Fatal("incomplete native table")
			}
			caseTable, err := os.ReadFile(filepath.Join(fixture, "cases.tsv"))
			if err != nil {
				t.Fatal(err)
			}
			expressions := map[string]string{}
			for _, line := range strings.Split(strings.TrimSpace(string(caseTable)), "\n")[1:] {
				columns := strings.SplitN(line, "\t", 3)
				if len(columns) != 3 {
					t.Fatal("invalid case table")
				}
				expressions[columns[0]+"/"+columns[1]] = columns[2]
			}
			if len(expressions) != 68 {
				t.Fatal("incomplete route table")
			}
			seen := map[string]bool{}
			for _, row := range rows[1:] {
				columns := strings.SplitN(row, "\t", 4)
				if len(columns) != 4 || columns[0] != api {
					t.Fatalf("invalid native row: %s", row)
				}
				route, name := columns[1], columns[2]
				key := route + "/" + name
				if seen[key] || expressions[key] == "" {
					t.Fatalf("unexpected native row: %s", key)
				}
				seen[key] = true
				t.Run(key, func(t *testing.T) {
					machine := runner.newMachine()
					var execErr error
					if route == "istest" {
						machine.EnableTestContext()
						for _, c := range cases {
							if c.MethodName != name {
								continue
							}
							caseKey := testCaseKey(c)
							if err := machine.RegisterMethod(methods[caseKey]); err != nil {
								t.Fatal(err)
							}
							_, execErr = machine.ExecuteInClass(programs[caseKey], c.ClassName)
						}
					} else if route == "anonymous" {
						body := "System.assert(false,'CLONE_OBS|'+JSON.serialize(" + expressions[key] + "));"
						if analysis := sema.AnalyzeAnonymous(index, body, api); analysis.HasErrors() {
							t.Fatal(analysis.Diagnostics)
						}
						program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						_, execErr = machine.Execute(program)
					} else {
						t.Fatalf("unknown route %s", route)
					}
					var fault *vm.RuntimeError
					if !errors.As(execErr, &fault) || !strings.HasSuffix(fault.Type, "AssertException") {
						t.Fatalf("missing observation: %v", execErr)
					}
					_, payload, found := strings.Cut(fault.ExceptionMessage(), "CLONE_OBS|")
					var got, want any
					if !found || json.Unmarshal([]byte(payload), &got) != nil || json.Unmarshal([]byte(columns[3]), &want) != nil {
						t.Fatalf("invalid observation: %s", fault.ExceptionMessage())
					}
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("native=%s; Glade=%s", columns[3], payload)
					}
				})
			}
		})
	}
}
