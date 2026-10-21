package apextest

import (
	"crypto/sha256"
	"encoding/hex"
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

const numberMagnitudeFixture = "testdata/conformance/number_magnitude"

type numberMagnitudeRow struct {
	Native        json.RawMessage   `json:"native"`
	CarriedFields map[string]string `json:"carriedFields"`
}

type numberMagnitudeContext struct {
	ID           string                        `json:"id"`
	SourceAPI    string                        `json:"sourceApi"`
	CallerAPI    string                        `json:"callerApi"`
	TransportAPI string                        `json:"transportApi"`
	Route        string                        `json:"route"`
	Rows         map[string]numberMagnitudeRow `json:"rows"`
}

// Replay the same captured calls once per context, retaining source-version
// pins independently of transport versions. Only explicitly carried message
// text is excluded; all other observed row members remain exact assertions.
func TestNumberMagnitudeConformance(t *testing.T) {
	var data struct {
		ShapeCount             int `json:"shapeCount"`
		NativeObservationCount int `json:"nativeObservationCount"`
		Files                  []struct {
			Path   string `json:"path"`
			SHA256 string `json:"sha256"`
		} `json:"files"`
		Contexts     []numberMagnitudeContext `json:"contexts"`
		Supplemental numberMagnitudeContext   `json:"supplemental"`
	}
	raw, err := os.ReadFile(filepath.Join(numberMagnitudeFixture, "observations.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data.ShapeCount != 14 || data.NativeObservationCount != 112 || len(data.Contexts) != 8 || len(data.Files) != 5 {
		t.Fatal("incomplete numeric magnitude observation matrix")
	}
	expectedFiles := map[string]bool{
		"setup/CustomMagnitudeObservation.cls":                   true,
		"setup/CustomMagnitudeObservationTest.cls":               true,
		"setup/BoundaryMagnitudeObservation.cls":                 true,
		"setup/Account/fields/MagnitudeSmall__c.field-meta.xml":  true,
		"setup/Account/fields/MagnitudeMedium__c.field-meta.xml": true,
	}
	sources := map[string]string{}
	for _, file := range data.Files {
		if !expectedFiles[file.Path] || sources[file.Path] != "" {
			t.Fatalf("invalid or repeated numeric fixture file: %s", file.Path)
		}
		raw, err := os.ReadFile(filepath.Join(numberMagnitudeFixture, filepath.FromSlash(file.Path)))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(raw)
		if len(raw) == 0 || hex.EncodeToString(sum[:]) != file.SHA256 {
			t.Fatalf("captured numeric fixture bytes changed: %s", file.Path)
		}
		sources[file.Path] = string(raw)
	}
	expectedContexts := map[string]bool{}
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, route := range []string{"anonymous", "istest"} {
			expectedContexts[api+"/"+api+"/"+api+"/"+route] = true
		}
	}
	expectedContexts["67.0/62.0/62.0/anonymous"] = true
	expectedContexts["67.0/67.0/62.0/istest"] = true
	rowIDs := []string{
		"smallBoundary", "smallBoundaryOverflow", "smallOverflow", "smallNegativeOverflow",
		"mediumBoundary", "mediumOverflow", "mediumNegativeOverflow", "extraFraction",
		"boundaryFraction", "nullControl", "zeroControl", "partial", "update", "statement",
	}
	seenContexts, seenIDs := map[string]bool{}, map[string]bool{}
	partial := 0
	for _, context := range data.Contexts {
		key := context.SourceAPI + "/" + context.CallerAPI + "/" + context.TransportAPI + "/" + context.Route
		if !expectedContexts[key] || seenContexts[key] || context.ID == "" || seenIDs[context.ID] {
			t.Fatalf("invalid or repeated numeric context: %s", key)
		}
		seenContexts[key], seenIDs[context.ID] = true, true
		partial += numberMagnitudeValidateRows(t, context, rowIDs)
	}
	if partial != 14 {
		t.Fatalf("main message-text carries = %d, want 14", partial)
	}
	supplemental := data.Supplemental
	if supplemental.ID == "" || seenIDs[supplemental.ID] || supplemental.SourceAPI != "62.0" || supplemental.CallerAPI != "62.0" || supplemental.TransportAPI != "62.0" || supplemental.Route != "anonymous" {
		t.Fatal("changed supplemental numeric observation context")
	}
	boundaryIDs := []string{"boundary1", "boundary2", "boundary3", "boundary4", "boundary5", "boundary6"}
	if partial := numberMagnitudeValidateRows(t, supplemental, boundaryIDs); partial != 2 {
		t.Fatalf("supplemental message-text carries = %d, want 2", partial)
	}
	for _, context := range data.Contexts {
		t.Run(context.ID, func(t *testing.T) {
			observed := numberMagnitudeExecute(t, context, sources, false)
			numberMagnitudeCompareRows(t, context, rowIDs, observed)
		})
	}
	t.Run(supplemental.ID, func(t *testing.T) {
		observed := numberMagnitudeExecute(t, supplemental, sources, true)
		numberMagnitudeCompareRows(t, supplemental, boundaryIDs, observed)
	})
}

func numberMagnitudeValidateRows(t *testing.T, context numberMagnitudeContext, ids []string) int {
	t.Helper()
	if len(context.Rows) != len(ids) {
		t.Fatalf("incomplete numeric rows: %s", context.ID)
	}
	partial := 0
	for _, id := range ids {
		row, exists := context.Rows[id]
		var native map[string]any
		if !exists || json.Unmarshal(row.Native, &native) != nil || len(native) == 0 {
			t.Fatalf("missing numeric native row: %s/%s", context.ID, id)
		}
		if len(row.CarriedFields) == 0 {
			continue
		}
		if context.SourceAPI != "62.0" || len(row.CarriedFields) != 1 || strings.TrimSpace(row.CarriedFields["message"]) == "" {
			t.Fatalf("unbounded numeric carry: %s/%s", context.ID, id)
		}
		if message, ok := native["message"].(string); !ok || message == "" {
			t.Fatalf("message-text carry lacks its native value: %s/%s", context.ID, id)
		}
		partial++
	}
	return partial
}

func numberMagnitudeExecute(t *testing.T, context numberMagnitudeContext, sources map[string]string, supplemental bool) map[string]json.RawMessage {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+context.TransportAPI+`","packageDirectories":[{"path":"force-app","default":true}]}`)
	for path, source := range sources {
		relative := strings.TrimPrefix(path, "setup/")
		folder := "objects"
		if strings.HasSuffix(path, ".cls") {
			isBoundary := relative == "BoundaryMagnitudeObservation.cls"
			if isBoundary != supplemental {
				continue
			}
			folder = "classes"
		}
		destination := filepath.Join(root, "force-app/main/default", folder, filepath.FromSlash(relative))
		writeFile(t, destination, source)
		if folder == "classes" {
			writeFile(t, destination+"-meta.xml", "<ApexClass><apiVersion>"+context.SourceAPI+"</apiVersion></ApexClass>")
		}
	}
	index := loadTestIndex(t, root)
	for _, typ := range index.Types {
		if typ.File != "" && strings.HasPrefix(typ.File, root) && typ.EffectiveAPIVersion != context.SourceAPI {
			t.Fatalf("changed numeric class source pin: %s=%s", typ.Name, typ.EffectiveAPIVersion)
		}
	}
	analysis := sema.Analyze(index)
	if index.HasErrors() || analysis.HasErrors() {
		t.Fatalf("native numeric fixture accepted; local diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
	}
	org := orgFromIndex(index)
	org.APIVersion = context.TransportAPI
	runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
	helper, marker := "CustomMagnitudeObservation", "NATIVE_CUSTOM_MAGNITUDE:"
	if supplemental {
		helper, marker = "BoundaryMagnitudeObservation", "NATIVE_BOUNDARY_MAGNITUDE:"
	}
	var execErr error
	if context.Route == "anonymous" {
		body := "System.assert(false,'" + marker + "'+JSON.serialize(" + helper + ".observe()));"
		analysis := sema.AnalyzeAnonymous(index, body, context.CallerAPI)
		program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: context.CallerAPI})
		if analysis.HasErrors() || err != nil {
			t.Fatalf("native anonymous numeric source accepted; local diagnostics=%v compile=%v", analysis.Diagnostics, err)
		}
		_, execErr = runner.execute(program)
	} else {
		cases := Discover(index, Options{SelectedClasses: []string{"CustomMagnitudeObservationTest"}, SelectedMethod: "capture"})
		if len(cases) != 1 {
			t.Fatalf("numeric named observation discovery: %d", len(cases))
		}
		methods, methodErrors := compileTestMethodsForIndex(&index, cases)
		programs, programErrors := compileTestInvokePrograms(cases)
		key := testCaseKey(cases[0])
		if methodErrors[key] != nil || programErrors[key] != nil {
			t.Fatalf("numeric named compilation: %v %v", methodErrors[key], programErrors[key])
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
		t.Fatalf("missing numeric observation transport: %v", execErr)
	}
	message := runtimeErr.ExceptionMessage()
	_, payload, found := strings.Cut(message, marker)
	if !found || strings.Count(message, marker) != 1 {
		t.Fatalf("invalid numeric observation marker: %v", execErr)
	}
	rows := map[string]json.RawMessage{}
	if supplemental {
		var list []json.RawMessage
		if err := json.Unmarshal([]byte(payload), &list); err != nil || len(list) != 6 {
			t.Fatalf("invalid supplemental numeric payload: %s", payload)
		}
		for i, row := range list {
			rows[fmt.Sprintf("boundary%d", i+1)] = row
		}
	} else if err := json.Unmarshal([]byte(payload), &rows); err != nil {
		t.Fatalf("invalid numeric payload: %s", payload)
	}
	return rows
}

