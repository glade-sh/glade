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
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

type nestedConstructorNativeCapture struct {
	Context, Kind, SourceAPI, CallerAPI, ProjectAPI, TransportAPI, Route, NamespaceContext, TestClass, Marker string
	SourceAPIs                                                                                                map[string]string
	Deployment                                                                                                struct {
		ID, Status                                                                      string
		Done, CheckOnly                                                                 bool
		NumberComponentErrors, NumberTestErrors, NumberTestsCompleted, NumberTestsTotal int
	}
	Execution struct {
		Status                           int
		Success, Compiled                bool
		CompileProblem, ExceptionMessage string
	}
	Sources                  map[string]struct{ Source, SHA256 string }
	Source                   struct{ Source, SHA256 string }
	SourceDigests, RawSHA256 map[string]string
	ExceptionTraceSHA256     string
	Rows                     []struct {
		ID            string
		Native        map[string]any
		NativeFailure struct{ Message string }
	}
}

func TestNestedConstructorIdentityNativeConformance(t *testing.T) {
	var data struct {
		APIs             []string `json:"apiVersions"`
		Routes           []string
		NamespaceContext string
		Known            int      `json:"knownRows"`
		Pending          []string `json:"pendingCoverage"`
		Captures         []nestedConstructorNativeCapture
	}
	raw, err := os.ReadFile("testdata/conformance/nested_constructor_identity_native.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(data.APIs, []string{"52.0", "62.0", "67.0"}) || !reflect.DeepEqual(data.Routes, []string{"isTest", "anonymous"}) || data.NamespaceContext != "none" || data.Known != 28 || len(data.Captures) != 9 || len(data.Pending) == 0 {
		t.Fatal("native nested-constructor scope differs")
	}
	expected := map[string]string{"api52": "52.0", "api62": "62.0", "api67": "67.0", "anonymous-api52": "52.0", "anonymous-api62": "62.0", "anonymous-api67": "67.0", "cycle-api52": "52.0", "cycle-anonymous-api52/direct": "52.0", "cycle-anonymous-api52/mutual": "52.0"}
	count := 0
	for _, capture := range data.Captures {
		t.Run(capture.Context, func(t *testing.T) {
			if expected[capture.Context] != capture.SourceAPI || capture.CallerAPI != capture.SourceAPI || capture.NamespaceContext != data.NamespaceContext {
				t.Fatal("captured source/caller scope differs")
			}
			delete(expected, capture.Context)
			actual := replayNestedConstructorNative(t, capture)
			if len(actual) != len(capture.Rows) {
				t.Fatal("replay row count differs")
			}
			seen := map[string]bool{}
			for _, row := range capture.Rows {
				if seen[row.ID] {
					t.Fatal("duplicate native row")
				}
				seen[row.ID] = true
				count++
				if !reflect.DeepEqual(actual[row.ID], row.Native) {
					t.Errorf("row %s: native=%v; Glade=%v", row.ID, row.Native, actual[row.ID])
				}
			}
		})
	}
	if count != data.Known || len(expected) != 0 {
		t.Fatal("native nested-constructor coverage differs")
	}
}

