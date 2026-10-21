package apextest

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// These rows exercise the inclusive query, child-query and DML statement caps.
// Empty parent results still consume the child-query budget; empty DML does not
// consume a statement. Only strict mode enforces native governor boundaries.
func TestGovernorEdgesNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/governor_edges"
	read := func(path string) string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(fixture, path))
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	source := read("sources/GovernorEdgeProbe.cls")
	wrapper := read("sources/GovernorEdgeProbeTest.cls")
	var cases []struct {
		ID, Shape string
		Count     int
	}
	if err := json.Unmarshal([]byte(read("cases.json")), &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) != 7 {
		t.Fatalf("native case count = %d", len(cases))
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			index := flowRefsIndex(t, api, map[string]string{"GovernorEdgeProbe": source, "GovernorEdgeProbeTest": wrapper}, nil)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native declarations accepted: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			tests := Discover(index, Options{SelectedClasses: []string{"GovernorEdgeProbeTest"}})
			if len(tests) != len(cases) {
				t.Fatalf("named method count = %d", len(tests))
			}
			methods, methodErrors := compileTestMethodsForIndex(&index, tests)
			programs, programErrors := compileTestInvokePrograms(tests)
			keys := make(map[string]string)
			for _, test := range tests {
				key := testCaseKey(test)
				if methodErrors[key] != nil || programErrors[key] != nil {
					t.Fatalf("named compilation: %v %v", methodErrors[key], programErrors[key])
				}
				keys[test.MethodName] = key
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			base := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			for _, route := range []string{"anonymous", "istest"} {
				t.Run(route, func(t *testing.T) {
					capture := "api" + strings.TrimSuffix(api, ".0") + "/" + route + "/"
					run := read(capture + "RUN-FACTS.txt")
					facts := []string{"api=" + api, "context=" + route, "rows=7", "native_complete=true", "deployment_success=true", "helper_class_api=" + api, fmt.Sprintf("helper_source_sha256=%x", sha256.Sum256([]byte(source)))}
					if route == "istest" {
						facts = append(facts, "wrapper_class_api="+api, fmt.Sprintf("wrapper_source_sha256=%x", sha256.Sum256([]byte(wrapper))))
					}
					for _, fact := range facts {
						if !strings.Contains("\n"+run, "\n"+fact+"\n") {
							t.Fatalf("missing capture fact %s", fact)
						}
					}
					rows := strings.Split(strings.TrimSpace(read(capture+"org.tsv")), "\n")
					if len(rows) != len(cases)+1 || rows[0] != "id\tshape\tcount\toutcome\tvalue" {
						t.Fatal("invalid native table")
					}
					for i, tc := range cases {
						t.Run(tc.ID, func(t *testing.T) {
							row := strings.Split(rows[i+1], "\t")
							if len(row) != 5 || row[0] != tc.ID || row[1] != tc.Shape || row[2] != strconv.Itoa(tc.Count) || (row[3] != "success" && row[3] != "exception") {
								t.Fatal("invalid native row")
							}
							invoke := fmt.Sprintf("GovernorEdgeProbe.observe('%s', %d);", tc.Shape, tc.Count)
							if route == "istest" {
								invoke = "GovernorEdgeProbeTest." + tc.ID + "();"
							}
							program, err := vm.CompileAnonymousWithOptions(invoke, vm.CompileOptions{APIVersion: api})
							if err != nil {
								t.Fatal(err)
							}
							machine := base.newMachine()
							machine.SetLimitMode(vm.LimitModeStrict)
							var result vm.Result
							if route == "istest" {
								machine.EnableTestContext()
								key := keys[tc.ID]
								if err := machine.RegisterMethod(methods[key]); err != nil {
									t.Fatal(err)
								}
								program = programs[key]
								result, err = machine.ExecuteTestInClass(program, "GovernorEdgeProbeTest")
							} else {
								result, err = machine.Execute(program)
							}
							got := "SUCCESS"
							if err != nil {
								var runtimeErr *vm.RuntimeError
								if !errors.As(err, &runtimeErr) {
									t.Fatal(err)
								}
								got = runtimeErr.Type + ": " + runtimeErr.ExceptionMessage()
								if route == "istest" && runtimeErr.Type == "System.AssertException" {
									got = strings.TrimPrefix(runtimeErr.ExceptionMessage(), "Assertion Failed: ")
								}
							} else if len(result.Debug) == 1 {
								got = result.Debug[0]
							}
							if got != row[4] {
								t.Fatalf("native %s; Glade %s", row[4], got)
							}
						})
					}
				})
			}
		})
	}
}
