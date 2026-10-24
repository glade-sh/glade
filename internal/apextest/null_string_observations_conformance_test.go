package apextest

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

type nullStringCell struct {
	API               string                    `json:"api"`
	Route             string                    `json:"route"`
	Status            string                    `json:"status"`
	Disposition       string                    `json:"disposition"`
	Observation       stringBoundaryObservation `json:"observation"`
	HelperDiagnostics json.RawMessage           `json:"helperDiagnostics"`
}

type nullStringRow struct {
	ID          string           `json:"id"`
	Apex        string           `json:"apex"`
	Helpers     []string         `json:"helpers"`
	Kind        string           `json:"kind"`
	CarryReason string           `json:"carryReason"`
	Cells       []nullStringCell `json:"cells"`
}

// Stable null, empty and text results replay on both routes. Reduced-visibility
// helper rejections remain explicit compiler carries; their anonymous fixture
// failures do not supply runtime answers. No general access rule is inferred.
func TestNullStringObservationsConformance(t *testing.T) {
	var data struct {
		APIVersions            []string          `json:"apiVersions"`
		Routes                 []string          `json:"routes"`
		ShapeCount             int               `json:"shapeCount"`
		PlannedCellCount       int               `json:"plannedCellCount"`
		NativeObservationCount int               `json:"nativeObservationCount"`
		ExactCellCount         int               `json:"exactCellCount"`
		PartialCellCount       int               `json:"partialCellCount"`
		CarriedCellCount       int               `json:"carriedCellCount"`
		UnansweredCellCount    int               `json:"unansweredCellCount"`
		HelperClasses          map[string]string `json:"helperClasses"`
		Rows                   []nullStringRow   `json:"rows"`
	}
	raw, err := os.ReadFile("testdata/conformance/null_string_observations.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" || strings.Join(data.Routes, ",") != "anonymous,istest" ||
		data.ShapeCount != 42 || len(data.Rows) != data.ShapeCount || len(data.HelperClasses) != 5 ||
		data.PlannedCellCount != 252 || data.NativeObservationCount != 246 || data.ExactCellCount != 234 ||
		data.PartialCellCount != 6 || data.CarriedCellCount != 12 || data.UnansweredCellCount != 6 {
		t.Fatal("incomplete native null-string matrix")
	}
	seen := map[string]bool{}
	counts := map[string]int{}
	cells, unanswered := 0, 0
	for _, row := range data.Rows {
		if row.ID == "" || seen[row.ID] || row.Apex == "" || len(row.Cells) != 6 {
			t.Fatalf("invalid or repeated row %s", row.ID)
		}
		seen[row.ID] = true
		for _, helper := range row.Helpers {
			if data.HelperClasses[helper] == "" {
				t.Fatalf("missing helper %s", helper)
			}
		}
		matrix := map[string]bool{}
		for _, cell := range row.Cells {
			key := cell.API + "/" + cell.Route
			if matrix[key] || !strings.Contains(",62.0,67.0,68.0,", ","+cell.API+",") ||
				(cell.Route != "anonymous" && cell.Route != "istest") || cell.Status != cell.Observation.Outcome {
				t.Fatalf("invalid native cell %s/%s", row.ID, key)
			}
			matrix[key] = true
			switch cell.Disposition {
			case "exact":
				if cell.Status != "runtime" && !strings.HasPrefix(cell.Status, "uncaught_") {
					t.Fatal("non-runtime answer admitted as exact")
				}
			case "partial":
				if cell.Status != "compile-rejected" || row.CarryReason == "" || len(cell.Observation.Diagnostics) == 0 {
					t.Fatal("compiler framing carry lacks native evidence")
				}
			case "carried":
				if row.CarryReason == "" {
					t.Fatal("missing carry reason")
				}
				if cell.Status == "FIXTURE_ERROR" {
					if len(cell.HelperDiagnostics) == 0 {
						t.Fatal("unanswered anonymous cell lacks helper rejection evidence")
					}
					unanswered++
				} else if cell.Status != "compile-rejected" || len(cell.Observation.Diagnostics) == 0 {
					t.Fatal("carry lacks its native compiler answer")
				}
			default:
				t.Fatal("unknown native-cell disposition")
			}
			counts[cell.Disposition]++
			cells++
		}
	}
	if cells != data.PlannedCellCount || cells-unanswered != data.NativeObservationCount || unanswered != data.UnansweredCellCount ||
		counts["exact"] != data.ExactCellCount || counts["partial"] != data.PartialCellCount || counts["carried"] != data.CarriedCellCount {
		t.Fatalf("native accounting changed: cells=%d unanswered=%d dispositions=%v", cells, unanswered, counts)
	}
	for _, api := range data.APIVersions {
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "NullStringObservations", api)
			for _, row := range data.Rows {
				for _, cell := range row.Cells {
					if cell.API != api {
						continue
					}
					route := cell.Route
					if route == "istest" {
						route = "isTest"
					}
					counts.RunKind(t, route, row.ID, row.ID+"/"+cell.Route, cell.Disposition, func(t *testing.T) {
						if cell.Disposition == "carried" {
							t.Logf("CARRIED %s; native outcome=%s diagnostics=%s helper rejection=%s", row.CarryReason,
								cell.Status, cell.Observation.Diagnostics, cell.HelperDiagnostics)
							return
						}
						classes := map[string]string{}
						for _, helper := range row.Helpers {
							classes[helper] = data.HelperClasses[helper]
						}
						body := nullStringObservationBody(row)
						if cell.Route == "istest" {
							classes["NullStringSpec"] = "@IsTest private class NullStringSpec {@IsTest static void observe(){" + body + "}}"
						}
						index := nativeBoundaryIndex(t, api, classes, nil)
						analysis := sema.Analyze(index)
						diagnostics := append(append([]diagnostic.Diagnostic{}, index.Diagnostics...), analysis.Diagnostics...)
						var execErr error
						if cell.Route == "anonymous" {
							if index.HasErrors() || analysis.HasErrors() {
								t.Fatalf("native helpers accepted: %v", diagnostics)
							}
							anonymous := sema.AnalyzeAnonymous(index, body, api)
							program, compileErr := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
							if cell.Disposition == "partial" {
								nullStringCompilerControl(t, row, cell, anonymous.Diagnostics, compileErr)
								return
							}
							if anonymous.HasErrors() || compileErr != nil {
								t.Fatalf("native executes; diagnostics=%v compile=%v", anonymous.Diagnostics, compileErr)
							}
							org := orgFromIndex(index)
							org.APIVersion = api
							_, execErr = newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org}).execute(program)
						} else {
							if cell.Disposition == "partial" {
								nullStringCompilerControl(t, row, cell, diagnostics, nil)
								return
							}
							if index.HasErrors() || analysis.HasErrors() {
								t.Fatalf("native executes; named diagnostics=%v", diagnostics)
							}
							cases := Discover(index, Options{SelectedClasses: []string{"NullStringSpec"}, SelectedMethod: "observe"})
							if len(cases) != 1 {
								t.Fatalf("named discovery: %d", len(cases))
							}
							methods, methodErrors := compileTestMethodsForIndex(&index, cases)
							programs, programErrors := compileTestInvokePrograms(cases)
							key := testCaseKey(cases[0])
							if methodErrors[key] != nil || programErrors[key] != nil {
								t.Fatalf("named compile: %v %v", methodErrors[key], programErrors[key])
							}
							org := orgFromIndex(index)
							org.APIVersion = api
							machine := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org}).newMachine()
							if err := machine.RegisterMethod(methods[key]); err != nil {
								t.Fatal(err)
							}
							machine.EnableTestContext()
							_, execErr = machine.ExecuteInClass(programs[key], cases[0].ClassName)
						}
						stringBoundaryValue(t, row.ID, cell.Observation, execErr)
					})
				}
			}
		})
	}
}

func nullStringObservationBody(row nullStringRow) string {
	if row.Kind == "uncaught" {
		return row.Apex
	}
	return "Map<String,Object> payload=new Map<String,Object>{'effectStarted'=>false,'readBackComplete'=>false,'exceptionType'=>null,'exceptionMessage'=>null};" +
		"try{payload.put('effectStarted',true);" + row.Apex +
		"}catch(Exception e){payload.put('exceptionType',e.getTypeName());payload.put('exceptionMessage',e.getMessage());}" +
		"System.assert(false,'P|" + row.ID + "|'+JSON.serialize(payload));"
}

func nullStringCompilerControl(t *testing.T, row nullStringRow, cell nullStringCell, diagnostics []diagnostic.Diagnostic, compileErr error) {
	t.Helper()
	stringBoundaryRejection(t, stringBoundaryRow{ID: row.ID, CarryReason: row.CarryReason},
		stringBoundaryCell{API: cell.API, Route: cell.Route, Status: cell.Status, Observation: cell.Observation}, diagnostics, compileErr)
}
