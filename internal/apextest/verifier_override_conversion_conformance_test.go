package apextest

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

type overrideConversionNative struct {
	Outcome, ExceptionType, ExceptionMessage string
	Value, Diagnostics                       json.RawMessage
	RawMessageSHA256                         string `json:"raw_message_sha256"`
}

type overrideConversionCell struct {
	Native      overrideConversionNative
	CarryReason string
}

type overrideConversionRow struct {
	ID, Expression, Body, Bare string
	Statement                  bool
	ExpressionSHA256           string `json:"expression_sha256"`
	Cells                      map[string]map[string]overrideConversionCell
}

type overrideConversionCapture struct {
	APIVersions, Routes           []string
	Context, CaseTableSHA256      string
	SupportClasses, SupportSHA256 map[string]string
	Rows                          []overrideConversionRow
}

// The plain capture pins both helpers and callers to each source API. Native
// values remain independent for both routes; generated stubs are outside it.
func TestVerifierOverrideConversionOrgConformance(t *testing.T) {
	data := readOverrideConversionCapture(t)
	for _, api := range data.APIVersions {
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "VerifierOverrideConversion", api)
			classes := make(map[string]string, len(data.SupportClasses)+1)
			for name, source := range data.SupportClasses {
				classes[name] = source
			}
			var methods strings.Builder
			for _, row := range data.Rows {
				if row.ID != "CTRL_COMPILE" {
					fmt.Fprintf(&methods, "@IsTest static void %s(){%s}\n", row.ID, overrideConversionSource(row))
				}
			}
			classes["ConversionObservation"] = "@IsTest private class ConversionObservation {" + methods.String() + "}"
			index := nativeBoundaryIndex(t, api, classes, nil)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native helpers accepted; local diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			for _, route := range data.Routes {
				for _, row := range data.Rows {
					cell := row.Cells[api][route]
					kind := "exact"
					if cell.CarryReason != "" {
						kind = "carried"
					}
					counts.RunKind(t, route, row.ID, route+"/"+row.ID, kind, func(t *testing.T) {
						if cell.CarryReason != "" {
							t.Logf("carry: %s; native=%+v", cell.CarryReason, cell.Native)
							return
						}
						if row.ID == "CTRL_COMPILE" {
							assertOverrideConversionRejection(t, index, classes, api, route, row, cell)
							return
						}
						err := executeOverrideConversion(t, index, runner, api, route, row)
						assertOverrideConversionValue(t, row.ID, cell.Native, err)
					})
				}
			}
		})
	}
}

