package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

func TestHeaderJSONNativeConformance(t *testing.T) {
	var data struct {
		APIs                 []string `json:"apiVersions"`
		Route                string
		TransportAPI         string   `json:"transportApi"`
		ProjectAPI           string   `json:"projectApi"`
		Namespace            string   `json:"namespaceContext"`
		Known                int      `json:"knownRows"`
		Pending              []string `json:"pendingCoverage"`
		AnonymousRows        int      `json:"anonymousRows"`
		AnonymousMatchedRows int      `json:"anonymousMatchedRows"`
		AnonymousCarriedRows int      `json:"anonymousCarriedRows"`
		AnonymousCaptures    []struct {
			Context, SourceAPI, TransportAPI, Namespace, Marker string
			Execution                                           struct {
				Status                           int
				Success, Compiled                bool
				CompileProblem, ExceptionMessage string
			}
			RawSHA256 map[string]string
			Source    struct{ Source, SHA256 string }
			Rows      []struct {
				ID, Classification, CarryReason string
				Native                          json.RawMessage
			}
		} `json:"anonymousCaptures"`
		Captures []struct {
			Context, SourceAPI, TestClass, Marker string
			Deployment                            struct {
				ID, Status                              string
				Done, CheckOnly                         bool
				NumberComponentErrors, NumberTestErrors int
				NumberTestsCompleted, NumberTestsTotal  int
			}
			RawSHA256 map[string]string
			Sources   map[string]struct{ Source, SHA256 string }
			Rows      []struct {
				Method        string
				Native        json.RawMessage
				NativeFailure struct{ Message string }
			}
		}
	}
	raw, err := os.ReadFile("testdata/conformance/header_json_native.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(data.APIs, []string{"62.0", "67.0", "68.0"}) || data.Route != "isTest" || data.TransportAPI != "62.0" || data.ProjectAPI != "62.0" || data.Namespace != "none" || data.Known != 18 || len(data.Captures) != 3 || len(data.Pending) == 0 {
		t.Fatal("native scope differs")
	}
	decode := func(t *testing.T, message, marker string) any {
		t.Helper()
		_, payload, ok := strings.Cut(message, marker)
		if !ok {
			t.Fatalf("native observation marker missing: %s", message)
		}
		var observed any
		if err := json.Unmarshal([]byte(payload), &observed); err != nil {
			t.Fatal(err)
		}
		return observed
	}
	seen := map[string]bool{}
	expectedAPIs := map[string]string{"api62-named": "62.0", "api67-named": "67.0", "api68-named": "68.0"}
	for _, capture := range data.Captures {
		t.Run(capture.Context, func(t *testing.T) {
			if expectedAPIs[capture.Context] != capture.SourceAPI || capture.TestClass != "OwnedHeaderJSONObservation" || capture.Marker != "HEADER_JSON_OBSERVATION|" {
				t.Fatal("native context differs")
			}
			delete(expectedAPIs, capture.Context)
			d := capture.Deployment
			if d.ID == "" || !d.Done || !d.CheckOnly || d.Status != "Failed" || d.NumberComponentErrors != 0 || d.NumberTestsCompleted != 0 || d.NumberTestErrors != d.NumberTestsTotal || d.NumberTestsTotal != len(capture.Rows) {
				t.Fatal("native deployment incomplete")
			}
			for _, name := range []string{"org.tsv", "deploy.json", "ORG-RUN.txt", "errors.json"} {
				sum, err := hex.DecodeString(capture.RawSHA256[name])
				if err != nil || len(sum) != sha256.Size {
					t.Fatalf("missing raw digest %s", name)
				}
			}
			root := t.TempDir()
			for name, source := range capture.Sources {
				sum := sha256.Sum256([]byte(source.Source))
				if filepath.IsAbs(name) || strings.Contains(name, "..") || source.Source == "" || hex.EncodeToString(sum[:]) != source.SHA256 {
					t.Fatal("captured source differs")
				}
				writeFile(t, filepath.Join(root, name), source.Source)
			}
			var metadata struct {
				API string `xml:"apiVersion"`
			}
			if err := xml.Unmarshal([]byte(capture.Sources["force-app/main/default/classes/"+capture.TestClass+".cls-meta.xml"].Source), &metadata); err != nil || metadata.API != capture.SourceAPI {
				t.Fatal("source API differs")
			}
			var project struct {
				API string `json:"sourceApiVersion"`
			}
			if err := json.Unmarshal([]byte(capture.Sources["sfdx-project.json"].Source), &project); err != nil || project.API != data.ProjectAPI {
				t.Fatal("project API differs")
			}
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native compiled; Glade diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = data.ProjectAPI
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			cases := Discover(index, Options{SelectedClasses: []string{capture.TestClass}})
			if len(cases) != len(capture.Rows) {
				t.Fatal("native method discovery differs")
			}
			methods, methodErrors := compileTestMethodsForIndex(&index, cases)
			programs, programErrors := compileTestInvokePrograms(cases)
			for _, row := range capture.Rows {
				t.Run(row.Method, func(t *testing.T) {
					keyName := capture.Context + "/" + row.Method
					if seen[keyName] {
						t.Fatal("duplicate native row")
					}
					seen[keyName] = true
					var want any
					if err := json.Unmarshal(row.Native, &want); err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(want, decode(t, row.NativeFailure.Message, capture.Marker)) {
						t.Fatal("native envelope differs")
					}
					key := ""
					for _, c := range cases {
						if c.MethodName == row.Method {
							key = testCaseKey(c)
						}
					}
					if key == "" || methodErrors[key] != nil || programErrors[key] != nil {
						t.Fatalf("named compilation: %v %v", methodErrors[key], programErrors[key])
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
					if got := decode(t, runtimeErr.ExceptionMessage(), capture.Marker); !reflect.DeepEqual(got, want) {
						t.Fatalf("native %s; Glade %s", row.Native, runtimeErr.ExceptionMessage())
					}
				})
			}
		})
	}
	for _, context := range []string{"api62-named", "api67-named", "api68-named"} {
		for _, method := range []string{"populatedHeader", "emptyHeader", "nullHeader", "plainDTOControl", "saveResultControl", "strictUnknownHeader"} {
			if !seen[context+"/"+method] {
				t.Fatalf("missing native method %s/%s", context, method)
			}
		}
	}
	if len(seen) != data.Known || len(expectedAPIs) != 0 {
		t.Fatal("native row count differs")
	}
	expectedAnonymous := map[string]string{"anonymous-api62": "62.0", "anonymous-api67": "67.0", "anonymous-api68": "68.0"}
	if data.AnonymousRows != 18 || data.AnonymousMatchedRows != 18 || data.AnonymousCarriedRows != 0 || len(data.AnonymousCaptures) != 3 {
		t.Fatal("anonymous native scope differs")
	}
	anonymousRows := 0
	anonymousMatchedRows := 0
	for _, capture := range data.AnonymousCaptures {
		t.Run(capture.Context, func(t *testing.T) {
			if expectedAnonymous[capture.Context] != capture.SourceAPI || capture.TransportAPI != capture.SourceAPI || capture.Namespace != "none" || capture.Marker != "HEADER_JSON_ANONYMOUS|" {
				t.Fatal("anonymous source/transport/context differs")
			}
			delete(expectedAnonymous, capture.Context)
			if capture.Execution.Status != 0 || !capture.Execution.Success || !capture.Execution.Compiled || capture.Execution.CompileProblem != "" || capture.Execution.ExceptionMessage != "" || len(capture.Rows) != 6 {
				t.Fatal("native anonymous execution incomplete")
			}
			for _, name := range []string{"org.tsv", "run.json", "source.apex", "ORG-RUN.txt", "errors.json"} {
				sum, err := hex.DecodeString(capture.RawSHA256[name])
				if err != nil || len(sum) != sha256.Size {
					t.Fatalf("missing anonymous digest %s", name)
				}
			}
			sum := sha256.Sum256([]byte(capture.Source.Source))
			if capture.Source.Source == "" || hex.EncodeToString(sum[:]) != capture.Source.SHA256 {
				t.Fatal("anonymous source digest differs")
			}
			analysis := sema.AnalyzeAnonymous(typesys.Index{}, capture.Source.Source, capture.SourceAPI)
			if analysis.HasErrors() {
				t.Fatalf("native anonymous compiled; Glade diagnostics=%v", analysis.Diagnostics)
			}
			program, err := vm.CompileAnonymousWithOptions(capture.Source.Source, vm.CompileOptions{APIVersion: capture.SourceAPI})
			if err != nil {
				t.Fatal(err)
			}
			runner := newConformanceRunner(t, typesys.Index{}, conformanceRunnerOptions{})
			result, err := runner.execute(program)
			if err != nil {
				t.Fatal(err)
			}
			var actual []any
			for _, message := range result.Debug {
				if strings.HasPrefix(message, capture.Marker) {
					actual = append(actual, decode(t, message, capture.Marker))
				}
			}
			if len(actual) != len(capture.Rows) {
				t.Fatalf("anonymous observations=%d want=%d", len(actual), len(capture.Rows))
			}
			cells := []string{"populatedHeader", "emptyHeader", "nullHeader", "strictUnknownHeader", "unknownHeader", "nullFieldsHeader"}
			for i, row := range capture.Rows {
				t.Run(row.ID, func(t *testing.T) {
					anonymousRows++
					if row.ID != fmt.Sprint(i) {
						t.Fatal("anonymous row identity differs")
					}
					var want map[string]any
					if err := json.Unmarshal(row.Native, &want); err != nil {
						t.Fatal(err)
					}
					if want["cell"] != cells[i] {
						t.Fatal("native anonymous cell differs")
					}
					if row.Classification != "full" || row.CarryReason != "" {
						t.Fatal("native row classification differs")
					}
					anonymousMatchedRows++
					if !reflect.DeepEqual(actual[i], want) {
						t.Fatalf("native %s; Glade %v", row.Native, actual[i])
					}
				})
			}
		})
	}
	if anonymousRows != data.AnonymousRows || anonymousMatchedRows != data.AnonymousMatchedRows || len(expectedAnonymous) != 0 {
		t.Fatal("anonymous native row count differs")
	}
}
