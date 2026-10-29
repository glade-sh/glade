package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

type stubRecorderCell struct {
	stubNamespaceCell
	Comparison      string `json:"comparison"`
	DiagnosticCode  string `json:"diagnosticCode"`
	Anonymous       string `json:"anonymous"`
	AnonymousSHA256 string `json:"anonymousSha256"`
	BindingPrefix   string `json:"bindingPrefix"`
}

type stubRecorderRow struct {
	ID, Group, Context, Route string
	CellsByAPI                map[string]stubRecorderCell `json:"cellsByAPI"`
}

type stubRecorderCapture struct {
	Namespace       string
	Fixtures        stubNamespaceSources
	ReceiptsByRoute map[string]map[string]string `json:"receiptsByRoute"`
}

// APIs, contexts and routes come from captured cells. Pending APIs have no
// expectations; another completed API needs only fixture data, not runner edits.
func runStubRecorderConformance(t *testing.T, group string) {
	t.Helper()
	var data struct {
		Marker                                                                                       string
		APIVersions                                                                                  []string `json:"apiVersions"`
		PendingAPIVersions                                                                           []string `json:"pendingApiVersions"`
		ShapeCount, CellCount, NativeObservationCount, NativeAssertionCount, CarriedObservationCount int
		Groups                                                                                       map[string]int
		CapturesByAPI                                                                                map[string]map[string]stubRecorderCapture `json:"capturesByAPI"`
		UncapturedCells                                                                              []json.RawMessage                         `json:"uncapturedCells"`
		DriftChecks                                                                                  []struct {
			APIs           []string `json:"apis"`
			ComparedCells  int
			DifferentCells []string
		} `json:"driftChecks"`
		Rows []stubRecorderRow
	}
	raw, err := os.ReadFile("testdata/conformance/stub_recorder_display.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data.Marker == "" || data.ShapeCount != 82 || len(data.Rows) != data.ShapeCount || len(data.UncapturedCells) != 0 || len(data.APIVersions) == 0 || len(data.CapturesByAPI) != len(data.APIVersions) {
		t.Fatal("invalid captured context matrix")
	}
	versions := map[string]bool{}
	for _, api := range data.APIVersions {
		if versions[api] || (api != "59.0" && api != "62.0" && api != "67.0" && api != "68.0") {
			t.Fatalf("invalid captured API: %s", api)
		}
		versions[api] = true
	}
	for _, api := range data.PendingAPIVersions {
		if versions[api] {
			t.Fatal("pending API has asserted cells")
		}
	}
	groups := map[string]int{}
	seen := map[string]bool{}
	cells, carried := 0, 0
	var rows []stubRecorderRow
	for _, row := range data.Rows {
		key := row.Context + "/" + row.Route + "/" + row.ID
		if row.ID == "" || seen[key] || len(row.CellsByAPI) != len(versions) || (row.Route != "isTest" && row.Route != "anonymous") {
			t.Fatalf("invalid captured row: %s", key)
		}
		seen[key] = true
		groups[row.Group]++
		for _, api := range data.APIVersions {
			cell, ok := row.CellsByAPI[api]
			capture, capturedContext := data.CapturesByAPI[api][row.Context]
			if !ok || !capturedContext || len(capture.Fixtures.Classes) != 8 || cell.BindingPrefix == "" || len(cell.Native) == 0 || len(cell.NativeReceipt) == 0 {
				t.Fatalf("missing native cell: %s/%s", api, key)
			}
			receipts := capture.ReceiptsByRoute[row.Route]
			for _, name := range []string{"orgTsvSha256", "orgRunSha256", "nativeJsonSha256", "fixtureZipSha256", "captureManifestSha256", "sourceHashesSha256", "captureCompletionSha256", "completionSha256"} {
				if decoded, err := hex.DecodeString(receipts[name]); err != nil || len(decoded) != sha256.Size {
					t.Fatalf("missing receipt hash: %s/%s/%s", api, key, name)
				}
			}
			if row.Route == "isTest" {
				if cell.TestClass == "" || len(cell.Classes) != 1 || cell.Classes[cell.TestClass] == "" || cell.Anonymous != "" {
					t.Fatal("invalid named source")
				}
				stubNamespaceCheckSources(t, cell.stubNamespaceSources)
			} else {
				sum := sha256.Sum256([]byte(cell.Anonymous))
				if cell.Anonymous == "" || cell.AnonymousSHA256 != hex.EncodeToString(sum[:]) || cell.TestClass != "" || len(cell.Classes) != 0 {
					t.Fatal("invalid anonymous source")
				}
			}
			switch cell.Outcome {
			case "runtime":
				if !json.Valid(cell.Value) || cell.Carry != "" {
					t.Fatal("invalid native runtime cell")
				}
			case "compile-rejected":
				if cell.Carry != "" {
					if cell.Comparison != "carry" || cell.Primary == "" {
						t.Fatal("invalid captured rejection carry")
					}
					carried++
				} else if cell.Primary == "" || cell.Comparison != "category" || cell.DiagnosticCode == "" {
					t.Fatal("invalid native rejection")
				}
			case "uncaught_test_failure", "uncaught_anonymous":
				if cell.ExceptionType == "" || cell.Message == "" || cell.Carry != "" {
					t.Fatal("empty uncaught artifact")
				}
			default:
				t.Fatal("native artifact requires an unasserted carry")
			}
			cells++
		}
		if row.Group == group {
			rows = append(rows, row)
		}
	}
	if !reflect.DeepEqual(groups, data.Groups) || len(rows) != data.Groups[group] || len(rows) == 0 || cells != data.CellCount || cells != data.NativeObservationCount || cells != data.ShapeCount*len(versions) || carried != data.CarriedObservationCount || cells-carried != data.NativeAssertionCount {
		t.Fatal("native matrix counts differ")
	}
	for _, check := range data.DriftChecks {
		if len(check.APIs) != 2 || !versions[check.APIs[0]] || !versions[check.APIs[1]] || check.ComparedCells != len(data.Rows) {
			t.Fatal("invalid native drift comparison")
		}
		different := []string{}
		for _, row := range data.Rows {
			normalized := func(api string) string {
				cell := row.CellsByAPI[api]
				var native any
				if err := json.Unmarshal(cell.Native, &native); err != nil {
					t.Fatal(err)
				}
				encoded, err := json.Marshal(native)
				if err != nil {
					t.Fatal(err)
				}
				return strings.ReplaceAll(string(encoded), cell.BindingPrefix, "Fixture")
			}
			if normalized(check.APIs[0]) != normalized(check.APIs[1]) {
				different = append(different, row.Context+"/"+row.Route+"/"+row.ID)
			}
		}
		if !reflect.DeepEqual(different, check.DifferentCells) {
			t.Fatalf("native API drift differs: %v", different)
		}
	}
	for _, api := range data.APIVersions {
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			for _, context := range []string{"namespace", "plain"} {
				t.Run(context, func(t *testing.T) {
					capture, ok := data.CapturesByAPI[api][context]
					if !ok || (context == "namespace" && capture.Namespace == "") || (context == "plain" && capture.Namespace != "") {
						t.Fatal("missing captured context")
					}
					counts := newConformanceCounts()
					defer counts.Log(t, "stub recorder", api, context+"/"+group)
					stubNamespaceCheckSources(t, capture.Fixtures)
					index := stubNamespaceIndex(t, api, capture.Namespace, capture.Fixtures.Classes)
					stubNamespaceCompile(t, stubNamespaceCell{}, append(index.Diagnostics, sema.Analyze(index).Diagnostics...))
					org := orgFromIndex(index)
					org.APIVersion = api
					base := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
					for _, row := range rows {
						if row.Context != context {
							continue
						}
						cell := row.CellsByAPI[api]
						if cell.Carry != "" {
							t.Logf("carried %s/%s/%s: %s", api, row.Route, row.ID, cell.Carry)
							continue
						}
						comparison := cell.Comparison
						if comparison == "" {
							comparison = "exact"
						}
						counts.RunKind(t, row.Route, row.ID, row.Route+"/"+row.ID, comparison, func(t *testing.T) {
							if row.Route == "anonymous" {
								analysis := sema.AnalyzeAnonymous(index, cell.Anonymous, api)
								if cell.Outcome == "compile-rejected" {
									stubRecorderCompile(t, cell, analysis.Diagnostics)
									return
								}
								stubNamespaceCompile(t, cell.stubNamespaceCell, analysis.Diagnostics)
								program, err := vm.CompileAnonymousWithOptions(cell.Anonymous, vm.CompileOptions{APIVersion: api})
								if err != nil {
									t.Fatal(err)
								}
								_, execErr := base.execute(program)
								stubRecorderValue(t, capture.Namespace, data.Marker+row.ID+"|", cell, execErr)
								return
							}
							classes := make(map[string]string, len(capture.Fixtures.Classes)+1)
							for name, source := range capture.Fixtures.Classes {
								classes[name] = source
							}
							for name, source := range cell.Classes {
								classes[name] = source
							}
							index := stubNamespaceIndex(t, api, capture.Namespace, classes)
							diagnostics := append(index.Diagnostics, sema.Analyze(index).Diagnostics...)
							if cell.Outcome == "compile-rejected" {
								stubRecorderCompile(t, cell, diagnostics)
								return
							}
							stubNamespaceCompile(t, cell.stubNamespaceCell, diagnostics)
							cases := Discover(index, Options{SelectedClasses: []string{cell.TestClass}, SelectedMethod: "observe"})
							if len(cases) != 1 {
								t.Fatalf("named discovery: %d", len(cases))
							}
							methods, methodErrors := compileTestMethods(cases)
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
							_, execErr := machine.ExecuteInClass(programs[key], cases[0].ClassName)
							stubRecorderValue(t, capture.Namespace, data.Marker+row.ID+"|", cell, execErr)
						})
					}
				})
			}
		})
	}
}

