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
	"regexp"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

type stubDispatchExpectation struct {
	Outcome          string          `json:"outcome"`
	Value            json.RawMessage `json:"value"`
	Primary          string          `json:"primary"`
	ExceptionType    string          `json:"exceptionType"`
	ExceptionMessage string          `json:"exceptionMessage"`
	Native           json.RawMessage `json:"native"`
}

type stubDispatchSource struct {
	Anonymous    string                  `json:"anonymous"`
	Classes      map[string]string       `json:"classes"`
	Tests        []string                `json:"tests"`
	SourceSHA256 map[string]string       `json:"sourceSha256"`
	Expected     stubDispatchExpectation `json:"expected"`
	Carry        string                  `json:"carry,omitempty"`
}

type stubDispatchDeclarations struct {
	Classes      map[string]string `json:"classes"`
	SourceSHA256 map[string]string `json:"sourceSha256"`
}

// Every source version and route has an independent native outcome. The older
// source version is a same-release drift control, not an added support floor.
func TestStubDispatchOrgConformance(t *testing.T) {
	var data struct {
		APIVersions            []string                            `json:"apiVersions"`
		SupportedAPIs          []string                            `json:"supportedAPIs"`
		DriftControlAPIs       []string                            `json:"driftControlAPIs"`
		ShapeCount             int                                 `json:"shapeCount"`
		NativeObservationCount int                                 `json:"nativeObservationCount"`
		MarkerPrefix           string                              `json:"markerPrefix"`
		HelpersByAPI           map[string]stubDispatchDeclarations `json:"helpersByAPI"`
		Rows                   []struct {
			ID     string `json:"id"`
			Routes map[string]struct {
				ByAPI map[string]stubDispatchSource `json:"byAPI"`
			} `json:"routes"`
		} `json:"rows"`
	}
	raw, err := os.ReadFile("testdata/conformance/stub_dispatch.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIVersions, ",") != "59.0,62.0,63.0,64.0,65.0,66.0,67.0,68.0" ||
		strings.Join(data.SupportedAPIs, ",") != "62.0,63.0,64.0,65.0,66.0,67.0,68.0" ||
		strings.Join(data.DriftControlAPIs, ",") != "59.0" || data.ShapeCount != 50 ||
		len(data.Rows) != data.ShapeCount || data.NativeObservationCount != 800 || data.MarkerPrefix == "" {
		t.Fatal("incomplete native dispatch matrix")
	}
	seen := make(map[string]bool)
	for _, api := range data.APIVersions {
		helpers, ok := data.HelpersByAPI[api]
		if !ok || len(helpers.Classes) != 4 {
			t.Fatal("missing native helper declarations")
		}
		stubDispatchCheckSources(t, helpers.Classes, helpers.SourceSHA256)
	}
	for _, row := range data.Rows {
		if row.ID == "" || seen[row.ID] || len(row.Routes) != 2 {
			t.Fatal("missing or repeated native dispatch shape")
		}
		seen[row.ID] = true
		for _, route := range []string{"anonymous", "isTest"} {
			source, ok := row.Routes[route]
			if !ok || len(source.ByAPI) != len(data.APIVersions) {
				t.Fatal("missing independent native routes")
			}
			for _, api := range data.APIVersions {
				cell, ok := source.ByAPI[api]
				if !ok || !json.Valid(cell.Expected.Native) {
					t.Fatal("missing retained native answer")
				}
				texts := make(map[string]string)
				if cell.Anonymous != "" {
					texts["anonymous"] = cell.Anonymous
				}
				for name, text := range cell.Classes {
					texts["class:"+name] = text
				}
				stubDispatchCheckSources(t, texts, cell.SourceSHA256)
				switch cell.Expected.Outcome {
				case "runtime":
					if !json.Valid(cell.Expected.Value) {
						t.Fatal("missing native payload")
					}
				case "compile-rejected":
					if strings.TrimSpace(cell.Expected.Primary) == "" {
						t.Fatal("empty compile rejection is not an answer")
					}
				case "uncaught_anonymous", "uncaught_test_failure":
					if cell.Expected.ExceptionType == "" || cell.Expected.ExceptionMessage == "" ||
						(route == "anonymous") != (cell.Expected.Outcome == "uncaught_anonymous") {
						t.Fatal("missing raw uncaught native message")
					}
				default:
					t.Fatal("unknown native outcome")
				}
			}
		}
	}
	for _, api := range data.APIVersions {
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "StubDispatch", api)
			helpers := data.HelpersByAPI[api].Classes
			baseIndex := stubDispatchIndex(t, api, helpers, nil)
			baseDiagnostics := append(append([]diagnostic.Diagnostic{}, baseIndex.Diagnostics...), sema.Analyze(baseIndex).Diagnostics...)
			org := orgFromIndex(baseIndex)
			org.APIVersion = api
			base := newConformanceRunner(t, baseIndex, conformanceRunnerOptions{LinkProject: true, Org: &org})
			for _, row := range data.Rows {
				t.Run(row.ID, func(t *testing.T) {
					for _, route := range []string{"anonymous", "isTest"} {
						cell := row.Routes[route].ByAPI[api]
						kind := "exact"
						if cell.Carry != "" {
							kind = "carried"
						}
						counts.RunKind(t, route, row.ID, route, kind, func(t *testing.T) {
							if cell.Carry != "" {
								t.Log(cell.Carry)
								return
							}
							var execErr error
							if route == "anonymous" {
								analysis := sema.AnalyzeAnonymous(baseIndex, cell.Anonymous, api)
								diagnostics := append([]diagnostic.Diagnostic{}, baseDiagnostics...)
								diagnostics = append(diagnostics, analysis.Diagnostics...)
								program, compileErr := vm.CompileAnonymousWithOptions(cell.Anonymous, vm.CompileOptions{APIVersion: api})
								stubDispatchCompile(t, cell.Expected, diagnostics, compileErr)
								if cell.Expected.Outcome == "compile-rejected" {
									return
								}
								_, execErr = base.execute(program)
							} else {
								index := stubDispatchIndex(t, api, helpers, cell.Classes)
								diagnostics := append(append([]diagnostic.Diagnostic{}, index.Diagnostics...), sema.Analyze(index).Diagnostics...)
								stubDispatchCompile(t, cell.Expected, diagnostics, nil)
								if cell.Expected.Outcome == "compile-rejected" {
									return
								}
								if len(cell.Tests) != 1 {
									t.Fatal("one native test entry required")
								}
								cases := Discover(index, Options{SelectedClasses: cell.Tests, SelectedMethod: "observe"})
								if len(cases) != 1 {
									t.Fatal("native named entry not discovered")
								}
								methods, methodErrors := compileTestMethods(cases)
								programs, programErrors := compileTestInvokePrograms(cases)
								key := testCaseKey(cases[0])
								if methodErrors[key] != nil || programErrors[key] != nil {
									t.Fatalf("named entry compile: %v %v", methodErrors[key], programErrors[key])
								}
								runner := newConformanceRunner(t, index, conformanceRunnerOptions{Base: base, LinkProject: true, Org: &org})
								machine := runner.newMachine()
								if err := machine.RegisterMethod(methods[key]); err != nil {
									t.Fatal(err)
								}
								machine.EnableTestContext()
								_, execErr = machine.ExecuteInClass(programs[key], cases[0].ClassName)
							}
							stubDispatchCompare(t, data.MarkerPrefix+row.ID+"|", cell.Expected, execErr)
						})
					}
				})
			}
		})
	}
}

