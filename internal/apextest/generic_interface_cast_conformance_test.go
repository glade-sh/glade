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
	"strconv"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

type genericInterfaceCastExpected struct {
	Outcome       string          `json:"outcome"`
	Value         string          `json:"value,omitempty"`
	RejectUnit    string          `json:"rejectUnit,omitempty"`
	Primary       string          `json:"primary,omitempty"`
	ExceptionType string          `json:"exceptionType,omitempty"`
	Message       string          `json:"message,omitempty"`
	Native        json.RawMessage `json:"native"`
}
type genericInterfaceCastSource struct {
	Anonymous    string            `json:"anonymous,omitempty"`
	Classes      map[string]string `json:"classes,omitempty"`
	Tests        []string          `json:"tests,omitempty"`
	SourceSHA256 map[string]string `json:"sourceSha256"`
}
type genericInterfaceCastRoute struct {
	SourcesByAPI  map[string]genericInterfaceCastSource   `json:"sourcesByAPI"`
	ExpectedByAPI map[string]genericInterfaceCastExpected `json:"expectedByAPI"`
}
type genericInterfaceCastRow struct {
	ID          string                               `json:"id"`
	Disposition string                               `json:"disposition"`
	Routes      map[string]genericInterfaceCastRoute `json:"routes"`
}

