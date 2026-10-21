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

func TestCursorJSONShellNativeConformance(t *testing.T) {
	var data struct {
		APIVersion       string              `json:"apiVersion"`
		ObservationCount int                 `json:"observationCount"`
		Source           nativeBoundaryRoute `json:"source"`
		Carries          []struct {
			Shape  string `json:"shape"`
			Reason string `json:"reason"`
		} `json:"carries"`
		Rows []struct {
			ID              string                               `json:"id"`
			TestMethod      string                               `json:"testMethod"`
			Anonymous       string                               `json:"anonymous"`
			AnonymousSHA256 string                               `json:"anonymousSha256"`
			Expected        map[string]nativeObservationExpected `json:"expected"`
		} `json:"rows"`
	}
	raw, err := os.ReadFile("testdata/conformance/cursor_json_shell_native.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data.APIVersion != "64.0" || data.ObservationCount != 12 || len(data.Rows) != 6 || len(data.Carries) == 0 {
		t.Fatal("incomplete captured cursor matrix")
	}
	nativeBoundaryCheckSources(t, data.Source)
	for _, carry := range data.Carries {
		if carry.Shape == "" || carry.Reason == "" {
			t.Fatal("unanswered cursor shape needs a reason")
		}
		t.Logf("carried %s: %s", carry.Shape, carry.Reason)
	}
	counts := newConformanceCountLog(t, "CursorJSONShell", data.APIVersion)
	seen := map[string]bool{}
	for _, row := range data.Rows {
		if row.ID == "" || seen[row.ID] || row.TestMethod == "" || len(row.Expected) != 2 {
			t.Fatal("invalid cursor observation row")
		}
		seen[row.ID] = true
		for _, route := range []string{"anonymous", "isTest"} {
			t.Run(row.ID+"/"+route, func(t *testing.T) {
				counts.TrackKind(t, route, "exact", 1)
				source := data.Source
				source.ID, source.TestMethod = row.ID, row.TestMethod
				want, ok := row.Expected[route]
				if !ok || want.Outcome != "runtime" {
					t.Fatal("missing native runtime cell")
				}
				if route == "isTest" {
					cursorJSONShellNamedObservation(t, data.APIVersion, source, want)
					return
				}
				source.Anonymous = row.Anonymous
				source.SourceSHA256 = map[string]string{"anonymous": row.AnonymousSHA256}
				for key, value := range data.Source.SourceSHA256 {
					source.SourceSHA256[key] = value
				}
				nativeBoundaryCheckSources(t, source)
				index := nativeBoundaryIndex(t, data.APIVersion, source.Classes, nil)
				analysis := sema.Analyze(index)
				anonymous := sema.AnalyzeAnonymous(index, source.Anonymous, data.APIVersion)
				if index.HasErrors() || analysis.HasErrors() || anonymous.HasErrors() {
					t.Fatalf("native executes; Glade diagnostics=%v %v %v", index.Diagnostics, analysis.Diagnostics, anonymous.Diagnostics)
				}
				program, err := vm.CompileAnonymousWithOptions(source.Anonymous, vm.CompileOptions{APIVersion: data.APIVersion})
				if err != nil {
					t.Fatal(err)
				}
				org := orgFromIndex(index)
				org.APIVersion = data.APIVersion
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				result, err := runner.execute(program)
				if err != nil {
					t.Fatal(err)
				}
				prefix := "NATIVE_RESULT|" + row.ID + "|"
				var observed json.RawMessage
				for _, line := range result.Debug {
					if strings.HasPrefix(line, prefix) {
						if observed != nil {
							t.Fatal("repeated cursor observation")
						}
						observed = json.RawMessage(strings.TrimPrefix(line, prefix))
					}
				}
				var got, expected map[string]any
				if json.Unmarshal(observed, &got) != nil || json.Unmarshal(want.Value, &expected) != nil || len(expected) == 0 {
					t.Fatal("missing native or local cursor JSON")
				}
				if !reflect.DeepEqual(got, expected) {
					t.Fatalf("native=%s; Glade=%s", want.Value, observed)
				}
			})
		}
	}
}

func cursorJSONShellNamedObservation(t *testing.T, api string, source nativeBoundaryRoute, want nativeObservationExpected) {
	t.Helper()
	index := nativeBoundaryIndex(t, api, source.Classes, nil)
	analysis := sema.Analyze(index)
	if index.HasErrors() || analysis.HasErrors() {
		t.Fatalf("native executes; named diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
	}
	cases := Discover(index, Options{SelectedClasses: source.Tests, SelectedMethod: source.TestMethod})
	if len(cases) != 1 {
		t.Fatalf("named cursor observation discovery: %d", len(cases))
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
	_, execErr := machine.ExecuteInClass(programs[key], cases[0].ClassName)
	var runtimeErr *vm.RuntimeError
	if !errors.As(execErr, &runtimeErr) || runtimeErr.Type != "System.AssertException" {
		t.Fatalf("missing named cursor observation transport: %v", execErr)
	}
	marker := "NATIVE_RESULT|" + source.ID + "|"
	_, tail, found := strings.Cut(runtimeErr.ExceptionMessage(), marker)
	if !found || strings.Count(runtimeErr.ExceptionMessage(), "NATIVE_RESULT|") != 1 {
		t.Fatal("invalid named cursor observation transport")
	}
	var observed, expected map[string]any
	if json.Unmarshal([]byte(tail), &observed) != nil || json.Unmarshal(want.Value, &expected) != nil || len(expected) == 0 {
		t.Fatal("missing native or local named cursor JSON")
	}
	if !reflect.DeepEqual(observed, expected) {
		t.Fatalf("native=%s; Glade=%s", want.Value, tail)
	}
}
