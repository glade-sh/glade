package apextest

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

type inheritedFinalTypeExpected struct {
	Outcome, ExceptionType, ExceptionMessage string
	Value, Diagnostics                       json.RawMessage
}

// The capture is sparse: nested sibling rows are named-only. Retain all 609
// observed cells without inventing anonymous counterparts. Owner/initializer
// neighbors outside constructor admission are explicit carries, not matches.
func TestInheritedFinalTypesOrgConformance(t *testing.T) {
	var data struct {
		APIVersions, Routes    []string
		RouteRowCounts         map[string]int
		NativeObservationCount int
		Rows                   []struct {
			nativeBoundaryRoute
			Route, Group, CarryReason, CarryOwner string
			ExpectedByAPI                         map[string]inheritedFinalTypeExpected
		}
	}
	raw, err := os.ReadFile("testdata/conformance/inherited_final_types.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" || strings.Join(data.Routes, ",") != "anonymous,isTest" ||
		data.NativeObservationCount != 609 || len(data.Rows) != 203 || data.RouteRowCounts["anonymous"] != 19 || data.RouteRowCounts["isTest"] != 184 {
		t.Fatal("incomplete native final-field capture")
	}
	seen, routes := make(map[string]bool), make(map[string]int)
	for _, row := range data.Rows {
		key := row.ID + "/" + row.Route
		if row.ID == "" || seen[key] || len(row.ExpectedByAPI) != 3 || row.Route != "anonymous" && row.Route != "isTest" {
			t.Fatalf("invalid native row %s", key)
		}
		if row.Stage == "carried" && (row.CarryReason == "" || row.CarryOwner == "") {
			t.Fatalf("carry without reason/owner: %s", key)
		}
		seen[key], routes[row.Route] = true, routes[row.Route]+1
		nativeBoundaryCheckSources(t, row.nativeBoundaryRoute)
	}
	if !reflect.DeepEqual(routes, data.RouteRowCounts) {
		t.Fatalf("route counts=%v; native=%v", routes, data.RouteRowCounts)
	}
	for _, api := range data.APIVersions {
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "InheritedFinalTypes", api)
			for _, row := range data.Rows {
				kind := "exact"
				if row.Stage == "carried" {
					kind = "carried"
				}
				counts.RunKind(t, row.Route, row.ID, row.ID+"/"+row.Route, kind, func(t *testing.T) {
					want, ok := row.ExpectedByAPI[api]
					if !ok {
						t.Fatal("missing observed API cell")
					}
					if row.Stage == "carried" {
						t.Logf("carried to %s: %s; native=%s", row.CarryOwner, row.CarryReason, want.Value)
						return
					}
					body := row.Anonymous
					classes := row.Classes
					if row.Route == "anonymous" {
						if body == "" || len(classes) != 0 {
							t.Fatal("invalid captured anonymous source")
						}
						classes = nativeBoundaryAnonymousDeclarations(t, body)
						bytes := []byte(body)
						for _, decl := range apexast.ParseSource("NativeFinalAnonymous.cls", body).Declarations {
							if decl.Kind != apexast.DeclarationClass && decl.Kind != apexast.DeclarationInterface && decl.Kind != apexast.DeclarationEnum {
								continue
							}
							for i := decl.Range.Start.Offset; i < decl.Range.End.Offset; i++ {
								if bytes[i] != '\n' && bytes[i] != '\r' {
									bytes[i] = ' '
								}
							}
						}
						body = string(bytes)
					}
					index := nativeBoundaryIndex(t, api, classes, nil)
					var diagnostics []diagnostic.Diagnostic
					if row.Route == "anonymous" {
						index = sema.WithAnonymousDeclarationContext(index)
						diagnostics = sema.NativeLifecycleDiagnostics(index, index.Diagnostics)
						if !nativeObservationHasErrors(diagnostics) {
							diagnostics = append(diagnostics, sema.AnalyzeAnonymousDeclarations(index).Diagnostics...)
						}
						if !nativeObservationHasErrors(diagnostics) {
							diagnostics = append(diagnostics, sema.AnalyzeAnonymous(index, body, api).Diagnostics...)
						}
					} else {
						diagnostics = append(append([]diagnostic.Diagnostic(nil), index.Diagnostics...), sema.Analyze(index).Diagnostics...)
					}
					if want.Outcome == "compile-rejected" {
						inheritedFinalTypeReject(t, row.Route, want.Diagnostics, diagnostics)
						return
					}
					if nativeObservationHasErrors(diagnostics) {
						t.Fatalf("native executes; local diagnostics=%v", diagnostics)
					}
					org := orgFromIndex(index)
					org.APIVersion = api
					var execErr error
					if row.Route == "anonymous" {
						runtime, err := CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
						if err != nil {
							t.Fatal(err)
						}
						program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api, ApprovedPrefixStatements: sema.ApprovedAnonymousPrefixStatements(index, body, api)})
						if err != nil {
							t.Fatal(err)
						}
						runner := newConformanceRunner(t, index, conformanceRunnerOptions{RequestRuntime: &runtime, Org: &org})
						_, execErr = runner.execute(program)
					} else {
						if len(row.Tests) != 1 {
							t.Fatal("missing captured test invocation")
						}
						cases := Discover(index, Options{SelectedClasses: row.Tests, SelectedMethod: "observe"})
						if len(cases) != 1 {
							t.Fatalf("captured method discovery: %v", cases)
						}
						methods, methodErrors := compileTestMethods(cases)
						programs, programErrors := compileTestInvokePrograms(cases)
						key := testCaseKey(cases[0])
						if methodErrors[key] != nil || programErrors[key] != nil {
							t.Fatalf("test compile: %v %v", methodErrors[key], programErrors[key])
						}
						runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
						machine := runner.newMachine()
						if err := machine.RegisterMethod(methods[key]); err != nil {
							t.Fatal(err)
						}
						machine.EnableTestContext()
						_, execErr = machine.ExecuteInClass(programs[key], cases[0].ClassName)
					}
					var runtimeErr *vm.RuntimeError
					if !errors.As(execErr, &runtimeErr) {
						t.Fatalf("missing native failure/transport: %v", execErr)
					}
					if want.Outcome == "uncaught_test_failure" || want.Outcome == "uncaught_anonymous" {
						qualified := *runtimeErr
						qualified.Type = flowRefsExceptionType(qualified.Type)
						if !nativeObservationFailureMatches(&qualified, want.ExceptionType, want.ExceptionMessage) {
							t.Fatalf("native failure=%s; local=%v", want.ExceptionMessage, execErr)
						}
						return
					}
					_, payload, found := strings.Cut(runtimeErr.ExceptionMessage(), "C142_RESULT|"+row.ID+"|")
					var actual, expected any
					if want.Outcome != "runtime" || runtimeErr.Type != "System.AssertException" || !found ||
						json.Unmarshal([]byte(payload), &actual) != nil || json.Unmarshal(want.Value, &expected) != nil || !reflect.DeepEqual(actual, expected) {
						t.Fatalf("native value=%s; local=%v", want.Value, execErr)
					}
				})
			}
		})
	}
}

