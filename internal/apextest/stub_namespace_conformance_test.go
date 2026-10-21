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

type stubNamespaceSources struct {
	Classes      map[string]string `json:"classes"`
	SourceSHA256 map[string]string `json:"sourceSha256"`
}

type stubNamespaceCell struct {
	stubNamespaceSources
	TestClass     string          `json:"testClass"`
	Outcome       string          `json:"outcome"`
	Value         json.RawMessage `json:"value"`
	Primary       string          `json:"primary"`
	ExceptionType string          `json:"exceptionType"`
	Message       string          `json:"message"`
	Carry         string          `json:"carry"`
	Native        json.RawMessage `json:"native"`
	NativeReceipt json.RawMessage `json:"nativeReceipt"`
}

type stubNamespaceRow struct {
	ID         string                       `json:"id"`
	Group      string                       `json:"group"`
	CellsByAPI map[string]stubNamespaceCell `json:"cellsByAPI"`
}

// The capture packet admits only named @IsTest on its namespaced org. Preserve
// each API's exact declarations and observation assertion, including inner-class
// rejection twins. API 59 is a same-release control; its row-deadline no-answer
// cell is retained without a local semantic assertion.
func runStubNamespaceConformance(t *testing.T, group string, shapes int) {
	t.Helper()
	var data struct {
		Route                  string                          `json:"route"`
		Namespace              string                          `json:"namespace"`
		APIVersions            []string                        `json:"apiVersions"`
		ShapeCount             int                             `json:"shapeCount"`
		CellCount              int                             `json:"cellCount"`
		NativeObservationCount int                             `json:"nativeObservationCount"`
		FixturesByAPI          map[string]stubNamespaceSources `json:"fixturesByAPI"`
		ReceiptsByAPI          map[string]map[string]string    `json:"receiptsByAPI"`
		UncapturedCells        []struct {
			ID, APIVersion, Route, Owner, Reason string
		} `json:"uncapturedCells"`
		Rows []stubNamespaceRow `json:"rows"`
	}
	raw, err := os.ReadFile("testdata/conformance/stub_namespace.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data.Route != "isTest" || data.Namespace != "gladens001" || data.ShapeCount != 98 || len(data.Rows) != 98 {
		t.Fatal("invalid named-only native matrix")
	}
	versions := map[string]bool{}
	for _, api := range data.APIVersions {
		if versions[api] || (api != "59.0" && api != "62.0" && api != "67.0" && api != "68.0") {
			t.Fatalf("invalid captured API: %s", api)
		}
		versions[api] = true
	}
	for _, api := range []string{"59.0", "62.0", "67.0", "68.0"} {
		if !versions[api] {
			t.Fatalf("missing captured API: %s", api)
		}
		keys := []string{"orgTsvSha256", "orgRunSha256", "nativeJsonSha256", "fixtureZipSha256", "captureManifestSha256"}
		if api == "59.0" {
			keys = append(keys, "completionSha256", "captureCompletionSha256", "sourceHashesSha256")
		}
		for _, key := range keys {
			value := data.ReceiptsByAPI[api][key]
			if decoded, err := hex.DecodeString(value); err != nil || len(decoded) != sha256.Size {
				t.Fatalf("missing native receipt hash: %s/%s", api, key)
			}
		}
		if data.ReceiptsByAPI[api]["captureManifestSha256"] != data.ReceiptsByAPI["62.0"]["captureManifestSha256"] {
			t.Fatalf("capture manifest differs at API %s", api)
		}
	}
	uncaptured := map[string]string{}
	for _, cell := range data.UncapturedCells {
		if cell.ID != "E9_091" || cell.APIVersion != "59.0" || cell.Route != "isTest" || cell.Owner != "stub namespace emulation removal" || cell.Reason != "row-deadline no answer at 59.0" {
			t.Fatal("invalid uncaptured native cell")
		}
		uncaptured[cell.APIVersion+"/"+cell.ID] = cell.Reason
	}
	if len(data.UncapturedCells) != 1 || len(uncaptured) != 1 {
		t.Fatal("missing native row-deadline carry")
	}
	seen := map[string]bool{}
	var rows []stubNamespaceRow
	cells, observations, carried := 0, 0, 0
	for _, row := range data.Rows {
		if row.ID == "" || seen[row.ID] || len(row.CellsByAPI) != len(versions) {
			t.Fatalf("invalid native row: %s", row.ID)
		}
		seen[row.ID] = true
		for _, api := range data.APIVersions {
			cell, ok := row.CellsByAPI[api]
			if !ok || len(cell.Native) == 0 || len(cell.NativeReceipt) == 0 || cell.TestClass == "" || len(cell.Classes) != 1 {
				t.Fatalf("missing native cell: %s/%s", row.ID, api)
			}
			stubNamespaceCheckSources(t, cell.stubNamespaceSources)
			if cell.Classes[cell.TestClass] == "" {
				t.Fatal("missing captured observation class")
			}
			reason, noAnswer := uncaptured[api+"/"+row.ID]
			if cell.Carry != "" {
				if !noAnswer || cell.Carry != reason || cell.Outcome != "NO_ANSWER" || len(cell.Value) != 0 || cell.Primary != "" || cell.ExceptionType != "" || cell.Message != "" {
					t.Fatalf("invalid unasserted carry: %s/%s", row.ID, api)
				}
				carried++
			} else {
				if noAnswer {
					t.Fatalf("uncaptured cell cannot have a semantic expectation: %s/%s", row.ID, api)
				}
				switch cell.Outcome {
				case "runtime":
					if !json.Valid(cell.Value) {
						t.Fatal("invalid native runtime payload")
					}
				case "compile-rejected":
					if cell.Primary == "" {
						t.Fatal("empty compile artifact must be carried")
					}
				case "uncaught_test_failure":
					if cell.ExceptionType == "" || cell.Message == "" {
						t.Fatal("uncaught artifact must be carried")
					}
				default:
					t.Fatalf("native artifact requires carry: %s/%s/%s", row.ID, api, cell.Outcome)
				}
				observations++
			}
			cells++
		}
		matches := row.Group == group || group == "CONTROLS" && strings.HasPrefix(row.Group, "CONTROL")
		if matches {
			rows = append(rows, row)
		}
	}
	if len(rows) != shapes || cells != data.CellCount || cells != 392 || observations != data.NativeObservationCount || observations != 391 || carried != 1 {
		t.Fatalf("matrix: shapes=%d cells=%d native=%d carried=%d", len(rows), cells, observations, carried)
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCounts()
			defer counts.Log(t, "STUB-NS", api, group)
			fixture, ok := data.FixturesByAPI[api]
			if !ok || len(fixture.Classes) != 7 {
				t.Fatal("missing native fixture declarations")
			}
			stubNamespaceCheckSources(t, fixture)
			index := stubNamespaceIndex(t, api, data.Namespace, fixture.Classes)
			stubNamespaceCompile(t, stubNamespaceCell{}, append(index.Diagnostics, sema.Analyze(index).Diagnostics...))
			org := orgFromIndex(index)
			org.APIVersion = api
			base := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			for _, row := range rows {
				cell := row.CellsByAPI[api]
				if cell.Carry != "" {
					t.Logf("carried %s/%s/isTest (%s): %s", row.ID, api, cell.Outcome, cell.Carry)
					continue
				}
				kind := "exact"
				if row.ID == "E9_CONTROL_COMPILE" {
					kind = "category"
				}
				counts.RunKind(t, "isTest", row.ID, row.ID, kind, func(t *testing.T) {
					classes := make(map[string]string, len(fixture.Classes)+1)
					for name, source := range fixture.Classes {
						classes[name] = source
					}
					for name, source := range cell.Classes {
						classes[name] = source
					}
					index := stubNamespaceIndex(t, api, data.Namespace, classes)
					analysis := sema.Analyze(index)
					diagnostics := append(index.Diagnostics, analysis.Diagnostics...)
					if row.ID == "E9_CONTROL_COMPILE" {
						stubNamespaceCompileControl(t, cell, diagnostics)
						return
					}
					stubNamespaceCompile(t, cell, diagnostics)
					if cell.Outcome == "compile-rejected" {
						return
					}
					cases := Discover(index, Options{SelectedClasses: []string{cell.TestClass}, SelectedMethod: "observe"})
					if len(cases) != 1 {
						t.Fatalf("named observation discovery: %d", len(cases))
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
					stubNamespaceValue(t, row.ID, cell, execErr)
				})
			}
		})
	}
}

