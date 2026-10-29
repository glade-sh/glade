package apextest

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// These anonymous rows use deployed named companions, not anonymous-local
// classes. Their setup rejections and the @IsTest deployment answers retain
// every native diagnostic; compare direct errors, excluding dependent-class
// recompilation cascades, which are not separate local semantic errors.
func TestAbstractHierarchyOrgConformance(t *testing.T) {
	var data struct {
		APIVersions            []string `json:"apiVersions"`
		Routes                 []string `json:"routes"`
		ShapeCount             int      `json:"shapeCount"`
		NativeObservationCount int      `json:"nativeObservationCount"`
		Rows                   []struct {
			nativeBoundaryRoute
			Route         string `json:"route"`
			ExpectedByAPI map[string]struct {
				Outcome     string          `json:"outcome"`
				Value       json.RawMessage `json:"value"`
				Diagnostics []struct {
					Name, Problem string
				} `json:"diagnostics"`
			} `json:"expectedByAPI"`
		} `json:"rows"`
	}
	raw, err := os.ReadFile("testdata/conformance/abstract_hierarchy.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" || strings.Join(data.Routes, ",") != "anonymous,isTest" ||
		data.ShapeCount != 13 || data.NativeObservationCount != 78 || len(data.Rows) != 26 {
		t.Fatal("incomplete captured abstract hierarchy matrix")
	}
	seen := make(map[string]bool)
	for _, row := range data.Rows {
		key := row.ID + "/" + row.Route
		if !strings.HasPrefix(row.ID, "AB_") || seen[key] || len(row.ExpectedByAPI) != 3 ||
			(row.Route != "anonymous" && row.Route != "isTest") {
			t.Fatalf("invalid captured row %s", key)
		}
		seen[key] = true
		nativeBoundaryCheckSources(t, row.nativeBoundaryRoute)
	}
	for _, row := range data.Rows {
		for _, route := range data.Routes {
			if !seen[row.ID+"/"+route] {
				t.Fatalf("missing captured route %s/%s", row.ID, route)
			}
		}
	}
	for _, api := range data.APIVersions {
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "AbstractHierarchy", api)
			for _, row := range data.Rows {
				counts.Run(t, row.Route, row.ID, row.ID+"/"+row.Route, func(t *testing.T) {
					want, ok := row.ExpectedByAPI[api]
					if !ok {
						t.Fatal("missing captured API cell")
					}
					index := nativeBoundaryIndex(t, api, row.Classes, nil)
					analysis := sema.Analyze(index)
					diagnostics := append(append([]diagnostic.Diagnostic(nil), index.Diagnostics...), analysis.Diagnostics...)
					if want.Outcome == "compile-rejected" {
						expected := make(map[string]bool)
						for _, d := range want.Diagnostics {
							if strings.TrimSpace(d.Problem) == "" {
								t.Fatal("captured rejection has no message")
							}
							if !strings.HasPrefix(d.Problem, "Dependent class is invalid and needs recompilation:") {
								expected[d.Name+": "+d.Problem] = true
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
							actual[strings.TrimSuffix(filepath.Base(d.File), ".cls")+": "+message] = true
						}
						if len(expected) == 0 || !reflect.DeepEqual(expected, actual) {
							t.Fatalf("native direct rejections=%v; local=%v", expected, actual)
						}
						return
					}
					if want.Outcome != "runtime" || nativeObservationHasErrors(diagnostics) {
						t.Fatalf("native outcome=%s; local diagnostics=%v", want.Outcome, diagnostics)
					}
					org := orgFromIndex(index)
					org.APIVersion = api
					runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
					var execErr error
					if row.Route == "anonymous" {
						if row.Anonymous == "" || len(row.Tests) != 0 {
							t.Fatal("invalid captured anonymous invocation")
						}
						analysis := sema.AnalyzeAnonymous(index, row.Anonymous, api)
						program, compileErr := vm.CompileAnonymousWithOptions(row.Anonymous, vm.CompileOptions{APIVersion: api})
						if analysis.HasErrors() || compileErr != nil {
							t.Fatalf("anonymous compile: %v %v", analysis.Diagnostics, compileErr)
						}
						_, execErr = runner.execute(program)
					} else {
						if len(row.Tests) != 1 || row.Anonymous != "" {
							t.Fatal("invalid captured @IsTest invocation")
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
						machine := runner.newMachine()
						if err := machine.RegisterMethod(methods[key]); err != nil {
							t.Fatal(err)
						}
						machine.EnableTestContext()
						_, execErr = machine.ExecuteInClass(programs[key], cases[0].ClassName)
					}
					var runtimeErr *vm.RuntimeError
					if !errors.As(execErr, &runtimeErr) || runtimeErr.Type != "System.AssertException" {
						t.Fatalf("missing captured assertion transport: %v", execErr)
					}
					_, payload, found := strings.Cut(runtimeErr.ExceptionMessage(), "C135C_RESULT|"+row.ID+"|")
					var actual, expected any
					if !found || json.Unmarshal([]byte(payload), &actual) != nil || json.Unmarshal(want.Value, &expected) != nil || !reflect.DeepEqual(actual, expected) {
						t.Fatalf("native value=%s; local transport=%s", want.Value, runtimeErr.ExceptionMessage())
					}
				})
			}
		})
	}
}