func replayNestedConstructorNative(t *testing.T, capture nestedConstructorNativeCapture) map[string]map[string]any {
	t.Helper()
	anonymous := capture.Route == "anonymous"
	cycle := capture.Kind == "cycle"
	if capture.Kind != "positive" && !cycle {
		t.Fatal("unknown observation kind")
	}
	wantRows := 4
	if cycle {
		wantRows = 2
		if anonymous {
			wantRows = 1
		}
	}
	if len(capture.Rows) != wantRows {
		t.Fatal("native observation row count differs")
	}
	if cycle && capture.SourceAPI != "52.0" {
		t.Fatal("unobserved cycle source API")
	}
	rawNames := []string{"org.tsv", "errors.json", "ORG-RUN.txt", "source-digests.json"}
	if anonymous {
		if capture.TransportAPI != capture.SourceAPI || capture.TestClass != "" {
			t.Fatal("anonymous route differs")
		}
		rawNames = append(rawNames, "run.json", "source.apex")
		e := capture.Execution
		if !e.Compiled || e.CompileProblem != "" {
			t.Fatal("native anonymous compilation incomplete")
		}
		if cycle {
			traceSum, traceErr := hex.DecodeString(capture.ExceptionTraceSHA256)
			if traceErr != nil || len(traceSum) != sha256.Size {
				t.Fatal("uncaught native trace digest missing")
			}
			if e.Status != 1 || e.Success || e.ExceptionMessage == "" || capture.Marker != "" || capture.ProjectAPI != "" || len(capture.Sources) != 0 || len(capture.SourceAPIs) != 0 {
				t.Fatal("native uncaught cycle envelope differs")
			}
		} else {
			if e.Status != 0 || !e.Success || e.ExceptionMessage != "" || capture.Marker != "NESTED_CONSTRUCTOR_OBSERVATION|" {
				t.Fatal("native positive anonymous execution incomplete")
			}
			rawNames = append(rawNames, "setup-result.json", "deploy-retry.json")
			d := capture.Deployment
			if d.ID == "" || !d.Done || d.CheckOnly || d.Status != "Succeeded" || d.NumberComponentErrors != 0 || d.NumberTestErrors != 0 || d.NumberTestsCompleted != 0 || d.NumberTestsTotal != 0 {
				t.Fatal("native helper deployment incomplete")
			}
		}
	} else {
		if capture.Route != "isTest" || capture.TransportAPI != "62.0" {
			t.Fatal("named route differs")
		}
		rawNames = append(rawNames, "deploy.json")
		d := capture.Deployment
		if d.ID == "" || !d.Done || !d.CheckOnly || d.Status != "Failed" || d.NumberComponentErrors != 0 || d.NumberTestsCompleted != 0 || d.NumberTestsTotal != wantRows || d.NumberTestErrors != wantRows {
			t.Fatal("native named observation deployment incomplete")
		}
		if cycle {
			if capture.TestClass != "NestedCycleObservation" || capture.Marker != "" {
				t.Fatal("named uncaught cycle scope differs")
			}
		} else if capture.TestClass != "NestedConstructorObservation" || capture.Marker != "NESTED_CONSTRUCTOR_OBSERVATION " {
			t.Fatal("named positive scope differs")
		}
	}
	for _, name := range rawNames {
		sum, err := hex.DecodeString(capture.RawSHA256[name])
		if err != nil || len(sum) != sha256.Size {
			t.Fatalf("missing native raw digest %s", name)
		}
	}
	root := t.TempDir()
	for name, source := range capture.Sources {
		sum := sha256.Sum256([]byte(source.Source))
		if filepath.IsAbs(name) || strings.Contains(name, "..") || source.Source == "" || hex.EncodeToString(sum[:]) != source.SHA256 || capture.SourceDigests[name] != source.SHA256 {
			t.Fatal("captured source/D7 digest differs")
		}
		writeFile(t, filepath.Join(root, name), source.Source)
	}
	var index typesys.Index
	var body string
	if anonymous {
		sum := sha256.Sum256([]byte(capture.Source.Source))
		if capture.Source.Source == "" || hex.EncodeToString(sum[:]) != capture.Source.SHA256 || capture.SourceDigests["observe.apex"] != capture.Source.SHA256 || capture.RawSHA256["source.apex"] != capture.Source.SHA256 {
			t.Fatal("anonymous body source/D7 digest differs")
		}
		body = capture.Source.Source
	}
	if anonymous && cycle {
		index, body = nestedConstructorAnonymousCycle(t, body, capture.SourceAPI)
	} else {
		prefix := ""
		projectRoot := root
		if anonymous {
			prefix = "project/"
			projectRoot = filepath.Join(root, "project")
		}
		var project struct {
			API string `json:"sourceApiVersion"`
		}
		if capture.ProjectAPI != "62.0" || json.Unmarshal([]byte(capture.Sources[prefix+"sfdx-project.json"].Source), &project) != nil || project.API != capture.ProjectAPI {
			t.Fatal("project API pin differs")
		}
		classes := []string{"OwnedBranch", "OwnedNestBase", "OwnedNestLeaf"}
		if cycle {
			classes = []string{"OwnedDirectCycle", "OwnedMutualCycle"}
		}
		if !anonymous {
			classes = append(classes, capture.TestClass)
		}
		if len(capture.SourceAPIs) != len(classes) || len(capture.Sources) != 2*len(classes)+1 {
			t.Fatal("captured helper set differs")
		}
		for _, name := range classes {
			var meta struct {
				API string `xml:"apiVersion"`
			}
			if capture.SourceAPIs[name] != capture.SourceAPI || xml.Unmarshal([]byte(capture.Sources[prefix+"force-app/main/default/classes/"+name+".cls-meta.xml"].Source), &meta) != nil || meta.API != capture.SourceAPI {
				t.Fatal("helper metadata pin differs")
			}
		}
		index = loadTestIndex(t, projectRoot)
		if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
			t.Fatalf("native compiled; Glade diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
		}
	}
	decode := func(message string) map[string]any {
		t.Helper()
		_, payload, ok := strings.Cut(message, capture.Marker)
		if !ok {
			t.Fatal("observation marker missing")
		}
		var row map[string]any
		if err := json.Unmarshal([]byte(payload), &row); err != nil {
			t.Fatal(err)
		}
		return row
	}
	actual := map[string]map[string]any{}
	org := orgFromIndex(index)
	org.APIVersion = capture.ProjectAPI
	if anonymous {
		if cycle {
			org.APIVersion = capture.SourceAPI
		}
		if analysis := sema.AnalyzeAnonymous(index, body, capture.CallerAPI); analysis.HasErrors() {
			t.Fatalf("native compiled; anonymous diagnostics=%v", analysis.Diagnostics)
		}
		program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: capture.CallerAPI})
		if err != nil {
			t.Fatal(err)
		}
		runtime, err := CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner := newConformanceRunner(t, index, conformanceRunnerOptions{RequestRuntime: &runtime, Org: &org})
		result, execErr := runner.execute(program)
		if cycle {
			var runtimeErr *vm.RuntimeError
			if !errors.As(execErr, &runtimeErr) || genericInterfaceCastExceptionType(runtimeErr.Type) != "System.LimitException" || execErr.Error() != capture.Execution.ExceptionMessage {
				t.Fatalf("uncaught cycle differs: native=%s; Glade=%v", capture.Execution.ExceptionMessage, execErr)
			}
			actual[capture.Rows[0].ID] = map[string]any{"compiled": true, "success": false, "compileProblem": "", "exceptionMessage": execErr.Error()}
		} else {
			if execErr != nil {
				t.Fatal(execErr)
			}
			for _, message := range result.Debug {
				if strings.HasPrefix(message, capture.Marker) {
					actual[strconv.Itoa(len(actual))] = decode(message)
				}
			}
		}
		return actual
	}
	runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
	cases := Discover(index, Options{SelectedClasses: []string{capture.TestClass}})
	if len(cases) != wantRows {
		t.Fatal("named observation discovery differs")
	}
	methods, methodErrors := compileTestMethodsForIndex(&index, cases)
	programs, programErrors := compileTestInvokePrograms(cases)
	nativeRows := map[string]struct {
		Message string
		Native  map[string]any
	}{}
	for _, row := range capture.Rows {
		nativeRows[row.ID] = struct {
			Message string
			Native  map[string]any
		}{row.NativeFailure.Message, row.Native}
	}
	for _, c := range cases {
		key := testCaseKey(c)
		if methodErrors[key] != nil || programErrors[key] != nil {
			t.Fatal("named observation compilation differs")
		}
		row, ok := nativeRows[c.MethodName]
		if !ok {
			t.Fatal("named native observation missing")
		}
		machine := runner.newMachine()
		if err := machine.RegisterMethod(methods[key]); err != nil {
			t.Fatal(err)
		}
		machine.EnableTestContext()
		_, execErr := machine.ExecuteInClass(programs[key], capture.TestClass)
		var runtimeErr *vm.RuntimeError
		if !errors.As(execErr, &runtimeErr) {
			t.Fatalf("native observer failure absent: %v", execErr)
		}
		if cycle {
			if genericInterfaceCastExceptionType(runtimeErr.Type) != "System.LimitException" || execErr.Error() != row.Message {
				t.Fatalf("uncaught cycle differs: native=%s; Glade=%v", row.Message, execErr)
			}
			actual[c.MethodName] = map[string]any{"uncaught": execErr.Error()}
		} else {
			if genericInterfaceCastExceptionType(runtimeErr.Type) != "System.AssertException" || !reflect.DeepEqual(decode(row.Message), row.Native) {
				t.Fatal("native intentional assertion envelope differs")
			}
			actual[c.MethodName] = decode(runtimeErr.Message)
		}
	}
	return actual
}