func stubNamespaceCheckSources(t *testing.T, source stubNamespaceSources) {
	t.Helper()
	if len(source.Classes) == 0 || len(source.Classes) != len(source.SourceSHA256) {
		t.Fatal("captured source hash matrix incomplete")
	}
	for name, text := range source.Classes {
		sum := sha256.Sum256([]byte(text))
		if hex.EncodeToString(sum[:]) != source.SourceSHA256[name] {
			t.Fatalf("captured source changed: %s", name)
		}
	}
}

func stubNamespaceIndex(t *testing.T, api, namespace string, classes map[string]string) typesys.Index {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"namespace":%q,"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, namespace, api))
	for name, source := range classes {
		path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
		writeFile(t, path, source)
		writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
	}
	return loadTestIndex(t, root)
}

// The unrelated compile control rejects an invalid initializer in the same
// class on both runtimes. Compare rejection kind and compilation unit; keep
// each compiler's wording in its evidence and count this as category-only.
func stubNamespaceCompileControl(t *testing.T, want stubNamespaceCell, diagnostics []diagnostic.Diagnostic) {
	t.Helper()
	var native struct {
		Outcome     string
		Diagnostics struct {
			Compile []struct{ Name, Type string }
		}
	}
	if err := json.Unmarshal(want.Native, &native); err != nil {
		t.Fatal(err)
	}
	if want.Outcome != "compile-rejected" || native.Outcome != "compile-rejected" || len(native.Diagnostics.Compile) != 1 || native.Diagnostics.Compile[0].Name != want.TestClass || native.Diagnostics.Compile[0].Type != "ApexClass" {
		t.Fatal("native control must reject its captured Apex class")
	}
	rejected := false
	for _, d := range diagnostics {
		if d.Severity != diagnostic.Error {
			continue
		}
		if d.Code != "GLADESEMA018" || filepath.Base(d.File) != want.TestClass+".cls" {
			t.Fatalf("Glade control rejection has a different kind or compilation unit: %+v", d)
		}
		rejected = true
	}
	if !rejected {
		t.Fatal("native rejects the control class initializer; Glade accepted")
	}
}

