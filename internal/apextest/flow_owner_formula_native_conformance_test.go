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
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/automation"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

type flowOwnerNativeCapture struct {
	Context, CallerAPI, FlowAPI, HelperAPI, ProjectAPI, TransportAPI, Route, NamespaceContext, TestClass, Marker string
	SourceManifestKind, SourceManifestSHA256                                                                     string
	Deployment                                                                                                   struct {
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
	Rows                     []struct {
		ID            string
		Native        map[string]any
		NativeFailure struct{ Message string }
	}
}

func TestFlowOwnerFormulaNativeConformance(t *testing.T) {
	var data struct {
		APIs                                  []string `json:"callerApiVersions"`
		FlowAPI, ProjectAPI, NamespaceContext string
		Routes                                []string
		Known                                 int      `json:"knownRows"`
		Pending                               []string `json:"pendingCoverage"`
		Captures                              []flowOwnerNativeCapture
	}
	raw, err := os.ReadFile("testdata/conformance/flow_owner_formula_native.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(data.APIs, []string{"61.0", "62.0", "65.0", "67.0"}) || !reflect.DeepEqual(data.Routes, []string{"isTest", "anonymous"}) || data.FlowAPI != "65.0" || data.ProjectAPI != "62.0" || data.NamespaceContext != "none" || data.Known != 23 || len(data.Captures) != 9 || len(data.Pending) == 0 {
		t.Fatal("native owner-formula scope differs")
	}
	expected := map[string]string{"api65-identity": "65.0", "caller61": "61.0", "caller62": "62.0", "caller67": "67.0", "anonymous61": "61.0", "anonymous62": "62.0", "anonymous65": "65.0", "anonymous67": "67.0", "null-controls": "65.0"}
	count := 0
	for _, capture := range data.Captures {
		t.Run(capture.Context, func(t *testing.T) {
			if expected[capture.Context] != capture.CallerAPI || capture.FlowAPI != data.FlowAPI || capture.ProjectAPI != data.ProjectAPI || capture.NamespaceContext != data.NamespaceContext {
				t.Fatal("native caller/Flow/project pins differ")
			}
			delete(expected, capture.Context)
			actual := replayFlowOwnerNative(t, capture)
			if len(actual) != len(capture.Rows) {
				t.Fatal("replayed native row count differs")
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
		t.Fatal("native owner-formula row coverage differs")
	}
}

func replayFlowOwnerNative(t *testing.T, capture flowOwnerNativeCapture) map[string]map[string]any {
	t.Helper()
	anonymous := capture.Route == "anonymous"
	nullControls := capture.Context == "null-controls"
	rawNames := []string{"org.tsv", "errors.json", "ORG-RUN.txt", "deploy.json"}
	wantRows := 3
	if anonymous {
		wantRows = 2
		if capture.HelperAPI != "65.0" || capture.TransportAPI != capture.CallerAPI || capture.TestClass != "" || capture.Marker != "FLOW_OWNER_OBSERVATION|" {
			t.Fatal("anonymous source/caller scope differs")
		}
		e := capture.Execution
		if e.Status != 0 || !e.Success || !e.Compiled || e.CompileProblem != "" || e.ExceptionMessage != "" {
			t.Fatal("native anonymous execution incomplete")
		}
		rawNames = append(rawNames, "run.json", "source.apex", "setup-ORG-RUN.txt")
	} else {
		wantClass, wantMarker := "OwnerFormulaObservation", "FLOW_OWNER_OBSERVATION "
		if nullControls {
			wantClass, wantMarker = "NullFormulaObservation", "FLOW_NULL_OBSERVATION "
		}
		if capture.Route != "isTest" || capture.HelperAPI != "" || capture.TransportAPI != "62.0" || capture.TestClass != wantClass || capture.Marker != wantMarker {
			t.Fatal("named source/caller scope differs")
		}
	}
	d := capture.Deployment
	if d.ID == "" || !d.Done || d.NumberComponentErrors != 0 || len(capture.Rows) != wantRows {
		t.Fatal("native deployment incomplete")
	}
	if anonymous {
		if d.Status != "Succeeded" || d.CheckOnly || d.NumberTestErrors != 0 || d.NumberTestsCompleted != 0 || d.NumberTestsTotal != 0 {
			t.Fatal("native helper deployment incomplete")
		}
	} else if d.Status != "Failed" || !d.CheckOnly || d.NumberTestErrors != wantRows || d.NumberTestsTotal != wantRows || d.NumberTestsCompleted != 0 {
		t.Fatal("native named observation deployment incomplete")
	}
	for _, name := range rawNames {
		sum, err := hex.DecodeString(capture.RawSHA256[name])
		if err != nil || len(sum) != sha256.Size {
			t.Fatalf("native raw digest missing: %s", name)
		}
	}
	// These hashes are an export of the frozen synthetic sources. The capture
	// did not retain a pre-deployment D7 source manifest.
	manifest, err := json.Marshal(capture.SourceDigests)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(manifest)
	if capture.SourceManifestKind != "exported-frozen-sources" || hex.EncodeToString(sum[:]) != capture.SourceManifestSHA256 {
		t.Fatal("exported source manifest differs")
	}
	root := t.TempDir()
	for name, source := range capture.Sources {
		sum := sha256.Sum256([]byte(source.Source))
		if filepath.IsAbs(name) || strings.Contains(name, "..") || source.Source == "" || hex.EncodeToString(sum[:]) != source.SHA256 || capture.SourceDigests[name] != source.SHA256 {
			t.Fatal("frozen source digest differs")
		}
		writeFile(t, filepath.Join(root, name), source.Source)
	}
	projectRoot := filepath.Join(root, "project")
	var projectJSON struct {
		API string `json:"sourceApiVersion"`
	}
	if json.Unmarshal([]byte(capture.Sources["project/sfdx-project.json"].Source), &projectJSON) != nil || projectJSON.API != capture.ProjectAPI {
		t.Fatal("project API metadata differs")
	}
	class := capture.TestClass
	classAPI := capture.CallerAPI
	if anonymous {
		class = "OwnerFormulaProbe"
		classAPI = capture.HelperAPI
	}
	var classMetadata struct {
		API string `xml:"apiVersion"`
	}
	if len(capture.Sources) != 4 || xml.Unmarshal([]byte(capture.Sources["project/force-app/main/default/classes/"+class+".cls-meta.xml"].Source), &classMetadata) != nil || classMetadata.API != classAPI {
		t.Fatal("class source API differs")
	}
	flowName := "OwnedOwnerFormula"
	if nullControls {
		flowName = "OwnedNullFormula"
	}
	var flowMetadata struct {
		API    string `xml:"apiVersion"`
		Status string `xml:"status"`
	}
	if xml.Unmarshal([]byte(capture.Sources["project/force-app/main/default/flows/"+flowName+".flow-meta.xml"].Source), &flowMetadata) != nil || flowMetadata.API != capture.FlowAPI || flowMetadata.Status != "Active" {
		t.Fatal("Flow metadata scope differs")
	}
	p, err := project.Load(projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	flows, err := automation.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(flows.Diagnostics) != 0 {
		t.Fatalf("native Flow accepted; loader diagnostics=%v", flows.Diagnostics)
	}
	index := loadTestIndex(t, projectRoot)
	analysis := sema.Analyze(index)
	if index.HasErrors() || analysis.HasErrors() {
		t.Fatalf("native Apex compiled; diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
	}
	org := orgFromIndex(index)
	org.APIVersion = capture.ProjectAPI
	decode := func(message string) map[string]any {
		t.Helper()
		_, payload, ok := strings.Cut(message, capture.Marker)
		if !ok {
			t.Fatal("native observer marker missing")
		}
		var value map[string]any
		if err := json.Unmarshal([]byte(payload), &value); err != nil {
			t.Fatal(err)
		}
		return value
	}
	actual := map[string]map[string]any{}
	if anonymous {
		sum := sha256.Sum256([]byte(capture.Source.Source))
		if capture.Source.Source == "" || hex.EncodeToString(sum[:]) != capture.Source.SHA256 || capture.SourceDigests["source.apex"] != capture.Source.SHA256 || capture.RawSHA256["source.apex"] != capture.Source.SHA256 {
			t.Fatal("anonymous frozen source differs")
		}
		if analysis := sema.AnalyzeAnonymous(index, capture.Source.Source, capture.CallerAPI); analysis.HasErrors() {
			t.Fatalf("native anonymous compiled; diagnostics=%v", analysis.Diagnostics)
		}
		program, err := vm.CompileAnonymousWithOptions(capture.Source.Source, vm.CompileOptions{APIVersion: capture.CallerAPI})
		if err != nil {
			t.Fatal(err)
		}
		runtime, err := CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
		if err != nil {
			t.Fatal(err)
		}
		runner := newConformanceRunner(t, index, conformanceRunnerOptions{RequestRuntime: &runtime, Org: &org})
		result, err := runner.execute(program)
		if err != nil {
			t.Fatal(err)
		}
		for _, message := range result.Debug {
			if strings.HasPrefix(message, capture.Marker) {
				row := decode(message)
				cell, ok := row["cell"].(string)
				if !ok || actual[cell] != nil {
					t.Fatal("invalid repeated anonymous cell")
				}
				actual[cell] = row
			}
		}
		return actual
	}
	runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
	cases := Discover(index, Options{SelectedClasses: []string{capture.TestClass}})
	if len(cases) != wantRows {
		t.Fatal("named observer discovery differs")
	}
	methods, methodErrors := compileTestMethodsForIndex(&index, cases)
	programs, programErrors := compileTestInvokePrograms(cases)
	for _, row := range capture.Rows {
		if !reflect.DeepEqual(decode(row.NativeFailure.Message), row.Native) {
			t.Fatal("native payload differs from assertion envelope")
		}
		key := ""
		for _, c := range cases {
			if c.MethodName == row.ID {
				key = testCaseKey(c)
			}
		}
		if key == "" || methodErrors[key] != nil || programErrors[key] != nil {
			t.Fatal("named observer compilation differs")
		}
		machine := runner.newMachine()
		if err := machine.RegisterMethod(methods[key]); err != nil {
			t.Fatal(err)
		}
		machine.EnableTestContext()
		_, execErr := machine.ExecuteInClass(programs[key], capture.TestClass)
		var runtimeErr *vm.RuntimeError
		if !errors.As(execErr, &runtimeErr) || genericInterfaceCastExceptionType(runtimeErr.Type) != "System.AssertException" {
			t.Fatalf("native observer assertion absent: %v", execErr)
		}
		actual[row.ID] = decode(runtimeErr.Message)
	}
	return actual
}
