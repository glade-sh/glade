package apextest

import (
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

type formulaSummaryNativeCell struct {
	API    string          `json:"api"`
	Route  string          `json:"route"`
	Status string          `json:"status"`
	Value  json.RawMessage `json:"value"`
}

type formulaSummaryNativeRow struct {
	Label       string                     `json:"label"`
	Anonymous   string                     `json:"anonymous"`
	NamedSource string                     `json:"namedSource"`
	Cells       []formulaSummaryNativeCell `json:"cells"`
}

func TestFormulaSummaryObservationsConformance(t *testing.T) {
	var data struct {
		APIVersions            []string                  `json:"apiVersions"`
		Routes                 []string                  `json:"routes"`
		ShapeCount             int                       `json:"shapeCount"`
		NativeObservationCount int                       `json:"nativeObservationCount"`
		Objects                map[string]string         `json:"objects"`
		Rows                   []formulaSummaryNativeRow `json:"rows"`
	}
	raw, err := os.ReadFile("testdata/conformance/formula_summary_observations.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" || strings.Join(data.Routes, ",") != "anonymous,istest" || data.ShapeCount != 16 || len(data.Rows) != data.ShapeCount || data.NativeObservationCount != 96 || len(data.Objects) != 2 {
		t.Fatal("incomplete native formula/summary matrix")
	}
	labels, cells := map[string]bool{}, 0
	for _, row := range data.Rows {
		if row.Label == "" || labels[row.Label] || row.Anonymous == "" || row.NamedSource == "" || len(row.Cells) != 6 {
			t.Fatalf("invalid/repeated native shape %s", row.Label)
		}
		labels[row.Label] = true
		seen := map[string]bool{}
		for _, cell := range row.Cells {
			key := cell.API + "/" + cell.Route
			if seen[key] || (cell.API != "62.0" && cell.API != "67.0" && cell.API != "68.0") || (cell.Route != "anonymous" && cell.Route != "istest") || cell.Status != "runtime" || len(cell.Value) == 0 {
				t.Fatalf("invalid native cell %s/%s", row.Label, key)
			}
			seen[key] = true
			cells++
		}
	}
	if cells != data.NativeObservationCount {
		t.Fatalf("native observations: %d", cells)
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "FormulaSummaryObservations", api)
			for _, row := range data.Rows {
				for _, cell := range row.Cells {
					if cell.API != api {
						continue
					}
					route := cell.Route
					if route == "istest" {
						route = "isTest"
					}
					counts.Run(t, route, row.Label, row.Label+"/"+cell.Route, func(t *testing.T) {
						execErr := executeFormulaSummaryObservation(t, api, cell.Route, row, data.Objects)
						var runtimeErr *vm.RuntimeError
						if !errors.As(execErr, &runtimeErr) || listSObjectCastExceptionType(runtimeErr.Type) != "System.AssertException" {
							t.Fatalf("missing terminal native observation: %v", execErr)
						}
						marker := "P|" + row.Label + "|"
						message := runtimeErr.ExceptionMessage()
						_, payload, found := strings.Cut(message, marker)
						if !found || strings.Count(message, marker) != 1 {
							t.Fatalf("missing native marker: %v", execErr)
						}
						var expected, observed map[string]any
						if err := json.Unmarshal(cell.Value, &expected); err != nil {
							t.Fatal(err)
						}
						if err := json.Unmarshal([]byte(payload), &observed); err != nil {
							t.Fatalf("local observation %q: %v", payload, err)
						}
						// JSON object member ordering is transport framing. Every
						// observed field, value and null distinction stays exact.
						if !reflect.DeepEqual(expected, observed) {
							t.Fatalf("native=%s; local=%s", cell.Value, payload)
						}
					})
				}
			}
		})
	}
}

func executeFormulaSummaryObservation(t *testing.T, api, route string, row formulaSummaryNativeRow, objects map[string]string) error {
	t.Helper()
	classes := map[string]string{}
	if route == "istest" {
		classes["FormulaSummaryObservationSpec"] = row.NamedSource
	}
	index := nativeBoundaryIndex(t, api, classes, objects)
	analysis := sema.Analyze(index)
	if index.HasErrors() || analysis.HasErrors() {
		t.Fatalf("native metadata/source accepted; local diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
	}
	org := orgFromIndex(index)
	org.APIVersion = api
	runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
	if route == "anonymous" {
		anonymous := sema.AnalyzeAnonymous(index, row.Anonymous, api)
		program, compileErr := vm.CompileAnonymousWithOptions(row.Anonymous, vm.CompileOptions{APIVersion: api})
		if anonymous.HasErrors() || compileErr != nil {
			t.Fatalf("native anonymous executes; local diagnostics=%v compile=%v", anonymous.Diagnostics, compileErr)
		}
		_, execErr := runner.execute(program)
		return execErr
	}
	cases := Discover(index, Options{SelectedClasses: []string{"FormulaSummaryObservationSpec"}, SelectedMethod: "observe"})
	if len(cases) != 1 {
		t.Fatalf("named observation discovery: %d", len(cases))
	}
	methods, methodErrors := compileTestMethods(cases)
	programs, programErrors := compileTestInvokePrograms(cases)
	key := testCaseKey(cases[0])
	if methodErrors[key] != nil || programErrors[key] != nil {
		t.Fatalf("named compile: %v %v", methodErrors[key], programErrors[key])
	}
	machine := runner.newMachine()
	if err := machine.RegisterMethod(methods[key]); err != nil {
		t.Fatal(err)
	}
	machine.EnableTestContext()
	_, execErr := machine.ExecuteInClass(programs[key], cases[0].ClassName)
	return execErr
}
