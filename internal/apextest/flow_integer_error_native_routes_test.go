package apextest

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func flowIntegerErrorProject(t *testing.T, api string) (string, map[string]any, [][]string) {
	t.Helper()
	root, _, _ := flowValueBoundaryProject(t, api)
	fixture := filepath.Join(flowValueBoundaryFixture, "integer_errors")
	directory := "api" + strings.TrimSuffix(api, ".0")
	read := func(path string) string {
		body, err := os.ReadFile(filepath.Join(fixture, path))
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}
	capture := read(directory + "/RUN-FACTS.txt")
	for _, fact := range []string{"api=" + api, "native_complete=true", "deployment_success=true"} {
		if !strings.Contains("\n"+capture, "\n"+fact+"\n") {
			t.Fatalf("missing native fact: %s", fact)
		}
	}
	var rows [][]string
	for _, line := range strings.Split(strings.TrimSpace(read("cases.tsv")), "\n")[1:] {
		cells := strings.Split(line, "\t")
		if len(cells) != 3 {
			t.Fatal("invalid case table")
		}
		rows = append(rows, cells)
	}
	want := map[string]any{}
	for _, line := range strings.Split(strings.TrimSpace(read(directory+"/org.tsv")), "\n")[1:] {
		cells := strings.SplitN(line, "\t", 3)
		if len(cells) != 3 {
			t.Fatal("invalid native row")
		}
		key := cells[0] + "/" + cells[1]
		if _, ok := want[key]; ok {
			t.Fatal("duplicate native row")
		}
		var value any
		if err := json.Unmarshal([]byte(cells[2]), &value); err != nil {
			t.Fatal(err)
		}
		want[key] = value
	}
	if len(rows) != 76 || len(want) != 2*len(rows) {
		t.Fatalf("incomplete native matrix: cases=%d rows=%d", len(rows), len(want))
	}
	sourceRoot := filepath.Join(fixture, "project")
	if err := filepath.WalkDir(sourceRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		body = []byte(strings.ReplaceAll(string(body), "62.0", api))
		rel, err := filepath.Rel(sourceRoot, path)
		if err != nil {
			return err
		}
		if !strings.Contains(capture, fmt.Sprintf("source_sha256:%s=%x\n", filepath.ToSlash(rel), sha256.Sum256(body))) {
			t.Fatalf("native source changed: %s", rel)
		}
		writeFile(t, filepath.Join(root, rel), string(body))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return root, want, rows
}

func TestFlowIntegerErrorsInterviewActionTriggerSubflowNativeRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root, want, rows := flowIntegerErrorProject(t, api)
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native accepted project; Glade rejected: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			cases := Discover(index, Options{SelectedClasses: []string{"FlowIntegerErrorProbeTest"}})
			if len(cases) != len(rows) {
				t.Fatalf("test count=%d", len(cases))
			}
			methods, methodErrors := compileTestMethodsForIndex(&index, cases)
			programs, programErrors := compileTestInvokePrograms(cases)
			byName := map[string]TestCase{}
			for _, c := range cases {
				byName[c.MethodName] = c
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			captureProject, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			if err := applyConfiguredTestPermissions(&org, captureProject); err != nil {
				t.Fatal(err)
			}
			base := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			for _, context := range []string{"anonymous", "istest"} {
				t.Run(context, func(t *testing.T) {
					for _, row := range rows {
						t.Run(row[0], func(t *testing.T) {
							machine := base.newMachine()
							var runErr error
							if context == "anonymous" {
								source := fmt.Sprintf("System.assert(false,'OBS|'+JSON.serialize(FlowIntegerErrorProbe.observe('%s','%s')));", row[1], row[2])
								analysis := sema.AnalyzeAnonymous(index, source, api)
								program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
								if err != nil || analysis.HasErrors() {
									t.Fatalf("compile: %v %v", err, analysis.Diagnostics)
								}
								_, runErr = machine.Execute(program)
							} else {
								c := byName[row[0]]
								key := testCaseKey(c)
								if methodErrors[key] != nil || programErrors[key] != nil {
									t.Fatalf("compile: %v %v", methodErrors[key], programErrors[key])
								}
								machine.EnableTestContext()
								if err := machine.RegisterMethod(methods[key]); err != nil {
									t.Fatal(err)
								}
								_, runErr = machine.ExecuteTestInClass(programs[key], c.ClassName)
							}
							var runtimeErr *vm.RuntimeError
							if !errors.As(runErr, &runtimeErr) || !strings.HasSuffix(runtimeErr.Type, "AssertException") {
								t.Fatalf("native observation completed; Glade: %v", runErr)
							}
							flowValueBoundaryCompare(t, runtimeErr.ExceptionMessage(), want[context+"/"+row[0]])
						})
					}
				})
			}
		})
	}
}

func TestFlowIntegerErrorsTestRunnerNativeRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root, want, rows := flowIntegerErrorProject(t, api)
			index := loadTestIndex(t, root)
			report := Run(index, Options{SelectedClasses: []string{"FlowIntegerErrorProbeTest"}, RuntimeRESTAPIVersion: "67.0", NoDiskCache: true, Parallelism: 1})
			count := 0
			for _, suite := range report.Suites {
				for _, c := range suite.Cases {
					count++
					t.Run(c.MethodName, func(t *testing.T) {
						if c.Problem == nil {
							t.Fatalf("missing observation: %+v", c)
						}
						flowValueBoundaryCompare(t, c.Problem.Message, want["istest/"+c.MethodName])
					})
				}
			}
			if count != len(rows) {
				t.Fatalf("runner cases=%d, want %d", count, len(rows))
			}
		})
	}
}
