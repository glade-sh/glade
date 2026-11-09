package apextest

import (
	"encoding/csv"
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

func TestFlowIntegerActionErrorAPI68NativeRoutes(t *testing.T) {
	root, _, cases := flowIntegerErrorProject(t, "67.0")
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		if !strings.HasSuffix(path, ".xml") && !strings.HasSuffix(path, ".json") {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(path, []byte(strings.ReplaceAll(string(body), "67.0", "68.0")), 0600)
	}); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(filepath.Join(flowValueBoundaryFixture, "integer_errors", "api68", "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{}
	for _, row := range rows[1:] {
		var v any
		if err := json.Unmarshal([]byte(row[2]), &v); err != nil {
			t.Fatal(err)
		}
		want[row[0]+"/"+row[1]] = v
	}
	if len(want) != 8 {
		t.Fatalf("native delta rows=%d", len(want))
	}
	index := loadTestIndex(t, root)
	if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
		t.Fatalf("native project rejected: %v %v", index.Diagnostics, analysis.Diagnostics)
	}
	tests := Discover(index, Options{SelectedClasses: []string{"FlowIntegerErrorProbeTest"}})
	methods, methodErrors := compileTestMethodsForIndex(&index, tests)
	programs, programErrors := compileTestInvokePrograms(tests)
	byName := map[string]TestCase{}
	for _, c := range tests {
		byName[c.MethodName] = c
	}
	org := orgFromIndex(index)
	org.APIVersion = "68.0"
	config, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := applyConfiguredTestPermissions(&org, config); err != nil {
		t.Fatal(err)
	}
	base := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
	for _, row := range cases {
		for _, route := range []string{"anonymous", "istest"} {
			expected, present := want[route+"/"+row[0]]
			if !present {
				continue
			}
			t.Run(route+"/"+row[0], func(t *testing.T) {
				machine := base.newMachine()
				var runErr error
				if route == "anonymous" {
					source := fmt.Sprintf("System.assert(false,'OBS|'+JSON.serialize(FlowIntegerErrorProbe.observe('%s','%s')));", row[1], row[2])
					analysis := sema.AnalyzeAnonymous(index, source, "68.0")
					program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: "68.0"})
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
				var native *vm.RuntimeError
				if !errors.As(runErr, &native) || !strings.HasSuffix(native.Type, "AssertException") {
					t.Fatalf("native observation completed; Glade: %v", runErr)
				}
				flowValueBoundaryCompare(t, native.ExceptionMessage(), expected)
			})
		}
	}
}

func TestFlowIntegerActionErrorMixedVersions(t *testing.T) {
	file, err := os.Open(filepath.Join(flowValueBoundaryFixture, "integer_errors", "api68", "cross-versions.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 33 {
		t.Fatalf("mixed native rows=%d", len(rows)-1)
	}
	for _, row := range rows[1:] {
		if len(row) != 8 || row[2] != "68.0" || row[3] != "62.0" || row[4] != "62.0" {
			t.Fatalf("unexpected observer, callee or test source versions: %v", row)
		}
	}
	for _, version := range []struct{ flow, rest string }{{"62.0", "68.0"}, {"68.0", "62.0"}, {"68.0", "67.0"}, {"68.0", "68.0"}} {
		t.Run("flow="+version.flow+"/rest="+version.rest, func(t *testing.T) {
			root, _, cases := flowIntegerErrorProject(t, "67.0")
			if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				if !strings.HasSuffix(path, ".xml") && !strings.HasSuffix(path, ".json") {
					return nil
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				api := "68.0"
				if strings.HasSuffix(path, ".flow-meta.xml") {
					api = version.flow
				}
				if strings.HasSuffix(path, "FlowIntegerProbe.cls-meta.xml") || strings.HasSuffix(path, "FlowIntegerErrorProbeTest.cls-meta.xml") {
					api = "62.0"
				}
				return os.WriteFile(path, []byte(strings.ReplaceAll(string(body), "67.0", api)), 0600)
			}); err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native project rejected: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			tests := Discover(index, Options{SelectedClasses: []string{"FlowIntegerErrorProbeTest"}})
			methods, methodErrors := compileTestMethodsForIndex(&index, tests)
			programs, programErrors := compileTestInvokePrograms(tests)
			byName := map[string]TestCase{}
			for _, c := range tests {
				byName[c.MethodName] = c
			}
			org := orgFromIndex(index)
			org.APIVersion = version.rest
			config, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			if err := applyConfiguredTestPermissions(&org, config); err != nil {
				t.Fatal(err)
			}
			base := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			check := func(t *testing.T, message, code string) {
				t.Helper()
				_, payload, ok := strings.Cut(message, "OBS|")
				var value struct {
					Code string `json:"actionErrorCode"`
				}
				if !ok || json.Unmarshal([]byte(payload), &value) != nil {
					t.Fatalf("missing native observation: %s", message)
				}
				if value.Code != code {
					t.Fatalf("native actionErrorCode=%s; Glade=%s", code, value.Code)
				}
			}
			for _, native := range rows[1:] {
				if native[0] != version.flow || native[1] != version.rest {
					continue
				}
				route, name, code := native[5], native[6], native[7]
				var input []string
				for _, c := range cases {
					if c[0] == name {
						input = c
						break
					}
				}
				if len(input) != 3 {
					t.Fatalf("missing case %s", name)
				}
				t.Run(route+"/"+name, func(t *testing.T) {
					machine := base.newMachine()
					var runErr error
					if route == "anonymous" {
						source := fmt.Sprintf("System.assert(false,'OBS|'+JSON.serialize(FlowIntegerErrorProbe.observe('%s','%s')));", input[1], input[2])
						// The named observer class is source API 68; Salesforce compiles
						// the outer anonymous block at the request API used in the capture.
						analysis := sema.AnalyzeAnonymous(index, source, version.rest)
						program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: version.rest})
						if err != nil || analysis.HasErrors() {
							t.Fatalf("compile: %v %v", err, analysis.Diagnostics)
						}
						_, runErr = machine.Execute(program)
					} else {
						c := byName[name]
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
					check(t, runtimeErr.ExceptionMessage(), code)
				})
			}
			report := Run(index, Options{SelectedClasses: []string{"FlowIntegerErrorProbeTest"}, SelectedMethod: "actionFraction", RuntimeRESTAPIVersion: version.rest, NoDiskCache: true, Parallelism: 1})
			count := 0
			for _, suite := range report.Suites {
				for _, c := range suite.Cases {
					count++
					if c.Problem == nil {
						t.Fatalf("missing runner observation: %+v", c)
					}
					if version.rest == "62.0" {
						if !strings.Contains(c.Problem.Message, `unsupported REST API version "62.0"`) {
							t.Fatalf("expected runner REST boundary: %s", c.Problem.Message)
						}
						continue
					}
					expected := "UNKNOWN_EXCEPTION"
					if version.flow == "68.0" && version.rest == "68.0" {
						expected = "INVOCABLE_ACTION_EXCEPTION"
					}
					check(t, c.Problem.Message, expected)
				}
			}
			if count != 1 {
				t.Fatalf("runner cases=%d", count)
			}
		})
	}
}
