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

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

type setNullNumericExpected struct {
	Outcome          string          `json:"outcome"`
	Value            string          `json:"value,omitempty"`
	Primary          string          `json:"primary,omitempty"`
	ExceptionType    string          `json:"exceptionType,omitempty"`
	Message          string          `json:"message,omitempty"`
	Native           json.RawMessage `json:"native"`
	GladeDisposition string          `json:"gladeDisposition,omitempty"`
	CarryReason      string          `json:"carryReason,omitempty"`
}
type setNullNumericRoute struct {
	Anonymous     string                            `json:"anonymous,omitempty"`
	Classes       map[string]string                 `json:"classes,omitempty"`
	Tests         []string                          `json:"tests,omitempty"`
	SourceSHA256  map[string]string                 `json:"sourceSha256"`
	ExpectedByAPI map[string]setNullNumericExpected `json:"expectedByAPI"`
}
type setNullNumericRow struct {
	ID     string                         `json:"id"`
	Routes map[string]setNullNumericRoute `json:"routes"`
}

// Keep captured source and its assertion envelope unchanged. Each route/API
// checks compilation before executing on a private shared-runner clone.
func runSetNullNumericConformance(t *testing.T, cluster string, shapes, cells, carried int) {
	t.Helper()
	var data struct {
		APIVersions              []string            `json:"apiVersions"`
		Cluster                  string              `json:"cluster"`
		ShapeCount               int                 `json:"shapeCount"`
		NativeObservationCount   int                 `json:"nativeObservationCount"`
		AssertedObservationCount int                 `json:"assertedObservationCount"`
		CarriedObservationCount  int                 `json:"carriedObservationCount"`
		Rows                     []setNullNumericRow `json:"rows"`
	}
	raw, err := os.ReadFile(filepath.Join("testdata/conformance/set_null_numeric", cluster+".json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data.Cluster != cluster || data.ShapeCount != shapes || len(data.Rows) != shapes || data.NativeObservationCount != cells || data.AssertedObservationCount != cells-carried || data.CarriedObservationCount != carried || strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" {
		t.Fatal("incomplete native matrix")
	}
	seen := map[string]bool{}
	observations := 0
	carriedObservations := 0
	for _, row := range data.Rows {
		if (!strings.HasPrefix(row.ID, "E23N") && !strings.HasPrefix(row.ID, "E23C")) || seen[row.ID] || len(row.Routes) == 0 {
			t.Fatalf("missing/repeated row or route: %s", row.ID)
		}
		seen[row.ID] = true
		if row.ID == "E23C004" || row.ID == "E23C005" {
			if len(row.Routes) != 1 || len(row.Routes["named"].ExpectedByAPI) != 3 {
				t.Fatal("missing named declaration control")
			}
		} else if len(row.Routes) != 2 || len(row.Routes["anonymous"].ExpectedByAPI) != 3 || len(row.Routes["isTest"].ExpectedByAPI) != 3 {
			t.Fatal("missing anonymous or @IsTest route")
		}
		for _, route := range []string{"anonymous", "isTest", "named"} {
			source, ok := row.Routes[route]
			if !ok {
				continue
			}
			if len(source.ExpectedByAPI) != 3 {
				t.Fatal("missing independent native cells")
			}
			values := map[string]string{}
			if route == "anonymous" {
				values["anonymous"] = source.Anonymous
			}
			for name, text := range source.Classes {
				values["class:"+name] = text
			}
			if len(values) == 0 || len(values) != len(source.SourceSHA256) {
				t.Fatal("incomplete source hash matrix")
			}
			for name, text := range values {
				sum := sha256.Sum256([]byte(text))
				if text == "" || hex.EncodeToString(sum[:]) != source.SourceSHA256[name] {
					t.Fatalf("captured source changed: %s", name)
				}
			}
			for _, api := range data.APIVersions {
				want, ok := source.ExpectedByAPI[api]
				if !ok || !json.Valid(want.Native) {
					t.Fatal("missing native receipt")
				}
				if want.GladeDisposition != "" || want.CarryReason != "" {
					if row.ID != "E23C005" || route != "named" || want.Outcome != "reject" || want.GladeDisposition != "uncaptured-by-Glade/known gap" || want.CarryReason != "native rejects a String literal in an Integer field initializer; Glade sema does not check field initializers" {
						t.Fatal("unsupported native carry")
					}
					carriedObservations++
				}
				switch want.Outcome {
				case "runtime":
					if !json.Valid([]byte(want.Value)) {
						t.Fatal("invalid native payload")
					}
				case "reject":
					if want.Primary == "" {
						t.Fatal("empty native compile diagnostic")
					}
				case "accept":
					if route != "named" {
						t.Fatal("compile acceptance requires named route")
					}
				case "uncaught":
					if want.ExceptionType == "" || want.Message == "" {
						t.Fatal("empty native exception")
					}
				default:
					t.Fatalf("non-answer admitted: %s", want.Outcome)
				}
				observations++
			}
		}
	}
	if observations != cells || carriedObservations != carried {
		t.Fatalf("native cells=%d, carried=%d, want=%d/%d", observations, carriedObservations, cells, carried)
	}
	for _, api := range data.APIVersions {
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			// Each runtime cell executes on a private clone of the API base.
			counts := newConformanceCountLog(t, "SetNullNumeric-"+cluster, api)
			baseIndex := setNullNumericIndex(t, api, nil)
			org := orgFromIndex(baseIndex)
			org.APIVersion = api
			base := newConformanceRunner(t, baseIndex, conformanceRunnerOptions{Org: &org})
			for _, row := range data.Rows {
				t.Run(row.ID, func(t *testing.T) {
					for _, route := range []string{"anonymous", "isTest", "named"} {
						source, captured := row.Routes[route]
						if !captured {
							continue
						}
						t.Run(route, func(t *testing.T) {
							want := source.ExpectedByAPI[api]
							if want.CarryReason != "" {
								counts.TrackKind(t, route, "carried", 1)
								t.Logf("carried %s/%s/%s: %s; native rejection <%s>", row.ID, route, api, want.CarryReason, want.Primary)
								return
							}
							kind := "exact"
							if source.ExpectedByAPI[api].Outcome == "reject" {
								kind = "category"
							}
							counts.TrackKind(t, route, kind, 1)
							var execErr error
							if route == "anonymous" {
								index := setNullNumericIndex(t, api, source.Classes)
								setup := sema.Analyze(index)
								if index.HasErrors() || setup.HasErrors() {
									t.Fatalf("native setup accepted; Glade diagnostics=%v %v", index.Diagnostics, setup.Diagnostics)
								}
								analysis := sema.AnalyzeAnonymous(index, source.Anonymous, api)
								program, compileErr := vm.CompileAnonymousWithOptions(source.Anonymous, vm.CompileOptions{APIVersion: api})
								setNullNumericCompile(t, want, analysis.Diagnostics, compileErr)
								if want.Outcome == "reject" {
									return
								}
								runner := newConformanceRunner(t, index, conformanceRunnerOptions{Base: base, LinkProject: true, Org: &org})
								_, execErr = runner.execute(program)
							} else {
								index := setNullNumericIndex(t, api, source.Classes)
								analysis := sema.Analyze(index)
								diagnostics := append(append([]diagnostic.Diagnostic{}, index.Diagnostics...), analysis.Diagnostics...)
								setNullNumericCompile(t, want, diagnostics, nil)
								if want.Outcome == "reject" || want.Outcome == "accept" {
									return
								}
								if len(source.Tests) != 1 {
									t.Fatal("one captured test class required")
								}
								cases := Discover(index, Options{SelectedClasses: source.Tests, SelectedMethod: "observe"})
								if len(cases) != 1 {
									t.Fatalf("named observation discovery: %d", len(cases))
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
							setNullNumericValue(t, row.ID, want, execErr)
						})
					}
				})
			}
		})
	}
}

func setNullNumericIndex(t *testing.T, api string, classes map[string]string) typesys.Index {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
	for name, source := range classes {
		path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
		writeFile(t, path, source)
		writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
	}
	return loadTestIndex(t, root)
}

// Compile rejection cells retain the exact native diagnostic as evidence.
// Glade must reject the same source unit; its diagnostic wording is independent.
func setNullNumericCompile(t *testing.T, want setNullNumericExpected, diagnostics []diagnostic.Diagnostic, compileErr error) {
	t.Helper()
	var messages []string
	for _, d := range diagnostics {
		if d.Severity == diagnostic.Error {
			message := d.NativeMessage
			if message == "" {
				message = d.Message
			}
			messages = append(messages, message)
		}
	}
	if len(messages) == 0 && compileErr != nil {
		message := compileErr.Error()
		var syntax *vm.ApexSyntaxError
		if errors.As(compileErr, &syntax) && syntax.NativeMessage != "" {
			message = syntax.NativeMessage
		}
		messages = append(messages, message)
	}
	if want.Outcome == "reject" {
		if want.Primary == "" || len(messages) == 0 {
			t.Fatalf("native rejection <%s>; Glade diagnostics=%v compile=%v", want.Primary, messages, compileErr)
		}
		return
	}
	if len(messages) != 0 || compileErr != nil {
		t.Fatalf("native compilation accepted; Glade diagnostics=%v compile=%v", messages, compileErr)
	}
}

func setNullNumericExceptionType(name string) string {
	if !strings.Contains(name, ".") {
		return "System." + name
	}
	return name
}

func setNullNumericValue(t *testing.T, id string, want setNullNumericExpected, execErr error) {
	t.Helper()
	var runtimeErr *vm.RuntimeError
	if execErr == nil || !errors.As(execErr, &runtimeErr) {
		t.Fatalf("native %s; missing Glade exception: %v", want.Outcome, execErr)
	}
	// ExceptionMessage is the raw Apex message, unlike Error/report decoration.
	message := runtimeErr.ExceptionMessage()
	if want.Outcome == "uncaught" {
		if setNullNumericExceptionType(runtimeErr.Type) != want.ExceptionType || message != want.Message {
			t.Fatalf("native <%s: %s>; Glade <%s: %s>", want.ExceptionType, want.Message, runtimeErr.Type, message)
		}
		return
	}
	if want.Outcome != "runtime" || setNullNumericExceptionType(runtimeErr.Type) != "System.AssertException" {
		t.Fatalf("native marker payload; Glade <%v>", execErr)
	}
	marker := "E23_RESULT|" + id + "|"
	_, envelope, found := strings.Cut(message, marker)
	if !found || strings.Count(message, "E23_RESULT|") != 1 {
		t.Fatalf("missing native envelope: %s", message)
	}
	payload := []byte(envelope)
	var got, expected map[string]any
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(want.Value), &expected); err != nil {
		t.Fatal(err)
	}
	// Qualify only a caught exception type; messages and ordinary values stay exact.
	if value, ok := got["exceptionType"].(string); ok {
		got["exceptionType"] = setNullNumericExceptionType(value)
	}
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("native payload <%s>; Glade <%s>", want.Value, payload)
	}
}

func TestSetNullNumericDecimalConformance(t *testing.T) {
	runSetNullNumericConformance(t, "decimal", 9, 54, 0)
}
func TestSetNullNumericDoubleConformance(t *testing.T) {
	runSetNullNumericConformance(t, "double", 9, 54, 0)
}
func TestSetNullNumericIntegerConformance(t *testing.T) {
	runSetNullNumericConformance(t, "integer", 7, 42, 0)
}
func TestSetNullNumericLongConformance(t *testing.T) {
	runSetNullNumericConformance(t, "long", 7, 42, 0)
}
func TestSetNullNumericControlsConformance(t *testing.T) {
	runSetNullNumericConformance(t, "controls", 5, 24, 3)
}
