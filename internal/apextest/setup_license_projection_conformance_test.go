package apextest

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// These are observations of the default Developer fixture, not a license
// capacity contract for arbitrary orgs. Supplied fixture values remain owned by
// the caller and are covered separately by the override regression.
func TestSetupLicenseProjectionOrgConformance(t *testing.T) {
	var data struct {
		APIVersions            []string `json:"apiVersions"`
		Routes                 []string `json:"routes"`
		ShapeCount             int      `json:"shapeCount"`
		NativeObservationCount int      `json:"nativeObservationCount"`
		Rows                   []struct {
			ID     string `json:"id"`
			Routes map[string]struct {
				nativeBoundaryRoute
				ExpectedByAPI map[string]nativeObservationExpected `json:"expectedByAPI"`
			} `json:"routes"`
		} `json:"rows"`
	}
	raw, err := os.ReadFile(filepath.Join("testdata/conformance", "setup_license_projection.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data.ShapeCount != 6 || len(data.Rows) != 6 || data.NativeObservationCount != 36 ||
		strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" || strings.Join(data.Routes, ",") != "anonymous,isTest" {
		t.Fatal("incomplete setup license observation matrix")
	}
	for i, row := range data.Rows {
		if row.ID != fmt.Sprintf("A%02d", i+1) || len(row.Routes) != 2 {
			t.Fatalf("unexpected setup license row: %s", row.ID)
		}
		for _, route := range data.Routes {
			source, ok := row.Routes[route]
			if !ok || source.ID != row.ID || source.Stage != "observed" || len(source.ExpectedByAPI) != 3 {
				t.Fatalf("incomplete setup license route: %s/%s", row.ID, route)
			}
			if route == "anonymous" && strings.TrimSpace(source.Anonymous) == "" {
				t.Fatal("missing anonymous measurement source")
			}
			if route == "isTest" && (len(source.Tests) != 1 || len(source.Classes) != 1 || strings.TrimSpace(source.Classes[source.Tests[0]]) == "") {
				t.Fatal("missing named measurement source")
			}
			for _, api := range data.APIVersions {
				want, ok := source.ExpectedByAPI[api]
				if !ok || want.Outcome != "runtime" || len(want.Value) == 0 {
					t.Fatalf("missing native value: %s/%s/%s", row.ID, route, api)
				}
			}
		}
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "SetupLicenseProjection", api)
			for _, row := range data.Rows {
				t.Run(row.ID, func(t *testing.T) {
					for _, route := range data.Routes {
						t.Run(route, func(t *testing.T) {
							counts.TrackKind(t, route, "exact", 1)
							source := row.Routes[route]
							setupLicenseProjectionObserve(t, api, route, source.nativeBoundaryRoute, source.ExpectedByAPI[api].Value)
						})
					}
				})
			}
		})
	}
}

func setupLicenseProjectionObserve(t *testing.T, api, route string, source nativeBoundaryRoute, want json.RawMessage) {
	t.Helper()
	index := nativeBoundaryIndex(t, api, source.Classes, nil)
	org := orgFromIndex(index)
	org.APIVersion = api
	runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
	machine := runner.newMachine()
	var execErr error
	if route == "anonymous" {
		diagnostics := sema.AnalyzeAnonymous(index, source.Anonymous, api).Diagnostics
		if nativeObservationHasErrors(diagnostics) {
			t.Fatalf("native executes; anonymous diagnostics=%v", diagnostics)
		}
		program, err := vm.CompileAnonymousWithOptions(source.Anonymous, vm.CompileOptions{APIVersion: api})
		if err != nil {
			t.Fatal(err)
		}
		_, execErr = machine.Execute(program)
	} else {
		analysis := sema.Analyze(index)
		if nativeObservationHasErrors(index.Diagnostics) || nativeObservationHasErrors(analysis.Diagnostics) {
			t.Fatalf("native executes; named diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
		}
		if len(source.Tests) != 1 {
			t.Fatal("one observer class required")
		}
		cases := Discover(index, Options{SelectedClasses: source.Tests, SelectedMethod: "observe"})
		if len(cases) != 1 {
			t.Fatalf("named observation discovery: %d", len(cases))
		}
		methods, methodErrors := compileTestMethods(cases)
		programs, programErrors := compileTestInvokePrograms(cases)
		key := testCaseKey(cases[0])
		if methodErrors[key] != nil || programErrors[key] != nil {
			t.Fatalf("named compile: %v %v", methodErrors[key], programErrors[key])
		}
		if err := machine.RegisterMethod(methods[key]); err != nil {
			t.Fatal(err)
		}
		machine.EnableTestContext()
		_, execErr = machine.ExecuteInClass(programs[key], cases[0].ClassName)
	}
	var runtimeErr *vm.RuntimeError
	if !errors.As(execErr, &runtimeErr) || runtimeErr.Type != "System.AssertException" {
		t.Fatalf("missing observation assertion: %v", execErr)
	}
	marker := "SETUP_LICENSE_RESULT|" + source.ID + "|"
	message := runtimeErr.ExceptionMessage()
	_, result, found := strings.Cut(message, marker)
	if !found || strings.Count(message, "SETUP_LICENSE_RESULT|") != 1 {
		t.Fatalf("invalid observation transport: %s", message)
	}
	var expected, observed map[string]any
	if err := json.Unmarshal(want, &expected); err != nil || len(expected) == 0 {
		t.Fatal("invalid native JSON value")
	}
	if err := json.Unmarshal([]byte(result), &observed); err != nil {
		t.Fatalf("invalid local observation: %s", result)
	}
	if !reflect.DeepEqual(expected, observed) {
		t.Fatalf("native value %s; Glade value %s", want, result)
	}
}