func numberMagnitudeCompareRows(t *testing.T, context numberMagnitudeContext, ids []string, observed map[string]json.RawMessage) {
	t.Helper()
	if len(observed) != len(ids) {
		t.Fatalf("numeric row count = %d, want %d", len(observed), len(ids))
	}
	counts := newConformanceCountLog(t, "NumberMagnitude", context.SourceAPI+" transport "+context.TransportAPI)
	route := context.Route
	if route == "istest" {
		route = "isTest"
	}
	for _, id := range ids {
		row := context.Rows[id]
		kind := "exact"
		if len(row.CarriedFields) != 0 {
			kind = "partial"
		}
		counts.RunKind(t, route, id, id, kind, func(t *testing.T) {
			var native, local map[string]any
			if err := json.Unmarshal(row.Native, &native); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(observed[id], &local); err != nil || len(local) == 0 {
				t.Fatalf("missing observed numeric row: %s", observed[id])
			}
			for field, reason := range row.CarriedFields {
				if _, exists := local[field].(string); !exists {
					t.Fatalf("carried formatter field is absent: %s", field)
				}
				t.Logf("CARRIED %s: %s; native=%v; local=%v", field, reason, native[field], local[field])
				delete(native, field)
				delete(local, field)
			}
			if !reflect.DeepEqual(native, local) {
				t.Fatalf("native numeric row=%s; local=%s", row.Native, observed[id])
			}
		})
	}
}
