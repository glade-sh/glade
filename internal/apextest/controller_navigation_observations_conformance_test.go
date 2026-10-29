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

type controllerNavigationCell struct {
	API         string          `json:"api"`
	Route       string          `json:"route"`
	Status      string          `json:"status"`
	Native      json.RawMessage `json:"native"`
	CarryReason string          `json:"carryReason"`
}

type controllerNavigationRow struct {
	ID    string                     `json:"id"`
	Apex  string                     `json:"apex"`
	Cells []controllerNavigationCell `json:"cells"`
}

// Replay captured cancel/view behavior on both routes. Each observation checks
// the complete URL's prefix, width and binding to the input record; generated
// record IDs differ per run. Uncaptured action paths remain separate controls.
func TestControllerNavigationObservationsConformance(t *testing.T) {
	var data struct {
		APIVersions            []string                  `json:"apiVersions"`
		Routes                 []string                  `json:"routes"`
		ShapeCount             int                       `json:"shapeCount"`
		NativeObservationCount int                       `json:"nativeObservationCount"`
		Rows                   []controllerNavigationRow `json:"rows"`
		UncapturedControls     []struct {
			ID     string `json:"id"`
			Reason string `json:"reason"`
		} `json:"uncapturedControls"`
	}
	raw, err := os.ReadFile("testdata/conformance/controller_navigation_observations.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" || strings.Join(data.Routes, ",") != "anonymous,istest" || data.ShapeCount != 8 || len(data.Rows) != 8 || data.NativeObservationCount != 48 {
		t.Fatal("incomplete navigation source/API/route matrix")
	}
	seen := map[string]bool{}
	cells := 0
	for _, row := range data.Rows {
		if row.ID == "" || seen[row.ID] || row.Apex == "" || len(row.Cells) != 6 {
			t.Fatalf("invalid navigation shape %s", row.ID)
		}
		seen[row.ID] = true
		matrix := map[string]bool{}
		for _, cell := range row.Cells {
			key := cell.API + "/" + cell.Route
			if matrix[key] || !strings.Contains(",62.0,67.0,68.0,", ","+cell.API+",") || (cell.Route != "anonymous" && cell.Route != "istest") || len(cell.Native) == 0 {
				t.Fatalf("invalid navigation cell %s/%s", row.ID, key)
			}
			matrix[key] = true
			if cell.Status != "runtime" && cell.CarryReason == "" {
				t.Fatalf("unsettled navigation cell needs native data and a carry reason: %s/%s", row.ID, key)
			}
			cells++
		}
	}
	if cells != data.NativeObservationCount {
		t.Fatalf("native navigation cells: %d", cells)
	}
	for _, control := range data.UncapturedControls {
		if control.ID == "" || control.Reason == "" {
			t.Fatal("uncaptured control needs its scope and reason")
		}
		t.Logf("PENDING %s: %s", control.ID, control.Reason)
	}
	for _, api := range data.APIVersions {
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "ControllerNavigation", api)
			for _, row := range data.Rows {
				for _, cell := range row.Cells {
					if cell.API != api {
						continue
					}
					kind := "exact"
					if cell.Status != "runtime" || cell.CarryReason != "" {
						kind = "carried"
					}
					countRoute := cell.Route
					if countRoute == "istest" {
						countRoute = "isTest"
					}
					counts.RunKind(t, countRoute, row.ID, row.ID+"/"+cell.Route, kind, func(t *testing.T) {
						if kind == "carried" {
							if cell.CarryReason == "" {
								t.Fatal("carry requires a reason")
							}
							t.Logf("CARRIED %s; native=%s", cell.CarryReason, cell.Native)
							return
						}
						body := "Map<String,Object> payload=new Map<String,Object>();" + row.Apex + "payload.put('readBackComplete',true);payload.put('stage','effect');System.assert(false,'P|" + row.ID + "|'+JSON.serialize(payload));"
						classes := map[string]string{}
						if cell.Route == "istest" {
							classes["ControllerNavigationSpec"] = "@IsTest private class ControllerNavigationSpec {@IsTest static void observe(){" + body + "}}"
						}
						index := nativeBoundaryIndex(t, api, classes, nil)
						analysis := sema.Analyze(index)
						if index.HasErrors() || analysis.HasErrors() {
							t.Fatalf("native source accepted; local diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
						}
						org := orgFromIndex(index)
						org.APIVersion = api
						runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
						var execErr error
						if cell.Route == "anonymous" {
							anonymous := sema.AnalyzeAnonymous(index, body, api)
							program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
							if anonymous.HasErrors() || err != nil {
								t.Fatalf("anonymous compile: %v %v", anonymous.Diagnostics, err)
							}
							_, execErr = runner.execute(program)
						} else {
							cases := Discover(index, Options{SelectedClasses: []string{"ControllerNavigationSpec"}, SelectedMethod: "observe"})
							if len(cases) != 1 {
								t.Fatalf("named navigation discovery: %d", len(cases))
							}
							methods, methodErrors := compileTestMethods(cases)
							programs, programErrors := compileTestInvokePrograms(cases)
							key := testCaseKey(cases[0])
							if methodErrors[key] != nil || programErrors[key] != nil {
								t.Fatalf("named navigation compile: %v %v", methodErrors[key], programErrors[key])
							}
							machine := runner.newMachine()
							if err := machine.RegisterMethod(methods[key]); err != nil {
								t.Fatal(err)
							}
							machine.EnableTestContext()
							_, execErr = machine.ExecuteInClass(programs[key], cases[0].ClassName)
						}
						controllerNavigationValue(t, row.ID, cell.Native, execErr)
					})
				}
			}
		})
	}
}

func controllerNavigationValue(t *testing.T, id string, native json.RawMessage, execErr error) {
	t.Helper()
	var runtimeErr *vm.RuntimeError
	if !errors.As(execErr, &runtimeErr) || runtimeErr.Type != "System.AssertException" {
		t.Fatalf("missing navigation observation: %v", execErr)
	}
	_, payload, found := strings.Cut(runtimeErr.ExceptionMessage(), "P|"+id+"|")
	if !found {
		t.Fatalf("missing navigation marker: %v", execErr)
	}
	var expected, actual map[string]any
	if err := json.Unmarshal(native, &expected); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(payload), &actual); err != nil {
		t.Fatalf("invalid navigation payload: %v", err)
	}
	for _, observation := range []map[string]any{expected, actual} {
		path, ok := observation["path"].(string)
		if !ok || !strings.HasPrefix(path, "/") || len(path) != 16 || observation["pathLength"] != float64(len(path)) || observation["matches15"] != true || observation["matches18"] != false {
			t.Fatalf("navigation prefix/width/record binding: %v", observation)
		}
	}
	if strings.HasPrefix(id, "manual_") && expected["path"] != actual["path"] {
		t.Fatalf("manual navigation path: native=%v local=%v", expected["path"], actual["path"])
	}
	// The exact captured paths remain in testdata. Generated persisted record
	// IDs differ locally, while matches15 checks the complete record-bound URL.
	delete(expected, "path")
	delete(actual, "path")
	if !reflect.DeepEqual(expected, actual) {
		t.Fatalf("native navigation=%v; local=%v", expected, actual)
	}
}