func readOverrideConversionCapture(t *testing.T) overrideConversionCapture {
	t.Helper()
	var data overrideConversionCapture
	raw, err := os.ReadFile("testdata/conformance/verifier_override_conversion.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Rows) != 26 || strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" || strings.Join(data.Routes, ",") != "anonymous,istest" || data.Context != "plain" || len(data.CaseTableSHA256) != 64 || len(data.SupportClasses) != 2 || len(data.SupportSHA256) != 2 {
		t.Fatal("incomplete plain conversion capture")
	}
	for name, source := range data.SupportClasses {
		if fmt.Sprintf("%x", sha256.Sum256([]byte(source))) != data.SupportSHA256[name] {
			t.Fatal("native helper source changed")
		}
	}
	seen := map[string]bool{}
	for _, row := range data.Rows {
		if seen[row.ID] || row.ID == "" || len(row.Cells) != 3 {
			t.Fatal("duplicate or incomplete row")
		}
		seen[row.ID] = true
		if row.Expression != "" && fmt.Sprintf("%x", sha256.Sum256([]byte(row.Expression))) != row.ExpressionSHA256 {
			t.Fatal("native expression changed")
		}
		for _, api := range data.APIVersions {
			if len(row.Cells[api]) != 2 {
				t.Fatal("missing independent route")
			}
			for _, route := range data.Routes {
				cell := row.Cells[api][route]
				if cell.Native.Outcome == "" || (cell.Native.Outcome == "runtime" && !json.Valid(cell.Native.Value)) {
					t.Fatal("missing native observation")
				}
				if cell.CarryReason != "" && (row.ID != "CTRL_UNCAUGHT" || cell.Native.ExceptionType != "System.MathException" || cell.Native.ExceptionMessage != "System.MathException: Divide by 0") {
					t.Fatal("unsettled cell lacks its native failure")
				}
			}
		}
	}
	for _, id := range append([]string{"CTRL_POS", "CTRL_CAUGHT", "CTRL_UNCAUGHT", "CTRL_COMPILE"}, conversionCaptureIDs()...) {
		if !seen[id] {
			t.Fatalf("missing native row %s", id)
		}
	}
	return data
}

func conversionCaptureIDs() []string {
	ids := make([]string, 22)
	for i := range ids {
		ids[i] = fmt.Sprintf("C%02d", i+1)
	}
	return ids
}

func overrideConversionSource(row overrideConversionRow) string {
	if row.Bare != "" {
		return row.Bare
	}
	body := row.Body
	if row.Expression != "" {
		if row.Statement {
			body = row.Expression
		} else {
			body = "Object observed=" + row.Expression + ";result.put('value',observed);"
		}
	}
	// The corrected native C12 puts no statement after its unconditional throw.
	if !strings.HasPrefix(strings.TrimSpace(body), "throw ") {
		body += "result.put('effectFired',true);"
	}
	return "Map<String,Object> result=new Map<String,Object>();try{" + body + "}catch(Exception e){result.put('effectFired',true);result.put('exceptionType',e.getTypeName());result.put('exceptionMessage',e.getMessage());}System.assert(false,'C126_RESULT|" + row.ID + "|'+JSON.serialize(result));"
}

func executeOverrideConversion(t *testing.T, index typesys.Index, runner *conformanceRunner, api, route string, row overrideConversionRow) error {
	t.Helper()
	if route == "anonymous" {
		source := overrideConversionSource(row)
		if analysis := sema.AnalyzeAnonymous(index, source, api); analysis.HasErrors() {
			t.Fatal(analysis.Diagnostics)
		}
		program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
		if err != nil {
			t.Fatal(err)
		}
		_, err = runner.execute(program)
		return err
	}
	cases := Discover(index, Options{SelectedClasses: []string{"ConversionObservation"}, SelectedMethod: row.ID})
	if len(cases) != 1 {
		t.Fatalf("named discovery=%d", len(cases))
	}
	methods, methodErrors := compileTestMethods(cases)
	programs, programErrors := compileTestInvokePrograms(cases)
	key := testCaseKey(cases[0])
	if methodErrors[key] != nil || programErrors[key] != nil {
		t.Fatalf("named compile=%v %v", methodErrors[key], programErrors[key])
	}
	machine := runner.newMachine()
	if err := machine.RegisterMethod(methods[key]); err != nil {
		t.Fatal(err)
	}
	machine.EnableTestContext()
	_, err := machine.ExecuteInClass(programs[key], cases[0].ClassName)
	return err
}

func assertOverrideConversionValue(t *testing.T, id string, native overrideConversionNative, err error) {
	t.Helper()
	var runtimeErr *vm.RuntimeError
	if !errors.As(err, &runtimeErr) {
		t.Fatalf("native runtime outcome=%s; local=%v", native.Outcome, err)
	}
	if strings.HasPrefix(native.Outcome, "uncaught_") {
		if !nativeObservationFailureMatches(runtimeErr, native.ExceptionType, native.ExceptionMessage) {
			t.Fatalf("native=%s; local=%v", native.ExceptionMessage, err)
		}
		return
	}
	_, payload, found := strings.Cut(runtimeErr.ExceptionMessage(), "C126_RESULT|"+id+"|")
	var actual, expected any
	if native.Outcome != "runtime" || runtimeErr.Type != "System.AssertException" || !found || json.Unmarshal([]byte(payload), &actual) != nil || json.Unmarshal(native.Value, &expected) != nil || !reflect.DeepEqual(actual, expected) {
		t.Fatalf("native=%s; local=%v", native.Value, err)
	}
}

func assertOverrideConversionRejection(t *testing.T, index typesys.Index, classes map[string]string, api, route string, row overrideConversionRow, cell overrideConversionCell) {
	t.Helper()
	if cell.Native.Outcome != "compile-rejected" || !json.Valid(cell.Native.Diagnostics) {
		t.Fatal("missing native compiler observation")
	}
	problem := ""
	if route == "anonymous" {
		var native struct{ CompileProblem string }
		if err := json.Unmarshal(cell.Native.Diagnostics, &native); err != nil {
			t.Fatal(err)
		}
		problem = native.CompileProblem
	} else {
		var native []nativeBoundaryDiagnostic
		if err := json.Unmarshal(cell.Native.Diagnostics, &native); err != nil || len(native) != 1 {
			t.Fatalf("native primary diagnostics=%s", cell.Native.Diagnostics)
		}
		problem = native[0].Problem
	}
	if problem == "" {
		t.Fatal("missing native primary compiler message")
	}
	source := nativeBoundaryRoute{NativeDiagnostics: []nativeBoundaryDiagnostic{{Problem: problem}}}
	if route == "anonymous" {
		nativeBoundaryReject(t, source, api, sema.AnalyzeAnonymous(index, row.Bare, api).Diagnostics, nil)
		return
	}
	withControl := make(map[string]string, len(classes)+1)
	for name, text := range classes {
		withControl[name] = text
	}
	withControl["ConversionCompileControl"] = "@IsTest private class ConversionCompileControl {@IsTest static void observe(){" + row.Bare + "}}"
	control := nativeBoundaryIndex(t, api, withControl, nil)
	nativeBoundaryReject(t, source, api, sema.Analyze(control).Diagnostics, nil)
}