func inheritedFinalTypeReject(t *testing.T, route string, raw json.RawMessage, diagnostics []diagnostic.Diagnostic) {
	t.Helper()
	expected := make(map[string]bool)
	if route == "anonymous" {
		var answer struct{ CompileProblem string }
		if err := json.Unmarshal(raw, &answer); err != nil || answer.CompileProblem == "" {
			t.Fatal("missing native anonymous rejection message")
		}
		expected[answer.CompileProblem] = true
	} else {
		var answers []struct{ Name, Problem string }
		if err := json.Unmarshal(raw, &answers); err != nil {
			t.Fatal(err)
		}
		for _, answer := range answers {
			if strings.TrimSpace(answer.Problem) == "" {
				t.Fatal("empty captured rejection message")
			}
			if !strings.HasPrefix(answer.Problem, "Dependent class is invalid and needs recompilation:") {
				expected[answer.Name+": "+answer.Problem] = true
			}
		}
	}
	actual := make(map[string]bool)
	for _, d := range diagnostics {
		if d.Severity != diagnostic.Error {
			continue
		}
		message := d.NativeMessage
		if message == "" {
			message = d.Message
		}
		if route == "anonymous" {
			// Execute-anonymous returns only its primary problem.
			actual[message] = true
			break
		}
		actual[strings.TrimSuffix(filepath.Base(d.File), ".cls")+": "+message] = true
	}
	if len(expected) == 0 || !reflect.DeepEqual(actual, expected) {
		t.Fatalf("native rejections=%v; local=%v", expected, actual)
	}
}
