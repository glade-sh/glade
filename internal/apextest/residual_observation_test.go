package apextest

import (
	"encoding/json"
	"errors"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func residualObservationObserve(t *testing.T, api, route string, source nativeBoundaryRoute, want nativeObservationExpected, pages ...string) {
	t.Helper()
	residualObservationObserveCore(t, api, route, source, want, true, nil, pages...)
}

func residualObservationObserveExact(t *testing.T, api, route string, source nativeBoundaryRoute, want nativeObservationExpected, pages ...string) {
	t.Helper()
	residualObservationObserveCore(t, api, route, source, want, false, nil, pages...)
}

func residualObservationObserveCore(t *testing.T, api, route string, source nativeBoundaryRoute, want nativeObservationExpected, sanitizeKeys bool, base *conformanceRunner, pages ...string) {
	t.Helper()
	index := nativeBoundaryIndex(t, api, source.Classes, nil)
	if len(pages) != 0 {
		index.VisualforcePageNames = pages
		index.VisualforcePagesKnown = true
	}
	options := conformanceRunnerOptions{Base: base, LinkProject: true}
	if base != nil {
		options.Org = base.orgSource
	} else {
		org := orgFromIndex(index)
		org.APIVersion = api
		if source.RuntimeAPIVersion != "" {
			org.APIVersion = source.RuntimeAPIVersion
		}
		options.Org = &org
	}
	runner := newConformanceRunner(t, index, options)
	machine := runner.newMachine()
	for _, page := range pages {
		machine.RegisterPageReference(page)
	}
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
		method := source.TestMethod
		if method == "" {
			method = "observe"
		}
		cases := Discover(index, Options{SelectedClasses: source.Tests, SelectedMethod: method})
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
	if want.Outcome == "assertion-failure" {
		if runtimeErr.Type != want.ExceptionType || "System.AssertException: "+runtimeErr.ExceptionMessage() != want.ExceptionMessage {
			t.Fatalf("native assertion %s; Glade assertion %s", want.ExceptionMessage, runtimeErr.ExceptionMessage())
		}
		return
	}
	marker := "NATIVE_RESULT|" + source.ID + "|"
	message := runtimeErr.ExceptionMessage()
	_, result, found := strings.Cut(message, marker)
	if !found || strings.Count(message, "NATIVE_RESULT|") != 1 {
		t.Fatalf("invalid observation transport: %s", message)
	}
	var expected, observed map[string]any
	if err := json.Unmarshal(want.Value, &expected); err != nil || len(expected) == 0 {
		t.Fatal("invalid native JSON value")
	}
	if err := json.Unmarshal([]byte(result), &observed); err != nil {
		t.Fatalf("invalid local observation: %s", result)
	}
	if sanitizeKeys {
		observed = residualSanitizeKeys(observed).(map[string]any)
	}
	if !reflect.DeepEqual(expected, observed) {
		t.Fatalf("native value %s; Glade value %s", want.Value, result)
	}
}

var residualIdentityPattern = regexp.MustCompile(`^[a-zA-Z0-9]{15}([a-zA-Z0-9]{3})?$`)

func residualSanitizeKeys(value any) any {
	switch value := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(value))
		// The assertion probes put the method before the passing flag; the native
		// sanitizer retains the first key when both names become an identity marker.
		if method, ok := value["assertionMethod"]; ok {
			out["[ID]"] = method
		}
		for key, cell := range value {
			if residualIdentityPattern.MatchString(key) {
				key = "[ID]"
			}
			if _, exists := out[key]; !exists {
				out[key] = residualSanitizeKeys(cell)
			}
		}
		return out
	case []any:
		out := make([]any, len(value))
		for i, cell := range value {
			out[i] = residualSanitizeKeys(cell)
		}
		return out
	default:
		return value
	}
}
