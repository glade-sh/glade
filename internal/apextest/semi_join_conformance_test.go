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

type semiJoinExpected struct {
	Outcome string          `json:"outcome"`
	Value   json.RawMessage `json:"value"`
}

type semiJoinRoute struct {
	nativeBoundaryRoute
	ExpectedByAPI map[string]semiJoinExpected `json:"expectedByAPI"`
	Comparison    string                      `json:"comparison"`
	Carry         string                      `json:"carry"`
}

func TestSemiJoinTransportOrgConformance(t *testing.T) {
	runSemiJoinConformance(t, "transport", 1)
}

func TestSemiJoinControlOrgConformance(t *testing.T) {
	runSemiJoinConformance(t, "control", 6)
}

func TestSemiJoinQualifierOrgConformance(t *testing.T) {
	runSemiJoinConformance(t, "qualifier", 18)
}

func TestSemiJoinCustomOrgConformance(t *testing.T) {
	runSemiJoinConformance(t, "custom", 6)
}

func TestSemiJoinBooleanOrgConformance(t *testing.T) {
	runSemiJoinConformance(t, "boolean", 18)
}

func TestSemiJoinMultipleOrgConformance(t *testing.T) {
	runSemiJoinConformance(t, "multiple", 6)
}

func TestSemiJoinSubqueryOrgConformance(t *testing.T) {
	runSemiJoinConformance(t, "subquery", 10)
}

func TestSemiJoinWindowOrgConformance(t *testing.T) {
	runSemiJoinConformance(t, "window", 4)
}

// Captured Apex is retained byte for byte. Each cell uses the shared runner's
// isolated org. Transport controls compare only the marker and phase: native
// baseline record counts describe the capture org, not this isolated org.
func runSemiJoinConformance(t *testing.T, cluster string, shapes int) {
	t.Helper()
	var data struct {
		APIVersions            []string `json:"apiVersions"`
		Routes                 []string `json:"routes"`
		ShapeCount             int      `json:"shapeCount"`
		NativeObservationCount int      `json:"nativeObservationCount"`
		Rows                   []struct {
			ID     string                   `json:"id"`
			Routes map[string]semiJoinRoute `json:"routes"`
		} `json:"rows"`
	}
	raw, err := os.ReadFile(filepath.Join("testdata/conformance", "semi_join_"+cluster+".json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Rows) != shapes || data.ShapeCount != shapes || data.NativeObservationCount != shapes*6 || strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" || strings.Join(data.Routes, ",") != "anonymous,isTest" {
		t.Fatal("incomplete native matrix")
	}
	seen := map[string]bool{}
	for _, row := range data.Rows {
		if row.ID == "" || seen[row.ID] || len(row.Routes) != 2 {
			t.Fatalf("invalid/repeated native row: %s", row.ID)
		}
		seen[row.ID] = true
		for _, route := range data.Routes {
			source, ok := row.Routes[route]
			if !ok || source.ID != row.ID || len(source.ExpectedByAPI) != 3 {
				t.Fatalf("missing captured cells: %s/%s", row.ID, route)
			}
			nativeBoundaryCheckSources(t, source.nativeBoundaryRoute)
		}
	}
	for _, api := range data.APIVersions {
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "SemiJoin"+cluster, api)
			for _, row := range data.Rows {
				t.Run(row.ID, func(t *testing.T) {
					for _, route := range data.Routes {
						t.Run(route, func(t *testing.T) {
							source := row.Routes[route]
							want, ok := source.ExpectedByAPI[api]
							if !ok {
								t.Fatal("missing native API observation")
							}
							if source.Carry != "" {
								counts.record(route, "carried", 1, false)
								t.Logf("carried, not asserted: %s", source.Carry)
								return
							}
							kind := "exact"
							if source.Comparison == "transportOnly" {
								kind = "partial"
							}
							counts.TrackKind(t, route, kind, 1)
							semiJoinCell(t, api, route, source, want)
						})
					}
				})
			}
		})
	}
}