// Compile controls compare rejection kind and unit while keeping native prose
// in the fixture. Anonymous controls belong to the standalone anonymous unit.
func stubRecorderCompile(t *testing.T, want stubRecorderCell, diagnostics []diagnostic.Diagnostic) {
	t.Helper()
	var native struct {
		Outcome     string
		Diagnostics json.RawMessage
	}
	if err := json.Unmarshal(want.Native, &native); err != nil {
		t.Fatal(err)
	}
	if native.Outcome != "compile-rejected" {
		t.Fatal("invalid native rejected unit")
	}
	expectedFile := ""
	if want.TestClass != "" {
		var rejected []struct{ Name, Type, Problem string }
		if err := json.Unmarshal(native.Diagnostics, &rejected); err != nil {
			t.Fatal(err)
		}
		if len(rejected) != 1 || rejected[0].Name != want.TestClass || rejected[0].Type != "ApexClass" || rejected[0].Problem != want.Primary {
			t.Fatal("invalid native rejected class")
		}
		expectedFile = want.TestClass + ".cls"
	} else {
		var rejected struct{ CompileProblem string }
		if err := json.Unmarshal(native.Diagnostics, &rejected); err != nil {
			t.Fatal(err)
		}
		if rejected.CompileProblem != want.Primary {
			t.Fatal("invalid native anonymous rejection")
		}
	}
	rejected := false
	for _, d := range diagnostics {
		if d.Severity != diagnostic.Error {
			continue
		}
		if d.Code != want.DiagnosticCode || filepath.Base(d.File) != filepath.Base(expectedFile) {
			t.Fatalf("different rejection kind or unit: %+v", d)
		}
		rejected = true
	}
	if !rejected {
		t.Fatal("native rejects the unit; Glade accepted")
	}
}