func stubDispatchIndex(t *testing.T, api string, helpers, classes map[string]string) typesys.Index {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
	for _, sources := range []map[string]string{helpers, classes} {
		for name, source := range sources {
			path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
			writeFile(t, path, source)
			writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
		}
	}
	return loadTestIndex(t, root)
}

func stubDispatchCheckSources(t *testing.T, sources, hashes map[string]string) {
	t.Helper()
	if len(sources) == 0 || len(sources) != len(hashes) {
		t.Fatal("incomplete source digest matrix")
	}
	for name, source := range sources {
		digest := sha256.Sum256([]byte(source))
		if hex.EncodeToString(digest[:]) != hashes[name] {
			t.Fatal("native source bytes changed")
		}
	}
}

func stubDispatchCompile(t *testing.T, want stubDispatchExpectation, diagnostics []diagnostic.Diagnostic, compileErr error) {
	t.Helper()
	var messages []string
	for _, item := range diagnostics {
		if item.Severity == diagnostic.Error {
			message := item.NativeMessage
			if message == "" {
				message = item.Message
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
	if want.Outcome == "compile-rejected" {
		if len(messages) == 0 || messages[0] == "" || messages[0] != want.Primary {
			t.Fatalf("native rejection <%s>; local diagnostics=%v compile=%v", want.Primary, messages, compileErr)
		}
		return
	}
	if len(messages) != 0 || compileErr != nil {
		t.Fatalf("native compilation accepted; local diagnostics=%v compile=%v", messages, compileErr)
	}
}

func stubDispatchCompare(t *testing.T, marker string, want stubDispatchExpectation, execErr error) {
	t.Helper()
	var runtimeErr *vm.RuntimeError
	if !errors.As(execErr, &runtimeErr) {
		t.Fatalf("native outcome missing: %v", execErr)
	}
	if want.Outcome != "runtime" {
		typ := stubDispatchBuiltinExceptionName(runtimeErr.Type)
		if typ != want.ExceptionType || typ+": "+runtimeErr.ExceptionMessage() != want.ExceptionMessage {
			t.Fatalf("native escaped <%s>; local <%s: %s>", want.ExceptionMessage, typ, runtimeErr.ExceptionMessage())
		}
		return
	}
	if stubDispatchBuiltinExceptionName(runtimeErr.Type) != "System.AssertException" {
		t.Fatalf("native payload marker missing: %v", execErr)
	}
	_, text, ok := strings.Cut(runtimeErr.ExceptionMessage(), marker)
	if !ok || strings.Count(runtimeErr.ExceptionMessage(), strings.Split(marker, "|")[0]+"|") != 1 {
		t.Fatalf("native marker framing missing: %v", execErr)
	}
	var actual, expected any
	if err := json.Unmarshal([]byte(text), &actual); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(want.Value, &expected); err != nil {
		t.Fatal(err)
	}
	actual = stubDispatchNormalize(actual)
	if !reflect.DeepEqual(actual, expected) {
		payload, _ := json.Marshal(actual)
		t.Fatalf("native payload <%s>; local <%s>", want.Value, payload)
	}
}

func stubDispatchBuiltinExceptionName(name string) string {
	switch name {
	case "AssertException", "MathException", "TypeException", "NullPointerException", "SObjectException":
		return "System." + name
	}
	return name
}

var stubDispatchRecordID = regexp.MustCompile(`(?:[0-9][A-Za-z0-9]{2}|a[A-Za-z0-9]{2})[A-Za-z0-9]{12}(?:[A-Za-z0-9]{3})?`)

func stubDispatchMaskIDs(text string) string {
	wordByte := func(b byte) bool {
		return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || b == '_'
	}
	var out strings.Builder
	previous := 0
	for _, span := range stubDispatchRecordID.FindAllStringIndex(text, -1) {
		start, end := span[0], span[1]
		if start > 0 && wordByte(text[start-1]) || end < len(text) && wordByte(text[end]) {
			continue
		}
		if !strings.ContainsAny(text[start:end], "0123456789") {
			continue
		}
		out.WriteString(text[previous:start])
		out.WriteString("[ID]")
		previous = end
	}
	out.WriteString(text[previous:])
	return out.String()
}

// Retain the capture's ID masking. Exception messages otherwise stay exact;
// only builtin type names in typed fields or helper snapshots are qualified.
func stubDispatchNormalize(value any) any {
	switch value := value.(type) {
	case string:
		text := stubDispatchMaskIDs(value)
		parts := strings.Split(text, "|")
		if len(parts) > 2 && parts[0] == "EXC" {
			parts[1] = stubDispatchBuiltinExceptionName(parts[1])
		}
		return strings.Join(parts, "|")
	case []any:
		out := make([]any, len(value))
		for i, item := range value {
			out[i] = stubDispatchNormalize(item)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(value))
		for key, item := range value {
			if key == "exceptionType" || key == "caught" {
				if name, ok := item.(string); ok {
					item = stubDispatchBuiltinExceptionName(name)
				}
			}
			out[key] = stubDispatchNormalize(item)
		}
		return out
	}
	return value
}