func stubNamespaceCompile(t *testing.T, want stubNamespaceCell, diagnostics []diagnostic.Diagnostic) {
	t.Helper()
	var messages []string
	for _, d := range diagnostics {
		if d.Severity != diagnostic.Error {
			continue
		}
		message := d.NativeMessage
		if message == "" {
			message = d.Message
		}
		messages = append(messages, message)
	}
	if want.Outcome == "compile-rejected" {
		if len(messages) == 0 || messages[0] != want.Primary {
			t.Fatalf("native rejection <%s>; Glade diagnostics=%v", want.Primary, messages)
		}
	} else if len(messages) != 0 {
		t.Fatalf("native compilation accepted; Glade diagnostics=%v", messages)
	}
}

// Qualify only the exception type, leaving every message byte intact.
func stubNamespaceExceptionType(name string) string {
	switch name {
	case "TypeException", "NullPointerException", "ListException", "AssertException":
		return "System." + name
	default:
		return name
	}
}

func stubNamespaceValue(t *testing.T, id string, want stubNamespaceCell, execErr error) {
	t.Helper()
	var runtimeErr *vm.RuntimeError
	if execErr == nil || !errors.As(execErr, &runtimeErr) {
		t.Fatalf("native %s; missing Glade exception: %v", want.Outcome, execErr)
	}
	if want.Outcome == "uncaught_test_failure" {
		if stubNamespaceExceptionType(runtimeErr.Type) != want.ExceptionType || runtimeErr.ExceptionMessage() != want.Message {
			t.Fatalf("native <%s: %s>; Glade <%s: %s>", want.ExceptionType, want.Message, runtimeErr.Type, runtimeErr.ExceptionMessage())
		}
		return
	}
	if stubNamespaceExceptionType(runtimeErr.Type) != "System.AssertException" {
		t.Fatalf("native marker; Glade exception: %v", execErr)
	}
	message := runtimeErr.ExceptionMessage()
	marker := "C126_RESULT|" + id + "|"
	_, payload, found := strings.Cut(message, marker)
	if !found || strings.Count(message, "C126_RESULT|") != 1 {
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
		actual["exceptionType"] = stubNamespaceExceptionType(name)
	}
	switch id {
	case "E9_049", "E9_057", "E9_065", "E9_089":
		// These integers vary across all three native runs. Preserve the carried
		// integers in the fixture, compare only their captured equality relations.
		fields := []string{"hashA", "hashB", "hashC"}
		for i, a := range fields {
			for _, values := range []map[string]any{actual, expected} {
				n, ok := values[a].(float64)
				if !ok || n != float64(int32(n)) {
					t.Fatalf("invalid Integer hash %s=%v", a, values[a])
				}
			}
			for _, b := range fields[i+1:] {
				if (actual[a] == actual[b]) != (expected[a] == expected[b]) {
					t.Fatalf("native hash relation %s=%s differs", a, b)
				}
			}
		}
		for _, field := range fields {
			delete(actual, field)
			delete(expected, field)
		}
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("native payload %s; Glade payload %s", want.Value, payload)
	}
}

