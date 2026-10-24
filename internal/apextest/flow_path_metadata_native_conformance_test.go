package apextest

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

func TestFlowPathMetadataNativeConformanceAPIRoutes(t *testing.T) {
	cases := readPlatformTransactionCases(t)
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			anonymous := readPlatformTransactionNative(t, "flow_paths", api, "anonymous", cases)
			named := readPlatformTransactionNative(t, "flow_paths", api, "istest", cases)
			root := copyPlatformTransactionProject(t, "flow_paths", api)
			index := loadTestIndex(t, root)
			selected := Discover(index, Options{SelectedClasses: []string{"CapturePlatformProbeTest"}})
			var own []TestCase
			for _, row := range selected {
				if row.MethodName == "F01" || row.MethodName == "F02" || row.MethodName == "F03" || row.MethodName == "F04" {
					own = append(own, row)
				}
			}
			if len(own) != 4 {
				t.Fatalf("discovered %d path observation methods, want 4", len(own))
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			methods, methodErrors := compileTestMethodsForIndex(&index, own)
			programs, programErrors := compileTestInvokePrograms(own)
			template, err := os.ReadFile(filepath.Join(platformTransactionFixture, "anonymous.apex"))
			if err != nil {
				t.Fatal(err)
			}
			for _, row := range own {
				t.Run(row.MethodName, func(t *testing.T) {
					// Native rejects test-only start/stop calls on the anonymous
					// route before reaching a Flow. Their rejection remains in the
					// platform transaction matrix; exercise reachable paths here.
					if anonymous[row.MethodName]["uncaught"] != true {
						t.Run("anonymous", func(t *testing.T) {
							t.Run("metadata_snapshot", func(t *testing.T) { assertNativeFlowPathSnapshot(t, runner.newOrg()) })
							program, err := vm.CompileAnonymousWithOptions(strings.ReplaceAll(string(template), "CASE_ID", row.MethodName), vm.CompileOptions{APIVersion: api})
							if err != nil {
								t.Fatal(err)
							}
							_, err = runner.newMachine().Execute(program)
							assertPlatformTransactionObservation(t, row.MethodName, err, anonymous[row.MethodName])
						})
					}
					t.Run("named_istest", func(t *testing.T) {
						t.Run("metadata_snapshot", func(t *testing.T) { assertNativeFlowPathSnapshot(t, runner.newOrg()) })
						key := testCaseKey(row)
						if methodErrors[key] != nil || programErrors[key] != nil {
							t.Fatalf("compile: %v %v", methodErrors[key], programErrors[key])
						}
						machine := runner.newMachine()
						machine.EnableTestContext()
						if err := machine.RegisterMethod(methods[key]); err != nil {
							t.Fatal(err)
						}
						_, err := machine.ExecuteTestInClass(programs[key], row.ClassName)
						assertPlatformTransactionObservation(t, row.MethodName, err, named[row.MethodName])
					})
				})
			}
			t.Run("project_test_runner", func(t *testing.T) {
				t.Run("metadata_snapshot", func(t *testing.T) { assertNativeFlowPathSnapshot(t, org) })
				report := RunCasesContext(context.Background(), index, Options{SelectedClasses: []string{"CapturePlatformProbeTest"}, NoDiskCache: true, Parallelism: 1}, own)
				observed := 0
				for _, suite := range report.Suites {
					for _, result := range suite.Cases {
						observed++
						t.Run(result.MethodName, func(t *testing.T) {
							if result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
								t.Fatalf("native observation completed; project status=%s problem=%+v", result.Status, result.Problem)
							}
							assertPlatformTransactionPayload(t, result.MethodName, result.Problem.Message, named[result.MethodName])
						})
					}
				}
				if observed != len(own) {
					t.Fatalf("project observations=%d, want %d", observed, len(own))
				}
			})
		})
	}
}

func assertNativeFlowPathSnapshot(t *testing.T, org storage.OrgState) {
	t.Helper()
	rules := org.Objects["CaptureRecord__c"].Definition.FlowRules
	for _, want := range []struct{ name, kind string }{{"CaptureAsync", "async"}, {"CaptureScheduled", "scheduled"}} {
		found := false
		for _, rule := range rules {
			if rule.Name != want.name {
				continue
			}
			found = true
			encoded, err := json.Marshal(rule)
			if err != nil {
				t.Fatal(err)
			}
			var snapshot struct {
				Paths []struct {
					Name, Kind  string
					Steps       []storage.FlowStep
					StepTargets map[string][]storage.FlowStep
				}
			}
			if err := json.Unmarshal(encoded, &snapshot); err != nil {
				t.Fatal(err)
			}
			if len(snapshot.Paths) != 1 {
				t.Errorf("%s native-deployed %s path missing from project runtime snapshot: paths=%d", want.name, want.kind, len(snapshot.Paths))
				continue
			}
			path := snapshot.Paths[0]
			if path.Name != "DelayedPath" || path.Kind != want.kind || !storage.FlowStepsHaveReachableEffects(path.Steps, path.StepTargets, nil) {
				t.Errorf("%s lost path identity or connector graph: %+v", want.name, path)
			}
		}
		if !found {
			t.Errorf("native-deployed Flow %s missing from runtime snapshot", want.name)
		}
	}
}
