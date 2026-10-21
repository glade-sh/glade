package apextest

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestCursorTokenStringNativeAPI65Conformance(t *testing.T) {
	var data struct {
		APIVersion       string                               `json:"apiVersion"`
		Routes           []string                             `json:"routes"`
		ObservationCount int                                  `json:"observationCount"`
		Source           nativeBoundaryRoute                  `json:"source"`
		Native           map[string]map[string]map[string]any `json:"native"`
		Expected         map[string]map[string]map[string]any `json:"expected"`
		CarriedFields    []struct {
			Route, Field, Native, Reason string
		} `json:"carriedFields"`
	}
	raw, err := os.ReadFile("testdata/conformance/cursor_token_string_api65.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data.APIVersion != "65.0" || strings.Join(data.Routes, ",") != "anonymous,istest" || data.ObservationCount != 10 || len(data.CarriedFields) != 2 {
		t.Fatal("incomplete native cursor token matrix")
	}
	nativeBoundaryCheckSources(t, data.Source)
	seenCarries := map[string]bool{}
	for _, carry := range data.CarriedFields {
		if carry.Field != "renewed.serialized" || carry.Reason == "" || seenCarries[carry.Route] || carry.Native != data.Native[carry.Route]["renewed"]["serialized"] {
			t.Fatal("cursor nonce carry lacks its native value or reason")
		}
		seenCarries[carry.Route] = true
		// Keep the native nonce in the fixture; log its digest rather than the token.
		t.Logf("carried %s/%s native SHA256=%x: %s", carry.Route, carry.Field, sha256.Sum256([]byte(carry.Native)), carry.Reason)
	}
	for _, route := range data.Routes {
		t.Run(route, func(t *testing.T) {
			if !seenCarries[route] || len(data.Native[route]) != 5 || len(data.Expected[route]) != 5 {
				t.Fatal("missing cursor token route cells")
			}
			want := data.Expected[route]
			if !reflect.DeepEqual(cursorTokenStringProjection(t, data.Native[route]), want) {
				t.Fatal("asserted cursor projection differs from native observations")
			}
			index := nativeBoundaryIndex(t, data.APIVersion, data.Source.Classes, nil)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native source accepted; Glade diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = data.APIVersion
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			var execErr error
			if route == "anonymous" {
				analysis := sema.AnalyzeAnonymous(index, data.Source.Anonymous, data.APIVersion)
				program, err := vm.CompileAnonymousWithOptions(data.Source.Anonymous, vm.CompileOptions{APIVersion: data.APIVersion})
				if err != nil || analysis.HasErrors() {
					t.Fatalf("native anonymous source accepted; Glade=%v %v", err, analysis.Diagnostics)
				}
				_, execErr = runner.execute(program)
			} else {
				cases := Discover(index, Options{SelectedClasses: data.Source.Tests, SelectedMethod: "observe"})
				if len(cases) != 1 {
					t.Fatal("missing captured named observation")
				}
				methods, methodErrs := compileTestMethodsForIndex(&index, cases)
				programs, programErrs := compileTestInvokePrograms(cases)
				key := testCaseKey(cases[0])
				if methodErrs[key] != nil || programErrs[key] != nil {
					t.Fatalf("named compile: %v %v", methodErrs[key], programErrs[key])
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
				t.Fatalf("missing cursor observation transport: %v", execErr)
			}
			_, tail, found := strings.Cut(runtimeErr.ExceptionMessage(), "CURSOR_CELL|")
			if !found || strings.Count(runtimeErr.ExceptionMessage(), "CURSOR_CELL|") != 1 {
				t.Fatal("invalid cursor observation marker")
			}
			var got map[string]map[string]any
			if err := json.Unmarshal([]byte(tail), &got); err != nil {
				t.Fatal("invalid cursor observation JSON")
			}
			if projected := cursorTokenStringProjection(t, got); !reflect.DeepEqual(projected, want) {
				t.Fatalf("native projection=%v; Glade projection=%v", want, projected)
			}
		})
	}
}

func cursorTokenStringProjection(t *testing.T, observed map[string]map[string]any) map[string]map[string]any {
	t.Helper()
	projected := make(map[string]map[string]any, len(observed))
	for name, cell := range observed {
		projected[name] = make(map[string]any, len(cell))
		for field, value := range cell {
			projected[name][field] = value
		}
	}
	cell := projected["renewed"]
	serialized, ok := cell["serialized"].(string)
	if !ok {
		t.Fatal("renewed cursor lacks serialized JSON")
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(serialized), &payload); err != nil || len(payload) != 1 {
		t.Fatal("renewed cursor serialization is not a single-field JSON object")
	}
	token, ok := payload["queryId"].(string)
	if !ok {
		t.Fatal("renewed cursor serialization lacks a String queryId")
	}
	delete(cell, "serialized")
	cell["serializedSuffixRetained"] = strings.HasSuffix(token, "-renewed")
	cell["serializedPrefixNonempty"] = len(strings.TrimSuffix(token, "-renewed")) > 0
	if len(projected) != 5 {
		t.Fatalf("got %d cursor cells; want 5", len(projected))
	}
	return projected
}