func TestStubNamespaceControlsConformance(t *testing.T) {
	runStubNamespaceConformance(t, "CONTROLS", 4)
}
func TestStubNamespaceDisplayConformance(t *testing.T) { runStubNamespaceConformance(t, "DISPLAY", 16) }
func TestStubNamespaceTypeLookupConformance(t *testing.T) {
	runStubNamespaceConformance(t, "TYPE_LOOKUP", 17)
}
func TestStubNamespaceQualifiedConformance(t *testing.T) {
	runStubNamespaceConformance(t, "QUALIFIED", 3)
}
func TestStubNamespacePlainConformance(t *testing.T) { runStubNamespaceConformance(t, "PLAIN", 3) }
func TestStubNamespaceBoundaryConformance(t *testing.T) {
	runStubNamespaceConformance(t, "BOUNDARY", 3)
}
func TestStubNamespaceDefaultContextConformance(t *testing.T) {
	runStubNamespaceConformance(t, "DEFAULT_CONTEXT", 2)
}
func TestStubNamespaceHelperConformance(t *testing.T) { runStubNamespaceConformance(t, "HELPER", 2) }
func TestStubNamespaceEqualityDirectConformance(t *testing.T) {
	runStubNamespaceConformance(t, "EQ_DIRECT", 6)
}
func TestStubNamespaceEqualityObjectConformance(t *testing.T) {
	runStubNamespaceConformance(t, "EQ_OBJECT", 6)
}
func TestStubNamespaceEqualityMapConformance(t *testing.T) {
	runStubNamespaceConformance(t, "EQ_MAP", 6)
}
func TestStubNamespaceEqualitySetConformance(t *testing.T) {
	runStubNamespaceConformance(t, "EQ_SET", 6)
}
func TestStubNamespaceEqualityListConformance(t *testing.T) {
	runStubNamespaceConformance(t, "EQ_LIST", 6)
}
func TestStubNamespaceEqualityMatcherConformance(t *testing.T) {
	runStubNamespaceConformance(t, "EQ_MATCHER", 6)
}
func TestStubNamespaceEqualityTypeNameConformance(t *testing.T) {
	runStubNamespaceConformance(t, "EQ_TYPENAME", 6)
}
func TestStubNamespaceEqualityKeyConformance(t *testing.T) {
	runStubNamespaceConformance(t, "EQ_KEY", 6)
}