func nestedConstructorAnonymousCycle(t *testing.T, source, api string) (typesys.Index, string) {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"","sourceApiVersion":"`+api+`"}`)
	masked := []byte(source)
	count := 0
	for _, decl := range apexast.NewParser().ParseSource("NestedCycleAnonymous.cls", source).Declarations {
		if decl.Kind != apexast.DeclarationClass {
			continue
		}
		start, end := decl.Range.Start.Offset, decl.Range.End.Offset
		if start < 0 || end <= start || end > len(source) {
			t.Fatal("invalid anonymous declaration range")
		}
		path := filepath.Join(root, "force-app", "main", "default", "classes", decl.Name+".cls")
		writeFile(t, path, source[start:end])
		writeFile(t, path+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
		count++
		for i := start; i < end; i++ {
			if masked[i] != '\n' && masked[i] != '\r' {
				masked[i] = ' '
			}
		}
	}
	if count != 1 {
		t.Fatal("expected exactly one anonymous cycle declaration")
	}
	index := sema.WithAnonymousDeclarationContext(loadTestIndex(t, root))
	if analysis := sema.AnalyzeAnonymousDeclarations(index); index.HasErrors() || analysis.HasErrors() {
		t.Fatalf("native compiled; anonymous declaration diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
	}
	return index, string(masked)
}
