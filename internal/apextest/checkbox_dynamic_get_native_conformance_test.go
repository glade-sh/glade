package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

type checkboxReadNativeCapture struct {
	Context, Kind, Route, NamespaceContext, SourceAPI, CallerAPI, TransportAPI, TestClass, Marker string
	Deployment                                                                                    struct {
		ID, Status                                                                      string
		Done, CheckOnly                                                                 bool
		NumberComponentErrors, NumberTestErrors, NumberTestsCompleted, NumberTestsTotal int
	}
	Execution struct {
		Status                           int
		Success, Compiled                bool
		CompileProblem, ExceptionMessage string
	}
	Sources   map[string]struct{ Source, SHA256 string }
	Source    struct{ Source, SHA256 string }
	RawSHA256 map[string]string
	Rows      []struct {
		ID            string
		Native        map[string]any
		NativeFailure struct{ Message string }
	}
}

func TestSObjectDynamicCheckboxNativeConformance(t *testing.T) {
	var data struct {
		APIs      []string `json:"apiVersions"`
		Routes    []string
		Namespace string   `json:"namespaceContext"`
		Known     int      `json:"knownRows"`
		Pending   []string `json:"pendingCoverage"`
		Captures  []checkboxReadNativeCapture
	}
	raw, err := os.ReadFile("testdata/conformance/checkbox_dynamic_get_native.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(data.APIs, []string{"62.0", "67.0"}) ||
		!reflect.DeepEqual(data.Routes, []string{"isTest", "anonymous"}) ||
		data.Namespace != "none" || data.Known != 36 || len(data.Captures) != 4 || len(data.Pending) != 4 {
		t.Fatal("native checkbox read scope differs")
	}
	seen := map[string]bool{}
	for _, capture := range data.Captures {
		t.Run(capture.Context, func(t *testing.T) {
			if capture.SourceAPI != "62.0" && capture.SourceAPI != "67.0" ||
				capture.CallerAPI != capture.SourceAPI || capture.NamespaceContext != "none" || len(capture.Rows) != 9 || seen[capture.Context] {
				t.Fatal("native checkbox capture scope differs")
			}
			seen[capture.Context] = true
			actual := replayCheckboxReadNative(t, capture)
			if len(actual) != len(capture.Rows) {
				t.Fatal("native checkbox replay row count differs")
			}
			for _, row := range capture.Rows {
				if !reflect.DeepEqual(actual[row.ID], row.Native) {
					t.Errorf("cell %s: got %#v; native %#v", row.ID, actual[row.ID], row.Native)
				}
			}
		})
	}
}

func replayCheckboxReadNative(t *testing.T, capture checkboxReadNativeCapture) map[string]map[string]any {
	t.Helper()
	decode := func(message string) map[string]any {
		t.Helper()
		_, payload, ok := strings.Cut(message, capture.Marker)
		if !ok {
			t.Fatalf("missing checkbox observation marker: %s", message)
		}
		var result map[string]any
		if err := json.Unmarshal([]byte(payload), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	rawNames := []string{"org.tsv", "errors.json", "ORG-RUN.txt"}
	if capture.Kind == "anonymous" {
		rawNames = append(rawNames, "run.json", "source.apex")
	} else {
		rawNames = append(rawNames, "deploy.json")
	}
	for _, name := range rawNames {
		sum, err := hex.DecodeString(capture.RawSHA256[name])
		if err != nil || len(sum) != sha256.Size {
			t.Fatalf("native raw digest missing: %s", name)
		}
	}
	actual := map[string]map[string]any{}
	if capture.Kind == "anonymous" {
		if capture.Route != "anonymous" || capture.TransportAPI != capture.SourceAPI || capture.Marker != "CHECKBOX_READ_OBSERVATION|" {
			t.Fatal("anonymous checkbox route differs")
		}
		e := capture.Execution
		if e.Status != 0 || !e.Success || !e.Compiled || e.CompileProblem != "" || e.ExceptionMessage != "" {
			t.Fatal("native anonymous execution incomplete")
		}
		sum := sha256.Sum256([]byte(capture.Source.Source))
		if capture.Source.Source == "" || hex.EncodeToString(sum[:]) != capture.Source.SHA256 {
			t.Fatal("anonymous source digest differs")
		}
		index, source := checkboxReadAnonymousDeclarations(t, capture.Source.Source, capture.SourceAPI)
		analysis := sema.AnalyzeAnonymous(index, source, capture.SourceAPI)
		if analysis.HasErrors() {
			t.Fatalf("native compiled; Glade diagnostics=%v", analysis.Diagnostics)
		}
		program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: capture.SourceAPI})
		if err != nil {
			t.Fatal(err)
		}
		runtime, err := CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
		if err != nil {
			t.Fatal(err)
		}
		org := checkboxReadNativeOrg(orgFromIndex(index))
		org.APIVersion = capture.SourceAPI
		runner := newConformanceRunner(t, index, conformanceRunnerOptions{RequestRuntime: &runtime, Org: &org})
		result, err := runner.execute(program)
		if err != nil {
			t.Fatal(err)
		}
		for _, message := range result.Debug {
			if strings.HasPrefix(message, capture.Marker) {
				actual[strconv.Itoa(len(actual))] = decode(message)
			}
		}
		return actual
	}
	if capture.Kind != "named" || capture.Route != "isTest" || capture.TransportAPI != "62.0" ||
		capture.TestClass != "CheckboxReadObservation" || capture.Marker != "CHECKBOX_READ_OBSERVATION " {
		t.Fatal("named checkbox route differs")
	}
	d := capture.Deployment
	if d.ID == "" || !d.Done || !d.CheckOnly || d.Status != "Failed" || d.NumberComponentErrors != 0 ||
		d.NumberTestsCompleted != 0 || d.NumberTestErrors != 9 || d.NumberTestsTotal != 9 {
		t.Fatal("native named deployment incomplete")
	}
	root := t.TempDir()
	for name, source := range capture.Sources {
		sum := sha256.Sum256([]byte(source.Source))
		if filepath.IsAbs(name) || strings.Contains(name, "..") || source.Source == "" || hex.EncodeToString(sum[:]) != source.SHA256 {
			t.Fatal("named source digest differs")
		}
		writeFile(t, filepath.Join(root, name), source.Source)
	}
	var project struct {
		API string `json:"sourceApiVersion"`
	}
	if err := json.Unmarshal([]byte(capture.Sources["sfdx-project.json"].Source), &project); err != nil || project.API != capture.SourceAPI {
		t.Fatal("named project API differs")
	}
	for _, name := range []string{capture.TestClass, "OwnedCheckboxRead"} {
		var metadata struct {
			API string `xml:"apiVersion"`
		}
		if err := xml.Unmarshal([]byte(capture.Sources["force-app/main/default/classes/"+name+".cls-meta.xml"].Source), &metadata); err != nil || metadata.API != capture.SourceAPI {
			t.Fatal("named class API differs")
		}
	}
	index := loadTestIndex(t, root)
	analysis := sema.Analyze(index)
	if index.HasErrors() || analysis.HasErrors() {
		t.Fatalf("native compiled; Glade diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
	}
	org := checkboxReadNativeOrg(orgFromIndex(index))
	org.APIVersion = capture.SourceAPI
	runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
	cases := Discover(index, Options{SelectedClasses: []string{capture.TestClass}})
	if len(cases) != len(capture.Rows) {
		t.Fatal("named observation discovery differs")
	}
	methods, methodErrors := compileTestMethodsForIndex(&index, cases)
	programs, programErrors := compileTestInvokePrograms(cases)
	for _, row := range capture.Rows {
		if !reflect.DeepEqual(decode(row.NativeFailure.Message), row.Native) {
			t.Fatal("native assertion payload differs")
		}
		key := ""
		for _, c := range cases {
			if c.MethodName == row.ID {
				key = testCaseKey(c)
			}
		}
		if key == "" || methodErrors[key] != nil || programErrors[key] != nil {
			t.Fatal("named observation compilation differs")
		}
		machine := runner.newMachine()
		if err := machine.RegisterMethod(methods[key]); err != nil {
			t.Fatal(err)
		}
		machine.EnableTestContext()
		_, execErr := machine.ExecuteInClass(programs[key], capture.TestClass)
		var runtimeErr *vm.RuntimeError
		if !errors.As(execErr, &runtimeErr) || genericInterfaceCastExceptionType(runtimeErr.Type) != "System.AssertException" {
			t.Fatalf("observation assertion missing: %v", execErr)
		}
		actual[row.ID] = decode(runtimeErr.Message)
	}
	return actual
}

// Explicit schema typing makes the replay exercise the Boolean fallback rather
// than accidentally passing through an untyped synthetic field.
func checkboxReadNativeOrg(org storage.OrgState) storage.OrgState {
	contact := org.Objects["Contact"]
	contact.Definition = contact.Definition.Clone()
	if contact.Definition.Fields == nil {
		contact.Definition.Fields = map[string]storage.Field{}
	}
	contact.Definition.Fields["DoNotCall"] = storage.Field{APIName: "DoNotCall", Type: storage.FieldBoolean, DisplayType: "BOOLEAN", DefaultValue: "false"}
	org.Objects["Contact"] = contact
	return org
}

// Match exec's transient declaration path. The captured source stays intact in
// the oracle; only its executable-body copy masks declarations with whitespace,
// retaining source offsets and line breaks for the anonymous invocation.
func checkboxReadAnonymousDeclarations(t *testing.T, source, api string) (typesys.Index, string) {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"","sourceApiVersion":"`+api+`"}`)
	masked := []byte(source)
	declarations := 0
	for _, decl := range apexast.NewParser().ParseSource("CheckboxReadAnonymous.cls", source).Declarations {
		if decl.Kind != apexast.DeclarationClass {
			continue
		}
		start, end := decl.Range.Start.Offset, decl.Range.End.Offset
		if start < 0 || end <= start || end > len(source) {
			t.Fatal("invalid anonymous checkbox declaration range")
		}
		path := filepath.Join(root, "force-app", "main", "default", "classes", decl.Name+".cls")
		writeFile(t, path, source[start:end])
		writeFile(t, path+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
		declarations++
		for i := start; i < end; i++ {
			if masked[i] != '\n' && masked[i] != '\r' {
				masked[i] = ' '
			}
		}
	}
	if declarations != 2 {
		t.Fatalf("expected two anonymous checkbox declarations, got %d", declarations)
	}
	index := sema.WithAnonymousDeclarationContext(loadTestIndex(t, root))
	if index.HasErrors() {
		t.Fatalf("native compiled; anonymous declaration parser=%v", index.Diagnostics)
	}
	if analysis := sema.AnalyzeAnonymousDeclarations(index); analysis.HasErrors() {
		t.Fatalf("native compiled; anonymous declaration semantics=%v", analysis.Diagnostics)
	}
	return index, string(masked)
}