func stubRecorderExceptionType(namespace, name string) string {
	if strings.Contains(name, ".") {
		return name
	}
	switch name {
	case "AssertException", "MathException", "TypeException", "NullPointerException":
		return "System." + name
	default:
		if namespace == "" {
			return name
		}
		return namespace + "." + name
	}
}

func stubRecorderValue(t *testing.T, namespace, marker string, want stubRecorderCell, execErr error) {
	t.Helper()
	var runtimeErr *vm.RuntimeError
	if execErr == nil || !errors.As(execErr, &runtimeErr) {
		t.Fatalf("native %s; missing exception: %v", want.Outcome, execErr)
	}
	if want.Outcome == "uncaught_test_failure" || want.Outcome == "uncaught_anonymous" {
		if stubRecorderExceptionType(namespace, runtimeErr.Type) != want.ExceptionType || runtimeErr.ExceptionMessage() != want.Message {
			t.Fatalf("native <%s: %s>; Glade <%s: %s>", want.ExceptionType, want.Message, runtimeErr.Type, runtimeErr.ExceptionMessage())
		}
		return
	}
	if stubRecorderExceptionType(namespace, runtimeErr.Type) != "System.AssertException" {
		t.Fatalf("native observation marker; Glade exception: %v", execErr)
	}
	message := runtimeErr.ExceptionMessage()
	_, payload, found := strings.Cut(message, marker)
	if !found || strings.Count(message, marker) != 1 {
		t.Fatalf("missing observation marker: %s", message)
	}
	var actual, expected map[string]any
	if err := json.Unmarshal([]byte(payload), &actual); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(want.Value, &expected); err != nil {
		t.Fatal(err)
	}
	if name, ok := actual["exceptionType"].(string); ok {
		actual["exceptionType"] = stubRecorderExceptionType(namespace, name)
	}
	if name, ok := actual["caught"].(string); ok {
		actual["caught"] = stubRecorderExceptionType(namespace, name)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("native payload %s; Glade payload %s", want.Value, payload)
	}
}

func TestStubRecorderControlsConformance(t *testing.T) { runStubRecorderConformance(t, "CONTROL") }
func TestStubRecorderDisplayConformance(t *testing.T)  { runStubRecorderConformance(t, "TEST_DISPLAY") }
func TestStubRecorderTypeSurfaceConformance(t *testing.T) {
	runStubRecorderConformance(t, "TYPE_SURFACE")
}
func TestStubRecorderTypeConformance(t *testing.T)      { runStubRecorderConformance(t, "TYPE") }
func TestStubRecorderProviderConformance(t *testing.T)  { runStubRecorderConformance(t, "PROVIDER") }
func TestStubRecorderNestedConformance(t *testing.T)    { runStubRecorderConformance(t, "NESTED") }
func TestStubRecorderFormatConformance(t *testing.T)    { runStubRecorderConformance(t, "FORMAT") }
func TestStubRecorderCastConformance(t *testing.T)      { runStubRecorderConformance(t, "CAST") }
func TestStubRecorderAfterCallConformance(t *testing.T) { runStubRecorderConformance(t, "AFTER_CALL") }
func TestStubRecorderIdentityConformance(t *testing.T) {
	runStubRecorderConformance(t, "METHOD_IDENTITY")
}
func TestStubRecorderMessagesConformance(t *testing.T) { runStubRecorderConformance(t, "MESSAGES") }
func TestStubRecorderAssertConformance(t *testing.T)   { runStubRecorderConformance(t, "ASSERT") }
func TestStubRecorderBoundaryConformance(t *testing.T) { runStubRecorderConformance(t, "BOUNDARY") }
