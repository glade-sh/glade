package apextest

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestRuntimeInputNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/runtime_boundaries"
	file, err := os.Open(filepath.Join(fixture, "org.tsv"))
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
	if len(rows) != 17 {
		t.Fatalf("native matrix has %d rows; want header plus 16", len(rows))
	}
	for _, group := range []string{"trigger_prefix", "trigger_boundary", "null", "flow", "profile"} {
		t.Run(group, func(t *testing.T) {
			for _, api := range []string{"62.0", "67.0"} {
				t.Run(api, func(t *testing.T) {
					root := invocableFlowObservedProject(t, fixture)
					err := filepath.WalkDir(filepath.Join(root, "force-app"), func(path string, entry os.DirEntry, walkErr error) error {
						if walkErr != nil {
							return walkErr
						}
						if entry.IsDir() || !strings.HasSuffix(path, "-meta.xml") {
							return nil
						}
						raw, err := os.ReadFile(path)
						if err != nil {
							return err
						}
						return os.WriteFile(path, []byte(strings.ReplaceAll(string(raw), "<apiVersion>62.0</apiVersion>", "<apiVersion>"+api+"</apiVersion>")), 0600)
					})
					if err != nil {
						t.Fatal(err)
					}
					raw, err := os.ReadFile(filepath.Join(root, "sfdx-project.json"))
					if err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(root, "sfdx-project.json"), []byte(strings.ReplaceAll(string(raw), "62.0", api)), 0600); err != nil {
						t.Fatal(err)
					}
					index := loadTestIndex(t, root)
					analysis := sema.Analyze(index)
					if index.HasErrors() || analysis.HasErrors() {
						t.Fatalf("native setup accepted; diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
					}
					org := orgFromIndex(index)
					org.APIVersion = api
					runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
					triggers, failures := compileProjectTriggers(index)
					if len(failures) != 0 {
						t.Fatal(failures)
					}
					for _, trigger := range triggers {
						if err := runner.base.RegisterTrigger(trigger); err != nil {
							t.Fatal(err)
						}
					}
					kind, observer, method := group, "", ""
					if strings.HasPrefix(group, "trigger_") {
						kind = "trigger"
					}
					switch kind {
					case "trigger":
						observer, method = "observeTrigger", "triggerFrames"
					case "null":
						observer, method = "observeNull", "nullAssignment"
					case "flow":
						observer, method = "observeFlow", "flowDefaults"
					case "profile":
						observer, method = "observeProfile", "profileLicense"
					}
					for _, route := range []string{"anonymous", "istest"} {
						t.Run(route, func(t *testing.T) {
							var expected string
							for _, row := range rows[1:] {
								if len(row) == 4 && row[0] == api && row[1] == route && row[2] == kind {
									if expected != "" {
										t.Fatal("duplicate native cell")
									}
									expected = row[3]
								}
							}
							if expected == "" {
								t.Fatal("missing native cell")
							}
							machine := runner.newMachine()
							var result vm.Result
							var execErr error
							if route == "anonymous" {
								program, err := vm.CompileAnonymousWithOptions("RuntimeBoundaryProbe."+observer+"();", vm.CompileOptions{APIVersion: api})
								if err != nil {
									t.Fatal(err)
								}
								result, execErr = machine.Execute(program)
							} else {
								cases := Discover(index, Options{SelectedClasses: []string{"RuntimeBoundaryTest"}, SelectedMethod: method})
								if len(cases) != 1 {
									t.Fatalf("test discovery: %d", len(cases))
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
								result, execErr = machine.ExecuteInClass(programs[key], cases[0].ClassName)
							}
							marker := "RUNTIME_ROW|" + kind + "|"
							var observed string
							if route == "istest" {
								var runtimeErr *vm.RuntimeError
								if !errors.As(execErr, &runtimeErr) || runtimeErr.Type != "System.AssertException" {
									t.Fatalf("missing observation assertion: %v", execErr)
								}
								_, observed, _ = strings.Cut(runtimeErr.ExceptionMessage(), marker)
							} else {
								if execErr != nil {
									t.Fatal(execErr)
								}
								for _, line := range result.Debug {
									if _, payload, ok := strings.Cut(line, marker); ok {
										if observed != "" {
											t.Fatal("duplicate observation")
										}
										observed = payload
									}
								}
							}
							var got, want any
							if err := json.Unmarshal([]byte(observed), &got); err != nil {
								t.Fatalf("observation %q: %v", observed, err)
							}
							if err := json.Unmarshal([]byte(expected), &want); err != nil {
								t.Fatal(err)
							}
							if strings.HasPrefix(group, "trigger_") {
								gotTrace, wantTrace := got.(map[string]any), want.(map[string]any)
								for _, key := range []string{"direct", "helper"} {
									actual, native := strings.Split(gotTrace[key].(string), "\n"), strings.Split(wantTrace[key].(string), "\n")
									if group == "trigger_prefix" {
										at := 0
										if key == "helper" {
											at = 1
										}
										if len(actual) <= at || actual[at] != native[at] {
											t.Fatalf("%s native frame %q; Glade %q", key, native[at], actual)
										}
									} else if len(actual) != len(native) || (key == "helper" && actual[0] != native[0]) {
										t.Fatalf("%s native stack %q; Glade %q", key, native, actual)
									}
								}
							} else if !reflect.DeepEqual(got, want) {
								t.Fatalf("native %s; Glade %s", expected, observed)
							}
						})
					}
				})
			}
		})
	}
}
