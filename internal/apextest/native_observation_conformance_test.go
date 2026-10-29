package apextest

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

type nativeObservationFailure struct {
	Class         string `json:"cls"`
	Method        string `json:"method"`
	Message       string `json:"message"`
	ExceptionType string `json:"exceptionType"`
}

type nativeObservationExpected struct {
	Outcome          string                     `json:"outcome"`
	Value            json.RawMessage            `json:"value"`
	ExceptionMessage string                     `json:"exceptionMessage"`
	ExceptionType    string                     `json:"exceptionType"`
	TestFailures     []nativeObservationFailure `json:"testFailures"`
}

type nativeObservationRoute struct {
	nativeBoundaryRoute
	ExpectedByAPI   map[string]nativeObservationExpected `json:"expectedByAPI"`
	Comparison      string                               `json:"comparison,omitempty"`
	RecipientObject string                               `json:"recipientObject,omitempty"`
	CarryReason     string                               `json:"carryReason,omitempty"`
	CarryOwner      string                               `json:"carryOwner,omitempty"`
}

// Native observations can differ between routes. The captured final-field failure
// escapes the test method without reaching its assertion; it is an answer.
// Source hashes, primary compile diagnostics and runtime isolation reuse the
// native boundary and conformance helpers used by the earlier export.
func runNativeObservationConformance(t *testing.T, file, family string, shapeCount int) {
	t.Helper()
	var data struct {
		APIVersions            []string `json:"apiVersions"`
		Routes                 []string `json:"routes"`
		ShapeCount             int      `json:"shapeCount"`
		NativeObservationCount int      `json:"nativeObservationCount"`
		Rows                   []struct {
			ID     string                            `json:"id"`
			Routes map[string]nativeObservationRoute `json:"routes"`
		} `json:"rows"`
	}
	raw, err := os.ReadFile(filepath.Join("testdata/conformance", file))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Rows) != shapeCount || data.ShapeCount != shapeCount || data.NativeObservationCount != shapeCount*6 || strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" || strings.Join(data.Routes, ",") != "anonymous,isTest" {
		t.Fatalf("incomplete native matrix: %s", file)
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
			if source.Stage == "carried" && (strings.TrimSpace(source.CarryReason) == "" || strings.TrimSpace(source.CarryOwner) == "") {
				t.Fatalf("carried cell needs a reason and follow-up: %s/%s", row.ID, route)
			}
			nativeBoundaryCheckSources(t, source.nativeBoundaryRoute)
		}
	}
	for _, api := range data.APIVersions {
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			counts := newConformanceCountLog(t, family, api)
			for _, row := range data.Rows {
				t.Run(row.ID, func(t *testing.T) {
					for _, route := range data.Routes {
						t.Run(route, func(t *testing.T) {
							source := row.Routes[route]
							kind := "exact"
							if source.Stage == "carried" {
								kind = "carried"
							} else if source.Comparison == "identityHashRelation" {
								kind = "partial"
							}
							counts.TrackKind(t, route, kind, 1)
							want, ok := source.ExpectedByAPI[api]
							if !ok {
								t.Fatal("missing native API observation")
							}
							if source.Stage == "carried" {
								native, err := json.Marshal(want)
								if err != nil {
									t.Fatal(err)
								}
								t.Logf("carried to %s: %s; native=%s", source.CarryOwner, source.CarryReason, native)
								return
							}
							if route == "anonymous" {
								nativeObservationAnonymous(t, api, source, want)
							} else {
								nativeObservationTest(t, api, source, want)
							}
						})
					}
				})
			}
		})
	}
}

func nativeObservationAnonymous(t *testing.T, api string, source nativeObservationRoute, want nativeObservationExpected) {
	t.Helper()
	declarations := nativeBoundaryAnonymousDeclarations(t, source.Anonymous)
	index := sema.WithAnonymousDeclarationContext(nativeBoundaryIndex(t, api, declarations, source.Objects))
	body := []byte(source.Anonymous)
	parsed := apexast.ParseSource("NativeObservationAnonymous.cls", source.Anonymous)
	for _, decl := range parsed.Declarations {
		if decl.Kind != apexast.DeclarationClass && decl.Kind != apexast.DeclarationInterface && decl.Kind != apexast.DeclarationEnum {
			continue
		}
		for i := decl.Range.Start.Offset; i < decl.Range.End.Offset; i++ {
			if body[i] != '\n' && body[i] != '\r' {
				body[i] = ' '
			}
		}
	}
	// Match the product request path: declaration errors precede body errors.
	diagnostics := sema.NativeLifecycleDiagnostics(index, index.Diagnostics)
	if !nativeObservationHasErrors(diagnostics) {
		diagnostics = append(diagnostics, sema.AnalyzeAnonymousDeclarations(index).Diagnostics...)
	}
	if !nativeObservationHasErrors(diagnostics) {
		diagnostics = append(diagnostics, sema.AnalyzeAnonymous(index, string(body), api).Diagnostics...)
	}
	program, compileErr := vm.CompileAnonymousWithOptions(string(body), vm.CompileOptions{APIVersion: api, ApprovedPrefixStatements: sema.ApprovedAnonymousPrefixStatements(index, string(body), api)})
	if want.Outcome == "compile-rejected" {
		nativeBoundaryReject(t, source.nativeBoundaryRoute, api, diagnostics, compileErr)
		return
	}
	if nativeObservationHasErrors(diagnostics) || compileErr != nil {
		t.Fatalf("native executes; anonymous diagnostics=%v compile=%v", diagnostics, compileErr)
	}
	runtime, err := CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
	if err != nil {
		t.Fatal(err)
	}
	org := orgFromIndex(index)
	org.APIVersion = api
	runner := newConformanceRunner(t, index, conformanceRunnerOptions{RequestRuntime: &runtime, Org: &org})
	machine := runner.newMachine()
	_, execErr := machine.Execute(program)
	nativeObservationValue(t, source, "anonymous", want, execErr, machine.Org)
}