// Keep captured source and its assertion envelope unchanged. Each route/API
// checks compilation before executing on a private shared-runner clone.
func runGenericInterfaceCastConformance(t *testing.T, cluster string, shapes, cells int) {
	t.Helper()
	var data struct {
		APIVersions            []string                  `json:"apiVersions"`
		Cluster                string                    `json:"cluster"`
		ShapeCount             int                       `json:"shapeCount"`
		NativeObservationCount int                       `json:"nativeObservationCount"`
		Rows                   []genericInterfaceCastRow `json:"rows"`
	}
	raw, err := os.ReadFile(filepath.Join("testdata/conformance/generic_interface_cast", cluster+".json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data.Cluster != cluster || data.ShapeCount != shapes || len(data.Rows) != shapes || data.NativeObservationCount != cells || strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" {
		t.Fatal("incomplete native matrix")
	}
	seen := map[string]bool{}
	observations := 0
	for _, row := range data.Rows {
		if row.ID == "" || seen[row.ID] || len(row.Routes) != 2 {
			t.Fatalf("missing/repeated row or route: %s", row.ID)
		}
		seen[row.ID] = true
		if row.Disposition != "conformance" && (row.Disposition != "transport-control" || cluster != "transport-control") {
			t.Fatal("unsupported disposition")
		}
		for _, route := range []string{"anonymous", "isTest"} {
			source, ok := row.Routes[route]
			if !ok || len(source.ExpectedByAPI) != 3 {
				t.Fatal("missing independent native cells")
			}
			for _, api := range data.APIVersions {
				captured, ok := source.SourcesByAPI[api]
				if !ok {
					t.Fatal("missing captured source")
				}
				values := map[string]string{}
				if captured.Anonymous != "" {
					values["anonymous"] = captured.Anonymous
				}
				for name, body := range captured.Classes {
					values["class:"+name] = body
				}
				if len(values) == 0 || len(values) != len(captured.SourceSHA256) {
					t.Fatal("incomplete source hash matrix")
				}
				for name, body := range values {
					sum := sha256.Sum256([]byte(body))
					if body == "" || hex.EncodeToString(sum[:]) != captured.SourceSHA256[name] {
						t.Fatalf("captured source changed: %s", name)
					}
				}
				want, ok := source.ExpectedByAPI[api]
				if !ok || !json.Valid(want.Native) {
					t.Fatal("missing native receipt")
				}
				switch want.Outcome {
				case "runtime":
					if !json.Valid([]byte(want.Value)) {
						t.Fatal("invalid native payload")
					}
				case "reject", "subject-reject":
					if want.Primary == "" || want.RejectUnit == "" {
						t.Fatal("empty native compile diagnostic")
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
	if observations != cells {
		t.Fatalf("native cells=%d, want=%d", observations, cells)
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			// Transport controls retain their own count label, separate from GI matches.
			counts := newConformanceCountLog(t, "GenericInterfaceCast-"+cluster, api)
			baseIndex := genericInterfaceCastIndex(t, api, nil)
			org := orgFromIndex(baseIndex)
			org.APIVersion = api
			base := newConformanceRunner(t, baseIndex, conformanceRunnerOptions{Org: &org})
			for _, row := range data.Rows {
				t.Run(row.ID, func(t *testing.T) {
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
							setup := sema.Analyze(index)
							diagnostics := append(append([]diagnostic.Diagnostic{}, index.Diagnostics...), setup.Diagnostics...)
							if want.Outcome == "subject-reject" {
								genericInterfaceCastCompile(t, want, diagnostics, nil)
								return
							}
							var execErr error
							if route == "anonymous" {
								if index.HasErrors() || setup.HasErrors() {
									t.Fatalf("native setup accepted; Glade diagnostics=%v", diagnostics)
								}
								analysis := sema.AnalyzeAnonymous(index, source.Anonymous, api)
								program, compileErr := vm.CompileAnonymousWithOptions(source.Anonymous, vm.CompileOptions{APIVersion: api})
								genericInterfaceCastCompile(t, want, analysis.Diagnostics, compileErr)
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
							genericInterfaceCastValue(t, row.ID, want, execErr)
						})
					}
				})
			}
		})
	}
}

func genericInterfaceCastIndex(t *testing.T, api string, classes map[string]string) typesys.Index {
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

func genericInterfaceCastCompile(t *testing.T, want genericInterfaceCastExpected, diagnostics []diagnostic.Diagnostic, compileErr error) {
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
	if want.Outcome == "reject" || want.Outcome == "subject-reject" {
		// Native rejects this unit; local wording is intentionally independent.
		rejected := compileErr != nil && want.RejectUnit == "anonymous"
		for _, d := range diagnostics {
			if d.Severity != diagnostic.Error {
				continue
			}
			if want.RejectUnit == "anonymous" || filepath.Base(d.File) == want.RejectUnit+".cls" {
				rejected = true
			}
		}
		if want.Primary == "" || !rejected {
			t.Fatalf("native rejects unit %s <%s>; Glade diagnostics=%v compile=%v", want.RejectUnit, want.Primary, diagnostics, compileErr)
		}
		return
	}
	if len(messages) != 0 || compileErr != nil {
		t.Fatalf("native compilation accepted; Glade diagnostics=%v compile=%v", messages, compileErr)
	}
}

func genericInterfaceCastExceptionType(name string) string {
	if !strings.Contains(name, ".") {
		return "System." + name
	}
	return name
}

func genericInterfaceCastValue(t *testing.T, id string, want genericInterfaceCastExpected, execErr error) {
	t.Helper()
	var runtimeErr *vm.RuntimeError
	if execErr == nil || !errors.As(execErr, &runtimeErr) {
		t.Fatalf("native %s; missing Glade exception: %v", want.Outcome, execErr)
	}
	// ExceptionMessage is the raw Apex message, unlike Error/report decoration.
	message := runtimeErr.ExceptionMessage()
	if want.Outcome == "uncaught" {
		if genericInterfaceCastExceptionType(runtimeErr.Type) != want.ExceptionType || message != want.Message {
			t.Fatalf("native <%s: %s>; Glade <%s: %s>", want.ExceptionType, want.Message, runtimeErr.Type, message)
		}
		return
	}
	if want.Outcome != "runtime" || genericInterfaceCastExceptionType(runtimeErr.Type) != "System.AssertException" {
		t.Fatalf("native marker payload; Glade <%v>", execErr)
	}
	marker := "C14|" + id + "|"
	_, envelope, found := strings.Cut(message, marker)
	parts := strings.Split(envelope, "|")
	if !found || strings.Count(message, "C14|") != 1 || len(parts) != 3 {
		t.Fatalf("missing native envelope: %s", message)
	}
	size, err := strconv.Atoi(parts[0])
	if err != nil {
		t.Fatal(err)
	}
	payload, err := hex.DecodeString(parts[2])
	if err != nil || len(payload) != size || size > 240 {
		t.Fatal("truncated or invalid native payload envelope")
	}
	sum := sha256.Sum256(payload)
	if hex.EncodeToString(sum[:]) != parts[1] {
		t.Fatal("payload digest differs")
	}
	var got, expected map[string]any
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(want.Value), &expected); err != nil {
		t.Fatal(err)
	}
	// Qualify only a returned exception type. Messages and ordinary values stay exact.
	if value, ok := got["value"].(string); ok && strings.HasPrefix(value, "EXC|") {
		fields := strings.SplitN(value, "|", 3)
		if len(fields) != 3 {
			t.Fatal("invalid caught exception payload")
		}
		got["value"] = "EXC|" + genericInterfaceCastExceptionType(fields[1]) + "|" + fields[2]
	}
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("native payload <%s>; Glade <%s>", want.Value, payload)
	}
}

func TestGenericInterfaceCastSeedsConformance(t *testing.T) {
	runGenericInterfaceCastConformance(t, "GI-seeds", 8, 48)
}

func TestGenericInterfaceCastInterfaceMatrixConformance(t *testing.T) {
	runGenericInterfaceCastConformance(t, "interface-matrix", 24, 144)
}

func TestGenericInterfaceCastNongenericBoundariesConformance(t *testing.T) {
	runGenericInterfaceCastConformance(t, "nongeneric-boundaries", 8, 48)
}

func TestGenericInterfaceCastDifferentArgumentConformance(t *testing.T) {
	runGenericInterfaceCastConformance(t, "different-argument", 5, 30)
}

func TestGenericInterfaceCastInnerClassesConformance(t *testing.T) {
	runGenericInterfaceCastConformance(t, "inner-classes", 6, 36)
}

func TestGenericInterfaceCastTopLevelControlsConformance(t *testing.T) {
	runGenericInterfaceCastConformance(t, "top-level-interface-controls", 1, 6)
}

func TestGenericInterfaceCastStaticInitializerConformance(t *testing.T) {
	runGenericInterfaceCastConformance(t, "static-initializer", 2, 12)
}

func TestGenericInterfaceCastApplicationBoundariesConformance(t *testing.T) {
	runGenericInterfaceCastConformance(t, "application-boundaries", 5, 30)
}

func TestGenericInterfaceCastTransportControlConformance(t *testing.T) {
	runGenericInterfaceCastConformance(t, "transport-control", 3, 18)
}
