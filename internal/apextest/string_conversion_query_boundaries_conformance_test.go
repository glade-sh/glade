package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

type stringBoundaryObservation struct {
	Outcome          string          `json:"outcome"`
	Value            json.RawMessage `json:"value"`
	ExceptionType    string          `json:"exceptionType"`
	ExceptionMessage string          `json:"exceptionMessage"`
	Diagnostics      json.RawMessage `json:"diagnostics"`
}

type stringBoundaryCell struct {
	API         string                    `json:"api"`
	Route       string                    `json:"route"`
	Status      string                    `json:"status"`
	Observation stringBoundaryObservation `json:"observation"`
	Provenance  struct {
		Capture           string `json:"capture"`
		Source            string `json:"source"`
		SourceSHA256      string `json:"sourceSHA256"`
		MergedTSVSHA256   string `json:"mergedTSVSHA256"`
		NativeValueSHA256 string `json:"nativeValueSHA256"`
	} `json:"provenance"`
}

type stringBoundaryRow struct {
	ID          string               `json:"id"`
	Group       string               `json:"group"`
	Kind        string               `json:"kind"`
	Apex        string               `json:"apex"`
	ApexSHA256  string               `json:"apexSHA256"`
	CarryReason string               `json:"carryReason"`
	Cells       []stringBoundaryCell `json:"cells"`
}