func semiJoinCell(t *testing.T, api, route string, source semiJoinRoute, want semiJoinExpected) {
	t.Helper()
	index := nativeBoundaryIndex(t, api, source.Classes, source.Objects)
	analysis := sema.Analyze(index)
	diagnostics := append(append([]diagnostic.Diagnostic{}, index.Diagnostics...), analysis.Diagnostics...)
	var execErr error
	if route == "anonymous" {
		if index.HasErrors() || analysis.HasErrors() {
			t.Fatalf("native metadata accepted; Glade rejected: %v", diagnostics)
		}
		anonymous := sema.AnalyzeAnonymous(index, source.Anonymous, api)
		program, compileErr := vm.CompileAnonymousWithOptions(source.Anonymous, vm.CompileOptions{APIVersion: api})
		if want.Outcome == "compile-rejected" {
			nativeBoundaryReject(t, source.nativeBoundaryRoute, api, anonymous.Diagnostics, compileErr)
			return
		}
		if anonymous.HasErrors() || compileErr != nil {
			t.Fatalf("native executes; anonymous diagnostics=%v compile=%v", anonymous.Diagnostics, compileErr)
		}
		org := orgFromIndex(index)
		org.APIVersion = api
		runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
		_, execErr = runner.execute(program)
	} else {
		if want.Outcome == "compile-rejected" {
			nativeBoundaryReject(t, source.nativeBoundaryRoute, api, diagnostics, nil)
			return
		}
		if index.HasErrors() || analysis.HasErrors() {
			t.Fatalf("native executes; named diagnostics=%v", diagnostics)
		}
		if len(source.Tests) != 1 {
			t.Fatal("one captured test class required")
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
		org := orgFromIndex(index)
		org.APIVersion = api
		runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
		machine := runner.newMachine()
		if err := machine.RegisterMethod(methods[key]); err != nil {
			t.Fatal(err)
		}
		machine.EnableTestContext()
		_, execErr = machine.ExecuteInClass(programs[key], cases[0].ClassName)
	}
	semiJoinValue(t, source, want, execErr)
}

func semiJoinValue(t *testing.T, source semiJoinRoute, want semiJoinExpected, execErr error) {
	t.Helper()
	if want.Outcome != "runtime" {
		t.Fatalf("unassertable native outcome requires carry: %s", want.Outcome)
	}
	var runtimeErr *vm.RuntimeError
	if execErr == nil || !errors.As(execErr, &runtimeErr) || runtimeErr.Type != "System.AssertException" {
		t.Fatalf("captured terminal assertion missing: %v", execErr)
	}
	message := runtimeErr.ExceptionMessage()
	marker := "C157_RESULT|" + source.ID + "|"
	_, tail, found := strings.Cut(message, marker)
	if !found || strings.Count(message, "C157_RESULT|") != 1 {
		t.Fatalf("invalid observation marker: %s", message)
	}
	var observed, expected map[string]any
	if err := json.Unmarshal([]byte(tail), &observed); err != nil || len(observed) == 0 {
		t.Fatalf("invalid transport JSON: %s", tail)
	}
	if err := json.Unmarshal(want.Value, &expected); err != nil || len(expected) == 0 {
		t.Fatal("invalid captured value")
	}
	if source.Comparison == "transportOnly" {
		if len(observed) != len(expected) || observed["exceptionType"] != nil || observed["exceptionMessage"] != nil {
			t.Fatalf("transport control failed: %s", tail)
		}
		for _, key := range []string{"accountBaseline", "contactBaseline"} {
			value, ok := observed[key].(float64)
			if !ok || value < 0 || value != float64(int64(value)) {
				t.Fatalf("invalid baseline count: %s", tail)
			}
			delete(observed, key)
			delete(expected, key)
		}
	} else if source.Comparison != "" {
		t.Fatalf("unknown comparison: %s", source.Comparison)
	}
	if !reflect.DeepEqual(observed, expected) {
		t.Fatalf("native value %s; Glade value %s", want.Value, tail)
	}
}
