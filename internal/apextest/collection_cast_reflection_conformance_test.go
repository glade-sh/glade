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

func TestCollectionCastReflectionConformance(t *testing.T) {
	var data struct {
		APIVersions            []string             `json:"apiVersions"`
		ShapeCount             int                  `json:"shapeCount"`
		NativeObservationCount int                  `json:"nativeObservationCount"`
		Marker                 string               `json:"marker"`
		Rows                   []listSObjectCastRow `json:"rows"`
	}
	raw, err := os.ReadFile("testdata/conformance/collection_cast_reflection.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" || data.ShapeCount != 16 || len(data.Rows) != 16 || data.NativeObservationCount != 96 || data.Marker == "" {
		t.Fatal("incomplete native matrix")
	}
	seen := map[string]bool{}
	cells := 0
	for _, row := range data.Rows {
		if row.ID == "" || seen[row.ID] || len(row.Routes) != 2 {
			t.Fatal("missing or repeated row/route")
		}
		seen[row.ID] = true
		for _, route := range []string{"anonymous", "isTest"} {
			source, ok := row.Routes[route]
			if !ok || len(source.ExpectedByAPI) != 3 {
				t.Fatal("missing native cells")
			}
			texts := map[string]string{}
			for name, text := range source.Classes {
				texts["class:"+name] = text
			}
			if route == "anonymous" {
				texts["anonymous"] = source.Anonymous
			}
			if len(texts) == 0 || len(texts) != len(source.SourceSHA256) {
				t.Fatal("incomplete source hashes")
			}
			for name, text := range texts {
				sum := sha256.Sum256([]byte(text))
				if text == "" || hex.EncodeToString(sum[:]) != source.SourceSHA256[name] {
					t.Fatalf("source changed: %s", name)
				}
			}
			for _, api := range data.APIVersions {
				want, ok := source.ExpectedByAPI[api]
				if !ok || !json.Valid(want.Native) {
					t.Fatal("missing native answer")
				}
				switch want.Outcome {
				case "runtime":
					if !json.Valid([]byte(want.Value)) {
						t.Fatal("invalid native payload")
					}
				case "reject":
					if want.Primary == "" {
						t.Fatal("empty native compile rejection")
					}
				case "uncaught":
					if want.ExceptionType == "" || want.Message == "" {
						t.Fatal("empty native exception")
					}
				default:
					t.Fatal("non-answer admitted")
				}
				cells++
			}
		}
	}
	if cells != data.NativeObservationCount {
		t.Fatal("native cell count differs")
	}
	for _, api := range data.APIVersions {
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "CollectionCastReflection", api)
			baseIndex := listSObjectCastIndex(t, api, nil)
			org := orgFromIndex(baseIndex)
			org.APIVersion = api
			base := newConformanceRunner(t, baseIndex, conformanceRunnerOptions{Org: &org})
			for _, row := range data.Rows {
				t.Run(row.ID, func(t *testing.T) {
					for _, route := range []string{"anonymous", "isTest"} {
						counts.Run(t, route, row.ID, route, func(t *testing.T) {
							source := row.Routes[route]
							want := source.ExpectedByAPI[api]
							index := listSObjectCastIndex(t, api, source.Classes)
							analysis := sema.Analyze(index)
							diagnostics := append(append([]diagnostic.Diagnostic{}, index.Diagnostics...), analysis.Diagnostics...)
							var execErr error
							if route == "anonymous" {
								anonymous := sema.AnalyzeAnonymous(index, source.Anonymous, api)
								diagnostics = append(diagnostics, anonymous.Diagnostics...)
								program, compileErr := vm.CompileAnonymousWithOptions(source.Anonymous, vm.CompileOptions{APIVersion: api})
								listSObjectCastCompile(t, want, diagnostics, compileErr)
								if want.Outcome == "reject" {
									return
								}
								runtime, err := CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
								if err != nil {
									t.Fatal(err)
								}
								runner := newConformanceRunner(t, index, conformanceRunnerOptions{Base: base, RequestRuntime: &runtime, Org: &org})
								_, execErr = runner.execute(program)
							} else {
								listSObjectCastCompile(t, want, diagnostics, nil)
								if want.Outcome == "reject" {
									return
								}
								if len(source.Tests) != 1 {
									t.Fatal("one observation class required")
								}
								cases := Discover(index, Options{SelectedClasses: source.Tests, SelectedMethod: "observe"})
								if len(cases) != 1 {
									t.Fatalf("observation discovery: %d", len(cases))
								}
								methods, methodErrors := compileTestMethodsForIndex(&index, cases)
								programs, programErrors := compileTestInvokePrograms(cases)
								key := testCaseKey(cases[0])
								if methodErrors[key] != nil || programErrors[key] != nil {
									t.Fatalf("named compile: %v %v", methodErrors[key], programErrors[key])
								}
								runner := newConformanceRunner(t, index, conformanceRunnerOptions{Base: base, LinkProject: true, Org: &org})
								machine := runner.newMachine()
								if err := machine.RegisterMethod(methods[key]); err != nil {
									t.Fatal(err)
								}
								machine.EnableTestContext()
								_, execErr = machine.ExecuteInClass(programs[key], cases[0].ClassName)
							}
							collectionCastReflectionValue(t, data.Marker+row.ID+"|", want, execErr)
						})
					}
				})
			}
		})
	}
}

func collectionCastReflectionValue(t *testing.T, marker string, want listSObjectCastExpected, execErr error) {
	t.Helper()
	var runtimeErr *vm.RuntimeError
	if !errors.As(execErr, &runtimeErr) {
		t.Fatalf("missing observation exception: %v", execErr)
	}
	message := runtimeErr.ExceptionMessage()
	if want.Outcome == "uncaught" {
		if listSObjectCastExceptionType(runtimeErr.Type) != want.ExceptionType || message != want.Message {
			t.Fatalf("native <%s: %s>; Glade <%s: %s>", want.ExceptionType, want.Message, runtimeErr.Type, message)
		}
		return
	}
	if listSObjectCastExceptionType(runtimeErr.Type) != "System.AssertException" {
		t.Fatalf("missing assertion marker: %v", execErr)
	}
	_, payload, found := strings.Cut(message, marker)
	if !found || strings.Count(message, marker) != 1 {
		t.Fatalf("missing payload: %s", message)
	}
	var got, expected map[string]any
	if err := json.Unmarshal([]byte(payload), &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(want.Value), &expected); err != nil {
		t.Fatal(err)
	}
	// Only exception type names are qualified; messages remain exact.
	for _, key := range []string{"exceptionType", "castType", "castExceptionType", "reflectionType"} {
		if name, ok := got[key].(string); ok {
			got[key] = listSObjectCastExceptionType(name)
		}
	}
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("native <%s>; Glade <%s>", want.Value, payload)
	}
}