func nativeObservationTest(t *testing.T, api string, source nativeObservationRoute, want nativeObservationExpected) {
	t.Helper()
	index := nativeBoundaryIndex(t, api, source.Classes, source.Objects)
	analysis := sema.Analyze(index)
	diagnostics := append(append([]diagnostic.Diagnostic{}, index.Diagnostics...), analysis.Diagnostics...)
	if want.Outcome == "compile-rejected" {
		nativeBoundaryReject(t, source.nativeBoundaryRoute, api, diagnostics, nil)
		return
	}
	if nativeObservationHasErrors(diagnostics) {
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
	_, execErr := machine.ExecuteInClass(programs[key], cases[0].ClassName)
	if want.Outcome == "uncaught_test_failure" {
		if len(want.TestFailures) != 1 || want.TestFailures[0].Class != cases[0].ClassName || want.TestFailures[0].Method != "observe" {
			t.Fatal("uncaught test answer must identify the captured method")
		}
	}
	nativeObservationValue(t, source, "isTest", want, execErr, machine.Org)
}

func nativeObservationHasErrors(items []diagnostic.Diagnostic) bool {
	for _, d := range items {
		if d.Severity == diagnostic.Error {
			return true
		}
	}
	return false
}

func nativeObservationValue(t *testing.T, source nativeObservationRoute, route string, want nativeObservationExpected, execErr error, org *storage.OrgState) {
	t.Helper()
	var runtimeErr *vm.RuntimeError
	if execErr == nil || !errors.As(execErr, &runtimeErr) {
		t.Fatalf("native exception/transport missing: %v", execErr)
	}
	message := runtimeErr.ExceptionMessage()
	if want.Outcome == "runtime" {
		if route == "isTest" && runtimeErr.Type != "System.AssertException" || route == "anonymous" && runtimeErr.Type != "C126TransportException" && !strings.HasSuffix(runtimeErr.Type, ".C126TransportException") {
			t.Fatalf("wrong observation transport: %v", execErr)
		}
		marker := "C126_RESULT|" + source.ID + "|"
		_, tail, found := strings.Cut(message, marker)
		if !found || strings.Count(message, "C126_RESULT|") != 1 {
			t.Fatalf("invalid transport marker: %s", message)
		}
		var observed, expected map[string]any
		if err := json.Unmarshal([]byte(tail), &observed); err != nil || len(observed) == 0 {
			t.Fatalf("invalid transport JSON: %s", tail)
		}
		if err := json.Unmarshal(want.Value, &expected); err != nil || len(expected) == 0 {
			t.Fatal("invalid captured value")
		}
		matched, err := nativeObservationRuntimeMatches(source, expected, observed, org)
		if err != nil {
			t.Fatal(err)
		}
		if !matched {
			t.Fatalf("native value %s; Glade value %s", want.Value, tail)
		}
		return
	}
	if strings.Contains(message, "C126_RESULT|") {
		t.Fatal("uncaught native answer cannot be replaced by a transport marker")
	}
	switch want.Outcome {
	case "uncaught_test_failure":
		failure := want.TestFailures[0]
		if route != "isTest" || !nativeObservationFailureMatches(runtimeErr, failure.ExceptionType, failure.Message) {
			t.Fatalf("native failure %s; Glade failure %s: %s", failure.Message, runtimeErr.Type, message)
		}
	case "uncaught_anonymous":
		if route != "anonymous" || !nativeObservationFailureMatches(runtimeErr, want.ExceptionType, want.ExceptionMessage) {
			t.Fatalf("native failure %s; Glade failure %s: %s", want.ExceptionMessage, runtimeErr.Type, message)
		}
	default:
		t.Fatalf("unsupported native observation: %s", want.Outcome)
	}
}

func nativeObservationFailureMatches(local *vm.RuntimeError, nativeType, nativeMessage string) bool {
	// Some native transports supply a separate type field; others return the
	// type prefix inside their message. Compare the literal returned message in
	// either representation. A missing native type makes no assertion on type.
	if nativeType != "" && local.Type != nativeType {
		return false
	}
	return nativeMessage == local.ExceptionMessage() || nativeMessage == local.Type+": "+local.ExceptionMessage()
}
