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

type listSObjectCastExpected struct {
	Outcome       string          `json:"outcome"`
	Value         string          `json:"value,omitempty"`
	Primary       string          `json:"primary,omitempty"`
	ExceptionType string          `json:"exceptionType,omitempty"`
	Message       string          `json:"message,omitempty"`
	Native        json.RawMessage `json:"native"`
}
type listSObjectCastRoute struct {
	Anonymous     string                             `json:"anonymous,omitempty"`
	Classes       map[string]string                  `json:"classes,omitempty"`
	Tests         []string                           `json:"tests,omitempty"`
	SourceSHA256  map[string]string                  `json:"sourceSha256"`
	ExpectedByAPI map[string]listSObjectCastExpected `json:"expectedByAPI"`
}
type listSObjectCastRow struct {
	ID          string                          `json:"id"`
	Disposition string                          `json:"disposition"`
	Routes      map[string]listSObjectCastRoute `json:"routes"`
}

// Keep captured source and its assertion envelope unchanged. Each route/API
// checks compilation before executing on a private shared-runner clone.
func runListSObjectCastConformance(t *testing.T, cluster string, shapes, cells int) {
	t.Helper()
	var data struct {
		APIVersions            []string             `json:"apiVersions"`
		Cluster                string               `json:"cluster"`
		ShapeCount             int                  `json:"shapeCount"`
		NativeObservationCount int                  `json:"nativeObservationCount"`
		Rows                   []listSObjectCastRow `json:"rows"`
	}
	raw, err := os.ReadFile(filepath.Join("testdata/conformance/list_sobject_cast", cluster+".json"))
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
			values := map[string]string{}
			if route == "anonymous" {
				values["anonymous"] = source.Anonymous
			} else {
				for name, text := range source.Classes {
					values["class:"+name] = text
				}
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
				switch want.Outcome {
				case "runtime":
					if !json.Valid([]byte(want.Value)) {
						t.Fatal("invalid native payload")
					}
				case "reject":
					if want.Primary == "" {
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
			// Transport controls retain their own count label, separate from CL matches.
			counts := newConformanceCountLog(t, "ListSObjectCast-"+cluster, api)
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
							var execErr error
							if route == "anonymous" {
								analysis := sema.AnalyzeAnonymous(baseIndex, source.Anonymous, api)
								program, compileErr := vm.CompileAnonymousWithOptions(source.Anonymous, vm.CompileOptions{APIVersion: api})
								listSObjectCastCompile(t, want, analysis.Diagnostics, compileErr)
								if want.Outcome == "reject" {
									return
								}
								_, execErr = base.execute(program)
							} else {
								index := listSObjectCastIndex(t, api, source.Classes)
								analysis := sema.Analyze(index)
								diagnostics := append(append([]diagnostic.Diagnostic{}, index.Diagnostics...), analysis.Diagnostics...)
								listSObjectCastCompile(t, want, diagnostics, nil)
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
							listSObjectCastValue(t, row.ID, want, execErr)
						})
					}
				})
			}
		})
	}
}

func listSObjectCastIndex(t *testing.T, api string, classes map[string]string) typesys.Index {
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

func listSObjectCastCompile(t *testing.T, want listSObjectCastExpected, diagnostics []diagnostic.Diagnostic, compileErr error) {
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
		if len(messages) == 0 || messages[0] != want.Primary {
			t.Fatalf("native rejection <%s>; Glade diagnostics=%v compile=%v", want.Primary, messages, compileErr)
		}
		return
	}
	if len(messages) != 0 || compileErr != nil {
		t.Fatalf("native compilation accepted; Glade diagnostics=%v compile=%v", messages, compileErr)
	}
}

func listSObjectCastExceptionType(name string) string {
	if !strings.Contains(name, ".") {
		return "System." + name
	}
	return name
}

func listSObjectCastValue(t *testing.T, id string, want listSObjectCastExpected, execErr error) {
	t.Helper()
	var runtimeErr *vm.RuntimeError
	if execErr == nil || !errors.As(execErr, &runtimeErr) {
		t.Fatalf("native %s; missing Glade exception: %v", want.Outcome, execErr)
	}
	// ExceptionMessage is the raw Apex message, unlike Error/report decoration.
	message := runtimeErr.ExceptionMessage()
	if want.Outcome == "uncaught" {
		if listSObjectCastExceptionType(runtimeErr.Type) != want.ExceptionType || message != want.Message {
			t.Fatalf("native <%s: %s>; Glade <%s: %s>", want.ExceptionType, want.Message, runtimeErr.Type, message)
		}
		return
	}
	if want.Outcome != "runtime" || listSObjectCastExceptionType(runtimeErr.Type) != "System.AssertException" {
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
		got["value"] = "EXC|" + listSObjectCastExceptionType(fields[1]) + "|" + fields[2]
	}
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("native payload <%s>; Glade <%s>", want.Value, payload)
	}
}

func TestListSObjectCastSeedsConformance(t *testing.T) {
	runListSObjectCastConformance(t, "CL-seeds", 10, 60)
}
func TestListSObjectCastListMatrixConformance(t *testing.T) {
	runListSObjectCastConformance(t, "list-matrix", 21, 126)
}
func TestListSObjectCastNullBoundariesConformance(t *testing.T) {
	runListSObjectCastConformance(t, "null-boundaries", 4, 24)
}
func TestListSObjectCastElementBoundariesConformance(t *testing.T) {
	runListSObjectCastConformance(t, "element-boundaries", 4, 24)
}
func TestListSObjectCastSOQLBoundariesConformance(t *testing.T) {
	runListSObjectCastConformance(t, "SOQL-boundaries", 4, 24)
}
func TestListSObjectCastMustNotApplyConformance(t *testing.T) {
	runListSObjectCastConformance(t, "must-not-apply", 3, 18)
}
func TestListSObjectCastDirectTypedCastsConformance(t *testing.T) {
	runListSObjectCastConformance(t, "direct-typed-casts", 9, 54)
}
func TestListSObjectCastListElementCastsConformance(t *testing.T) {
	runListSObjectCastConformance(t, "list-element-casts", 4, 24)
}
func TestListSObjectCastQueryListElementCastsConformance(t *testing.T) {
	runListSObjectCastConformance(t, "query-list-element-casts", 4, 24)
}
func TestListSObjectCastObjectListElementCastsConformance(t *testing.T) {
	runListSObjectCastConformance(t, "object-list-element-casts", 4, 24)
}
func TestListSObjectCastListUpcastControlsConformance(t *testing.T) {
	runListSObjectCastConformance(t, "list-upcast-controls", 2, 12)
}
func TestListSObjectCastTransportControlConformance(t *testing.T) {
	runListSObjectCastConformance(t, "transport-control", 3, 18)
}
