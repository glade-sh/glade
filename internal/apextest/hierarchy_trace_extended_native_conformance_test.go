package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

// This matrix retains the measured setup and ordinary-method contexts separately.
// Assertion chunk transport is replaced only after the captured measurement;
// the helper names, caller names, source lines and expected trace bytes remain exact.
func TestHierarchyAccessorExtendedNativeConformance(t *testing.T) {
	runHierarchyTraceExtendedNativeConformance(t, "C02")
}

func runHierarchyTraceExtendedNativeConformance(t *testing.T, group string) {
	t.Helper()
	var data struct {
		APIVersions            []string `json:"apiVersions"`
		Routes                 []string `json:"routes"`
		NativeObservationCount int      `json:"nativeObservationCount"`
		Sources                map[string]struct {
			ClassName         string `json:"className"`
			MethodName        string `json:"methodName"`
			TestContext       string `json:"testContext"`
			Template          string `json:"template"`
			TemplateSHA256    string `json:"templateSHA256"`
			Measurement       string `json:"measurement"`
			MeasurementSHA256 string `json:"measurementSHA256"`
		} `json:"sources"`
		Fixtures map[string]struct {
			Bindings          map[string]string `json:"bindings"`
			Classes           map[string]string `json:"classes"`
			Objects           map[string]string `json:"objects"`
			ClassSHA256       map[string]string `json:"classSHA256"`
			ObjectSHA256      map[string]string `json:"objectSHA256"`
			HelperAPIReadback map[string]struct {
				API    string `json:"apiVersion"`
				Status string `json:"status"`
				Kind   string `json:"kind"`
			} `json:"helperAPIReadback"`
		} `json:"fixtures"`
		Cells []struct {
			ID                   string          `json:"id"`
			Group                string          `json:"group"`
			API                  string          `json:"api"`
			Route                string          `json:"route"`
			SourceKey            string          `json:"sourceKey"`
			CapturedSourceSHA256 string          `json:"capturedSourceSHA256"`
			NativeAnswerSHA256   string          `json:"nativeAnswerSHA256"`
			NativeValueSHA256    string          `json:"nativeValueSHA256"`
			OrgTSVSHA256         string          `json:"orgTSVSHA256"`
			OrgRunSHA256         string          `json:"orgRunSHA256"`
			Native               json.RawMessage `json:"native"`
		} `json:"cells"`
	}
	raw, err := os.ReadFile("testdata/conformance/hierarchy_trace_extended_native.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	checkHash := func(source, want string) {
		t.Helper()
		sum := sha256.Sum256([]byte(source))
		if hex.EncodeToString(sum[:]) != want {
			t.Fatal("captured source or value bytes changed")
		}
	}
	if strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" || strings.Join(data.Routes, ",") != "anonymous,istest" || data.NativeObservationCount != 120 || len(data.Cells) != 120 {
		t.Fatal("incomplete extended native matrix")
	}
	seen := map[string]bool{}
	for _, cell := range data.Cells {
		key := cell.ID + "@" + cell.API + "@" + cell.Route
		if seen[key] || !strings.HasPrefix(cell.ID, cell.Group+"-") || cell.SourceKey != cell.ID+"/"+cell.Route {
			t.Fatalf("duplicate or invalid native cell: %s", key)
		}
		seen[key] = true
		for _, hash := range []string{cell.NativeAnswerSHA256, cell.OrgTSVSHA256, cell.OrgRunSHA256} {
			if decoded, err := hex.DecodeString(hash); err != nil || len(decoded) != sha256.Size {
				t.Fatalf("unbound native cell: %s", key)
			}
		}
		if cell.Group != group {
			continue
		}
		t.Run(key, func(t *testing.T) {
			source, found := data.Sources[cell.SourceKey]
			if !found {
				t.Fatal("missing captured source template")
			}
			checkHash(source.Template, source.TemplateSHA256)
			checkHash(source.Measurement, source.MeasurementSHA256)
			fixture, found := data.Fixtures[cell.API]
			if !found {
				t.Fatal("missing API-specific native fixture")
			}
			// The capture renderer substitutes identifiers and quoted patterns alike.
			// Preserve its longest-first, single-pass substitution transparently.
			bindingNames := make([]string, 0, len(fixture.Bindings))
			for name := range fixture.Bindings {
				bindingNames = append(bindingNames, name)
			}
			sort.Slice(bindingNames, func(i, j int) bool {
				if len(bindingNames[i]) != len(bindingNames[j]) {
					return len(bindingNames[i]) > len(bindingNames[j])
				}
				return bindingNames[i] < bindingNames[j]
			})
			var replacements []string
			for _, name := range bindingNames {
				replacements = append(replacements, name, fixture.Bindings[name])
			}
			renderer := strings.NewReplacer(replacements...)
			capturedSource := renderer.Replace(source.Template)
			checkHash(capturedSource, cell.CapturedSourceSHA256)
			if !strings.Contains(capturedSource, renderer.Replace(source.Measurement)) {
				t.Fatal("measurement no longer appears in captured source")
			}
			executionSource := capturedSource
			if source.TestContext == "setup" {
				prefix, _, found := strings.Cut(capturedSource, "String capturePayload=JSON.serialize(r);")
				if !found || source.MethodName != "captureSeed" {
					t.Fatal("missing captured setup transport boundary")
				}
				executionSource = prefix + "System.assert(false,'C126_RESULT|" + cell.ID + "|'+JSON.serialize(r));}\n}\n"
			}
			classes := map[string]string{}
			for name, helper := range fixture.Classes {
				checkHash(helper, fixture.ClassSHA256[name])
				readback, found := fixture.HelperAPIReadback[name]
				if !found || readback.API != cell.API || readback.Status != "Active" || readback.Kind != "ApexClass" {
					t.Fatal("missing active helper API readback")
				}
				classes[name] = helper
			}
			objects := map[string]string{}
			for name, metadata := range fixture.Objects {
				checkHash(metadata, fixture.ObjectSHA256[name])
				// The native recorder redacts URLs, including the public metadata
				// namespace. Restore only that standard namespace for local parsing.
				objects[name] = strings.ReplaceAll(metadata, `xmlns="[URL]"`, `xmlns="http://soap.sforce.com/2006/04/metadata"`)
			}
			if cell.Route == "istest" {
				classes[source.ClassName] = executionSource
			}
			index := nativeBoundaryIndex(t, cell.API, classes, objects)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native setup accepted; diagnostics %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = cell.API
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			machine := runner.newMachine()
			var execErr error
			if cell.Route == "anonymous" {
				program, err := vm.CompileAnonymousWithOptions(executionSource, vm.CompileOptions{APIVersion: cell.API})
				if err != nil {
					t.Fatal(err)
				}
				_, execErr = machine.Execute(program)
			} else if source.TestContext == "setup" {
				methods, methodErrors, programs, programErrors := compileTestSetupMethodsForClasses(index, map[string]bool{source.ClassName: true})
				if methodErrors[source.ClassName] != nil || programErrors[source.ClassName] != nil || len(methods[source.ClassName]) != 1 || len(programs[source.ClassName]) != 1 {
					t.Fatalf("setup compilation: %v %v", methodErrors[source.ClassName], programErrors[source.ClassName])
				}
				if err := machine.RegisterMethod(methods[source.ClassName][0]); err != nil {
					t.Fatal(err)
				}
				machine.EnableTestContext()
				_, execErr = machine.ExecuteInClass(programs[source.ClassName][0], source.ClassName)
			} else {
				cases := Discover(index, Options{SelectedClasses: []string{source.ClassName}, SelectedMethod: source.MethodName})
				if len(cases) != 1 {
					t.Fatalf("named discovery: %d", len(cases))
				}
				methods, methodErrors := compileTestMethods(cases)
				programs, programErrors := compileTestInvokePrograms(cases)
				caseKey := testCaseKey(cases[0])
				if methodErrors[caseKey] != nil || programErrors[caseKey] != nil {
					t.Fatalf("named compilation: %v %v", methodErrors[caseKey], programErrors[caseKey])
				}
				if err := machine.RegisterMethod(methods[caseKey]); err != nil {
					t.Fatal(err)
				}
				machine.EnableTestContext()
				_, execErr = machine.ExecuteInClass(programs[caseKey], source.ClassName)
			}
			var runtimeErr *vm.RuntimeError
			if !errors.As(execErr, &runtimeErr) || !strings.HasSuffix(runtimeErr.Type, "AssertException") {
				t.Fatalf("missing assertion transport: %v", execErr)
			}
			_, payload, found := strings.Cut(runtimeErr.ExceptionMessage(), "C126_RESULT|"+cell.ID+"|")
			if !found {
				t.Fatalf("missing observation marker: %v", execErr)
			}
			var got, want any
			if err := json.Unmarshal([]byte(payload), &got); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(cell.Native, &want); err != nil {
				t.Fatal(err)
			}
			canonical, err := json.Marshal(want)
			if err != nil {
				t.Fatal(err)
			}
			checkHash(string(canonical), cell.NativeValueSHA256)
			got = hierarchyTraceRedactedIDs(t, got, want)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("native %s; Glade %s", cell.Native, payload)
			}
		})
	}
	for _, api := range data.APIVersions {
		for _, route := range data.Routes {
			for _, group := range []string{"C02", "C03"} {
				count := 10
				if group == "C03" {
					count = 9
				}
				for n := 1; n <= count; n++ {
					key := fmt.Sprintf("%s-%02d@%s@%s", group, n, api, route)
					if !seen[key] {
						t.Fatalf("missing measured cell: %s", key)
					}
				}
			}
		}
		for _, id := range []string{"C03-01-METHOD", "C03-02-METHOD"} {
			if !seen[id+"@"+api+"@istest"] {
				t.Fatalf("missing ordinary-method twin: %s/%s", id, api)
			}
		}
	}
}

// Only a value explicitly redacted by the native recorder is projected. Extra
// fields, missing fields, reference-identity answers and trace bytes stay exact.
func hierarchyTraceRedactedIDs(t *testing.T, got, want any) any {
	t.Helper()
	if text, ok := want.(string); ok && text == "[ID]" {
		actual, ok := got.(string)
		if !ok || storage.ValidateID(storage.ID(actual)) != nil {
			t.Fatalf("native redacted an Id; local value is %v", got)
		}
		return "[ID]"
	}
	if expected, ok := want.(map[string]any); ok {
		if actual, ok := got.(map[string]any); ok {
			for key, value := range actual {
				if expectedValue, found := expected[key]; found {
					actual[key] = hierarchyTraceRedactedIDs(t, value, expectedValue)
				}
			}
		}
	}
	return got
}
