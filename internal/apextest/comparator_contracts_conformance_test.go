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

type comparatorContractRow struct {
	ID     string                               `json:"id"`
	Name   string                               `json:"name"`
	Marker string                               `json:"marker"`
	Routes map[string]genericInterfaceCastRoute `json:"routes"`
}

func TestComparatorContractsConformance(t *testing.T) {
	var data struct {
		APIVersions            []string                `json:"apiVersions"`
		ShapeCount             int                     `json:"shapeCount"`
		NativeObservationCount int                     `json:"nativeObservationCount"`
		Rows                   []comparatorContractRow `json:"rows"`
	}
	raw, err := os.ReadFile("testdata/conformance/comparator_contracts.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data.ShapeCount != 19 || len(data.Rows) != 19 || data.NativeObservationCount != 114 || strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" {
		t.Fatal("incomplete native matrix")
	}
	seen := map[string]bool{}
	cells := 0
	for _, row := range data.Rows {
		if row.ID == "" || row.Name == "" || row.Marker == "" || seen[row.ID] || len(row.Routes) != 2 {
			t.Fatal("missing or repeated native shape")
		}
		seen[row.ID] = true
		for _, route := range []string{"anonymous", "isTest"} {
			captured, ok := row.Routes[route]
			if !ok || len(captured.SourcesByAPI) != 3 || len(captured.ExpectedByAPI) != 3 {
				t.Fatal("incomplete route matrix")
			}
			for _, api := range data.APIVersions {
				source, ok := captured.SourcesByAPI[api]
				if !ok {
					t.Fatal("missing source")
				}
				texts := map[string]string{}
				if route == "anonymous" {
					texts["anonymous"] = source.Anonymous
				}
				for name, body := range source.Classes {
					texts["class:"+name] = body
				}
				if len(texts) == 0 || len(texts) != len(source.SourceSHA256) {
					t.Fatal("incomplete source hashes")
				}
				for name, body := range texts {
					sum := sha256.Sum256([]byte(body))
					if body == "" || hex.EncodeToString(sum[:]) != source.SourceSHA256[name] {
						t.Fatal("captured source changed")
					}
				}
				want, ok := captured.ExpectedByAPI[api]
				if !ok || !json.Valid(want.Native) {
					t.Fatal("missing native answer")
				}
				switch want.Outcome {
				case "runtime":
					if !json.Valid([]byte(want.Value)) {
						t.Fatal("invalid native payload")
					}
				case "reject", "subject-reject":
					if strings.TrimSpace(want.Primary) == "" || want.RejectUnit == "" {
						t.Fatal("empty native rejection")
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
		t.Fatal("incomplete native cells")
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "ComparatorContracts", api)
			baseIndex := genericInterfaceCastIndex(t, api, nil)
			org := orgFromIndex(baseIndex)
			org.APIVersion = api
			base := newConformanceRunner(t, baseIndex, conformanceRunnerOptions{Org: &org})
			for _, row := range data.Rows {
				t.Run(row.Name, func(t *testing.T) {
					for _, route := range []string{"anonymous", "isTest"} {
						t.Run(route, func(t *testing.T) {
							source := row.Routes[route].SourcesByAPI[api]
							want := row.Routes[route].ExpectedByAPI[api]
							kind := "exact"
							if want.Outcome == "reject" || want.Outcome == "subject-reject" {
								kind = "category"
							}
							counts.TrackKind(t, route, kind, 1)
							index := genericInterfaceCastIndex(t, api, source.Classes)
							analysis := sema.Analyze(index)
							diagnostics := append(append([]diagnostic.Diagnostic{}, index.Diagnostics...), analysis.Diagnostics...)
							if want.Outcome == "subject-reject" {
								genericInterfaceCastCompile(t, want, diagnostics, nil)
								return
							}
							var execErr error
							if route == "anonymous" {
								if index.HasErrors() || analysis.HasErrors() {
									t.Fatalf("native fixture accepted; Glade diagnostics=%v", diagnostics)
								}
								checked := sema.AnalyzeAnonymous(index, source.Anonymous, api)
								program, compileErr := vm.CompileAnonymousWithOptions(source.Anonymous, vm.CompileOptions{APIVersion: api})
								genericInterfaceCastCompile(t, want, checked.Diagnostics, compileErr)
								if want.Outcome == "reject" {
									return
								}
								runner := newConformanceRunner(t, index, conformanceRunnerOptions{Base: base, LinkProject: true, Org: &org})
								_, execErr = runner.execute(program)
							} else {
								genericInterfaceCastCompile(t, want, diagnostics, nil)
								if want.Outcome == "reject" {
									return
								}
								cases := Discover(index, Options{SelectedClasses: source.Tests, SelectedMethod: "observe"})
								if len(source.Tests) != 1 || len(cases) != 1 {
									t.Fatal("named observation discovery differs")
								}
								methods, methodErrors := compileTestMethodsForIndex(&index, cases)
								programs, programErrors := compileTestInvokePrograms(cases)
								key := testCaseKey(cases[0])
								if methodErrors[key] != nil || programErrors[key] != nil {
									t.Fatalf("named compilation: %v %v", methodErrors[key], programErrors[key])
								}
								runner := newConformanceRunner(t, index, conformanceRunnerOptions{Base: base, LinkProject: true, Org: &org})
								machine := runner.newMachine()
								if err := machine.RegisterMethod(methods[key]); err != nil {
									t.Fatal(err)
								}
								machine.EnableTestContext()
								_, execErr = machine.ExecuteInClass(programs[key], cases[0].ClassName)
							}
							comparatorContractValue(t, row.Marker, want, execErr)
						})
					}
				})
			}
		})
	}
}

func comparatorContractValue(t *testing.T, marker string, want genericInterfaceCastExpected, execErr error) {
	t.Helper()
	var runtimeErr *vm.RuntimeError
	if !errors.As(execErr, &runtimeErr) {
		t.Fatalf("native observation missing: %v", execErr)
	}
	message := runtimeErr.ExceptionMessage()
	if want.Outcome == "uncaught" {
		if genericInterfaceCastExceptionType(runtimeErr.Type) != want.ExceptionType || message != want.Message {
			t.Fatalf("uncaught exception differs: %v", execErr)
		}
		return
	}
	if genericInterfaceCastExceptionType(runtimeErr.Type) != "System.AssertException" || strings.Count(message, marker) != 1 {
		t.Fatalf("native observation envelope differs: %v", execErr)
	}
	_, payload, found := strings.Cut(message, marker)
	if !found {
		t.Fatal("missing observation payload")
	}
	var got, expected map[string]any
	if err := json.Unmarshal([]byte(payload), &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(want.Value), &expected); err != nil {
		t.Fatal(err)
	}
	if name, ok := got["exceptionType"].(string); ok {
		got["exceptionType"] = genericInterfaceCastExceptionType(name)
	}
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("native payload <%s>; Glade <%s>", want.Value, payload)
	}
}