// Replay stable native fields on both execution routes. Captured now/timezone
// values are provenance, not claims about the local clock. Scheduling snapshots
// and compiler wrapper framing remain explicit carries with their native values.
func TestStringConversionQueryBoundariesConformance(t *testing.T) {
	var data struct {
		APIVersions            []string            `json:"apiVersions"`
		Routes                 []string            `json:"routes"`
		ShapeCount             int                 `json:"shapeCount"`
		NativeObservationCount int                 `json:"nativeObservationCount"`
		HelperClasses          map[string]string   `json:"helperClasses"`
		HelperSHA256           map[string]string   `json:"helperSHA256"`
		Rows                   []stringBoundaryRow `json:"rows"`
	}
	raw, err := os.ReadFile("testdata/conformance/string_conversion_query_boundaries.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" || strings.Join(data.Routes, ",") != "anonymous,istest" || data.ShapeCount != 33 || len(data.Rows) != data.ShapeCount || data.NativeObservationCount != 198 {
		t.Fatal("incomplete native API/route/shape matrix")
	}
	checkHash := func(source, want string) {
		t.Helper()
		sum := sha256.Sum256([]byte(source))
		if hex.EncodeToString(sum[:]) != want {
			t.Fatal("owned source bytes changed without updating their hash")
		}
	}
	for name, source := range data.HelperClasses {
		checkHash(source, data.HelperSHA256[name])
	}
	seen, cells := map[string]bool{}, 0
	for _, row := range data.Rows {
		if row.ID == "" || seen[row.ID] || len(row.Cells) != 6 {
			t.Fatalf("invalid/repeated shape %s", row.ID)
		}
		seen[row.ID] = true
		checkHash(row.Apex, row.ApexSHA256)
		matrix := map[string]bool{}
		for _, cell := range row.Cells {
			key := cell.API + "/" + cell.Route
			if matrix[key] || !strings.Contains(",62.0,67.0,68.0,", ","+cell.API+",") || (cell.Route != "anonymous" && cell.Route != "istest") || cell.Status != cell.Observation.Outcome {
				t.Fatalf("invalid/repeated native cell %s/%s", row.ID, key)
			}
			matrix[key] = true
			if cell.Provenance.Capture != "capture1" && cell.Provenance.Capture != "capture2" {
				t.Fatal("missing neutral capture attribution")
			}
			for _, hash := range []string{cell.Provenance.SourceSHA256, cell.Provenance.MergedTSVSHA256, cell.Provenance.NativeValueSHA256} {
				if decoded, err := hex.DecodeString(hash); err != nil || len(decoded) != sha256.Size {
					t.Fatal("missing native evidence hash")
				}
			}
			cells++
		}
	}
	if cells != data.NativeObservationCount {
		t.Fatalf("native cells: %d", cells)
	}
	for _, api := range data.APIVersions {
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "StringConversionQueryBoundaries", api)
			for _, row := range data.Rows {
				for _, cell := range row.Cells {
					if cell.API != api {
						continue
					}
					kind := "exact"
					if row.Kind == "cron" {
						kind = "carried"
					} else if cell.Status == "compile-rejected" {
						kind = "partial"
					}
					countRoute := cell.Route
					if countRoute == "istest" {
						countRoute = "isTest"
					}
					counts.RunKind(t, countRoute, row.ID, row.ID+"/"+cell.Route, kind, func(t *testing.T) {
						if row.Kind == "cron" {
							if row.CarryReason == "" || len(cell.Observation.Value) == 0 {
								t.Fatal("carry must preserve native value and reason")
							}
							t.Logf("CARRIED %s; native=%s", row.CarryReason, cell.Observation.Value)
							return
						}
						helpers := map[string]string{}
						if row.Group == "conversion" {
							for _, name := range []string{"DisplaySeed", "RelaySeed"} {
								helpers[name] = data.HelperClasses[name]
							}
						}
						body := stringBoundaryBody(row)
						classes := helpers
						if cell.Route == "istest" {
							classes["StringBoundarySpec"] = "@IsTest private class StringBoundarySpec {@IsTest static void observe(){" + body + "}}"
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
							if cell.Status == "compile-rejected" {
								stringBoundaryRejection(t, row, cell, anonymous.Diagnostics, compileErr)
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
							if cell.Status == "compile-rejected" {
								stringBoundaryRejection(t, row, cell, diagnostics, nil)
								return
							}
							if index.HasErrors() || analysis.HasErrors() {
								t.Fatalf("native executes; named diagnostics=%v", diagnostics)
							}
							cases := Discover(index, Options{SelectedClasses: []string{"StringBoundarySpec"}, SelectedMethod: "observe"})
							if len(cases) != 1 {
								t.Fatalf("named discovery: %d", len(cases))
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
						stringBoundaryValue(t, row.ID, cell.Observation, execErr)
					})
				}
			}
		})
	}
}

func stringBoundaryBody(row stringBoundaryRow) string {
	if row.Kind == "uncaught" {
		return row.Apex
	}
	observe := "observation.put('value',value);observation.put('null',value==null);"
	if row.Kind == "string" {
		observe += "if(value==null){observation.put('length',null);}else{observation.put('length',value.length());}"
	}
	return "Map<String,Object> observation=new Map<String,Object>();try{" + row.Apex + observe + "observation.put('complete',true);}catch(Exception e){observation.put('type',e.getTypeName());observation.put('message',e.getMessage());observation.put('complete',false);}System.assert(false,'P|" + row.ID + "|'+JSON.serialize(observation));"
}

func stringBoundaryRejection(t *testing.T, row stringBoundaryRow, cell stringBoundaryCell, diagnostics []diagnostic.Diagnostic, compileErr error) {
	t.Helper()
	var primary string
	if cell.Route == "anonymous" {
		var native struct {
			Problem string `json:"compileProblem"`
		}
		if err := json.Unmarshal(cell.Observation.Diagnostics, &native); err != nil {
			t.Fatal(err)
		}
		primary = native.Problem
	} else {
		var native []nativeBoundaryDiagnostic
		if err := json.Unmarshal(cell.Observation.Diagnostics, &native); err != nil || len(native) == 0 {
			t.Fatalf("native rejection diagnostics: %v", err)
		}
		primary = native[0].Problem
	}
	nativeBoundaryReject(t, nativeBoundaryRoute{NativeDiagnostics: []nativeBoundaryDiagnostic{{Problem: primary}}}, cell.API, diagnostics, compileErr)
	if row.CarryReason == "" {
		t.Fatal("compiler framing carry requires a reason")
	}
	t.Logf("PARTIAL rejection and primary text matched; CARRIED %s; native=%s", row.CarryReason, cell.Observation.Diagnostics)
}

func stringBoundaryValue(t *testing.T, id string, want stringBoundaryObservation, execErr error) {
	t.Helper()
	var runtimeErr *vm.RuntimeError
	if !errors.As(execErr, &runtimeErr) {
		t.Fatalf("native observation missing: %v", execErr)
	}
	if strings.HasPrefix(want.Outcome, "uncaught_") {
		// Normalize only exception type qualification; the native message stays exact.
		exceptionType := listSObjectCastExceptionType(runtimeErr.Type)
		if exceptionType != want.ExceptionType || exceptionType+": "+runtimeErr.ExceptionMessage() != want.ExceptionMessage {
			t.Fatalf("native uncaught %s/%s; local=%v", want.ExceptionType, want.ExceptionMessage, execErr)
		}
		return
	}
	if want.Outcome != "runtime" || runtimeErr.Type != "System.AssertException" {
		t.Fatalf("native observation transport mismatch: %v", execErr)
	}
	marker := "P|" + id + "|"
	_, payload, found := strings.Cut(runtimeErr.ExceptionMessage(), marker)
	if !found {
		t.Fatalf("missing terminal observation: %v", execErr)
	}
	var expected, actual map[string]any
	if err := json.Unmarshal(want.Value, &expected); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(payload), &actual); err != nil {
		t.Fatalf("invalid local observation %q: %v", payload, err)
	}
	delete(expected, "now")
	delete(expected, "timezone")
	if !reflect.DeepEqual(expected, actual) {
		t.Fatalf("native stable payload=%v; local=%v", expected, actual)
	}
}
